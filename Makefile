#####################################################################
# top-level Makefile for cluster-api-provider-linode
#####################################################################
REGISTRY            ?= docker.io/linode
IMAGE_NAME          ?= cluster-api-provider-linode
CONTROLLER_IMAGE    ?= $(REGISTRY)/$(IMAGE_NAME)
TAG                 ?= dev
ENVTEST_K8S_VERSION := $(shell go list -m -f '{{.Version}}' k8s.io/client-go)
VERSION             ?= $(shell git describe --always --tag --dirty=-dev)
BUILD_ARGS          := --build-arg VERSION=$(VERSION)
SHELL                = /usr/bin/env bash -o pipefail
.SHELLFLAGS          = -ec
CONTAINER_TOOL      ?= docker
MDBOOK_DEV_HOST      = 0.0.0.0
MDBOOK_DEV_PORT      = 3000
E2E_SELECTOR        ?= all

# ENVTEST_K8S_VERSION
# - refers to the version of kubebuilder assets to be downloaded by envtest binary.
# CONTAINER_TOOL
# - defines the container tool to be used for building images.
#   Be aware that the target commands are only tested with Docker which is
#   scaffolded by default. However, you might want to replace it to use other
#   tools. (i.e. podman)

#####################################################################
# OS / ARCH
#####################################################################
OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m)
ARCH_SHORT=$(ARCH)
ifeq ($(ARCH_SHORT),x86_64)
ARCH_SHORT := amd64
else ifeq ($(ARCH_SHORT),aarch64)
ARCH_SHORT := arm64
endif
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

#####################################################################
##@ Build All
#####################################################################
.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php


## --------------------------------------
## Help
## --------------------------------------

##@ Help:

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Generate
## --------------------------------------

##@ Generate:
.PHONY: generate
generate: generate-manifests generate-code generate-mock generate-api-docs

.PHONY: generate-manifests
generate-manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate-code
generate-code: controller-gen gowrap ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	go generate ./...
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-mock
generate-mock: mockgen ## Generate mocks for the Linode API client.
	$(MOCKGEN) -source=./clients/clients.go -destination ./mock/client.go -package mock

.PHONY: generate-flavors ## Generate template flavors.
generate-flavors: $(KUSTOMIZE)
	bash hack/generate-flavors.sh

.PHONY: generate-api-docs
generate-api-docs: crd-ref-docs ## Generate API reference documentation.
	$(CRD_REF_DOCS) \
		--config=./docs/.crd-ref-docs.yaml \
		--source-path=./api/ \
		--renderer=markdown \
		--output-path=./docs/src/reference

.PHONY: check-gen-diff
check-gen-diff:
	git diff --no-ext-diff --exit-code

## --------------------------------------
## Development
## --------------------------------------

##@ Development:

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: gosec
gosec: ## Run gosec against code.
	docker run --rm -w /workdir -v $(PWD):/workdir securego/gosec:$(GOSEC_VERSION) -exclude-dir=bin -exclude-generated ./...

.PHONY: lint
lint: ## Run lint against code.
	$(GOLANGCI_LINT) run -c .golangci.yml

.PHONY: lint
lint-api: golangci-lint-custom ## Run lint against code.
	$(GOLANGCI_LINT_CUSTOM) run -c .golangci-kal.yml

.PHONY: nilcheck
nilcheck: golangci-lint-custom ## Run nil check against code.
	$(GOLANGCI_LINT_CUSTOM) run -c .golangci-nilaway.yml

.PHONY: vulncheck
vulncheck: govulncheck ## Run vulnerability check against code.
	govulncheck ./...

.PHONY: docs
docs:
	@cd docs && mdbook serve -n $(MDBOOK_DEV_HOST) -p $(MDBOOK_DEV_PORT)

## --------------------------------------
## Testing
## --------------------------------------

##@ Testing:

