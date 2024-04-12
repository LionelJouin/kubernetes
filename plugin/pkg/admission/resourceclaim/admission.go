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

package resourceclaim

import (
	"context"
	"io"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/apis/core"
)

const (
	// PluginName is the name of this admission controller plugin
	PluginName = "PodNetwork"

	DefaultNetworkResourceClaimTemplateName = "default-network"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return newPlugin(), nil
	})
}

// Plugin implements admission.Interface.
type Plugin struct {
	*admission.Handler
}

var _ admission.MutationInterface = &Plugin{}
var _ admission.ValidationInterface = &Plugin{}
var _ = genericadmissioninitializer.WantsExternalKubeClientSet(&Plugin{})
var _ = genericadmissioninitializer.WantsExternalKubeInformerFactory(&Plugin{})

// newPlugin creates a new admission plugin.
func newPlugin() *Plugin {
	return &Plugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

// SetExternalKubeClientSet sets the client for the plugin
func (s *Plugin) SetExternalKubeClientSet(cl kubernetes.Interface) {
}

// SetExternalKubeInformerFactory registers informers with the plugin
func (s *Plugin) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
}

// ValidateInitialization ensures an authorizer is set.
func (s *Plugin) ValidateInitialization() error {
	return nil
}

var (
	podResource = core.Resource("pods")
)

// Admit makes an admission decision based on the request attributes
func (p *Plugin) Admit(ctx context.Context, attributes admission.Attributes, o admission.ObjectInterfaces) (err error) {
	operation := attributes.GetOperation()
	// Ignore all calls to subresources
	if len(attributes.GetSubresource()) != 0 {
		return nil
	}
	switch attributes.GetResource().GroupResource() {
	case podResource:
		if operation == admission.Create || operation == admission.Update {
			return p.admitPod(attributes)
		}
		return nil

	default:
		return nil
	}
}

// admitPod adds the default pod network to the pod spec if not already set.
func (p *Plugin) admitPod(a admission.Attributes) error {
	operation := a.GetOperation()
	pod, ok := a.GetObject().(*core.Pod)
	if !ok {
		return errors.NewBadRequest("resource was marked with kind Pod but was unable to be converted")
	}

	if pod.Spec.SecurityContext.HostNetwork {
		return nil
	}

	if operation == admission.Create {
		defaultNetworkexists := false
		defaultNetworkResourceClaimTemplateName := DefaultNetworkResourceClaimTemplateName

		for _, resourceClaim := range pod.Spec.ResourceClaims {
			if resourceClaim.Source.ResourceClaimTemplateName != nil && *resourceClaim.Source.ResourceClaimTemplateName == DefaultNetworkResourceClaimTemplateName {
				defaultNetworkexists = true
			}
		}

		if !defaultNetworkexists {
			pod.Spec.ResourceClaims = append(pod.Spec.ResourceClaims, core.PodResourceClaim{
				Name: DefaultNetworkResourceClaimTemplateName,
				Source: core.ClaimSource{
					ResourceClaimTemplateName: &defaultNetworkResourceClaimTemplateName,
				},
			})

			// if pod.Spec.Containers[0].Resources.Claims == nil {
			// 	pod.Spec.Containers[0].Resources.Claims = []core.ResourceClaim{}
			// }

			pod.Spec.Containers[0].Resources.Claims = append(pod.Spec.Containers[0].Resources.Claims, core.ResourceClaim{
				Name: defaultNetworkResourceClaimTemplateName,
			})
		}
	}

	return nil
}

// Validate checks pods and admits or rejects them.
func (p *Plugin) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	operation := a.GetOperation()
	// Ignore all calls to subresources
	if len(a.GetSubresource()) != 0 {
		return nil
	}

	switch a.GetResource().GroupResource() {
	case podResource:
		if operation == admission.Create || operation == admission.Update {
			return p.validatePod(a)
		}
		return nil

	default:
		return nil
	}
}

// validatePod ensures that the the default pod network exists in the pod spec.
func (p *Plugin) validatePod(a admission.Attributes) error {
	return nil
}
