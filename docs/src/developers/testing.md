# CAPL Testing

## Unit Tests
### Running Tests
In order to run the unit tests run the following command
```bash
make test
```

## E2E Tests
For e2e tests CAPL uses the [Chainsaw project](https://kyverno.github.io/chainsaw) which leverages `kind` and `tilt` to 
spin up a cluster with the CAPL controllers installed and then uses `chainsaw-test.yaml` files to drive e2e testing.

All test live in the e2e folder with a directory structure of `e2e/${CONTROLLER_NAME}/${TEST_NAME}`
### Running tests
In order to run e2e tests run the following command
```bash
make e2etest
```
### Adding tests
1. Create a new directory under the controller you are testing with the naming scheme of `e2e/${CONTROLLER_NAME}/${TEST_NAME}`
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