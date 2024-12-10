/*
Copyright 2024, OpenNebula Project, OpenNebula Systems.

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

package opennebula

import (
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/suite"

	cloudprovider "k8s.io/cloud-provider"
)

const (
	dotEnvPath     = "../../.env"
	defaultRetries = 8
)

type CPTestSuite struct {
	suite.Suite
	cfg OpenNebulaConfig
	lb  cloudprovider.LoadBalancer
}

func retryF(retries int, f func() bool) bool {
	for i := 0; i < retries; i++ {
		if f() {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func (s *CPTestSuite) SetupSuite() {
	if err := godotenv.Load(dotEnvPath); err != nil {
		s.T().Fatal("unable to load .env")
	}

	s.cfg = OpenNebulaConfig{}

	s.cfg.Endpoint = OpenNebulaEndpoint{
		ONE_XMLRPC: os.Getenv("ONE_XMLRPC"),
		ONE_AUTH:   os.Getenv("ONE_AUTH"),
	}
	if s.cfg.Endpoint.ONE_XMLRPC == "" {
		s.T().Fatal("Endpoint.ONE_XMLRPC must not be empty")
	}
	if s.cfg.Endpoint.ONE_AUTH == "" {
		s.T().Fatal("Endpoint.ONE_AUTH must not be empty")
	}

	replicas := int32(1)
	s.cfg.VirtualRouter = &ONEVirtualRouter{
		TemplateName: os.Getenv("ROUTER_TEMPLATE_NAME"),
		Replicas:     &replicas,
		ExtraContext: map[string]string{},
	}
	if s.cfg.VirtualRouter.TemplateName == "" {
		s.T().Fatal("VirtualRouter.TemplateName must not be empty")
	}

	s.cfg.PublicNetwork = &ONEVirtualNetwork{
		Name:           os.Getenv("PUBLIC_NETWORK_NAME"),
		AddressRangeID: nil,
		FloatingIP:     nil,
		FloatingOnly:   nil,
		Gateway:        nil,
		DNS:            nil,
	}
	if s.cfg.PublicNetwork.Name == "" {
		s.T().Fatal("PublicNetwork.Name must not be empty")
	}

	s.cfg.PrivateNetwork = &ONEVirtualNetwork{
		Name:           os.Getenv("PRIVATE_NETWORK_NAME"),
		AddressRangeID: nil,
		FloatingIP:     nil,
		FloatingOnly:   nil,
		Gateway:        nil,
		DNS:            nil,
	}
	if s.cfg.PrivateNetwork.Name == "" {
		s.T().Fatal("PrivateNetwork.Name must not be empty")
	}

	lb, err := NewLoadBalancer(s.cfg)
	if err != nil {
		s.T().Fatal("unable to create LoadBalancer")
	}
	s.lb = lb
}

func TestCPTestSuite(t *testing.T) {
	suite.Run(t, new(CPTestSuite))
}
