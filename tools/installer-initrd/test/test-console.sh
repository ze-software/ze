#!/bin/sh
# Unit tests for console fan-out (emit) and rescue-shell console selection
# (debug_console) in the init script. setup_console itself reads sysfs and
# filters to real /dev char devices, so it is exercised at boot rather than
# here; these tests cover the OS-portable selection and routing logic.

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INIT="$SCRIPT_DIR/../init"

# shellcheck disable=SC1090 # $INIT is dynamic; sourced to exercise init helpers
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

# --- debug_console: prefers a serial console (headless installs use serial) ---
CONSOLES="/dev/tty0 /dev/ttyS0"
assert_eq "debug_console: prefers serial over vga" "/dev/ttyS0" "$(debug_console)"

CONSOLES="/dev/ttyS1 /dev/ttyS0"
assert_eq "debug_console: first serial wins" "/dev/ttyS1" "$(debug_console)"

CONSOLES="/dev/ttyAMA0 /dev/tty0"
assert_eq "debug_console: ttyAMA0 counts as serial" "/dev/ttyAMA0" "$(debug_console)"

CONSOLES="/dev/tty0 /dev/ttyUSB0"
assert_eq "debug_console: ttyUSB counts as serial" "/dev/ttyUSB0" "$(debug_console)"

# --- debug_console: no serial -> last active console (kernel's preferred) ---
CONSOLES="/dev/tty0 /dev/tty1"
assert_eq "debug_console: falls back to last active" "/dev/tty1" "$(debug_console)"

# --- debug_console: empty -> /dev/console ---
CONSOLES=""
assert_eq "debug_console: empty falls back to /dev/console" "/dev/console" "$(debug_console)"

# --- emit: with no consoles known, writes to stdout ---
CONSOLES=""
out="$(emit "hello")"
assert_eq "emit: empty CONSOLES writes to stdout" "hello" "$out"

# --- emit: fans a line to every registered console ---
tmpdir="$(mktemp -d)"
CONSOLES="$tmpdir/c1 $tmpdir/c2"
emit "fan-line"
assert_eq "emit: wrote to console 1" "fan-line" "$(cat "$tmpdir/c1")"
assert_eq "emit: wrote to console 2" "fan-line" "$(cat "$tmpdir/c2")"
rm -rf "$tmpdir"

# --- log: prefixes and routes through emit ---
CONSOLES=""
out="$(log "msg")"
assert_eq "log: prefixes and routes through emit" "[ze-install] msg" "$out"

# --- setup_console: missing sysfs file is a no-op (no crash, CONSOLES empty) ---
CONSOLES=""
# shellcheck disable=SC2034 # read by setup_console (via the sourced init), not here
ZE_CONSOLE_ACTIVE="$(mktemp -u)/does-not-exist"
setup_console
assert_eq "setup_console: missing file leaves CONSOLES empty" "" "$CONSOLES"
unset ZE_CONSOLE_ACTIVE

# --- emit: if every console write fails, fall back to stdout (NOTE 3) ---
CONSOLES="$(mktemp -u)/nope/c1"   # parent dir absent, so the write fails
out="$(emit "kept")"
assert_eq "emit: unwritable console falls back to stdout" "kept" "$out"

# --- setup_console: a console with no /dev node must NOT abort under set -e ---
# Regression (BLOCKER): the for loop's final non-zero status used to propagate
# out of setup_console and kill PID 1, which runs under `set -e`. The tty names
# below have no /dev node on any host, so the failure reproduces deterministically.
active_tmp="$(mktemp)"
printf 'ttyzz0 ttyzz1\n' > "$active_tmp"
if (
    set -e
    CONSOLES=""
    # shellcheck disable=SC2034 # read by setup_console (via the sourced init)
    ZE_CONSOLE_ACTIVE="$active_tmp"
    setup_console
    true   # reached only if setup_console did not abort under set -e
); then
    assert_eq "setup_console: missing /dev nodes do not abort under set -e" "ok" "ok"
else
    assert_eq "setup_console: missing /dev nodes do not abort under set -e" "ok" "aborted"
fi
rm -f "$active_tmp"

echo ""
echo "console: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
