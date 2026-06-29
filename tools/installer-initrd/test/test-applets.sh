#!/bin/sh
# Verifies the Makefile symlinks every external applet the init script relies
# on. `busybox --install` runs at boot, but the explicit symlinks are the
# defense-in-depth guarantee: an applet init uses that is missing here would
# 404 at install time and panic. printf regressed this once (used by init,
# absent from the list), which this test now guards against.

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MAKEFILE="$SCRIPT_DIR/../Makefile"

# Scope matching to the `for cmd in ... ; do` symlink list so unrelated
# Makefile text cannot mask a missing applet.
applet_block="$(sed -n '/for cmd in/,/; do/p' "$MAKEFILE")"

assert_listed() {
    applet="$1"
    if printf '%s\n' "$applet_block" | grep -qE "[[:space:]]$applet([[:space:]]|;)"; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: applet '$applet' used by init is not symlinked in the Makefile"
        FAIL=$((FAIL + 1))
    fi
}

# External applets the init script invokes (POSIX shell builtins excluded).
# stty: verify_shell_auth disables echo while the rescue-shell password is typed.
# udhcpc: dhcp_acquire runs it for the per-interface DHCP recovery.
for applet in sh cat echo printf mount umount mkdir sleep wget udhcpc dd sync \
    reboot poweroff blockdev basename rm mktemp mkfifo sha256sum tee wc tr \
    ip gunzip grep losetup mknod stty; do
    assert_listed "$applet"
done

echo ""
echo "applets: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
