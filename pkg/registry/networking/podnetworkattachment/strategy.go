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

package podnetworkattachment

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

// podNetworkAttachmentStrategy implements verification logic for PodNetworkAttachment allocators.
type podNetworkAttachmentStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Replication PodNetworkAttachment objects.
var Strategy = podNetworkAttachmentStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

// Strategy should implement rest.RESTCreateStrategy
var _ rest.RESTCreateStrategy = Strategy

// Strategy should implement rest.RESTUpdateStrategy
var _ rest.RESTUpdateStrategy = Strategy

// NamespaceScoped returns true because all PodNetworkAttachments need to be within a namespace.
func (podNetworkAttachmentStrategy) NamespaceScoped() bool {
	return true
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (podNetworkAttachmentStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"networking/v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
	return fields
}

// PrepareForCreate clears the status of an PodNetworkAttachment before creation.
func (podNetworkAttachmentStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	_ = obj.(*networking.PodNetworkAttachment)

}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (podNetworkAttachmentStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPodNetworkAttachment := obj.(*networking.PodNetworkAttachment)
	oldPodNetworkAttachment := old.(*networking.PodNetworkAttachment)

	_, _ = newPodNetworkAttachment, oldPodNetworkAttachment
}

// Validate validates a new PodNetwork.
func (podNetworkAttachmentStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	podNetworkAttachment := obj.(*networking.PodNetworkAttachment)
	err := validation.ValidatePodNetworkAttachment(podNetworkAttachment)
	return err
}

// Canonicalize normalizes the object after validation.
func (podNetworkAttachmentStrategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate is false for PodNetwork; this means POST is needed to create one.
func (podNetworkAttachmentStrategy) AllowCreateOnUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (podNetworkAttachmentStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// ValidateUpdate is the default update validation for an end user.
func (podNetworkAttachmentStrategy) ValidateUpdate(ctx context.Context, new, old runtime.Object) field.ErrorList {
	newPodNetworkAttachment := new.(*networking.PodNetworkAttachment)
	oldPodNetworkAttachment := old.(*networking.PodNetworkAttachment)
	errList := validation.ValidatePodNetworkAttachment(newPodNetworkAttachment)
	errList = append(errList, validation.ValidatePodNetworkAttachmentUpdate(newPodNetworkAttachment, oldPodNetworkAttachment)...)
	return errList
}

// AllowUnconditionalUpdate is the default update policy for PodNetworkAttachment objects.
func (podNetworkAttachmentStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// WarningsOnUpdate returns warnings for the given update.
func (podNetworkAttachmentStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

type PodNetworkAttachmentStatusStrategy struct {
	podNetworkAttachmentStrategy
}

// StatusStrategy implements logic used to validate and prepare for updates of the status subresource
var StatusStrategy = PodNetworkAttachmentStatusStrategy{Strategy}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (PodNetworkAttachmentStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"networking/v1alpha1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
	return fields
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update of status
func (PodNetworkAttachmentStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPodNetworkAttachment := obj.(*networking.PodNetworkAttachment)
	oldPodNetworkAttachment := old.(*networking.PodNetworkAttachment)
	// status changes are not allowed to update spec
	newPodNetworkAttachment.Spec = oldPodNetworkAttachment.Spec
}

// ValidateUpdate is the default update validation for an end user updating status
func (PodNetworkAttachmentStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidatePodNetworkAttachmentStatusUpdate(obj.(*networking.PodNetworkAttachment), old.(*networking.PodNetworkAttachment))
}

// WarningsOnUpdate returns warnings for the given update.
func (PodNetworkAttachmentStatusStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
