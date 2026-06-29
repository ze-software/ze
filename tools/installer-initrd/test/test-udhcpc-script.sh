#!/bin/sh
# Unit tests for udhcpc.script, the udhcpc lease handler. busybox udhcpc only
# negotiates the lease and applies nothing itself; this script must configure
# the interface via `ip` so ensure_network's per-interface DHCP recovery works.
# We stub `ip` as a shell function and run the script in a subshell, capturing
# the `ip` invocations it issues.

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$SCRIPT_DIR/../udhcpc.script"

assert_contains() {
    label="$1"
    needle="$2"
    hay="$3"
    case "$hay" in
        *"$needle"*) PASS=$((PASS + 1)) ;;
        *)
            echo "FAIL: $label: expected to find '$needle' in:"
            printf '%s\n' "$hay"
            FAIL=$((FAIL + 1))
            ;;
    esac
}

refute_contains() {
    label="$1"
    needle="$2"
    hay="$3"
    case "$hay" in
        *"$needle"*)
            echo "FAIL: $label: did not expect '$needle' in:"
            printf '%s\n' "$hay"
            FAIL=$((FAIL + 1))
            ;;
        *) PASS=$((PASS + 1)) ;;
    esac
}

# run ACTION INTERFACE IP SUBNET ROUTER -> echoes the captured `ip` commands.
# The script is sourced in a subshell with `ip` stubbed; its `exit 0` ends the
# subshell, not the test. Note `ip` is both a lease variable ($ip) and the
# command (ip addr ...) -- shell keeps the two in separate namespaces, exactly
# as busybox udhcpc invokes the handler.
run() {
    r_cap="$(mktemp)"
    r_action="$1"
    r_if="$2"
    r_ip="$3"
    r_subnet="$4"
    r_router="$5"
    (
        # shellcheck disable=SC2317,SC2329 # invoked indirectly by the sourced script
        ip() { printf 'ip %s\n' "$*" >> "$r_cap"; }
        # interface/ip/subnet/router are read by the sourced handler, not here.
        # shellcheck disable=SC2034
        interface="$r_if"
        # shellcheck disable=SC2034
        ip="$r_ip"
        # shellcheck disable=SC2034
        subnet="$r_subnet"
        # shellcheck disable=SC2034
        router="$r_router"
        set -- "$r_action"
        # shellcheck disable=SC1090 # $SCRIPT is the handler under test
        . "$SCRIPT"
    )
    r_out="$(cat "$r_cap")"
    rm -f "$r_cap"
    printf '%s' "$r_out"
}

# /24: dotted netmask 255.255.255.0 -> prefix 24, address and default route set.
out="$(run bound eth0 10.0.0.5 255.255.255.0 10.0.0.1)"
assert_contains "bound /24: flushes first" "ip addr flush dev eth0" "$out"
assert_contains "bound /24: adds address with prefix" "ip addr add 10.0.0.5/24 dev eth0" "$out"
assert_contains "bound /24: adds default route" "ip route add default via 10.0.0.1 dev eth0" "$out"

# /16: 255.255.0.0 -> prefix 16.
out="$(run bound eth0 172.16.4.9 255.255.0.0 172.16.0.1)"
assert_contains "bound /16: prefix 16" "ip addr add 172.16.4.9/16 dev eth0" "$out"

# /25: 255.255.255.128 -> prefix 25 (non-octet-aligned mask).
out="$(run bound eth1 192.168.1.10 255.255.255.128 192.168.1.1)"
assert_contains "bound /25: prefix 25" "ip addr add 192.168.1.10/25 dev eth1" "$out"

# No subnet supplied -> default /24 so the address is still usable.
out="$(run bound eth0 10.1.2.3 '' 10.1.2.1)"
assert_contains "bound no-subnet: defaults to /24" "ip addr add 10.1.2.3/24 dev eth0" "$out"

# No router supplied -> address set, but no default route is added.
out="$(run bound eth0 10.0.0.5 255.255.255.0 '')"
assert_contains "bound no-router: still sets address" "ip addr add 10.0.0.5/24 dev eth0" "$out"
refute_contains "bound no-router: no default route" "route add default" "$out"

# deconfig -> flush only, no address or route added.
out="$(run deconfig eth0 '' '' '')"
assert_contains "deconfig: flushes interface" "ip addr flush dev eth0" "$out"
refute_contains "deconfig: adds no address" "addr add" "$out"
refute_contains "deconfig: adds no route" "route add" "$out"

# bound with empty IP (defensive) -> configures nothing.
out="$(run bound eth0 '' 255.255.255.0 10.0.0.1)"
refute_contains "bound empty-ip: adds no address" "addr add" "$out"

echo ""
echo "udhcpc-script: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
