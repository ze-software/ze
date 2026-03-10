#!/usr/bin/env bash
# Shared helper functions for interop test scenarios.
# Sourced by each scenario's check.sh.

set -euo pipefail

# Network and container naming.
NETWORK="ze-iop"
ZE_CONTAINER="ze-iop-ze"
FRR_CONTAINER="ze-iop-frr"
BIRD_CONTAINER="ze-iop-bird"

# IP addresses on the test network.
ZE_IP="172.30.0.2"
FRR_IP="172.30.0.3"
BIRD_IP="172.30.0.4"

# Default timeout for session establishment (seconds).
SESSION_TIMEOUT="${SESSION_TIMEOUT:-90}"

# ─── Logging ────────────────────────────────────────────────────────────────

log_info()  { printf "  %s\n" "$*"; }
log_pass()  { printf "  \033[32m✓ %s\033[0m\n" "$*"; }
log_fail()  { printf "  \033[31m✗ %s\033[0m\n" "$*"; }
log_debug() { [ "${VERBOSE:-0}" = "1" ] && printf "  [debug] %s\n" "$*" || true; }

# ─── FRR helpers ────────────────────────────────────────────────────────────

# wait_frr_session <neighbor_ip> [timeout_seconds]
# Waits for FRR's BGP session with the given neighbor to reach Established.
wait_frr_session() {
    local neighbor="$1"
    local timeout="${2:-$SESSION_TIMEOUT}"
    local deadline=$((SECONDS + timeout))

    log_info "waiting for FRR session with $neighbor (timeout ${timeout}s)..."
    while [ $SECONDS -lt $deadline ]; do
        # Use "show bgp neighbor X" which explicitly prints "BGP state = Established".
        # The summary view shows (Policy) or prefix count instead of "Estab" in FRR 10.x.
        if docker exec "$FRR_CONTAINER" vtysh -c "show bgp neighbor $neighbor" 2>/dev/null | grep -q "BGP state = Established"; then
            log_pass "FRR session with $neighbor is Established"
            return 0
        fi
        sleep 2
    done
    log_fail "FRR session with $neighbor did not reach Established within ${timeout}s"
    # Dump debug info.
    docker exec "$FRR_CONTAINER" vtysh -c "show bgp neighbor $neighbor" 2>/dev/null | head -10 || true
    docker logs "$ZE_CONTAINER" --tail 20 2>/dev/null || true
    return 1
}

# check_frr_route <prefix>
# Checks whether FRR has the given prefix in its BGP IPv4 unicast table.
check_frr_route() {
    local prefix="$1"
    if docker exec "$FRR_CONTAINER" vtysh -c "show bgp ipv4 unicast $prefix" 2>/dev/null | grep -q "$prefix"; then
        log_pass "FRR has route $prefix"
        return 0
    fi
    log_fail "FRR does not have route $prefix"
    return 1
}

# check_frr_route_community <prefix> <community>
# Checks whether FRR has the prefix with the expected community string.
check_frr_route_community() {
    local prefix="$1"
    local community="$2"
    if docker exec "$FRR_CONTAINER" vtysh -c "show bgp ipv4 unicast $prefix" 2>/dev/null | grep -q "$community"; then
        log_pass "FRR route $prefix has community $community"
        return 0
    fi
    log_fail "FRR route $prefix missing community $community"
    return 1
}

# check_frr_route_count <minimum>
# Checks that FRR has at least N routes in its BGP table.
check_frr_route_count() {
    local minimum="$1"
    local count
    count=$(docker exec "$FRR_CONTAINER" vtysh -c "show bgp ipv4 unicast summary" 2>/dev/null | \
        grep "$ZE_IP" | awk '{print $(NF-1)}')
    if [ -n "$count" ] && [ "$count" -ge "$minimum" ] 2>/dev/null; then
        log_pass "FRR received $count routes from Ze (expected >= $minimum)"
        return 0
    fi
    log_fail "FRR received ${count:-0} routes from Ze (expected >= $minimum)"
    return 1
}

