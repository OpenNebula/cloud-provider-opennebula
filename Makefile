-include .env
export

# Local image URL used for building/pushing image targets
CCM_IMG ?= localhost:5005/cloud-provider-opennebula:latest

# Image tag/URL used for publising the provider
RELEASE_TAG ?= latest
RELEASE_IMG ?= ghcr.io/opennebula/cloud-provider-opennebula:${RELEASE_TAG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test:
	go test ./... -v -count=1

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/opennebula-cloud-controller-manager cmd/opennebula-cloud-controller-manager/main.go

.PHONY: docker-build
docker-build:
	$(CONTAINER_TOOL) build -t ${CCM_IMG} .

.PHONY: docker-push
docker-push:
	$(CONTAINER_TOOL) push ${CCM_IMG}

.PHONY: docker-publish
docker-publish: docker-build
	$(CONTAINER_TOOL) tag ${CCM_IMG} ${RELEASE_IMG}
	$(CONTAINER_TOOL) push ${RELEASE_IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: deploy
deploy: kustomize envsubst kubectl ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build kustomize/default/ | $(ENVSUBST) | $(KUBECTL) apply -f-

.PHONY: undeploy
undeploy: kustomize envsubst kubectl ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build kustomize/default/ | $(ENVSUBST) | $(KUBECTL) --ignore-not-found=$(ignore-not-found) delete -f-

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	install -d $(LOCALBIN)

## Tool Binaries
ENVSUBST ?= $(LOCALBIN)/envsubst
KUBECTL ?= $(LOCALBIN)/kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize

## Tool Versions
ENVSUBST_VERSION ?= 1.4.2
KUBECTL_VERSION ?= 1.31.1
KUSTOMIZE_VERSION ?= 5.4.3

.PHONY: envsubst
envsubst: $(ENVSUBST)
$(ENVSUBST): $(LOCALBIN)
	$(call go-install-tool,$(ENVSUBST),github.com/a8m/envsubst/cmd/envsubst,v$(ENVSUBST_VERSION))

.PHONY: kubectl
kubectl: $(KUBECTL)
$(KUBECTL): $(LOCALBIN)
	@[ -f $@-v$(KUBECTL_VERSION) ] || \
	{ curl -fsSL https://dl.k8s.io/release/v$(KUBECTL_VERSION)/bin/linux/amd64/kubectl \
	| install -m u=rwx,go= -o $(USER) -D /dev/fd/0 $@-v$(KUBECTL_VERSION); }
	@ln -sf $@-v$(KUBECTL_VERSION) $@

.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,v$(KUSTOMIZE_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3); \
echo "Downloading $${package}"; \
rm -f $(1) ||:; \
GOBIN=$(LOCALBIN) go install $${package}; \
mv $(1) $(1)-$(3); \
}; \
ln -sf $(1)-$(3) $(1)
endef
