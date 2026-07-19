package ready

import (
	"fmt"
	"strings"

	corev1api "dependency-controller/api/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/jsonpath"
)

// Result is the outcome of evaluating a dependency object's readiness.
type Result struct {
	Ready   bool
	Reason  string
	Message string
}

// Evaluate checks whether obj satisfies the Compose-style condition.
func Evaluate(obj *unstructured.Unstructured, condition string, readyWhen *corev1api.ReadyWhen) Result {
	if obj == nil {
		return Result{Ready: false, Reason: "DependencyMissing", Message: "dependency object not found"}
	}
	if ts := obj.GetDeletionTimestamp(); ts != nil && !ts.IsZero() {
		return Result{Ready: false, Reason: "DependencyTerminating", Message: "dependency is terminating"}
	}

	cond := condition
	if cond == "" {
		cond = corev1api.ConditionServiceHealthy
	}

	gvk := obj.GroupVersionKind()

	switch cond {
	case corev1api.ConditionServiceStarted:
		return evaluateStarted(obj, gvk)
	case corev1api.ConditionServiceCompleted:
		return evaluateCompleted(obj, gvk)
	case corev1api.ConditionServiceHealthy:
		return evaluateHealthy(obj, gvk, readyWhen)
	default:
		return Result{Ready: false, Reason: "InvalidCondition", Message: fmt.Sprintf("unknown condition %q", cond)}
	}
}

func evaluateStarted(obj *unstructured.Unstructured, gvk schema.GroupVersionKind) Result {
	switch {
	case isKind(gvk, "Deployment", "StatefulSet", "ReplicaSet"):
		replicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "replicas")
		readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
		available, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
		if replicas > 0 || readyReplicas > 0 || available > 0 {
			return Result{Ready: true, Reason: "Started", Message: "workload has started replicas"}
		}
		// Still "started" if the object exists and is not terminating (Compose service_started).
		return Result{Ready: true, Reason: "Exists", Message: "workload object exists"}
	case isKind(gvk, "Pod"):
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase == string(corev1.PodPending) || phase == string(corev1.PodRunning) ||
			phase == string(corev1.PodSucceeded) || phase == string(corev1.PodFailed) {
			return Result{Ready: true, Reason: "PodStarted", Message: fmt.Sprintf("pod phase=%s", phase)}
		}
		return Result{Ready: true, Reason: "Exists", Message: "pod object exists"}
	case isKind(gvk, "Job"):
		return Result{Ready: true, Reason: "Exists", Message: "job object exists"}
	default:
		return Result{Ready: true, Reason: "Exists", Message: "object exists"}
	}
}

func evaluateCompleted(obj *unstructured.Unstructured, gvk schema.GroupVersionKind) Result {
	switch {
	case isKind(gvk, "Job"):
		if hasCondition(obj, string(batchv1.JobComplete), "True") {
			return Result{Ready: true, Reason: "JobComplete", Message: "job completed successfully"}
		}
		return Result{Ready: false, Reason: "JobNotComplete", Message: "job has not completed"}
	case isKind(gvk, "Pod"):
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase == string(corev1.PodSucceeded) {
			return Result{Ready: true, Reason: "PodSucceeded", Message: "pod succeeded"}
		}
		return Result{Ready: false, Reason: "PodNotSucceeded", Message: fmt.Sprintf("pod phase=%s", phase)}
	default:
		// Fall back to Ready condition / available replicas for other kinds.
		return evaluateHealthy(obj, gvk, nil)
	}
}

func evaluateHealthy(obj *unstructured.Unstructured, gvk schema.GroupVersionKind, readyWhen *corev1api.ReadyWhen) Result {
	switch {
	case isKind(gvk, "Deployment", "StatefulSet", "ReplicaSet"):
		available, found, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
		if found && available > 0 {
			return Result{Ready: true, Reason: "Available", Message: fmt.Sprintf("availableReplicas=%d", available)}
		}
		// StatefulSets often surface readyReplicas
		readyReplicas, found, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
		if found && readyReplicas > 0 {
			return Result{Ready: true, Reason: "ReadyReplicas", Message: fmt.Sprintf("readyReplicas=%d", readyReplicas)}
		}
		return Result{Ready: false, Reason: "NotAvailable", Message: "no available/ready replicas"}
	case isKind(gvk, "Pod"):
		if podReady(obj) {
			return Result{Ready: true, Reason: "PodReady", Message: "pod is Ready"}
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		return Result{Ready: false, Reason: "PodNotReady", Message: fmt.Sprintf("pod not Ready (phase=%s)", phase)}
	case isKind(gvk, "Job"):
		if hasCondition(obj, string(batchv1.JobComplete), "True") {
			return Result{Ready: true, Reason: "JobComplete", Message: "job completed"}
		}
		return Result{Ready: false, Reason: "JobNotHealthy", Message: "job not complete"}
	default:
		return evaluateCustom(obj, readyWhen)
	}
}

func evaluateCustom(obj *unstructured.Unstructured, readyWhen *corev1api.ReadyWhen) Result {
	if hasCondition(obj, "Ready", "True") {
		return Result{Ready: true, Reason: "ReadyCondition", Message: "status.conditions Ready=True"}
	}
	if readyWhen != nil && readyWhen.JSONPath != "" {
		ok, got, err := matchJSONPath(obj, readyWhen.JSONPath, readyWhen.Value)
		if err != nil {
			return Result{Ready: false, Reason: "JSONPathError", Message: err.Error()}
		}
		if ok {
			return Result{Ready: true, Reason: "JSONPathMatch", Message: fmt.Sprintf("%s == %q", readyWhen.JSONPath, readyWhen.Value)}
		}
		return Result{Ready: false, Reason: "JSONPathMismatch", Message: fmt.Sprintf("got %q want %q", got, readyWhen.Value)}
	}
	// Fall back to serviceStarted semantics for unknown CRs without Ready/readyWhen.
	return Result{Ready: true, Reason: "Exists", Message: "custom resource exists (no Ready condition or readyWhen)"}
}

func matchJSONPath(obj *unstructured.Unstructured, pathExpr, want string) (bool, string, error) {
	jp := jsonpath.New("readyWhen")
	jp.AllowMissingKeys(true)
	if err := jp.Parse(pathExpr); err != nil {
		return false, "", fmt.Errorf("parse jsonPath: %w", err)
	}
	var buf strings.Builder
	if err := jp.Execute(&buf, obj.Object); err != nil {
		return false, "", fmt.Errorf("execute jsonPath: %w", err)
	}
	got := strings.TrimSpace(buf.String())
	return got == want, got, nil
}

func podReady(obj *unstructured.Unstructured) bool {
	return hasCondition(obj, string(corev1.PodReady), "True")
}

func hasCondition(obj *unstructured.Unstructured, condType, status string) bool {
	conds, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conds {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(m, "type")
		s, _, _ := unstructured.NestedString(m, "status")
		if t == condType && s == status {
			return true
		}
	}
	return false
}

func isKind(gvk schema.GroupVersionKind, kinds ...string) bool {
	for _, k := range kinds {
		if gvk.Kind == k {
			return true
		}
	}
	return false
}
