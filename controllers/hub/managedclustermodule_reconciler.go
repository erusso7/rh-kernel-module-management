/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hub

import (
	"context"
	"fmt"

	buildv1 "github.com/openshift/api/build/v1"
	batchv1 "k8s.io/api/batch/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hubv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api-hub/v1beta1"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/cluster"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/filter"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/manifestwork"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/statusupdater"
)

const ManagedClusterModuleReconcilerName = "ManagedClusterModule"

// ManagedClusterModuleReconciler reconciles a ManagedClusterModule object
type ManagedClusterModuleReconciler struct {
	client client.Client

	manifestAPI      manifestwork.ManifestWorkCreator
	clusterAPI       cluster.ClusterAPI
	statusupdaterAPI statusupdater.ManagedClusterModuleStatusUpdater

	filter              *filter.Filter
	defaultJobNamespace string
}

//+kubebuilder:rbac:groups=hub.kmm.sigs.x-k8s.io,resources=managedclustermodules,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=hub.kmm.sigs.x-k8s.io,resources=managedclustermodules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hub.kmm.sigs.x-k8s.io,resources=managedclustermodules/finalizers,verbs=update
//+kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;list;watch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;delete;get;list;patch;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="core",resources=serviceaccounts,verbs=get;list;watch
//+kubebuilder:rbac:groups=build.openshift.io,resources=builds,verbs=get;list;create;delete;watch;patch

func NewManagedClusterModuleReconciler(
	client client.Client,
	manifestAPI manifestwork.ManifestWorkCreator,
	clusterAPI cluster.ClusterAPI,
	statusupdaterAPI statusupdater.ManagedClusterModuleStatusUpdater,
	filter *filter.Filter,
	defaultJobNamespace string,
) *ManagedClusterModuleReconciler {
	return &ManagedClusterModuleReconciler{
		client:              client,
		manifestAPI:         manifestAPI,
		clusterAPI:          clusterAPI,
		statusupdaterAPI:    statusupdaterAPI,
		filter:              filter,
		defaultJobNamespace: defaultJobNamespace,
	}
}

func (r *ManagedClusterModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res := ctrl.Result{}

	logger := log.FromContext(ctx)

	mcm, err := r.clusterAPI.RequestedManagedClusterModule(ctx, req.NamespacedName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("ManagedClusterModule deleted")
			return res, nil
		}

		return res, fmt.Errorf("failed to get the requested CR: %v", err)
	}

	logger.Info("Requested KMMO ManagedClusterModule")

	clusters, err := r.clusterAPI.SelectedManagedClusters(ctx, mcm)
	if err != nil {
		return res, fmt.Errorf("failed to get selected clusters: %v", err)
	}

	for _, cluster := range clusters.Items {
		logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)
		clusterCtx := log.IntoContext(ctx, logger)

		completedSuccessfully, err := r.clusterAPI.BuildAndSign(clusterCtx, *mcm, cluster)
		if err != nil {
			logger.Error(err, "failed to build")
			continue
		}
		if !completedSuccessfully {
			logger.Info("Build and Sign have not finished successfully yet; skipping ManifestWork reconciliation")
			continue
		}

		mw := &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mcm.Name,
				Namespace: cluster.Name,
			},
		}

		opRes, err := controllerutil.CreateOrPatch(clusterCtx, r.client, mw, func() error {
			return r.manifestAPI.SetManifestWorkAsDesired(ctx, mw, *mcm)
		})

		if err != nil {
			logger.Error(err, "failed to create/patch ManifestWork for managed cluster")
			continue
		}

		logger.Info("Reconciled ManifestWork", "name", mw.Name, "namespace", mw.Namespace, "result", opRes)
	}

	if err := r.manifestAPI.GarbageCollect(ctx, *clusters, *mcm); err != nil {
		return res, fmt.Errorf("failed to garbage collect ManifestWorks with no matching cluster selector: %v", err)
	}

	deleted, err := r.clusterAPI.GarbageCollectBuilds(ctx, *mcm)
	if err != nil {
		return res, fmt.Errorf("failed to garbage collect build objects: %v", err)
	}
	if len(deleted) > 0 {
		logger.Info("Garbage-collected Build objects", "names", deleted)
	}

	ownedManifestWorkList, err := r.manifestAPI.GetOwnedManifestWorks(ctx, *mcm)
	if err != nil {
		return res, fmt.Errorf("failed to fetch owned ManifestWorks of the ManagedClusterModule: %v", err)
	}
	if err := r.statusupdaterAPI.ManagedClusterModuleUpdateStatus(ctx, mcm, ownedManifestWorkList.Items); err != nil {
		return res, fmt.Errorf("failed to update status of the ManagedClusterModule: %v", err)
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedClusterModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hubv1beta1.ManagedClusterModule{}).
		Owns(&workv1.ManifestWork{}).
		Owns(&buildv1.Build{}).
		Owns(&batchv1.Job{}).
		Watches(
			&source.Kind{Type: &clusterv1.ManagedCluster{}},
			handler.EnqueueRequestsFromMapFunc(r.filter.FindManagedClusterModulesForCluster),
			builder.WithPredicates(
				r.filter.ManagedClusterModuleReconcilerManagedClusterPredicate(),
			),
		).
		Named(ManagedClusterModuleReconcilerName).
		Complete(r)
}
