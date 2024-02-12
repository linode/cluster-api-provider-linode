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

runTestSuit:
	@T="$$(KUBECONFIG="$$ROOT_DIR/kubeconfig" kubectl-kuttl test --timeout 300 --skip-delete --namespace "$$NAMESPACE" "$$TS" 2>&1)" ;\
	echo "$$T" |\
		grep -v harness.go |\
		grep -v ^=== |\
		grep -v "running without a 'kuttl-test.yaml" |\
		grep -v "creation of user-supplied namespace" |\
		grep -v "not match file name regexp" |\
		grep -v "kutt-test config testdirs is overridden" ;\
	echo "$$T" | \
		grep '^PASS' || \
			(echo "$$T" | grep harness.go && find $$TS -name *.yaml -exec cat {} \; && exit 1)

renderTestCase:
	@D="$$(mktemp -d)" ;\
	mkdir -p "$$D/case" ;\
	envsubst -i "$$TPL" -o "$$D/case/00-case.yaml" ;\
	printf "$$D\n"

renderManifest:
	@echo $(shell make --no-print-directory renderTestCase)/case/00-case.yaml

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
