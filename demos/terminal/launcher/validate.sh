#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

output=$(ze 2>&1 || true)
assert_contains "${output}" "show"
assert_contains "${output}" "doctor"
assert_contains "${output}" "Interactive"
finish_validation launcher
