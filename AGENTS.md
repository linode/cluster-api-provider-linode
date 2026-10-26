# Cluster API Provider Linode (CAPL)

Cluster API infrastructure provider for Linode/Akamai. Go, controller-runtime, API group version `v1alpha2` (`api/v1alpha2/`).

## Commands

IMPORTANT: Run build, test, lint, and verification only through `mise run <task>`. Do not call `go test`, `golangci-lint`, `make`, `chainsaw`, or other tools directly, and do not run a single package or test by hand, unless the user asks. If no task covers what you need, tell the user instead of improvising. List tasks with `mise tasks`; each description says what the task mutates.

| Need | Command | Cost |
|---|---|---|
| New environment | `mise install` | seconds |
| Quick feedback while editing | `mise run check` (fmt, vet, lint, lint-api) | seconds |
| Unit and envtest suites only | `mise run test` | about a minute |
| Full CI-equivalent gate before you call the work done | `mise run verify` | 1-3 minutes |
| Final check after `verify` passes (regenerates, builds local release, creates a kind cluster, then runs Chainsaw) | `mise run e2etest [selector]` | long-running |

- Mise installs and activates project-pinned dependencies for tasks; `mise install` sets up tools on a new environment. The `e2etest` task defaults its selector to `quick` and rejects a missing `LINODE_TOKEN`, `LINODE_REGION`, `LINODE_CONTROL_PLANE_MACHINE_TYPE`, or `LINODE_MACHINE_TYPE` before starting Make. Before E2E or cluster work, use `linode-context` and confirm the intended account/environment with the user. Having credentials does not authorize creating or deleting resources.
- For E2E, use the scripted `mise run e2etest [selector]` flow: it handles code generation, local release, kind/Tilt setup, clusterctl manifest generation/application, and Chainsaw readiness assertions. Do not manually inspect or pre-validate generated manifests, or add separate apply/readiness steps. Inspect a generated manifest only when absolutely necessary to diagnose a concrete failure or verify an intentional template, manifest, or CRD change. Never open, print, or expose the full raw manifest. Before viewing any part, use a YAML-aware filter that omits every Secret object entirely and redacts credential values from all remaining objects; then inspect only the specific non-Secret resource fields relevant to the failure. Never inspect a Secret object or its data, even in redacted form. If safe filtering cannot be guaranteed, do not inspect the manifest; use other diagnostics or request a sanitized excerpt.

- `test`, `build`, and `verify` regenerate code and reformat Go files in place. Review `git status` afterwards and keep regenerated files with your change; CI fails if they are stale. The tree may also hold the user's own uncommitted edits: leave those alone and attribute to your change only what you edited plus the generated paths listed under Code map.
- The suite in `internal/controller` needs envtest assets that only `mise run test` sets up. A bare `go test` there fails in `BeforeSuite` with a message pointing back to the task.
- `check-gen-diff` fails on any uncommitted change; it is a CI step. Use `verify` locally.
- Selector labels (`quick`, `all`, `flavors`, `linodemachine`, ...) are listed in `docs/src/developers/testing.md`. `mise run e2etest` defaults to `quick`; pass a selector argument or set `E2E_SELECTOR` to override it. Use the narrowest label for the area you changed, and use `all` only when you intend to run every E2E test. E2E needs `LINODE_TOKEN` and the environment in `docs/src/developers/development.md`. Tasks marked destructive or "run only when asked" in `mise tasks` need an explicit user request.
- Before running a selector other than `quick`, read the selector-specific environment inputs in `docs/src/developers/testing.md`; these inputs are not validated by the Mise task.

## What to run for each change

| Changed | Run |
|---|---|
| Go code | `check` while iterating, then `verify` |
| `api/v1alpha2/` types or markers | `verify` (regenerates CRDs, deepcopy, mocks, API reference docs; keep the generated output) |
| `clients/clients.go` interface | `verify` (regenerates `mock/client.go`) |
| `templates/` or `config/` | `mise run local-release` (it runs `git restore` on `config/default/*manager_image_patch.yaml`, so do not run it over uncommitted edits to those files), then `verify` |
| `e2e/` Chainsaw tests | `verify`, then `e2etest` with the matching label |
| Controller or webhook | `verify`, then `e2etest` with that component's label (see E2E labels) |
| Markdown only | no task |

## E2E labels

Label for each component's tests; the full list is in `docs/src/developers/testing.md`. If your change spans components, run each label.

| Changed | `E2E_SELECTOR` |
|---|---|
| `linodecluster-controller` | `linodecluster` |
| `linodemachine-controller`, `linodemachinetemplate-controller` | `linodemachine` |
| `linodevpc-controller` | `linodevpc` |
| `linodefirewall-controller` | `linodefirewall` |
| `linodeplacementgroup-controller` | `linodeplacementgroup` |
| `linodeobjectstoragebucket-controller` | `linodeobj` |
| `linodeobjectstoragekey-controller` | `linodeobjkey` |
| `admission-webhooks` | `webhook` |
| Templates or flavors | `flavors` |

## Tests for your change

Decide whether the change needs a unit test, an e2e test, or neither, and say which in your report. A behavior change or bug fix needs a test that fails without it; a rename, comment, or docs change does not. For a bug report, first reproduce it with a failing test or a stack trace. If the code does not show the reported failure, say so and ask the user for a trace or repro instead of guessing a fix.

Before writing one, read the existing tests for the code you touched (`*_test.go` beside it; `e2e/<component>/`). Extend them first:

