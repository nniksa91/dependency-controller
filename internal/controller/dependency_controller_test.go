/*
Copyright 2024 Nikola Niksa.

Licensed under the MIT License.
See the LICENSE file in the project root for full license information.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1api "dependency-controller/api/v1"
	"dependency-controller/internal/gate"
)

var _ = Describe("Dependency Controller", func() {
	const (
		ns          = "default"
		depName     = "test-dependency"
		dependencyN = "db"
		dependentN  = "app"
	)

	ctx := context.Background()
	crKey := types.NamespacedName{Name: depName, Namespace: ns}

	makeDeploy := func(name string, replicas *int32) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appsv1.DeploymentSpec{
				Replicas: replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "main", Image: "nginx:latest"}},
					},
				},
			},
			Status: appsv1.DeploymentStatus{AvailableReplicas: 0},
		}
	}

	makeSTS := func(name string, ready int32) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appsv1.StatefulSetSpec{
				Replicas:    ptr.To(int32(1)),
				Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
				ServiceName: name,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "main", Image: "nginx:latest"}},
					},
				},
			},
			Status: appsv1.StatefulSetStatus{ReadyReplicas: ready},
		}
	}

	makeCR := func(depRef, teeRef corev1api.ObjectRef, desired *int32, readyWhen *corev1api.ReadyWhen) *corev1api.Dependency {
		return &corev1api.Dependency{
			ObjectMeta: metav1.ObjectMeta{Name: depName, Namespace: ns},
			Spec: corev1api.DependencySpec{
				Dependency:      depRef,
				Dependent:       teeRef,
				Condition:       corev1api.ConditionServiceHealthy,
				DesiredReplicas: desired,
				ReadyWhen:       readyWhen,
			},
		}
	}

	deployRef := func(name string) corev1api.ObjectRef {
		return corev1api.ObjectRef{APIVersion: "apps/v1", Kind: "Deployment", Name: name}
	}
	stsRef := func(name string) corev1api.ObjectRef {
		return corev1api.ObjectRef{APIVersion: "apps/v1", Kind: "StatefulSet", Name: name}
	}

	newReconciler := func(c client.Client) *DependencyReconciler {
		return &DependencyReconciler{Client: c, Scheme: c.Scheme(), Log: GinkgoLogr}
	}

	reconcileCR := func(c client.Client) {
		_, err := newReconciler(c).Reconcile(ctx, reconcile.Request{NamespacedName: crKey})
		Expect(err).NotTo(HaveOccurred())
	}

	It("scales dependent Deployment to 0 when dependency is not ready", func() {
		c := newFakeClient(
			makeDeploy(dependencyN, ptr.To(int32(1))),
			makeDeploy(dependentN, ptr.To(int32(3))),
			makeCR(deployRef(dependencyN), deployRef(dependentN), nil, nil),
		)
		reconcileCR(c)

		dependent := &appsv1.Deployment{}
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(0)))
		Expect(dependent.Annotations[gate.OriginalReplicasAnnotation]).To(Equal("3"))

		cr := &corev1api.Dependency{}
		Expect(c.Get(ctx, crKey, cr)).To(Succeed())
		Expect(cr.Status.DependencyReady).To(BeFalse())
		Expect(cr.Status.DependentScaledDown).To(BeTrue())
	})

	It("restores replicas when dependency becomes ready", func() {
		dependency := makeDeploy(dependencyN, ptr.To(int32(1)))
		dependent := makeDeploy(dependentN, ptr.To(int32(3)))
		c := newFakeClient(dependency, dependent, makeCR(deployRef(dependencyN), deployRef(dependentN), nil, nil))
		reconcileCR(c)

		Expect(c.Get(ctx, types.NamespacedName{Name: dependencyN, Namespace: ns}, dependency)).To(Succeed())
		dependency.Status.AvailableReplicas = 1
		Expect(c.Status().Update(ctx, dependency)).To(Succeed())

		reconcileCR(c)

		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(3)))
		cr := &corev1api.Dependency{}
		Expect(c.Get(ctx, crKey, cr)).To(Succeed())
		Expect(cr.Status.DependencyReady).To(BeTrue())
	})

	It("gates Deployment on StatefulSet readiness", func() {
		c := newFakeClient(
			makeSTS(dependencyN, 0),
			makeDeploy(dependentN, ptr.To(int32(2))),
			makeCR(stsRef(dependencyN), deployRef(dependentN), nil, nil),
		)
		reconcileCR(c)

		dependent := &appsv1.Deployment{}
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(0)))

		sts := &appsv1.StatefulSet{}
		Expect(c.Get(ctx, types.NamespacedName{Name: dependencyN, Namespace: ns}, sts)).To(Succeed())
		sts.Status.ReadyReplicas = 1
		Expect(c.Status().Update(ctx, sts)).To(Succeed())

		reconcileCR(c)
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(2)))
	})

	It("uses desiredReplicas over annotation", func() {
		dependency := makeDeploy(dependencyN, ptr.To(int32(1)))
		dependent := makeDeploy(dependentN, ptr.To(int32(3)))
		c := newFakeClient(dependency, dependent, makeCR(deployRef(dependencyN), deployRef(dependentN), ptr.To(int32(5)), nil))
		reconcileCR(c)

		Expect(c.Get(ctx, types.NamespacedName{Name: dependencyN, Namespace: ns}, dependency)).To(Succeed())
		dependency.Status.AvailableReplicas = 1
		Expect(c.Status().Update(ctx, dependency)).To(Succeed())
		reconcileCR(c)

		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(5)))
	})

	It("does not mutate non-scalable dependent Pod and sets DependentNotScalable", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: dependentN, Namespace: ns},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "nginx:latest"}},
			},
		}
		c := newFakeClient(
			makeDeploy(dependencyN, ptr.To(int32(1))),
			pod,
			makeCR(deployRef(dependencyN), corev1api.ObjectRef{APIVersion: "v1", Kind: "Pod", Name: dependentN}, nil, nil),
		)
		reconcileCR(c)

		got := &corev1.Pod{}
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, got)).To(Succeed())
		// Pod still exists; not deleted.
		Expect(got.DeletionTimestamp.IsZero()).To(BeTrue())

		cr := &corev1api.Dependency{}
		Expect(c.Get(ctx, crKey, cr)).To(Succeed())
		Expect(cr.Status.Reason).To(Equal(gate.ReasonDependentNotScalable))
		Expect(cr.Status.DependencyReady).To(BeFalse())
	})

	It("evaluates custom resource dependency via readyWhen JSONPath", func() {
		db := &unstructured.Unstructured{}
		db.SetAPIVersion("db.example.com/v1")
		db.SetKind("Database")
		db.SetNamespace(ns)
		db.SetName(dependencyN)
		Expect(unstructured.SetNestedField(db.Object, "Pending", "status", "phase")).To(Succeed())

		c := newFakeClient(
			db,
			makeDeploy(dependentN, ptr.To(int32(2))),
			makeCR(
				corev1api.ObjectRef{APIVersion: "db.example.com/v1", Kind: "Database", Name: dependencyN},
				deployRef(dependentN),
				nil,
				&corev1api.ReadyWhen{JSONPath: "{.status.phase}", Value: "Ready"},
			),
		)
		reconcileCR(c)

		dependent := &appsv1.Deployment{}
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(0)))

		Expect(c.Get(ctx, types.NamespacedName{Name: dependencyN, Namespace: ns}, db)).To(Succeed())
		Expect(unstructured.SetNestedField(db.Object, "Ready", "status", "phase")).To(Succeed())
		Expect(c.Update(ctx, db)).To(Succeed())

		reconcileCR(c)
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(2)))
	})

	It("handles nil Spec.Replicas on dependent without panic", func() {
		c := newFakeClient(
			makeDeploy(dependencyN, ptr.To(int32(1))),
			makeDeploy(dependentN, nil),
			makeCR(deployRef(dependencyN), deployRef(dependentN), nil, nil),
		)
		Expect(func() { reconcileCR(c) }).NotTo(Panic())
		dependent := &appsv1.Deployment{}
		Expect(c.Get(ctx, types.NamespacedName{Name: dependentN, Namespace: ns}, dependent)).To(Succeed())
		Expect(*dependent.Spec.Replicas).To(Equal(int32(0)))
		Expect(dependent.Annotations[gate.OriginalReplicasAnnotation]).To(Equal("1"))
	})
})
