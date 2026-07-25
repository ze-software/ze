#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/network-lab.sh

state=/src/tmp/terminal-demos/state/bfd-failover
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/ze.log"
bgp_state_file="${state}/bgp-state"
lab=bfd
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    if [[ -f "${pid_file}" ]]; then
        while read -r pid; do
            kill "${pid}" 2>/dev/null || true
        done <"${pid_file}"
    fi
    ip netns exec "${lab}-peer" /usr/lib/frr/frrinit.sh stop >/dev/null 2>&1 || true
    lab_cleanup "${lab}"
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
    cat /src/demos/terminal/bfd-failover/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >/dev/null
    lab_create_pair "${lab}" 172.30.0.2/24 172.30.0.3/24
}

start() {
    install -o frr -g frr -m 640 /src/demos/terminal/bfd-failover/frr.conf /etc/frr/frr.conf
    cat >/etc/frr/daemons <<'EOF'
zebra=yes
bgpd=no
ospfd=no
ospf6d=no
bfdd=yes
vtysh_enable=yes
EOF
    ip netns exec "${lab}-peer" /usr/lib/frr/frrinit.sh start >/dev/null
    : >"${pid_file}"
    ip netns exec "${lab}-peer" ze-test peer --mode sink --bind 172.30.0.3 \
        --port 1179 --asn 65002 >"${state}/peer.log" 2>&1 &
    echo "$!" >>"${pid_file}"
    ip netns exec "${lab}-ze" env ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 ze start ze.conf >"${log_file}" 2>&1 &
    echo "$!" >>"${pid_file}"
    wait_for_text "${log_file}" "SSH server listening"
    wait_for_command 300 established cli "show bgp peer list" >/dev/null
    wait_for_command 300 up cli "show bfd sessions" >/dev/null
    printf 'BGP state: established\n' >"${bgp_state_file}"
}

cli() {
    capture_bounded ip netns exec "${lab}-ze" env \
        ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 \
        ze cli -c "$1"
}

cut_link() {
    ip link set "${lab}-p" down
    for ((i = 0; i < 100; i++)); do
        output=$(cli "show bgp peer list" 2>&1 || true)
        [[ "${output}" != *established* ]] && break
        sleep 0.1
    done
    printf 'BGP state: down\n' >"${bgp_state_file}"
    echo "Peer link cut; BFD notified BGP"
}

restore_link() {
    ip link set "${lab}-p" up
    wait_for_command 300 established cli "show bgp peer list" >/dev/null
    printf 'BGP state: established\n' >"${bgp_state_file}"
    echo "Peer link restored; BGP re-established"
}

record_cut() {
    local output
    ip link set "${lab}-p" down
    sleep 5
    output=$(timeout 5 ip netns exec "${lab}-ze" env ZE_CONFIG_DIR="${config_dir}" \
        ZE_SSH_PASSWORD=secret123 ze cli -c "show bgp peer list" 2>&1 || true)
    if [[ "${output}" == *established* ]]; then
        printf '%s\n' "${output}" >&2
        return 1
    fi
    printf 'BGP state: down\n' >"${bgp_state_file}"
    echo "Peer link cut; BFD notified BGP"
}

walkthrough() {
    local before_bfd before_bgp after_bfd after_bgp restored
    before_bfd=$(cli "show bfd sessions" 2>&1)
    before_bgp=$(cli "show bgp peer list" 2>&1)
    [[ "${before_bfd}" == *up* && "${before_bgp}" == *established* ]] || return 1
    echo "BFD state: up"
    echo "BGP state: established"
    sleep 4

    cut_link >/dev/null
    after_bfd=$(cli "show bfd sessions" 2>&1 || true)
    after_bgp=$(cli "show bgp peer list" 2>&1 || true)
    [[ "${after_bfd}" != *up* && "${after_bgp}" != *established* ]] || return 1
    echo "Peer link cut; BFD notified BGP"
    echo "BFD state: down"
    echo "BGP state: down"
    sleep 8

    restore_link >/dev/null
    restored=$(cli "show bgp peer list" 2>&1)
    [[ "${restored}" == *established* ]] || return 1
    echo "Peer link restored; BGP state: established"
    stop
    echo "BFD walkthrough complete"
}

case "${1:-}" in
    prepare) prepare; echo "BFD failover demo prepared" ;;
    start) start; echo "BFD and BGP sessions are up" ;;
    cli) shift; cli "$*" ;;
    bgp) cat "${bgp_state_file}" ;;
    cut) cut_link ;;
    restore) restore_link ;;
    walkthrough) walkthrough ;;
    stop) stop; echo "BFD failover demo stopped" ;;
    *) echo "usage: $0 prepare|start|cli <command>|bgp|cut|restore|walkthrough|stop" >&2; exit 2 ;;
esac
