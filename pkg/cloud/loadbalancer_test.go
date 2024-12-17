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

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	goca "github.com/OpenNebula/one/src/oca/go/src/goca"
)

type lbStep struct {
	destroy  bool
	services []*corev1.Service
	nodes    []*corev1.Node
	context  map[string]string
}

// Create a Service with a single Port.
var lbSinglePort = []lbStep{
	lbStep{
		destroy: false,
		services: []*corev1.Service{
			&corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Service0",
				},
				Spec: corev1.ServiceSpec{
					Type: "LoadBalancer",
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Name:     "http",
							Protocol: "TCP",
							Port:     80,
							NodePort: 30000,
						},
					},
				},
				Status: corev1.ServiceStatus{},
			},
		},
		nodes: []*corev1.Node{
			&corev1.Node{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Node",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Node0",
				},
				Spec: corev1.NodeSpec{},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						corev1.NodeAddress{
							Type:    corev1.NodeInternalIP,
							Address: "172.20.0.102",
						},
					},
				},
			},
		},
		context: map[string]string{
			"ONEAPP_VNF_HAPROXY_LB0_PORT":         "80",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_PORT": "30000",
		},
	},
}

// Ignore service with defined LB class.
var lbMismatchedClass = []lbStep{
	lbSinglePort[0],
	lbStep{
		destroy: false,
		services: []*corev1.Service{
			&corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Service0",
				},
				Spec: corev1.ServiceSpec{
					Type:              "LoadBalancer",
					LoadBalancerClass: &[]string{"asd"}[0], // other than default (nil)
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Name:     "http",
							Protocol: "TCP",
							Port:     8080,
							NodePort: 30001,
						},
					},
				},
				Status: corev1.ServiceStatus{},
			},
		},
		nodes:   lbSinglePort[0].nodes,
		context: lbSinglePort[0].context,
	},
}

// Create a Service, then modify an existing Port.
var lbModifyPort = []lbStep{
	lbSinglePort[0],
	lbStep{
		destroy: false,
		services: []*corev1.Service{
			&corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Service0",
				},
				Spec: corev1.ServiceSpec{
					Type: "LoadBalancer",
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Name:     "https",
							Protocol: "TCP",
							Port:     443,
							NodePort: 30000,
						},
					},
				},
				Status: corev1.ServiceStatus{},
			},
		},
		nodes: lbSinglePort[0].nodes,
		context: map[string]string{
			"ONEAPP_VNF_HAPROXY_LB0_PORT":         "443",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_PORT": "30000",
		},
	},
}

// Create a Service, then add an extra Port.
var lbAddPort = []lbStep{
	lbSinglePort[0],
	lbStep{
		destroy: false,
		services: []*corev1.Service{
			&corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Service0",
				},
				Spec: corev1.ServiceSpec{
					Type: "LoadBalancer",
					Ports: []corev1.ServicePort{
						lbSinglePort[0].services[0].Spec.Ports[0],
						corev1.ServicePort{
							Name:     "test",
							Protocol: "TCP",
							Port:     8686,
							NodePort: 30001,
						},
					},
				},
				Status: corev1.ServiceStatus{},
			},
		},
		nodes: lbSinglePort[0].nodes,
		context: map[string]string{
			"ONEAPP_VNF_HAPROXY_LB0_PORT":         "80",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_PORT": "30000",
			"ONEAPP_VNF_HAPROXY_LB1_PORT":         "8686",
			"ONEAPP_VNF_HAPROXY_LB1_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB1_SERVER0_PORT": "30001",
		},
	},
}

// Create a Service, then add an extra Node.
var lbAddNode = []lbStep{
	lbSinglePort[0],
	lbStep{
		destroy:  false,
		services: lbSinglePort[0].services,
		nodes: []*corev1.Node{
			lbSinglePort[0].nodes[0],
			&corev1.Node{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Node",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Node1",
				},
				Spec: corev1.NodeSpec{},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						corev1.NodeAddress{
							Type:    corev1.NodeInternalIP,
							Address: "172.20.0.103",
						},
					},
				},
			},
		},
		context: map[string]string{
			"ONEAPP_VNF_HAPROXY_LB0_PORT":         "80",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_PORT": "30000",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER1_HOST": "172.20.0.103",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER1_PORT": "30000",
		},
	},
}

