#!/usr/bin/env bash
set -euo pipefail

state=/src/tmp/terminal-demos/state/web-config
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/daemon.log"
export ZE_CONFIG_DIR="${config_dir}"

stop() {
    if [[ -f "${pid_file}" ]]; then
        while read -r pid; do
            kill "${pid}" 2>/dev/null || true
        done < "${pid_file}"
    fi
    rm -f "${pid_file}"
}

start() {
    stop
    rm -rf "${state}"
    mkdir -p "${config_dir}"

    printf 'admin\nsecret123\n127.0.0.1\n2222\nze-demo\n' \
        | ze init >"${state}/init.log" 2>&1
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/web-config/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" \
        >"${state}/import.log" 2>&1

    : >"${pid_file}"
    ze start ze.conf >"${log_file}" 2>&1 &
    echo "$!" >>"${pid_file}"
    for _ in {1..150}; do
        if [[ -f "${log_file}" ]] && grep -q "web server listening" "${log_file}"; then
            return 0
        fi
        sleep 0.1
    done
    cat "${log_file}" >&2
    return 1
}

case "${1:-}" in
    start) start; echo "web demo ready" ;;
    stop) stop; echo "web demo stopped" ;;
    *) echo "usage: $0 start|stop" >&2; exit 2 ;;
esac
