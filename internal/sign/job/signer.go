package signjob

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/hashstructure"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rh-ecosystem-edge/kernel-module-management/internal/api"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/ca"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/constants"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/utils"
)

//go:generate mockgen -source=signer.go -package=signjob -destination=mock_signer.go

type Signer interface {
	MakeJobTemplate(
		ctx context.Context,
		mld *api.ModuleLoaderData,
		labels map[string]string,
		imageToSign string,
		pushImage bool,
		owner metav1.Object,
	) (*batchv1.Job, error)
}

type hashData struct {
	PrivateKeyData []byte
	PublicKeyData  []byte
	PodTemplate    *v1.PodTemplateSpec
}

type signer struct {
	client    client.Client
	scheme    *runtime.Scheme
	jobHelper utils.JobHelper
	caHelper  ca.Helper
}

func NewSigner(
	client client.Client,
	scheme *runtime.Scheme,
	jobHelper utils.JobHelper,
	caHelper ca.Helper) Signer {
	return &signer{
		client:    client,
		scheme:    scheme,
		jobHelper: jobHelper,
		caHelper:  caHelper,
	}
}

func (s *signer) MakeJobTemplate(
	ctx context.Context,
	mld *api.ModuleLoaderData,
	labels map[string]string,
	imageToSign string,
	pushImage bool,
	owner metav1.Object) (*batchv1.Job, error) {

	signConfig := mld.Sign

	args := make([]string, 0)

	if pushImage {
		args = append(args, "-signedimage", mld.ContainerImage)

		if mld.RegistryTLS.Insecure {
			args = append(args, "--insecure")
		}
		if mld.RegistryTLS.InsecureSkipTLSVerify {
			args = append(args, "--skip-tls-verify")
		}
	} else {
		args = append(args, "-no-push")
	}

	if imageToSign != "" {
		args = append(args, "-unsignedimage", imageToSign)
	} else if signConfig.UnsignedImage != "" {
		args = append(args, "-unsignedimage", signConfig.UnsignedImage)
	} else {
		return nil, fmt.Errorf("no image to sign given")
	}
	args = append(args, "-key", "/signingkey/key.priv")
	args = append(args, "-cert", "/signingcert/public.der")

	if len(signConfig.FilesToSign) > 0 {
		args = append(args, "-filestosign", strings.Join(signConfig.FilesToSign, ":"))
	}

	if signConfig.UnsignedImageRegistryTLS.Insecure {
		args = append(args, "--insecure-pull")
	}

	if signConfig.UnsignedImageRegistryTLS.InsecureSkipTLSVerify {
		args = append(args, "--skip-tls-verify-pull")
	}

	clusterCACM, err := s.caHelper.GetClusterCA(ctx, mld.Namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get the cluster CA ConfigMap: %v", err)
	}

	servingCACM, err := s.caHelper.GetServiceCA(ctx, mld.Namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get the serving CA ConfigMap: %v", err)
	}

	const (
		trustedCAMountPath  = "/etc/pki/ca-trust/extracted/pem"
		trustedCAVolumeName = "trusted-ca"
	)

	volumes := []v1.Volume{
		utils.MakeSecretVolume(signConfig.KeySecret, "key", "key.priv"),
		utils.MakeSecretVolume(signConfig.CertSecret, "cert", "public.der"),
		{
			Name: trustedCAVolumeName,
			VolumeSource: v1.VolumeSource{
				Projected: &v1.ProjectedVolumeSource{
					Sources: []v1.VolumeProjection{
						{
							ConfigMap: &v1.ConfigMapProjection{
								LocalObjectReference: v1.LocalObjectReference{Name: clusterCACM.Name},
								Items: []v1.KeyToPath{
									{
										Key:  clusterCACM.KeyName,
										Path: "tls-ca-bundle.pem",
									},
								},
							},
						},
						{
							ConfigMap: &v1.ConfigMapProjection{
								LocalObjectReference: v1.LocalObjectReference{Name: servingCACM.Name},
								Items: []v1.KeyToPath{
									{
										Key:  servingCACM.KeyName,
										Path: "ocp-service-ca-bundle.pem",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	volumeMounts := []v1.VolumeMount{
		utils.MakeSecretVolumeMount(signConfig.CertSecret, "/signingcert"),
		utils.MakeSecretVolumeMount(signConfig.KeySecret, "/signingkey"),
		{
			Name:      trustedCAVolumeName,
			ReadOnly:  true,
			MountPath: trustedCAMountPath,
		},
	}

	imageSecret := mld.ImageRepoSecret
	buildImageSecret, err := s.getSAImageRepoSecret(ctx, mld, constants.OCPBuilderServiceAccountName)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secrets for service account %s: %v", constants.OCPBuilderServiceAccountName, err)
	}

	args = append(args, "-secretdir", "/docker_config/")
	if imageSecret != nil {
		volumes = append(volumes, utils.MakeSecretVolume(imageSecret, "", ""))
		volumeMounts = append(volumeMounts, utils.MakeSecretVolumeMount(imageSecret, "/docker_config/"+imageSecret.Name))
	}

	if len(buildImageSecret) > 0 {
		for _, secret := range buildImageSecret {
			buildSecret := &v1.LocalObjectReference{Name: secret.Name}
			volumes = append(volumes, utils.MakeSecretVolume(buildSecret, "", ""))
			volumeMounts = append(volumeMounts, utils.MakeSecretVolumeMount(buildSecret, "/docker_config/"+constants.OCPBuilderServiceAccountName+"/"+secret.Name))
		}
	}

	specTemplate := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "signimage",
					Image: os.Getenv("RELATED_IMAGES_SIGN"),
					Env: []v1.EnvVar{
						{
							Name:  "SSL_CERT_DIR",
							Value: trustedCAMountPath,
						},
					},
					Args:         args,
					VolumeMounts: volumeMounts,
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes:       volumes,
			NodeSelector:  mld.Selector,
		},
	}

	specTemplateHash, err := s.getHashAnnotationValue(ctx, signConfig.KeySecret.Name, signConfig.CertSecret.Name, mld.Namespace, &specTemplate)
	if err != nil {
		return nil, fmt.Errorf("could not hash job's definitions: %v", err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: mld.Name + "-sign-",
			Namespace:    mld.Namespace,
			Labels:       labels,
			Annotations:  map[string]string{constants.JobHashAnnotation: fmt.Sprintf("%d", specTemplateHash)},
		},
		Spec: batchv1.JobSpec{
			Completions:  pointer.Int32(1),
			Template:     specTemplate,
			BackoffLimit: pointer.Int32(0),
		},
	}

	if err := controllerutil.SetControllerReference(owner, job, s.scheme); err != nil {
		return nil, fmt.Errorf("could not set the owner reference: %v", err)
	}

	return job, nil
}

func (s *signer) getHashAnnotationValue(ctx context.Context, privateSecret, publicSecret, namespace string, podTemplate *v1.PodTemplateSpec) (uint64, error) {
	privateKeyData, err := s.getSecretData(ctx, privateSecret, constants.PrivateSignDataKey, namespace)
	if err != nil {
		return 0, fmt.Errorf("failed to get private secret %s for signing: %v", privateSecret, err)
	}
	publicKeyData, err := s.getSecretData(ctx, publicSecret, constants.PublicSignDataKey, namespace)
	if err != nil {
		return 0, fmt.Errorf("failed to get public secret %s for signing: %v", publicSecret, err)
	}

	return getHashValue(podTemplate, publicKeyData, privateKeyData)
}

func (s *signer) getSAImageRepoSecret(ctx context.Context, mld *api.ModuleLoaderData, accountName string) ([]v1.ObjectReference, error) {
	serviceaccount := v1.ServiceAccount{}

	namespacedName := types.NamespacedName{Name: accountName, Namespace: mld.Namespace}

	err := s.client.Get(ctx, namespacedName, &serviceaccount)
	if err != nil {
		return nil, err
	}

	return serviceaccount.Secrets, nil
}

func (s *signer) getSecretData(ctx context.Context, secretName, secretDataKey, namespace string) ([]byte, error) {
	secret := v1.Secret{}
	namespacedName := types.NamespacedName{Name: secretName, Namespace: namespace}
	err := s.client.Get(ctx, namespacedName, &secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret %s: %v", namespacedName, err)
	}
	data, ok := secret.Data[secretDataKey]
	if !ok {
		return nil, fmt.Errorf("invalid Secret %s format, %s key is missing", namespacedName, secretDataKey)
	}
	return data, nil
}

func getHashValue(podTemplate *v1.PodTemplateSpec, publicKeyData, privateKeyData []byte) (uint64, error) {
	dataToHash := hashData{
		PrivateKeyData: privateKeyData,
		PublicKeyData:  publicKeyData,
		PodTemplate:    podTemplate,
	}
	hashValue, err := hashstructure.Hash(dataToHash, nil)
	if err != nil {
		return 0, fmt.Errorf("could not hash job's spec template and dockefile: %v", err)
	}
	return hashValue, nil
}
