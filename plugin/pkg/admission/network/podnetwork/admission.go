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
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controlplane/controller/defaultpodnetwork"
	"k8s.io/kubernetes/pkg/features"
)

const (
	// PluginName is the name of this admission controller plugin
	PluginName = "PodNetwork"

	DefaultInterfaceName = "eth0"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		if utilfeature.DefaultFeatureGate.Enabled(features.MultiNetwork) {
			return newPlugin(), nil
		} else {
			return nil, fmt.Errorf("%s admission controller is an alpha feature and requires the %s feature gate to be enabled", PluginName, features.MultiNetwork)
		}
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
		defaultPodNetworkexists := false

		for _, network := range pod.Spec.Networks {
			if network.PodNetworkName == defaultpodnetwork.DefaultPodNetworkName {
				defaultPodNetworkexists = true
				network.InterfaceName = DefaultInterfaceName
				network.IsDefaultGW4 = true
				network.IsDefaultGW6 = true
			}
		}

		if !defaultPodNetworkexists {
			pod.Spec.Networks = append(pod.Spec.Networks, core.Network{
				PodNetworkName: defaultpodnetwork.DefaultPodNetworkName,
				InterfaceName:  DefaultInterfaceName,
				IsDefaultGW4:   true,
				IsDefaultGW6:   true,
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
	pod, ok := a.GetObject().(*core.Pod)
	if !ok {
		return errors.NewBadRequest("resource was marked with kind Pod but was unable to be converted")
	}

	if pod.Spec.SecurityContext.HostNetwork {
		if len(pod.Spec.Networks) != 0 {
			return admission.NewForbidden(a, fmt.Errorf("networks cannot be set for a pod using the host network namespace"))
		}

		return nil
	}

	defaultPodNetworkCount := 0

	for _, network := range pod.Spec.Networks {
		if network.PodNetworkName == defaultpodnetwork.DefaultPodNetworkName {
			defaultPodNetworkCount++

			if !network.IsDefaultGW4 || !network.IsDefaultGW6 {
				return admission.NewForbidden(a, fmt.Errorf("the default network %s must be the default gateway (v4 and v6)", defaultpodnetwork.DefaultPodNetworkName))
			}
		} else if network.IsDefaultGW4 || network.IsDefaultGW6 {
			return admission.NewForbidden(a, fmt.Errorf("only %s can be the default network", defaultpodnetwork.DefaultPodNetworkName))
		}
	}

	if defaultPodNetworkCount == 0 {
		return admission.NewForbidden(a, fmt.Errorf("the default network %s must referred in the pod networks", defaultpodnetwork.DefaultPodNetworkName))
	} else if defaultPodNetworkCount > 1 {
		return admission.NewForbidden(a, fmt.Errorf("the default network %s can only be referred once in a pod", defaultpodnetwork.DefaultPodNetworkName))
	}

	return nil
}
