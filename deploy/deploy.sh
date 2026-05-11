#!/usr/bin/env bash
# Pull a specific (or latest) tag from GHCR and roll the api+worker
# services. Stays out of the "rebuild on host" path entirely — the
# only inputs are the tag and a quick `docker compose up -d`.
#
# Usage:
#   deploy/deploy.sh                  # latest :main (rolling)
#   deploy/deploy.sh sha-abc1234      # pin to a specific build
#   deploy/deploy.sh v1.2.3           # pin to a release tag
#
# Pre-reqs (one-time, on the deploy host):
#   1. Repo cloned to /root/convertpdfgo (or wherever; the script is
#      path-agnostic but expects `docker-compose.yml` in the cwd).
#   2. Either:
#        a) Make the GHCR packages public — open
#           https://github.com/users/<owner>/packages/container/convertpdfgo-api/settings
#           and the same for /convertpdfgo-worker; flip visibility to
#           "Public". No host-side credentials needed afterward.
#        b) Or `docker login ghcr.io` with a PAT that has read:packages.
#           Stash the credential under /root/.docker/config.json so it
#           survives reboots.
#
# Rollback: re-run with the previous sha-XXXXXXX tag. Every main push
# produces one, retention follows GHCR defaults (forever for public).

set -euo pipefail

TAG="${1:-main}"
OWNER="${GHCR_OWNER:-infosec554}"

API_IMAGE="ghcr.io/${OWNER}/convertpdfgo-api:${TAG}"
WORKER_IMAGE="ghcr.io/${OWNER}/convertpdfgo-worker:${TAG}"

echo "[deploy] api    → ${API_IMAGE}"
echo "[deploy] worker → ${WORKER_IMAGE}"

# Pre-pull surfaces auth / missing-tag errors before we kill the running
# containers — safer than letting `up` discover the failure mid-recreate.
docker pull "$API_IMAGE"
docker pull "$WORKER_IMAGE"

API_IMAGE="$API_IMAGE" WORKER_IMAGE="$WORKER_IMAGE" \
  docker compose --profile full up -d api worker

# Quick readiness gate. We expect api on :8080 (nginx points there);
# don't return success until /healthz actually answers.
for i in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:8080/healthz > /dev/null 2>&1; then
    echo "[deploy] api healthy after ${i}s — done"
    exit 0
  fi
  sleep 1
done

echo "[deploy] api did not pass /healthz in 60s — recent logs:" >&2
docker logs --tail 50 convertpdfgo_api >&2 || true
exit 1
