#!/usr/bin/env bash
# Ze performance benchmark runner.
#
# Orchestrates Docker containers for each DUT (Ze, FRR, BIRD, GoBGP),
# runs ze-perf against each, collects results, and generates reports.
#
# Architecture: ze-perf runs inside a Docker container on the same network
# as the DUT. A second container provides the receiver IP so the DUT can
# distinguish sender (172.31.0.10) from receiver (172.31.0.11) by source IP.
# Both IPs route to the runner container via Docker network aliases.
#
# Usage:
#   test/perf/run.sh                    # run all DUTs
#   test/perf/run.sh ze                 # run single DUT
#   DUT_ROUTES=1000 test/perf/run.sh    # override route count
#
# Environment:
#   FRR_IMAGE    - FRR Docker image (default: quay.io/frrouting/frr:10.3.1)
#   NO_BUILD     - set to 1 to skip image builds
#   DUT_ROUTES   - number of routes to send (default: 1000)
#   DUT_SEED     - route generation seed (default: 42)
#   DUT_REPEAT   - benchmark iterations (default: 3)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
INTEROP_DIR="${PROJECT_ROOT}/test/interop"
CONFIGS_DIR="${SCRIPT_DIR}/configs"
RESULTS_DIR="${SCRIPT_DIR}/results"
ZE_PERF="${PROJECT_ROOT}/bin/ze-perf"
ZE_PERF_LINUX="${PROJECT_ROOT}/bin/ze-perf-linux"

# Network uses separate subnet from interop tests (172.30.0.0/24).
SUFFIX="$$"
NETWORK="ze-perf-${SUFFIX}"
SUBNET="172.31.0.0/24"

# DUT IPs (match config files).
ZE_DUT_IP="172.31.0.2"
FRR_DUT_IP="172.31.0.3"
BIRD_DUT_IP="172.31.0.4"
GOBGP_DUT_IP="172.31.0.5"

# ze-perf sender/receiver IPs (separate so DUTs can distinguish by source IP).
SENDER_IP="172.31.0.10"
RECEIVER_IP="172.31.0.11"

# Defaults.
FRR_IMAGE="${FRR_IMAGE:-quay.io/frrouting/frr:10.3.1}"
NO_BUILD="${NO_BUILD:-0}"
DUT_ROUTES="${DUT_ROUTES:-1000}"
DUT_SEED="${DUT_SEED:-42}"
DUT_REPEAT="${DUT_REPEAT:-3}"
DUT_FILTER="${1:-}"

# ── Image building ──────────────────────────────────────────────────────────

build_images() {
    if [ "${NO_BUILD}" = "1" ]; then
        echo "  skipping image builds (NO_BUILD=1)"
        return
    fi

    echo "Building Ze image..."
    DOCKER_BUILDKIT=1 docker build -t ze-interop \
        -f "${INTEROP_DIR}/Dockerfile.ze" \
        "${PROJECT_ROOT}" --quiet

    echo "Building BIRD image..."
    DOCKER_BUILDKIT=1 docker build -t bird-interop \
        -f "${INTEROP_DIR}/Dockerfile.bird" \
        "${INTEROP_DIR}" --quiet

    echo "Building GoBGP image..."
    DOCKER_BUILDKIT=1 docker build -t gobgp-interop \
        -f "${INTEROP_DIR}/Dockerfile.gobgp" \
        "${INTEROP_DIR}" --quiet || echo "  warning: GoBGP image build failed"

    echo "Pulling FRR image..."
    docker pull "${FRR_IMAGE}" --quiet
}

# ── Container management ────────────────────────────────────────────────────

start_dut() {
    local name="$1"
    local image="$2"
    local ip="$3"
    local container="ze-perf-${name}-${SUFFIX}"

    case "${name}" in
        ze)
            docker run -d --name "${container}" \
                --network "${NETWORK}" --ip "${ip}" \
                --cap-add NET_ADMIN \
                -v "${CONFIGS_DIR}/ze.conf:/etc/ze/bgp.conf:ro" \
                "${image}" /etc/ze/bgp.conf >/dev/null
            ;;
        frr)
            docker run -d --name "${container}" \
                --network "${NETWORK}" --ip "${ip}" \
                --cap-add NET_ADMIN --cap-add SYS_ADMIN \
                -v "${CONFIGS_DIR}/frr.conf:/etc/frr/frr.conf:ro" \
                -v "${INTEROP_DIR}/daemons:/etc/frr/daemons:ro" \
                -v "${INTEROP_DIR}/vtysh.conf:/etc/frr/vtysh.conf:ro" \
                "${image}" >/dev/null
            ;;
        bird)
            docker run -d --name "${container}" \
                --network "${NETWORK}" --ip "${ip}" \
                --cap-add NET_ADMIN \
                -v "${CONFIGS_DIR}/bird.conf:/etc/bird/bird.conf:ro" \
                "${image}" >/dev/null
            ;;
        gobgp)
            docker run -d --name "${container}" \
                --network "${NETWORK}" --ip "${ip}" \
                --cap-add NET_ADMIN \
                -v "${CONFIGS_DIR}/gobgp.toml:/etc/gobgp/gobgp.toml:ro" \
                "${image}" >/dev/null
            ;;
        *)
            echo "error: unknown DUT: ${name}" >&2
            return 1
            ;;
    esac

    echo "  started ${name} at ${ip}"
}

