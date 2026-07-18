#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
run=/src/demos/terminal/ospf-adjacency/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

"${run}" prepare >/dev/null
"${run}" start >/dev/null
neighbor=$("${run}" cli "show ospf neighbor detail" 2>&1)
database=$("${run}" cli "show ospf database router" 2>&1)
routes=$("${run}" cli "show ospf route" 2>&1)
assert_contains "${neighbor}" "full"
assert_contains "${neighbor}" "172.31.0.3"
assert_contains "${database}" "172.31.0.3"
assert_contains "${routes}" "10.255.0.3"
finish_validation ospf-adjacency
