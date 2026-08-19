#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
run=/src/demos/terminal/bfd-failover/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

"${run}" prepare >/dev/null
"${run}" start >/dev/null
# `raw` is the dispatcher's JSON, so every assertion below names a field and
# its value. `up` and `established` as bare substrings matched any field that
# spelled them, the transition history included.
before_bfd=$("${run}" cli "show bfd sessions | raw" 2>&1)
before_bgp=$("${run}" cli "show bgp peer list | raw" 2>&1)
assert_contains "${before_bfd}" '"peer": "172.30.0.3"'
assert_contains "${before_bfd}" '"state": "up"'
assert_contains "${before_bgp}" '"name": "edge-peer"'
assert_contains "${before_bgp}" '"state": "established"'

"${run}" cut >/dev/null
# No `|| true` here. It let a command that failed outright satisfy the two
# absence assertions that followed, so the whole failover half of this demo
# passed on an empty answer.
after_bfd=$("${run}" cli "show bfd sessions | raw" 2>&1)
after_bgp=$("${run}" cli "show bgp peer list | raw" 2>&1)
# BFD drops a session with its last client, so "no live BFD session" is the
# empty list, asserted by value rather than as the absence of a substring.
if [[ "${after_bfd}" != "[]" ]]; then
    printf 'validation failed: expected an empty BFD session list after the cut\n' >&2
    printf '%s\n' "${after_bfd}" >&2
    exit 1
fi
# The peer row must still be there. An empty answer is not a peer that left
# Established, and it is what the old assertion accepted.
assert_contains "${after_bgp}" '"name": "edge-peer"'
assert_not_contains "${after_bgp}" '"state": "established"'

"${run}" restore >/dev/null
restored=$("${run}" cli "show bgp peer list | raw" 2>&1)
assert_contains "${restored}" '"name": "edge-peer"'
assert_contains "${restored}" '"state": "established"'
finish_validation bfd-failover
