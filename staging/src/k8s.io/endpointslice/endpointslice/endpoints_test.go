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

package endpointslice

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	endpointsliceutil "k8s.io/endpointslice/util"
	endpointsv1 "k8s.io/kubernetes/pkg/api/v1/endpoints"
	"k8s.io/kubernetes/test/utils/ktesting"
	"k8s.io/utils/ptr"
)

const defaultMaxEndpointsPerSubset = int32(1000)

// TestReconcile ensures that Endpoints are reconciled into corresponding
// EndpointSlices with appropriate fields.
func TestReconcile(t *testing.T) {
	protoTCP := corev1.ProtocolTCP
	protoUDP := corev1.ProtocolUDP

	testCases := []struct {
		testName                 string
		subsets                  []corev1.EndpointSubset
		epLabels                 map[string]string
		epAnnotations            map[string]string
		endpointsDeletionPending bool
		maxEndpointsPerSubset    int32
		existingEndpointSlices   []*discovery.EndpointSlice
		expectedNumSlices        int
		expectedClientActions    int
		expectedMetrics          *expectedMetrics
	}{{
		testName:               "Endpoints with no subsets",
		subsets:                []corev1.EndpointSubset{},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      0,
		expectedClientActions:  0,
		expectedMetrics:        &expectedMetrics{},
	}, {
		testName: "Endpoints with no addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      0,
		expectedClientActions:  0,
		expectedMetrics:        &expectedMetrics{},
	}, {
		testName: "Endpoints with 1 subset, port, and address",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{desiredSlices: 1, actualSlices: 1, desiredEndpoints: 1, addedPerSync: 1, numCreated: 1},
	}, {
		testName: "Endpoints with 2 subset, different port and address",
		subsets: []corev1.EndpointSubset{
			{
				Ports: []corev1.EndpointPort{{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP:       "10.0.0.1",
					Hostname: "pod-1",
					NodeName: ptr.To("node-1"),
				}},
			},
			{
				Ports: []corev1.EndpointPort{{
					Name:     "https",
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP:       "10.0.0.2",
					Hostname: "pod-2",
					NodeName: ptr.To("node-1"),
				}},
			},
		},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      2,
		expectedClientActions:  2,
		expectedMetrics:        &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 2, addedPerSync: 2, numCreated: 2},
	}, {
		testName: "Endpoints with 2 subset, different port and same address",
		subsets: []corev1.EndpointSubset{
			{
				Ports: []corev1.EndpointPort{{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP:       "10.0.0.1",
					Hostname: "pod-1",
					NodeName: ptr.To("node-1"),
				}},
			},
			{
				Ports: []corev1.EndpointPort{{
					Name:     "https",
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP:       "10.0.0.1",
					Hostname: "pod-1",
					NodeName: ptr.To("node-1"),
				}},
			},
		},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{desiredSlices: 1, actualSlices: 1, desiredEndpoints: 1, addedPerSync: 1, numCreated: 1},
	}, {
		testName: "Endpoints with 2 subset, different address and same port",
		subsets: []corev1.EndpointSubset{
			{
				Ports: []corev1.EndpointPort{{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP:       "10.0.0.1",
					Hostname: "pod-1",
					NodeName: ptr.To("node-1"),
				}},
			},
			{
				Ports: []corev1.EndpointPort{{
					Name:     "http",
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				}},
				Addresses: []corev1.EndpointAddress{{
					IP:       "10.0.0.2",
					Hostname: "pod-2",
					NodeName: ptr.To("node-1"),
				}},
			},
		},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{desiredSlices: 1, actualSlices: 1, desiredEndpoints: 2, addedPerSync: 2, numCreated: 1},
	}, {
		testName: "Endpoints with 1 subset, port, and address, pending deletion",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}},
		}},
		endpointsDeletionPending: true,
		existingEndpointSlices:   []*discovery.EndpointSlice{},
		expectedNumSlices:        0,
		expectedClientActions:    0,
	}, {
		testName: "Endpoints with 1 subset, port, and address and existing slice with same fields",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ep-1",
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}},
			Endpoints: []discovery.Endpoint{{
				Addresses:  []string{"10.0.0.1"},
				Hostname:   ptr.To("pod-1"),
				Conditions: discovery.EndpointConditions{Ready: ptr.To(true)},
			}},
		}},
		expectedNumSlices:     1,
		expectedClientActions: 0,
	}, {
		testName: "Endpoints with 1 subset, port, and address and existing slice with an additional annotation",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-ep-1",
				Annotations: map[string]string{"foo": "bar"},
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}},
			Endpoints: []discovery.Endpoint{{
				Addresses:  []string{"10.0.0.1"},
				Hostname:   ptr.To("pod-1"),
				Conditions: discovery.EndpointConditions{Ready: ptr.To(true)},
			}},
		}},
		expectedNumSlices:     1,
		expectedClientActions: 1,
	}, {
		testName: "Endpoints with 1 subset, port, label and address and existing slice with same fields but the label",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		epLabels: map[string]string{"foo": "bar"},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-ep-1",
				Annotations: map[string]string{"foo": "bar"},
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}},
			Endpoints: []discovery.Endpoint{{
				Addresses:  []string{"10.0.0.1"},
				Hostname:   ptr.To("pod-1"),
				Conditions: discovery.EndpointConditions{Ready: ptr.To(true)},
			}},
		}},
		expectedNumSlices:     1,
		expectedClientActions: 1,
	}, {
		testName: "Endpoints with 1 subset, 2 ports, and 2 addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{desiredSlices: 1, actualSlices: 1, desiredEndpoints: 2, addedPerSync: 2, numCreated: 1},
	}, {
		testName: "Endpoints with 1 subset, 2 ports, and 2 not ready addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			NotReadyAddresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{desiredSlices: 1, actualSlices: 1, desiredEndpoints: 2, addedPerSync: 2, numCreated: 1},
	}, {
		testName: "Endpoints with 1 subset, 2 ports, and 2 ready and 2 not ready addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.1.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.1.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}},
			NotReadyAddresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{desiredSlices: 1, actualSlices: 1, desiredEndpoints: 4, addedPerSync: 4, numCreated: 1},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports and addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "10.0.1.3",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      2,
		expectedClientActions:  2,
		expectedMetrics:        &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 5, addedPerSync: 5, numCreated: 2},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports and addresses, existing empty EndpointSlice",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "10.0.1.3",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ep-1",
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}, {
				Name:     ptr.To("https"),
				Port:     ptr.To(int32(443)),
				Protocol: &protoUDP,
			}},
		}},
		expectedNumSlices:     2,
		expectedClientActions: 2,
		expectedMetrics:       &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 5, addedPerSync: 5, numCreated: 1, numUpdated: 1},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports and addresses, existing EndpointSlice with some addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "10.0.1.3",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ep-1",
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}, {
				Name:     ptr.To("https"),
				Port:     ptr.To(int32(443)),
				Protocol: &protoUDP,
			}},
			Endpoints: []discovery.Endpoint{{
				Addresses: []string{"10.0.0.2"},
				Hostname:  ptr.To("pod-2"),
			}, {
				Addresses: []string{"10.0.0.1", "10.0.0.3"},
				Hostname:  ptr.To("pod-1"),
			}},
		}},
		expectedNumSlices:     2,
		expectedClientActions: 2,
		expectedMetrics:       &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 5, addedPerSync: 4, updatedPerSync: 1, removedPerSync: 1, numCreated: 1, numUpdated: 1},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports and addresses, existing EndpointSlice identical to subset",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "10.0.1.3",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ep-1",
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}, {
				Name:     ptr.To("https"),
				Port:     ptr.To(int32(443)),
				Protocol: &protoUDP,
			}},
			Endpoints: []discovery.Endpoint{{
				Addresses:  []string{"10.0.0.1"},
				Hostname:   ptr.To("pod-1"),
				NodeName:   ptr.To("node-1"),
				Conditions: discovery.EndpointConditions{Ready: ptr.To(true)},
			}, {
				Addresses:  []string{"10.0.0.2"},
				Hostname:   ptr.To("pod-2"),
				NodeName:   ptr.To("node-2"),
				Conditions: discovery.EndpointConditions{Ready: ptr.To(true)},
			}},
		}},
		expectedNumSlices:     2,
		expectedClientActions: 1,
		expectedMetrics:       &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 5, addedPerSync: 3, numCreated: 1},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports, and dual stack addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "2001:db8:2222:3333:4444:5555:6666:7777",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "2001:db8:3333:4444:5555:6666:7777:8888",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      4,
		expectedClientActions:  4,
		expectedMetrics:        &expectedMetrics{desiredSlices: 4, actualSlices: 4, desiredEndpoints: 5, addedPerSync: 5, numCreated: 4},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports, ipv6 only addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "2001:db8:1111:3333:4444:5555:6666:7777",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "2001:db8:2222:3333:4444:5555:6666:7777",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "2001:db8:3333:3333:4444:5555:6666:7777",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "2001:db8:4444:3333:4444:5555:6666:7777",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "2001:db8:5555:3333:4444:5555:6666:7777",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      2,
		expectedClientActions:  2,
		expectedMetrics:        &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 5, addedPerSync: 5, numCreated: 2},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports, some invalid addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "2001:db8:1111:3333:4444:5555:6666:7777",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "this-is-not-an-ip",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "this-is-also-not-an-ip",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "2001:db8:4444:3333:4444:5555:6666:7777",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "2001:db8:5555:3333:4444:5555:6666:7777",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      2,
		expectedClientActions:  2,
		expectedMetrics:        &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 3, addedPerSync: 3, skippedPerSync: 2, numCreated: 2},
	}, {
		testName: "Endpoints with 2 subsets, multiple ports, all invalid addresses",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "this-is-not-an-ip1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "this-is-not-an-ip12",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "this-is-not-an-ip11",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "this-is-not-an-ip12",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "this-is-not-an-ip3",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      0,
		expectedClientActions:  0,
		expectedMetrics:        &expectedMetrics{desiredSlices: 0, actualSlices: 0, desiredEndpoints: 0, addedPerSync: 0, skippedPerSync: 5, numCreated: 0},
	}, {
		testName: "Endpoints with 2 subsets, 1 exceeding maxEndpointsPerSubset",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     443,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.0.2",
				Hostname: "pod-2",
				NodeName: ptr.To("node-2"),
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     3000,
				Protocol: corev1.ProtocolTCP,
			}, {
				Name:     "https",
				Port:     3001,
				Protocol: corev1.ProtocolUDP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.1.1",
				Hostname: "pod-11",
				NodeName: ptr.To("node-1"),
			}, {
				IP:       "10.0.1.2",
				Hostname: "pod-12",
				NodeName: ptr.To("node-2"),
			}, {
				IP:       "10.0.1.3",
				Hostname: "pod-13",
				NodeName: ptr.To("node-3"),
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      2,
		expectedClientActions:  2,
		maxEndpointsPerSubset:  2,
		expectedMetrics:        &expectedMetrics{desiredSlices: 2, actualSlices: 2, desiredEndpoints: 4, addedPerSync: 4, updatedPerSync: 0, removedPerSync: 0, skippedPerSync: 1, numCreated: 2, numUpdated: 0},
	}, {
		testName: "The last-applied-configuration annotation should not get mirrored to created or updated endpoint slices",
		epAnnotations: map[string]string{
			corev1.LastAppliedConfigAnnotation: "{\"apiVersion\":\"v1\",\"kind\":\"Endpoints\",\"subsets\":[]}",
		},
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{addedPerSync: 1, numCreated: 1, desiredEndpoints: 1, desiredSlices: 1, actualSlices: 1},
	}, {
		testName: "The last-applied-configuration annotation shouldn't get added to created endpoint slices",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{addedPerSync: 1, numCreated: 1, desiredEndpoints: 1, desiredSlices: 1, actualSlices: 1},
	}, {
		testName: "The last-applied-configuration shouldn't get mirrored to endpoint slices when it's value is empty",
		epAnnotations: map[string]string{
			corev1.LastAppliedConfigAnnotation: "",
		},
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{addedPerSync: 1, numCreated: 1, desiredEndpoints: 1, desiredSlices: 1, actualSlices: 1},
	}, {
		testName: "Annotations other than last-applied-configuration should get correctly mirrored",
		epAnnotations: map[string]string{
			corev1.LastAppliedConfigAnnotation: "{\"apiVersion\":\"v1\",\"kind\":\"Endpoints\",\"subsets\":[]}",
			"foo":                              "bar",
		},
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{},
		expectedNumSlices:      1,
		expectedClientActions:  1,
		expectedMetrics:        &expectedMetrics{addedPerSync: 1, numCreated: 1, desiredEndpoints: 1, desiredSlices: 1, actualSlices: 1},
	}, {
		testName: "Annotation mirroring should remove the last-applied-configuration annotation from existing endpoint slices",
		subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name:     "http",
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP:       "10.0.0.1",
				Hostname: "pod-1",
			}},
		}},
		existingEndpointSlices: []*discovery.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ep-1",
				Annotations: map[string]string{
					corev1.LastAppliedConfigAnnotation: "{\"apiVersion\":\"v1\",\"kind\":\"Endpoints\",\"subsets\":[]}",
				},
			},
			AddressType: discovery.AddressTypeIPv4,
			Ports: []discovery.EndpointPort{{
				Name:     ptr.To("http"),
				Port:     ptr.To(int32(80)),
				Protocol: &protoTCP,
			}},
			Endpoints: []discovery.Endpoint{{
				Addresses:  []string{"10.0.0.1"},
				Hostname:   ptr.To("pod-1"),
				Conditions: discovery.EndpointConditions{Ready: ptr.To(true)},
			}},
		}},
		expectedNumSlices:     1,
		expectedClientActions: 1,
	}}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			tCtx := ktesting.Init(t)
			client := newClientset()
			setupMetrics()
			namespace := "test"
			endpoints := corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ep", Namespace: namespace, Labels: tc.epLabels, Annotations: tc.epAnnotations},
				Subsets:    tc.subsets,
			}

			if tc.endpointsDeletionPending {
				now := metav1.Now()
				endpoints.DeletionTimestamp = &now
			}

			numInitialActions := 0
			for _, epSlice := range tc.existingEndpointSlices {
				epSlice.Labels = map[string]string{
					discovery.LabelServiceName: endpoints.Name,
					discovery.LabelManagedBy:   controllerName,
				}
				_, err := client.DiscoveryV1().EndpointSlices(namespace).Create(context.TODO(), epSlice, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Expected no error creating EndpointSlice, got %v", err)
				}
				numInitialActions++
			}

			maxEndpointsPerSubset := tc.maxEndpointsPerSubset
			if maxEndpointsPerSubset == 0 {
				maxEndpointsPerSubset = defaultMaxEndpointsPerSubset
			}
			r := newEndpointsReconciler(tCtx, client, maxEndpointsPerSubset)
			err := r.reconcileHelper(t, &endpoints, tc.existingEndpointSlices)
			if err != nil {
				t.Fatalf("Expected no error on reconcile, got %v", err)
			}

			numExtraActions := len(client.Actions()) - numInitialActions
			if numExtraActions != tc.expectedClientActions {
				t.Fatalf("Expected %d additional client actions, got %d: %#v", tc.expectedClientActions, numExtraActions, client.Actions()[numInitialActions:])
			}

			// if tc.expectedMetrics != nil {
			// 	expectMetrics(t, *tc.expectedMetrics)
			// }

			endpointSlices := fetchEndpointSlices(t, client, namespace)
			expectEndpointSlices(t, tc.expectedNumSlices, int(maxEndpointsPerSubset), endpoints, endpointSlices)
		})
	}
}

