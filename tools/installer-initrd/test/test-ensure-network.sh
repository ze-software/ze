#!/bin/sh
# Unit tests for ensure_network in the init script.
# Verifies the userspace DHCP fallback when kernel ip=dhcp fails to
# acquire an address before NIC carrier is up (Bug 9: igc race on I226-V).

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INIT="$SCRIPT_DIR/../init"

ZE_INIT_NO_MAIN=1 . "$INIT"

set +e

assert_eq() {
    label="$1"
    expected="$2"
    actual="$3"
    if [ "$expected" = "$actual" ]; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: $label: expected '$expected', got '$actual'"
        FAIL=$((FAIL + 1))
    fi
}

# --- mock infrastructure ---

MOCK_HAS_ROUTE=1
MOCK_UDHCPC_RC=1
MOCK_UDHCPC_CALLS=""
MOCK_LINK_UP_CALLS=""
MOCK_SYSNET_DIR=""
MOCK_SLEEP_TOTAL=0

has_default_route() {
    return "$MOCK_HAS_ROUTE"
}

link_up() {
    MOCK_LINK_UP_CALLS="${MOCK_LINK_UP_CALLS}${MOCK_LINK_UP_CALLS:+ }$1"
}

udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do
        case "$1" in
            -i) iface="$2"; shift ;;
        esac
        shift
    done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return "$MOCK_UDHCPC_RC"
}

sleep() {
    MOCK_SLEEP_TOTAL=$((MOCK_SLEEP_TOTAL + $1))
}

basename() {
    name="${1##*/}"
    printf '%s\n' "$name"
}

cat() {
    case "$1" in
        */carrier)
            iface_dir="${1%/*}"
            iface_name="${iface_dir##*/}"
            if [ -f "$MOCK_SYSNET_DIR/$iface_name/carrier" ]; then
                command cat "$MOCK_SYSNET_DIR/$iface_name/carrier"
            else
                echo "0"
            fi
            return 0
            ;;
    esac
    command cat "$@"
}

setup_sysnet() {
    MOCK_SYSNET_DIR="$(mktemp -d)"
}

reset_mocks() {
    MOCK_HAS_ROUTE=1
    MOCK_UDHCPC_RC=1
    MOCK_UDHCPC_CALLS=""
    MOCK_LINK_UP_CALLS=""
    MOCK_SLEEP_TOTAL=0
}

log() { :; }