// Create two Services.
var lbTwoServices = []lbStep{
	lbStep{
		destroy: false,
		services: []*corev1.Service{
			lbSinglePort[0].services[0],
			&corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "Service1",
				},
				Spec: corev1.ServiceSpec{
					Type: "LoadBalancer",
					Ports: []corev1.ServicePort{
						corev1.ServicePort{
							Name:     "https",
							Protocol: "TCP",
							Port:     443,
							NodePort: 30001,
						},
					},
				},
				Status: corev1.ServiceStatus{},
			},
		},
		nodes: lbSinglePort[0].nodes,
		context: map[string]string{
			"ONEAPP_VNF_HAPROXY_LB0_PORT":         "443",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_PORT": "30001",
			"ONEAPP_VNF_HAPROXY_LB1_PORT":         "80",
			"ONEAPP_VNF_HAPROXY_LB1_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB1_SERVER0_PORT": "30000",
		},
	},
}

// Delete second Service.
var lbDeleteSecondService = []lbStep{
	lbTwoServices[0],
	lbStep{
		destroy: true,
		services: []*corev1.Service{
			lbTwoServices[0].services[1],
		},
		nodes: []*corev1.Node{},
		context: map[string]string{
			"ONEAPP_VNF_HAPROXY_LB0_PORT":         "80",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_HOST": "172.20.0.102",
			"ONEAPP_VNF_HAPROXY_LB0_SERVER0_PORT": "30000",
		},
	},
}

func (s *CPTestSuite) TestLBSinglePort() {
	s.testLB("lbSinglePort", lbSinglePort)
}

func (s *CPTestSuite) TestLBMismatchedClass() {
	s.testLB("lbMismatchedClass", lbMismatchedClass)
}

func (s *CPTestSuite) TestLBModifyPort() {
	s.testLB("lbModifyPort", lbModifyPort)
}

func (s *CPTestSuite) TestLBAddPort() {
	s.testLB("lbAddPort", lbAddPort)
}

func (s *CPTestSuite) TestLBAddNode() {
	s.testLB("lbAddNode", lbAddNode)
}

func (s *CPTestSuite) TestLBTwoServices() {
	s.testLB("lbTwoServices", lbTwoServices)
}

func (s *CPTestSuite) TestLBDeleteSecondService() {
	s.testLB("lbDeleteSecondService", lbDeleteSecondService)
}

func (s *CPTestSuite) testLB(name string, steps []lbStep) {
	for _, step := range steps {
		for _, service := range step.services {
			if !step.destroy {
				retryF(defaultRetries, func() bool {
					_, err := s.lb.EnsureLoadBalancer(context.TODO(), name, service, step.nodes)
					if err != nil {
						s.T().Log(err)
					}
					return err == nil || err.Error() == "lb class unexpected"
				})
			} else {
				retryF(defaultRetries, func() bool {
					err := s.lb.EnsureLoadBalancerDeleted(context.TODO(), name, service)
					if err != nil {
						s.T().Log(err)
					}
					return err == nil || err.Error() == "lb class unexpected"
				})
			}
		}
		assert.Nil(s.T(), s.verifyVRContextVec(name, step.context))
	}
	for _, step := range steps {
		for _, service := range step.services {
			retryF(defaultRetries, func() bool {
				err := s.lb.EnsureLoadBalancerDeleted(context.TODO(), name, service)
				if err != nil {
					s.T().Log(err)
				}
				return err == nil || err.Error() == "lb class unexpected"
			})
		}
	}
	assert.NotNil(s.T(), s.verifyVRContextVec(name, steps[len(steps)-1].context))
}

func (s *CPTestSuite) verifyVRContextVec(clusterName string, contextMap map[string]string) error {
	ctrl := goca.NewController(goca.NewDefaultClient(goca.OneConfig{
		Endpoint: s.cfg.Endpoint.ONE_XMLRPC,
		Token:    s.cfg.Endpoint.ONE_AUTH,
	}))
	vrID, err := ctrl.VirtualRouterByName(fmt.Sprintf("%s-lb", clusterName))
	if err != nil {
		return err
	}
	vr, err := ctrl.VirtualRouter(vrID).Info(true)
	if err != nil {
		return err
	}
	for _, vmID := range vr.VMs.ID {
		vm, err := ctrl.VM(vmID).Info(true)
		if err != nil {
			return err
		}
		contextVec, err := vm.Template.GetVector("CONTEXT")
		if err != nil {
			return err
		}
		for k, v := range contextMap {
			y, err := contextVec.Pairs.GetStr(k)
			if err != nil {
				return err
			}
			if y != v {
				return fmt.Errorf("%s value mismatch", k)
			}
		}
	}
	return nil
}