stop_dut() {
    local name="$1"
    local container="ze-perf-${name}-${SUFFIX}"
    docker rm -f "${container}" >/dev/null 2>&1 || true
}

wait_dut_ready() {
    local name="$1"
    local container="ze-perf-${name}-${SUFFIX}"
    local deadline=$((SECONDS + 30))

    while [ "${SECONDS}" -lt "${deadline}" ]; do
        if docker inspect "${container}" --format '{{.State.Running}}' 2>/dev/null | grep -q true; then
            sleep 5
            echo "  ${name} ready"
            return 0
        fi
        sleep 1
    done

    echo "error: ${name} did not start within 30s" >&2
    docker logs "${container}" --tail 20 2>&1 || true
    return 1
}

# Run ze-perf inside Docker with two IPs (sender + receiver).
# Creates a runner container at SENDER_IP and adds RECEIVER_IP as a second interface.
run_perf_in_docker() {
    local dut_name="$1"
    local dut_ip="$2"
    local dut_port="$3"
    local sender_port="$4"
    local receiver_port="$5"
    local result_file="$6"
    local runner="ze-perf-runner-${SUFFIX}"
    local result_name
    result_name="$(basename "${result_file}")"

    # Start runner container at sender IP, keep it alive.
    # Use alpine (not ze-interop) to avoid entrypoint conflicts.
    docker run -d --name "${runner}" \
        --network "${NETWORK}" --ip "${SENDER_IP}" \
        --cap-add NET_ADMIN \
        -v "${ZE_PERF_LINUX}:/usr/local/bin/ze-perf:ro" \
        -v "${RESULTS_DIR}:/results" \
        alpine:3.21 sleep 3600 >/dev/null

    # Add receiver IP as a second address on the same interface.
    docker exec "${runner}" ip addr add "${RECEIVER_IP}/24" dev eth0 2>/dev/null || true

    # Run ze-perf inside the runner.
    # Build port flags (only add sender/receiver-port if non-zero).
    local port_flags=""
    if [ "${sender_port}" != "0" ]; then
        port_flags="${port_flags} --sender-port ${sender_port}"
    fi
    if [ "${receiver_port}" != "0" ]; then
        port_flags="${port_flags} --receiver-port ${receiver_port}"
    fi

    # shellcheck disable=SC2086
    docker exec "${runner}" /usr/local/bin/ze-perf run \
        --dut-addr "${dut_ip}" \
        --dut-port "${dut_port}" \
        --dut-asn 65000 \
        --dut-name "${dut_name}" \
        --sender-addr "${SENDER_IP}" \
        --sender-asn 65001 \
        --receiver-addr "${RECEIVER_IP}" \
        --receiver-asn 65002 \
        ${port_flags} \
        --routes "${DUT_ROUTES}" \
        --seed "${DUT_SEED}" \
        --repeat "${DUT_REPEAT}" \
        --warmup-runs 1 \
        --iter-delay 5s \
        --warmup 2s \
        --connect-timeout 10s \
        --duration 30s \
        --output "/results/${result_name}" \
        2>&1

    local rc=$?

    # Clean up runner.
    docker rm -f "${runner}" >/dev/null 2>&1 || true

    return "${rc}"
}

# ── Cleanup ─────────────────────────────────────────────────────────────────

cleanup() {
    echo ""
    echo "Cleaning up..."
    for dut in ze frr bird gobgp; do
        stop_dut "${dut}"
    done
    docker rm -f "ze-perf-runner-${SUFFIX}" >/dev/null 2>&1 || true
    docker network rm "${NETWORK}" >/dev/null 2>&1 || true
}

trap cleanup EXIT

# ── DUT definitions ─────────────────────────────────────────────────────────

declare -a DUT_NAMES=()
declare -A DUT_IMAGES=()
declare -A DUT_IPS=()
declare -A DUT_PORTS=()
declare -A DUT_SENDER_PORTS=()
declare -A DUT_RECEIVER_PORTS=()

