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
	"net"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	goca "github.com/OpenNebula/one/src/oca/go/src/goca"
	goca_dyn "github.com/OpenNebula/one/src/oca/go/src/goca/dynamic"
	goca_vn "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/virtualnetwork"
	goca_vr "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/virtualrouter"
)

type LoadBalancer struct {
	ctrl           *goca.Controller
	publicNetwork  *ONEVirtualNetwork
	privateNetwork *ONEVirtualNetwork
	virtualRouter  *ONEVirtualRouter
}

func NewLoadBalancer(cfg OpenNebulaConfig) (cloudprovider.LoadBalancer, error) {
	ctrl := goca.NewController(goca.NewDefaultClient(goca.OneConfig{
		Endpoint: cfg.Endpoint.ONE_XMLRPC,
		Token:    cfg.Endpoint.ONE_AUTH,
	}))
	if cfg.PublicNetwork == nil && cfg.PrivateNetwork == nil {
		return nil, fmt.Errorf("no networks defined")
	}
	return &LoadBalancer{
		ctrl:           ctrl,
		publicNetwork:  cfg.PublicNetwork,
		privateNetwork: cfg.PrivateNetwork,
		virtualRouter:  cfg.VirtualRouter,
	}, nil
}

func (lb *LoadBalancer) getLBReservationName(clusterName string) string {
	return fmt.Sprintf("%s-lb", clusterName)
}

func (lb *LoadBalancer) findLoadBalancer(ctx context.Context, clusterName, lbName string) (*goca_vn.VirtualNetwork, int, error) {
	vnID, err := lb.ctrl.VirtualNetworks().ByNameContext(ctx, lb.getLBReservationName(clusterName))
	if err != nil {
		if err.Error() == "resource not found" {
			return nil, -1, nil
		}
		return nil, -1, err
	}
	vn, err := lb.ctrl.VirtualNetwork(vnID).InfoContext(ctx, true)
	if err != nil {
		return nil, -1, err
	}

	arIdx := -1
	for i, ar := range vn.ARs {
		v, err := ar.Custom.GetStr("LB_NAME")
		if err != nil {
			continue
		}
		if v == lbName {
			arIdx = i
			break
		}
	}

	return vn, arIdx, nil
}

func (lb *LoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service) (*corev1.LoadBalancerStatus, bool, error) {
	klog.Infof("GetLoadBalancer(): %s", clusterName)

	vn, arIdx, err := lb.findLoadBalancer(ctx, clusterName, lb.GetLoadBalancerName(ctx, clusterName, service))
	if err != nil {
		return nil, false, err
	}
	if arIdx < 0 {
		return nil, false, nil
	}

	return &corev1.LoadBalancerStatus{
		Ingress: []corev1.LoadBalancerIngress{
			corev1.LoadBalancerIngress{IP: vn.ARs[arIdx].IP},
		},
	}, true, nil
}

func (lb *LoadBalancer) GetLoadBalancerName(_ context.Context, clusterName string, service *corev1.Service) string {
	return fmt.Sprintf("%s-%s-%s", clusterName, service.Namespace, service.Name)
}

func (lb *LoadBalancer) getVRReservationName(clusterName string) string {
	return fmt.Sprintf("%s-vr", clusterName)
}

func (lb *LoadBalancer) getPrimaryNetwork() *ONEVirtualNetwork {
	if lb.publicNetwork != nil {
		return lb.publicNetwork
	} else {
		return lb.privateNetwork
	}
}

func (lb *LoadBalancer) ensureVRReservationCreated(ctx context.Context, clusterName string) (*goca_vn.VirtualNetwork, error) {
	vnID, err := lb.ctrl.VirtualNetworks().ByNameContext(ctx, lb.getVRReservationName(clusterName))
	if err != nil && err.Error() != "resource not found" {
		return nil, err
	}
	if vnID < 0 {
		parentNetwork := lb.getPrimaryNetwork()
		parentID, err := lb.ctrl.VirtualNetworks().ByNameContext(ctx, parentNetwork.Name)
		if err != nil {
			return nil, err
		}

		replicas := 1
		if lb.virtualRouter.Replicas != nil {
			replicas = int(*lb.virtualRouter.Replicas)
		}
		reserve := &goca_dyn.Template{}
		reserve.AddPair("NAME", lb.getVRReservationName(clusterName))
		reserve.AddPair("SIZE", replicas)
		if parentNetwork.AddressRangeID != nil && *parentNetwork.AddressRangeID >= 0 {
			reserve.AddPair("AR_ID", *parentNetwork.AddressRangeID)
		} else {
			// NOTE: Expecting ETHER type AR at AR_ID=1.
			reserve.AddPair("AR_ID", 1)
		}
		vnID, err = lb.ctrl.VirtualNetwork(parentID).ReserveContext(ctx, reserve.String())
		if err != nil {
			return nil, err
		}
	}
	return lb.ctrl.VirtualNetwork(vnID).InfoContext(ctx, true)
}

