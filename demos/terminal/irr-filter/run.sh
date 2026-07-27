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

# wait_port blocks until a TCP listener accepts on host:port. 30 x 0.1s = 3s.
#
# Same class of bug as demos/terminal/rpki/run.sh, with a SIX times longer
# timer. ze resolves IRR data once at configure time via `go plug.initialResolve()`
# (internal/component/bgp/plugins/filter_irr/filter_irr.go:268); on failure the
# only further attempt is refreshLoop, a plain time.NewTicker(interval)
# (:414) with no fast retry and no backoff -- and this demo's tape sets
# `refresh-interval 3600`. So an IRR server a few milliseconds late to listen
# costs an HOUR, not a retry: `show bgp irr` stays empty, the tape's
# `Wait+Screen /AS-TEST/` blows its 30s VHS budget, and render.py aborts every
# remaining demo (check=True, no per-demo try) so the website publishes nothing.
# prepare() wipes ZE_CONFIG_DIR, so the loadFromStore() fallback (:255) is empty
# too. The budget is short on purpose: it is spent inside the tape's Wait.
wait_port() {
    local host="$1" port="$2" name="$3"
    for _ in {1..30}; do
        if (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; then
            return 0
        fi
        sleep 0.1
    done
    echo "timeout waiting for ${name} to listen on ${host}:${port}" >&2
    return 1
}

start() {

    : >"${pid_file}"
    ze-test irr --port 4343 >"${state}/irr.log" 2>&1 &
    echo "$!" >>"${pid_file}"

    # The IRR server must be accepting BEFORE ze configures (see wait_port).
    wait_port 127.0.0.1 4343 "irr server" || { cat "${state}/irr.log" >&2; return 1; }

    ze start ze.conf >"${log_file}" 2>&1 &
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
