/*
Copyright 2024 The Kubernetes Authors.

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

package podnetwork

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/networking"
	"k8s.io/kubernetes/pkg/apis/networking/validation"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// podNetworkStrategy implements verification logic for PodNetwork allocators.
type podNetworkStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Replication PodNetwork objects.
var Strategy = podNetworkStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

// Strategy should implement rest.RESTCreateStrategy
var _ rest.RESTCreateStrategy = Strategy

// Strategy should implement rest.RESTUpdateStrategy
var _ rest.RESTUpdateStrategy = Strategy

// NamespaceScoped returns false because all PodNetworks is cluster scoped.
func (podNetworkStrategy) NamespaceScoped() bool {
	return false
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (podNetworkStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"networking/v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
	return fields
}

// PrepareForCreate clears the status of an PodNetwork before creation.
func (podNetworkStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	_ = obj.(*networking.PodNetwork)

}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (podNetworkStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPodNetwork := obj.(*networking.PodNetwork)
	oldPodNetwork := old.(*networking.PodNetwork)

	_, _ = newPodNetwork, oldPodNetwork
}

// Validate validates a new PodNetwork.
func (podNetworkStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	podNetwork := obj.(*networking.PodNetwork)
	err := validation.ValidatePodNetwork(podNetwork)
	return err
}

// Canonicalize normalizes the object after validation.
func (podNetworkStrategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate is false for PodNetwork; this means POST is needed to create one.
func (podNetworkStrategy) AllowCreateOnUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (podNetworkStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// ValidateUpdate is the default update validation for an end user.
func (podNetworkStrategy) ValidateUpdate(ctx context.Context, new, old runtime.Object) field.ErrorList {
	newPodNetwork := new.(*networking.PodNetwork)
	oldPodNetwork := old.(*networking.PodNetwork)
	errList := validation.ValidatePodNetwork(newPodNetwork)
	errList = append(errList, validation.ValidatePodNetworkUpdate(newPodNetwork, oldPodNetwork)...)
	return errList
}

// AllowUnconditionalUpdate is the default update policy for PodNetwork objects.
func (podNetworkStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// WarningsOnUpdate returns warnings for the given update.
func (podNetworkStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

type PodNetworkStatusStrategy struct {
	podNetworkStrategy
}

// StatusStrategy implements logic used to validate and prepare for updates of the status subresource
var StatusStrategy = PodNetworkStatusStrategy{Strategy}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (PodNetworkStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"networking/v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
	return fields
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update of status
func (PodNetworkStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPodNetwork := obj.(*networking.PodNetwork)
	oldPodNetwork := old.(*networking.PodNetwork)
	// status changes are not allowed to update spec
	newPodNetwork.Spec = oldPodNetwork.Spec
}

// ValidateUpdate is the default update validation for an end user updating status
func (PodNetworkStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidatePodNetworkStatusUpdate(obj.(*networking.PodNetwork), old.(*networking.PodNetwork))
}

// WarningsOnUpdate returns warnings for the given update.
func (PodNetworkStatusStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