func (lb *LoadBalancer) ensureLBReservationCreated(ctx context.Context, clusterName, lbName string) (*goca_vn.VirtualNetwork, int, error) {
	vn, arIdx, err := lb.findLoadBalancer(ctx, clusterName, lbName)
	if err != nil {
		return nil, -1, err
	}
	if arIdx < 0 { // not found
		parentNetwork := lb.getPrimaryNetwork()
		parentID, err := lb.ctrl.VirtualNetworks().ByNameContext(ctx, parentNetwork.Name)
		if err != nil {
			return nil, -1, err
		}

		template := &goca_dyn.Template{}
		template.AddPair("NAME", lb.getLBReservationName(clusterName))
		template.AddPair("SIZE", 1)
		if parentNetwork.AddressRangeID != nil && *parentNetwork.AddressRangeID >= 0 {
			template.AddPair("AR_ID", *parentNetwork.AddressRangeID)
		} else {
			// NOTE: Expecting non-ETHER type AR at AR_ID=0.
			template.AddPair("AR_ID", 0)
		}
		if vn != nil {
			template.AddPair("NETWORK_ID", vn.ID)
		}
		vnID, err := lb.ctrl.VirtualNetwork(parentID).ReserveContext(ctx, template.String())
		if err != nil {
			return nil, -1, err
		}
		vn, err = lb.ctrl.VirtualNetwork(vnID).InfoContext(ctx, true)
		if err != nil {
			return nil, -1, err
		}

		arIdx = len(vn.ARs) - 1
		if arIdx >= 0 {
			arVec := goca_dyn.NewVector("AR")
			arVec.AddPair("AR_ID", vn.ARs[arIdx].ID)
			arVec.AddPair("LB_NAME", lbName)

			if err := lb.ctrl.VirtualNetwork(vnID).UpdateARContext(ctx, arVec.String()); err != nil {
				return nil, -1, err
			}
		}

		vn, err = lb.ctrl.VirtualNetwork(vnID).InfoContext(ctx, true)
		if err != nil {
			return nil, -1, err
		}

		hold := goca_dyn.NewVector("LEASES")
		hold.AddPair("IP", vn.ARs[len(vn.ARs)-1].IP)
		if err := lb.ctrl.VirtualNetwork(vnID).HoldContext(ctx, hold.String()); err != nil {
			return nil, -1, err
		}
	}
	return vn, arIdx, nil
}

func (lb *LoadBalancer) getVirtualRouterName(clusterName string) string {
	return fmt.Sprintf("%s-lb", clusterName)
}

func (lb *LoadBalancer) ensureVirtualRouterCreated(ctx context.Context, clusterName string) (*goca_vr.VirtualRouter, error) {
	vrID, err := lb.ctrl.VirtualRouterByNameContext(ctx, lb.getVirtualRouterName(clusterName))
	if err != nil && err.Error() != "resource not found" {
		return nil, err
	}
	if vrID < 0 {
		vrTemplate := goca_vr.NewTemplate()
		vrTemplate.Add("NAME", lb.getVirtualRouterName(clusterName))
		// Overwrite NIC 0 or 0 and 1, leave others intact.
		nicIndex := -1
		if lb.publicNetwork != nil {
			nicIndex++
			nicVec := ensureNIC(vrTemplate, nicIndex)
			nicVec.AddPair("NETWORK", lb.getVRReservationName(clusterName))
		}
		if lb.privateNetwork != nil {
			nicIndex++
			nicVec := ensureNIC(vrTemplate, nicIndex)
			nicVec.AddPair("NETWORK", lb.privateNetwork.Name)
			nicVec.AddPair("FLOATING_IP", "YES")
			if lb.privateNetwork.FloatingIP != nil && net.ParseIP(*lb.privateNetwork.FloatingIP) != nil {
				nicVec.AddPair("IP", *lb.privateNetwork.FloatingIP)
			}
			if lb.privateNetwork.FloatingOnly == nil || !*lb.privateNetwork.FloatingOnly {
				nicVec.AddPair("FLOATING_ONLY", "NO")
			} else {
				nicVec.AddPair("FLOATING_ONLY", "YES")
			}
		}
		vrID, err = lb.ctrl.VirtualRouters().CreateContext(ctx, vrTemplate.String())
		if err != nil {
			return nil, err
		}
	}
	vr, err := lb.ctrl.VirtualRouter(vrID).InfoContext(ctx, true)
	if err != nil {
		return nil, err
	}

	replicas := 1
	if lb.virtualRouter.Replicas != nil {
		replicas = int(*lb.virtualRouter.Replicas)
	}
	if len(vr.VMs.ID) == 0 && replicas > 0 {
		vmTemplateID, err := lb.ctrl.Templates().ByNameContext(ctx, lb.virtualRouter.TemplateName)
		if err != nil {
			return nil, err
		}
		vmTemplate, err := lb.ctrl.Template(vmTemplateID).InfoContext(ctx, false, true)
		if err != nil {
			return nil, err
		}

		contextVec, err := vmTemplate.Template.GetVector("CONTEXT")
		if err != nil {
			return nil, err
		}
		contextVec.Del("ONEAPP_VNF_HAPROXY_ENABLED")
		contextVec.AddPair("ONEAPP_VNF_HAPROXY_ENABLED", "YES")
		if lb.virtualRouter.ExtraContext != nil {
			for k, v := range lb.virtualRouter.ExtraContext {
				contextVec.Del(k)
				contextVec.AddPair(k, v)
			}
		}
		if _, err := lb.ctrl.VirtualRouter(vrID).InstantiateContext(
			ctx,
			replicas,
			vmTemplateID,
			"",    // name
			false, // hold
			vmTemplate.Template.String(),
		); err != nil {
			return nil, err
		}
	}

	return lb.ctrl.VirtualRouter(vrID).InfoContext(ctx, true)
}