// Test Helpers

type endpointsReconcilerHelper struct {
	reconciler            *Reconciler
	maxEndpointsPerSubset int32
}

func newEndpointsReconciler(ctx context.Context, client *fake.Clientset, maxEndpointsPerSubset int32) *endpointsReconcilerHelper {
	broadcaster := record.NewBroadcaster(record.WithContext(ctx))
	recorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "endpoint-slice-mirroring-controller"})

	reconciler := NewReconciler(
		client,
		endpointsliceutil.NewEndpointSliceTracker(),
		recorder,
		controllerName,
		maxEndpointsPerSubset,
	)

	return &endpointsReconcilerHelper{
		reconciler:            reconciler,
		maxEndpointsPerSubset: maxEndpointsPerSubset,
	}
}

func expectEndpointSlices(t *testing.T, num, maxEndpointsPerSubset int, endpoints corev1.Endpoints, endpointSlices []discovery.EndpointSlice) {
	t.Helper()
	if len(endpointSlices) != num {
		t.Fatalf("Expected %d EndpointSlices, got %d", num, len(endpointSlices))
	}

	if num == 0 {
		return
	}

	for _, epSlice := range endpointSlices {
		if !strings.HasPrefix(epSlice.Name, endpoints.Name) {
			t.Errorf("Expected EndpointSlice name to start with %s, got %s", endpoints.Name, epSlice.Name)
		}

		serviceNameVal, ok := epSlice.Labels[discovery.LabelServiceName]
		if !ok {
			t.Errorf("Expected EndpointSlice to have %s label set", discovery.LabelServiceName)
		}
		if serviceNameVal != endpoints.Name {
			t.Errorf("Expected EndpointSlice to have %s label set to %s, got %s", discovery.LabelServiceName, endpoints.Name, serviceNameVal)
		}

		_, ok = epSlice.Annotations[corev1.LastAppliedConfigAnnotation]
		if ok {
			t.Errorf("Expected LastAppliedConfigAnnotation to be unset, got %s", epSlice.Annotations[corev1.LastAppliedConfigAnnotation])
		}

		_, ok = epSlice.Annotations[corev1.EndpointsLastChangeTriggerTime]
		if ok {
			t.Errorf("Expected EndpointsLastChangeTriggerTime to be unset, got %s", epSlice.Annotations[corev1.EndpointsLastChangeTriggerTime])
		}

		for annotation, val := range endpoints.Annotations {
			if annotation == corev1.EndpointsLastChangeTriggerTime || annotation == corev1.LastAppliedConfigAnnotation {
				continue
			}
			if epSlice.Annotations[annotation] != val {
				t.Errorf("Expected endpoint annotation %s to be mirrored correctly, got %s", annotation, epSlice.Annotations[annotation])
			}
		}
	}

	// canonicalize endpoints to match the expected endpoints, otherwise the test
	// that creates more endpoints than allowed fail becaused the list of final
	// endpoints doesn't match.
	for _, epSubset := range endpointsv1.RepackSubsets(endpoints.Subsets) {
		if len(epSubset.Addresses) == 0 && len(epSubset.NotReadyAddresses) == 0 {
			continue
		}

		var matchingEndpointsV4, matchingEndpointsV6 []discovery.Endpoint

		for _, epSlice := range endpointSlices {
			if portsMatch(epSubset.Ports, epSlice.Ports) {
				switch epSlice.AddressType {
				case discovery.AddressTypeIPv4:
					matchingEndpointsV4 = append(matchingEndpointsV4, epSlice.Endpoints...)
				case discovery.AddressTypeIPv6:
					matchingEndpointsV6 = append(matchingEndpointsV6, epSlice.Endpoints...)
				default:
					t.Fatalf("Unexpected EndpointSlice address type found: %v", epSlice.AddressType)
				}
			}
		}

		if len(matchingEndpointsV4) == 0 && len(matchingEndpointsV6) == 0 {
			t.Fatalf("No EndpointSlices match Endpoints subset: %#v", epSubset.Ports)
		}

		expectMatchingAddresses(t, epSubset, matchingEndpointsV4, discovery.AddressTypeIPv4, maxEndpointsPerSubset)
		expectMatchingAddresses(t, epSubset, matchingEndpointsV6, discovery.AddressTypeIPv6, maxEndpointsPerSubset)
	}
}

