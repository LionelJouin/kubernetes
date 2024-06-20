/*
Copyright 2016 The Kubernetes Authors.

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

// Package app implements a server that runs a set of active
// components.  This includes replication controllers, service endpoints and
// nodes.
package app

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	informerv1 "k8s.io/client-go/informers/core/v1"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	kubernetes "k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/controller-manager/controller"
	"k8s.io/kubernetes/cmd/kube-controller-manager/names"
	"k8s.io/kubernetes/pkg/controller/apis"
	endpointslicecontroller "k8s.io/kubernetes/pkg/controller/endpointslice"
	endpointslicemirroringcontroller "k8s.io/kubernetes/pkg/controller/endpointslicemirroring"
)

func newEndpointSliceControllerDescriptor() *ControllerDescriptor {
	return &ControllerDescriptor{
		name:     names.EndpointSliceController,
		aliases:  []string{"endpointslice"},
		initFunc: startEndpointSliceController,
	}
}

func startEndpointSliceController(ctx context.Context, controllerContext ControllerContext, controllerName string) (controller.Interface, bool, error) {
	svcInformer, err := newServiceInformerLabelledKubernetesController(controllerContext.InformerFactory)
	if err != nil {
		return nil, true, err
	}

	go endpointslicecontroller.NewController(
		ctx,
		controllerContext.InformerFactory.Core().V1().Pods(),
		svcInformer,
		controllerContext.InformerFactory.Core().V1().Nodes(),
		controllerContext.InformerFactory.Discovery().V1().EndpointSlices(),
		controllerContext.ComponentConfig.EndpointSliceController.MaxEndpointsPerSlice,
		controllerContext.ClientBuilder.ClientOrDie("endpointslice-controller"),
		controllerContext.ComponentConfig.EndpointSliceController.EndpointUpdatesBatchPeriod.Duration,
	).Run(ctx, int(controllerContext.ComponentConfig.EndpointSliceController.ConcurrentServiceEndpointSyncs))
	return nil, true, nil
}

func newEndpointSliceMirroringControllerDescriptor() *ControllerDescriptor {
	return &ControllerDescriptor{
		name:     names.EndpointSliceMirroringController,
		aliases:  []string{"endpointslicemirroring"},
		initFunc: startEndpointSliceMirroringController,
	}
}

func startEndpointSliceMirroringController(ctx context.Context, controllerContext ControllerContext, controllerName string) (controller.Interface, bool, error) {
	svcInformer, err := newServiceInformerLabelledKubernetesController(controllerContext.InformerFactory)
	if err != nil {
		return nil, true, err
	}

	go endpointslicemirroringcontroller.NewController(
		ctx,
		controllerContext.InformerFactory.Core().V1().Endpoints(),
		controllerContext.InformerFactory.Discovery().V1().EndpointSlices(),
		svcInformer,
		controllerContext.ComponentConfig.EndpointSliceMirroringController.MirroringMaxEndpointsPerSubset,
		controllerContext.ClientBuilder.ClientOrDie("endpointslicemirroring-controller"),
		controllerContext.ComponentConfig.EndpointSliceMirroringController.MirroringEndpointUpdatesBatchPeriod.Duration,
	).Run(ctx, int(controllerContext.ComponentConfig.EndpointSliceMirroringController.MirroringConcurrentServiceEndpointSyncs))
	return nil, true, nil
}

// newServiceInformerLabelledKubernetesController creates a new service informer to select only services that should
// be handle by Kubernetes. Services handled by Kubnernetes are not be labelled with service.kubernetes.io/endpointslice-controller-name.
func newServiceInformerLabelledKubernetesController(informerFactory informers.SharedInformerFactory) (informerv1.ServiceInformer, error) {
	noEndpointSliceName, err := labels.NewRequirement(apis.LabelServiceEndpointControllerName, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}

	labelSelector := labels.NewSelector()
	labelSelector = labelSelector.Add(*noEndpointSliceName)

	tweakListOptions := func(lo *metav1.ListOptions) {
		lo.LabelSelector = labelSelector.String()
	}

	return &serviceInformer{factory: informerFactory, namespace: corev1.NamespaceAll, tweakListOptions: tweakListOptions}, nil
}

type serviceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

func (f *serviceInformer) defaultInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return informerv1.NewFilteredServiceInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *serviceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corev1.Service{}, f.defaultInformer)
}

func (f *serviceInformer) Lister() v1.ServiceLister {
	return v1.NewServiceLister(f.Informer().GetIndexer())
}