.PHONY: test
test: generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use ${ENVTEST_K8S_VERSION#v} --bin-dir $(CACHE_BIN) -p path)" go test -race -timeout 60s `go list ./... | grep -v ./mock$$`  -coverprofile cover.out.tmp
	grep -v "zz_generated.*" cover.out.tmp > cover.out
	rm cover.out.tmp

.PHONY: e2etest
e2etest: generate local-release local-deploy chainsaw s5cmd
	SSE_KEY=$$(openssl rand -base64 32) LOCALBIN=$(CACHE_BIN) $(CHAINSAW) test ./e2e --parallel 2 --selector $(E2E_SELECTOR) $(E2E_FLAGS)

.PHONY: local-deploy
local-deploy: kind-cluster tilt kustomize clusterctl
	$(TILT) ci -f Tiltfile

.PHONY: kind-cluster
kind-cluster: kind ctlptl
	$(CTLPTL) apply -f .tilt/ctlptl-config.yaml

##@ Test Upgrade:

LATEST_REF         := $(shell git rev-parse --short HEAD)
LAST_RELEASE       := $(shell git describe --abbrev=0 --tags)
COMMON_NAMESPACE   := test-upgrade

.PHONY: checkout-latest-commit
checkout-latest-commit:
	git checkout $(LATEST_REF)

.PHONY: checkout-last-release
checkout-last-release:
	git checkout $(LAST_RELEASE)

.PHONY: last-release-cluster
last-release-cluster: kind ctlptl tilt kustomize clusterctl chainsaw kind-cluster checkout-last-release local-release local-deploy
	LOCALBIN=$(CACHE_BIN) CLUSTERCTL_CONFIG=$(CLUSTERCTL_CONFIG) SKIP_CUSTOM_DELETE=true $(CHAINSAW) test --namespace $(COMMON_NAMESPACE) --assert-timeout 600s --skip-delete ./e2e/capl-cluster-flavors/kubeadm-capl-cluster

.PHONY: test-upgrade
test-upgrade: last-release-cluster checkout-latest-commit
	$(MAKE) local-release
	$(MAKE) local-deploy
	LOCALBIN=$(CACHE_BIN) CLUSTERCTL_CONFIG=$(CLUSTERCTL_CONFIG) $(CHAINSAW) test --namespace $(COMMON_NAMESPACE) --assert-timeout 800s ./e2e/capl-cluster-flavors/kubeadm-capl-cluster

.PHONY: clean-kind-cluster
clean-kind-cluster: ctlptl
	$(CTLPTL) delete -f .tilt/ctlptl-config.yaml

## --------------------------------------
## Build
## --------------------------------------

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -ldflags="-X github.com/linode/cluster-api-provider-linode/version.version=$(VERSION)" -o bin/manager cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build $(BUILD_ARGS) . -t $(CONTROLLER_IMAGE):$(VERSION)

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push $(CONTROLLER_IMAGE):$(VERSION)

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build $(BUILD_ARGS) --push --platform=$(PLATFORMS) --tag $(CONTROLLER_IMAGE):$(VERSION) -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

## --------------------------------------
## Deployment
## --------------------------------------

##@ Deployment:

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: tilt-cluster
tilt-cluster: ctlptl tilt kind clusterctl
	$(CTLPTL) apply -f .tilt/ctlptl-config.yaml
	$(TILT) up

## --------------------------------------
## Release
## --------------------------------------

##@ Release:

RELEASE_DIR ?= infrastructure-linode

.PHONY: release
release: kustomize clean-release set-manifest-image release-manifests generate-flavors release-templates release-metadata clean-release-git

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: release-metadata
release-metadata: $(RELEASE_DIR)
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-templates
release-templates: $(RELEASE_DIR)
	mv templates/cluster-template* $(RELEASE_DIR)/
	mv templates/clusterclass* $(RELEASE_DIR)/

.PHONY: set-manifest-image
set-manifest-image: ## Update kustomize image patch file for default resource.
	sed -i'' -e 's@image: .*@image: '"$(REGISTRY)/$(IMAGE_NAME):$(VERSION)"'@' ./config/default/manager_image_patch.yaml

.PHONY: release-manifests
release-manifests: $(KUSTOMIZE) $(RELEASE_DIR)
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

.PHONY: local-release
local-release:
	RELEASE_DIR=infrastructure-local-linode/v0.0.0 $(MAKE) release
	$(MAKE) clean-release-git

## --------------------------------------
## Cleanup
## --------------------------------------

##@ Cleanup:

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore config/default/*manager_image_patch.yaml

.PHONY: clean-release
clean-release: clean-release-git
	rm -rf $(RELEASE_DIR)

.PHONY: clean-child-clusters
clean-child-clusters: kubectl
	$(KUBECTL) delete clusters -A --all --timeout=180s

## --------------------------------------
## Build Dependencies
## --------------------------------------

##@ Build Dependencies:

## Location to install dependencies to

# Use CACHE_BIN for tools that cannot use devbox and LOCALBIN for tools that can use either method
CACHE_BIN ?= $(CURDIR)/bin
LOCALBIN ?= $(CACHE_BIN)

DEVBOX_BIN ?= $(DEVBOX_PACKAGES_DIR)/bin

# if the $DEVBOX_PACKAGES_DIR env variable exists that means we are within a devbox shell and can safely
# use devbox's bin for our tools
ifdef DEVBOX_PACKAGES_DIR
	LOCALBIN = $(DEVBOX_BIN)
endif

export PATH := $(CACHE_BIN):$(PATH)
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

$(CACHE_BIN):
	mkdir -p $(CACHE_BIN)

## --------------------------------------
## Tooling Binaries
## --------------------------------------

##@ Tooling Binaries:
# setup-envtest does not have devbox support so always use CACHE_BIN

KUBECTL        ?= $(LOCALBIN)/kubectl
KUSTOMIZE      ?= $(LOCALBIN)/kustomize
CTLPTL         ?= $(LOCALBIN)/ctlptl
CLUSTERCTL     ?= $(LOCALBIN)/clusterctl
CRD_REF_DOCS   ?= $(CACHE_BIN)/crd-ref-docs
KUBEBUILDER    ?= $(LOCALBIN)/kubebuilder
CONTROLLER_GEN ?= $(CACHE_BIN)/controller-gen
CONVERSION_GEN ?= $(CACHE_BIN)/conversion-gen
TILT           ?= $(LOCALBIN)/tilt
KIND           ?= $(LOCALBIN)/kind
CHAINSAW       ?= $(LOCALBIN)/chainsaw
ENVTEST        ?= $(CACHE_BIN)/setup-envtest
NILAWAY        ?= $(LOCALBIN)/nilaway
GOVULNC        ?= $(LOCALBIN)/govulncheck
MOCKGEN        ?= $(LOCALBIN)/mockgen
GOWRAP         ?= $(CACHE_BIN)/gowrap
GOLANGCI_LINT  ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_CUSTOM ?= $(CACHE_BIN)/golangci-lint-custom
S5CMD          ?= $(CACHE_BIN)/s5cmd

## Tool Versions
# renovate: datasource=go depName=sigs.k8s.io/kustomize
KUSTOMIZE_VERSION        ?= v5.7.1

# renovate: datasource=go depName=github.com/tilt-dev/ctlptl
CTLPTL_VERSION           ?= v0.9.0

# renovate: datasource=github-tags depName=kubernetes-sigs/cluster-api
CLUSTERCTL_VERSION       ?= v1.12.3

# renovate: datasource=go depName=github.com/elastic/crd-ref-docs
CRD_REF_DOCS_VERSION     ?= v0.2.0

# renovate: datasource=github-tags depName=kubernetes/kubernetes
KUBECTL_VERSION          ?= v1.35.1

# renovate: datasource=github-tags depName=kubernetes-sigs/kubebuilder
KUBEBUILDER_VERSION      ?= v4.11.1

# renovate: datasource=go depName=sigs.k8s.io/controller-runtime/tools/setup-envtest
ENVTEST_VERSION 	 ?= release-0.22

# renovate: datasource=go depName=sigs.k8s.io/controller-tools
CONTROLLER_TOOLS_VERSION ?= v0.20.0

# renovate: datasource=github-tags depName=tilt-dev/tilt
TILT_VERSION             ?= 0.36.3

# renovate: datasource=github-tags depName=kubernetes-sigs/kind
KIND_VERSION             ?= 0.31.0

# renovate: datasource=go depName=github.com/kyverno/chainsaw
CHAINSAW_VERSION         ?= v0.2.13

# renovate: datasource=go depName=go.uber.org/nilaway
NILAWAY_VERSION          ?= d2274102dc2eab9f77cef849a5470a6ebf983125

# renovate: datasource=go depName=golang.org/x/vuln
GOVULNC_VERSION          ?= v1.1.4

# renovate: datasource=go depName=go.uber.org/mock/mockgen
MOCKGEN_VERSION          ?= v0.6.0

# renovate: datasource=go depName=github.com/hexdigest/gowrap
GOWRAP_VERSION           ?= v1.4.3

# renovate: datasource=go depName=github.com/peak/s5cmd
S5CMD_VERSION            ?= v2.3.0

# renovate: datasource=go depName=k8s.io/code-generator
CONVERSION_GEN_VERSION   ?= v0.35.1

# renovate: datasource=github-tags depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION    ?= v2.10.1

# renovate: datasource=github-tags depName=securego/gosec
GOSEC_VERSION            ?= 2.22.11

.PHONY: tools
tools: $(KUSTOMIZE) $(CTLPTL) $(CLUSTERCTL) $(KUBECTL) $(CONTROLLER_GEN) $(CONVERSION_GEN) $(TILT) $(KIND) $(CHAINSAW) $(ENVTEST) $(NILAWAY) $(GOVULNC) $(MOCKGEN) $(GOWRAP)


.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: ctlptl
ctlptl: $(CTLPTL) ## Download ctlptl locally if necessary.
$(CTLPTL): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/tilt-dev/ctlptl/cmd/ctlptl@$(CTLPTL_VERSION)

.PHONY: clusterctl
clusterctl: $(CLUSTERCTL) ## Download clusterctl locally if necessary.
$(CLUSTERCTL): $(LOCALBIN)
	curl -fsSL https://github.com/kubernetes-sigs/cluster-api/releases/download/$(CLUSTERCTL_VERSION)/clusterctl-$(OS)-$(ARCH_SHORT) -o $(CLUSTERCTL)
	chmod +x $(CLUSTERCTL)

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	GOBIN=$(CACHE_BIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary.
$(KUBECTL): $(LOCALBIN)
	curl -fsSL https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OS)/$(ARCH_SHORT)/kubectl -o $(KUBECTL)
	chmod +x $(KUBECTL)

.PHONY: kubebuilder
kubebuilder: $(KUBEBUILDER) ## Download kubebuilder locally if necessary.
$(KUBEBUILDER): $(LOCALBIN)
	curl -L -o $(LOCALBIN)/kubebuilder https://github.com/kubernetes-sigs/kubebuilder/releases/download/$(KUBEBUILDER_VERSION)/kubebuilder_$(OS)_$(ARCH_SHORT)
	chmod +x $(LOCALBIN)/kubebuilder

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(CACHE_BIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: conversion-gen
conversion-gen: $(CONVERSION_GEN) ## Download conversion-gen locally if necessary.
$(CONVERSION_GEN): $(LOCALBIN)
	GOBIN=$(CACHE_BIN) go install k8s.io/code-generator/cmd/conversion-gen@$(CONVERSION_GEN_VERSION)

.PHONY: tilt
tilt: $(TILT) ## Download tilt locally if necessary.
$(TILT): $(LOCALBIN)
	TILT_OS=$(OS); \
	if [ $$TILT_OS = "darwin" ]; then \
		TILT_OS=mac; \
	fi; \
	curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$(TILT_VERSION)/tilt.$(TILT_VERSION).$$TILT_OS.$(ARCH).tar.gz | tar -xzvm -C $(LOCALBIN) tilt

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	curl -Lso $(KIND) https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-$(OS)-$(ARCH_SHORT)
	chmod +x $(KIND)

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(CACHE_BIN)
	GOBIN=$(CACHE_BIN) go install github.com/kyverno/chainsaw@$(CHAINSAW_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(CACHE_BIN)
	GOBIN=$(CACHE_BIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

.phony: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): # Build golangci-lint from tools folder.
	GOBIN=$(LOCALBIN)  go install  github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.phony: golangci-lint-custom
golangci-lint-custom: $(GOLANGCI_LINT_CUSTOM)
$(GOLANGCI_LINT_CUSTOM): $(GOLANGCI_LINT) # Build golangci-lint-custom from custom configuration.
	$(GOLANGCI_LINT) custom

.PHONY: nilaway
nilaway: $(NILAWAY) ## Download nilaway locally if necessary.
$(NILAWAY): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install go.uber.org/nilaway/cmd/nilaway@$(NILAWAY_VERSION)

.PHONY: govulncheck
govulncheck: $(GOVULNC) ## Download govulncheck locally if necessary.
$(GOVULNC): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNC_VERSION)

.PHONY: mockgen
mockgen: $(MOCKGEN) ## Download mockgen locally if necessary.
$(MOCKGEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install go.uber.org/mock/mockgen@$(MOCKGEN_VERSION)

.PHONY: gowrap
gowrap: $(GOWRAP) ## Download gowrap locally if necessary.
$(GOWRAP): $(CACHE_BIN)
	GOBIN=$(CACHE_BIN) go install github.com/hexdigest/gowrap/cmd/gowrap@$(GOWRAP_VERSION)

.PHONY: s5cmd
s5cmd: $(S5CMD)
$(S5CMD): $(CACHE_BIN)
	GOBIN=$(CACHE_BIN) go install github.com/peak/s5cmd/v2@$(S5CMD_VERSION)
