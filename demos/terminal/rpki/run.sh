#!/usr/bin/env bash
set -euo pipefail

state=/src/tmp/terminal-demos/state/rpki
config_dir="${state}/config"
pid_file="${state}/pids"
log_file="${state}/daemon.log"
export ZE_CONFIG_DIR="${config_dir}"
export ZE_SSH_PASSWORD=secret123

# VRP count the mock generates for IPv4: one /8 VRP per first octet except those
# with `octet % 3 == 2`, i.e. 256 - 85 = 171 (internal/test/mock/rpki/rpki.go
# generateVRPs). Named once here and in validate.sh rather than spelled as a
# bare literal at each use, so a change to the mock's generation rule surfaces as
# one edit instead of a silent readiness hang.
expected_vrp_ipv4=171

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

# wait_port blocks until a TCP listener accepts on host:port. 30 x 0.1s = 3s,
# which is generous: these fixtures are local processes binding a loopback port,
# a sub-second operation (measured: `start` completes in ~0.9s end to end).
#
# The budget is deliberately SHORT because every wait here is spent inside the
# tape's `Wait+Screen /RPKI demo ready/`, which VHS bounds at
# `Set WaitTimeout 30s` (demos/terminal/common.tape:9) -- and a VHS timeout is
# not local: render.py runs each demo with check=True and no per-demo try, so
# one blown Wait aborts every remaining demo and fails the whole website build.
# Worst case here is 3s + 3s (two fixtures) + 15s (the pre-existing SSH loop) +
# 5s (the VRP deadline below) = 26s, which leaves margin under the 30s.
#
# This is not defensive padding. ze dials the RTR cache ONCE at startup and, on
# failure, waits its RFC 8210 Retry Interval before trying again -- 600 seconds
# by default (internal/component/bgp/plugins/rpki/rtr_session.go:81). So a cache
# server that is a few milliseconds late to listen does not cost a retry, it
# costs TEN MINUTES: the session sits in `state: idle` with vrp-count 0, every
# prefix validates NotFound, `not-found accept` lets the RPKI-invalid
# 10.43.0.0/24 into the Adj-RIB-In, and the demo's whole point inverts. Nothing
# downstream can recover inside any sane window, which is why this waits here
# rather than polling harder later.
#
# /dev/tcp rather than the image's nc: one shell builtin, no subprocess per
# probe. The probe runs in a subshell, so the connection is closed by that
# subshell exiting -- the parent never holds the descriptor.
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
    ze config cat ze.conf >"${state}/active.conf"
    cat /src/demos/terminal/rpki/ze.conf >>"${state}/active.conf"
    ze config import --name ze.conf "${state}/active.conf" >"${state}/import.log" 2>&1

    : >"${pid_file}"
    ze-test rpki --bind 127.0.0.3 --port 3323 --valid-asn 65001 --invalid-asn 65099 \
        >"${state}/rpki.log" 2>&1 &
    echo "$!" >>"${pid_file}"
    ze-test peer --mode sink --bind 127.0.0.2 --port 1179 --asn 65001 \
        /src/demos/terminal/rpki/routes.msg >"${state}/peer.log" 2>&1 &
    echo "$!" >>"${pid_file}"

    # Both fixtures must be accepting BEFORE ze starts (see wait_port).
    wait_port 127.0.0.3 3323 "rpki cache" || { cat "${state}/rpki.log" >&2; return 1; }
    wait_port 127.0.0.2 1179 "bgp peer" || { cat "${state}/peer.log" >&2; return 1; }

    ze start ze.conf >"${log_file}" 2>&1 &
    echo "$!" >>"${pid_file}"

    for _ in {1..150}; do
        if [[ -f "${log_file}" ]] && grep -q "SSH server listening" "${log_file}"; then
            break
        fi
        sleep 0.1
    done
    if ! grep -q "SSH server listening" "${log_file}" 2>/dev/null; then
        cat "${log_file}" >&2
        return 1
    fi

    # Do not report ready until the VRP set is actually loaded. `sessions: 1` is
    # the CONFIGURED cache count and is true while the session is still idle, so
    # anything that gates on it is gating on nothing; the VRP count is the first
    # observable that means the cache synced. Until it does, origin validation
    # has no data and the demo shows the opposite of what it teaches.
    #
    # Command substitution, NOT `ze cli ... | grep -q`. Under `set -o pipefail`
    # (line 2) grep -q exits at the first match, ze cli takes SIGPIPE, and the
    # pipeline reports FAILURE even though the pattern matched -- so the loop
    # would spin to exhaustion on the success case.
    #
    # Bounded by WALL CLOCK, not by iteration count: each pass costs an `ze cli`
    # SSH round trip on top of the sleep, so an iteration budget silently means
    # something different on a slow host -- and this budget has to stay inside
    # the tape's 30s VHS Wait (see wait_port). Syncing 171 VRPs over loopback is
    # a sub-second exchange once connected.
    local status deadline
    deadline=$((SECONDS + 5))
    while ((SECONDS < deadline)); do
        status=$(ze cli -c 'show bgp rpki status | no-more' 2>/dev/null || true)
        if [[ "${status}" == *"vrp-count-ipv4: ${expected_vrp_ipv4}"* ]]; then
            return 0
        fi
        sleep 0.1
    done
    echo "timeout waiting for the RTR cache to sync (vrp-count-ipv4: ${expected_vrp_ipv4})" >&2
    echo "${status}" >&2
    return 1
}

case "${1:-}" in
    prepare) prepare; echo "RPKI demo prepared" ;;
    start) start; echo "RPKI demo ready" ;;
    stop) stop; echo "RPKI demo stopped" ;;
    *) echo "usage: $0 prepare|start|stop" >&2; exit 2 ;;
esac
