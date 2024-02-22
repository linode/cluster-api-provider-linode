# E2E Framework

The E2E framework uses Kuttl under the hood. Kuttl is a simple and most importantly static tool that handles applying manifests for running assertions in a namespace it generates for each test.

In order to dynamically generate manifests, Kuttl supports declaring a `TestStep` manifest which includes an option for running scripts. This allows us apply manifests with dynamic values from stdin.

Additionally, helper functions can be invoked in the scripts by creating a Makefile in each test folder that references `../../Makefile` (i.e. `e2e/Makefile`).

## Example TestStep

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: |-
      cat <<EOF | kubectl apply -n $NAMESPACE -f -
      apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
      kind: LinodeMachine
      metadata:
        ownerReferences:
        - apiVersion: cluster.x-k8s.io/v1beta1
          kind: Machine
          name: machine-sample
          uid: $(OBJ=machines/machine-sample make getKubeUid)
        name: linodemachine-sample
      spec:
        region: us-sea
        type: g5-nanode-1
      EOF
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
make _e2etest-infra # Only once per cluster!
(cd e2e/linodemachine-controller/minimal ; make runThisTest)
``` 