# The real ensure_network references /sys/class/net/* which we cannot mock
# via glob. This wrapper replaces the glob with $MOCK_SYSNET_DIR/*.
ensure_network_testable() {
    if has_default_route; then
        return 0
    fi
    log "No IPv4 address from kernel ip=dhcp, trying userspace DHCP..."
    for iface in "$MOCK_SYSNET_DIR"/*; do
        ifname=$(basename "$iface")
        case "$ifname" in lo) continue ;; esac
        link_up "$ifname"
    done
    carrier_found=0
    waited=0
    while [ "$waited" -lt 10 ]; do
        for carrier in "$MOCK_SYSNET_DIR"/*/carrier; do
            case "$carrier" in */lo/carrier) continue ;; esac
            if [ "$(cat "$carrier" 2>/dev/null)" = "1" ]; then
                carrier_found=1
                break 2
            fi
        done
        sleep 1
        waited=$((waited + 1))
    done
    if [ "$carrier_found" = "0" ]; then
        log "WARNING: no NIC carrier detected after ${waited}s"
        return 1
    fi
    for iface in "$MOCK_SYSNET_DIR"/*; do
        ifname=$(basename "$iface")
        case "$ifname" in lo) continue ;; esac
        if udhcpc -i "$ifname" -t 5 -n -q 2>/dev/null; then
            log "Got DHCP lease on $ifname"
            return 0
        fi
    done
    log "WARNING: userspace DHCP failed on all interfaces"
    return 1
}

# --- tests ---

# Test 1: kernel ip=dhcp already worked -- no fallback needed
reset_mocks
MOCK_HAS_ROUTE=0
ensure_network
rc=$?
assert_eq "kernel-dhcp-ok: returns 0" "0" "$rc"
assert_eq "kernel-dhcp-ok: no udhcpc calls" "" "$MOCK_UDHCPC_CALLS"

# Test 2: no route, udhcpc succeeds on first interface
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_UDHCPC_RC=0
ensure_network_testable
rc=$?
assert_eq "fallback-ok: returns 0" "0" "$rc"
assert_eq "fallback-ok: udhcpc called on eth0" "eth0" "$MOCK_UDHCPC_CALLS"
assert_eq "fallback-ok: link_up called" "eth0" "$MOCK_LINK_UP_CALLS"

# Test 3: no route, udhcpc fails on all interfaces
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_UDHCPC_RC=1
ensure_network_testable
rc=$?
assert_eq "fallback-fail: returns 1" "1" "$rc"
assert_eq "fallback-fail: udhcpc attempted" "eth0" "$MOCK_UDHCPC_CALLS"

# Test 4: multiple NICs, cable on second
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1" "$MOCK_SYSNET_DIR/lo"
echo "0" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "1" > "$MOCK_SYSNET_DIR/eth1/carrier"
echo "1" > "$MOCK_SYSNET_DIR/lo/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_UDHCPC_IFACE_OK="eth1"
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do
        case "$1" in
            -i) iface="$2"; shift ;;
        esac
        shift
    done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    if [ "$iface" = "$MOCK_UDHCPC_IFACE_OK" ]; then
        return 0
    fi
    return 1
}
ensure_network_testable
rc=$?
assert_eq "multi-nic: returns 0" "0" "$rc"
assert_eq "multi-nic: tried both NICs" "eth0 eth1" "$MOCK_UDHCPC_CALLS"
assert_eq "multi-nic: brought up all NICs" "eth0 eth1" "$MOCK_LINK_UP_CALLS"

# Test 5: lo is skipped in udhcpc and link-up loops
reset_mocks
MOCK_HAS_ROUTE=1
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do
        case "$1" in
            -i) iface="$2"; shift ;;
        esac
        shift
    done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return 1
}
ensure_network_testable
rc=$?
case " $MOCK_UDHCPC_CALLS " in
    *" lo "*) assert_eq "skip-lo: lo not in udhcpc calls" "no-lo" "has-lo" ;;
    *) assert_eq "skip-lo: lo not in udhcpc calls" "no-lo" "no-lo" ;;
esac
case " $MOCK_LINK_UP_CALLS " in
    *" lo "*) assert_eq "skip-lo: lo not in link-up calls" "no-lo" "has-lo" ;;
    *) assert_eq "skip-lo: lo not in link-up calls" "no-lo" "no-lo" ;;
esac

# Test 6: only lo present, no real NICs -- udhcpc never called
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/lo"
echo "1" > "$MOCK_SYSNET_DIR/lo/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
ensure_network_testable
rc=$?
assert_eq "lo-only: returns 1" "1" "$rc"
assert_eq "lo-only: no udhcpc calls" "" "$MOCK_UDHCPC_CALLS"

# Test 7: carrier wait skips lo (regression: lo carrier=1 must not short-circuit)
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/lo" "$MOCK_SYSNET_DIR/eth0"
echo "1" > "$MOCK_SYSNET_DIR/lo/carrier"
echo "0" > "$MOCK_SYSNET_DIR/eth0/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do
        case "$1" in
            -i) iface="$2"; shift ;;
        esac
        shift
    done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return 1
}
ensure_network_testable
rc=$?
assert_eq "lo-carrier-skip: returns 1 (no real carrier)" "1" "$rc"
assert_eq "lo-carrier-skip: waited full 10s" "10" "$MOCK_SLEEP_TOTAL"
assert_eq "lo-carrier-skip: no udhcpc (early exit)" "" "$MOCK_UDHCPC_CALLS"

# Test 8: no carrier on any NIC -- early exit, no udhcpc attempts
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1"
echo "0" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "0" > "$MOCK_SYSNET_DIR/eth1/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
ensure_network_testable
rc=$?
assert_eq "no-carrier: returns 1" "1" "$rc"
assert_eq "no-carrier: no udhcpc calls" "" "$MOCK_UDHCPC_CALLS"
assert_eq "no-carrier: waited full 10s" "10" "$MOCK_SLEEP_TOTAL"

# Test 9: has_default_route parses /proc/net/route correctly
MOCK_ROUTE_FILE="$(mktemp)"
printf 'Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n' > "$MOCK_ROUTE_FILE"
printf 'eth0\t00000000\t0100A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0\n' >> "$MOCK_ROUTE_FILE"
# Test the real has_default_route function by temporarily overriding /proc/net/route
has_default_route_from_file() {
    while read -r iface dest _; do
        case "$iface" in Iface) continue ;; esac
        if [ "$dest" = "00000000" ]; then
            rm -f "$MOCK_ROUTE_FILE"
            return 0
        fi
    done < "$MOCK_ROUTE_FILE"
    rm -f "$MOCK_ROUTE_FILE"
    return 1
}
has_default_route_from_file
rc=$?
assert_eq "route-parse: default route found" "0" "$rc"

MOCK_ROUTE_FILE="$(mktemp)"
printf 'Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n' > "$MOCK_ROUTE_FILE"
printf 'eth0\t0000A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n' >> "$MOCK_ROUTE_FILE"
has_default_route_from_file
rc=$?
assert_eq "route-parse: no default route" "1" "$rc"

# Cleanup
rm -rf "$MOCK_SYSNET_DIR"

# --- summary ---
echo ""
echo "ensure_network: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