func (lb *LoadBalancer) reindexLoadBalancers(vn *goca_vn.VirtualNetwork, contextVec *goca_dyn.Vector, update []map[string]string) {
	byLB := map[string]map[string]string{}
	for _, p := range contextVec.Pairs {
		if strings.HasPrefix(p.Key(), "ONEAPP_VNF_HAPROXY_LB") {
			t := strings.Split(p.Key(), "_")
			if _, ok := byLB[t[3]]; !ok {
				byLB[t[3]] = map[string]string{}
			}
			byLB[t[3]][strings.Join(t[4:], "_")] = p.Value
		}
	}

	filter := map[string]struct{}{}
	for _, ar := range vn.ARs {
		filter[ar.IP] = struct{}{}
	}

	if len(update) > 0 {
		for _, v := range byLB {
			if v["IP"] == update[0]["IP"] {
				continue
			}
			if _, ok := filter[v["IP"]]; !ok {
				continue
			}
			update = append(update, v)
		}
	} else {
		for _, v := range byLB {
			if _, ok := filter[v["IP"]]; !ok {
				continue
			}
			update = append(update, v)
		}
	}

	// Delete everything.
	for i, s := 0, len(contextVec.Pairs); i < s; {
		k := contextVec.Pairs[i].Key()
		switch {
		case strings.HasPrefix(k, "ONEAPP_VROUTER_ETH0_VIP"), strings.HasPrefix(k, "ONEAPP_VNF_HAPROXY_LB"):
			contextVec.Pairs = append(contextVec.Pairs[:i], contextVec.Pairs[i+1:]...)
			s--
		default:
			i++
		}
	}

	// Reconstruct everything.
	for i, ar := range vn.ARs {
		contextVec.AddPair(fmt.Sprintf("ONEAPP_VROUTER_ETH0_VIP%d", i), ar.IP)
	}
	for i, v := range update {
		for x, y := range v {
			contextVec.AddPair(fmt.Sprintf("ONEAPP_VNF_HAPROXY_LB%d_%s", i, x), y)
		}
	}
}

func (lb *LoadBalancer) updateVirtualRouterInstances(ctx context.Context, vr *goca_vr.VirtualRouter, vn *goca_vn.VirtualNetwork, arIdx int, service *corev1.Service, nodes []*corev1.Node) error {
	for _, vmID := range vr.VMs.ID {
		vm, err := lb.ctrl.VM(vmID).InfoContext(ctx, true)
		if err != nil {
			return err
		}
		contextVec, err := vm.Template.GetVector("CONTEXT")
		if err != nil {
			return err
		}

		update := []map[string]string{}
		if nodes != nil {
			for _, port := range service.Spec.Ports {
				v := map[string]string{
					"IP":   vn.ARs[arIdx].IP,
					"PORT": fmt.Sprint(port.Port),
				}
				for i, node := range nodes {
					var nodeIP string
					for _, addr := range node.Status.Addresses {
						if addr.Type == corev1.NodeInternalIP {
							nodeIP = addr.Address
							break
						}
					}
					if net.ParseIP(nodeIP) != nil {
						v[fmt.Sprintf("SERVER%d_HOST", i)] = nodeIP
						v[fmt.Sprintf("SERVER%d_PORT", i)] = fmt.Sprint(port.NodePort)
					}
				}
				update = append(update, v)
			}
		}
		lb.reindexLoadBalancers(vn, contextVec, update)

		if err := lb.ctrl.VM(vmID).UpdateConfContext(ctx, vm.Template.String()); err != nil {
			return err
		}
	}

	return nil
}

