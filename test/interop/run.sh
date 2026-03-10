#!/usr/bin/env bash
# Ze interoperability test runner.
#
# Usage:
#   test/interop/run.sh                  # run all scenarios
#   test/interop/run.sh 01-ebgp-ipv4-frr # run a specific scenario
#   VERBOSE=1 test/interop/run.sh        # verbose output
#
# Environment:
#   FRR_IMAGE   - FRR Docker image (default: quay.io/frrouting/frr:10.3.1)
#   VERBOSE     - set to 1 for debug output
#   NO_BUILD    - set to 1 to skip image builds (reuse existing)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

FRR_IMAGE="${FRR_IMAGE:-quay.io/frrouting/frr:10.3.1}"
SUBNET="172.30.0.0/24"

# ─── Image builds ──────────────────────────────────────────────────────────

build_images() {
    if [ "${NO_BUILD:-0}" = "1" ]; then
        log_info "skipping image builds (NO_BUILD=1)"
        return
    fi

    echo "Building Ze image..."
    docker build -t ze-interop -f "$SCRIPT_DIR/Dockerfile.ze" "$PROJECT_ROOT" --quiet

    echo "Building BIRD image..."
    docker build -t bird-interop -f "$SCRIPT_DIR/Dockerfile.bird" "$SCRIPT_DIR" --quiet

    echo "Pulling FRR image..."
    docker pull "$FRR_IMAGE" --quiet
}

# ─── Container lifecycle ───────────────────────────────────────────────────

cleanup() {
    log_debug "cleaning up containers and network..."
    docker rm -f "$ZE_CONTAINER" "$FRR_CONTAINER" "$BIRD_CONTAINER" 2>/dev/null || true
    docker network rm "$NETWORK" 2>/dev/null || true
}

start_network() {
    docker network create --subnet="$SUBNET" "$NETWORK" >/dev/null 2>&1 || true
}

start_ze() {
    local config_file="$1"
    shift
    # Build docker run args (avoids empty-array issues on bash <4.4).
    local args=(
        run -d
        --name "$ZE_CONTAINER"
        --network "$NETWORK"
        --ip "$ZE_IP"
        --cap-add NET_ADMIN
        -v "$config_file:/etc/ze/bgp.conf:ro"
    )
    while [ $# -gt 0 ]; do
        args+=("$1")
        shift
    done
    args+=(ze-interop /etc/ze/bgp.conf)
    docker "${args[@]}" >/dev/null
}

start_frr() {
    local config_file="$1"
    docker run -d \
        --name "$FRR_CONTAINER" \
        --network "$NETWORK" \
        --ip "$FRR_IP" \
        --cap-add NET_ADMIN \
        --cap-add SYS_ADMIN \
        -v "$config_file:/etc/frr/frr.conf:ro" \
        -v "$SCRIPT_DIR/daemons:/etc/frr/daemons:ro" \
        -v "$SCRIPT_DIR/vtysh.conf:/etc/frr/vtysh.conf:ro" \
        "$FRR_IMAGE" \
        >/dev/null
}

start_bird() {
    local config_file="$1"
    docker run -d \
        --name "$BIRD_CONTAINER" \
        --network "$NETWORK" \
        --ip "$BIRD_IP" \
        --cap-add NET_ADMIN \
        -v "$config_file:/etc/bird/bird.conf:ro" \
        bird-interop \
        >/dev/null
}

# ─── Scenario execution ───────────────────────────────────────────────────

run_scenario() {
    local scenario_dir="$1"
    local scenario_name
    scenario_name="$(basename "$scenario_dir")"

    # Clean slate.
    cleanup
    start_network

    # Start Ze (always present).
    if [ ! -f "$scenario_dir/ze.conf" ]; then
        log_fail "missing ze.conf in $scenario_name"
        return 1
    fi

    # Collect extra volume mounts for Ze (e.g., process plugin scripts).
    local ze_extra=()
    for extra_file in "$scenario_dir"/*.sh "$scenario_dir"/*.py; do
        [ -f "$extra_file" ] || continue
        local bname
        bname="$(basename "$extra_file")"
        # Don't mount the check script into the container.
        [ "$bname" = "check.sh" ] && continue
        ze_extra+=("-v" "$extra_file:/etc/ze/$bname:ro")
    done

    if [ ${#ze_extra[@]} -gt 0 ]; then
        start_ze "$scenario_dir/ze.conf" "${ze_extra[@]}"
    else
        start_ze "$scenario_dir/ze.conf"
    fi

    # Start FRR if config exists.
    if [ -f "$scenario_dir/frr.conf" ]; then
        start_frr "$scenario_dir/frr.conf"
    fi

    # Start BIRD if config exists.
    if [ -f "$scenario_dir/bird.conf" ]; then
        start_bird "$scenario_dir/bird.conf"
    fi

    # Wait for containers to be responsive.
    if ! wait_containers_healthy 30; then
        log_fail "containers failed to start"
        docker logs "$ZE_CONTAINER" --tail 30 2>&1 || true
        return 1
    fi

    # Run scenario-specific checks.
    if bash "$scenario_dir/check.sh"; then
        return 0
    else
        return 1
    fi
}

# ─── Main ──────────────────────────────────────────────────────────────────

main() {
    local filter="${1:-}"
    local passed=0
    local failed=0
    local failed_names=""

    # Ensure cleanup on exit.
    trap cleanup EXIT

    # Check Docker is available.
    if ! docker info >/dev/null 2>&1; then
        echo "error: Docker is not running or not accessible"
        exit 1
    fi

    build_images

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo " Ze Interoperability Tests"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    for scenario_dir in "$SCRIPT_DIR"/scenarios/*/; do
        [ -d "$scenario_dir" ] || continue

        local scenario_name
        scenario_name="$(basename "$scenario_dir")"

        # Filter if a specific scenario was requested.
        if [ -n "$filter" ] && [ "$scenario_name" != "$filter" ]; then
            continue
        fi

        echo "── $scenario_name ──"

        if run_scenario "$scenario_dir"; then
            log_pass "PASS"
            passed=$((passed + 1))
        else
            log_fail "FAIL"
            failed=$((failed + 1))
            failed_names="${failed_names:+$failed_names }$scenario_name"
        fi

        echo ""
    done

    # Summary.
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if [ $failed -eq 0 ]; then
        printf "\033[32mPASS  %d scenario(s)\033[0m\n" "$passed"
    else
        printf "\033[31mFAIL  %d passed, %d failed: %s\033[0m\n" "$passed" "$failed" "$failed_names"
    fi
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    [ $failed -eq 0 ]
}

main "$@"
