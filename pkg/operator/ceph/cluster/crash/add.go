/*
Copyright 2019 The Rook Authors. All rights reserved.

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

package crash

import (
	"reflect"

	"github.com/rook/rook/pkg/operator/k8sutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "ceph-crash-controller"
	// AppName is the value to the "app" label for the ceph-crash pods
	AppName       = "rook-ceph-crash"
	NodeNameLabel = "node_name"
)

// Add adds a new Controller based on nodedrain.ReconcileNode and registers the relevant watches and handlers
func Add(mgr manager.Manager) error {
	reconcileNode := &ReconcileNode{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
	reconciler := reconcile.Reconciler(reconcileNode)
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return err
	}

	// Watch for changes to the nodes
	specChangePredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			nodeOld := e.ObjectOld.DeepCopyObject().(*corev1.Node)
			nodeNew := e.ObjectNew.DeepCopyObject().(*corev1.Node)
			return !reflect.DeepEqual(nodeOld.Spec, nodeNew.Spec)
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{}, specChangePredicate)
	if err != nil {
		return err
	}
	// Watch for changes to the ceph-crash deployments
	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
				deployment, ok := obj.Object.(*appsv1.Deployment)
				if !ok {
					return []reconcile.Request{}
				}
				labels := deployment.GetLabels()
				if appName, ok := labels[k8sutil.AppAttr]; !ok || appName != AppName {
					return []reconcile.Request{}
				}
				nodeName, ok := deployment.Spec.Template.ObjectMeta.Labels[NodeNameLabel]
				if !ok {
					return []reconcile.Request{}
				}
				req := reconcile.Request{NamespacedName: types.NamespacedName{Name: nodeName}}
				return []reconcile.Request{req}
			}),
		},
	)
	if err != nil {
		return err
	}

	// Watch for changes to the ceph pod nodename and enqueue thier nodes
	err = c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
				pod, ok := obj.Object.(*corev1.Pod)
				if !ok {
					return []reconcile.Request{}
				}
				nodeName := pod.Spec.NodeName
				if len(nodeName) < 1 {
					return []reconcile.Request{}
				}
				if isCephPod(pod) {
					req := reconcile.Request{NamespacedName: types.NamespacedName{Name: nodeName}}
					return []reconcile.Request{req}
				}
				return []reconcile.Request{}
			}),
		},
		// only enqueue the update event if the pod moved nodes
		predicate.Funcs{
			UpdateFunc: func(event event.UpdateEvent) bool {
				oldPod, ok := event.ObjectOld.(*corev1.Pod)
				if !ok {
					return false
				}
				newPod, ok := event.ObjectNew.(*corev1.Pod)
				if !ok {
					return false
				}
				// only enqueue if the nodename has changed
				if oldPod.Spec.NodeName == newPod.Spec.NodeName {
					return false
				}
				return true
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func isCephPod(pod *corev1.Pod) bool {
	//TODO: identify all kinds of ceph pods
	return false
}
