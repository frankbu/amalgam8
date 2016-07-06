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

package checker

import (
	"errors"
	"io"
	"time"

	"github.com/amalgam8/sidecar/config"
	"github.com/amalgam8/sidecar/router/clients"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tenant listener", func() {

	var (
		consumer    *MockConsumer
		rc          *clients.MockController
		n           *mockNginx
		c           *config.Config
		l           *listener
		regToken    string
		updateCount int
	)

	BeforeEach(func() {
		updateCount = 0

		regToken = "sd_token"

		consumer = &MockConsumer{
			ReceiveEventKey: regToken,
		}
		rc = &clients.MockController{}
		n = &mockNginx{
			UpdateFunc: func(reader io.Reader) error {
				updateCount++
				return nil
			},
		}
		c = &config.Config{
			Tenant: config.Tenant{
				Token:     "token",
				TTL:       60 * time.Second,
				Heartbeat: 30 * time.Second,
			},
			Registry: config.Registry{
				URL:   "http://registry",
				Token: regToken,
			},
			Kafka: config.Kafka{
				Brokers: []string{
					"http://broker1",
					"http://broker2",
					"http://broker3",
				},
				Username: "username",
				Password: "password",
			},
			Nginx: config.Nginx{
				Port:    6379,
				Logging: false,
			},
			Controller: config.Controller{
				URL:  "http://controller",
				Poll: 60 * time.Second,
			},
		}

		l = &listener{
			consumer:   consumer,
			controller: rc,
			nginx:      n,
			config:     c,
		}

	})

	It("listens for an update event successfully", func() {
		Expect(l.listenForUpdate()).ToNot(HaveOccurred())
		Expect(updateCount).To(Equal(1))
	})

	It("reports NGINX update failure", func() {
		n.UpdateFunc = func(reader io.Reader) error {
			return errors.New("Update NGINX failed")
		}

		Expect(l.listenForUpdate()).To(HaveOccurred())
	})

	It("does not update NGINX if unable to obtain config from Controller", func() {
		rc.ConfigError = errors.New("Get rules failed")

		Expect(l.listenForUpdate()).To(HaveOccurred())
		Expect(updateCount).To(Equal(0))
	})

})
