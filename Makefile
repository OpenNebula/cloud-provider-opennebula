SELF := $(patsubst %/,%,$(dir $(abspath $(firstword $(MAKEFILE_LIST)))))
PATH := $(SELF)/bin:$(PATH)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN := $(shell go env GOPATH)/bin
else
GOBIN := $(shell go env GOBIN)
endif

ENVSUBST_VERSION  ?= 1.4.2
KUBECTL_VERSION   ?= 1.31.4
KUSTOMIZE_VERSION ?= 5.6.0

ENVSUBST  := $(SELF)/bin/envsubst
KUBECTL   := $(SELF)/bin/kubectl
KUSTOMIZE := $(SELF)/bin/kustomize

CLOSEST_TAG ?= $(shell git -C $(SELF) describe --tags --abbrev=0)

# Local image URL used for building/pushing image targets
CCM_IMG ?= localhost:5005/cloud-provider-opennebula:latest

# Image URL to use for building/pushing image targets
IMG_URL ?= ghcr.io/opennebula/cloud-provider-opennebula

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

-include .env
export

.PHONY: all clean

all: build

clean:
	rm --preserve-root -rf '$(SELF)/bin/'

# Development

.PHONY: fmt vet test

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./... -v -count=1

# Build

.PHONY: build docker-build docker-push docker-release

build: fmt vet
	go build -o bin/opennebula-cloud-controller-manager cmd/opennebula-cloud-controller-manager/main.go

docker-build:
	$(CONTAINER_TOOL) build -t $(CCM_IMG) .

docker-push:
	$(CONTAINER_TOOL) push $(CCM_IMG)

# _PLATFORMS defines the target platforms for the manager image be built to provide support to multiple architectures.
# To use this option you need to:
# - be able to use docker buildx (https://docs.docker.com/reference/cli/docker/buildx/)
# - have enabled BuildKit (https://docs.docker.com/build/buildkit/)
# - be able to push the image to your registry
_PLATFORMS ?= linux/amd64,linux/arm64

docker-release:
	-$(CONTAINER_TOOL) buildx create --name cloud-provider-opennebula-builder
	$(CONTAINER_TOOL) buildx use cloud-provider-opennebula-builder
	$(CONTAINER_TOOL) buildx build --push --platform=$(_PLATFORMS) -t $(IMG_URL):$(CLOSEST_TAG) -t $(IMG_URL):latest -f Dockerfile .
	-$(CONTAINER_TOOL) buildx rm cloud-provider-opennebula-builder

# Deployment

ifndef ignore-not-found
ignore-not-found := false
endif

.PHONY: deploy undeploy

deploy: $(KUSTOMIZE) $(ENVSUBST) $(KUBECTL) # Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build kustomize/base/ | $(ENVSUBST) | $(KUBECTL) apply -f-

undeploy: $(KUSTOMIZE) $(ENVSUBST) $(KUBECTL) # Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build kustomize/base/ | $(ENVSUBST) | $(KUBECTL) --ignore-not-found=$(ignore-not-found) delete -f-

# Dependencies

.PHONY: envsubst kubectl kustomize

envsubst: $(ENVSUBST)
$(ENVSUBST):
	$(call go-install-tool,$(ENVSUBST),github.com/a8m/envsubst/cmd/envsubst,v$(ENVSUBST_VERSION))

kubectl: $(KUBECTL)
$(KUBECTL):
	@[ -f $@-v$(KUBECTL_VERSION) ] || \
	{ curl -fsSL https://dl.k8s.io/release/v$(KUBECTL_VERSION)/bin/linux/amd64/kubectl \
	| install -m u=rwx,go= -o $(USER) -D /dev/fd/0 $@-v$(KUBECTL_VERSION); }
	@ln -sf $@-v$(KUBECTL_VERSION) $@

kustomize: $(KUSTOMIZE)
$(KUSTOMIZE):
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
GOBIN=$(SELF)/bin go install $${package}; \
mv $(1) $(1)-$(3); \
}; \
ln -sf $(1)-$(3) $(1)
endef
