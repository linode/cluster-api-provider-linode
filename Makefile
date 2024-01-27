
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
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

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: gosec
gosec: ## Run gosec against code.
	docker run --rm -w /workdir -v $(PWD):/workdir securego/gosec:2.18.2 -exclude-dir=bin -exclude-generated ./...

.PHONY: lint
lint: ## Run lint against code.
	docker run --rm -w /workdir -v $(PWD):/workdir golangci/golangci-lint:v1.55 golangci-lint run -c .golangci.yml

.PHONY: nilcheck
nilcheck: nilaway ## Run nil check against code.
	go list ./... | xargs -I {} -d '\n' nilaway -include-pkgs {} ./...

.PHONY: vulncheck
vulncheck: govulncheck ## Run vulnerability check against code.
	govulncheck ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -race -timeout 60s ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

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
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: tilt-cluster
tilt-cluster: ctlptl tilt kind clusterctl
	@echo -n "LINODE_TOKEN=$(LINODE_TOKEN)" > config/default/.env.linode
	$(CTLPTL) apply -f ctlptl-config.yaml
	$(TILT) up

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
export PATH := $(LOCALBIN):$(PATH)
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CTLPTL ?= $(LOCALBIN)/ctlptl
CLUSTERCTL ?= $(LOCALBIN)/clusterctl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
TILT ?= $(LOCALBIN)/tilt
KIND ?= $(LOCALBIN)/kind
ENVTEST ?= $(LOCALBIN)/setup-envtest
HUSKY ?= $(LOCALBIN)/husky
NILAWAY ?= $(LOCALBIN)/nilaway
GOVULNC ?= $(LOCALBIN)/govulncheck

## Tool Versions
KUSTOMIZE_VERSION ?= v5.1.1
CTLPTL_VERSION ?= v0.8.25
CLUSTERCTL_VERSION ?= v1.5.3
CONTROLLER_TOOLS_VERSION ?= v0.13.0
TILT_VERSION ?= 0.33.6
KIND_VERSION ?= 0.20.0
HUSKY_VERSION ?= v0.2.16
NILAWAY_VERSION ?= latest
GOVULNC_VERSION ?= v1.0.1

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: ctlptl
ctlptl: $(CTLPTL) ## Download ctlptl locally if necessary. If wrong version is installed, it will be overwritten.
$(CTLPTL): $(LOCALBIN)
	test -s $(LOCALBIN)/ctlptl && $(LOCALBIN)/ctlptl version | grep -q $(CTLPTL_VERSION) || \
	(GOBIN=$(LOCALBIN) go install github.com/tilt-dev/ctlptl/cmd/ctlptl@$(CTLPTL_VERSION))

.PHONY: clusterctl
clusterctl: $(CLUSTERCTL) ## Download clusterctl locally if necessary. If wrong version is installed, it will be overwritten.
$(CLUSTERCTL): $(LOCALBIN)
	test -s $(LOCALBIN)/clusterctl && $(LOCALBIN)/clusterctl version | grep -q $(CLUSTERCTL_VERSION) || \
	(cd $(LOCALBIN); curl -fsSL https://github.com/kubernetes-sigs/cluster-api/releases/download/$(CLUSTERCTL_VERSION)/clusterctl-$(OS)-$(ARCH_SHORT) -o clusterctl)
	@chmod +x $(CLUSTERCTL)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: tilt
tilt: $(TILT) ## Download tilt locally if necessary. If wrong version is installed, it will be overwritten.
$(TILT): $(LOCALBIN)
	TILT_OS=$(OS); \
	if [ $$TILT_OS = "darwin" ]; then \
		TILT_OS=mac; \
	fi; \
	test -s $(LOCALBIN)/tilt && $(LOCALBIN)/tilt version | grep -q $(TILT_VERSION) || \
	(cd $(LOCALBIN); curl -fsSL https://github.com/tilt-dev/tilt/releases/download/v$(TILT_VERSION)/tilt.$(TILT_VERSION).$$TILT_OS.$(ARCH).tar.gz | tar -xzv tilt)

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary. If wrong version is installed, it will be overwritten.
$(KIND): $(LOCALBIN)
	test -s $(KIND) && $(KIND) version | grep -q $(KIND_VERSION) || \
	(cd $(LOCALBIN); curl -Lso ./kind https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-$(OS)-$(ARCH_SHORT) && chmod +x kind)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

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

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)