func portsMatch(epPorts []corev1.EndpointPort, epsPorts []discovery.EndpointPort) bool {
	if len(epPorts) != len(epsPorts) {
		return false
	}

	portsToBeMatched := map[int32]corev1.EndpointPort{}

	for _, epPort := range epPorts {
		portsToBeMatched[epPort.Port] = epPort
	}

	for _, epsPort := range epsPorts {
		epPort, ok := portsToBeMatched[*epsPort.Port]
		if !ok {
			return false
		}
		delete(portsToBeMatched, *epsPort.Port)

		if epPort.Name != *epsPort.Name {
			return false
		}
		if epPort.Port != *epsPort.Port {
			return false
		}
		if epPort.Protocol != *epsPort.Protocol {
			return false
		}
		if epPort.AppProtocol != epsPort.AppProtocol {
			return false
		}
	}

	return true
}

func expectMatchingAddresses(t *testing.T, epSubset corev1.EndpointSubset, esEndpoints []discovery.Endpoint, addrType discovery.AddressType, maxEndpointsPerSubset int) {
	t.Helper()
	type addressInfo struct {
		ready     bool
		epAddress corev1.EndpointAddress
	}

	// This approach assumes that each IP is unique within an EndpointSubset.
	expectedEndpoints := map[string]addressInfo{}

	for _, address := range epSubset.Addresses {
		at := getAddressType(address.IP)
		if at != nil && *at == addrType && len(expectedEndpoints) < maxEndpointsPerSubset {
			expectedEndpoints[address.IP] = addressInfo{
				ready:     true,
				epAddress: address,
			}
		}
	}

	for _, address := range epSubset.NotReadyAddresses {
		at := getAddressType(address.IP)
		if at != nil && *at == addrType && len(expectedEndpoints) < maxEndpointsPerSubset {
			expectedEndpoints[address.IP] = addressInfo{
				ready:     false,
				epAddress: address,
			}
		}
	}

	if len(expectedEndpoints) != len(esEndpoints) {
		t.Errorf("Expected %d endpoints, got %d", len(expectedEndpoints), len(esEndpoints))
	}

	for _, endpoint := range esEndpoints {
		if len(endpoint.Addresses) != 1 {
			t.Fatalf("Expected endpoint to have 1 address, got %d", len(endpoint.Addresses))
		}
		address := endpoint.Addresses[0]
		expectedEndpoint, ok := expectedEndpoints[address]

		if !ok {
			t.Fatalf("EndpointSlice has endpoint with unexpected address: %s", address)
		}

		if expectedEndpoint.ready != *endpoint.Conditions.Ready {
			t.Errorf("Expected ready to be %t, got %t", expectedEndpoint.ready, *endpoint.Conditions.Ready)
		}

		if endpoint.Hostname == nil {
			if expectedEndpoint.epAddress.Hostname != "" {
				t.Errorf("Expected hostname to be %s, got nil", expectedEndpoint.epAddress.Hostname)
			}
		} else if expectedEndpoint.epAddress.Hostname != *endpoint.Hostname {
			t.Errorf("Expected hostname to be %s, got %s", expectedEndpoint.epAddress.Hostname, *endpoint.Hostname)
		}

		if expectedEndpoint.epAddress.NodeName != nil {
			if endpoint.NodeName == nil {
				t.Errorf("Expected nodeName to be set")
			}
			if *expectedEndpoint.epAddress.NodeName != *endpoint.NodeName {
				t.Errorf("Expected nodeName to be %s, got %s", *expectedEndpoint.epAddress.NodeName, *endpoint.NodeName)
			}
		}
	}
}

