#!/usr/bin/env bash
set -euo pipefail

state=/src/tmp/terminal-demos/state/traceroute
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/daemon.log"
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

cleanup_network() {
    ip route del 192.0.2.53/32 via 198.51.100.2 2>/dev/null || true
    ip link del ze-edge 2>/dev/null || true
    ip netns del edge 2>/dev/null || true
    ip netns del core 2>/dev/null || true
}

stop() {
    if [[ -f "${pid_file}" ]]; then
        while read -r pid; do
            kill "${pid}" 2>/dev/null || true
        done < "${pid_file}"
    fi
    rm -f "${pid_file}"
    cleanup_network
}

start_network() {
    cleanup_network
    ip netns add edge
    ip netns add core

    ip link add ze-edge type veth peer name edge-ze
    ip link set edge-ze netns edge
    ip address add 198.51.100.1/30 dev ze-edge
    ip link set ze-edge up
    ip -n edge address add 198.51.100.2/30 dev edge-ze
    ip -n edge link set edge-ze up
    ip -n edge link set lo up

    ip link add edge-core type veth peer name core-edge
    ip link set edge-core netns edge
    ip link set core-edge netns core
    ip -n edge address add 203.0.113.1/30 dev edge-core
    ip -n edge link set edge-core up
    ip -n core address add 203.0.113.2/30 dev core-edge
    ip -n core link set core-edge up
    ip -n core address add 192.0.2.53/32 dev lo
    ip -n core link set lo up

    ip netns exec edge sysctl -q -w net.ipv4.ip_forward=1
    ip netns exec core sysctl -q -w net.ipv4.ip_forward=1
    ip route add 192.0.2.53/32 via 198.51.100.2
    ip -n edge route add 192.0.2.53/32 via 203.0.113.2
    ip -n core route add 198.51.100.0/30 via 203.0.113.1

    if ! grep -q 'dns.demo' /etc/hosts; then
        printf '198.51.100.2 edge-gw.demo\n203.0.113.2 core-gw.demo\n192.0.2.53 dns.demo\n' \
            >>/etc/hosts
    fi
}

start() {
    cleanup_network
    rm -rf "${state}"
    mkdir -p "${config_dir}"
    start_network

    printf 'admin\nsecret123\n127.0.0.1\n2222\nze-demo\n' \
        | ze init >"${state}/init.log" 2>&1
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/traceroute/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" \
        >"${state}/import.log" 2>&1
    : >"${pid_file}"
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
    start) start; echo "terminal demo ready" ;;
    stop) stop; echo "terminal demo stopped" ;;
    *) echo "usage: $0 start|stop" >&2; exit 2 ;;
esac
