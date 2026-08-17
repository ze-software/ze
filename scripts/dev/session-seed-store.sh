#!/bin/bash
# Seed this session's own ze store, once, from the ze_core binary just built.
#
# A ze binary derives its config and database directory from its OWN location
# (internal/core/paths/paths.go, ConfigDirFromBinary: <prefix>/bin/<name> ->
# <prefix>/etc/ze), so a session's binary at
# tmp/session/<YYYY-MM-DD>-<sid>/bin/<name> resolves the session-local
# <session-dir>/etc/ze and never the repository's. That isolation is the intent
# (mk/session.mk). An EMPTY store is not: internal/component/config/storage/
# blob.go NewBlob calls zefs.Create and returns a nil error when the blob is
# absent, so an unseeded session gets no error at all -- it gets a daemon with
# no users and a fresh SSH host key, and finds out later. This script is what
# makes the isolated store a seeded store.
#
# The CALLER names the binary, and every ze_core binary is an equal seeder: ze,
# ze-appliance and ze-stripped all link internal/core/resolve (the silent
# NewBlob path), internal/component/ssh (the host key) and internal/plugins/init
# (`ze init` itself, registered in cmd/ze/ze_core_dispatch.go under //go:build
# ze_core). Measured with `go list -deps` over each recipe's tag set. A session
# that builds only ze-stripped therefore seeds its store from ze-stripped, and
# no recipe needs a binary it did not ask for. ze-setup, ze-test, ze-chaos,
# ze-analyze and ze-perf are NOT seeders: none of them links plugins/init, and
# none reaches the silent NewBlob path.
#
# The credentials are GENERATED per session and nothing is tracked: 24 random
# bytes from /dev/urandom, written 0600 into this session's own
# <session-dir>/etc/ze/.dev-password. tmp/ is gitignored (.gitignore, `tmp/*`),
# so the password cannot reach a commit, and two sessions never share one.
#
# Idempotent on the database's own existence: a second `make ze-build` finds
# database.zefs and returns without reseeding and without rotating the password.
#
# No --force, deliberately. Its only effect is to move an EXISTING database
# aside, and this script must never do that: it runs only when there is none,
# and if the binary ever resolved the operator's <repo>/etc/ze instead,
# `--force --yes` would displace the operator's real database without asking.
# Without it `ze init` refuses with "database already exists", which is the
# failure this script wants. --seed is required: it tells init to skip this
# host's interface discovery, which a dev store has no use for.
#
# Every failure is loud. A silently empty store is the one outcome this script
# exists to prevent, so a refused path, a missing binary, a failed init, or an
# init that reports success without leaving a database all exit non-zero and
# fail the build.
#
# Usage (mk/session.mk, on-session only; run from the checkout root, the path is
# root-relative):
#   scripts/dev/session-seed-store.sh tmp/session/<YYYY-MM-DD>-<sid>/bin/<name>

set -u

fail() {
    echo "session-seed-store: $1" >&2
    exit 1
}

if [ "$#" -ne 1 ]; then
    echo "usage: session-seed-store.sh <session-binary>" >&2
    exit 2
fi
ze="$1"

