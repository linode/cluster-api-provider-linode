# E2E Framework

The E2E framework uses Kuttl under the hood. Kuttl is a simple and most importantly static tool that handles applying manifests for running assertions in a namespace it generates for each test.


Helper functions can be invoked in the scripts by creating a Makefile in each test folder that references `../../Makefile` (i.e. `e2e/Makefile`) in order to do custom validation.

## Example TestStep

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: |-
      URI="nodebalancers" FILTER="{\"label\":\"cluster-test-api-server\"}" make callLinodeApiGet | grep 'results": 1'
```

## Test Execution

Note that test execution order is not guaranteed. If a test depends on another resource it is strongly suggested to create `assert` files to wait for an event. Almost every test depends on CAPI providers, so there is a good chance you have to create a `00-assert.yaml` in your test folder that asserts the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: capi-controller-manager
  namespace: capi-system
status:
  availableReplicas: 1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: capl-controller-manager
  namespace: capl-system
status:
  availableReplicas: 1
```

## Executing individual tests

```bash
make local-deploy # Only once per cluster!
(cd e2e/linodemachine-controller/minimal-linodemachine ; make runThisTest)
``` 
