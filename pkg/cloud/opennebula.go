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
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	ProviderName string = "opennebula"
)

type OpenNebula struct {
	instancesV2 cloudprovider.InstancesV2
}

type Config struct {
	OpenNebula OpenNebulaConfig `yaml:"opennebula"`
}

type OpenNebulaConfig struct {
	ONE_XMLRPC string `yaml:"ONE_XMLRPC"`
	ONE_AUTH   string `yaml:"ONE_AUTH"`
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(reader io.Reader) (cloudprovider.Interface, error) {
		cfg, err := ReadConfig(reader)
		if err != nil {
			return nil, err
		}
		return NewOpenNebula(cfg)
	})
}

func NewOpenNebula(cfg *Config) (cloudprovider.Interface, error) {
	instances, err := NewInstances(cfg.OpenNebula)
	if err != nil {
		return nil, err
	}
	return &OpenNebula{instancesV2: instances}, nil
}

func ReadConfig(reader io.Reader) (*Config, error) {
	if reader == nil {
		return nil, fmt.Errorf("Reader is nil")
	}
	cfg := &Config{}
	if err := yaml.NewDecoder(reader).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (one *OpenNebula) Initialize(builder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (one *OpenNebula) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (one *OpenNebula) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (one *OpenNebula) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return one.instancesV2, true
}

func (one *OpenNebula) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (one *OpenNebula) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (one *OpenNebula) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (one *OpenNebula) ProviderName() string {
	return ProviderName
}

func (one *OpenNebula) HasClusterID() bool {
	return true
}
