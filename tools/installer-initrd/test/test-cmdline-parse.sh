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
    sed -n '/^validate_port/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_image_name/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_source/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_media_id/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_decimal/,/^}/p' "$SCRIPT_DIR/../init"
    sed -n '/^validate_target_path/,/^}/p' "$SCRIPT_DIR/../init"
}

eval "$(extract_parse)"

# Test 1: ze.server, ze.image, ze.port all present
mkdir -p "$TMPDIR/proc"
echo "console=ttyS0 ze.server=10.0.0.1 ze.image=custom.img ze.port=8080 ze.source=iso ze.target=/dev/vda ze.wait=0 ze.media-id=0123456789abcdef0123456789abcdef ip=dhcp" > "$TMPDIR/proc/cmdline"
# Override parse_cmdline to use our mock. Mirrors the real parse_cmdline (it
# reads /proc/cmdline directly, which cannot be faked when sourced).
parse_cmdline_mock() {
    ZE_SOURCE="http"
    ZE_SERVER=""
    ZE_IMAGE="ze.img"
    ZE_PORT="80"
    ZE_TARGET=""
    ZE_WAIT="30"
    ZE_MEDIA_ID=""
    for param in $(cat "$TMPDIR/proc/cmdline"); do
        case "$param" in
            ze.source=*) ZE_SOURCE="${param#ze.source=}" ;;
            ze.server=*) ZE_SERVER="${param#ze.server=}" ;;
            ze.image=*) ZE_IMAGE="${param#ze.image=}" ;;
            ze.port=*) ZE_PORT="${param#ze.port=}" ;;
            ze.target=*) ZE_TARGET="${param#ze.target=}" ;;
            ze.wait=*) ZE_WAIT="${param#ze.wait=}" ;;
            ze.media-id=*) ZE_MEDIA_ID="${param#ze.media-id=}" ;;
        esac
    done
}

parse_cmdline_mock
assert_eq "server-present" "10.0.0.1" "$ZE_SERVER"
assert_eq "image-present" "custom.img" "$ZE_IMAGE"
assert_eq "port-present" "8080" "$ZE_PORT"
assert_eq "source-present" "iso" "$ZE_SOURCE"
assert_eq "target-present" "/dev/vda" "$ZE_TARGET"
assert_eq "wait-present" "0" "$ZE_WAIT"
assert_eq "media-id-present" "0123456789abcdef0123456789abcdef" "$ZE_MEDIA_ID"

# Test 2: ze.image and ze.port missing, defaults apply
echo "ze.server=192.168.1.1 ip=dhcp" > "$TMPDIR/proc/cmdline"
parse_cmdline_mock
assert_eq "server-only" "192.168.1.1" "$ZE_SERVER"
assert_eq "image-default" "ze.img" "$ZE_IMAGE"
assert_eq "port-default" "80" "$ZE_PORT"
assert_eq "source-default" "http" "$ZE_SOURCE"
assert_eq "target-default" "" "$ZE_TARGET"
assert_eq "wait-default" "30" "$ZE_WAIT"
assert_eq "media-id-default" "" "$ZE_MEDIA_ID"

# Test 3: neither present
echo "console=ttyS0 ip=dhcp" > "$TMPDIR/proc/cmdline"
parse_cmdline_mock
assert_eq "no-server" "" "$ZE_SERVER"
assert_eq "no-image-default" "ze.img" "$ZE_IMAGE"
assert_eq "no-port-default" "80" "$ZE_PORT"
assert_eq "no-source-default" "http" "$ZE_SOURCE"
assert_eq "no-target-default" "" "$ZE_TARGET"
assert_eq "no-wait-default" "30" "$ZE_WAIT"
assert_eq "no-media-id-default" "" "$ZE_MEDIA_ID"

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

# Test 6b: validate_port accepts 1-65535, rejects empty/zero/out-of-range/non-numeric
validate_port "80" && assert_eq "valid-port-80" "ok" "ok" || assert_eq "valid-port-80" "ok" "fail"
validate_port "1" && assert_eq "valid-port-min" "ok" "ok" || assert_eq "valid-port-min" "ok" "fail"
validate_port "65535" && assert_eq "valid-port-max" "ok" "ok" || assert_eq "valid-port-max" "ok" "fail"
validate_port "" && assert_eq "invalid-port-empty" "fail" "ok" || assert_eq "invalid-port-empty" "fail" "fail"
validate_port "0" && assert_eq "invalid-port-zero" "fail" "ok" || assert_eq "invalid-port-zero" "fail" "fail"
validate_port "65536" && assert_eq "invalid-port-over" "fail" "ok" || assert_eq "invalid-port-over" "fail" "fail"
validate_port "abc" && assert_eq "invalid-port-alpha" "fail" "ok" || assert_eq "invalid-port-alpha" "fail" "fail"
validate_port "8080x" && assert_eq "invalid-port-suffix" "fail" "ok" || assert_eq "invalid-port-suffix" "fail" "fail"

