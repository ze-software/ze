#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/zefs-config/config
export ZE_INIT_INPUT=/src/tmp/terminal-demos/state/zefs-config/init.input
export ZE_SSH_PASSWORD=secret123
export SSHPASS=secret123
run=/src/demos/terminal/zefs-config/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

"${run}" prepare >/dev/null
ze init <"${ZE_INIT_INPUT}" >/dev/null
assert_contains "$(ze config ls)" "ze.conf"
"${run}" start >/dev/null
before=$(ze cli -c 'show bgp summary' 2>&1)
assert_contains "${before}" "router-id"
assert_not_contains "${before}" "┌"
output=$(
    python3 /src/demos/terminal/pty-session.py \
        --command 'set environment cli format default table' \
        --command 'show | compare' \
        --command commit \
        --command '@wait Session committed' \
        --command exit \
        --command '@wait operational\]' \
        --command exit \
        -- sshpass -e ssh ze-demo
)
after=$(ze cli -c 'show bgp summary' 2>&1)
assert_contains "${output}" "default table"
assert_contains "${output}" "Session committed"
assert_contains "${output}" "router-id"
assert_contains "${after}" "summary:"
assert_contains "$(ze config cat ze.conf)" "default table"
finish_validation zefs-config
