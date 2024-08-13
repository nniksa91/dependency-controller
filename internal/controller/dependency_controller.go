package controller

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1api "dependency-controller/api/v1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DependencyReconciler reconciles a Dependency object
type DependencyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core.example.com,resources=dependencies,verbs=get;list

// Reconcile is part of the main Kubernetes reconciliation loop
func (r *DependencyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deployment", req.NamespacedName)

	// Fetch the Deployment that triggered the reconciliation
	var deployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Deployment not found, might have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Deployment")
		return ctrl.Result{}, err
	}

	// List all Dependency resources in the namespace
	var dependencyList corev1api.DependencyList
	if err := r.List(ctx, &dependencyList, &client.ListOptions{Namespace: req.Namespace}); err != nil {
		log.Error(err, "unable to list Dependency resources")
		return ctrl.Result{}, err
	}

	// Iterate over all Dependency resources
	for _, dependency := range dependencyList.Items {
		// Determine if this deployment is the dependency deployment (pod1-deployment)
		if deployment.Name == dependency.Spec.Dependency {
			dependencyReady := deployment.Status.AvailableReplicas > 0

			// Fetch the dependent deployment (pod2-deployment)
			var dependentDeployment appsv1.Deployment
			if err := r.Get(ctx, types.NamespacedName{Name: dependency.Spec.Dependent, Namespace: req.Namespace}, &dependentDeployment); err != nil {
				log.Error(err, "unable to fetch Dependent deployment")
				return ctrl.Result{}, err
			}

			if !dependencyReady {
				// Scale down the dependent deployment if the dependency is not ready
				if *dependentDeployment.Spec.Replicas != 0 {
					zeroReplicas := int32(0)
					dependentDeployment.Spec.Replicas = &zeroReplicas
					if err := r.Update(ctx, &dependentDeployment); err != nil {
						log.Error(err, "unable to scale down Dependent deployment")
						return ctrl.Result{}, err
					}
					log.Info("Scaled down dependent deployment", "deployment", dependentDeployment.Name)
				}
			} else {
				// Scale up the dependent deployment if the dependency is ready
				desiredReplicas := int32(1) // This should be the desired number of replicas for the dependent deployment
				if *dependentDeployment.Spec.Replicas == 0 {
					dependentDeployment.Spec.Replicas = &desiredReplicas
					if err := r.Update(ctx, &dependentDeployment); err != nil {
						log.Error(err, "unable to scale up Dependent deployment")
						return ctrl.Result{}, err
					}
					log.Info("Scaled up dependent deployment", "deployment", dependentDeployment.Name)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DependencyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}). // Watch for changes to Deployment resources
		Complete(r)
}