- Add a case to the existing `mocktest` path, table, or Ginkgo spec that already exercises the function.
- Add a new test function or e2e directory only when the behavior has no existing test to extend, or extending would make an existing test cover two unrelated things.
- Do not copy a test to change one input.
- For e2e, extend the existing `e2e/<component>/<test>/` when the behavior is a variant of its happy path. Add a sibling directory for a failure or negative case that needs its own resources and assertions. Webhook rejections go in `e2e/admission-webhooks/validating`.
- CEL rules such as `self == oldSelf` are evaluated only by a real API server, not by the webhook unit tests. No existing Go test covers one: use the envtest suite in `internal/controller` (it loads `config/crd/bases`) and tell the user you are adding the first such case.

Conventions and the `mocktest` DSL: `docs/src/developers/testing.md`. Run the result with `mise run test`.

## Manual verification on a cluster

Do this only when the user explicitly asks. Never start it on your own, even after `verify` passes. Before running anything, read the "Developing" section of `docs/src/developers/development.md` (local release, Tilt, deploying and cleaning up a workload cluster), and confirm the Linode account with the user. Use a very short cluster name (less than 20 chars), report the exact manifest you applied and what you observed, and leave the cluster running until the user asks you to delete it.

## Definition of done

1. `mise run verify` exits 0 on the final state of the tree, after your last edit.
2. Behavior changes have a unit or e2e test, extended from an existing one where possible (see Tests for your change).
3. For changes to controllers, webhooks, templates, or `e2e/`, `mise run e2etest <label>` passes as the last step, after `verify`. Confirm with the user before starting it unless they asked for it.
4. Report the commands you ran and their results. If a task fails or you did not run it, say so.
5. PR titles and description follow `.github/PULL_REQUEST_TEMPLATE.md`.

## Code map

| Path | Role | Start from |
|---|---|---|
| `api/v1alpha2/` | CRD types and kubebuilder markers | `linodevpc_types.go` |
| `internal/controller/` | Reconcilers | `linodevpc_controller.go` |
| `cloud/scope/` | Per-reconcile Kubernetes + Linode clients and patch helper | `vpc.go` |
| `cloud/services/` | Linode API logic that takes a scope | existing service files |
| `internal/webhook/v1alpha2/` | Validation webhooks | existing webhook files |
| `clients/`, `mock/`, `mock/mocktest/` | Linode client interfaces, generated gomock clients, test DSL | `docs/src/developers/testing.md` |
| `templates/flavors/` | Kustomize sources for cluster templates (outputs are git-ignored) | `hack/generate-flavors.sh` |
| `e2e/<component>/<test>/` | Chainsaw tests | `docs/src/developers/testing.md` |

Never hand-edit generated files: `zz_generated.*`, `mock/client.go`, `config/crd/bases/`, `docs/src/reference/`. Change the source and run `verify`.

## Conventions

- Controllers get the Kubernetes and Linode clients from their scope (`scope.LinodeClient`), never by building their own; reusable Linode API logic goes in `cloud/services`. Lint enforces the construction half (`linodego.NewClient` only in `cloud/scope`, plus the webhook helper); the rest is by review. This keeps each package unit testable with mocks.
- Resources accept both a direct ID and an object reference (`vpcID` / `vpcRef`) and support `credentialsRef` for multi-tenancy.
- Immutable fields use `+kubebuilder:validation:XValidation:rule="self == oldSelf"`.
- Status carries `ready`, `failureReason`, `failureMessage`, and `conditions`.
- Add finalizers early in reconcile and remove them only after cleanup. Treat a Linode 404 on delete as success (`util.IgnoreLinodeAPIError`).
- Unit tests use the `mock/mocktest` DSL as described in `docs/src/developers/testing.md`.

## Enforcing a convention with lint

Prose conventions drift; a lint rule fails `check` and `verify` instead. If you break a convention above, or the user wants one enforced, propose a rule to the user and add it only after they agree.

1. Pick a convention that is checkable by name or import (for example, who may call `linodego.NewClient`). If the code already violates it widely, say so instead of adding a noisy rule.
2. Add it in `.golangci.yml`: `forbidigo` for forbidden calls, `depguard` for forbidden imports. Put the fix in the rule's `msg`. Exempt the owning package and any intentional exception under `linters.exclusions.rules`, with a comment saying why.
3. Prove it with `mise run lint`: it must pass on the current tree, then fail on a temporary violation that no other linter flags on the same line (golangci-lint reports one issue per line). Delete the temporary file and run `mise run lint` again.
4. Run `mise run verify`, then edit the convention's line in Conventions to say lint enforces it and which exceptions exist.

## Read before you start

| Task | Read |
|---|---|
| Add a new resource | Follow an existing one end to end: types, controller, scope, webhook, services, then `verify`, then a test under `e2e/<resource>-controller/`. Layout: `docs/src/developers/development.md` (Code Overview) |
| Set up local dev, Tilt, env vars, deploy a workload cluster | `docs/src/developers/development.md` |
| Write unit or e2e tests, pick an e2e label | `docs/src/developers/testing.md` |
| Change a user-facing feature (VPC, firewall, DNS, backups, ...) | `docs/src/topics/` |
| Debug a cluster | `docs/src/topics/troubleshooting.md` |
| Cut a release | `docs/src/developers/releasing.md` |
| Contribution rules | `CONTRIBUTING.md` |
