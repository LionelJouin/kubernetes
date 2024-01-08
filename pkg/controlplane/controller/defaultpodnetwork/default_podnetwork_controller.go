/*
Copyright 2023 The Kubernetes Authors.

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

package defaultpodnetwork

import (
	"context"
	"fmt"
	"time"

	networkingapiv1alpha1 "k8s.io/api/networking/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	networkingv1alpha1informers "k8s.io/client-go/informers/networking/v1alpha1"
	"k8s.io/client-go/kubernetes"
	networkingv1alpha1listers "k8s.io/client-go/listers/networking/v1alpha1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog/v2"
)

const (
	controllerName        = "kubernetes-default-podnetwork-controller"
	DefaultPodNetworkName = "kubernetes" // todo: to be moved to api
)

// Controller ensure default podnetwork exist.
type Controller struct {
	client kubernetes.Interface

	podNetworkInformer cache.SharedIndexInformer
	podNetworkLister   networkingv1alpha1listers.PodNetworkLister
	podNetworksSynced  cache.InformerSynced

	interval time.Duration
}

// NewController creates a new Controller to ensure default podnetwork exist.
func NewController(clientset kubernetes.Interface) *Controller {
	c := &Controller{
		client:   clientset,
		interval: 10 * time.Second,
	}

	// instead of using the shared informers from the controlplane instance, we construct our own informer
	// because we need such a small subset of the information available, only the kubernetes.default PodNetwork
	c.podNetworkInformer = networkingv1alpha1informers.NewFilteredPodNetworkInformer(clientset, 12*time.Hour,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", DefaultPodNetworkName).String()
		})
	c.podNetworkLister = networkingv1alpha1listers.NewPodNetworkLister(c.podNetworkInformer.GetIndexer())
	c.podNetworksSynced = c.podNetworkInformer.HasSynced

	return c
}

// Run starts one worker.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer klog.Infof("Shutting down %s", controllerName)

	klog.Infof("Starting %s", controllerName)

	go c.podNetworkInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.podNetworksSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	go wait.Until(c.sync, c.interval, stopCh)

	<-stopCh
}

func (c *Controller) sync() {
	if err := c.createPodNetworkIfNeeded(DefaultPodNetworkName); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to create required kubernetes default PodNetwork %s: %v", DefaultPodNetworkName, err))
	}
}

func (c *Controller) createPodNetworkIfNeeded(ns string) error {
	if _, err := c.podNetworkLister.Get(DefaultPodNetworkName); err == nil {
		// the pod network already exists
		return nil
	}
	newPodNetwork := &networkingapiv1alpha1.PodNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultPodNetworkName,
		},
		Spec: networkingapiv1alpha1.PodNetworkSpec{
			Enabled: true,
		},
	}
	_, err := c.client.NetworkingV1alpha1().PodNetworks().Create(context.TODO(), newPodNetwork, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		err = nil
	}
	return err
}
