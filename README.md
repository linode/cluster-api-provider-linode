# Cluster API Provider Linode

[![Go Report Card](https://goreportcard.com/badge/github.com/linode/cluster-api-provider-linode)](https://goreportcard.com/report/github.com/linode/cluster-api-provider-linode)
[![Go Reference](https://pkg.go.dev/badge/github.com/linode/cluster-api-provider-linode.svg)](https://pkg.go.dev/github.com/linode/cluster-api-provider-linode)
[![Go Build and Test CI](https://github.com/linode/cluster-api-provider-linode/actions/workflows/go-test.yml/badge.svg)](https://github.com/linode/cluster-api-provider-linode/actions/workflows/go-test.yml)
[![CodeQL](https://github.com/linode/cluster-api-provider-linode/actions/workflows/codeql.yml/badge.svg)](https://github.com/linode/cluster-api-provider-linode/actions/workflows/codeql.yml)
[![Docker Image Build CI](https://github.com/linode/cluster-api-provider-linode/actions/workflows/build-docker-image.yml/badge.svg)](https://github.com/linode/cluster-api-provider-linode/actions/workflows/build-docker-image.yml)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://makeapullrequest.com)

------

## What is Cluster API Provider Linode (CAPL)

This is a [Cluster API](https://cluster-api.sigs.k8s.io/) implementation for [Linode](https://www.linode.com/)
to create, configure, and manage Kubernetes clusters.

------

## Compatibility

### Cluster API Versions
CAPL is compatible only with the `v1beta1` version of CAPI (v1.0.x).

### Kubernetes Versions

CAPL is able to install and manage the [versions of Kubernetes supported by the Cluster API (CAPI) project](https://cluster-api.sigs.k8s.io/reference/versions.html#supported-kubernetes-versions).

------

## Documentation

Please see our [Book](https://linode.github.io/cluster-api-provider-linode) for in-depth user documentation.

Additional docs can be found in the [/docs](/docs) directory.
