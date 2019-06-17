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

	"github.com/coreos/pkg/capnslog"
	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	rookalpha "github.com/rook/rook/pkg/apis/rook.io/v1alpha2"
	"github.com/rook/rook/pkg/clusterd"
	cephconfig "github.com/rook/rook/pkg/daemon/ceph/config"
	"github.com/rook/rook/pkg/operator/ceph/config"
	"github.com/rook/rook/pkg/operator/k8sutil"
)

var logger = capnslog.NewPackageLogger("github.com/rook/rook", "crash")

const (
	// AppName is the name of daemonset
	AppName = "rook-ceph-crash"
)

// Crash represents the Rook and environment configuration settings needed to set up ceph crash.
type Crash struct {
	ClusterInfo     *cephconfig.ClusterInfo
	Namespace       string
	placement       rookalpha.Placement
	annotations     rookalpha.Annotations
	context         *clusterd.Context
	cephVersion     cephv1.CephVersionSpec
	rookVersion     string
	Network         cephv1.NetworkSpec
	dataDirHostPath string
}

type crashConfig struct {
	ResourceName string              // the name rook gives to mgr resources in k8s metadata
	DataPathMap  *config.DataPathMap // location to store data in container
}

// New creates an instance of the crash daemon
func New(
	cluster *cephconfig.ClusterInfo,
	context *clusterd.Context,
	namespace, rookVersion string,
	cephVersion cephv1.CephVersionSpec,
	network cephv1.NetworkSpec,
	dataDirHostPath string,
) *Crash {
	return &Crash{
		ClusterInfo:     cluster,
		context:         context,
		Namespace:       namespace,
		rookVersion:     rookVersion,
		cephVersion:     cephVersion,
		Network:         network,
		dataDirHostPath: dataDirHostPath,
	}
}

var updateDeploymentAndWait = k8sutil.UpdateDeploymentAndWait

// Start begins the process of running crash daemon
func (c *Crash) Start() error {

	logger.Infof("running ceph-crash daemon")

	crashConfig := &crashConfig{
		ResourceName: AppName,
		DataPathMap:  config.NewDatalessDaemonDataPathMap(c.Namespace, c.dataDirHostPath),
	}

	err := c.startCrashDaemonset(crashConfig)
	if err != nil {
		return fmt.Errorf("failed running ceph crash: %+v", err)
	}

	return nil
}
