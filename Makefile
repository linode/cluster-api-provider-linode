#####################################################################
# top-level Makefile for cluster-api-provider-linode
#####################################################################
REGISTRY            ?= docker.io/linode
IMAGE_NAME          ?= cluster-api-provider-linode
CONTROLLER_IMAGE    ?= $(REGISTRY)/$(IMAGE_NAME)
TAG                 ?= dev
ENVTEST_K8S_VERSION := 1.28.0
VERSION             ?= $(shell git describe --always --tag --dirty=-dev)
GIT_REF             ?= $(shell git rev-parse --short HEAD)
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
generate: generate-manifests generate-code generate-mock

.PHONY: generate-manifests
generate-manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate-code
generate-code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-mock
generate-mock: mockgen ## Generate mocks for the Linode API client.
	$(MOCKGEN) -source=./clients/clients.go -destination ./mock/client.go -package mock

.PHONY: generate-flavors ## Generate template flavors.
generate-flavors: $(KUSTOMIZE)
	./hack/generate-flavors.sh

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
	docker run --rm -w /workdir -v $(PWD):/workdir securego/gosec:2.19.0 -exclude-dir=bin -exclude-generated ./...

.PHONY: lint
lint: ## Run lint against code.
	docker run --rm -w /workdir -v $(PWD):/workdir golangci/golangci-lint:v1.57.2 golangci-lint run -c .golangci.yml --fix

.PHONY: nilcheck
nilcheck: nilaway ## Run nil check against code.
	go list ./... | xargs -I {} -d '\n' nilaway -include-pkgs {} -exclude-file-docstrings "ignore_autogenerated" ./...

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
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(CACHE_BIN) -p path)" go test -race -timeout 60s `go list ./... | grep -v ./mock$$`  -coverprofile cover.out.tmp
	grep -v "zz_generated.deepcopy.go" cover.out.tmp > cover.out
	rm cover.out.tmp

.PHONY: e2etest
e2etest: generate local-release local-deploy chainsaw
	GIT_REF=$(GIT_REF) $(CHAINSAW) test ./e2e --selector $(E2E_SELECTOR) $(E2E_FLAGS)

local-deploy: kind ctlptl tilt kustomize clusterctl
	@echo -n "LINODE_TOKEN=$(LINODE_TOKEN)" > config/default/.env.linode
	$(CTLPTL) apply -f .tilt/ctlptl-config.yaml
	$(TILT) ci -f Tiltfile

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
	@echo -n "LINODE_TOKEN=$(LINODE_TOKEN)" > config/default/.env.linode
	$(CTLPTL) apply -f .tilt/ctlptl-config.yaml
	$(TILT) up --stream

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

## --------------------------------------
## Tooling Binaries
## --------------------------------------

##@ Tooling Binaries:
# setup-envtest does not have devbox support so always use CACHE_BIN

KUBECTL        ?= $(LOCALBIN)/kubectl
KUSTOMIZE      ?= $(LOCALBIN)/kustomize
CTLPTL         ?= $(LOCALBIN)/ctlptl
CLUSTERCTL     ?= $(LOCALBIN)/clusterctl
KUBEBUILDER    ?= $(LOCALBIN)/kubebuilder
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
TILT           ?= $(LOCALBIN)/tilt
KIND           ?= $(LOCALBIN)/kind
CHAINSAW       ?= $(LOCALBIN)/chainsaw
ENVTEST        ?= $(CACHE_BIN)/setup-envtest
HUSKY          ?= $(LOCALBIN)/husky
NILAWAY        ?= $(LOCALBIN)/nilaway
GOVULNC        ?= $(LOCALBIN)/govulncheck
MOCKGEN        ?= $(LOCALBIN)/mockgen

## Tool Versions
KUSTOMIZE_VERSION        ?= v5.1.1
CTLPTL_VERSION           ?= v0.8.25
CLUSTERCTL_VERSION       ?= v1.5.3
KUBEBUILDER_VERSION      ?= v3.14.1
CONTROLLER_TOOLS_VERSION ?= v0.14.0
TILT_VERSION             ?= 0.33.6
KIND_VERSION             ?= 0.20.0
CHAINSAW_VERSION         ?= v0.1.9
HUSKY_VERSION            ?= v0.2.16
NILAWAY_VERSION          ?= latest
GOVULNC_VERSION          ?= v1.0.1
MOCKGEN_VERSION          ?= v0.4.0

.PHONY: tools
tools: $(KUSTOMIZE) $(CTLPTL) $(CLUSTERCTL) $(CONTROLLER_GEN) $(TILT) $(KIND) $(CHAINSAW) $(ENVTEST) $(HUSKY) $(NILAWAY) $(GOVULNC) $(MOCKGEN)


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

.PHONY: kubebuilder
kubebuilder: $(KUBEBUILDER) ## Download kubebuilder locally if necessary.
$(KUBEBUILDER): $(LOCALBIN)
	curl -L -o $(LOCALBIN)/kubebuilder https://github.com/kubernetes-sigs/kubebuilder/releases/download/$(KUBEBUILDER_VERSION)/kubebuilder_$(OS)_$(ARCH_SHORT)
	chmod +x $(LOCALBIN)/kubebuilder

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(CACHE_BIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)


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
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(CACHE_BIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: husky
husky: $(HUSKY) ## Download husky locally if necessary.
	@echo Execute install command to enable git hooks: ./bin/husky install
	@echo Set any value for SKIP_GIT_PUSH_HOOK env variable to skip git hook execution.
$(HUSKY): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/automation-co/husky@$(HUSKY_VERSION)

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
