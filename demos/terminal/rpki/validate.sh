#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

state=/src/tmp/terminal-demos/state/rpki
run=/src/demos/terminal/rpki/run.sh
export ZE_CONFIG_DIR="${state}/config"
export ZE_SSH_PASSWORD=secret123
export SSHPASS=secret123
export ZE_INIT_INPUT="${state}/init.input"

# Kept in step with run.sh: 256 first octets minus the 85 with `octet % 3 == 2`
# (internal/test/mock/rpki/rpki.go generateVRPs).
expected_vrp_ipv4=171

trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" prepare >/dev/null
ze init <"${ZE_INIT_INPUT}" >/dev/null
"${run}" start >/dev/null

# Gate on the VRP count, not on `sessions: 1`. `sessions` is the number of
# CONFIGURED cache servers and reads 1 the moment the config loads, including
# while the session sits in `state: idle` having never connected -- so the old
# loop broke on its first iteration and the vrp assertion below raced the sync
# instead of waiting for it. `run.sh start` already blocks on this same signal;
# the loop stays as a cheap confirmation rather than a wait.
status=
for _ in {1..100}; do
    status=$(ze cli -c 'show bgp rpki status | no-more' 2>&1 || true)
    if [[ "${status}" == *"vrp-count-ipv4: ${expected_vrp_ipv4}"* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${status}" 'sessions: 1'
assert_contains "${status}" "vrp-count-ipv4: ${expected_vrp_ipv4}"

routes=
for _ in {1..100}; do
    routes=$(ze cli -c 'show bgp adj-rib-in | no-more' 2>&1 || true)
    if [[ "${routes}" == *'9.43.0.0/24'* && "${routes}" == *'11.43.0.0/24'* && "${routes}" != *'10.43.0.0/24'* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${routes}" '9.43.0.0/24'
assert_contains "${routes}" '11.43.0.0/24'
assert_contains "${routes}" 'validation-state'
assert_not_contains "${routes}" '10.43.0.0/24'
finish_validation rpki
