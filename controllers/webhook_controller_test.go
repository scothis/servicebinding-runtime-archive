/*
Copyright 2022 Scott Andrews.

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

package controllers_test

import (
	"testing"

	dieadmissionregistrationv1 "dies.dev/apis/admissionregistration/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	servicebindingv1beta1 "github.com/scothis/servicebinding-runtime/apis/v1beta1"
	"github.com/scothis/servicebinding-runtime/controllers"
	dieservicebindingv1beta1 "github.com/scothis/servicebinding-runtime/dies/v1beta1"
)

func TestAdmissionProjectorReconciler(t *testing.T) {
	name := "my-webhook"
	key := types.NamespacedName{Name: name}

	now := metav1.Now().Rfc3339Copy()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.MutatingWebhookConfigurationBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
			d.CreationTimestamp(now)
		}).
		WebhookDie("projector.servicebinding.io", func(d *dieadmissionregistrationv1.MutatingWebhookDie) {
			d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
				d.ServiceDie(func(d *dieadmissionregistrationv1.ServiceReferenceDie) {
					d.Namespace("my-system")
					d.Name("my-service")
				})
			})
			d.RulesDie(
				dieadmissionregistrationv1.RuleWithOperationsBlank.
					APIGroups("apps").
					APIVersions("*").
					Resources("deployments").
					Operations(
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
					),
			)
		})

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.ReconcilerTestSuite{{
		Name: "in sync",
		Key:  key,
		GivenObjects: []client.Object{
			webhook,
			serviceBinding,
		},
	}, {
		Name: "update",
		Key:  key,
		GivenObjects: []client.Object{
			webhook.
				WebhookDie("projector.servicebinding.io", func(d *dieadmissionregistrationv1.MutatingWebhookDie) {
					d.Rules()
				}),
			serviceBinding,
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(webhook, scheme, corev1.EventTypeNormal, "Updated", "Updated MutatingWebhookConfiguration %q", name),
		},
		ExpectUpdates: []client.Object{
			webhook,
		},
	}, {
		Name: "ignore other keys",
		Key: types.NamespacedName{
			Name: "other-webhook",
		},
		GivenObjects: []client.Object{
			webhook.
				WebhookDie("projector.servicebinding.io", func(d *dieadmissionregistrationv1.MutatingWebhookDie) {
					d.Rules()
				}),
			serviceBinding,
		},
	}, {
		Name: "ignore malformed webhook",
		Key:  key,
		GivenObjects: []client.Object{
			webhook.
				Webhooks(),
			serviceBinding,
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		return controllers.AdmissionProjectorReconciler(c, name)
	})
}

func TestTriggerReconciler(t *testing.T) {
	name := "my-webhook"
	key := types.NamespacedName{Name: name}

	now := metav1.Now().Rfc3339Copy()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
			d.CreationTimestamp(now)
		}).
		WebhookDie("trigger.servicebinding.io", func(d *dieadmissionregistrationv1.ValidatingWebhookDie) {
			d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
				d.ServiceDie(func(d *dieadmissionregistrationv1.ServiceReferenceDie) {
					d.Namespace("my-system")
					d.Name("my-service")
				})
			})
			d.RulesDie(
				dieadmissionregistrationv1.RuleWithOperationsBlank.
					APIGroups("apps").
					APIVersions("*").
					Resources("deployments").
					Operations(
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					),
				dieadmissionregistrationv1.RuleWithOperationsBlank.
					APIGroups("example").
					APIVersions("*").
					Resources("myservices").
					Operations(
						admissionregistrationv1.Create,
						admissionregistrationv1.Update,
						admissionregistrationv1.Delete,
					),
			)
		})

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.ReconcilerTestSuite{{
		Name: "in sync",
		Key:  key,
		GivenObjects: []client.Object{
			webhook,
			serviceBinding,
		},
	}, {
		Name: "update",
		Key:  key,
		GivenObjects: []client.Object{
			webhook.
				WebhookDie("trigger.servicebinding.io", func(d *dieadmissionregistrationv1.ValidatingWebhookDie) {
					d.Rules()
				}),
			serviceBinding,
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(webhook, scheme, corev1.EventTypeNormal, "Updated", "Updated ValidatingWebhookConfiguration %q", name),
		},
		ExpectUpdates: []client.Object{
			webhook,
		},
	}, {
		Name: "ignore other keys",
		Key: types.NamespacedName{
			Name: "other-webhook",
		},
		GivenObjects: []client.Object{
			webhook.
				WebhookDie("trigger.servicebinding.io", func(d *dieadmissionregistrationv1.ValidatingWebhookDie) {
					d.Rules()
				}),
			serviceBinding,
		},
	}, {
		Name: "ignore malformed webhook",
		Key:  key,
		GivenObjects: []client.Object{
			webhook.
				Webhooks(),
			serviceBinding,
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "example", Version: "v1", Kind: "MyService"}, meta.RESTScopeNamespace)
		return controllers.TriggerReconciler(c, name)
	})
}

func TestLoadServiceBindings(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
			d.ResourceVersion("999")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:     "list all servicebindings",
		Resource: webhook,
		GivenObjects: []client.Object{
			serviceBinding,
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
				serviceBinding.DieRelease(),
			},
		},
	}, {
		Name:     "error listing all servicebindings",
		Resource: webhook,
		GivenObjects: []client.Object{
			serviceBinding,
		},
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "ServiceBindingList"),
		},
		ShouldErr: true,
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-webhook"}}
		return controllers.LoadServiceBindings(req)
	})
}

func TestInterceptGVKs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
			d.ResourceVersion("999")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:     "collect workload gvks",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
				serviceBinding.DieRelease(),
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
	}, {
		Name:     "append workload gvks",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
				serviceBinding.DieRelease(),
			},
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "example", Version: "v1", Kind: "MyService"},
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "example", Version: "v1", Kind: "MyService"},
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.InterceptGVKs()
	})
}

func TestTriggerGVKs(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace("my-namespace")
			d.Name("my-binding")
			d.ResourceVersion("999")
		}).
		SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
			d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
				d.APIVersion("example/v1")
				d.Kind("MyService")
				d.Name("my-service")
			})
			d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
				d.APIVersion("apps/v1")
				d.Kind("Deployment")
				d.Name("my-workload")
			})
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:     "collect service gvks",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
				serviceBinding.DieRelease(),
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "example", Version: "v1", Kind: "MyService"},
			},
		},
	}, {
		Name:     "append service gvks",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
				serviceBinding.DieRelease(),
			},
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
				{Group: "example", Version: "v1", Kind: "MyService"},
			},
		},
	}, {
		Name:     "ignore direct binding",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ServiceBindingsStashKey: []servicebindingv1beta1.ServiceBinding{
				serviceBinding.
					SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
						d.ServiceDie(func(d *dieservicebindingv1beta1.ServiceBindingServiceReferenceDie) {
							d.APIVersion("v1")
							d.Kind("Secret")
						})
					}).
					DieRelease(),
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{},
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.TriggerGVKs()
	})
}

func TestWebhookRules(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	webhook := dieadmissionregistrationv1.ValidatingWebhookConfigurationBlank

	operations := []admissionregistrationv1.OperationType{
		admissionregistrationv1.Connect,
	}

	rts := rtesting.SubReconcilerTestSuite{{
		Name:     "empty",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{},
		},
	}, {
		Name:     "convert",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: operations,
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"apps"},
						APIVersions: []string{"*"},
						Resources:   []string{"deployments"},
					},
				},
			},
		},
	}, {
		Name:     "dedup versions",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "Deployment"},
				{Group: "apps", Version: "v1beta1", Kind: "Deployment"},
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: operations,
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"apps"},
						APIVersions: []string{"*"},
						Resources:   []string{"deployments"},
					},
				},
			},
		},
	}, {
		Name:     "merge resources of same group",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "apps", Version: "v1", Kind: "StatefulSet"},
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: operations,
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"apps"},
						APIVersions: []string{"*"},
						Resources:   []string{"deployments", "statefulsets"},
					},
				},
			},
		},
	}, {
		Name:     "preserve resources of different group",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "batch", Version: "v1", Kind: "Job"},
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WebhookRulesStashKey: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: operations,
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"apps"},
						APIVersions: []string{"*"},
						Resources:   []string{"deployments"},
					},
				},
				{
					Operations: operations,
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"batch"},
						APIVersions: []string{"*"},
						Resources:   []string{"jobs"},
					},
				},
			},
		},
	}, {
		Name:     "error on unknown resource",
		Resource: webhook,
		GivenStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.ObservedGVKsStashKey: []schema.GroupVersionKind{
				{Group: "foo", Version: "v1", Kind: "Bar"},
			},
		},
		ShouldErr: true,
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		restMapper := c.RESTMapper().(*meta.DefaultRESTMapper)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1beta1", Kind: "Deployment"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, meta.RESTScopeNamespace)

		return controllers.WebhookRules(operations)
	})
}
