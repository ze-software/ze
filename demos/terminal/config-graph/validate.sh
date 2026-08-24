#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
config=/src/demos/terminal/config-graph/router.conf

validation=$(ze config validate "${config}" 2>&1)
assert_contains "${validation}" "configuration valid"
graph=$(ze config graph "${config}" 2>&1)
assert_contains "${graph}" '"nodes"'
assert_contains "${graph}" '"edges"'
assert_contains "${graph}" 'peer/upstream-a'
assert_contains "${graph}" 'peer/upstream-b'
assert_contains "${graph}" 'group/transit'
assert_contains "${graph}" '"kind": "inherits"'
peer_view=$(printf '%s\n' "${graph}" | ze pipe text | ze pipe match peer/upstream)
assert_contains "${peer_view}" 'peer/upstream-a'
assert_contains "${peer_view}" 'peer/upstream-b'
group_view=$(printf '%s\n' "${graph}" | ze pipe text | ze pipe match group/transit)
assert_contains "${group_view}" 'group/transit'
inherits_view=$(printf '%s\n' "${graph}" | ze pipe text | ze pipe match inherits)
assert_contains "${inherits_view}" 'peer/upstream-a'
assert_contains "${inherits_view}" 'peer/upstream-b'
assert_contains "${inherits_view}" 'inherits'
assert_contains "${inherits_view}" 'group/transit'
finish_validation config-graph
