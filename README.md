[//]: # ( vim: set wrap : )

# OpenNebula Kubernetes Cloud Provider

This repo is an implementation of [Kubernetes Cloud Provider Interface](https://github.com/kubernetes/kubernetes/tree/master/staging/src/k8s.io/cloud-provider) for OpenNebula environments.

The main reason for its existence is to support the [OpenNebula Cluster-API Provider](https://github.com/OpenNebula/cluster-api-provider-opennebula), but it also can be successfully deployed in [OneKE](https://github.com/OpenNebula/one-apps/wiki/oneke_quick) clusters.

Current implementation includes [Kubernetes Node Metadata](https://github.com/kubernetes/kubernetes/blob/3e431adb036999dc2c2aacc465a50698458d6e5a/staging/src/k8s.io/cloud-provider/cloud.go#L207-L223) management and [LoadBalancer Service](https://github.com/kubernetes/kubernetes/blob/3e431adb036999dc2c2aacc465a50698458d6e5a/staging/src/k8s.io/cloud-provider/cloud.go#L122-L173) support, cloud storage support is planned for future releases.

## Documentation

* [Wiki Pages](https://github.com/OpenNebula/cloud-provider-opennebula/wiki)

## Contributing

* [Development and issue tracking](https://github.com/OpenNebula/cloud-provider-opennebula/issues)

## Contact Information

* [OpenNebula web site](https://opennebula.io)
* [Enterprise Services](https://opennebula.io/enterprise)

## License

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

## Author Information

Copyright 2002-2024, OpenNebula Project, OpenNebula Systems

## Acknowledgments
