#!/usr/bin/env bash
# Read-only check that this machine can run CAPL's mise tasks.
# Exit 1 when a tool needed for check/verify is missing. Missing e2e or Linode settings only warn.
set -u

fail=0
warn=0

ok()   { printf '  ok    %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1"; fail=1; }
note() { printf '  warn  %s\n' "$1"; warn=1; }

have() { command -v "$1" >/dev/null 2>&1; }

echo "Tools for check and verify (installed by 'mise install')"
for tool in go golangci-lint govulncheck mockgen controller-gen setup-envtest crd-ref-docs kustomize; do
  if have "$tool"; then ok "$tool"; else bad "$tool not on PATH: run 'mise install'"; fi
done

echo "Tools for e2e and manual cluster work"
for tool in kubectl kind clusterctl ctlptl tilt chainsaw jq linode-cli; do
  if have "$tool"; then ok "$tool"; else note "$tool not on PATH: run 'mise install'"; fi
done
if have docker && docker info >/dev/null 2>&1; then
  ok "docker daemon reachable"
else
  note "docker daemon not reachable (needed for local-deploy and e2etest)"
fi

echo "Environment for e2e and manual cluster work"
for var in LINODE_TOKEN LINODE_REGION LINODE_MACHINE_TYPE LINODE_CONTROL_PLANE_MACHINE_TYPE; do
  if [ -n "${!var:-}" ]; then ok "$var is set"; else note "$var is not set"; fi
done
[ -n "${LINODE_URL:-}" ] && ok "LINODE_URL=${LINODE_URL}"

echo "Linode account (read-only profile lookup; confirm this is the account you intend to use)"
if have linode-cli; then
  token="${LINODE_CLI_TOKEN:-${LINODE_TOKEN:-}}"
  if [ -z "$token" ]; then
    note "no LINODE_CLI_TOKEN or LINODE_TOKEN in the environment; linode-cli will use its config file"
  fi
  if profile=$(LINODE_CLI_TOKEN="$token" linode-cli profile view --json 2>/dev/null); then
    user=$(printf '%s' "$profile" | jq -r '.[0].username // empty' 2>/dev/null)
    ok "authenticated as ${user:-unknown user}; API host: ${LINODE_CLI_API_HOST:-${LINODE_URL:-default}}"
  else
    note "linode-cli could not authenticate: select an account, then re-run"
  fi
else
  note "linode-cli missing: cannot verify the Linode account"
fi

echo
if [ "$fail" -ne 0 ]; then
  echo "doctor: FAILED. Fix the FAIL lines above before running check or verify."
  exit 1
fi
if [ "$warn" -ne 0 ]; then
  echo "doctor: check and verify are ready. Fix the warn lines before e2e or cluster work."
else
  echo "doctor: ready for check, verify, e2e, and cluster work."
fi
