/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package controllers_test

import (
	"testing"

	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	servicebindingv1beta1 "github.com/scothis/servicebinding-runtime/api/v1beta1"
	"github.com/scothis/servicebinding-runtime/controllers"
	dieservicebindingv1beta1 "github.com/scothis/servicebinding-runtime/dies/v1beta1"
)

func TestServiceBindingReconciler(t *testing.T) {
	namespace := "test-namespace"
	name := "my-image"
	key := types.NamespacedName{Namespace: namespace, Name: name}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	resource := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	rts := rtesting.ReconcilerTestSuite{{
		Name: "in sync",
		Key:  key,
		GivenObjects: []client.Object{
			resource.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers("servicebinding.io/finalizer")
				}),
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return controllers.ServiceBindingReconciler(c)
	})
}
