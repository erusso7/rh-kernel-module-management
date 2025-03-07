package filter

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	imagev1 "github.com/openshift/api/image/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hubv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api-hub/v1beta1"
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/constants"
)

func HasLabel(label string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[label] != ""
	})
}

var skipDeletions predicate.Predicate = predicate.Funcs{
	DeleteFunc: func(_ event.DeleteEvent) bool { return false },
}

var kmmClusterClaimChanged predicate.Predicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldManagedCluster, ok := e.ObjectOld.(*clusterv1.ManagedCluster)
		if !ok {
			return false
		}

		newManagedCluster, ok := e.ObjectNew.(*clusterv1.ManagedCluster)
		if !ok {
			return false
		}

		newClusterClaim := clusterClaim(constants.KernelVersionsClusterClaimName, newManagedCluster.Status.ClusterClaims)
		if newClusterClaim == nil {
			return false
		}
		oldClusterClaim := clusterClaim(constants.KernelVersionsClusterClaimName, oldManagedCluster.Status.ClusterClaims)

		return !reflect.DeepEqual(newClusterClaim, oldClusterClaim)
	},
}

func clusterClaim(name string, clusterClaims []clusterv1.ManagedClusterClaim) *clusterv1.ManagedClusterClaim {
	for _, clusterClaim := range clusterClaims {
		if clusterClaim.Name == name {
			return &clusterClaim
		}
	}
	return nil
}

type Filter struct {
	client client.Client
	logger logr.Logger
}

func New(client client.Client, logger logr.Logger) *Filter {
	return &Filter{
		client: client,
		logger: logger,
	}
}

func (f *Filter) ModuleReconcilerNodePredicate(kernelLabel string) predicate.Predicate {
	return predicate.And(
		skipDeletions,
		HasLabel(kernelLabel),
		predicate.LabelChangedPredicate{},
	)
}

// NodeKernelReconcilePredicate will queue the request in the following cases:
// CREATE: always, as we need to make sure we add a new entry to 'kernelToOS' mapping
// UPDATE: only if the kernel version or the os image version changed
// DELETE: never
func (f *Filter) NodeKernelReconcilerPredicate(labelName string) predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			kernelVersionChanged := e.ObjectNew.GetLabels()[labelName] != e.ObjectNew.(*v1.Node).Status.NodeInfo.KernelVersion
			osImageChanged := e.ObjectNew.(*v1.Node).Status.NodeInfo.OSImage != e.ObjectOld.(*v1.Node).Status.NodeInfo.OSImage
			return kernelVersionChanged || osImageChanged
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

func (f *Filter) ImageStreamReconcilerPredicate() predicate.Predicate {

	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newTags := map[string]string{}
			oldTags := map[string]string{}
			for _, t := range e.ObjectNew.(*imagev1.ImageStream).Spec.Tags {
				newTags[t.Name] = t.From.Name
			}
			for _, t := range e.ObjectOld.(*imagev1.ImageStream).Spec.Tags {
				oldTags[t.Name] = t.From.Name
			}
			return !reflect.DeepEqual(newTags, oldTags)
		},
	}
}

func NodeUpdateKernelChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldNode, ok := updateEvent.ObjectOld.(*v1.Node)
			if !ok {
				return false
			}

			newNode, ok := updateEvent.ObjectNew.(*v1.Node)
			if !ok {
				return false
			}

			return oldNode.Status.NodeInfo.KernelVersion != newNode.Status.NodeInfo.KernelVersion
		},
	}
}

func (f *Filter) FindModulesForNode(node client.Object) []reconcile.Request {
	logger := f.logger.WithValues("node", node.GetName())

	reqs := make([]reconcile.Request, 0)

	logger.Info("Listing all modules")

	mods := kmmv1beta1.ModuleList{}

	if err := f.client.List(context.Background(), &mods); err != nil {
		logger.Error(err, "could not list modules")
		return reqs
	}

	logger.Info("Listed modules", "count", len(mods.Items))

	nodeLabelsSet := labels.Set(node.GetLabels())

	for _, mod := range mods.Items {
		logger := logger.WithValues("module name", mod.Name)

		logger.V(1).Info("Processing module")

		sel := labels.NewSelector()

		for k, v := range mod.Spec.Selector {
			logger.V(1).Info("Processing selector item", "key", k, "value", v)

			requirement, err := labels.NewRequirement(k, selection.Equals, []string{v})
			if err != nil {
				logger.Error(err, "could not generate requirement")
				return reqs
			}

			sel = sel.Add(*requirement)
		}

		if !sel.Matches(nodeLabelsSet) {
			logger.V(1).Info("Node labels do not match the module's selector; skipping")
			continue
		}

		nsn := types.NamespacedName{Name: mod.Name, Namespace: mod.Namespace}

		reqs = append(reqs, reconcile.Request{NamespacedName: nsn})
	}

	logger.Info("Adding reconciliation requests", "count", len(reqs))
	logger.V(1).Info("New requests", "requests", reqs)

	return reqs
}

