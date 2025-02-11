/*
Copyright 2025.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kccnv1alpha1 "github.com/mproffitt/kubeconfig-operator/api/v1alpha1"
	kubeconfig "github.com/mproffitt/kubeconfig-operator/internal/kubeconfig"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeconfig.choclab.net,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeconfig.choclab.net,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeconfig.choclab.net,resources=clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var cluster kccnv1alpha1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		log.Error(err, "unable to fetch Cluster")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	metadata := cluster.GetObjectMeta()
	if metadata.GetDeletionTimestamp() != nil {
		log.Info("Cluster is being deleted", "name", metadata.GetName())
		return ctrl.Result{}, nil
	}

	if cluster.Spec.Suspend {
		log.Info("Cluster is suspended, skipping reconciliation", "name", metadata.GetName())
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling Cluster", "name", metadata.GetName())

	manager := kubeconfig.NewManager(ctx, r.Client, &cluster)

	statuses, err := manager.ReconcileKubeconfig()
	if err != nil {
		log.Error(err, "unable to reconcile kubeconfig", "name", metadata.GetName())
		return ctrl.Result{}, err
	}

	cluster.Status.Clusters = statuses.ClusterStatus
	cluster.Status.FirewallRules = statuses.FirewallRules
	cluster.Status.DeletionRules = statuses.DeletionRules

	if err := r.Status().Update(ctx, &cluster); err != nil {
		log.Error(err, "unable to update Cluster status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		RequeueAfter: cluster.Spec.ReconcileInterval.Duration,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kccnv1alpha1.Cluster{}).
		Named("cluster").
		Complete(r)
}
