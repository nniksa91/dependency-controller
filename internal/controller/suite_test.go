/*
Copyright 2024.

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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	depv1 "dependency-controller/api/v1"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(s)).To(Succeed())
	Expect(appsv1.AddToScheme(s)).To(Succeed())
	Expect(batchv1.AddToScheme(s)).To(Succeed())
	Expect(corev1.AddToScheme(s)).To(Succeed())
	Expect(depv1.AddToScheme(s)).To(Succeed())

	// Example CR used in unit tests (unstructured).
	gv := schema.GroupVersion{Group: "db.example.com", Version: "v1"}
	s.AddKnownTypeWithName(gv.WithKind("Database"), &unstructured.Unstructured{})
	s.AddKnownTypeWithName(gv.WithKind("DatabaseList"), &unstructured.UnstructuredList{})
	metav1.AddToGroupVersion(s, gv)
	return s
}

func newFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithObjects(objs...).
		WithStatusSubresource(&depv1.Dependency{}, &appsv1.Deployment{}, &appsv1.StatefulSet{}).
		Build()
}
