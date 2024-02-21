REGISTRY ?= docker.io/linode
IMAGE_NAME ?= cluster-api-provider-linode
CONTROLLER_IMAGE ?= $(REGISTRY)/$(IMAGE_NAME)
TAG ?= dev
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.0
OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m)
ARCH_SHORT=$(ARCH)
ifeq ($(ARCH_SHORT),x86_64)
ARCH_SHORT := amd64
else ifeq ($(ARCH_SHORT),aarch64)
ARCH_SHORT := arm64
endif
VERSION ?= $(shell git describe --tags --dirty=-dev)
BUILD_ARGS := --build-arg VERSION=$(VERSION)
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

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

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
	docker run --rm -w /workdir -v $(PWD):/workdir golangci/golangci-lint:v1.56.1 golangci-lint run -c .golangci.yml

.PHONY: nilcheck
nilcheck: nilaway ## Run nil check against code.
	go list ./... | xargs -I {} -d '\n' nilaway -include-pkgs {} ./...

.PHONY: vulncheck
vulncheck: govulncheck ## Run vulnerability check against code.
	govulncheck ./...

## --------------------------------------
## Testing
## --------------------------------------

##@ Testing:

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -race -timeout 60s ./... -coverprofile cover.out

.PHONY: e2etest
e2etest:
	make --no-print-directory _e2etest # Workaround to force the flag on Github Action

_e2etest-infra: kind ctlptl tilt kuttl kustomize clusterctl
	@echo -n "LINODE_TOKEN=$(LINODE_TOKEN)" > config/default/.env.linode
	$(CTLPTL) apply -f .tilt/ctlptl-config.yaml
	$(TILT) ci --timeout 240s -f Tiltfile

_e2etest: manifests generate _e2etest-infra
	ROOT_DIR="$(PWD)" $(KUTTL) test --config e2e/kuttl-config.yaml

## --------------------------------------
## Build
## --------------------------------------

##@ Build:

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -ldflags="-X github.com/linode/cluster-api-provider-linode/version.version=$(VERSION)" -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build $(BUILD_ARGS) . -t $(CONTROLLER_IMAGE):$(TAG)

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push $(CONTROLLER_IMAGE):$(TAG)

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
	- $(CONTAINER_TOOL) buildx build $(BUILD_ARGS) --push --platform=$(PLATFORMS) --tag $(CONTROLLER_IMAGE):$(TAG) -f Dockerfile.cross .
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

RELEASE_DIR ?= release
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)

.PHONY: set-manifest-image
set-manifest-image: ## Update kustomize image patch file for default resource.
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:${MANIFEST_TAG}"'@' ./config/default/manager_image_patch.yaml

.PHONY: release
release: $(KUSTOMIZE)
	rm -rf $(RELEASE_DIR)
	mkdir -p $(RELEASE_DIR)/
	$(MAKE) set-manifest-image MANIFEST_IMG=$(REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG)
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/infrastructure-components.yaml
	cp templates/cluster-template* $(RELEASE_DIR)/
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

## --------------------------------------
## Cleanup
## --------------------------------------

##@ Cleanup:

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)

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

KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CTLPTL ?= $(LOCALBIN)/ctlptl
CLUSTERCTL ?= $(LOCALBIN)/clusterctl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
TILT ?= $(LOCALBIN)/tilt
KIND ?= $(LOCALBIN)/kind
KUTTL ?= $(LOCALBIN)/kubectl-kuttl
# setup-envtest does not have devbox support so always use CACHE_BIN
ENVTEST ?= $(CACHE_BIN)/setup-envtest
HUSKY ?= $(LOCALBIN)/husky
NILAWAY ?= $(LOCALBIN)/nilaway
GOVULNC ?= $(LOCALBIN)/govulncheck

## Tool Versions
KUSTOMIZE_VERSION ?= v5.1.1
CTLPTL_VERSION ?= v0.8.25
CLUSTERCTL_VERSION ?= v1.5.3
CONTROLLER_TOOLS_VERSION ?= v0.14.0
TILT_VERSION ?= 0.33.6
KIND_VERSION ?= 0.20.0
KUTTL_VERSION ?= 0.15.0
HUSKY_VERSION ?= v0.2.16
NILAWAY_VERSION ?= latest
GOVULNC_VERSION ?= v1.0.1

.PHONY: tools
tools: $(KUSTOMIZE) $(CTLPTL) $(CLUSTERCTL) $(CONTROLLER_GEN) $(TILT) $(KIND) $(KUTTL) $(ENVTEST) $(HUSKY) $(NILAWAY) $(GOVULNC)

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

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)


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

.PHONY: kuttl
kuttl: $(KUTTL) ## Download kuttl locally if necessary.
$(KUTTL): $(LOCALBIN)
	curl -Lso $(KUTTL) https://github.com/kudobuilder/kuttl/releases/download/v$(KUTTL_VERSION)/kubectl-kuttl_$(KUTTL_VERSION)_$(OS)_$(ARCH)
	chmod +x $(KUTTL)

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
