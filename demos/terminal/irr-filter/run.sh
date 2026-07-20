#!/usr/bin/env bash
set -euo pipefail

state=/src/tmp/terminal-demos/state/irr-filter
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/daemon.log"
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    if [[ -f "${pid_file}" ]]; then
        while read -r pid; do
            kill "${pid}" 2>/dev/null || true
        done <"${pid_file}"
    fi
    rm -f "${pid_file}"
}

prepare() {
    stop
    rm -rf "${state}"
    mkdir -p "${config_dir}"
    printf 'admin\nsecret123\n127.0.0.1\n2222\nze-demo\n' >"${state}/init.input"
    chmod 600 "${state}/init.input"
}

seed() {
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/irr-filter/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >"${state}/import.log" 2>&1
}

start() {

    : >"${pid_file}"
    ze-test irr --port 4343 >"${state}/irr.log" 2>&1 &
    echo "$!" >>"${pid_file}"
    ze ze.conf >"${log_file}" 2>&1 &
    echo "$!" >>"${pid_file}"

    for _ in {1..200}; do
        if [[ -f "${log_file}" ]] \
            && grep -q "SSH server listening" "${log_file}"; then
            return 0
        fi
        sleep 0.1
    done
    cat "${log_file}" >&2
    return 1
}

announce() {
    ze-test peer --mode sink --bind 127.0.0.2 --port 1179 --asn 65001 \
        /src/demos/terminal/irr-filter/routes.msg >"${state}/peer.log" 2>&1 &
    echo "$!" >>"${pid_file}"
}

case "${1:-}" in
    prepare) prepare; echo "IRR filter demo prepared" ;;
    seed) seed; echo "Base BGP configuration loaded without IRR filtering" ;;
    start) start; echo "IRR filter demo ready" ;;
    announce) announce; echo "Customer routes announced" ;;
    stop) stop; echo "IRR filter demo stopped" ;;
    *) echo "usage: $0 prepare|seed|start|announce|stop" >&2; exit 2 ;;
esac
