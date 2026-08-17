#!/usr/bin/env bash
# Build the complete GitHub Pages artifact outside the source worktree.
# Forwards arguments to the underlying site generator, for example:
# ./update-website.sh --only cli
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
exec tools/build-site.py "$@"
