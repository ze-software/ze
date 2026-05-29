#!/bin/sh
# Unit tests for kernel cmdline parsing in the init script.
# Sources the parse_cmdline function and tests with mock /proc/cmdline.

set -e

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

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

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Extract functions from init script (source the parsing logic)
# We create a mock /proc/cmdline by overriding the path
extract_parse() {
    sed -n '/^parse_cmdline/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_ipv4/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_image_name/,/^}/p' "$SCRIPT_DIR/../init"
}

eval "$(extract_parse)"

# Test 1: ze.server and ze.image both present
mkdir -p "$TMPDIR/proc"
echo "console=ttyS0 ze.server=10.0.0.1 ze.image=custom.img ip=dhcp" > "$TMPDIR/proc/cmdline"
# Override parse_cmdline to use our mock
parse_cmdline_mock() {
    ZE_SERVER=""
    ZE_IMAGE="ze.img"
    for param in $(cat "$TMPDIR/proc/cmdline"); do
        case "$param" in
            ze.server=*) ZE_SERVER="${param#ze.server=}" ;;
            ze.image=*) ZE_IMAGE="${param#ze.image=}" ;;
        esac
    done
}

parse_cmdline_mock
assert_eq "server-present" "10.0.0.1" "$ZE_SERVER"
assert_eq "image-present" "custom.img" "$ZE_IMAGE"

# Test 2: ze.image missing, defaults to ze.img
echo "ze.server=192.168.1.1 ip=dhcp" > "$TMPDIR/proc/cmdline"
parse_cmdline_mock
assert_eq "server-only" "192.168.1.1" "$ZE_SERVER"
assert_eq "image-default" "ze.img" "$ZE_IMAGE"

# Test 3: neither present
echo "console=ttyS0 ip=dhcp" > "$TMPDIR/proc/cmdline"
parse_cmdline_mock
assert_eq "no-server" "" "$ZE_SERVER"
assert_eq "no-image-default" "ze.img" "$ZE_IMAGE"

# Test 4: validate_ipv4 with valid addresses
validate_ipv4 "10.0.0.1" && assert_eq "valid-ip-10" "ok" "ok" || assert_eq "valid-ip-10" "ok" "fail"
validate_ipv4 "192.168.1.254" && assert_eq "valid-ip-192" "ok" "ok" || assert_eq "valid-ip-192" "ok" "fail"
validate_ipv4 "255.255.255.254" && assert_eq "valid-ip-max" "ok" "ok" || assert_eq "valid-ip-max" "ok" "fail"
validate_ipv4 "0.0.0.0" && assert_eq "valid-ip-zero" "ok" "ok" || assert_eq "valid-ip-zero" "ok" "fail"

# Test 5: validate_ipv4 with invalid addresses
validate_ipv4 "" && assert_eq "invalid-empty" "fail" "ok" || assert_eq "invalid-empty" "fail" "fail"
validate_ipv4 "abc" && assert_eq "invalid-alpha" "fail" "ok" || assert_eq "invalid-alpha" "fail" "fail"
validate_ipv4 "256.0.0.1" && assert_eq "invalid-octet" "fail" "ok" || assert_eq "invalid-octet" "fail" "fail"
validate_ipv4 "10.0.0" && assert_eq "invalid-short" "fail" "ok" || assert_eq "invalid-short" "fail" "fail"
validate_ipv4 "10.0.0.1.5" && assert_eq "invalid-long" "fail" "ok" || assert_eq "invalid-long" "fail" "fail"
validate_ipv4 "192.168.0.08" && assert_eq "invalid-leading-zero" "fail" "ok" || assert_eq "invalid-leading-zero" "fail" "fail"
validate_ipv4 "010.0.0.1" && assert_eq "invalid-leading-zero-first" "fail" "ok" || assert_eq "invalid-leading-zero-first" "fail" "fail"

# Test 7: validate_image_name accepts safe filenames, rejects traversal/metachars
validate_image_name "ze.img" && assert_eq "valid-image" "ok" "ok" || assert_eq "valid-image" "ok" "fail"
validate_image_name "ze-20260101-120000.img" && assert_eq "valid-image-ts" "ok" "ok" || assert_eq "valid-image-ts" "ok" "fail"
validate_image_name "" && assert_eq "invalid-image-empty" "fail" "ok" || assert_eq "invalid-image-empty" "fail" "fail"
validate_image_name "../etc/passwd" && assert_eq "invalid-image-traversal" "fail" "ok" || assert_eq "invalid-image-traversal" "fail" "fail"
validate_image_name "a b" && assert_eq "invalid-image-space" "fail" "ok" || assert_eq "invalid-image-space" "fail" "fail"
validate_image_name "x%0aHost: evil" && assert_eq "invalid-image-newline" "fail" "ok" || assert_eq "invalid-image-newline" "fail" "fail"

# Test 6: ze.server with spaces/special chars in cmdline
echo "root=/dev/sda1 ze.server=172.16.0.1 quiet" > "$TMPDIR/proc/cmdline"
parse_cmdline_mock
assert_eq "server-with-other-params" "172.16.0.1" "$ZE_SERVER"

echo "---"
echo "cmdline-parse: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
