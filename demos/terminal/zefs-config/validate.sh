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
# The committed default now decides the answer. This used to read `summary:`,
# a YAML key, which passed before and after the commit because `ze cli -c`
# answered YAML whatever was configured -- the assertion pinned the defect it
# was meant to catch. The box-drawing rule is the table, and line 18 asserts
# its absence before the commit, so the pair proves the setting took effect.
assert_contains "${after}" "┌"
assert_contains "$(ze config cat ze.conf)" "default table"

# An operator's own format operator outranks the committed default, and `raw`
# answers the dispatcher JSON that `ze pipe` formats on the shell side. Both
# are shown in the recording, so both are checked here.
assert_not_contains "$(ze cli -c 'show bgp summary | text' 2>&1)" "┌"
assert_contains "$(ze cli -c 'show bgp summary | raw' 2>&1)" '"router-id"'
finish_validation zefs-config
