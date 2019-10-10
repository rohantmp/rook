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
	"path"

	"github.com/rook/rook/pkg/operator/ceph/cluster/mon"
	"github.com/rook/rook/pkg/operator/ceph/config"
	opspec "github.com/rook/rook/pkg/operator/ceph/spec"
	"github.com/rook/rook/pkg/operator/k8sutil"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createOrUpdateCephCrash is a wrapper around controllerutil.CreateOrUpdate
func (r *ReconcileNode) createOrUpdateCephCrash(
	node corev1.Node,
	tolerations []corev1.Toleration,
	cephCluster cephv1.CephCluster,
) (controllerutil.OperationResult, error) {
	// Create or Update the deployment default/foo
	nodeHostnameLabel, ok := node.ObjectMeta.Labels[corev1.LabelHostname]
	if !ok {
		return controllerutil.OperationResultNone, fmt.Errorf("Label key %s does not exist on node %s", corev1.LabelHostname, node.GetName())
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sutil.TruncateNodeName(fmt.Sprintf("%s-%%s", AppName), nodeHostnameLabel),
			Namespace: cephCluster.GetNamespace(),
			// Namespace: r.context.OperatorNamespace,
		},
	}

	mutateFunc := func() error {

		// lablels for the pod, the deployment, and the deploymentSelector
		deploymentLabels := map[string]string{
			corev1.LabelHostname: nodeHostnameLabel,
			k8sutil.AppAttr:      AppName,
			NodeNameLabel:        node.GetName(),
		}

		nodeSelector := map[string]string{corev1.LabelHostname: nodeHostnameLabel}

		// Deployment selector is immutable so we set this value only if
		// a new object is going to be created
		if deploy.ObjectMeta.CreationTimestamp.IsZero() {
			deploy.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: deploymentLabels,
			}
		}

		deploy.ObjectMeta.Labels = deploymentLabels
		deploy.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: deploymentLabels},
			Spec: corev1.PodSpec{
				NodeSelector: nodeSelector,
				InitContainers: []corev1.Container{
					getCrashDirInitContainer(cephCluster),
					getCrashChownInitContainer(cephCluster),
				},
				Containers: []corev1.Container{
					getCrashDaemonContainer(cephCluster),
				},
				Tolerations:   tolerations,
				RestartPolicy: corev1.RestartPolicyAlways,
				HostNetwork:   cephCluster.Spec.Network.IsHost(),
			},
		}

		return nil
	}

	return controllerutil.CreateOrUpdate(context.TODO(), r.client, deploy, mutateFunc)
}

func getCrashDirInitContainer(cephCluster cephv1.CephCluster) corev1.Container {
	dataPathMap := config.NewDatalessDaemonDataPathMap(cephCluster.GetNamespace(), cephCluster.Spec.DataDirHostPath)
	crashPostedDir := path.Join(dataPathMap.ContainerCrashDir, "posted")

	container := corev1.Container{
		Name: "make-container-crash-dir",
		Command: []string{
			"mkdir",
			"-p",
		},
		Args: []string{
			crashPostedDir,
		},
		Image:           cephCluster.Spec.CephVersion.Image,
		SecurityContext: mon.PodSecurityContext(),
	}
	return container
}

func getCrashChownInitContainer(cephCluster cephv1.CephCluster) corev1.Container {
	dataPathMap := config.NewDatalessDaemonDataPathMap(cephCluster.GetNamespace(), cephCluster.Spec.DataDirHostPath)
	container := corev1.Container{
		Name: "chown-container-crash-dir",
		Command: []string{
			"chown",
		},
		Args: []string{
			"--verbose",
			"--recursive",
			config.ChownUserGroup,
			dataPathMap.ContainerCrashDir,
		},
		Image:           cephCluster.Spec.CephVersion.Image,
		SecurityContext: mon.PodSecurityContext(),
	}
	return container
}

func getCrashDaemonContainer(cephCluster cephv1.CephCluster) corev1.Container {
	cephImage := cephCluster.Spec.CephVersion.Image
	dataPathMap := config.NewDatalessDaemonDataPathMap(cephCluster.GetNamespace(), cephCluster.Spec.DataDirHostPath)
	crashEnvVar := corev1.EnvVar{Name: "CEPH_ARGS", Value: "-m $(ROOK_CEPH_MON_HOST) -k /etc/ceph/keyring-store/keyring -n client.admin"}
	envVars := append(opspec.DaemonEnvVars(cephImage), crashEnvVar)

	container := corev1.Container{
		Name: "ceph-crash",
		Command: []string{
			"ceph-crash",
		},
		Image:        cephImage,
		Env:          envVars,
		VolumeMounts: opspec.DaemonVolumeMounts(dataPathMap, mon.KeyringStoreName),
	}

	return container
}
