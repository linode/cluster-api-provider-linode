ROOT_DIR ?= $(shell git rev-parse --show-toplevel)
TARGET_API ?= api.linode.com
TARGET_API_VERSION ?= v4beta

export PATH := $(ROOT_DIR)/bin:$(ROOT_DIR)/.devbox/nix/profile/default/bin:$(PATH)
runThisTest:
	@echo "PLease execute make -C ($ROOT_DIR)/Makefile _e2etest-infra to spin up infrastructure"
	make --no-print-directory _runThisTest

_runThisTest:
	@D="$$(mktemp -d)" ;\
	cp $(ROOT_DIR)/e2e/Makefile $$D ;\
	mkdir $$D/suit ;\
	cp -r $(PWD) $$D/suit ;\
	ROOT_DIR=$(ROOT_DIR) KUBECONFIG="$(ROOT_DIR)/kubeconfig" kubectl-kuttl test --timeout 300 --skip-delete "$$D/suit"

getKubeUid:
	@kubectl get -o jsonpath='{.metadata.uid}' -n "$$NAMESPACE" "$$OBJ"

patchKubeObj:
	@kubectl patch --type=merge -p "$$PATCH" -n "$$NAMESPACE" "$$OBJ"

patchKubeObjStatus:
	@kubectl patch --type=merge --subresource status -p "$$PATCH" -n "$$NAMESPACE" "$$OBJ"

callLinodeApiGet:
	@curl -s -H "Authorization: Bearer $$LINODE_TOKEN" -H "X-Filter: $$FILTER" -H "Content-Type: application/json" "https://$(TARGET_API)/$(TARGET_API_VERSION)/$$URI"

callLinodeApiPost:
	@curl -s -H "Authorization: Bearer $$LINODE_TOKEN" -H "Content-Type: application/json" -d "$$BODY" -X POST "https://$(TARGET_API)/$(TARGET_API_VERSION)/$$URI"
