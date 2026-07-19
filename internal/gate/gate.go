package gate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// OriginalReplicasAnnotation stores the dependent's replica count before scale-down.
	OriginalReplicasAnnotation = "dependency-controller/original-replicas"

	ReasonDependentNotScalable = "DependentNotScalable"
	ReasonScaledDown           = "ScaledDown"
	ReasonScaledUp             = "ScaledUp"
	ReasonAlreadyAtTarget      = "AlreadyAtTarget"
)

// ScalableKinds can be gated by adjusting spec.replicas.
var ScalableKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"ReplicaSet":  true,
}

// IsScalable reports whether the GVK supports replica gating.
func IsScalable(gvk schema.GroupVersionKind) bool {
	return ScalableKinds[gvk.Kind]
}

// Result describes a gate action.
type Result struct {
	ScaledDown bool
	Mutated    bool
	Reason     string
	Message    string
}

// ScaleDown sets replicas to 0 after saving the current count in an annotation.
// Non-scalable objects are left untouched.
func ScaleDown(ctx context.Context, c client.Client, log logr.Logger, obj *unstructured.Unstructured) (Result, error) {
	if obj == nil {
		return Result{Reason: "DependentMissing", Message: "dependent object not found"}, nil
	}
	gvk := obj.GroupVersionKind()
	if !IsScalable(gvk) {
		return Result{
			Reason:  ReasonDependentNotScalable,
			Message: fmt.Sprintf("%s/%s is not a scalable kind; left unchanged", gvk.Kind, obj.GetName()),
		}, nil
	}

	current := replicasOrDefault(obj)
	if current == 0 {
		return Result{ScaledDown: true, Reason: ReasonAlreadyAtTarget, Message: "dependent already at 0 replicas"}, nil
	}

	updated := obj.DeepCopy()
	anns := updated.GetAnnotations()
	if anns == nil {
		anns = map[string]string{}
	}
	if _, ok := anns[OriginalReplicasAnnotation]; !ok {
		anns[OriginalReplicasAnnotation] = strconv.FormatInt(int64(current), 10)
		updated.SetAnnotations(anns)
	}
	if err := unstructured.SetNestedField(updated.Object, int64(0), "spec", "replicas"); err != nil {
		return Result{}, err
	}
	if err := c.Update(ctx, updated); err != nil {
		return Result{}, err
	}
	log.Info("Scaled down dependent", "kind", gvk.Kind, "name", updated.GetName(), "from", current)
	return Result{
		ScaledDown: true,
		Mutated:    true,
		Reason:     ReasonScaledDown,
		Message:    fmt.Sprintf("scaled %s/%s from %d to 0", gvk.Kind, updated.GetName(), current),
	}, nil
}

// ScaleUp restores replicas to desired (CR override), annotation, or 1.
func ScaleUp(ctx context.Context, c client.Client, log logr.Logger, obj *unstructured.Unstructured, desiredReplicas *int32) (Result, error) {
	if obj == nil {
		return Result{Reason: "DependentMissing", Message: "dependent object not found"}, nil
	}
	gvk := obj.GroupVersionKind()
	if !IsScalable(gvk) {
		return Result{
			Reason:  ReasonDependentNotScalable,
			Message: fmt.Sprintf("%s/%s is not a scalable kind; left unchanged", gvk.Kind, obj.GetName()),
		}, nil
	}

	target := resolveDesiredReplicas(obj, desiredReplicas)
	current := replicasOrDefault(obj)
	if current == target {
		return Result{
			ScaledDown: false,
			Reason:     ReasonAlreadyAtTarget,
			Message:    fmt.Sprintf("dependent already at %d replicas", target),
		}, nil
	}

	updated := obj.DeepCopy()
	if err := unstructured.SetNestedField(updated.Object, int64(target), "spec", "replicas"); err != nil {
		return Result{}, err
	}
	if err := c.Update(ctx, updated); err != nil {
		return Result{}, err
	}
	log.Info("Scaled up dependent", "kind", gvk.Kind, "name", updated.GetName(), "replicas", target)
	return Result{
		ScaledDown: false,
		Mutated:    true,
		Reason:     ReasonScaledUp,
		Message:    fmt.Sprintf("scaled %s/%s to %d", gvk.Kind, updated.GetName(), target),
	}, nil
}

// IsScaledDown reports whether a scalable object is at 0 replicas.
func IsScaledDown(obj *unstructured.Unstructured) bool {
	if obj == nil || !IsScalable(obj.GroupVersionKind()) {
		return false
	}
	return replicasOrDefault(obj) == 0
}

func replicasOrDefault(obj *unstructured.Unstructured) int32 {
	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil || !found {
		return 1
	}
	return int32(replicas)
}

func resolveDesiredReplicas(obj *unstructured.Unstructured, desired *int32) int32 {
	if desired != nil && *desired > 0 {
		return *desired
	}
	if anns := obj.GetAnnotations(); anns != nil {
		if raw, ok := anns[OriginalReplicasAnnotation]; ok {
			n, err := strconv.ParseInt(raw, 10, 32)
			if err == nil && n > 0 {
				return int32(n)
			}
		}
	}
	return 1
}
