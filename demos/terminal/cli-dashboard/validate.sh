#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/cli-dashboard/config
export ZE_SSH_PASSWORD=secret123
run=/src/demos/terminal/cli-dashboard/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" start >/dev/null

output=
for _ in {1..100}; do
    output=$(ze cli -c 'show bgp peer list | json' 2>&1 || true)
    if [[ "${output}" == *'"state": "established"'* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${output}" '"name": "edge-a"'
assert_contains "${output}" '"name": "edge-b"'
assert_contains "${output}" '"name": "transit-c"'
assert_contains "${output}" '"state": "established"'
finish_validation cli-dashboard
