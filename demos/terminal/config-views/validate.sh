#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

root=/src/demos/terminal/config-views
state=/src/tmp/terminal-demos/state/config-views
"${root}/run.sh" prepare >/dev/null

tree=$(ze config show "${root}/router.conf" bgp peer transit-a)
assert_contains "${tree}" 'local ip 192.0.2.1'
assert_contains "${tree}" 'remote ip 192.0.2.2'

set_view=$(ze config migrate --format set "${root}/router.conf" 2>/dev/null | ze pipe match 'bgp peer transit-a')
assert_contains "${set_view}" 'set bgp peer transit-a connection local ip 192.0.2.1'
assert_contains "${set_view}" 'set bgp peer transit-a session asn remote 65001'

cmp -s "${state}/router.set" "${state}/roundtrip.set" || {
    printf 'validation failed: hierarchical/set round trip changed canonical output\n' >&2
    exit 1
}

matches=$(ze --plugins | ze pipe match flowspec)
assert_contains "${matches}" 'bgp-nlri-flowspec'
count=$(printf '%s\n' "${matches}" | ze pipe count)
[[ "${count}" =~ \"count\":[1-9][0-9]* ]] || {
    printf 'validation failed: expected positive FlowSpec plugin count, got %s\n' "${count}" >&2
    exit 1
}
finish_validation config-views
