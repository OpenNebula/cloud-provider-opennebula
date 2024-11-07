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
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"

	cloudprovider "k8s.io/cloud-provider"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	goca_vm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
)

type Instances struct {
	rpc2 *goca.Client
}

func NewInstances(cfg OpenNebulaConfig) (cloudprovider.InstancesV2, error) {
	auth := goca.OneConfig{Endpoint: cfg.ONE_XMLRPC, Token: cfg.ONE_AUTH}
	client := goca.NewDefaultClient(auth)
	return &Instances{rpc2: client}, nil
}

func (i *Instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	vm, err := i.byUUID(ctx, node.Status.NodeInfo.SystemUUID)
	if err != nil {
		return false, err
	}
	return vm != nil, nil
}

func (i *Instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	vm, err := i.byUUID(ctx, node.Status.NodeInfo.SystemUUID)
	if err != nil {
		return false, err
	}
	if vm == nil {
		return false, fmt.Errorf("Not found")
	}

	state, _, err := vm.State()
	if err != nil {
		return false, err
	}

	switch state {
	case goca_vm.Poweroff, goca_vm.Undeployed:
		return true, nil
	default:
		return false, nil
	}
}

func (i *Instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	vm, err := i.byUUID(ctx, node.Status.NodeInfo.SystemUUID)
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return nil, fmt.Errorf("Not found")
	}

	address4, err := vm.Template.GetStrFromVec("CONTEXT", "ETH0_IP")
	if err != nil {
		return nil, err
	}

	nodeAddresses := []v1.NodeAddress{
		v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: address4,
		},
		v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: address4,
		},
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    fmt.Sprintf("one://%d", vm.ID),
		NodeAddresses: nodeAddresses,
		InstanceType:  "",
		Zone:          "",
		Region:        "",
	}, nil
}

func (i *Instances) byUUID(ctx context.Context, vmUUID string) (*goca_vm.VM, error) {
	pool, err := goca.NewController(i.rpc2).VMs().InfoExtendedContext(ctx, -2)
	if err != nil {
		return nil, err
	}
	for _, vm := range pool.VMs {
		osUUID, err := vm.Template.GetStrFromVec("OS", "UUID")
		if err != nil {
			return nil, err
		}
		if vmUUID == osUUID {
			return &vm, nil
		}
	}
	return nil, nil
}
