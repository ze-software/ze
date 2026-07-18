#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
run=/src/demos/terminal/vrrp-failover/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

"${run}" prepare >/dev/null
"${run}" start >/dev/null
show=$("${run}" show 2>&1)
owner_before=$("${run}" owner 2>&1)
assert_contains "${show}" "master"
assert_contains "${owner_before}" "VIP owner: Ze"
assert_contains "${owner_before}" "00:00:5e:00:01:0a"
owner_after=$("${run}" failover 2>&1)
assert_contains "${owner_after}" "VIP owner: keepalived"
assert_contains "${owner_after}" "00:00:5e:00:01:0a"
assert_contains "${owner_after}" "2/2 probes answered"
finish_validation vrrp-failover