# ─── BIRD helpers ───────────────────────────────────────────────────────────

# wait_bird_session <protocol_name> [timeout_seconds]
# Waits for a BIRD BGP protocol to reach Established state.
wait_bird_session() {
    local proto="$1"
    local timeout="${2:-$SESSION_TIMEOUT}"
    local deadline=$((SECONDS + timeout))

    log_info "waiting for BIRD protocol $proto (timeout ${timeout}s)..."
    while [ $SECONDS -lt $deadline ]; do
        if docker exec "$BIRD_CONTAINER" birdc show protocols 2>/dev/null | grep "$proto" | grep -q "Established"; then
            log_pass "BIRD protocol $proto is Established"
            return 0
        fi
        sleep 2
    done
    log_fail "BIRD protocol $proto did not reach Established within ${timeout}s"
    docker exec "$BIRD_CONTAINER" birdc show protocols all 2>/dev/null || true
    docker logs "$ZE_CONTAINER" --tail 20 2>/dev/null || true
    return 1
}

# check_bird_route <prefix>
# Checks whether BIRD has the given prefix in its routing table.
check_bird_route() {
    local prefix="$1"
    if docker exec "$BIRD_CONTAINER" birdc "show route for $prefix" 2>/dev/null | grep -q "$prefix"; then
        log_pass "BIRD has route $prefix"
        return 0
    fi
    log_fail "BIRD does not have route $prefix"
    return 1
}

# ─── Ze helpers ─────────────────────────────────────────────────────────────

# ze_logs [lines]
# Print last N lines from Ze container logs.
ze_logs() {
    local lines="${1:-30}"
    docker logs "$ZE_CONTAINER" --tail "$lines" 2>&1 || true
}

# check_ze_rib_received <minimum>
# Checks that Ze's Adj-RIB-In has at least N received routes.
# Requires bgp-rib plugin to be configured in ze.conf.
# Uses "rib status" which returns {"routes-in":N,...}.
check_ze_rib_received() {
    local minimum="$1"
    local output count
    output=$(docker exec "$ZE_CONTAINER" ze show rib status 2>/dev/null || true)
    count=$(echo "$output" | grep -o '"routes-in":[0-9]*' | grep -o '[0-9]*' || true)
    if [ -n "$count" ] && [ "$count" -ge "$minimum" ] 2>/dev/null; then
        log_pass "Ze RIB has $count received routes (expected >= $minimum)"
        return 0
    fi
    log_fail "Ze RIB has ${count:-0} received routes (expected >= $minimum)"
    echo "  rib status: $output"
    return 1
}

# ─── Lifecycle ──────────────────────────────────────────────────────────────

# wait_containers_healthy [timeout_seconds]
# Waits for all running containers to be responsive.
wait_containers_healthy() {
    local timeout="${1:-30}"
    local deadline=$((SECONDS + timeout))

    while [ $SECONDS -lt $deadline ]; do
        local all_ready=true

        # Check Ze is running.
        if ! docker inspect "$ZE_CONTAINER" --format '{{.State.Running}}' 2>/dev/null | grep -q true; then
            all_ready=false
        fi

        # Check FRR if running.
        if docker inspect "$FRR_CONTAINER" --format '{{.State.Running}}' 2>/dev/null | grep -q true; then
            # FRR needs vtysh to be responsive.
            if ! docker exec "$FRR_CONTAINER" vtysh -c "show version" >/dev/null 2>&1; then
                all_ready=false
            fi
        fi

        # Check BIRD if running.
        if docker inspect "$BIRD_CONTAINER" --format '{{.State.Running}}' 2>/dev/null | grep -q true; then
            if ! docker exec "$BIRD_CONTAINER" birdc show status >/dev/null 2>&1; then
                all_ready=false
            fi
        fi

        if $all_ready; then
            log_debug "all containers healthy"
            return 0
        fi

        sleep 1
    done

    log_fail "containers did not become healthy within ${timeout}s"
    return 1
}
