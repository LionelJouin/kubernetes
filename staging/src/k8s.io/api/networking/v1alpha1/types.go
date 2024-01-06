/*
Copyright 2022 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.27

// IPAddress represents a single IP of a single IP Family. The object is designed to be used by APIs
// that operate on IP addresses. The object is used by the Service core API for allocation of IP addresses.
// An IP address can be represented in different formats, to guarantee the uniqueness of the IP,
// the name of the object is the IP address in canonical format, four decimal digits separated
// by dots suppressing leading zeros for IPv4 and the representation defined by RFC 5952 for IPv6.
// Valid: 192.168.1.5 or 2001:db8::1 or 2001:db8:aaaa:bbbb:cccc:dddd:eeee:1
// Invalid: 10.01.2.3 or 2001:db8:0:0:0::1
type IPAddress struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// spec is the desired state of the IPAddress.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec IPAddressSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// IPAddressSpec describe the attributes in an IP Address.
type IPAddressSpec struct {
	// ParentRef references the resource that an IPAddress is attached to.
	// An IPAddress must reference a parent object.
	// +required
	ParentRef *ParentReference `json:"parentRef,omitempty" protobuf:"bytes,1,opt,name=parentRef"`
}

// ParentReference describes a reference to a parent object.
type ParentReference struct {
	// Group is the group of the object being referenced.
	// +optional
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// Resource is the resource of the object being referenced.
	// +required
	Resource string `json:"resource,omitempty" protobuf:"bytes,2,opt,name=resource"`
	// Namespace is the namespace of the object being referenced.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	// Name is the name of the object being referenced.
	// +required
	Name string `json:"name,omitempty" protobuf:"bytes,4,opt,name=name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.27

// IPAddressList contains a list of IPAddress.
type IPAddressList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// items is the list of IPAddresses.
	Items []IPAddress `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.27

// ServiceCIDR defines a range of IP addresses using CIDR format (e.g. 192.168.0.0/24 or 2001:db2::/64).
// This range is used to allocate ClusterIPs to Service objects.
type ServiceCIDR struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// spec is the desired state of the ServiceCIDR.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec ServiceCIDRSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// status represents the current state of the ServiceCIDR.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ServiceCIDRStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ServiceCIDRSpec define the CIDRs the user wants to use for allocating ClusterIPs for Services.
type ServiceCIDRSpec struct {
	// CIDRs defines the IP blocks in CIDR notation (e.g. "192.168.0.0/24" or "2001:db8::/64")
	// from which to assign service cluster IPs. Max of two CIDRs is allowed, one of each IP family.
	// This field is immutable.
	// +optional
	CIDRs []string `json:"cidrs,omitempty" protobuf:"bytes,1,opt,name=cidrs"`
}

const (
	// ServiceCIDRConditionReady represents status of a ServiceCIDR that is ready to be used by the
	// apiserver to allocate ClusterIPs for Services.
	ServiceCIDRConditionReady = "Ready"
	// ServiceCIDRReasonTerminating represents a reason where a ServiceCIDR is not ready because it is
	// being deleted.
	ServiceCIDRReasonTerminating = "Terminating"
)

// ServiceCIDRStatus describes the current state of the ServiceCIDR.
type ServiceCIDRStatus struct {
	// conditions holds an array of metav1.Condition that describe the state of the ServiceCIDR.
	// Current service state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.27

// ServiceCIDRList contains a list of ServiceCIDR objects.
type ServiceCIDRList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// items is the list of ServiceCIDRs.
	Items []ServiceCIDR `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.30

// PodNetwork represents a logical network in Kubernetes Cluster.
type PodNetwork struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the behavior of a PodNetwork.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec PodNetworkSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Most recently observed status of the PodNetwork.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status PodNetworkStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// PodNetworkSpec contains the specifications for podNetwork object
type PodNetworkSpec struct {

	// Enabled is used to administratively enable/disable a PodNetwork.
	// When set to false, PodNetwork Ready condition will be set to False.
	// Defaults to True.
	//
	// +optional
	Enabled bool `json:"enabled,omitempty" protobuf:"bytes,1,opt,name=enabled"`

	// ParametersRefs points to the vendor or implementation specific parameters
	// objects for the PodNetwork.
	//
	// +optional
	ParametersRefs []ParametersRef `json:"parametersRefs,omitempty" protobuf:"bytes,2,opt,name=parametersRefs"`

	// Provider specifies the provider implementing this PodNetwork.
	//
	// +optional
	Provider string `json:"provider,omitempty" protobuf:"bytes,3,opt,name=provider"`
}

// ParametersRef points to a custom resource containing additional
// parameters for thePodNetwork.
type ParametersRef struct {
	// Group is the API group of k8s resource e.g. k8s.cni.cncf.io
	Group string `json:"group" protobuf:"bytes,1,opt,name=group"`

	// Kind is the API name of k8s resource e.g. network-attachment-definitions
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`

	// Name of the resource.
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`

	// Namespace of the resource.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
}

// PodNetworkStatus contains the status information related to the PodNetwork.
type PodNetworkStatus struct {
	// Conditions describe the current conditions of the PodNetwork.
	//
	// Known condition types are:
	// * "Ready"
	// * "ParamsReady"
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,req,name=conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.30

// PodNetworkList is a list of PodNetwork objects.
type PodNetworkList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// items is a list of schema objects.
	Items []PodNetwork `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.30

// PodNetworkAttachment provides optional pod-level configuration of PodNetwork.
type PodNetworkAttachment struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the behavior of a PodNetworkAttachment.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec PodNetworkAttachmentSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Most recently observed status of the PodNetworkAttachment.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status PodNetworkAttachmentStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// PodNetworkAttachmentSpec is the specification for the PodNetworkAttachment resource.
type PodNetworkAttachmentSpec struct {
	// PodNetworkName refers to a PodNetwork object that this PodNetworkAttachment is
	// connected to.
	//
	// +required
	PodNetworkName string `json:"podNetworkName" protobuf:"bytes,1,req,name=podNetworkName"`

	// ParametersRefs points to the vendor or implementation specific parameters
	// object for the PodNetworkAttachment.
	//
	// +optional
	ParametersRefs []ParametersRef `json:"parametersRefs,omitempty" protobuf:"bytes,2,opt,name=parametersRefs"`
}

// PodNetworkAttachmentStatus is the status for the PodNetworkAttachment resource.
type PodNetworkAttachmentStatus struct {
	// Conditions describe the current conditions of the PodNetworkAttachment.
	//
	// Known condition types are:
	// * "Ready"
	// * "ParamsReady"
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,req,name=conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.30

// PodNetworkAttachmentList contains a list of PodNetworkAttachment.
type PodNetworkAttachmentList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// items is the list of PodNetworkAttachments.
	Items []PodNetworkAttachment `json:"items" protobuf:"bytes,2,rep,name=items"`
}
