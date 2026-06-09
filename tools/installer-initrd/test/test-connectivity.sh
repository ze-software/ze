#!/bin/sh
# Unit tests for log_network_state and wait_for_server in the init script.
# Validates diagnostic output uses sysfs/procfs only (no ip command) and
# that the server probe distinguishes "unreachable" from "HTTP error."

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

assert_contains() {
    label="$1"
    haystack="$2"
    needle="$3"
    case "$haystack" in
        *"$needle"*)
            PASS=$((PASS + 1))
            ;;
        *)
            echo "FAIL: $label: output does not contain '$needle'"
            echo "  got: $haystack"
            FAIL=$((FAIL + 1))
            ;;
    esac
}

assert_not_contains() {
    label="$1"
    haystack="$2"
    needle="$3"
    case "$haystack" in
        *"$needle"*)
            echo "FAIL: $label: output should not contain '$needle'"
            echo "  got: $haystack"
            FAIL=$((FAIL + 1))
            ;;
        *)
            PASS=$((PASS + 1))
            ;;
    esac
}

# --- mocks ---

MOCK_SYSNET_DIR=""
MOCK_ROUTE_FILE=""
LOG_OUTPUT=""

sleep() { :; }

log() {
    LOG_OUTPUT="${LOG_OUTPUT}${LOG_OUTPUT:+
}$1"
}

basename() {
    name="${1##*/}"
    printf '%s\n' "$name"
}

cat() {
    command cat "$@"
}

setup_sysnet() {
    MOCK_SYSNET_DIR="$(mktemp -d)"
}

setup_route() {
    MOCK_ROUTE_FILE="$(mktemp)"
}

cleanup() {
    rm -rf "$MOCK_SYSNET_DIR" "$MOCK_ROUTE_FILE"
}

