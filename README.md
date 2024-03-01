# Cluster API Provider Linode

<p align="center">
<!-- go doc / reference card -->
<a href="https://pkg.go.dev/github.com/linode/cluster-api-provider-linode">
<img src="https://pkg.go.dev/badge/github.com/linode/cluster-api-provider-linode.svg"></a>
<!-- goreportcard badge -->
<a href="https://goreportcard.com/report/github.com/linode/cluster-api-provider-linode">
<img src="https://goreportcard.com/badge/github.com/linode/cluster-api-provider-linode"></a>
<!-- join kubernetes slack channel for linode -->
<a href="https://kubernetes.slack.com/messages/CD4B15LUR">
<img src="https://img.shields.io/badge/join%20slack-%23linode-brightgreen"></a>
<!-- PRs welcome -->
<a href="http://makeapullrequest.com">
<img src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg"></a>
</p>
<p align="center">
<!-- go build / test CI -->
<a href="https://github.com/linode/cluster-api-provider-linode/actions/workflows/go-test.yml">
<img src="https://github.com/linode/cluster-api-provider-linode/actions/workflows/go-test.yml/badge.svg"></a>
<!-- docker build CI -->
<a href="https://github.com/linode/cluster-api-provider-linode/actions/workflows/build-docker-image.yml">
<img src="https://github.com/linode/cluster-api-provider-linode/actions/workflows/build-docker-image.yml/badge.svg"></a>
<!-- CodeQL -->
<a href="https://github.com/linode/cluster-api-provider-linode/actions/workflows/codeql.yml">
<img src="https://github.com/linode/cluster-api-provider-linode/actions/workflows/codeql.yml/badge.svg"></a>
</p>

------
*PLEASE NOTE*: This project is considered ALPHA quality and should NOT be used for production, as it is currently in active development. Use at your own risk. APIs, configuration file formats, and functionality are all subject to change frequently. That said, please try it out in your development and test environments and let us know if it works. Contributions welcome! Thanks!

------

## What is Cluster API Provider Linode (CAPL)

This is a [Cluster API](https://cluster-api.sigs.k8s.io/) implementation for [Linode](https://www.linode.com/)
to create, configure, and manage Kubernetes clusters.

------

## Compatibility

### Cluster API Versions
CAPL is compatible only with the `v1beta1` version of CAPI (v1.x).

### Kubernetes Versions

CAPL is able to install and manage the [versions of Kubernetes supported by the Cluster API (CAPI) project](https://cluster-api.sigs.k8s.io/reference/versions.html#supported-kubernetes-versions).

------

## Documentation

Please see our [Book](https://linode.github.io/cluster-api-provider-linode) for in-depth user and developer documentation.
