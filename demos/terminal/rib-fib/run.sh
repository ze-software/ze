#!/usr/bin/env bash
set -euo pipefail

state=/src/tmp/terminal-demos/state/rib-fib
config_dir="${state}/config"
pid_file="${state}/daemon.pid"
log_file="${state}/daemon.log"
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    if [[ -f "${pid_file}" ]]; then
        kill "$(cat "${pid_file}")" 2>/dev/null || true
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

start() {
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/rib-fib/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >"${state}/import.log" 2>&1
    ze ze.conf >"${log_file}" 2>&1 &
    echo "$!" >"${pid_file}"

    for _ in {1..150}; do
        if [[ -f "${log_file}" ]] && grep -q "SSH server listening" "${log_file}"; then
            return 0
        fi
        sleep 0.1
    done
    cat "${log_file}" >&2
    return 1
}

case "${1:-}" in
    prepare) prepare; echo "RIB/FIB demo prepared" ;;
    start) start; echo "RIB/FIB demo ready" ;;
    stop) stop; echo "RIB/FIB demo stopped" ;;
    *) echo "usage: $0 prepare|start|stop" >&2; exit 2 ;;
esac
