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

	corev1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	goca "github.com/OpenNebula/one/src/oca/go/src/goca"
	goca_vm "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
)

type InstancesV2 struct {
	ctrl *goca.Controller
}

func NewInstancesV2(cfg OpenNebulaConfig) (cloudprovider.InstancesV2, error) {
	auth := goca.OneConfig{Endpoint: cfg.Endpoint.ONE_XMLRPC, Token: cfg.Endpoint.ONE_AUTH}
	ctrl := goca.NewController(goca.NewDefaultClient(auth))
	return &InstancesV2{ctrl}, nil
}

func (i2 *InstancesV2) InstanceExists(ctx context.Context, node *corev1.Node) (bool, error) {
	vm, err := i2.byUUID(ctx, node.Status.NodeInfo.SystemUUID)
	if err != nil {
		return false, err
	}
	return vm != nil, nil
}

func (i2 *InstancesV2) InstanceShutdown(ctx context.Context, node *corev1.Node) (bool, error) {
	vm, err := i2.byUUID(ctx, node.Status.NodeInfo.SystemUUID)
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

func (i2 *InstancesV2) InstanceMetadata(ctx context.Context, node *corev1.Node) (*cloudprovider.InstanceMetadata, error) {
	vm, err := i2.byUUID(ctx, node.Status.NodeInfo.SystemUUID)
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

	nodeAddresses := []corev1.NodeAddress{
		corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: address4,
		},
		corev1.NodeAddress{
			Type:    corev1.NodeExternalIP,
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

func (i2 *InstancesV2) byUUID(ctx context.Context, vmUUID string) (*goca_vm.VM, error) {
	pool, err := i2.ctrl.VMs().InfoExtendedContext(ctx, -2)
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
