package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1api "dependency-controller/api/v1"
	"dependency-controller/internal/gate"
	"dependency-controller/internal/ready"
)

const requeueAfterMissing = 10 * time.Second

// DependencyReconciler reconciles a Dependency object across typed ObjectRefs.
type DependencyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	cache      cache.Cache
	controller controller.Controller

	mu      sync.Mutex
	watched map[schema.GroupVersionKind]bool
}

// Least-privilege manager-role (no wildcards). Regenerated via `make manifests`.
// update on apps/* is required: internal/gate uses client.Update to set spec.replicas
// (and original-replicas annotation). Prefer patch-only only after gate is rewritten to Patch.
// Dependency CR: no create/delete — humans use dependency-editor-role. Controller updates status.
// Custom GVKs: do not widen this role; use config/rbac/custom_dependency_reader_role.yaml.
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;replicasets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.example.com,resources=dependencies,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core.example.com,resources=dependencies/status,verbs=get;update;patch

var builtInGVKs = []schema.GroupVersionKind{
	{Group: "apps", Version: "v1", Kind: "Deployment"},
	{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
	{Group: "", Version: "v1", Kind: "Pod"},
	{Group: "batch", Version: "v1", Kind: "Job"},
}

// Reconcile enforces Compose-style depends_on for typed dependency/dependent refs.
func (r *DependencyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("dependency", req.NamespacedName)

	var dep corev1api.Dependency
	if err := r.Get(ctx, req.NamespacedName, &dep); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Dependency")
		return ctrl.Result{}, err
	}

	condition := dep.Spec.Condition
	if condition == "" {
		condition = corev1api.ConditionServiceHealthy
	}

	// Ensure watches for any custom GVKs referenced by this CR.
	if err := r.ensureWatch(refGVK(dep.Spec.Dependency)); err != nil {
		log.Error(err, "unable to watch dependency GVK")
	}
	if err := r.ensureWatch(refGVK(dep.Spec.Dependent)); err != nil {
		log.Error(err, "unable to watch dependent GVK")
	}

	dependencyObj, depErr := r.getRef(ctx, dep.Namespace, dep.Spec.Dependency)
	if depErr != nil && !errors.IsNotFound(depErr) {
		return ctrl.Result{}, depErr
	}
	dependentObj, teeErr := r.getRef(ctx, dep.Namespace, dep.Spec.Dependent)
	if teeErr != nil && !errors.IsNotFound(teeErr) {
		return ctrl.Result{}, teeErr
	}

	missingDep := errors.IsNotFound(depErr)
	missingTee := errors.IsNotFound(teeErr)
	missing := missingDep || missingTee

	var eval ready.Result
	if missingDep {
		eval = ready.Result{Ready: false, Reason: "DependencyMissing", Message: fmt.Sprintf("%s/%s not found", dep.Spec.Dependency.Kind, dep.Spec.Dependency.Name)}
	} else {
		eval = ready.Evaluate(dependencyObj, condition, dep.Spec.ReadyWhen)
	}

	statusReason := eval.Reason
	statusMessage := eval.Message
	scaledDown := false

	if !missing {
		if !eval.Ready {
			gr, err := gate.ScaleDown(ctx, r.Client, log, dependentObj)
			if err != nil {
				return ctrl.Result{}, err
			}
			if gr.Reason != "" {
				statusReason = gr.Reason
				if gr.Message != "" {
					statusMessage = gr.Message
				}
			}
			// Reload dependent
			dependentObj, _ = r.getRef(ctx, dep.Namespace, dep.Spec.Dependent)
		} else {
			gr, err := gate.ScaleUp(ctx, r.Client, log, dependentObj, dep.Spec.DesiredReplicas)
			if err != nil {
				return ctrl.Result{}, err
			}
			if gr.Reason == gate.ReasonDependentNotScalable {
				statusReason = gr.Reason
				statusMessage = gr.Message
			} else if !eval.Ready {
				// keep eval reason
			} else if gr.Reason != "" && gr.Reason != gate.ReasonAlreadyAtTarget {
				statusReason = gr.Reason
				statusMessage = gr.Message
			}
			dependentObj, _ = r.getRef(ctx, dep.Namespace, dep.Spec.Dependent)
		}
	} else {
		if missingDep {
			log.Info("dependency object not found, will requeue", "ref", formatRef(dep.Spec.Dependency))
		}
		if missingTee {
			log.Info("dependent object not found, will requeue", "ref", formatRef(dep.Spec.Dependent))
			statusReason = "DependentMissing"
			statusMessage = fmt.Sprintf("%s/%s not found", dep.Spec.Dependent.Kind, dep.Spec.Dependent.Name)
		}
	}

	if dependentObj != nil {
		scaledDown = gate.IsScaledDown(dependentObj)
		if !gate.IsScalable(dependentObj.GroupVersionKind()) && !eval.Ready {
			statusReason = gate.ReasonDependentNotScalable
			statusMessage = fmt.Sprintf("dependency not ready; dependent %s is not scalable and was left unchanged", dependentObj.GroupVersionKind().Kind)
		}
	}

	if err := r.updateStatus(ctx, &dep, eval.Ready, scaledDown, condition, statusReason, statusMessage); err != nil {
		log.Error(err, "unable to update Dependency status")
		return ctrl.Result{}, err
	}

	if missing {
		return ctrl.Result{RequeueAfter: requeueAfterMissing}, nil
	}
	return ctrl.Result{}, nil
}

