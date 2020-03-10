/*
Copyright The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	debugerv1 "github.com/zhfg/k8s-crd-debug/pkg/apis/debuger/v1"
	versioned "github.com/zhfg/k8s-crd-debug/pkg/client/clientset/versioned"
	internalinterfaces "github.com/zhfg/k8s-crd-debug/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/zhfg/k8s-crd-debug/pkg/client/listers/debuger/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// DebugerTypeInformer provides access to a shared informer and lister for
// DebugerTypes.
type DebugerTypeInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.DebugerTypeLister
}

type debugerTypeInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewDebugerTypeInformer constructs a new informer for DebugerType type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewDebugerTypeInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredDebugerTypeInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredDebugerTypeInformer constructs a new informer for DebugerType type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredDebugerTypeInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.DebugerV1().DebugerTypes(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.DebugerV1().DebugerTypes(namespace).Watch(context.TODO(), options)
			},
		},
		&debugerv1.DebugerType{},
		resyncPeriod,
		indexers,
	)
}

func (f *debugerTypeInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredDebugerTypeInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *debugerTypeInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&debugerv1.DebugerType{}, f.defaultInformer)
}

func (f *debugerTypeInformer) Lister() v1.DebugerTypeLister {
	return v1.NewDebugerTypeLister(f.Informer().GetIndexer())
}
