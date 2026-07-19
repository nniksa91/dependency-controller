package ready

import (
	"testing"

	corev1api "dependency-controller/api/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeploymentHealthy(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "app"},
		"status":     map[string]interface{}{"availableReplicas": int64(2)},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceHealthy, nil)
	if !r.Ready {
		t.Fatalf("expected ready, got %#v", r)
	}
}

func TestDeploymentNotHealthy(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "app"},
		"status":     map[string]interface{}{"availableReplicas": int64(0)},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceHealthy, nil)
	if r.Ready {
		t.Fatalf("expected not ready, got %#v", r)
	}
}

func TestStatefulSetReadyReplicas(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "StatefulSet",
		"metadata":   map[string]interface{}{"name": "db"},
		"status":     map[string]interface{}{"readyReplicas": int64(1)},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceHealthy, nil)
	if !r.Ready {
		t.Fatalf("expected ready, got %#v", r)
	}
}

func TestJobCompleted(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata":   map[string]interface{}{"name": "migrate"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceCompleted, nil)
	if !r.Ready {
		t.Fatalf("expected ready, got %#v", r)
	}
}

func TestPodReady(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "p"},
		"status": map[string]interface{}{
			"phase": "Running",
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceHealthy, nil)
	if !r.Ready {
		t.Fatalf("expected ready, got %#v", r)
	}
}

func TestCustomJSONPath(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "db.example.com/v1",
		"kind":       "Database",
		"metadata":   map[string]interface{}{"name": "mydb"},
		"status":     map[string]interface{}{"phase": "Ready"},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceHealthy, &corev1api.ReadyWhen{
		JSONPath: "{.status.phase}",
		Value:    "Ready",
	})
	if !r.Ready {
		t.Fatalf("expected ready, got %#v", r)
	}
}

func TestCustomReadyCondition(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "db.example.com/v1",
		"kind":       "Database",
		"metadata":   map[string]interface{}{"name": "mydb"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceHealthy, nil)
	if !r.Ready {
		t.Fatalf("expected ready, got %#v", r)
	}
}

func TestServiceStartedExists(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "app"},
		"status":     map[string]interface{}{"availableReplicas": int64(0)},
	}}
	r := Evaluate(obj, corev1api.ConditionServiceStarted, nil)
	if !r.Ready {
		t.Fatalf("expected started when object exists, got %#v", r)
	}
}
