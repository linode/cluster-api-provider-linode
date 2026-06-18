#####################################################################
# top-level Makefile for cluster-api-provider-linode
#####################################################################
VERSION             ?= $(shell git describe --always --tag --dirty=-dev)
IMAGE_TAGS          ?= $(VERSION)
WITH_GOFLAGS        ?= "GOFLAGS=\"-ldflags=-X github.com/linode/cluster-api-provider-linode/version.version=$(VERSION)\""
KO_DOCKER_REPO      ?= docker.io/linode/cluster-api-provider-linode
KOCACHE ?= ~/.ko
RELEASE_DIR ?= release
ENVTEST_K8S_VERSION := $(shell go list -m -f '{{.Version}}' k8s.io/client-go)
BUILD_ARGS          := --build-arg VERSION=$(VERSION)
SHELL                = /usr/bin/env bash -o pipefail
.SHELLFLAGS          = -ec
MDBOOK_DEV_HOST      = 0.0.0.0
MDBOOK_DEV_PORT      = 3000
E2E_SELECTOR        ?= all

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
generate-manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate-code
generate-code: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	go generate ./...
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-mock
generate-mock: ## Generate mocks for the Linode API client.
	mockgen -source=./clients/clients.go -destination ./mock/client.go -package mock

.PHONY: generate-flavors ## Generate template flavors.
generate-flavors:
	bash hack/generate-flavors.sh

.PHONY: generate-api-docs
generate-api-docs: ## Generate API reference documentation.
	crd-ref-docs \
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

.PHONY: tidy
tidy: ## Run go mod tidy against code.
	go mod tidy

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: gosec
gosec: ## Run gosec against code.
	gosec -exclude-dir=bin -exclude-generated ./...

.PHONY: lint
lint: ## Run lint against code.
	golangci-lint run -c .golangci.yml

.PHONY: lint
lint-api: golangci-lint-custom ## Run lint against code.
	$(GOLANGCI_LINT_CUSTOM) run -c .golangci-kal.yml


.PHONY: nilcheck
nilcheck: golangci-lint-custom ## Run nil check against code.
	$(GOLANGCI_LINT_CUSTOM) run -c .golangci-nilaway.yml

.PHONY: vulncheck
vulncheck: ## Run vulnerability check against code.
	govulncheck ./...

.PHONY: docs
docs:
	@cd docs && mdbook serve -n $(MDBOOK_DEV_HOST) -p $(MDBOOK_DEV_PORT)

## --------------------------------------
## Testing
## --------------------------------------

##@ Testing:

.PHONY: test
test: generate fmt vet ## Run tests.
	KUBEBUILDER_ASSETS="$(shell setup-envtest use ${ENVTEST_K8S_VERSION#v} -p path)" go test -race -timeout 60s `go list ./... | grep -v ./mock$$`  -coverprofile cover.out.tmp
	grep -v "zz_generated.*" cover.out.tmp > cover.out
	rm cover.out.tmp

.PHONY: e2etest
e2etest: generate local-release local-deploy
	SSE_KEY=$$(openssl rand -base64 32) chainsaw test ./e2e --parallel 2 --selector $(E2E_SELECTOR) $(E2E_FLAGS)

.PHONY: local-deploy
local-deploy: kind-cluster
	tilt ci -f Tiltfile

.PHONY: kind-cluster
kind-cluster:
	ctlptl apply -f .tilt/ctlptl-config.yaml

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
last-release-cluster: kind-cluster checkout-last-release local-release local-deploy
	CLUSTERCTL_CONFIG=$(CLUSTERCTL_CONFIG) SKIP_CUSTOM_DELETE=true chainsaw test --namespace $(COMMON_NAMESPACE) --assert-timeout 600s --skip-delete ./e2e/capl-cluster-flavors/kubeadm-capl-cluster

.PHONY: test-upgrade
test-upgrade: last-release-cluster checkout-latest-commit
	$(MAKE) local-release
	$(MAKE) local-deploy
	CLUSTERCTL_CONFIG=$(CLUSTERCTL_CONFIG) chainsaw test --namespace $(COMMON_NAMESPACE) --assert-timeout 800s ./e2e/capl-cluster-flavors/kubeadm-capl-cluster

.PHONY: clean-kind-cluster
clean-kind-cluster:
	ctlptl delete -f .tilt/ctlptl-config.yaml

## --------------------------------------
## Build
## --------------------------------------

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -ldflags="-X github.com/linode/cluster-api-provider-linode/version.version=$(VERSION)" -o bin/manager cmd/main.go

.PHONY: ko-build
ko-build:
	$(WITH_GO_FLAGS) KO_CACHE=KOCACHE$(KOCACHE) ko build --local -t $(IMAGE_TAGS) --bare github.com/linode/cluster-api-provider-linode/cmd

.PHONY: ko-publish
ko-publish:
	$(WITH_GO_FLAGS) KO_CACHE=KOCACHE$(KOCACHE) KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko build -t $(IMAGE_TAGS) --bare github.com/linode/cluster-api-provider-linode/cmd

## --------------------------------------
## Deployment
## --------------------------------------

##@ Deployment:

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: tilt-cluster
tilt-cluster:
	ctltpl apply -f .tilt/ctlptl-config.yaml
	tilt up

## --------------------------------------
## Release
## --------------------------------------

##@ Release:

RELEASE_DIR ?= infrastructure-linode

.PHONY: release
release: clean-release set-manifest-image release-manifests generate-flavors release-templates release-metadata clean-release-git

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
release-manifests: $(RELEASE_DIR)
	kustomize build config/default > $(RELEASE_DIR)/infrastructure-components.yaml

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
clean-child-clusters:
	kubectl delete clusters -A --all --timeout=180s

## --------------------------------------------
## Build deps (that can't be installed by mise)
## --------------------------------------------

CACHE_BIN ?= $(CURDIR)/bin
LOCALBIN ?= $(CACHE_BIN)

NILAWAY              ?= $(LOCALBIN)/nilaway
GOLANGCI_LINT_CUSTOM ?= $(CACHE_BIN)/golangci-lint-custom
CONVERSION_GEN       ?= $(CACHE_BIN)/conversion-gen

# renovate: datasource=go depName=go.uber.org/nilaway
NILAWAY_VERSION          ?= d2274102dc2eab9f77cef849a5470a6ebf983125
# renovate: datasource=go depName=k8s.io/code-generator
CONVERSION_GEN_VERSION   ?= v0.35.3

.PHONY: nilaway
nilaway: $(NILAWAY) ## Download nilaway locally if necessary.

$(NILAWAY): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install go.uber.org/nilaway/cmd/nilaway@$(NILAWAY_VERSION)

.PHONY: conversion-gen
conversion-gen: $(CONVERSION_GEN) ## Download conversion-gen locally if necessary.
$(CONVERSION_GEN): $(LOCALBIN)
	GOBIN=$(CACHE_BIN) go install k8s.io/code-generator/cmd/conversion-gen@$(CONVERSION_GEN_VERSION)

.phony: golangci-lint-custom
golangci-lint-custom: $(GOLANGCI_LINT_CUSTOM)
$(GOLANGCI_LINT_CUSTOM): # Build golangci-lint-custom from custom configuration.
	golangci-lint custom
