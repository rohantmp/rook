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

package operator

import (
	"github.com/rook/rook/pkg/operator/ceph/cluster"
	"github.com/rook/rook/pkg/operator/ceph/disruption"
	"github.com/rook/rook/pkg/operator/ceph/disruption/controllerconfig"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func (o *Operator) startManager(stopCh <-chan struct{}) {

	// Set up a manager
	mgrOpts := manager.Options{
		LeaderElection: false,
	}

	logger.Info("setting up the controller-runtime manager")
	mgr, err := manager.New(o.context.KubeConfig, mgrOpts)
	if err != nil {
		logger.Errorf("unable to set up overall controller-runtime manager: %+v", err)
		return
	}

	// Add the registered controllers to the manager (entrypoint for controllers)
	err = cluster.AddToManager(mgr)
	if err != nil {
		logger.Errorf("Can't add controllers to controller-runtime manager: %+v", err)
	}
	// options to pass to the controllers
	disruptionOpts := &controllerconfig.Context{
		ClusterdContext:   o.context,
		OperatorNamespace: o.operatorNamespace,
		ReconcileCanaries: &controllerconfig.LockingBool{},
	}
	err = disruption.AddToManager(mgr, disruptionOpts)
	if err != nil {
		logger.Errorf("Can't add controllers to controller-runtime manager: %+v", err)
	}

	logger.Info("starting the controller-runtime manager")
	if err := mgr.Start(stopCh); err != nil {
		logger.Errorf("unable to run the controller-runtime manager: %+v", err)
		return
	}
}
