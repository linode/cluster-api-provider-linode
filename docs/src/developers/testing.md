# CAPL Testing

<!-- TOC depthFrom:2 -->

- [Unit Tests](#unit-tests)
  - [Executing Tests](#executing-tests)
  - [Creating Tests](#creating-tests)
- [E2E Tests](#e2e-tests)
  - [Running Tests](#running-tests)
  - [Adding Tests](#adding-tests)

<!-- /TOC -->

## Unit Tests
### Executing Tests
In order to run the unit tests run the following command
```bash
make test
```
### Creating Tests
General unit tests of functions follow the same conventions for testing using Go's `testing` standard library, along with the [testify](https://github.com/stretchr/testify) toolkit for making assertions.

Unit tests that require API clients use mock clients generated using [gomock](https://github.com/uber-go/mock). To simplify the usage of mock clients, this repo also uses an internal library defined in `mock/mocktest`.

`mocktest` is usually imported as a dot import along with the `mock` package:

```go
import (
  "github.com/linode/cluster-api-provider-linode/mock"

  . "github.com/linode/cluster-api-provider-linode/mock/mocktest"
)
```

Using `mocktest` involves creating a test suite that specifies the mock clients to be used within each test scope and running the test suite using a DSL for defnining test nodes belong to one or more test paths.

#### Example
The following is a contrived example using the mock Linode machine client.

Let's say we've written an idempotent function `EnsureInstanceRuns` that 1) gets an instance or creates it if it doesn't exist, 2) boots the instance if it's offline. Testing this function would mean we'd need to write test cases for all permutations, i.e.
* instance exists and is not offline
* instance exists but is offline, and is able to boot
* instance exists but is offline, and is not able to boot
* instance does not exist, and is not able to be created
* instance does not exist, and is able to be created, and is able to boot
* instance does not exist, and is able to be created, and is not able to boot

While writing test cases for each scenario, we'd likely find a lot of overlap between each. `mocktest` provides a DSL for defining each unique test case without needing to spell out all required mock client calls for each case. Here's how we could test `EnsureInstanceRuns` using `mocktest`:

```go
func TestEnsureInstanceNotOffline(t *testing.T) {
  suite := NewSuite(t, mock.MockLinodeMachineClient{})
  
  suite.Run(
    OneOf(
      Path(
        Call("instance exists and is not offline", func(ctx context.Context, mck Mock) {
          mck.MachineClient.EXPECT().GetInstance(ctx, /* ... */).Return(&linodego.Instance{Status: linodego.InstanceRunning}, nil)
        }),
        Result("success", func(ctx context.Context, mck Mock) {
          inst, err := EnsureInstanceNotOffline(ctx, /* ... */)
          require.NoError(t, err)
          assert.Equal(t, inst.Status, linodego.InstanceRunning)
        })
      ),
      Path(
        Call("instance does not exist", func(ctx context.Context, mck Mock) {
          mck.MachineClient.EXPECT().GetInstance(ctx, /* ... */).Return(nil, linodego.Error{Code: 404})
        }),
        OneOf(
          Path(Call("able to be created", func(ctx context.Context, mck Mock) {
            mck.MachineClient.EXPECT().CreateInstance(ctx, /* ... */).Return(&linodego.Instance{Status: linodego.InstanceOffline}, nil)
          })),
          Path(
            Call("not able to be created", func(ctx context.Context, mck Mock) {/* ... */})
            Result("error", func(ctx context.Context, mck Mock) {
              inst, err := EnsureInstanceNotOffline(ctx, /* ... */)
              require.ErrorContains(t, err, "instance was not booted: failed to create instance: reasons...")
              assert.Empty(inst)
            }),
          )
        ),
      ),
      Path(Call("instance exists but is offline", func(ctx context.Context, mck Mock) {
        mck.MachineClient.EXPECT().GetInstance(ctx, /* ... */).Return(&linodego.Instance{Status: linodego.InstanceOffline}, nil)
      })),
    ),
    OneOf(
      Path(
        Call("able to boot", func(ctx context.Context, mck Mock) {/*  */})
        Result("success", func(ctx context.Context, mck Mock) {
          inst, err := EnsureInstanceNotOffline(ctx, /* ... */)
          require.NoError(t, err)
          assert.Equal(t, inst.Status, linodego.InstanceBooting)
        })
      ),
      Path(
        Call("not able to boot", func(ctx context.Context, mck Mock) {/* returns API error */})
        Result("error", func(ctx context.Context, mck Mock) {
          inst, err := EnsureInstanceNotOffline(/* options */)
          require.ErrorContains(t, err, "instance was not booted: boot failed: reasons...")
          assert.Empty(inst)
        })
      )
    ),
  )
}
```
In this example, the nodes passed into `Run` are used to describe each permutation of the function being called with different results from the mock Linode machine client.

#### Nodes
* `Call` describes the behavior of method calls by mock clients. A `Call` node can belong to one or more paths.
* `Result` invokes the function with mock clients and tests the output. A `Result` node terminates each path it belongs to.
* `OneOf` is a collection of diverging paths that will be evaluated in separate test cases.
* `Path` is a collection of nodes that all belong to the same test path. Each child node of a `Path` is evaluated in order. Note that `Path` is only needed for logically grouping and isolating nodes within different test cases in a `OneOf` node.

#### Setup, tear down, and event triggers
Setup and tear down nodes can be scheduled before and after each run. `suite.BeforeEach` receives a `func(context.Context, Mock)` function that will run before each path is evaluated. Likewise, `suite.AfterEach` will run after each path is evaluated.

In addition to the path nodes listed in the section above, a special node type `Once` may be specified to inject a function that will only be evaluated one time across all paths. It can be used to trigger side effects outside of mock client behavior that can impact the output of the function being tested.

#### Control flow
When `Run` is called on a test suite, paths are evaluated in parallel using `t.Parallel()`. Each path will be run with a separate `t.Run` call, and each test run will be named according to the descriptions specified in each node.

To help with visualizing the paths that will be rendered from nodes, a `DescribePaths` helper function can be called which returns a slice of strings describing each path. For instance, the following shows the output of `DescribePaths` on the paths described in the example above:

```go
DescribePaths(/* nodes... */) /* [
  "instance exists and is not offline > success",
  "instance does not exist > not able to be created > error",
  "instance does not exist > able to be created > able to boot > success",
  "instance does not exist > able to be created > not able to boot > error",
  "instance exists but is offline > able to boot > success",
  "instance exists but is offline > not able to boot > error"
] */
```

#### Testing controllers
CAPL uses controller-runtime's [envtest](https://book.kubebuilder.io/reference/envtest) package which runs an instance of etcd and the Kubernetes API server for testing controllers. The test setup uses [ginkgo](https://onsi.github.io/ginkgo/) as its test runner as well as [gomega](https://onsi.github.io/gomega/) for assertions.

`mocktest` is also recommended when writing tests for controllers. The following is another contrived example of how to use its controller suite:

```go
var _ = Describe("linode creation", func() {
  // Create a mocktest controller suite.
  suite := NewControllerSuite(GinkgoT(), mock.MockLinodeMachineClient{})

  obj := infrav1alpha1.LinodeMachine{
    ObjectMeta: metav1.ObjectMeta{/* ... */}
    Spec: infrav1alpha1.LinodeMachineSpec{/* ... */}
  }

  suite.Run(
    Once("create resource", func(ctx context.Context, _ Mock) {
      // Use the EnvTest k8sClient to create the resource in the test server
      Expect(k8sClient.Create(ctx, &obj).To(Succeed()))
    }),
    Call("create a linode", func(ctx context.Context, mck Mock) {
      mck.MachineClient.CreateInstance(ctx, gomock.Any(), gomock.Any()).Return(&linodego.Instance{/* ... */}, nil)
    }),
    Result("update the resource status after linode creation", func(ctx context.Context, mck Mock) {
      reconciler := LinodeMachineReconciler{
        // Configure the reconciler to use the mock client for this test path
        LinodeClient: mck.MachineClient,
        // Use a managed recorder for capturing events published during this test
        Recorder: mck.Recorder(),
        // Use a managed logger for capturing logs written during the test
        // Note: This isn't a real struct field in LinodeMachineReconciler. A logger is configured elsewhere.
        Logger: mck.Logger(),
      }

      _, err := reconciler.Reconcile(ctx, reconcile.Request{/* ... */})
      Expect(err).NotTo(HaveOccurred())
      
      // Fetch the updated object in the test server and confirm it was updated
      Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj))).To(Succeed())
      Expect(obj.Status.Ready).To(BeTrue())

      // Check for expected events and logs
      Expect(mck.Events()).To(ContainSubstring("Linode created!"))
      Expect(mck.Logs()).To(ContainSubstring("Linode created!"))
    }),
  )
})
```

## E2E Tests
For e2e tests CAPL uses the [Chainsaw project](https://kyverno.github.io/chainsaw) which leverages `kind` and `tilt` to 
spin up a cluster with the CAPL controllers installed and then uses `chainsaw-test.yaml` files to drive e2e testing.

All test live in the e2e folder with a directory structure of `e2e/${COMPONENT}/${TEST_NAME}`
### Running Tests
In order to run e2e tests run the following commands: 
```bash
# Required env vars to run e2e tests
export INSTALL_K3S_PROVIDER=true
export INSTALL_RKE2_PROVIDER=true
export LINODE_REGION=us-sea
export LINODE_CONTROL_PLANE_MACHINE_TYPE=g6-standard-2
export LINODE_MACHINE_TYPE=g6-standard-2

# IMPORTANT: Set linode, k3s, and rke2 providers in this config file.
# Find an example at e2e/gha-clusterctl-config.yaml
export CLUSTERCTL_CONFIG=~/.cluster-api/clusterctl.yaml

make e2etest
```
*Note: By default `make e2etest` runs all the e2e tests defined under `/e2e` dir*

In order to run specific test, you need to pass flags to chainsaw by setting env var `E2E_SELECTOR`

Additional settings can be passed to chainsaw by setting env var `E2E_FLAGS`

Example: Only running e2e tests for flavors *(default, k3s, rke2)*
```bash
make e2etest E2E_SELECTOR='flavors' E2E_FLAGS='--assert-timeout 10m0s'
```
*Note: We need to bump up the assert timeout to 10 mins to allow the cluster to complete building and become available*

There are other selectors you can use to invoke specfic tests. Please look at the table below for all the selectors available:

| Tests                            | Selector          |
|----------------------------------|-------------------|
| All Tests                        | `all`             |
| All Controllers                  | `quick`           |
| All Flavors (default, k3s, rke2) | `flavors`         |
| K3S Cluster                      | `k3s`             | 
| RKE2 Cluster                     | `rke2`            |
| Default (kubeadm) Cluster        | `default-cluster` |
| Linode Cluster Controller        | `linodecluster`   |
| Linode Machine Controller        | `linodemachine`   |
| Linode Obj Controller            | `linodeobj`       | 
| Linode VPC Controller            | `linodevpc`       | 

*Note: For any flavor e2e tests, please set the required env variables*

### Adding Tests
1. Create a new directory under the controller you are testing with the naming scheme of `e2e/${COMPONENT}/${TEST_NAME}`
2. Create a minimal `chainsaw-test.yaml` file in the new test dir
    ```yaml
   # yaml-language-server: $schema=https://raw.githubusercontent.com/kyverno/chainsaw/main/.schemas/json/test-chainsaw-v1alpha1.json
    apiVersion: chainsaw.kyverno.io/v1alpha1
    kind: Test
    metadata:
      name: $TEST_NAME
    spec:
      template: true # set to true if you are going to use any chainsaw templating
      steps:
      - name: step-01
        try:
        - apply:
            file: ${resource_name}.yaml
        - assert:
            file: 01-assert.yaml
    ```
3. Add any resources to create or assert on in the same directory