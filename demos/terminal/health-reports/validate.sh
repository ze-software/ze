#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

state=/src/tmp/terminal-demos/state/health-reports
run=/src/demos/terminal/health-reports/run.sh
export ZE_CONFIG_DIR="${state}/config"
export ZE_SSH_PASSWORD=secret123
export SSHPASS=secret123
export ZE_INIT_INPUT="${state}/init.input"

trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" prepare >/dev/null
ze init <"${ZE_INIT_INPUT}" >/dev/null
"${run}" start >/dev/null

warnings=
peers=
for _ in {1..100}; do
    warnings=$(ze cli -c 'show warnings source bgp | no-more' 2>&1 || true)
    peers=$(ze cli -c 'show bgp peer list | no-more' 2>&1 || true)
    if [[ "${warnings}" == *'prefix-stale'* && "${peers,,}" == *'established'* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${warnings}" 'prefix-stale'
assert_contains "${warnings}" '2024-01-01'
assert_contains "${warnings}" '127.0.0.2'

health=$(ze cli -c 'show health | no-more' 2>&1)
assert_contains "${health}" 'status: down'

teardown=$(ze cli -c 'request peer 127.0.0.2 teardown 4' 2>&1)
assert_not_contains "${teardown}" 'error'
errors=
for _ in {1..100}; do
    errors=$(ze cli -c 'show errors source bgp | no-more' 2>&1 || true)
    [[ "${errors}" == *'notification-sent'* ]] && break
    sleep 0.1
done
assert_contains "${errors}" 'notification-sent'
assert_contains "${errors}" 'direction: sent'
assert_contains "${errors}" 'subcode: 4'
finish_validation health-reports
