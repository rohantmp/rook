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
	"fmt"
	"path"

	"github.com/rook/rook/pkg/operator/ceph/cluster/mon"
	"github.com/rook/rook/pkg/operator/ceph/config"
	opspec "github.com/rook/rook/pkg/operator/ceph/spec"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Use the same keyring as the mons
	keyringStoreName = "rook-ceph-mons"
)

func (c *Crash) startCrashDaemonset(crashConfig *crashConfig) error {
	ds := c.makeCrashDaemonSet(c.Namespace, crashConfig)

	logger.Debugf("starting ceph crash daemonset: %+v", ds)
	_, err := c.context.Clientset.AppsV1().DaemonSets(c.Namespace).Create(ds)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ceph crash daemonset %s: %+v", AppName, err)
		}
		logger.Infof("daemonset for ceph crash %s already exists. updating if needed", AppName)
		// There may be a *lot* of rgws, and they are stateless, so don't bother waiting until the
		// entire daemonset is updated to move on.
		// TODO: is the above statement safe to assume?
		// TODO: Are there any steps for RGW that need to happen before the daemons upgrade?
		_, err = c.context.Clientset.AppsV1().DaemonSets(c.Namespace).Update(ds)
		if err != nil {
			return fmt.Errorf("failed to update rgw daemonset %s. %+v", AppName, err)
		}
	}

	return nil
}

func (c *Crash) makeCrashDaemonSet(namespace string, crashConfig *crashConfig) *apps.DaemonSet {
	ds := &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: AppName,
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": AppName,
				},
			},
			UpdateStrategy: apps.DaemonSetUpdateStrategy{
				Type: apps.RollingUpdateDaemonSetStrategyType,
			},
			Template: c.makeCrashPodSpec(crashConfig),
		},
	}

	return ds
}

func (c *Crash) makeCrashPodSpec(crashConfig *crashConfig) v1.PodTemplateSpec {
	podSpec := v1.PodSpec{
		InitContainers: []v1.Container{
			c.makeCrashDirInitContainer(crashConfig),
			c.makeCrashChownInitContainer(crashConfig),
		},
		Containers: []v1.Container{
			c.makeCrashDaemonContainer(crashConfig),
		},
		RestartPolicy: v1.RestartPolicyAlways,
		HostNetwork:   c.Network.IsHost(),
		Volumes:       opspec.DaemonVolumesBase(crashConfig.DataPathMap, keyringStoreName),
	}

	podTemplateSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": AppName,
			},
		},
		Spec: podSpec,
	}

	return podTemplateSpec
}

func (c *Crash) makeCrashChownInitContainer(crashConfig *crashConfig) v1.Container {
	container := v1.Container{
		Name: "chown-container-crash-dir",
		Command: []string{
			"chown",
		},
		Args: []string{
			"--verbose",
			"--recursive",
			config.ChownUserGroup,
			crashConfig.DataPathMap.ContainerCrashDir,
		},
		Image:           c.cephVersion.Image,
		SecurityContext: mon.PodSecurityContext(),
	}
	return container
}

func (c *Crash) makeCrashDirInitContainer(crashConfig *crashConfig) v1.Container {
	crashPostedDir := path.Join(crashConfig.DataPathMap.ContainerCrashDir, "posted")

	container := v1.Container{
		Name: "make-container-crash-dir",
		Command: []string{
			"mkdir",
			"-p",
		},
		Args: []string{
			crashPostedDir,
		},
		Image:           c.cephVersion.Image,
		SecurityContext: mon.PodSecurityContext(),
	}
	return container
}

func (c *Crash) makeCrashDaemonContainer(crashConfig *crashConfig) v1.Container {

	envVars := append(opspec.DaemonEnvVars(c.cephVersion.Image), crashEnvVar())

	container := v1.Container{
		Name: "ceph-crash",
		Command: []string{
			"ceph-crash",
		},
		Image:        c.cephVersion.Image,
		Env:          envVars,
		VolumeMounts: opspec.DaemonVolumeMounts(crashConfig.DataPathMap, keyringStoreName),
	}

	return container
}

// crashEnvVar sets the CEPH_ARGS env variable so that any Ceph command can work
func crashEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: "CEPH_ARGS", Value: "-m $(ROOK_CEPH_MON_HOST) -k /etc/ceph/keyring-store/keyring -n client.admin"}
}
