runTestCase:
	@T="$$(KUBECONFIG="$$ROOT_DIR/kubeconfig" kuttl test --timeout 300 --skip-delete --namespace "$$NAMESPACE" "$$TC" 2>&1)" ;\
	echo "$$T" |\
		grep -v harness.go |\
		grep -v ^=== |\
		grep -v "running without a 'kuttl-test.yaml" |\
		grep -v "creation of user-supplied namespace" |\
		grep -v "not match file name regexp" |\
		grep -v "kutt-test config testdirs is overridden" ;\
	echo "$$T" | \
		grep '^PASS' || \
			(echo "$$T" | grep harness.go && cat "$$TC/step/00-step.yaml" && exit 1)

renderTestCase:
	@D="$$(mktemp -d)" ;\
	mkdir -p "$$D/step" ;\
	envsubst -i "$$TPL" -o "$$D/step/00-step.yaml" ;\
	echo -n "$$D"

getKubeUid:
	@kubectl get -o jsonpath='{.metadata.uid}' -n "$$NAMESPACE" "$$OBJ"

callLinodeApiGet:
	@curl -s -H "Authorization: Bearer $$LINODE_TOKEN" -H "X-Filter: $$FILTER" "https://api.linode.com/v4beta/$$URI"
