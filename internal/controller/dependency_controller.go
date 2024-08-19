package controller

import (
	"context"

	corev1api "dependency-controller/api/v1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DependencyReconciler reconciles a Dependency object
type DependencyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=core.example.com,resources=dependencies,verbs=get;list;watch;update;patch

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

	// List all Dependency resources across the namespace
	var dependencyList corev1api.DependencyList
	if err := r.List(ctx, &dependencyList); err != nil {
		log.Error(err, "unable to list Dependency resources")
		return ctrl.Result{}, err
	}

	// Iterate over all Dependency resources
	for _, dependency := range dependencyList.Items {
		// Check if this deployment matches any of the Dependency resources
		if deployment.Name == dependency.Spec.Dependency && dependency.Spec.DependencyType == "Deployment" {
			log.Info("Found matching Dependency resource", "dependency", dependency.Name)
			dependencyReady := deployment.Status.AvailableReplicas > 0

			if dependency.Spec.DependentType == "Deployment" {
				// Handle dependent Deployment
				if err := r.handleDependentDeployment(ctx, req.Namespace, dependency, dependencyReady); err != nil {
					return ctrl.Result{}, err
				}
			} else if dependency.Spec.DependentType == "Job" {
				// Handle dependent Job
				if err := r.handleDependentJob(ctx, req.Namespace, dependency, dependencyReady); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *DependencyReconciler) handleDependentDeployment(ctx context.Context, namespace string, dependency corev1api.Dependency, dependencyReady bool) error {
	log := r.Log.WithValues("dependentDeployment", dependency.Spec.Dependent)

	// Fetch the dependent Deployment
	var dependentDeployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: dependency.Spec.Dependent, Namespace: namespace}, &dependentDeployment); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Dependent deployment not found")
			return nil
		}
		log.Error(err, "Error fetching Dependent deployment")
		return err
	}

	if !dependencyReady {
		// Scale down the dependent deployment if the dependency is not ready
		if *dependentDeployment.Spec.Replicas != 0 {
			zeroReplicas := int32(0)
			dependentDeployment.Spec.Replicas = &zeroReplicas
			if err := r.Update(ctx, &dependentDeployment); err != nil {
				log.Error(err, "unable to scale down Dependent deployment")
				return err
			}
			log.Info("Scaled down dependent deployment", "deployment", dependentDeployment.Name)
		}
	} else {
		// Scale up the dependent deployment if the dependency is ready
		desiredReplicas := int32(1) // Adjust this to the desired number of replicas
		if *dependentDeployment.Spec.Replicas == 0 {
			dependentDeployment.Spec.Replicas = &desiredReplicas
			if err := r.Update(ctx, &dependentDeployment); err != nil {
				log.Error(err, "unable to scale up Dependent deployment")
				return err
			}
			log.Info("Scaled up dependent deployment", "deployment", dependentDeployment.Name)
		}
	}
	return nil
}

func (r *DependencyReconciler) handleDependentJob(ctx context.Context, namespace string, dependency corev1api.Dependency, dependencyReady bool) error {
	log := r.Log.WithValues("dependentJob", dependency.Spec.Dependent)

	// Fetch the dependent Job
	var dependentJob batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Name: dependency.Spec.Dependent, Namespace: namespace}, &dependentJob)

	if !dependencyReady {
		// If dependency (Deployment) is not ready and the Job exists, store its spec and delete the Job
		if err == nil {
			// Sanitize the Job spec
			sanitizedJobSpec := sanitizeJobSpec(dependentJob.Spec.DeepCopy())

			// Store the sanitized Job spec in the DependencyStatus
			dependency.Status.LastKnownJobSpec = sanitizedJobSpec
			if updateErr := r.Status().Update(ctx, &dependency); updateErr != nil {
				log.Error(updateErr, "unable to update Dependency status with last known Job spec")
				return updateErr
			}

			// Delete the Job's pods
			if podErr := r.deleteJobPods(ctx, namespace, dependency.Spec.Dependent); podErr != nil {
				log.Error(podErr, "unable to delete pods of the dependent Job")
				return podErr
			}

			// Delete the Job
			if err := r.Delete(ctx, &dependentJob); err != nil {
				log.Error(err, "unable to delete dependent Job")
				return err
			}
			log.Info("Deleted dependent Job as dependency is not ready", "job", dependentJob.Name)
		}
	} else {
		// If dependency (Deployment) is ready and Job doesn't exist, recreate it using the stored spec
		if errors.IsNotFound(err) {
			if dependency.Status.LastKnownJobSpec != nil {
				// Recreate the Job using the stored spec, ensuring labels and selector match
				dependentJob := batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dependency.Spec.Dependent,
						Namespace: namespace,
					},
					Spec: *dependency.Status.LastKnownJobSpec,
				}

				// Set the selector to match the labels
				dependentJob.Spec.Selector = &metav1.LabelSelector{
					MatchLabels: dependentJob.Spec.Template.Labels,
				}

				if err := r.Create(ctx, &dependentJob); err != nil {
					log.Error(err, "unable to create dependent Job")
					return err
				}
				log.Info("Created dependent Job as dependency is ready", "job", dependentJob.Name)
			} else {
				log.Info("No last known Job spec found; cannot recreate Job")
			}
		}
	}
	return nil
}

// deleteJobPods deletes all pods created by the specified Job in the given namespace
func (r *DependencyReconciler) deleteJobPods(ctx context.Context, namespace string, jobName string) error {
	log := r.Log.WithValues("jobName", jobName)
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{"job-name": jobName}),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods for Job")
		return err
	}

	for _, pod := range podList.Items {
		if err := r.Delete(ctx, &pod); err != nil {
			log.Error(err, "Failed to delete pod", "podName", pod.Name)
			return err
		}
		log.Info("Deleted pod created by Job", "podName", pod.Name)
	}
	return nil
}

// sanitizeJobSpec removes Kubernetes-managed labels and ensures consistency between selector and template labels
func sanitizeJobSpec(spec *batchv1.JobSpec) *batchv1.JobSpec {
	// Remove Kubernetes-managed labels
	spec.Template.Labels = filterLabels(spec.Template.Labels)
	// Ensure the selector matches the template labels
	spec.Selector = &metav1.LabelSelector{
		MatchLabels: spec.Template.Labels,
	}
	return spec
}

// filterLabels removes Kubernetes-managed labels that could cause conflicts
func filterLabels(labels map[string]string) map[string]string {
	newLabels := make(map[string]string)
	for key, value := range labels {
		if key != "controller-uid" && key != "job-name" && key != "batch.kubernetes.io/controller-uid" && key != "batch.kubernetes.io/job-name" {
			newLabels[key] = value
		}
	}
	return newLabels
}

// SetupWithManager sets up the controller with the Manager.
func (r *DependencyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}). // Watch for changes to Deployment resources
		Complete(r)
}
