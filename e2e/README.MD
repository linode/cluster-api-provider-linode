# E2E Framework

The E2E framework uses Kuttl under the hood. Kuttl is a simple and most importantly static tool.
That means there is no way to dynamically generate manifests.
The solution is KIK (Kuttl-In-Kuttl)! 

## Kuttl-In-Kuttl

## How does it work

 - Instead of creating a static Kuttl manifest, you can create template. Template extension must be `.yml` otherwise Kuttl starts executing it.
 - Template is rendered via `envsubst`, so you can use Bash variables for dynamic data injection.
  - Variables always present
    - `${KUBECONFIG}`
    - `${NAMESPACE}`
    - `${ROOT_DIR}`

    Example template
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  ownerReferences:
  - apiVersion: cluster.x-k8s.io/v1beta1
    kind: Machine
    name: machine-sample
    uid: ${MACHINE_UID}
  name: linodemachine-sample
spec:
  region: us-sea
  type: g5-nanode-1
```

 - Helper tools are available, please create a Makefile in your test folder. In this way you can use built in helpers without including any script in the test, plus this file is a good place for test specific helpers.

    Example Makefile
```makefile
include ../../Makefile
```

 - Template is nothing without a runner, please use helpers provided in Makefile to render and run template.

    Example template runner
```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: |-
      MACHINE_UID="$(OBJ=machines/machine-sample make getKubeUid)" \
      TS="$(TPL="$PWD/02-create-linodemachine.tpl.yml" make renderTestCase)" make runTestSuit
```

  - You can run testsuits (which can contain template rendering).

    Example testsuit runner:
```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: TS="$PWD/testsuit" make runTestSuit
```

  - Test execution order is not guaranteed. If a test depends on another resource it is strongly suggested to create `assert` files to wait for an event. Almost every test depends on CAPI providers, so there is a good chance, you have to create a `00-assert.yaml` in your test folder.

    Example `00-assert.yaml`:
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
  name: cluster-api-provider-linode-controller-manager
  namespace: cluster-api-provider-linode-system
status:
  availableReplicas: 1
```

## Executing individual test suit

```bash
make _e2etest-infra # Only once per cluster!
(cd e2e/linodemachine-controller/minimal ; make runThisTest)
``` 
