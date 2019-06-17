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
	testop "github.com/rook/rook/pkg/operator/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCrash(t *testing.T) {
	clientset := testop.New(1)

	c := New(
		&cephconfig.ClusterInfo{FSID: "myfsid"},
		&clusterd.Context{Clientset: clientset},
		"ns",
		"rook/rook:myversion",
		cephv1.CephVersionSpec{Image: "ceph/ceph:myceph"},
		cephv1.NetworkSpec{},
		"/var/lib/rook/",
	)

	err := c.Start()
	assert.NoError(t, err)

	opts := metav1.ListOptions{}
	d, err := clientset.AppsV1().DaemonSets(c.Namespace).List(opts)
	assert.Equal(t, 1, len(d.Items))

	crashName := "rook-ceph-crash"
	r, err := clientset.AppsV1().DaemonSets(c.Namespace).Get(crashName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, crashName, r.Name)
}
