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

# --- rescue-shell password gate (fix #3) ---
# password_matches + verify_shell_auth hash the typed password with sha256sum;
# guard on its presence so the suite still runs on a host without it (the
# busybox installer always has it).
if command -v sha256sum >/dev/null 2>&1; then
    EXPECT="$(printf '%s' 'sup3r-secret' | sha256sum)"
    EXPECT="${EXPECT%% *}"

    # password_matches: only the correct password against a set hash matches.
    ZE_SHELL_AUTH="$EXPECT"
    if password_matches "sup3r-secret"; then pm_rc=0; else pm_rc=1; fi
    assert_eq "password_matches: correct password" "0" "$pm_rc"
    if password_matches "wrong"; then pm_rc=0; else pm_rc=1; fi
    assert_eq "password_matches: wrong password" "1" "$pm_rc"
    ZE_SHELL_AUTH=""
    if password_matches "sup3r-secret"; then pm_rc=0; else pm_rc=1; fi
    assert_eq "password_matches: empty auth never matches (fail-closed)" "1" "$pm_rc"

    # verify_shell_auth: forks a shell ONLY after the correct password. stty and
    # start_shell are stubbed; the password is fed on stdin via a here-doc (no
    # subshell, so START_SHELL_CALLED is visible here).
    stty() { :; }
    start_shell() { START_SHELL_CALLED=1; }
    ZE_SHELL_AUTH="$EXPECT"

    START_SHELL_CALLED=0
    verify_shell_auth >/dev/null 2>&1 <<EOF
sup3r-secret
EOF
    assert_eq "verify_shell_auth: correct password starts shell" "1" "$START_SHELL_CALLED"

    START_SHELL_CALLED=0
    verify_shell_auth >/dev/null 2>&1 <<EOF
nope1
nope2
nope3
EOF
    vsa_rc=$?
    assert_eq "verify_shell_auth: wrong password starts no shell" "0" "$START_SHELL_CALLED"
    assert_eq "verify_shell_auth: wrong password returns 1" "1" "$vsa_rc"
else
    echo "SKIP: sha256sum unavailable; rescue-shell gate tests skipped"
fi

# fatal policy by trust context (fix #3 + review finding #1: ISO had no
# ze.shell-auth and must NOT fail-closed/reboot-loop -- the operator controls the
# physical media). Force the no-CONSOLES fallback via a non-device debug_console.
debug_console() { echo "/nonexistent-console"; }
CONSOLES=""
start_shell() { START_SHELL_CALLED=1; }
reboot() { REBOOT_CALLED=1; }
poweroff() { POWEROFF_CALLED=1; }
sleep() { :; }

# network install, no credential -> fail closed (reboot, never a shell).
ZE_SHELL_AUTH=""
ZE_SOURCE="http"
START_SHELL_CALLED=0; REBOOT_CALLED=0; POWEROFF_CALLED=0
fatal "net boom" >/dev/null 2>&1
assert_eq "fatal: http no-cred fails closed (reboot)" "1" "$REBOOT_CALLED"
assert_eq "fatal: http no-cred starts no shell" "0" "$START_SHELL_CALLED"

# ISO install, no credential -> ungated rescue shell, then poweroff (not reboot).
ZE_SHELL_AUTH=""
ZE_SOURCE="iso"
START_SHELL_CALLED=0; REBOOT_CALLED=0; POWEROFF_CALLED=0
fatal "iso boom" >/dev/null 2>&1
assert_eq "fatal: ISO no-cred opens rescue shell" "1" "$START_SHELL_CALLED"
assert_eq "fatal: ISO no-cred powers off" "1" "$POWEROFF_CALLED"
assert_eq "fatal: ISO no-cred does not reboot" "0" "$REBOOT_CALLED"

# Structure: the all-console loop lives in rescue_on_all_consoles; fatal routes
# to it for the credentialed (gated) and ISO (open) paths.
roac_src="$(sed -n '/^rescue_on_all_consoles()/,/^}/p' "$INIT")"
case "$roac_src" in
    *'for con in $CONSOLES'*) assert_eq "rescue_on_all_consoles: iterates all consoles" "yes" "yes" ;;
    *) assert_eq "rescue_on_all_consoles: iterates all consoles" "yes" "no" ;;
esac
fatal_src="$(sed -n '/^fatal()/,/^}/p' "$INIT")"
case "$fatal_src" in
    *"rescue_on_all_consoles gated"*) assert_eq "fatal: gated (credentialed) path" "yes" "yes" ;;
    *) assert_eq "fatal: gated (credentialed) path" "yes" "no" ;;
esac
case "$fatal_src" in
    *"rescue_on_all_consoles open"*) assert_eq "fatal: ISO open-shell path" "yes" "yes" ;;
    *) assert_eq "fatal: ISO open-shell path" "yes" "no" ;;
esac

echo ""
echo "console: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
