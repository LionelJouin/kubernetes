/*
Copyright 2020 The Kubernetes Authors.

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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	networkingv1alpha1listers "k8s.io/client-go/listers/networking/v1alpha1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/feature"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/names"
)

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = names.PodNetwork

	// ErrReasonEnabled is the reason for one of the PodNetwork referred in the networks
	// of the pod to not be enabled.
	ErrReasonEnabled = "PodNetworks referred in the pod must be enabled"

	// ErrReasonExists is the reason for one of the PodNetwork referred in the networks
	// of the pod to not be exist.
	ErrReasonExists = "PodNetworks referred in the pod must exist"
)

// PodNetwork is a plugin that checks if the PodNetworks attached to the pod are
// existing and enabled.
type PodNetwork struct {
	enabled          bool
	podNetworkLister networkingv1alpha1listers.PodNetworkLister
}

var _ framework.PreFilterPlugin = &PodNetwork{}

// New initializes a new plugin and returns it.
func New(_ context.Context, pnArgs runtime.Object, fh framework.Handle, fts feature.Features) (framework.Plugin, error) {
	if !fts.EnableMultiNetworks {
		// Disabled, won't do anything.
		return &PodNetwork{}, nil
	}

	return &PodNetwork{
		enabled:          true,
		podNetworkLister: fh.SharedInformerFactory().Networking().V1alpha1().PodNetworks().Lister(),
	}, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (pn *PodNetwork) Name() string {
	return Name
}

// PreFilter invoked at the prefilter extension point to check if pod has all
// immediate claims bound. UnschedulableAndUnresolvable is returned if
// the pod cannot be scheduled at the moment on any node.
func (pn *PodNetwork) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	if !pn.enabled || pod.Spec.HostNetwork {
		return nil, framework.NewStatus(framework.Skip)
	}

	for _, network := range pod.Spec.Networks {
		podNetwork, err := pn.podNetworkLister.Get(network.PodNetworkName)
		if err != nil { // todo: check if error is not existing.
			return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonExists)
		}

		if !podNetwork.Spec.Enabled {
			return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonEnabled)
		}
	}

	return nil, nil
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (pn *PodNetwork) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}
