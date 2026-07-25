#!/usr/bin/env bash
set -euo pipefail

demo_id=${ZE_DEMO_ID:-zefs-config}
config_sources=${ZE_DEMO_CONFIG_SOURCES:-/src/demos/terminal/zefs-config/ze.conf}
read -r -a config_source_list <<<"${config_sources}"
state="/src/tmp/terminal-demos/state/${demo_id}"
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/daemon.log"
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    if [[ -f "${pid_file}" ]]; then
        while read -r pid; do
            kill "${pid}" 2>/dev/null || true
        done < "${pid_file}"
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
    for config_source in "${config_source_list[@]}"; do
        cat "${config_source}" >>"${state}/active.conf"
    done
    ze config import --name ze.conf "${state}/active.conf" \
        >"${state}/import.log" 2>&1

    : >"${pid_file}"
    ze-test peer --mode sink --bind 127.0.0.2 --port 1179 --asn 65001 \
        >"${state}/peer.log" 2>&1 &
    echo "$!" >>"${pid_file}"
    ze start ze.conf >"${log_file}" 2>&1 &
    echo "$!" >>"${pid_file}"

    for _ in {1..100}; do
        if [[ -f "${log_file}" ]] && grep -q "SSH server listening" "${log_file}"; then
            return 0
        fi
        sleep 0.1
    done
    cat "${log_file}" >&2
    return 1
}

case "${1:-}" in
    prepare) prepare; echo "terminal demo prepared" ;;
    start) start; echo "terminal demo ready" ;;
    stop) stop; echo "terminal demo stopped" ;;
    *) echo "usage: $0 prepare|start|stop" >&2; exit 2 ;;
esac
