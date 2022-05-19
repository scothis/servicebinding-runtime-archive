/*
Copyright 2021 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package controllers_test

import (
	"fmt"
	"testing"

	dieappsv1 "dies.dev/apis/apps/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestServiceBindingReconciler(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"
	key := types.NamespacedName{Namespace: namespace, Name: name}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	secretName := "my-secret"
	directSecretRef := dieservicebindingv1beta1.ServiceBindingServiceReferenceBlank.
		APIVersion("v1").
		Kind("Secret").
		Name(secretName)

	rts := rtesting.ReconcilerTestSuite{{
		Name: "in sync",
		Key:  key,
		GivenObjects: []client.Object{
			serviceBinding.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.Finalizers("servicebinding.io/finalizer")
				}).
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}).
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady,
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected,
					)
					d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
				}),
		},
	}, {
		Name: "newly created",
		Key:  key,
		GivenObjects: []client.Object{
			serviceBinding.
				SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
					d.Service(directSecretRef.DieRelease())
				}),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "FinalizerPatched", "Patched finalizer %q", "servicebinding.io/finalizer"),
			rtesting.NewEvent(serviceBinding, scheme, corev1.EventTypeNormal, "StatusUpdated", "Updated status"),
		},
		ExpectPatches: []rtesting.PatchRef{
			{
				Group:     "servicebinding.io",
				Kind:      "ServiceBinding",
				Namespace: serviceBinding.GetNamespace(),
				Name:      serviceBinding.GetName(),
				PatchType: types.MergePatchType,
				Patch:     []byte(`{"metadata":{"finalizers":["servicebinding.io/finalizer"],"resourceVersion":"999"}}`),
			},
		},
		ExpectStatusUpdates: []client.Object{
			serviceBinding.
				StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
					d.ConditionsDie(
						dieservicebindingv1beta1.ServiceBindingConditionReady,
						dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.True().Reason("ResolvedBindingSecret"),
						dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected,
					)
					d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
						d.Name(secretName)
					})
				}),
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return controllers.ServiceBindingReconciler(c)
	})
}

func TestResolveBindingSecret(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	secretName := "my-secret"
	directSecretRef := dieservicebindingv1beta1.ServiceBindingServiceReferenceBlank.
		APIVersion("v1").
		Kind("Secret").
		Name(secretName)
	serviceRef := dieservicebindingv1beta1.ServiceBindingServiceReferenceBlank.
		APIVersion("example/v1").
		Kind("MyProvisionedService").
		Name("my-service")

	notProvisionedService := &unstructured.Unstructured{}
	notProvisionedService.SetAPIVersion("example/v1")
	notProvisionedService.SetKind("MyProvisionedService")
	notProvisionedService.SetNamespace(namespace)
	notProvisionedService.SetName("my-service")
	notProvisionedService.SetResourceVersion("999")
	provisionedService := notProvisionedService.DeepCopy()
	provisionedService.UnstructuredContent()["status"] = map[string]interface{}{
		"binding": map[string]interface{}{
			"name": secretName,
		},
	}

	rts := rtesting.SubReconcilerTestSuite{{
		Name: "resolve direct secret",
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(directSecretRef.DieRelease())
			}),
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(directSecretRef.DieRelease())
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
					d.Name(secretName)
				})
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
						True().Reason("ResolvedBindingSecret"),
				)
			}),
	}, {
		Name: "service is a provisioned service",
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}),
		GivenObjects: []client.Object{
			provisionedService,
		},
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.BindingDie(func(d *dieservicebindingv1beta1.ServiceBindingSecretReferenceDie) {
					d.Name(secretName)
				})
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
						True().Reason("ResolvedBindingSecret"),
				)
			}),
	}, {
		Name: "service is not a provisioned service",
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}),
		GivenObjects: []client.Object{
			notProvisionedService,
		},
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionReady.
						Reason("ServiceMissingBinding").
						Message("the service was found, but did not contain a binding secret"),
					dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
						Reason("ServiceMissingBinding").
						Message("the service was found, but did not contain a binding secret"),
				)
			}),
	}, {
		Name: "service not found",
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}),
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "MyProvisionedService", rtesting.InduceFailureOpts{
				Error: apierrs.NewNotFound(schema.GroupResource{}, "my-service"),
			}),
		},
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionReady.
						Reason("ServiceNotFound").
						Message("the service was not found"),
					dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
						Reason("ServiceNotFound").
						Message("the service was not found"),
				)
			}),
	}, {
		Name: "service forbidden",
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}),
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "MyProvisionedService", rtesting.InduceFailureOpts{
				Error: apierrs.NewForbidden(schema.GroupResource{}, "my-service", fmt.Errorf("test forbidden")),
			}),
		},
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionReady.
						False().
						Reason("ServiceForbidden").
						Message("the controller does not have permission to get the service"),
					dieservicebindingv1beta1.ServiceBindingConditionServiceAvailable.
						False().
						Reason("ServiceForbidden").
						Message("the controller does not have permission to get the service"),
				)
			}),
	}, {
		Name: "service generic get error",
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.Service(serviceRef.DieRelease())
			}),
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "MyProvisionedService"),
		},
		ShouldErr: true,
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.ResolveBindingSecret()
	})
}

func TestResolveWorkload(t *testing.T) {
	namespace := "test-namespace"
	name := "my-binding"

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(servicebindingv1beta1.AddToScheme(scheme))

	serviceBinding := dieservicebindingv1beta1.ServiceBindingBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	workload := dieappsv1.DeploymentBlank.
		DieStamp(func(r *appsv1.Deployment) {
			r.APIVersion = "apps/v1"
			r.Kind = "Deployment"
			r.ResourceVersion = "999"
		})
	workload1 := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload-1")
			d.AddLabel("app", "my")
		})
	workload2 := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("my-workload-2")
			d.AddLabel("app", "my")
		})
	workload3 := workload.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("not-my-workload")
			d.AddLabel("app", "not")
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name: "resolve named workload",
		GivenObjects: []client.Object{
			workload1,
			workload2,
			workload3,
		},
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.Name("my-workload-1")
				})
			}),
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WorkloadsStashKey: []runtime.Object{
				workload1.DieReleaseUnstructured(),
			},
		},
	}, {
		Name: "resolve named workload not found",
		GivenObjects: []client.Object{
			workload1,
			workload2,
			workload3,
		},
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.Name("my-workload-not-found")
				})
			}),
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.Name("my-workload-not-found")
				})
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionReady.
						Reason("WorkloadNotFound").Message("the workload was not found"),
					dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
						Reason("WorkloadNotFound").Message("the workload was not found"),
				)
			}),
	}, {
		Name: "resolve named workload forbidden",
		GivenObjects: []client.Object{
			workload1,
			workload2,
			workload3,
		},
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.Name("my-workload-1")
				})
			}),
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("get", "Deployment", rtesting.InduceFailureOpts{
				Error: apierrs.NewForbidden(schema.GroupResource{}, "my-workload-1", fmt.Errorf("test forbidden")),
			}),
		},
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.Name("my-workload-1")
				})
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionReady.
						False().
						Reason("WorkloadForbidden").
						Message("the controller does not have permission to get the workload"),
					dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
						False().
						Reason("WorkloadForbidden").
						Message("the controller does not have permission to get the workload"),
				)
			}),
	}, {
		Name: "resolve selected workload",
		GivenObjects: []client.Object{
			workload1,
			workload2,
			workload3,
		},
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
						d.AddMatchLabel("app", "my")
					})
				})
			}),
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WorkloadsStashKey: []runtime.Object{
				workload1.DieReleaseUnstructured(),
				workload2.DieReleaseUnstructured(),
			},
		},
	}, {
		Name:         "resolve selected workload not found",
		GivenObjects: []client.Object{},
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
						d.AddMatchLabel("app", "my")
					})
				})
			}),
		ExpectStashedValues: map[reconcilers.StashKey]interface{}{
			controllers.WorkloadsStashKey: []runtime.Object{},
		},
	}, {
		Name: "resolve selected workload forbidden",
		GivenObjects: []client.Object{
			workload1,
			workload2,
			workload3,
		},
		Resource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
						d.AddMatchLabel("app", "my")
					})
				})
			}),
		WithReactors: []rtesting.ReactionFunc{
			rtesting.InduceFailure("list", "DeploymentList", rtesting.InduceFailureOpts{
				Error: apierrs.NewForbidden(schema.GroupResource{}, "", fmt.Errorf("test forbidden")),
			}),
		},
		ExpectResource: serviceBinding.
			SpecDie(func(d *dieservicebindingv1beta1.ServiceBindingSpecDie) {
				d.WorkloadDie(func(d *dieservicebindingv1beta1.ServiceBindingWorkloadReferenceDie) {
					d.APIVersion("apps/v1")
					d.Kind("Deployment")
					d.SelectorDie(func(d *diemetav1.LabelSelectorDie) {
						d.AddMatchLabel("app", "my")
					})
				})
			}).
			StatusDie(func(d *dieservicebindingv1beta1.ServiceBindingStatusDie) {
				d.ConditionsDie(
					dieservicebindingv1beta1.ServiceBindingConditionReady.
						False().
						Reason("WorkloadForbidden").
						Message("the controller does not have permission to list the workloads"),
					dieservicebindingv1beta1.ServiceBindingConditionWorkloadProjected.
						False().
						Reason("WorkloadForbidden").
						Message("the controller does not have permission to list the workloads"),
				)
			}),
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.ResolveWorkloads()
	})
}