func (r *DependencyReconciler) getRef(ctx context.Context, namespace string, ref corev1api.ObjectRef) (*unstructured.Unstructured, error) {
	gvk := refGVK(ref)
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func refGVK(ref corev1api.ObjectRef) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return schema.GroupVersionKind{Kind: ref.Kind}
	}
	return schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: ref.Kind}
}

func formatRef(ref corev1api.ObjectRef) string {
	return fmt.Sprintf("%s/%s/%s", ref.APIVersion, ref.Kind, ref.Name)
}

func (r *DependencyReconciler) updateStatus(ctx context.Context, dep *corev1api.Dependency, ready, scaledDown bool, condition, reason, message string) error {
	if dep.Status.DependencyReady == ready &&
		dep.Status.DependentScaledDown == scaledDown &&
		dep.Status.Condition == condition &&
		dep.Status.Reason == reason &&
		dep.Status.Message == message &&
		dep.Status.ObservedGeneration == dep.Generation {
		return nil
	}
	updated := dep.DeepCopy()
	updated.Status.DependencyReady = ready
	updated.Status.DependentScaledDown = scaledDown
	updated.Status.Condition = condition
	updated.Status.Reason = reason
	updated.Status.Message = message
	updated.Status.ObservedGeneration = dep.Generation
	return r.Status().Update(ctx, updated)
}

// SetupWithManager registers primary Dependency watch and built-in + dynamic GVK watches.
func (r *DependencyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.cache = mgr.GetCache()
	r.watched = map[schema.GroupVersionKind]bool{}

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1api.Dependency{}).
		Build(r)
	if err != nil {
		return err
	}
	r.controller = c

	for _, gvk := range builtInGVKs {
		if err := r.ensureWatch(gvk); err != nil {
			return fmt.Errorf("watch %s: %w", gvk.String(), err)
		}
	}

	return nil
}

func (r *DependencyReconciler) ensureWatch(gvk schema.GroupVersionKind) error {
	if gvk.Kind == "" || gvk.Version == "" {
		return nil
	}
	if r.controller == nil || r.cache == nil {
		// Unit tests call Reconcile without SetupWithManager.
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.watched[gvk] {
		return nil
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	src := source.Kind(r.cache, u, handler.TypedEnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj *unstructured.Unstructured) []reconcile.Request {
			return r.mapObjectToDependencies(ctx, obj)
		},
	))
	if err := r.controller.Watch(src); err != nil {
		return err
	}
	r.watched[gvk] = true
	r.Log.Info("watching GVK", "gvk", gvk.String())
	return nil
}

func (r *DependencyReconciler) mapObjectToDependencies(ctx context.Context, obj client.Object) []reconcile.Request {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		// Unstructured from cache usually has GVK; typed objects may need scheme.
		if u, ok := obj.(*unstructured.Unstructured); ok {
			gvk = u.GroupVersionKind()
		}
	}

	var list corev1api.DependencyList
	if err := r.List(ctx, &list, client.InNamespace(obj.GetNamespace())); err != nil {
		r.Log.Error(err, "unable to list Dependencies for map", "object", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()))
		return nil
	}

	var reqs []reconcile.Request
	for i := range list.Items {
		d := list.Items[i]
		if refMatches(d.Spec.Dependency, gvk, obj.GetName()) || refMatches(d.Spec.Dependent, gvk, obj.GetName()) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: d.Name, Namespace: d.Namespace},
			})
		}
	}
	return reqs
}

func refMatches(ref corev1api.ObjectRef, gvk schema.GroupVersionKind, name string) bool {
	if ref.Name != name || ref.Kind != gvk.Kind {
		return false
	}
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return false
	}
	return gv.Group == gvk.Group && gv.Version == gvk.Version
}