func (lb *LoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	klog.Infof("EnsureLoadBalancer(): %s", clusterName)

	_, err := lb.ensureVRReservationCreated(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	vn, arIdx, err := lb.ensureLBReservationCreated(ctx, clusterName, lb.GetLoadBalancerName(ctx, clusterName, service))
	if err != nil {
		return nil, err
	}

	vr, err := lb.ensureVirtualRouterCreated(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	if err := lb.updateVirtualRouterInstances(ctx, vr, vn, arIdx, service, nodes); err != nil {
		return nil, err
	}

	return &corev1.LoadBalancerStatus{
		Ingress: []corev1.LoadBalancerIngress{
			corev1.LoadBalancerIngress{IP: vn.ARs[arIdx].IP},
		},
	}, nil
}

func (lb *LoadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	klog.Infof("UpdateLoadBalancer(): %s", clusterName)

	vrID, err := lb.ctrl.VirtualRouterByNameContext(ctx, lb.getVirtualRouterName(clusterName))
	if err != nil {
		return err
	}
	vr, err := lb.ctrl.VirtualRouter(vrID).InfoContext(ctx, true)
	if err != nil {
		return err
	}

	vn, arIdx, err := lb.findLoadBalancer(ctx, clusterName, lb.GetLoadBalancerName(ctx, clusterName, service))
	if err != nil {
		return err
	}
	if arIdx < 0 {
		return nil
	}

	if err := lb.updateVirtualRouterInstances(ctx, vr, vn, arIdx, service, nodes); err != nil {
		return err
	}

	return nil
}

func (lb *LoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *corev1.Service) error {
	klog.Infof("EnsureLoadBalancerDeleted(): %s", clusterName)

	vn, arIdx, err := lb.findLoadBalancer(ctx, clusterName, lb.GetLoadBalancerName(ctx, clusterName, service))
	if err != nil {
		return err
	}
	if arIdx < 0 {
		return nil
	}

	switch len(vn.ARs) {
	default:
		arID, err := strconv.Atoi(vn.ARs[arIdx].ID)
		if err != nil {
			return err
		}
		release := goca_dyn.NewVector("LEASES")
		release.AddPair("IP", vn.ARs[arIdx].IP)
		if err := lb.ctrl.VirtualNetwork(vn.ID).ReleaseContext(ctx, release.String()); err != nil {
			return err
		}
		if err := lb.ctrl.VirtualNetwork(vn.ID).RmARContext(ctx, arID); err != nil {
			return err
		}
		vn, err = lb.ctrl.VirtualNetwork(vn.ID).InfoContext(ctx, true)
		if err != nil {
			return err
		}

		vrID, err := lb.ctrl.VirtualRouterByNameContext(ctx, lb.getVirtualRouterName(clusterName))
		if err != nil {
			return err
		}
		vr, err := lb.ctrl.VirtualRouter(vrID).InfoContext(ctx, true)
		if err != nil {
			return err
		}
		if err := lb.updateVirtualRouterInstances(ctx, vr, vn, arIdx, service, nil); err != nil {
			return err
		}
	case 1: // Since this is the last item in the reservation then VR itself can be removed.
		vrID, err := lb.ctrl.VirtualRouterByNameContext(ctx, lb.getVirtualRouterName(clusterName))
		if err != nil && err.Error() != "resource not found" {
			return err
		}
		if vrID >= 0 {
			if err := lb.ctrl.VirtualRouter(vrID).DeleteContext(ctx); err != nil {
				return err
			}
		}

		if vnID, err := lb.ctrl.VirtualNetworks().ByNameContext(ctx, lb.getVRReservationName(clusterName)); err != nil {
			klog.Error(err)
		} else {
			if err = lb.ctrl.VirtualNetwork(vnID).DeleteContext(ctx); err != nil {
				return err
			}
		}

		// The LB-reservation VN *must* be deleted last.
		for _, ar := range vn.ARs {
			release := goca_dyn.NewVector("LEASES")
			release.AddPair("IP", ar.IP)
			if err := lb.ctrl.VirtualNetwork(vn.ID).ReleaseContext(ctx, release.String()); err != nil {
				return err
			}
		}
		if err := lb.ctrl.VirtualNetwork(vn.ID).DeleteContext(ctx); err != nil {
			return err
		}
	case 0: // Should never happen.
	}

	return nil
}
