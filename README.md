[![Go Report Card](https://goreportcard.com/badge/github.com/linode/cluster-api-provider-linode)](https://goreportcard.com/report/github.com/linode/cluster-api-provider-linode)
[![Go Reference](https://pkg.go.dev/badge/github.com/linode/cluster-api-provider-linode.svg)](https://pkg.go.dev/github.com/linode/cluster-api-provider-linode)
[![Go Build and Test CI](https://github.com/linode/cluster-api-provider-linode/actions/workflows/go-test.yml/badge.svg)](https://github.com/linode/cluster-api-provider-linode/actions/workflows/go-test.yml)
[![CodeQL](https://github.com/linode/cluster-api-provider-linode/actions/workflows/codeql.yml/badge.svg)](https://github.com/linode/cluster-api-provider-linode/actions/workflows/codeql.yml)
[![Docker Image Build CI](https://github.com/linode/cluster-api-provider-linode/actions/workflows/build-docker-image.yml/badge.svg)](https://github.com/linode/cluster-api-provider-linode/actions/workflows/build-docker-image.yml)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://makeapullrequest.com)



# Cluster API Provider Linode
A [Cluster API](https://cluster-api.sigs.k8s.io/) implementation for the [Linode](https://www.linode.com/) to create kubernetes clusters.

### Local development with Tilt

For local development execute the following `make` target:

```bash
LINODE_TOKEN=<YOUR LINODE TOKEN> make tilt-cluster
```

This command creates a Kind cluster, and deploys resources via Tilt. You can freely change the code and wait for Tilt to update provider.
