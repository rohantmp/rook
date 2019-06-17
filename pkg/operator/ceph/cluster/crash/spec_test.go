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
	"testing"

	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	"github.com/rook/rook/pkg/clusterd"
	cephconfig "github.com/rook/rook/pkg/daemon/ceph/config"
	"github.com/rook/rook/pkg/operator/ceph/config"
	testop "github.com/rook/rook/pkg/operator/test"
	"github.com/stretchr/testify/assert"
)

func TestPodSpec(t *testing.T) {
	ns := "rook-ceph"
	c := New(
		&cephconfig.ClusterInfo{FSID: "myfsid"},
		&clusterd.Context{Clientset: testop.New(1)},
		ns,
		"rook/rook:myversion",
		cephv1.CephVersionSpec{Image: "ceph/ceph:myceph"},
		cephv1.NetworkSpec{},
		"/var/lib/rook/",
	)

	daemonConf := crashConfig{
		ResourceName: "rook-ceph-crash",
		DataPathMap:  config.NewDatalessDaemonDataPathMap("rook-ceph", "/var/lib/rook"),
	}

	d := c.makeCrashDaemonSet(ns, &daemonConf)
	assert.Equal(t, "rook-ceph-crash", d.Name)

	podSpec := d.Spec.Template.Spec
	assert.Equal(t, 2, len(podSpec.InitContainers), podSpec)
	assert.Equal(t, 1, len(podSpec.Containers), podSpec)
	assert.Equal(t, 4, len(podSpec.Volumes), podSpec)
	assert.Equal(t, "rook-config-override", podSpec.Volumes[0].Name)
	assert.Equal(t, "rook-ceph-mons-keyring", podSpec.Volumes[1].Name)
	assert.Equal(t, "rook-ceph-log", podSpec.Volumes[2].Name)
	assert.Equal(t, "rook-ceph-crash", podSpec.Volumes[3].Name)

	initCont := podSpec.InitContainers[0]
	assert.Equal(t, 0, len(initCont.VolumeMounts), initCont.VolumeMounts)

	initCont = podSpec.InitContainers[1]
	assert.Equal(t, 0, len(initCont.VolumeMounts), initCont.VolumeMounts)

	cont := podSpec.Containers[0]
	assert.Equal(t, 4, len(cont.VolumeMounts), cont.VolumeMounts)
	assert.Equal(t, "/etc/ceph", cont.VolumeMounts[0].MountPath)
	assert.Equal(t, "/etc/ceph/keyring-store/", cont.VolumeMounts[1].MountPath)
	assert.Equal(t, "/var/log/ceph", cont.VolumeMounts[2].MountPath)
	assert.Equal(t, "/var/lib/ceph/crash", cont.VolumeMounts[3].MountPath)

	assert.Equal(t, 11, len(cont.Env), cont.Env)
	assert.Equal(t, "CEPH_ARGS", cont.Env[len(cont.Env)-1].Name)
}
