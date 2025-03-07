package statusupdater

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hubv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api-hub/v1beta1"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/daemonset"
)

//go:generate mockgen -source=statusupdater.go -package=statusupdater -destination=mock_statusupdater.go

type ModuleStatusUpdater interface {
	ModuleUpdateStatus(ctx context.Context, mod *kmmv1beta1.Module, kernelMappingNodes []v1.Node,
		targetedNodes []v1.Node, dsByKernelVersion map[string]*appsv1.DaemonSet) error
}

//go:generate mockgen -source=statusupdater.go -package=statusupdater -destination=mock_statusupdater.go

type ManagedClusterModuleStatusUpdater interface {
	ManagedClusterModuleUpdateStatus(ctx context.Context, mcm *hubv1beta1.ManagedClusterModule,
		ownedManifestWorks []workv1.ManifestWork) error
}

//go:generate mockgen -source=statusupdater.go -package=statusupdater -destination=mock_statusupdater.go

type PreflightStatusUpdater interface {
	PreflightPresetStatuses(ctx context.Context, pv *kmmv1beta1.PreflightValidation,
		existingModules sets.String, newModules []string) error
	PreflightSetVerificationStatus(ctx context.Context, preflight *kmmv1beta1.PreflightValidation, moduleName string,
		verificationStatus string, message string) error
	PreflightSetVerificationStage(ctx context.Context, preflight *kmmv1beta1.PreflightValidation,
		moduleName string, stage string) error
}

type PreflightOCPStatusUpdater interface {
	PreflightOCPUpdateStatus(ctx context.Context, pvo *kmmv1beta1.PreflightValidationOCP, pv *kmmv1beta1.PreflightValidation) error
}

type moduleStatusUpdater struct {
	client client.Client
}

type managedClusterModuleStatusUpdater struct {
	client client.Client
}

type preflightStatusUpdater struct {
	client client.Client
}

type preflightOCPStatusUpdater struct {
	client client.Client
}

func NewModuleStatusUpdater(client client.Client) ModuleStatusUpdater {
	return &moduleStatusUpdater{
		client: client,
	}
}

func NewManagedClusterModuleStatusUpdater(client client.Client) ManagedClusterModuleStatusUpdater {
	return &managedClusterModuleStatusUpdater{
		client: client,
	}
}

func NewPreflightStatusUpdater(client client.Client) PreflightStatusUpdater {
	return &preflightStatusUpdater{
		client: client,
	}
}

func NewPreflightOCPStatusUpdater(client client.Client) PreflightOCPStatusUpdater {
	return &preflightOCPStatusUpdater{
		client: client,
	}
}

func (m *moduleStatusUpdater) ModuleUpdateStatus(ctx context.Context,
	mod *kmmv1beta1.Module,
	kernelMappingNodes []v1.Node,
	targetedNodes []v1.Node,
	dsByKernelVersion map[string]*appsv1.DaemonSet) error {

	nodesMatchingSelectorNumber := int32(len(targetedNodes))
	numDesired := int32(len(kernelMappingNodes))
	var numAvailableDevicePlugin int32
	var numAvailableKernelModule int32
	for kernelVersion, ds := range dsByKernelVersion {
		if daemonset.IsDevicePluginKernelVersion(kernelVersion) {
			numAvailableDevicePlugin += ds.Status.NumberAvailable
		} else {
			numAvailableKernelModule += ds.Status.NumberAvailable
		}
	}

	unmodifiedMod := mod.DeepCopy()

	mod.Status.ModuleLoader.NodesMatchingSelectorNumber = nodesMatchingSelectorNumber
	mod.Status.ModuleLoader.DesiredNumber = numDesired
	mod.Status.ModuleLoader.AvailableNumber = numAvailableKernelModule
	if mod.Spec.DevicePlugin != nil {
		mod.Status.DevicePlugin.NodesMatchingSelectorNumber = nodesMatchingSelectorNumber
		mod.Status.DevicePlugin.DesiredNumber = numDesired
		mod.Status.DevicePlugin.AvailableNumber = numAvailableDevicePlugin
	}
	return m.client.Status().Patch(ctx, mod, client.MergeFrom(unmodifiedMod))
}

func (m *managedClusterModuleStatusUpdater) ManagedClusterModuleUpdateStatus(ctx context.Context,
	mcm *hubv1beta1.ManagedClusterModule,
	ownedManifestWorks []workv1.ManifestWork) error {

	var numApplied int32
	var numDegraded int32
	for _, mw := range ownedManifestWorks {
		for _, condition := range mw.Status.Conditions {
			if condition.Status != metav1.ConditionTrue {
				continue
			}

			switch condition.Type {
			case workv1.WorkApplied:
				numApplied += 1
			case workv1.WorkDegraded:
				numDegraded += 1
			}
		}
	}

	unmodifiedMCM := mcm.DeepCopy()

	mcm.Status.NumberDesired = int32(len(ownedManifestWorks))
	mcm.Status.NumberApplied = numApplied
	mcm.Status.NumberDegraded = numDegraded

	return m.client.Status().Patch(ctx, mcm, client.MergeFrom(unmodifiedMCM))
}

func (p *preflightStatusUpdater) PreflightPresetStatuses(ctx context.Context,
	pv *kmmv1beta1.PreflightValidation, existingModules sets.String, newModules []string) error {

	modulesInStatus := sets.StringKeySet(pv.Status.CRStatuses)
	modulesToDelete := modulesInStatus.Difference(existingModules).UnsortedList()
	for _, moduleName := range modulesToDelete {
		delete(pv.Status.CRStatuses, moduleName)
	}

	for _, moduleName := range newModules {
		pv.Status.CRStatuses[moduleName] = &kmmv1beta1.CRStatus{
			VerificationStatus: kmmv1beta1.VerificationFalse,
			VerificationStage:  kmmv1beta1.VerificationStageImage,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
	}
	return p.client.Status().Update(ctx, pv)
}

func (p *preflightStatusUpdater) PreflightSetVerificationStatus(ctx context.Context, pv *kmmv1beta1.PreflightValidation, moduleName string,
	verificationStatus string, message string) error {
	if _, ok := pv.Status.CRStatuses[moduleName]; !ok {
		return fmt.Errorf("failed to find module status %s in preflight %s", moduleName, pv.Name)
	}
	pv.Status.CRStatuses[moduleName].VerificationStatus = verificationStatus
	pv.Status.CRStatuses[moduleName].StatusReason = message
	pv.Status.CRStatuses[moduleName].LastTransitionTime = metav1.NewTime(time.Now())
	return p.client.Status().Update(ctx, pv)
}

func (p *preflightStatusUpdater) PreflightSetVerificationStage(ctx context.Context, pv *kmmv1beta1.PreflightValidation,
	moduleName string, stage string) error {
	if _, ok := pv.Status.CRStatuses[moduleName]; !ok {
		return fmt.Errorf("failed to find module status %s in preflight %s", moduleName, pv.Name)
	}
	pv.Status.CRStatuses[moduleName].VerificationStage = stage
	pv.Status.CRStatuses[moduleName].LastTransitionTime = metav1.NewTime(time.Now())
	return p.client.Status().Update(ctx, pv)
}

func (p *preflightOCPStatusUpdater) PreflightOCPUpdateStatus(ctx context.Context, pvo *kmmv1beta1.PreflightValidationOCP, pv *kmmv1beta1.PreflightValidation) error {
	pv.Status.DeepCopyInto(&pvo.Status)
	return p.client.Status().Update(ctx, pvo)
}
