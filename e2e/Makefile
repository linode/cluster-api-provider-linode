MAKEFLAGS += -s

runTestCase:
	@T="$$(kuttl test --skip-delete --namespace $(NAMESPACE) $$(make _renderTestCase))" ;\
	echo "$$T" | grep -s '0-step' ;\
	echo "$$T" | grep -e '^PASS'

_renderTestCase:
	@D=$$(mktemp -d) ;\
	mkdir -p $$D/step ;\
	envsubst -i $(TPL) -o $$D/step/00-step.yaml ;\
	echo -n $$D

getKubeUid:
	@kubectl get -o jsonpath='{.metadata.uid}' -n $(NAMESPACE) $R

callLinodeApiGet:
	@curl -s -H "Authorization: Bearer $(LINODE_TOKEN)" -H "X-Filter: $(F)" "https://api.linode.com/v4beta/$(U)"
