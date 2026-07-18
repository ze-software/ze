#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/traceroute/config
export ZE_SSH_PASSWORD=secret123
run=/src/demos/terminal/traceroute/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" start >/dev/null

output=$(ze cli -c 'show traceroute 192.0.2.53 | json' 2>&1)
assert_contains "${output}" "198.51.100.2"
assert_contains "${output}" "192.0.2.53"
finish_validation traceroute
