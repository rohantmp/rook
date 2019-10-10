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
	"context"
	"fmt"

	"github.com/coreos/pkg/capnslog"

	"github.com/rook/rook/pkg/operator/ceph/cluster/osd"
	"github.com/rook/rook/pkg/operator/k8sutil"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	logger = capnslog.NewPackageLogger("github.com/rook/rook", controllerName)
	// Implement reconcile.Reconciler so the controller can reconcile objects
	_ reconcile.Reconciler = &ReconcileNode{}
)

// ReconcileNode reconciles ReplicaSets
type ReconcileNode struct {
	// client can be used to retrieve objects from the APIServer.
	scheme *runtime.Scheme
	client client.Client
}

// Reconcile reconciles a node and ensures that it has a drain-detection deployment
// attached to it.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNode) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// workaround because the rook logging mechanism is not compatible with the controller-runtime loggin interface
	result, err := r.reconcile(request)
	if err != nil {
		logger.Error(err)
	}
	return result, err
}

func (r *ReconcileNode) reconcile(request reconcile.Request) (reconcile.Result, error) {

	logger.Debugf("reconciling node: %s", request.Name)

	// get the node object
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: request.Name}}
	err := r.client.Get(context.TODO(), request.NamespacedName, node)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("Could not get node %s", request.NamespacedName)
	}

	cephPods := make([]corev1.Pod, 0)

	osdPodList := &corev1.PodList{}
	err = r.client.List(context.TODO(), osdPodList, client.MatchingLabels{k8sutil.AppAttr: osd.AppName})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not list the osd pods: %+v", err)
	}

	// TODO: get other ceph pods and append them to cephPods

	cephPods = append(cephPods, osdPodList.Items...)

	namespaceToPodList := make(map[string][]corev1.Pod)
	for _, cephPod := range cephPods {
		podNamespace := cephPod.GetNamespace()
		podList, ok := namespaceToPodList[podNamespace]
		if !ok {
			// initialize list
			namespaceToPodList[podNamespace] = []corev1.Pod{cephPod}
		} else {
			// append cephPod to namespace's pod list
			namespaceToPodList[podNamespace] = append(podList, cephPod)
		}
	}

	for namespace, cephPods := range namespaceToPodList {

		//get dataDirHostPath from the CephCluster
		cephClusters := &cephv1.CephClusterList{}
		err = r.client.List(context.TODO(), cephClusters, client.InNamespace(namespace))
		if len(cephClusters.Items) < 1 {
			return reconcile.Result{}, nil
		} else if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not get cephcluster in namespaces %s: %+v", namespace, err)
		}

		cephCluster := cephClusters.Items[0]
		if len(cephClusters.Items) > 0 {
			logger.Errorf("more than one CephCluster found in the namespace %s, choosing the first one %s", namespace, cephCluster.GetName())
		}

		// map with tolerations as keys and empty struct as values for uniqueness
		uniqueTolerations := make(map[corev1.Toleration]struct{})
		hasCephPods := false
		for _, cephPod := range cephPods {
			if cephPod.Spec.NodeName == request.Name {
				hasCephPods = true
				// get the osd tolerations
				for _, osdToleration := range cephPod.Spec.Tolerations {
					uniqueTolerations[osdToleration] = struct{}{}
				}
			}
		}

		if hasCephPods {
			tolerations := tolerationMapToList(uniqueTolerations)
			op, err := r.createOrUpdateCephCrash(*node, tolerations, cephCluster)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("Node reconcile failed on op: %s : %+v", op, err)
			}
			logger.Debugf("deployment successfully reconciled for node %s. operation: %s", request.Name, op)
		} else {
			// TODO optional: find and delete any ceph-crash deployments that were previously created here.
			logger.Debugf("not watching for drains on node %s as there are no osds running there.", request.Name)
		}
	}
	return reconcile.Result{}, nil

}

func tolerationMapToList(tolerationMap map[corev1.Toleration]struct{}) []corev1.Toleration {
	tolerationList := make([]corev1.Toleration, 0)
	for toleration := range tolerationMap {
		tolerationList = append(tolerationList, toleration)
	}
	return tolerationList
}
