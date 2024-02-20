# CAPL Releases

## Release Cadence

CAPL currently has no set release cadence.

## Bug Fixes

Any significant user-facing bug fix that lands in the main branch should be
backported to the current and previous release lines.

## Versioning Scheme

CAPL follows the [semantic versionining](https://semver.org/#semantic-versioning-200) specification.

Example versions:

- Pre-release: `v0.1.1-alpha.1`
- Minor release: `v0.1.0`
- Patch release: `v0.1.1`
- Major release: `v1.0.0`

## Release Process

### Update metadata.yaml (skip for patch releases)

- Make sure [metadata.yaml](https://github.com/linode/cluster-api-provider-linode/blob/main/metadata.yaml)
is up to date and contains the new release with the correct Cluster API contract version.
  - If not, open a PR to add it.

### Create a release branch (skip for patch releases)

- Create a release branch off of `main` named
`release-$(MAJOR_VERSION).$(MINOR_VERSION)` (e.g. release-0.1)

### Create a tag for the release branch

- After ensuring all desired changes for the release are in the release branch,
create a tag following semantic versioning (e.g. v0.1.1)
- Ensure the [release workflow](https://github.com/linode/cluster-api-provider-linode/actions/workflows/release.yml)
succeeds for the created tag to build and push the Docker image and generate
the [release artifacts](#expected-artifacts).

### Release in GitHub

- Create a [new release](https://github.com/linode/cluster-api-provider-linode/releases/new).
  - Use the newly created tag
  - Make sure to click "Generate Release Notes"
  - Review the generated Release Notes and make any necessary changes.
  - If the tag is a pre-release, make sure to check the "Set as a pre-release box"

### Expected artifacts

- A `infrastructure-components.yaml` file containing the resources needed to deploy to Kubernetes
- A `cluster-templates.yaml` file for each supported flavor
- A `metadata.yaml` file which maps release series to the Cluster API contract version

### Communication

1. Announce the release in the Kubernetes Slack on the
[#linode](https://kubernetes.slack.com/messages/CD4B15LUR) channel
