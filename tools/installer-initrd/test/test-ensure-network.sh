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
MOCK_WGET_RC=0
MOCK_WGET_CALLS=""
MOCK_IP_CALLS=""
ZE_SERVER="10.0.0.1"
ZE_PORT="80"

has_default_route() {
    return "$MOCK_HAS_ROUTE"
}

link_up() {
    MOCK_LINK_UP_CALLS="${MOCK_LINK_UP_CALLS}${MOCK_LINK_UP_CALLS:+ }$1"
}

wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    return "$MOCK_WGET_RC"
}

ip() {
    MOCK_IP_CALLS="${MOCK_IP_CALLS}${MOCK_IP_CALLS:+ }ip-$1-$2-$4"
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
    MOCK_WGET_RC=0
    MOCK_WGET_CALLS=""
    MOCK_IP_CALLS=""
    ZE_MAC=""          # default: no boot-NIC pin (existing tests scan all NICs)
}

log() { :; }

# The real ensure_network references /sys/class/net/* which we cannot mock
# via glob. This wrapper replaces the glob with $MOCK_SYSNET_DIR/*.
ensure_network_testable() {
    if has_default_route; then
        if server_reachable; then
            return 0
        fi
        log "Default route present but $ZE_SERVER:$ZE_PORT unreachable; retrying DHCP per interface..."
    fi
    # Mirror of the real ensure_network boot-NIC pinning block (ze.mac).
    if [ -n "$ZE_MAC" ]; then
        pin_if="$(iface_for_mac "$ZE_MAC")"
        if [ -n "$pin_if" ]; then
            log "Pinning install to boot NIC $pin_if (ze.mac=$ZE_MAC)"
            link_up "$pin_if"
            pin_waited=0
            while [ "$pin_waited" -lt 10 ]; do
                if [ "$(cat "$MOCK_SYSNET_DIR/$pin_if/carrier" 2>/dev/null)" = "1" ]; then
                    break
                fi
                sleep 1
                pin_waited=$((pin_waited + 1))
            done
            if dhcp_acquire "$pin_if"; then
                probe_err="/tmp/ze-ensure-probe.err"
                if wget -T 3 --spider -q "http://${ZE_SERVER}:${ZE_PORT}/" 2>"$probe_err" ||
                   grep -qE "server returned error|HTTP/" "$probe_err" 2>/dev/null; then
                    log "Got working network on boot NIC $pin_if (ze.mac)"
                    return 0
                fi
                log "boot NIC $pin_if cannot reach $ZE_SERVER:$ZE_PORT; flushing, scanning all NICs"
                ip addr flush dev "$pin_if" 2>/dev/null
                ip route del default 2>/dev/null
            fi
        else
            log "ze.mac=$ZE_MAC matches no interface; scanning all NICs"
        fi
    fi
    log "Acquiring a lease that can reach $ZE_SERVER via userspace DHCP..."
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
        if dhcp_acquire "$ifname"; then
            probe_err="/tmp/ze-ensure-probe.err"
            if wget -T 3 --spider -q "http://${ZE_SERVER}:${ZE_PORT}/" 2>"$probe_err" ||
               grep -qE "server returned error|HTTP/" "$probe_err" 2>/dev/null; then
                log "Got working network on $ifname"
                return 0
            fi
            log "$ifname: lease ok but cannot reach $ZE_SERVER:$ZE_PORT, trying next"
            ip addr flush dev "$ifname" 2>/dev/null
            ip route del default 2>/dev/null
        fi
    done
    log "WARNING: no interface could reach $ZE_SERVER:$ZE_PORT"
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

# Test 10 (Bug 15): multi-NIC, first lease cannot reach ze.server, second can
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "1" > "$MOCK_SYSNET_DIR/eth1/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_WGET_IFACE_OK="eth1"
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do
        case "$1" in
            -i) iface="$2"; shift ;;
        esac
        shift
    done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return 0
}
wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    if [ "$MOCK_WGET_IFACE_OK" = "done" ]; then
        return 0
    fi
    # After ip flush of a bad iface, the next udhcpc+wget pair is for the next iface.
    # Track which iface wget is probing via MOCK_UDHCPC_CALLS (last entry).
    last_iface="${MOCK_UDHCPC_CALLS##* }"
    if [ "$last_iface" = "$MOCK_WGET_IFACE_OK" ]; then
        MOCK_WGET_IFACE_OK="done"
        return 0
    fi
    return 1
}
ensure_network_testable
rc=$?
assert_eq "bug15-skip-bad: returns 0" "0" "$rc"
assert_eq "bug15-skip-bad: tried both NICs" "eth0 eth1" "$MOCK_UDHCPC_CALLS"
# eth0 should have been flushed
case "$MOCK_IP_CALLS" in
    *addr-flush-eth0*) assert_eq "bug15-skip-bad: flushed eth0" "yes" "yes" ;;
    *) assert_eq "bug15-skip-bad: flushed eth0" "yes" "no" ;;
esac

# Test 11 (Bug 15): first NIC can reach server, stops immediately
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "1" > "$MOCK_SYSNET_DIR/eth1/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_UDHCPC_RC=0
MOCK_WGET_RC=0
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
wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    return "$MOCK_WGET_RC"
}
ensure_network_testable
rc=$?
assert_eq "bug15-first-ok: returns 0" "0" "$rc"
assert_eq "bug15-first-ok: only tried eth0" "eth0" "$MOCK_UDHCPC_CALLS"
assert_eq "bug15-first-ok: no flush" "" "$MOCK_IP_CALLS"