// func fetchEndpointSlices(t *testing.T, client *fake.Clientset, namespace string) []discovery.EndpointSlice {
// 	t.Helper()
// 	fetchedSlices, err := client.DiscoveryV1().EndpointSlices(namespace).List(context.TODO(), metav1.ListOptions{
// 		LabelSelector: discovery.LabelManagedBy + "=" + controllerName,
// 	})
// 	if err != nil {
// 		t.Fatalf("Expected no error fetching Endpoint Slices, got: %v", err)
// 		return []discovery.EndpointSlice{}
// 	}
// 	return fetchedSlices.Items
// }

func (erh *endpointsReconcilerHelper) reconcileHelper(t *testing.T, endpoints *corev1.Endpoints, existingSlices []*discovery.EndpointSlice) error {
	t.Helper()
	logger, _ := ktesting.NewTestContext(t)

	labelsAnnotationsFromEndpoints := LabelsAnnotationsFromEndpoints{Endpoints: endpoints}
	desiredEndpoints, supportedAddressesTypes := FromEndpoints(endpoints, erh.maxEndpointsPerSubset)

	// err := erh.reconciler.Reconcile(logger, endpoints, existingSlices)
	// if err != nil {
	// 	t.Fatalf("Expected no error reconciling Endpoint Slices, got: %v", err)
	// }
	return erh.reconciler.Reconcile(
		logger,
		schema.GroupVersionKind{Version: "v1", Kind: "Endpoints"},
		endpoints,
		desiredEndpoints,
		existingSlices,
		supportedAddressesTypes,
		nil,
		labelsAnnotationsFromEndpoints.Set,
		time.Time{},
	)
}
