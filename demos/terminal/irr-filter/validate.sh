#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

state=/src/tmp/terminal-demos/state/irr-filter
run=/src/demos/terminal/irr-filter/run.sh
export ZE_CONFIG_DIR="${state}/config"
export ZE_SSH_PASSWORD=secret123
export SSHPASS=secret123
export ZE_INIT_INPUT="${state}/init.input"

trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" prepare >/dev/null
ze init <"${ZE_INIT_INPUT}" >/dev/null
"${run}" seed >/dev/null

before=$(ze config cat ze.conf)
assert_not_contains "${before}" 'bgp-filter-irr'
assert_not_contains "${before}" 'AS-TEST'

ze config set ze.conf plugin internal bgp-filter-irr use bgp-filter-irr >/dev/null
ze config set ze.conf bgp policy irr server 127.0.0.1:4343 >/dev/null
ze config set ze.conf bgp policy irr refresh-interval 3600 >/dev/null
ze config set ze.conf bgp peer customer-a session irr as-set AS-TEST >/dev/null
ze config set ze.conf bgp peer customer-a filter import bgp-filter-irr:65001 >/dev/null

configured=$(ze config cat ze.conf)
assert_contains "${configured}" 'bgp-filter-irr'
assert_contains "${configured}" 'AS-TEST'
assert_contains "${configured}" '127.0.0.1:4343'

"${run}" start >/dev/null

status=
for _ in {1..100}; do
    status=$(ze cli -c 'show bgp irr | no-more | yaml' 2>&1 || true)
    if [[ "${status}" == *'status: ok'* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${status}" 'AS-TEST'
assert_contains "${status}" 'ipv4-count: 3'
assert_contains "${status}" 'status: ok'
prefixes=$(ze cli -c 'show bgp irr prefix customer-a | no-more | yaml')
assert_contains "${prefixes}" '10.0.0.0/24'
assert_contains "${prefixes}" '2001:db8::/32'

allowed=$(ze cli -c 'show bgp irr check customer-a 10.0.0.0/24 | no-more | yaml')
assert_contains "${allowed}" 'accepted: true'
assert_contains "${allowed}" 'matched-entry: 10.0.0.0/24'

rejected=$(ze cli -c 'show bgp irr check customer-a 192.168.0.0/24 | no-more | yaml')
assert_contains "${rejected}" 'accepted: false'

"${run}" announce >/dev/null

routes=
for _ in {1..100}; do
    routes=$(ze cli -c 'show bgp adj-rib-in | no-more | yaml' 2>&1 || true)
    if [[ "${routes}" == *'10.0.0.0/24'* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${routes}" '10.0.0.0/24'
assert_not_contains "${routes}" '192.168.0.0/24'
finish_validation irr-filter
