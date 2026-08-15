#!/usr/bin/env bash
# Deploy the latest CI-built artifacts to this droplet.
#
# Pulls the GHCR images referenced by the Quadlets, refreshes the static
# frontend bundle, and restarts the services. Running IDE containers are left
# alone; projects pick up the new idehost image the next time they start
# (pull=always on a mutable tag).
#
# Usage:
#   sudo ./deploy.sh                 # deploy what's on the default branch now
#   sudo ./deploy.sh <sha>           # pin frontend bundle to a commit's build
#
# Requires: podman, curl, jq, and GITHUB_TOKEN (a PAT with read:packages) set in
# the environment or /etc/dustdev/github.env.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# shellcheck disable=SC1091
[[ -f /etc/dustdev/github.env ]] && source /etc/dustdev/github.env

: "${GITHUB_REPOSITORY:?set GITHUB_REPOSITORY (e.g. openlyfree/dustdev)}"
: "${GITHUB_TOKEN:?set GITHUB_TOKEN with read:packages scope to pull GHCR images}"

# `gh` authenticates with GH_TOKEN, not GITHUB_TOKEN — export both so the
# GitHub CLI calls below work off the same PAT.
export GH_TOKEN="${GITHUB_TOKEN}"

# Lower-cased namespace must match CD's IMAGE_NAMESPACE (ghcr.io/owner/repo-*).
NAMESPACE="ghcr.io/$(echo "${GITHUB_REPOSITORY}" | tr '[:upper:]' '[:lower:]')"
SHA="${1:-master}"
TAG="production"
FRONTEND_DIR=/opt/dustdev/frontend

echo "==> Logging in to GHCR"
echo "${GITHUB_TOKEN}" | podman login ghcr.io -u "$(cut -d/ -f1 <<<"${GITHUB_REPOSITORY}")" --password-stdin

echo "==> Pulling images (${NAMESPACE}-*:latest)"
for img in frontbackend caddy idehost; do
	# CI pushes :production and :<sha>; mirror :production to :latest locally so
	# the Quadlets' Image= lines resolve without editing them.
	podman pull "${NAMESPACE}-${img}:${TAG}"
	podman tag "${NAMESPACE}-${img}:${TAG}" "${NAMESPACE}-${img}:latest"
done

echo "==> Fetching frontend bundle (artifact from ${SHA})"
# Resolve the cd.yml run to pull from. `gh run list -c` takes a branch or SHA
# and prints JSON, which sidesteps the REST runs endpoint's branch-only filter
# and avoids its pagination gotchas. `gh` reads GITHUB_TOKEN automatically.
RUN_ID="$(gh run list \
	--repo "${GITHUB_REPOSITORY}" \
	--workflow cd.yml \
	--commit "${SHA}" \
	--status success \
	--limit 1 \
	--json databaseId --jq '.[0].databaseId // empty')"
[[ -n "${RUN_ID}" ]] || {
	echo "error: no successful cd.yml run found for ${SHA}" >&2
	exit 1
}

tmp_extract="$(mktemp -d)"
trap 'rm -rf "${tmp_extract}"' EXIT
gh run download "${RUN_ID}" \
	--repo "${GITHUB_REPOSITORY}" \
	--name frontend \
	--dir "${tmp_extract}"

install -d -m 0755 "${FRONTEND_DIR}"
tar -xzf "${tmp_extract}/frontend.tar.gz" -C "${FRONTEND_DIR}"

echo "==> Restarting services"
systemctl daemon-reload
systemctl restart frontbackend.service caddy.service

echo "==> Done. Verifying:"
systemctl --no-pager --full status frontbackend.service caddy.service | grep -E 'Loaded:|Active:' || true
echo "    https://${BASE_DOMAIN:-dustdev.app} should now serve the new build."