# Fail closed on any path that is not a binary in a session bin directory. The
# binary's own path is what decides where it writes, so bin/<name> (the shared,
# off-session directory) and anything else outside tmp/session/ is refused
# rather than served: seeding there would write into the operator's own etc/ze.
bindir="${ze%/*}"
session="${bindir%/bin}"
case "$ze" in
    tmp/session/????-??-??-*/bin/*) ;;
    *) fail "refusing $ze: not a binary in a session bin directory" ;;
esac
case "$ze" in
    */*/*/*/*/*) fail "refusing $ze: deeper than tmp/session/<dated-id>/bin/<name>" ;;
esac

etcdir="$session/etc/ze"
db="$etcdir/database.zefs"

# Already seeded. Nothing is reseeded and no password is rotated.
[ -e "$db" ] && exit 0

[ -x "$ze" ] || fail "no binary at $ze"

# Refuse when the environment overrides the config directory: ze would then seed
# a directory this script cannot vouch for, and `ze init` is not a command to
# point at an unknown store. The comparison mirrors normalize() in
# internal/core/env/env.go -- lowercase, dots and underscores equivalent -- so
# every spelling of the override is caught, not just ZE_CONFIG_DIR.
#
# The names come from `env` rather than from `compgen -e`, because env.Set()
# calls os.Setenv with the CANONICAL dotted key, and bash cannot hold a variable
# named ze.config.dir at all: any ze process that set the override exports a
# name the shell's own variable list does not show. A here-string rather than a
# pipe, so `fail` exits this shell instead of a subshell of it.
while IFS= read -r entry; do
    case "$(printf '%s' "${entry%%=*}" | tr '.A-Z' '_a-z')" in
        ze_config_dir)
            fail "${entry%%=*} overrides the config directory; unset it to seed $etcdir"
            ;;
    esac
done <<< "$(env)"

# 0700 directories and 0600 files for everything below: the store holds
# credentials and the password file is a real secret, throwaway or not.
umask 077

mkdir -p "$etcdir" || fail "cannot create $etcdir"

# ONE seeder at a time. `make build` names $(ZEBIN_ZE), $(ZEBIN_APPLIANCE) and
# $(ZEBIN_STRIPPED) as prerequisites and each calls this script, so `make -j` on
# a fresh session runs three of them at once. The existence test above is a
# check-then-act: all three see no database, all three reach `ze init`, and they
# then race on one database file. mkdir is the atomic primitive here -- it
# succeeds for exactly one caller -- so the losers wait for the winner's
# postcondition instead of writing beside it.
#
# The wait is bounded. A build killed between the mkdir and the trap leaves the
# lock behind, and a stale lock that blocked forever would be worse than the
# race: after the bound, a caller that still sees no database says so and fails
# loudly, which is this script's contract for every other failure too.
lockdir="$etcdir/.seed-lock"
if ! mkdir "$lockdir" 2>/dev/null; then
    waited=0
    while [ ! -e "$db" ] && [ "$waited" -lt 300 ]; do
        sleep 1
        waited=$((waited + 1))
    done
    [ -e "$db" ] && exit 0
    fail "another build holds $lockdir and $db never appeared; remove the lock if no build is running"
fi
trap 'rmdir "$lockdir" 2>/dev/null || true' EXIT

# Re-check under the lock. The winner of the mkdir can still be the second
# caller to arrive, when the first finished and released between our existence
# test and our mkdir.
[ -e "$db" ] && exit 0

# The password survives the database, so a store removed by hand and reseeded
# keeps the credentials an agent already has.
pwfile="$etcdir/.dev-password"
if [ ! -s "$pwfile" ]; then
    generated=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
    [ "${#generated}" -eq 48 ] || fail "cannot read 24 random bytes from /dev/urandom"
    printf '%s\n' "$generated" > "$pwfile" || fail "cannot write $pwfile"
fi
chmod 600 "$pwfile" || fail "cannot restrict $pwfile"

# The first line only, and no test on read's status: a file written without a
# trailing newline still fills the variable while read reports end of input.
password=""
IFS= read -r password < "$pwfile"
[ -n "$password" ] || fail "$pwfile holds no password"

user=admin
name=$(basename "$session")

printf '%s\n' "$user" "$password" 127.0.0.1 2222 "$name" | "$ze" init --seed ||
    fail "ze init failed for $db"

# ze init reports the path it wrote, but the store this session will USE is the
# one its binary resolves. Assert the file is where the binary's own location
# puts it, or the build stops here rather than running on an empty store.
[ -e "$db" ] || fail "ze init reported success and $db does not exist"

printf 'session store seeded: %s (user %s, password in %s)\n' "$db" "$user" "$pwfile"
