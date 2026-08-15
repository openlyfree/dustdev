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

: "${GITHUB_REPOSITORY:?set GITHUB_REPOSITORY (e.g. ethan/code-az)}"
: "${GITHUB_TOKEN:?set GITHUB_TOKEN with read:packages scope to pull GHCR images}"

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
RUN_ID="$(
	curl -fsSL \
		-H "Authorization: Bearer ${GITHUB_TOKEN}" \
		-H "Accept: application/vnd.github+json" \
		"https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/workflows/cd.yml/runs?branch=${SHA}&status=success&per_page=1" |
		jq -r '.workflow_runs[0].id // empty'
)"
[[ -n "${RUN_ID}" ]] || {
	echo "error: no successful cd.yml run found for ${SHA}" >&2
	exit 1
}

curl -fsSL \
	-H "Authorization: Bearer ${GITHUB_TOKEN}" \
	-H "Accept: application/vnd.github+json" \
	"https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/runs/${RUN_ID}/artifacts/frontend/zip" \
	-o /tmp/frontend.zip

install -d -m 0755 "${FRONTEND_DIR}"
tmp_extract="$(mktemp -d)"
trap 'rm -rf "${tmp_extract}" /tmp/frontend.zip' EXIT
unzip -q -o /tmp/frontend.zip -d "${tmp_extract}"
tar -xzf "${tmp_extract}/frontend.tar.gz" -C "${FRONTEND_DIR}"

echo "==> Restarting services"
systemctl daemon-reload
systemctl restart frontbackend.service caddy.service

echo "==> Done. Verifying:"
systemctl --no-pager --full status frontbackend.service caddy.service | grep -E 'Loaded:|Active:' || true
echo "    https://${BASE_DOMAIN:-dustdev.app} should now serve the new build."
