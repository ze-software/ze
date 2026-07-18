#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

state=/src/tmp/terminal-demos/state/rib-fib
run=/src/demos/terminal/rib-fib/run.sh
prefix=198.51.100.0/24
export ZE_CONFIG_DIR="${state}/config"
export ZE_SSH_PASSWORD=secret123
export SSHPASS=secret123
export ZE_INIT_INPUT="${state}/init.input"

trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" prepare >/dev/null
ze init <"${ZE_INIT_INPUT}" >/dev/null
"${run}" start >/dev/null

inject=$(ze cli -c "request bgp rib inject 192.0.2.10 ipv4/unicast ${prefix} origin igp nexthop 127.0.0.1 med 42" 2>&1)
assert_not_contains "${inject}" "error"

best=
system_rib=
kernel=
for _ in {1..100}; do
    best=$(ze cli -c 'show bgp rib best | no-more' 2>&1 || true)
    system_rib=$(ze cli -c 'show rib | no-more' 2>&1 || true)
    kernel=$(ip -details route show exact "${prefix}" 2>&1 || true)
    if [[ "${best}" == *"${prefix}"* && "${system_rib}" == *"${prefix}"* && "${kernel}" == *"${prefix}"* ]]; then
        break
    fi
    sleep 0.1
done
assert_contains "${best}" "${prefix}"
assert_contains "${system_rib}" "${prefix}"
assert_contains "${kernel}" "${prefix}"
assert_contains "${kernel}" "proto 250"

withdraw=$(ze cli -c "request bgp rib withdraw 192.0.2.10 ipv4/unicast ${prefix}" 2>&1)
assert_not_contains "${withdraw}" "error"
for _ in {1..100}; do
    kernel=$(ip route show exact "${prefix}" 2>&1 || true)
    [[ -z "${kernel}" ]] && break
    sleep 0.1
done
[[ -z "${kernel}" ]] || {
    printf 'validation failed: kernel route survived withdrawal\n%s\n' "${kernel}" >&2
    exit 1
}
finish_validation rib-fib