func (f *Filter) FindManagedClusterModulesForCluster(cluster client.Object) []reconcile.Request {
	logger := f.logger.WithValues("managedcluster", cluster.GetName())

	reqs := make([]reconcile.Request, 0)

	logger.Info("Listing all ManagedClusterModules")

	mods := hubv1beta1.ManagedClusterModuleList{}

	if err := f.client.List(context.Background(), &mods); err != nil {
		logger.Error(err, "could not list ManagedClusterModules")
		return reqs
	}

	logger.Info("Listed ManagedClusterModules", "count", len(mods.Items))

	clusterLabelsSet := labels.Set(cluster.GetLabels())

	for _, mod := range mods.Items {
		logger := logger.WithValues("ManagedClusterModule name", mod.Name)

		logger.V(1).Info("Processing ManagedClusterModule")

		sel := labels.NewSelector()

		for k, v := range mod.Spec.Selector {
			logger.V(1).Info("Processing selector item", "key", k, "value", v)

			requirement, err := labels.NewRequirement(k, selection.Equals, []string{v})
			if err != nil {
				logger.Error(err, "could not generate requirement")
				return reqs
			}

			sel = sel.Add(*requirement)
		}

		if !sel.Matches(clusterLabelsSet) {
			logger.V(1).Info("Cluster labels do not match the ManagedClusterModule's selector; skipping")
			continue
		}

		nsn := types.NamespacedName{Name: mod.Name}

		reqs = append(reqs, reconcile.Request{NamespacedName: nsn})
	}

	logger.Info("Adding reconciliation requests", "count", len(reqs))
	logger.V(1).Info("New requests", "requests", reqs)

	return reqs
}

func (f *Filter) ManagedClusterModuleReconcilerManagedClusterPredicate() predicate.Predicate {
	return predicate.Or(
		predicate.LabelChangedPredicate{},
		kmmClusterClaimChanged,
	)
}

func (f *Filter) EnqueueAllPreflightValidations(mod client.Object) []reconcile.Request {
	reqs := make([]reconcile.Request, 0)

	logger := f.logger.WithValues("module", mod.GetName())
	logger.Info("Listing all preflights")
	preflights := kmmv1beta1.PreflightValidationList{}
	if err := f.client.List(context.Background(), &preflights); err != nil {
		logger.Error(err, "could not list preflights")
		return reqs
	}

	for _, preflight := range preflights.Items {
		// skip the preflight being deleted
		if preflight.GetDeletionTimestamp() != nil {
			continue
		}
		nsn := types.NamespacedName{Name: preflight.Name, Namespace: preflight.Namespace}
		reqs = append(reqs, reconcile.Request{NamespacedName: nsn})
	}
	return reqs
}

// DeletingPredicate returns a predicate that returns true if the object is being deleted.
func DeletingPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return !object.GetDeletionTimestamp().IsZero()
	})
}

func MatchesNamespacedNamePredicate(nsn types.NamespacedName) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == nsn.Name && object.GetNamespace() == nsn.Namespace
	})
}

// PodHasSpecNodeName returns a predicate that returns true if the object is a *v1.Pod and its .spec.nodeName
// property is set.
func PodHasSpecNodeName() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		pod, ok := o.(*v1.Pod)
		return ok && pod.Spec.NodeName != ""
	})
}

// PodReadinessChangedPredicate returns a predicate for Update events that only returns true if the Ready condition
// changed.
func PodReadinessChangedPredicate(logger logr.Logger) predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, ok := e.ObjectOld.(*v1.Pod)
			if !ok {
				logger.Info("Old object is not a pod", "object", e.ObjectOld)
				return true
			}

			newPod, ok := e.ObjectNew.(*v1.Pod)
			if !ok {
				logger.Info("New object is not a pod", "object", e.ObjectNew)
				return true
			}

			return podutils.IsPodReady(oldPod) != podutils.IsPodReady(newPod)
		},
	}
}

func PreflightReconcilerUpdatePredicate() predicate.Predicate {
	return predicate.GenerationChangedPredicate{}
}

func PreflightOCPReconcilerUpdatePredicate() predicate.Predicate {
	return predicate.GenerationChangedPredicate{}
}
