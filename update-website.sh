#!/usr/bin/env bash
# Regenerate the entire gh-pages site. Thin wrapper around tools/build.py so
# there's a single obvious command at the repo root -- see AI.md for what
# each step does. Forwards all arguments, e.g. ./update-website.sh --only cli
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
exec tools/build.py "$@"
