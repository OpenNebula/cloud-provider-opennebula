-include .env
export

benchmark-build:
	$(CONTAINER_TOOL) build -t $(LOCAL_REGISTRY)/fio-benchmark:dev -f benchmark/csi/Dockerfile .

benchmark-push:
	$(CONTAINER_TOOL) push $(LOCAL_REGISTRY)/fio-benchmark:dev

benchmark-apply: $(ENVSUBST) $(KUBECTL) $(WORKLOAD_CLUSTER_KUBECONFIG)
	$(ENVSUBST) < benchmark/csi/statefulset.yaml | $(KUBECTL) --kubeconfig $(WORKLOAD_CLUSTER_KUBECONFIG) apply -f-