# Test 7: validate_image_name accepts safe filenames, rejects traversal/metachars
validate_image_name "ze.img" && assert_eq "valid-image" "ok" "ok" || assert_eq "valid-image" "ok" "fail"
validate_image_name "ze-20260101-120000.img" && assert_eq "valid-image-ts" "ok" "ok" || assert_eq "valid-image-ts" "ok" "fail"
validate_image_name "" && assert_eq "invalid-image-empty" "fail" "ok" || assert_eq "invalid-image-empty" "fail" "fail"
validate_image_name "../etc/passwd" && assert_eq "invalid-image-traversal" "fail" "ok" || assert_eq "invalid-image-traversal" "fail" "fail"
validate_image_name "a b" && assert_eq "invalid-image-space" "fail" "ok" || assert_eq "invalid-image-space" "fail" "fail"
validate_image_name "x%0aHost: evil" && assert_eq "invalid-image-newline" "fail" "ok" || assert_eq "invalid-image-newline" "fail" "fail"


# Test 7b: validate_source accepts only explicit source modes.
validate_source "http" && assert_eq "valid-source-http" "ok" "ok" || assert_eq "valid-source-http" "ok" "fail"
validate_source "iso" && assert_eq "valid-source-iso" "ok" "ok" || assert_eq "valid-source-iso" "ok" "fail"
validate_source "" && assert_eq "invalid-source-empty" "fail" "ok" || assert_eq "invalid-source-empty" "fail" "fail"
validate_source "nfs" && assert_eq "invalid-source-nfs" "fail" "ok" || assert_eq "invalid-source-nfs" "fail" "fail"

# Test 7c: validate_media_id accepts lowercase 32-hex ids, rejects empty/bad chars/wrong length.
validate_media_id "0123456789abcdef0123456789abcdef" && assert_eq "valid-media-id" "ok" "ok" || assert_eq "valid-media-id" "ok" "fail"
validate_media_id "" && assert_eq "invalid-media-id-empty" "fail" "ok" || assert_eq "invalid-media-id-empty" "fail" "fail"
validate_media_id "0123456789ABCDEF0123456789ABCDEF" && assert_eq "invalid-media-id-upper" "fail" "ok" || assert_eq "invalid-media-id-upper" "fail" "fail"
validate_media_id "0123456789abcdef0123456789abcde" && assert_eq "invalid-media-id-short" "fail" "ok" || assert_eq "invalid-media-id-short" "fail" "fail"
validate_media_id "0123456789abcdef0123456789abcdeg" && assert_eq "invalid-media-id-char" "fail" "ok" || assert_eq "invalid-media-id-char" "fail" "fail"

# Test 7d: validate_target_path accepts whole block disks and rejects partitions/metacharacters.
validate_target_path "/dev/sda" && assert_eq "valid-target-sda" "ok" "ok" || assert_eq "valid-target-sda" "ok" "fail"
validate_target_path "/dev/vda" && assert_eq "valid-target-vda" "ok" "ok" || assert_eq "valid-target-vda" "ok" "fail"
validate_target_path "/dev/nvme0n1" && assert_eq "valid-target-nvme" "ok" "ok" || assert_eq "valid-target-nvme" "ok" "fail"
validate_target_path "/dev/mmcblk0" && assert_eq "valid-target-mmc" "ok" "ok" || assert_eq "valid-target-mmc" "ok" "fail"
validate_target_path "" && assert_eq "invalid-target-empty" "fail" "ok" || assert_eq "invalid-target-empty" "fail" "fail"
validate_target_path "vda" && assert_eq "invalid-target-relative" "fail" "ok" || assert_eq "invalid-target-relative" "fail" "fail"
validate_target_path "/dev/vda1" && assert_eq "invalid-target-partition" "fail" "ok" || assert_eq "invalid-target-partition" "fail" "fail"
validate_target_path "/dev/../sda" && assert_eq "invalid-target-traversal" "fail" "ok" || assert_eq "invalid-target-traversal" "fail" "fail"
# Test 6: ze.server with spaces/special chars in cmdline
echo "root=/dev/sda1 ze.server=172.16.0.1 quiet" > "$TMPDIR/proc/cmdline"
parse_cmdline_mock
assert_eq "server-with-other-params" "172.16.0.1" "$ZE_SERVER"

echo "---"
echo "cmdline-parse: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
