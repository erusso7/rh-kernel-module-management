package signjob

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/ca"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/api"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/client"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/constants"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/utils"
)

var _ = Describe("MakeJobTemplate", func() {
	const (
		unsignedImage      = "my.registry/my/image"
		signedImage        = "my.registry/my/image-signed"
		signerImage        = "some-signer-image:some-tag"
		filesToSign        = "/modules/simple-kmod.ko:/modules/simple-procfs-kmod.ko"
		kernelVersion      = "1.2.3"
		moduleName         = "module-name"
		namespace          = "some-namespace"
		privateKey         = "some private key"
		publicKey          = "some public key"
		clusterCACMName    = "cluster-ca"
		clusterCACMKeyName = "cluster-ca-key"
		serviceCACMName    = "service-ca"
		serviceCACMKeyName = "service-ca-key"
		caMountPath        = "/etc/pki/ca-trust/extracted/pem"
	)

	var (
		ctrl      *gomock.Controller
		clnt      *client.MockClient
		mld       api.ModuleLoaderData
		m         Signer
		jobhelper *utils.MockJobHelper
		caHelper  *ca.MockHelper
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clnt = client.NewMockClient(ctrl)
		jobhelper = utils.NewMockJobHelper(ctrl)
		caHelper = ca.NewMockHelper(ctrl)
		m = NewSigner(clnt, scheme, jobhelper, caHelper)
		mld = api.ModuleLoaderData{
			Name:      moduleName,
			Namespace: namespace,
			Owner: &kmmv1beta1.Module{
				ObjectMeta: metav1.ObjectMeta{
					Name:      moduleName,
					Namespace: namespace,
				},
			},
			KernelVersion: kernelVersion,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	labels := map[string]string{"kmm.node.kubernetes.io/job-type": "sign",
		"kmm.node.kubernetes.io/module.name":   moduleName,
		"kmm.node.kubernetes.io/target-kernel": kernelVersion,
	}

	publicSignData := map[string][]byte{constants.PublicSignDataKey: []byte(publicKey)}
	privateSignData := map[string][]byte{constants.PrivateSignDataKey: []byte(privateKey)}

	DescribeTable("should set fields correctly", func(imagePullSecret *v1.LocalObjectReference) {
		GinkgoT().Setenv("RELATED_IMAGES_SIGN", signerImage)

		ctx := context.Background()
		nodeSelector := map[string]string{"arch": "x64"}

		mld.Sign = &kmmv1beta1.Sign{
			UnsignedImage: signedImage,
			KeySecret:     &v1.LocalObjectReference{Name: "securebootkey"},
			CertSecret:    &v1.LocalObjectReference{Name: "securebootcert"},
			FilesToSign:   strings.Split(filesToSign, ","),
		}
		mld.ContainerImage = signedImage
		mld.RegistryTLS = &kmmv1beta1.TLSOptions{}

		secretMount := v1.VolumeMount{
			Name:      "secret-securebootcert",
			ReadOnly:  true,
			MountPath: "/signingcert",
		}
		certMount := v1.VolumeMount{
			Name:      "secret-securebootkey",
			ReadOnly:  true,
			MountPath: "/signingkey",
		}
		trustedCAMount := v1.VolumeMount{
			Name:      "trusted-ca",
			ReadOnly:  true,
			MountPath: "/etc/pki/ca-trust/extracted/pem",
		}
		keysecret := v1.Volume{
			Name: "secret-securebootkey",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "securebootkey",
					Items: []v1.KeyToPath{
						{
							Key:  "key",
							Path: "key.priv",
						},
					},
				},
			},
		}
		certsecret := v1.Volume{
			Name: "secret-securebootcert",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "securebootcert",
					Items: []v1.KeyToPath{
						{
							Key:  "cert",
							Path: "public.der",
						},
					},
				},
			},
		}
		trustedCA := v1.Volume{
			Name: "trusted-ca",
			VolumeSource: v1.VolumeSource{
				Projected: &v1.ProjectedVolumeSource{
					Sources: []v1.VolumeProjection{
						{
							ConfigMap: &v1.ConfigMapProjection{
								LocalObjectReference: v1.LocalObjectReference{Name: clusterCACMName},
								Items: []v1.KeyToPath{
									{
										Key:  clusterCACMKeyName,
										Path: "tls-ca-bundle.pem",
									},
								},
							},
						},
						{
							ConfigMap: &v1.ConfigMapProjection{
								LocalObjectReference: v1.LocalObjectReference{Name: serviceCACMName},
								Items: []v1.KeyToPath{
									{
										Key:  serviceCACMKeyName,
										Path: "ocp-service-ca-bundle.pem",
									},
								},
							},
						},
					},
				},
			},
		}

		expected := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: mld.Name + "-sign-",
				Namespace:    namespace,
				Labels: map[string]string{
					constants.ModuleNameLabel:    moduleName,
					constants.TargetKernelTarget: kernelVersion,
					constants.JobType:            "sign",
				},

				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "kmm.sigs.x-k8s.io/v1beta1",
						Kind:               "Module",
						Name:               moduleName,
						Controller:         pointer.Bool(true),
						BlockOwnerDeletion: pointer.Bool(true),
					},
				},
			},
			Spec: batchv1.JobSpec{
				Completions:  pointer.Int32(1),
				BackoffLimit: pointer.Int32(0),
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "signimage",
								Image: signerImage,
								Env: []v1.EnvVar{
									{
										Name:  "SSL_CERT_DIR",
										Value: caMountPath,
									},
								},
								Args: []string{
									"-signedimage", signedImage,
									"-unsignedimage", unsignedImage,
									"-key", "/signingkey/key.priv",
									"-cert", "/signingcert/public.der",
									"-filestosign", filesToSign,
									"-secretdir", "/docker_config/",
								},
								VolumeMounts: []v1.VolumeMount{secretMount, certMount, trustedCAMount},
							},
						},
						NodeSelector:  nodeSelector,
						RestartPolicy: v1.RestartPolicyNever,

						Volumes: []v1.Volume{keysecret, certsecret, trustedCA},
					},
				},
			},
		}
		if imagePullSecret != nil {
			mld.ImageRepoSecret = imagePullSecret
			expected.Spec.Template.Spec.Containers[0].VolumeMounts =
				append(expected.Spec.Template.Spec.Containers[0].VolumeMounts,
					v1.VolumeMount{
						Name:      "secret-pull-push-secret",
						ReadOnly:  true,
						MountPath: "/docker_config/pull-push-secret",
					},
				)

			expected.Spec.Template.Spec.Volumes =
				append(expected.Spec.Template.Spec.Volumes,
					v1.Volume{
						Name: "secret-pull-push-secret",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: "pull-push-secret",
							},
						},
					},
				)
		}

		hash, err := getHashValue(&expected.Spec.Template, []byte(publicKey), []byte(privateKey))
		Expect(err).NotTo(HaveOccurred())
		annotations := map[string]string{constants.JobHashAnnotation: fmt.Sprintf("%d", hash)}
		expected.SetAnnotations(annotations)

		mld.Selector = nodeSelector

		gomock.InOrder(
			caHelper.
				EXPECT().
				GetClusterCA(ctx, mld.Namespace).
				Return(&ca.ConfigMap{KeyName: clusterCACMKeyName, Name: clusterCACMName}, nil),
			caHelper.
				EXPECT().
				GetServiceCA(ctx, mld.Namespace).
				Return(&ca.ConfigMap{KeyName: serviceCACMKeyName, Name: serviceCACMName}, nil),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: "builder", Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, svcaccnt *v1.ServiceAccount, _ ...ctrlclient.GetOption) error {
					svcaccnt.Secrets = []v1.ObjectReference{}
					return nil
				},
			),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: mld.Sign.KeySecret.Name, Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, secret *v1.Secret, _ ...ctrlclient.GetOption) error {
					secret.Data = privateSignData
					return nil
				},
			),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: mld.Sign.CertSecret.Name, Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, secret *v1.Secret, _ ...ctrlclient.GetOption) error {
					secret.Data = publicSignData
					return nil
				},
			),
		)

		actual, err := m.MakeJobTemplate(ctx, &mld, labels, unsignedImage, true, mld.Owner)
		Expect(err).NotTo(HaveOccurred())

		Expect(
			cmp.Diff(expected, actual),
		).To(
			BeEmpty(),
		)
	},
		Entry(
			"no secrets at all",
			nil,
		),
		Entry(
			"only imagePullSecrets",
			&v1.LocalObjectReference{Name: "pull-push-secret"},
		),
	)

	DescribeTable("should set correct kmod-signer flags", func(filelist []string, pushImage bool) {
		ctx := context.Background()
		mld.Sign = &kmmv1beta1.Sign{
			UnsignedImage: signedImage,
			KeySecret:     &v1.LocalObjectReference{Name: "securebootkey"},
			CertSecret:    &v1.LocalObjectReference{Name: "securebootcert"},
			FilesToSign:   filelist,
		}
		mld.ContainerImage = unsignedImage
		mld.RegistryTLS = &kmmv1beta1.TLSOptions{}

		gomock.InOrder(
			caHelper.EXPECT().GetClusterCA(ctx, mld.Namespace).Return(&ca.ConfigMap{}, nil),
			caHelper.EXPECT().GetServiceCA(ctx, mld.Namespace).Return(&ca.ConfigMap{}, nil),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: "builder", Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, svcaccnt *v1.ServiceAccount, _ ...ctrlclient.GetOption) error {
					svcaccnt.Secrets = []v1.ObjectReference{}
					return nil
				},
			),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: mld.Sign.KeySecret.Name, Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, secret *v1.Secret, _ ...ctrlclient.GetOption) error {
					secret.Data = privateSignData
					return nil
				},
			),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: mld.Sign.CertSecret.Name, Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, secret *v1.Secret, _ ...ctrlclient.GetOption) error {
					secret.Data = publicSignData
					return nil
				},
			),
		)

		actual, err := m.MakeJobTemplate(ctx, &mld, labels, "", pushImage, mld.Owner)

		Expect(err).NotTo(HaveOccurred())
		Expect(actual.Spec.Template.Spec.Containers[0].Args).To(ContainElement("-unsignedimage"))
		Expect(actual.Spec.Template.Spec.Containers[0].Args).To(ContainElement("-key"))
		Expect(actual.Spec.Template.Spec.Containers[0].Args).To(ContainElement("-cert"))

		if pushImage {
			Expect(actual.Spec.Template.Spec.Containers[0].Args).To(ContainElement("-signedimage"))
		} else {
			Expect(actual.Spec.Template.Spec.Containers[0].Args).To(ContainElement("-no-push"))
		}

	},
		Entry(
			"filelist and push",
			[]string{"simple-kmod", "complicated-kmod"},
			true,
		),
		Entry(
			"filelist and no push",
			[]string{"simple-kmod", "complicated-kmod"},
			false,
		),
		Entry(
			"all kmods and push",
			[]string{},
			true,
		),
		Entry(
			"all kmods and dont push",
			[]string{},
			false,
		),
	)

	DescribeTable("should set correct kmod-signer TLS flags", func(kmRegistryTLS,
		unsignedImageRegistryTLS kmmv1beta1.TLSOptions, expectedFlag string) {
		ctx := context.Background()
		mld.Sign = &kmmv1beta1.Sign{
			UnsignedImage:            signedImage,
			UnsignedImageRegistryTLS: unsignedImageRegistryTLS,
			KeySecret:                &v1.LocalObjectReference{Name: "securebootkey"},
			CertSecret:               &v1.LocalObjectReference{Name: "securebootcert"},
		}
		mld.RegistryTLS = &kmRegistryTLS
		gomock.InOrder(
			caHelper.EXPECT().GetClusterCA(ctx, mld.Namespace).Return(&ca.ConfigMap{}, nil),
			caHelper.EXPECT().GetServiceCA(ctx, mld.Namespace).Return(&ca.ConfigMap{}, nil),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: "builder", Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, svcaccnt *v1.ServiceAccount, _ ...ctrlclient.GetOption) error {
					svcaccnt.Secrets = []v1.ObjectReference{}
					return nil
				},
			),
			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: mld.Sign.KeySecret.Name, Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, secret *v1.Secret, _ ...ctrlclient.GetOption) error {
					secret.Data = privateSignData
					return nil
				},
			),

			clnt.EXPECT().Get(ctx, types.NamespacedName{Name: mld.Sign.CertSecret.Name, Namespace: mld.Namespace}, gomock.Any()).DoAndReturn(
				func(_ interface{}, _ interface{}, secret *v1.Secret, _ ...ctrlclient.GetOption) error {
					secret.Data = publicSignData
					return nil
				},
			),
		)

		actual, err := m.MakeJobTemplate(ctx, &mld, labels, "", true, mld.Owner)

		Expect(err).NotTo(HaveOccurred())
		Expect(actual.Spec.Template.Spec.Containers[0].Args).To(ContainElement(expectedFlag))
	},
		Entry(
			"filelist and push",
			kmmv1beta1.TLSOptions{
				Insecure: true,
			},
			kmmv1beta1.TLSOptions{},
			"--insecure",
		),
		Entry(
			"filelist and push",
			kmmv1beta1.TLSOptions{
				InsecureSkipTLSVerify: true,
			},
			kmmv1beta1.TLSOptions{},
			"--skip-tls-verify",
		),
		Entry(
			"filelist and push",
			kmmv1beta1.TLSOptions{},
			kmmv1beta1.TLSOptions{
				Insecure: true,
			},
			"--insecure-pull",
		),
		Entry(
			"filelist and push",
			kmmv1beta1.TLSOptions{},
			kmmv1beta1.TLSOptions{
				InsecureSkipTLSVerify: true,
			},
			"--skip-tls-verify-pull",
		),
	)
})
