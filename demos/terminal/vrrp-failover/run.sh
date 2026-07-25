#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/network-lab.sh

state=/src/tmp/terminal-demos/state/vrrp-failover
config_dir="${state}/config"
ze_pid="${state}/ze.pid"
ka_pid="${state}/keepalived.pid"
log_file="${state}/ze.log"
lab=vrrp
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    [[ -f "${ze_pid}" ]] && kill "$(cat "${ze_pid}")" 2>/dev/null || true
    [[ -f "${ka_pid}" ]] && kill "$(cat "${ka_pid}")" 2>/dev/null || true
    lab_cleanup "${lab}"
    rm -f "${ze_pid}" "${ka_pid}"
}

prepare() {
    stop
    rm -rf "${state}"
    mkdir -p "${config_dir}"
    printf 'admin\nsecret123\n127.0.0.1\n2222\nze-demo\n' >"${state}/init.input"
    chmod 600 "${state}/init.input"
    ze init <"${state}/init.input" >/dev/null
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/vrrp-failover/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >/dev/null
    lab_create_pair "${lab}" 192.0.2.251/24 192.0.2.252/24
    ip addr add 192.0.2.253/24 dev "${lab}-br"
}

start() {
    ip netns exec "${lab}-ze" env ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 ze start ze.conf >"${log_file}" 2>&1 &
    echo "$!" >"${ze_pid}"
    wait_for_text "${log_file}" "SSH server listening"
    wait_for_command 450 master show_ze >/dev/null
    ip netns exec "${lab}-peer" keepalived --dont-fork --log-console --use-file /src/demos/terminal/vrrp-failover/keepalived.conf >"${state}/keepalived.log" 2>&1 &
    echo "$!" >"${ka_pid}"
    sleep 5
    ip netns exec "${lab}-ze" env ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 ze cli -c "show vrrp" >/dev/null
    echo "Ze is master; keepalived is backup"
}

show_ze() {
    capture_bounded ip netns exec "${lab}-ze" env \
        ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 \
        ze cli -c "show vrrp | no-more"
}

owner() {
    if ip -n "${lab}-ze" -o addr show 2>/dev/null | grep -q '192.0.2.1/24'; then
        dev=$(ip -n "${lab}-ze" -o addr show | awk '$4 == "192.0.2.1/24" {print $2; exit}')
        mac=$(ip -n "${lab}-ze" -o link show "${dev}" | awk '{for (i=1;i<=NF;i++) if ($i=="link/ether") {print $(i+1); exit}}')
        echo "VIP owner: Ze (${dev}), virtual MAC ${mac}"
        return
    fi
    if ip -n "${lab}-peer" -o addr show | grep -q '192.0.2.1/24'; then
        dev=$(ip -n "${lab}-peer" -o addr show | awk '$4 == "192.0.2.1/24" {print $2; exit}')
        mac=$(ip -n "${lab}-peer" -o link show "${dev}" | awk '{for (i=1;i<=NF;i++) if ($i=="link/ether") {print $(i+1); exit}}')
        echo "VIP owner: keepalived (${dev}), virtual MAC ${mac}"
        return
    fi
    echo "VIP owner: none"
    return 1
}

crash() {
    local pid
    pid=$(cat "${ze_pid}")
    kill -KILL "${pid}"
    for ((i = 0; i < 50; i++)); do
        kill -0 "${pid}" 2>/dev/null || break
        sleep 0.1
    done
    rm -f "${ze_pid}"
    ip netns del "${lab}-ze"
    echo "VRRP node namespace removed"
}

failover() {
    local pid
    pid=$(cat "${ze_pid}")
    kill "${pid}"
    for ((i = 0; i < 50; i++)); do
        kill -0 "${pid}" 2>/dev/null || break
        sleep 0.1
    done
    kill -KILL "${pid}" 2>/dev/null || true
    rm -f "${ze_pid}"
    ip netns del "${lab}-ze" 2>/dev/null || true
    for ((i = 0; i < 150; i++)); do
        if ip -n "${lab}-peer" -o addr show | grep -q '192.0.2.1/24' \
            && ping -q -c 2 -W 1 192.0.2.1 >/dev/null; then
            owner
            echo 'VIP 192.0.2.1: 2/2 probes answered after failover'
            return
        fi
        sleep 0.1
    done
    cat "${state}/keepalived.log" >&2
    return 1
}

walkthrough() {
    local show before after
    show=$(show_ze 2>&1)
    before=$(owner 2>&1)
    [[ "${show}" == *master* && "${before}" == *"VIP owner: Ze"* ]] || return 1
    echo '$ ze show vrrp'
    echo 'gateway state: master, priority: 200'
    printf '%s\n' "${before}"
    sleep 5

    after=$(failover 2>&1)
    [[ "${after}" == *"VIP owner: keepalived"* ]] || return 1
    printf '%s\n' "${after}"
    sleep 8
    stop
    echo "VRRP walkthrough complete"
}

case "${1:-}" in
    prepare) prepare; echo "VRRP lab prepared" ;;
    start) start ;;
    show) show_ze ;;
    owner) owner ;;
    crash) crash ;;
    failover) failover ;;
    walkthrough) walkthrough ;;
    stop) stop; echo "VRRP lab stopped" ;;
    *) echo "usage: $0 prepare|start|show|owner|crash|failover|walkthrough|stop" >&2; exit 2 ;;
esac