add_dut() {
    local name="$1" image="$2" ip="$3" port="$4" sport="${5:-0}" rport="${6:-0}"
    DUT_NAMES+=("${name}")
    DUT_IMAGES["${name}"]="${image}"
    DUT_IPS["${name}"]="${ip}"
    DUT_PORTS["${name}"]="${port}"
    DUT_SENDER_PORTS["${name}"]="${sport}"
    DUT_RECEIVER_PORTS["${name}"]="${rport}"
}

# Ze uses separate per-peer ports; others use single port 179.
add_dut "ze"    "ze-interop"     "${ZE_DUT_IP}"    179  1790  1791
add_dut "frr"   "${FRR_IMAGE}"   "${FRR_DUT_IP}"   179
add_dut "bird"  "bird-interop"   "${BIRD_DUT_IP}"  179
add_dut "gobgp" "gobgp-interop"  "${GOBGP_DUT_IP}" 179

# ── Main ────────────────────────────────────────────────────────────────────

if ! command -v docker >/dev/null 2>&1; then
    echo "error: docker is not installed" >&2
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    echo "error: docker is not running" >&2
    exit 1
fi

if [ ! -x "${ZE_PERF}" ]; then
    echo "error: ze-perf not found. Run: make ze-perf-build" >&2
    exit 1
fi

# Build Linux binary for Docker.
if [ ! -f "${ZE_PERF_LINUX}" ]; then
    echo "Cross-compiling ze-perf for Linux..."
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o "${ZE_PERF_LINUX}" ./cmd/ze-perf/
fi

# Build images.
build_images

mkdir -p "${RESULTS_DIR}"
docker network create --subnet="${SUBNET}" "${NETWORK}" >/dev/null 2>&1 || true

echo ""
echo "--------------------------------------------"
echo " Ze Performance Benchmarks"
echo " Routes: ${DUT_ROUTES}  Seed: ${DUT_SEED}  Repeat: ${DUT_REPEAT}"
echo "--------------------------------------------"
echo ""

passed=0
failed=0
failed_names=""
result_files=""

for dut_name in "${DUT_NAMES[@]}"; do
    if [ -n "${DUT_FILTER}" ] && [ "${DUT_FILTER}" != "${dut_name}" ]; then
        continue
    fi

    dut_image="${DUT_IMAGES[${dut_name}]}"
    dut_ip="${DUT_IPS[${dut_name}]}"
    dut_port="${DUT_PORTS[${dut_name}]}"
    dut_sender_port="${DUT_SENDER_PORTS[${dut_name}]}"
    dut_receiver_port="${DUT_RECEIVER_PORTS[${dut_name}]}"
    result_file="${RESULTS_DIR}/${dut_name}.json"

    echo "-- ${dut_name} --"

    if ! start_dut "${dut_name}" "${dut_image}" "${dut_ip}"; then
        echo "  FAIL: could not start ${dut_name}"
        failed=$((failed + 1))
        failed_names="${failed_names:+${failed_names} }${dut_name}"
        continue
    fi

    if ! wait_dut_ready "${dut_name}"; then
        echo "  FAIL: ${dut_name} not ready"
        stop_dut "${dut_name}"
        failed=$((failed + 1))
        failed_names="${failed_names:+${failed_names} }${dut_name}"
        continue
    fi

    if run_perf_in_docker "${dut_name}" "${dut_ip}" "${dut_port}" "${dut_sender_port}" "${dut_receiver_port}" "${result_file}"; then
        echo "  PASS"
        passed=$((passed + 1))
        result_files="${result_files:+${result_files} }${result_file}"
    else
        echo "  FAIL"
        failed=$((failed + 1))
        failed_names="${failed_names:+${failed_names} }${dut_name}"
    fi

    stop_dut "${dut_name}"
    echo ""
done

# ── Reports ─────────────────────────────────────────────────────────────────

echo "--------------------------------------------"

if [ -n "${result_files}" ]; then
    echo ""
    echo "Comparison report:"
    echo ""
    # shellcheck disable=SC2086
    "${ZE_PERF}" report --md ${result_files} || true

    echo ""
    # shellcheck disable=SC2086
    "${ZE_PERF}" report --html ${result_files} > "${RESULTS_DIR}/report.html" 2>/dev/null || true
    echo "HTML report: ${RESULTS_DIR}/report.html"
fi

echo ""
if [ "${failed}" -eq 0 ]; then
    echo "PASS  ${passed} DUT(s) benchmarked"
else
    echo "FAIL  ${passed} passed, ${failed} failed: ${failed_names}"
fi
echo "--------------------------------------------"

exit $(( failed > 0 ? 1 : 0 ))
