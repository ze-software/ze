#!/usr/bin/env bash
set -euo pipefail

lab_cleanup() {
    local prefix=$1
    ip link del "${prefix}-br" 2>/dev/null || true
    ip netns del "${prefix}-ze" 2>/dev/null || true
    ip netns del "${prefix}-peer" 2>/dev/null || true
}

lab_create_pair() {
    local prefix=$1 ze_addr=$2 peer_addr=$3
    lab_cleanup "${prefix}"
    ip netns add "${prefix}-ze"
    ip netns add "${prefix}-peer"
    ip link add "${prefix}-br" type bridge
    ip link set "${prefix}-br" up
    ip link add "${prefix}-z" type veth peer name eth0 netns "${prefix}-ze"
    ip link add "${prefix}-p" type veth peer name eth0 netns "${prefix}-peer"
    ip link set "${prefix}-z" master "${prefix}-br"
    ip link set "${prefix}-p" master "${prefix}-br"
    ip link set "${prefix}-z" up
    ip link set "${prefix}-p" up
    ip -n "${prefix}-ze" link set lo up
    ip -n "${prefix}-peer" link set lo up
    ip -n "${prefix}-ze" link set eth0 up
    ip -n "${prefix}-peer" link set eth0 up
    ip -n "${prefix}-ze" addr add "${ze_addr}" dev eth0
    ip -n "${prefix}-peer" addr add "${peer_addr}" dev eth0
}

capture_bounded() {
    local output_file pid status=0
    output_file=$(mktemp)
    setsid "$@" </dev/null >"${output_file}" 2>&1 &
    pid=$!
    for ((i = 0; i < 50; i++)); do
        kill -0 "${pid}" 2>/dev/null || break
        sleep 0.1
    done
    if kill -0 "${pid}" 2>/dev/null; then
        kill -TERM -- "-${pid}" 2>/dev/null || true
        sleep 0.1
        kill -KILL -- "-${pid}" 2>/dev/null || true
    fi
    wait "${pid}" 2>/dev/null || status=$?
    cat "${output_file}"
    rm -f "${output_file}"
    [[ ${status} -eq 0 || ${status} -eq 137 || ${status} -eq 143 ]]
}

wait_for_text() {
    local file=$1 pattern=$2 attempts=${3:-200}
    for ((i = 0; i < attempts; i++)); do
        if [[ -f "${file}" ]] && grep -q -- "${pattern}" "${file}"; then
            return 0
        fi
        sleep 0.1
    done
    [[ -f "${file}" ]] && cat "${file}" >&2
    return 1
}

wait_for_command() {
    local attempts=$1 pattern=$2
    shift 2
    local output=
    for ((i = 0; i < attempts; i++)); do
        output=$("$@" 2>&1 || true)
        if [[ "${output}" == *"${pattern}"* ]]; then
            printf '%s\n' "${output}"
            return 0
        fi
        sleep 0.1
    done
    printf '%s\n' "${output}" >&2
    return 1
}
