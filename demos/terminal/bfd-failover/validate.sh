#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
run=/src/demos/terminal/bfd-failover/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

"${run}" prepare >/dev/null
"${run}" start >/dev/null
before_bfd=$("${run}" cli "show bfd sessions" 2>&1)
before_bgp=$("${run}" cli "show bgp peer list" 2>&1)
assert_contains "${before_bfd}" "up"
assert_contains "${before_bgp}" "established"

"${run}" cut >/dev/null
after_bfd=$("${run}" cli "show bfd sessions" 2>&1 || true)
after_bgp=$("${run}" cli "show bgp peer list" 2>&1 || true)
assert_not_contains "${after_bfd}" "up"
assert_not_contains "${after_bgp}" "established"

"${run}" restore >/dev/null
restored=$("${run}" cli "show bgp peer list" 2>&1)
assert_contains "${restored}" "established"
finish_validation bfd-failover
