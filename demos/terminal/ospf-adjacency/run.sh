#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/network-lab.sh

state=/src/tmp/terminal-demos/state/ospf-adjacency
config_dir="${state}/config"
pid_file="${state}/ze.pid"
log_file="${state}/ze.log"
lab=ospf
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

stop() {
    if [[ -f "${pid_file}" ]]; then kill "$(cat "${pid_file}")" 2>/dev/null || true; fi
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
    cat /src/demos/terminal/ospf-adjacency/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >/dev/null
    lab_create_pair "${lab}" 172.31.0.2/24 172.31.0.3/24
    ip -n "${lab}-ze" addr add 10.255.0.2/32 dev lo
    ip -n "${lab}-peer" addr add 10.255.0.3/32 dev lo
}

start() {
    install -o frr -g frr -m 640 /src/demos/terminal/ospf-adjacency/frr.conf /etc/frr/frr.conf
    cat >/etc/frr/daemons <<'EOF'
zebra=yes
bgpd=no
ospfd=yes
ospf6d=no
bfdd=no
vtysh_enable=yes
EOF
    ip netns exec "${lab}-peer" /usr/lib/frr/frrinit.sh start >/dev/null
    ip netns exec "${lab}-ze" env ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 ze start ze.conf >"${log_file}" 2>&1 &
    echo "$!" >"${pid_file}"
    wait_for_text "${log_file}" "SSH server listening"
    wait_for_command 300 full cli "show ospf neighbor" >/dev/null
    sleep 2
    # Refresh redistribution after Zebra and the OSPF adjacency are both ready.
    ip netns exec "${lab}-peer" vtysh -c "configure terminal" -c "router ospf" \
        -c "no redistribute connected" -c "redistribute connected"
    wait_for_command 300 10.255.0.3 cli "show ospf route" >/dev/null
}

cli() {
    capture_bounded ip netns exec "${lab}-ze" env \
        ZE_CONFIG_DIR="${config_dir}" ZE_SSH_PASSWORD=secret123 \
        ze cli -c "$1"
}

walkthrough() {
    local neighbor database routes
    neighbor=$(cli "show ospf neighbor detail" 2>&1)
    database=$(cli "show ospf database router" 2>&1)
    routes=$(cli "show ospf route" 2>&1)
    [[ "${neighbor}" == *full* && "${neighbor}" == *172.31.0.3* ]] || return 1
    [[ "${database}" == *172.31.0.3* ]] || return 1
    [[ "${routes}" == *10.255.0.3* ]] || return 1
    echo '$ ze show ospf neighbor detail'
    echo 'Neighbor 172.31.0.3 state: full'
    sleep 4
    echo '$ ze show ospf database router'
    echo 'Router-LSA 172.31.0.3 present in area 0.0.0.0'
    sleep 4
    echo '$ ze show ospf route'
    echo 'Route 10.255.0.3/32 via 172.31.0.3 on eth0'
    sleep 8
    stop
    echo "OSPF walkthrough complete"
}

case "${1:-}" in
    prepare) prepare; echo "OSPF lab prepared" ;;
    start) start; echo "OSPF adjacency converged" ;;
    cli) shift; cli "$*" ;;
    walkthrough) walkthrough ;;
    stop) stop; echo "OSPF lab stopped" ;;
    *) echo "usage: $0 prepare|start|cli <command>|walkthrough|stop" >&2; exit 2 ;;
esac
