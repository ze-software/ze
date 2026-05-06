#!/usr/bin/env bash
# Run the release gate from a clean source clone inside Docker.

set -euo pipefail

IMAGE="${ZE_CLEAN_VERIFY_IMAGE:-golang:1.25}"
PLATFORM="${ZE_CLEAN_VERIFY_PLATFORM:-linux/amd64}"

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "missing required command: $1" >&2
        exit 1
    fi
}

require_cmd docker
require_cmd git

ROOT="$(git rev-parse --show-toplevel)"
STATUS="$(git -C "$ROOT" status --porcelain)"
if [ -n "$STATUS" ]; then
    echo "error: clean release-candidate evidence requires a clean git worktree" >&2
    echo "commit, remove, or intentionally exclude these paths before running this target:" >&2
    git -C "$ROOT" status --short >&2
    exit 1
fi

docker run --rm \
    --privileged \
    --platform "$PLATFORM" \
    -v "$ROOT:/host:ro" \
    "$IMAGE" \
    bash -lc '
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y --no-install-recommends build-essential curl git iputils-ping iproute2 iptables nftables python3 python3-venv util-linux
curl -LsSf https://astral.sh/uv/install.sh | sh
export PATH="/go/bin:/usr/local/go/bin:$HOME/.local/bin:$PATH"
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1

git config --global --add safe.directory /host
git clone --no-local /host /work/src
cd /work/src
if [ -n "$(git status --porcelain)" ]; then
    echo "error: cloned source is not clean" >&2
    git status --short >&2
    exit 1
fi

ZE_SKIP_SUITES=firewall,web ZE_SUITE_TIMEOUT=1200s make ze-verify
'