# Testable version replacing /sys/class/net/* glob and /proc/net/route
log_network_state_testable() {
    for iface in "$MOCK_SYSNET_DIR"/*; do
        [ -d "$iface" ] || continue
        ifname=$(basename "$iface")
        case "$ifname" in lo) continue ;; esac
        carrier="$(cat "$iface/carrier" 2>/dev/null || echo "?")"
        operstate="$(cat "$iface/operstate" 2>/dev/null || echo "?")"
        log "  $ifname: carrier=$carrier operstate=$operstate"
    done
    if [ -n "$MOCK_ROUTE_FILE" ] && [ -f "$MOCK_ROUTE_FILE" ]; then
        while read -r iface dest gw _ _ _ _ mask _; do
            case "$iface" in Iface) continue ;; esac
            log "  route: $iface dest=$dest gw=$gw mask=$mask"
        done < "$MOCK_ROUTE_FILE"
    fi
}

# --- log_network_state tests ---

# Test 1: single interface with carrier up
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "up" > "$MOCK_SYSNET_DIR/eth0/operstate"
LOG_OUTPUT=""
log_network_state_testable
assert_contains "single-up: shows carrier" "$LOG_OUTPUT" "carrier=1"
assert_contains "single-up: shows operstate" "$LOG_OUTPUT" "operstate=up"
assert_contains "single-up: shows ifname" "$LOG_OUTPUT" "eth0:"
cleanup

# Test 2: interface with carrier down
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/enp2s0"
echo "0" > "$MOCK_SYSNET_DIR/enp2s0/carrier"
echo "down" > "$MOCK_SYSNET_DIR/enp2s0/operstate"
LOG_OUTPUT=""
log_network_state_testable
assert_contains "single-down: shows carrier=0" "$LOG_OUTPUT" "carrier=0"
assert_contains "single-down: shows operstate=down" "$LOG_OUTPUT" "operstate=down"
cleanup

# Test 3: missing carrier file shows ?
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/eth0"
echo "up" > "$MOCK_SYSNET_DIR/eth0/operstate"
LOG_OUTPUT=""
log_network_state_testable
assert_contains "missing-carrier: shows ?" "$LOG_OUTPUT" "carrier=?"
cleanup

# Test 4: lo is skipped
setup_sysnet
mkdir -p "$MOCK_SYSNET_DIR/lo" "$MOCK_SYSNET_DIR/eth0"
echo "1" > "$MOCK_SYSNET_DIR/lo/carrier"
echo "up" > "$MOCK_SYSNET_DIR/lo/operstate"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "up" > "$MOCK_SYSNET_DIR/eth0/operstate"
LOG_OUTPUT=""
log_network_state_testable
assert_not_contains "skip-lo: lo not in output" "$LOG_OUTPUT" "lo:"
assert_contains "skip-lo: eth0 in output" "$LOG_OUTPUT" "eth0:"
cleanup

# Test 5: route table is logged
setup_sysnet
setup_route
mkdir -p "$MOCK_SYSNET_DIR/eth0"
echo "1" > "$MOCK_SYSNET_DIR/eth0/carrier"
echo "up" > "$MOCK_SYSNET_DIR/eth0/operstate"
printf 'Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n' > "$MOCK_ROUTE_FILE"
printf 'eth0\t00000000\t01FF13C6\t0003\t0\t0\t0\t00000000\t0\t0\t0\n' >> "$MOCK_ROUTE_FILE"
LOG_OUTPUT=""
log_network_state_testable
assert_contains "route: shows default" "$LOG_OUTPUT" "dest=00000000"
assert_contains "route: shows gw" "$LOG_OUTPUT" "gw=01FF13C6"
cleanup

# Test 6: no ip command is used
# Verify the real log_network_state source does not call ip addr
INIT_SRC="$(cat "$INIT")"
# Extract just the log_network_state function body
case "$INIT_SRC" in
    *"log_network_state()"*)
        fn_body="$(sed -n '/^log_network_state()/,/^}/p' "$INIT")"
        assert_not_contains "no-ip-addr: function body" "$fn_body" "ip addr"
        assert_not_contains "no-ip-show: function body" "$fn_body" "ip link show"
        ;;
    *)
        echo "FAIL: could not find log_network_state in init"
        FAIL=$((FAIL + 1))
        ;;
esac

# --- wait_for_server tests ---

MOCK_WGET_RC=0
MOCK_WGET_ERR=""
MOCK_WGET_CALLS=0

wget() {
    MOCK_WGET_CALLS=$((MOCK_WGET_CALLS + 1))
    # Write error to stderr dest if -q and 2>file pattern was used
    if [ -n "$MOCK_WGET_ERR" ]; then
        for arg in "$@"; do
            case "$arg" in
                /tmp/ze-probe.err)
                    echo "$MOCK_WGET_ERR" > "$arg"
                    ;;
            esac
        done
    fi
    return "$MOCK_WGET_RC"
}

grep() {
    command grep "$@"
}

# Test 7: server reachable on first attempt
MOCK_WGET_RC=0
MOCK_WGET_CALLS=0
LOG_OUTPUT=""
wait_for_server "198.19.255.1" "80" 30
rc=$?
assert_eq "reachable-first: returns 0" "0" "$rc"
assert_eq "reachable-first: one wget call" "1" "$MOCK_WGET_CALLS"
assert_contains "reachable-first: log says reachable" "$LOG_OUTPUT" "Server reachable"

# Test 8: server responds with HTTP error (404) -- still reachable
MOCK_WGET_RC=1
MOCK_WGET_ERR="wget: server returned error: HTTP/1.1 404 Not Found"
MOCK_WGET_CALLS=0
LOG_OUTPUT=""

# Mock wget to write error file for --spider path
wget() {
    MOCK_WGET_CALLS=$((MOCK_WGET_CALLS + 1))
    # Find the stderr redirect target
    echo "$MOCK_WGET_ERR" > /tmp/ze-probe.err
    return "$MOCK_WGET_RC"
}

wait_for_server "198.19.255.1" "80" 30
rc=$?
assert_eq "http-error-reachable: returns 0" "0" "$rc"
assert_contains "http-error-reachable: log mentions HTTP" "$LOG_OUTPUT" "got HTTP response"

# Test 9: server unreachable (can't connect) -- fails after max attempts
MOCK_WGET_RC=1
MOCK_WGET_ERR="wget: can't connect to remote host (198.19.255.1): Connection refused"
MOCK_WGET_CALLS=0
LOG_OUTPUT=""

wget() {
    MOCK_WGET_CALLS=$((MOCK_WGET_CALLS + 1))
    echo "$MOCK_WGET_ERR" > /tmp/ze-probe.err
    return "$MOCK_WGET_RC"
}

wait_for_server "198.19.255.1" "80" 30
rc=$?
assert_eq "unreachable: returns 1" "1" "$rc"
assert_eq "unreachable: tried all attempts" "30" "$MOCK_WGET_CALLS"
assert_contains "unreachable: log says not reachable" "$LOG_OUTPUT" "not reachable"

# Test 10: server becomes reachable on attempt 5
ATTEMPT_COUNTER=0
MOCK_WGET_CALLS=0
LOG_OUTPUT=""

wget() {
    MOCK_WGET_CALLS=$((MOCK_WGET_CALLS + 1))
    if [ "$MOCK_WGET_CALLS" -ge 5 ]; then
        return 0
    fi
    echo "wget: can't connect to remote host (198.19.255.1): Network is unreachable" > /tmp/ze-probe.err
    return 1
}

wait_for_server "198.19.255.1" "80" 30
rc=$?
assert_eq "delayed-connect: returns 0" "0" "$rc"
assert_eq "delayed-connect: connected on attempt 5" "5" "$MOCK_WGET_CALLS"

# Test 11: network state is logged during wait
MOCK_WGET_CALLS=0
LOG_OUTPUT=""

wget() {
    MOCK_WGET_CALLS=$((MOCK_WGET_CALLS + 1))
    echo "wget: can't connect to remote host (198.19.255.1): Connection refused" > /tmp/ze-probe.err
    return 1
}

wait_for_server "198.19.255.1" "80" 30
assert_contains "diag-logged: shows waiting" "$LOG_OUTPUT" "Waiting for server"
assert_contains "diag-logged: shows probe error" "$LOG_OUTPUT" "can't connect"

# Test 12: custom max_attempts=5 limits retries
MOCK_WGET_CALLS=0
LOG_OUTPUT=""

wget() {
    MOCK_WGET_CALLS=$((MOCK_WGET_CALLS + 1))
    echo "wget: can't connect to remote host (198.19.255.1): Connection refused" > /tmp/ze-probe.err
    return 1
}

wait_for_server "198.19.255.1" "80" 5
rc=$?
assert_eq "custom-limit: returns 1" "1" "$rc"
assert_eq "custom-limit: tried 5 attempts" "5" "$MOCK_WGET_CALLS"

# Test 13: max_attempts=0 skips probe entirely
MOCK_WGET_CALLS=0
LOG_OUTPUT=""
wait_for_server "198.19.255.1" "80" 0
rc=$?
assert_eq "skip-probe: returns 1" "1" "$rc"
assert_eq "skip-probe: no wget calls" "0" "$MOCK_WGET_CALLS"

# Test 14: grep is in the Makefile explicit symlink list (regression: fail-open
# if grep is missing because busybox --install fails silently)
MAKEFILE="$SCRIPT_DIR/../Makefile"
makefile_cmds="$(sed -n '/for cmd in/,/; do/p' "$MAKEFILE" | tr '\\\n' ' ')"
case "$makefile_cmds" in
    *" grep "*)
        PASS=$((PASS + 1))
        ;;
    *)
        echo "FAIL: grep-in-makefile: grep not in Makefile explicit symlink list"
        FAIL=$((FAIL + 1))
        ;;
esac

# Cleanup
rm -f /tmp/ze-probe.err

# --- summary ---
echo ""
echo "connectivity: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
