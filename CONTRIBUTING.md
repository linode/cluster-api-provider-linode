# Contributing Guidelines

:+1::tada: First off, we appreciate you taking the time to contribute! THANK YOU! :tada::+1:

We put together the handy guide below to help you get support for your work. Read on!  

## I Just Want to Ask the Maintainers a Question

The [Linode Community](https://www.linode.com/community/questions/) is a great place to get additional support.

## How Do I Submit A (Good) Bug Report or Feature Request

Please open a [github issue](https://guides.github.com/features/issues/) to report bugs or suggest features.

When filing an issue or feature request, help us avoid duplication and redundant effort -- check existing open or recently closed issues first.

Detailed bug reports and requests are easier for us to work with. Please include the following in your issue:

* A reproducible test case or series of steps
* The version of our code being used
* Any modifications you've made, relevant to the bug
* Anything unusual about your environment or deployment
* Screenshots and code samples where illustrative and helpful

## How to Open a Pull Request

We follow the [fork and pull model](https://opensource.guide/how-to-contribute/#opening-a-pull-request) for open source contributions.

Tips for a faster merge:
 * address one feature or bug per pull request. 
 * large formatting changes make it hard for us to focus on your work.
 * follow language coding conventions.
 * make sure that tests pass.
 * make sure your commits are atomic, [addressing one change per commit](https://chris.beams.io/posts/git-commit/). 
 * add tests!

## Contributing a Patch

1. Fork the desired repo, develop and test your code changes.
    1. See the [Development Guide](https://linode.github.io/cluster-api-provider-linode/developers/development.html) for more instructions on setting up your environment and testing changes locally.
2. Submit a pull request.
    1. All PRs should be labeled with one of the following kinds
         - `/kind feature` for PRs related to adding new features/tests
         - `/kind bug` for PRs related to bug fixes and patches
         - `/kind api-change` for PRs related to adding, removing, or otherwise changing an API
         - `/kind cleanup` for PRs related to code refactoring and cleanup
         - `/kind deprecation` for PRs related to a feature/enhancement marked for deprecation.
         - `/kind design` for PRs related to design proposals
         - `/kind documentation` for PRs related to documentation
         - `/kind other` for PRs related to updating dependencies, minor changes or other
     2. All code changes must be covered by unit tests and E2E tests.
     3. All new features should come with user documentation.
3. Ensure that commit message(s) are be meaningful and commit history is readable.
5. All changes must be code reviewed. Refer to the following for code conventions and standards:
    - The official [Kubernetes developer guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
    - [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) identifies some common style mistakes when writing Go
    - [Uber's Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) promotes preferred code conventions
    - This repo's [golangci-lint](https://golangci-lint.run) [configuration](https://github.com/linode/cluster-api-provider-linode/blob/main/.golangci.yml), which runs on all PRs

In case you want to run our E2E tests locally, please refer to the [E2E Testing](https://linode.github.io/cluster-api-provider-linode/developers/development.html#e2e-testing) guide.

## Vulnerability Reporting

If you discover a potential security issue in this project we ask that you notify Linode Security via our [vulnerability reporting process](https://hackerone.com/linode). Please do **not** create a public github issue.

## Licensing

See the [LICENSE file](/LICENSE) for our project's licensing.
