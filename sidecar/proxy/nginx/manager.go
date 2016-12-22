// Copyright 2016 IBM Corporation
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package nginx

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/amalgam8/amalgam8/pkg/api"
)

// Manager of updates to NGINX
type Manager interface {
	// Update NGINX with the provided configuration
	Update([]api.ServiceInstance, []api.Rule) error
}

type manager struct {
	service     Service
	serviceName string
	mutex       sync.Mutex
	client      Client
}

// ManagerConfig options
type ManagerConfig struct {
	Client Client
}

// NewManager creates new a instance
func NewManager(conf ManagerConfig) Manager {
	return &manager{
		client: conf.Client,
	}
}

// Update NGINX
func (n *manager) Update(instances []api.ServiceInstance, rules []api.Rule) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if err := n.client.Update(instances, rules); err != nil {
		logrus.WithError(err).Error("Failed to update HTTP upstreams with NGINX")
		return err
	}

	return nil
}
