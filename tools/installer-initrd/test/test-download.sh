#!/bin/sh
# Unit tests for download_to_disk in the init script.
#
# Regression target: a `wget | dd` pipeline reports only dd's exit status, so
# a wget that dies mid-stream would leave a truncated image silently accepted
# and then booted on a freshly wiped disk. download_to_disk must instead fail
# (and retry, then give up) whenever wget fails OR the written bytes do not
# match the expected checksum.

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INIT="$SCRIPT_DIR/../init"

# Source the real init script (functions only -- the guard suppresses main).
ZE_INIT_NO_MAIN=1 . "$INIT"

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

# --- mocks (shell functions shadow the real commands) ---

# sleep is mocked so retry backoff does not slow the test.
sleep() { :; }

# wget writes MOCK_PAYLOAD to stdout and exits MOCK_WGET_RC. The first arg may
# be a .sha256 URL (used by main, not under test here) -- this mock only ever
# serves the image stream path.
wget() {
    [ "${MOCK_WGET_RC:-0}" -eq 0 ] || return "${MOCK_WGET_RC}"
    printf '%s' "${MOCK_PAYLOAD:-IMAGE}"
    return 0
}

DD_COUNT_FILE="$(mktemp)"
echo 0 > "$DD_COUNT_FILE"

# dd drains stdin (so tee never blocks). It fails for the first
# MOCK_DD_FAIL_UNTIL-1 invocations (to exercise retry), then exits MOCK_DD_RC.
dd() {
    cat >/dev/null 2>&1
    n=$(cat "$DD_COUNT_FILE" 2>/dev/null || echo 0)
    n=$((n + 1))
    echo "$n" > "$DD_COUNT_FILE"
    if [ "$n" -lt "${MOCK_DD_FAIL_UNTIL:-0}" ]; then
        return 1
    fi
    return "${MOCK_DD_RC:-0}"
}

# sha256sum drains stdin and emits a controllable digest.
sha256sum() {
    cat >/dev/null 2>&1
    printf '%s  -\n' "${MOCK_HASH:-deadbeef}"
}

run_download() {
    # $1 expected_sha. Toggle set -e (inherited from init) so a non-zero
    # return is captured rather than aborting the test.
    set +e
    download_to_disk "http://server/install/image/ze.img" "/dev/null" "$1" >/dev/null 2>&1
    rc=$?
    set -e
}

run_read_expected_sha() {
    # $1 content. Captures stdout and status without letting set -e abort.
    sha_file="$(mktemp)"
    printf '%s' "$1" > "$sha_file"
    set +e
    sha_out="$(read_expected_sha "$sha_file" 2>/dev/null)"
    sha_rc=$?
    set -e
    rm -f "$sha_file"
}

run_local_image() {
    sha_file="$(mktemp)"
    printf '%s  ze.img\n' "$1" > "$sha_file"
    set +e
    local_image_to_disk "/mock/ze.img" "$sha_file" "/dev/null" >/dev/null 2>&1
    local_rc=$?
    set -e
    rm -f "$sha_file"
}


# Test 1: clean transfer, no checksum requested -> success.
MOCK_WGET_RC=0 MOCK_DD_RC=0 MOCK_PAYLOAD="IMAGE"
run_download ""
assert_eq "clean-no-checksum" "0" "$rc"

# Test 2 (regression): wget fails but dd succeeds -> MUST fail, not silently pass.
MOCK_WGET_RC=1 MOCK_DD_RC=0 MOCK_PAYLOAD="IMAGE"
run_download ""
assert_eq "wget-fail-dd-ok" "1" "$rc"

# Test 3: dd itself fails -> failure.
MOCK_WGET_RC=0 MOCK_DD_RC=1 MOCK_PAYLOAD="IMAGE"
run_download ""
assert_eq "dd-fail" "1" "$rc"

# Test 4: checksum matches -> success.
MOCK_WGET_RC=0 MOCK_DD_RC=0 MOCK_PAYLOAD="IMAGE" MOCK_HASH="abc123"
run_download "abc123"
assert_eq "checksum-match" "0" "$rc"

# Test 5: checksum mismatch -> failure (corrupt/truncated write rejected).
MOCK_WGET_RC=0 MOCK_DD_RC=0 MOCK_PAYLOAD="IMAGE" MOCK_HASH="abc123"
run_download "deadbeef"
assert_eq "checksum-mismatch" "1" "$rc"

# Test 6: wget failure leaves the failure marker behind.
MOCK_WGET_RC=1 MOCK_DD_RC=0 MOCK_PAYLOAD="IMAGE"
run_download ""
if [ -f "$WGET_FAIL_MARKER" ]; then
    PASS=$((PASS + 1))
else
    echo "FAIL: marker-written: expected $WGET_FAIL_MARKER to exist"
    FAIL=$((FAIL + 1))
fi

# Test 7 (set -e safety): a dd failure on the first attempt must flow into the
# retry loop, not abort the script. Called standalone under `set -e` -- the
# earlier standalone-pipeline form aborted here before retrying. The command
# substitution runs in a subshell with `set -e`; if download_to_disk aborts
# mid-function, the trailing `echo` never runs and the result is empty.
echo 0 > "$DD_COUNT_FILE"
result="$(MOCK_WGET_RC=0 MOCK_DD_RC=0 MOCK_DD_FAIL_UNTIL=2 MOCK_PAYLOAD="IMAGE"
          set -e
          download_to_disk "http://server/install/image/ze.img" "/dev/null" "" >/dev/null 2>&1
          echo "RC=$?")"
assert_eq "set-e-retry-no-abort" "RC=0" "$result"

VALID_SHA="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
run_read_expected_sha "${VALID_SHA}  ze.img
"
assert_eq "sha-sidecar-valid" "0" "$sha_rc"
assert_eq "sha-sidecar-valid-value" "$VALID_SHA" "$sha_out"

run_read_expected_sha ""
assert_eq "sha-sidecar-empty-rejected" "1" "$sha_rc"

run_read_expected_sha "not-a-sha  ze.img
"
assert_eq "sha-sidecar-malformed-rejected" "1" "$sha_rc"

run_read_expected_sha "0123  ze.img
"
assert_eq "sha-sidecar-short-rejected" "1" "$sha_rc"


echo 0 > "$DD_COUNT_FILE"
MOCK_HASH="$VALID_SHA"
run_local_image "$VALID_SHA"
assert_eq "local-checksum-match" "0" "$local_rc"
assert_eq "local-checksum-match-dd" "1" "$(cat "$DD_COUNT_FILE")"

echo 0 > "$DD_COUNT_FILE"
MOCK_HASH="$VALID_SHA"
run_local_image "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
assert_eq "local-checksum-mismatch" "1" "$local_rc"
assert_eq "local-checksum-mismatch-no-dd" "0" "$(cat "$DD_COUNT_FILE")"

rm -f "$WGET_FAIL_MARKER" "$HASH_FILE" "$DD_COUNT_FILE"

echo "---"
echo "download: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
