#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
run=/src/demos/terminal/traffic-anomaly/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

"${run}" prepare >/dev/null
"${run}" start >/dev/null
before=$("${run}" show 2>&1)
assert_contains "${before}" "traffic0"
"${run}" generate >/dev/null
after=$("${run}" show 2>&1)
assert_contains "${after}" "traffic0"
assert_contains "${after}" "10.77.0.2"
assert_contains "${after}" "8080"
assert_contains "${after}" "icmp"
finish_validation traffic-anomaly
