#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/network-lab.sh

state=/src/tmp/terminal-demos/state/traffic-anomaly
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/ze.log"
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    if [[ -f "${pid_file}" ]]; then
        while read -r pid; do kill "${pid}" 2>/dev/null || true; done <"${pid_file}"
    fi
    ip link del traffic0 2>/dev/null || true
    ip netns del traffic-peer 2>/dev/null || true
    rm -f "${pid_file}"
}

prepare() {
    stop
    rm -rf "${state}"
    mkdir -p "${config_dir}"
    printf 'admin\nsecret123\n127.0.0.1\n2222\nze-demo\n' >"${state}/init.input"
    chmod 600 "${state}/init.input"
    ze init <"${state}/init.input" >/dev/null
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/traffic-anomaly/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >/dev/null
    ip netns add traffic-peer
    ip link add traffic0 type veth peer name eth0 netns traffic-peer
    ip addr add 10.77.0.1/24 dev traffic0
    ip link set traffic0 up
    ip -n traffic-peer link set lo up
    ip -n traffic-peer link set eth0 up
    ip -n traffic-peer addr add 10.77.0.2/24 dev eth0
}

start() {
    : >"${pid_file}"
    python3 -m http.server 8080 --bind 10.77.0.1 --directory /src/demos/terminal/traffic-anomaly >"${state}/http.log" 2>&1 &
    echo "$!" >>"${pid_file}"
    ze ze.conf >"${log_file}" 2>&1 &
    echo "$!" >>"${pid_file}"
    for ((i = 0; i < 300; i++)); do
        if grep -q "SSH server listening" "${log_file}" 2>/dev/null && ! grep -q "traffic usage.*failed" "${log_file}" 2>/dev/null; then
            output=$(ze cli -c "show traffic usage name traffic0" 2>&1 || true)
            [[ "${output}" == *traffic0* ]] && { echo "Traffic monitor attached to traffic0"; return 0; }
        fi
        sleep 0.1
    done
    cat "${log_file}" >&2
    return 1
}

generate() {
    ip netns exec traffic-peer ping -q -c 8 10.77.0.1 >/dev/null
    for _ in {1..12}; do
        ip netns exec traffic-peer curl -fsS http://10.77.0.1:8080/payload.txt >/dev/null
    done
    sleep 1
    echo "Generated ICMP and HTTP burst from 10.77.0.2"
}

show() {
    capture_bounded ze cli -c "show traffic usage name traffic0 | no-more"
}

walkthrough() {
    local before after
    before=$(ze cli -c "show traffic usage name traffic0" 2>&1)
    [[ "${before}" == *traffic0* ]] || return 1
    echo '$ ze show traffic usage name traffic0'
    echo 'Interface traffic0 baseline: no counters'
    sleep 4
    generate
    after=$(ze cli -c "show traffic usage name traffic0" 2>&1)
    [[ "${after}" == *10.77.0.2* && "${after}" == *8080* && "${after}" == *icmp* ]] || return 1
    echo '$ ze show traffic usage name traffic0'
    echo 'Source 10.77.0.2: ICMP and TCP/8080 accounted'
    sleep 8
    stop
    echo "Traffic walkthrough complete"
}

case "${1:-}" in
    prepare) prepare; echo "Traffic lab prepared" ;;
    start) start ;;
    generate) generate ;;
    show) show ;;
    walkthrough) walkthrough ;;
    stop) stop; echo "Traffic lab stopped" ;;
    *) echo "usage: $0 prepare|start|generate|show|walkthrough|stop" >&2; exit 2 ;;
esac
