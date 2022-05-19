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

package controllers

import (
	"context"

	servicebindingv1beta1 "github.com/scothis/servicebinding-runtime/apis/v1beta1"
	"github.com/scothis/servicebinding-runtime/resolver"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=servicebinding.io,resources=servicebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// ServiceBindingReconciler reconciles a ServiceBinding object
func ServiceBindingReconciler(c reconcilers.Config) *reconcilers.ResourceReconciler {
	return &reconcilers.ResourceReconciler{
		Type: &servicebindingv1beta1.ServiceBinding{},
		Reconciler: &reconcilers.WithFinalizer{
			Finalizer: servicebindingv1beta1.GroupVersion.Group + "/finalizer",
			Reconciler: reconcilers.Sequence{
				ResolveBindingSecret(),
				ResolveWorkloads(),
			},
		},

		Config: c,
	}
}

func ResolveBindingSecret() reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "ResolveBindingSecret",
		Sync: func(ctx context.Context, parent *servicebindingv1beta1.ServiceBinding) error {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			ref := corev1.ObjectReference{
				APIVersion: parent.Spec.Service.APIVersion,
				Kind:       parent.Spec.Service.Kind,
				Namespace:  parent.Namespace,
				Name:       parent.Spec.Service.Name,
			}
			secretName, err := resolver.New(c).LookupBindingSecret(ctx, ref)
			if err != nil {
				if apierrs.IsNotFound(err) {
					// leave Unknown, the provisioned service may be created shortly
					parent.GetConditionManager().MarkUnknown(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ServiceNotFound", "the service was not found")
					return nil
				}
				if apierrs.IsForbidden(err) {
					// set False, the operator needs to give access to the resource
					// see https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac
					parent.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ServiceForbidden", "the controller does not have permission to get the service")
					return nil
				}
				// TODO handle other err cases
				return err
			}

			if secretName != "" {
				// success
				parent.GetConditionManager().MarkTrue(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ResolvedBindingSecret", "")
				parent.Status.Binding = &servicebindingv1beta1.ServiceBindingSecretReference{Name: secretName}
			} else {
				// leave Unknown, not success but also not an error
				parent.GetConditionManager().MarkUnknown(servicebindingv1beta1.ServiceBindingConditionServiceAvailable, "ServiceMissingBinding", "the service was found, but did not contain a binding secret")
				// TODO should we clear the existing binding?
				parent.Status.Binding = nil
			}

			return nil
		},
	}
}

func ResolveWorkloads() reconcilers.SubReconciler {
	return &reconcilers.SyncReconciler{
		Name: "ResolveWorkloads",
		Sync: func(ctx context.Context, parent *servicebindingv1beta1.ServiceBinding) error {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			ref := corev1.ObjectReference{
				APIVersion: parent.Spec.Workload.APIVersion,
				Kind:       parent.Spec.Workload.Kind,
				Namespace:  parent.Namespace,
				Name:       parent.Spec.Workload.Name,
			}
			workloads, err := resolver.New(c).LookupWorkloads(ctx, ref, parent.Spec.Workload.Selector)
			if err != nil {
				if apierrs.IsNotFound(err) {
					// leave Unknown, the workload may be created shortly
					parent.GetConditionManager().MarkUnknown(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadNotFound", "the workload was not found")
					return nil
				}
				if apierrs.IsForbidden(err) {
					// set False, the operator needs to give access to the resource
					// see https://servicebinding.io/spec/core/1.0.0/#considerations-for-role-based-access-control-rbac-1
					if parent.Spec.Workload.Name == "" {
						parent.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadForbidden", "the controller does not have permission to list the workloads")
					} else {
						parent.GetConditionManager().MarkFalse(servicebindingv1beta1.ServiceBindingConditionWorkloadProjected, "WorkloadForbidden", "the controller does not have permission to get the workload")
					}
					return nil
				}
				// TODO handle other err cases
				return err
			}

			StashWorkloads(ctx, workloads)

			return nil
		},
	}
}

const WorkloadsStashKey reconcilers.StashKey = "servicebinding.io:workloads"

func StashWorkloads(ctx context.Context, workloads []runtime.Object) {
	reconcilers.StashValue(ctx, WorkloadsStashKey, workloads)
}

func RetrieveWorkloads(ctx context.Context) []runtime.Object {
	value := reconcilers.RetrieveValue(ctx, WorkloadsStashKey)
	if workloads, ok := value.([]runtime.Object); ok {
		return workloads
	}
	return nil
}