# Test 12 (Bug 15): all NICs get leases but none can reach server
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "1" > "$MOCK_SYSNET_DIR/eth1/carrier"
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_UDHCPC_RC=0
MOCK_WGET_RC=1
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
wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    return "$MOCK_WGET_RC"
}
ensure_network_testable
rc=$?
assert_eq "bug15-all-unreachable: returns 1" "1" "$rc"
assert_eq "bug15-all-unreachable: tried both" "eth0 eth1" "$MOCK_UDHCPC_CALLS"
assert_eq "bug15-all-unreachable: flushed both" "ip-addr-flush-eth0 ip-route-del- ip-addr-flush-eth1 ip-route-del-" "$MOCK_IP_CALLS"

# Test 13 (foreign DHCP): kernel ip=dhcp installed a default route via a
# foreign/corporate DHCP with no route to ze.server. ensure_network must NOT
# trust it; it must re-run per-interface DHCP and pick the reachable interface.
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "1" > "$MOCK_SYSNET_DIR/eth1/carrier"
reset_mocks
MOCK_HAS_ROUTE=0          # kernel default route present (foreign lease)
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do case "$1" in -i) iface="$2"; shift ;; esac; shift; done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return 0
}
wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    last_iface="${MOCK_UDHCPC_CALLS##* }"
    [ "$last_iface" = "eth1" ] && return 0
    return 1
}
ensure_network_testable
rc=$?
assert_eq "foreign-dhcp: returns 0" "0" "$rc"
assert_eq "foreign-dhcp: re-ran per-iface DHCP" "eth0 eth1" "$MOCK_UDHCPC_CALLS"
case "$MOCK_IP_CALLS" in
    *addr-flush-eth0*) assert_eq "foreign-dhcp: flushed unreachable eth0" "yes" "yes" ;;
    *) assert_eq "foreign-dhcp: flushed unreachable eth0" "yes" "no" ;;
esac

# Test 14 (ze.mac): iface_for_mac resolves the interface whose MAC matches,
# is case-insensitive, skips lo, and returns nothing for an unknown MAC.
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1" "$MOCK_SYSNET_DIR/lo"
echo "00:11:22:33:44:55" > "$MOCK_SYSNET_DIR/eth0/address"
echo "60:be:b4:22:2d:46" > "$MOCK_SYSNET_DIR/eth1/address"
echo "00:00:00:00:00:00" > "$MOCK_SYSNET_DIR/lo/address"
ZE_SYSNET_DIR="$MOCK_SYSNET_DIR"
assert_eq "iface_for_mac: matches eth1 by MAC" "eth1" "$(iface_for_mac 60:be:b4:22:2d:46)"
assert_eq "iface_for_mac: case-insensitive" "eth1" "$(iface_for_mac 60:BE:B4:22:2D:46)"
assert_eq "iface_for_mac: unknown MAC -> empty" "" "$(iface_for_mac de:ad:be:ef:00:00)"
ZE_SYSNET_DIR=""

# Test 15 (ze.mac): boot NIC is pinned; a second NIC on a foreign (corporate)
# network is NEVER brought up or DHCP'd. eth0=corporate, eth1=boot NIC.
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0" "$MOCK_SYSNET_DIR/eth1"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "1" > "$MOCK_SYSNET_DIR/eth1/carrier"
echo "00:11:22:33:44:55" > "$MOCK_SYSNET_DIR/eth0/address"
echo "60:be:b4:22:2d:46" > "$MOCK_SYSNET_DIR/eth1/address"
reset_mocks
MOCK_HAS_ROUTE=0          # corporate kernel default route present, unreachable
ZE_MAC="60:be:b4:22:2d:46"
ZE_SYSNET_DIR="$MOCK_SYSNET_DIR"
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do case "$1" in -i) iface="$2"; shift ;; esac; shift; done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return 0
}
wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    last_iface="${MOCK_UDHCPC_CALLS##* }"   # reachable only after the boot NIC is up
    [ "$last_iface" = "eth1" ] && return 0
    return 1
}
ensure_network_testable
rc=$?
assert_eq "ze-mac-pin: returns 0" "0" "$rc"
assert_eq "ze-mac-pin: only the boot NIC DHCP'd" "eth1" "$MOCK_UDHCPC_CALLS"
assert_eq "ze-mac-pin: only the boot NIC brought up" "eth1" "$MOCK_LINK_UP_CALLS"
ZE_SYSNET_DIR=""

# Test 16 (ze.mac): a MAC matching no interface falls back to the all-NIC scan.
rm -rf "$MOCK_SYSNET_DIR"
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "00:11:22:33:44:55" > "$MOCK_SYSNET_DIR/eth0/address"
reset_mocks
MOCK_HAS_ROUTE=1
MOCK_UDHCPC_RC=0
MOCK_WGET_RC=0
ZE_MAC="de:ad:be:ef:00:00"   # matches nothing
ZE_SYSNET_DIR="$MOCK_SYSNET_DIR"
udhcpc() {
    iface=""
    while [ $# -gt 0 ]; do case "$1" in -i) iface="$2"; shift ;; esac; shift; done
    MOCK_UDHCPC_CALLS="${MOCK_UDHCPC_CALLS}${MOCK_UDHCPC_CALLS:+ }$iface"
    return "$MOCK_UDHCPC_RC"
}
wget() {
    MOCK_WGET_CALLS="${MOCK_WGET_CALLS}${MOCK_WGET_CALLS:+ }wget"
    return "$MOCK_WGET_RC"
}
ensure_network_testable
rc=$?
assert_eq "ze-mac-nomatch: returns 0 via scan" "0" "$rc"
assert_eq "ze-mac-nomatch: scanned eth0" "eth0" "$MOCK_UDHCPC_CALLS"
ZE_SYSNET_DIR=""

# Cleanup
rm -rf "$MOCK_SYSNET_DIR"

# --- summary ---
echo ""
echo "ensure_network: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
