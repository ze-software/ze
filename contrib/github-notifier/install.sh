#!/bin/sh
# Install reviewed native binary and systemd units; preserve all private state.
# Runbook: docs/contributing/github-notifier.md
set -eu

notifier_binary=
notifier_revision=
notifier_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$notifier_directory/leaf-guards.sh"
notifier_account=ze-github-notifier
notifier_prefix=/opt/ze-github-notifier
notifier_record=
notifier_link=
trap 'if [ -n "$notifier_record" ]; then rm -f "$notifier_record"; fi; if [ -n "$notifier_link" ]; then rm -f "$notifier_link"; fi' EXIT HUP INT TERM

fail() { printf '%s\n' "$1" >&2; exit 1; }
usage() { printf '%s\n' 'Usage: sudo sh contrib/github-notifier/install.sh --binary /absolute/path/le --revision <full-commit-id>'; }
while [ "$#" -gt 0 ]; do
    case "$1" in
        --binary|--revision)
            [ "$#" -ge 2 ] || fail 'A binary or revision value is missing. Run install.sh --help.'
            case "$2" in -*) fail 'A value cannot begin with a dash. Run install.sh --help.';; esac
            case "$1" in --binary) notifier_binary=$2;; --revision) notifier_revision=$2;; esac
            shift 2
            ;;
        --help) usage; exit 0;;
        *) fail 'Unknown install option. Run install.sh --help.';;
    esac
done
[ "$(id -u)" -eq 0 ] || fail 'Installation needs root to create the account and units. Run the documented sudo command.'
[ "${#notifier_revision}" -eq 40 ] || fail 'Supply the reviewed full 40-character commit ID with --revision.'
case "$notifier_revision" in *[!0-9a-f]*) fail 'The revision must be a lowercase hexadecimal commit ID.';; esac
case "$notifier_binary" in /*/le) ;; *) fail 'Supply the absolute path of the native binary, whose filename must be le.';; esac
[ -f "$notifier_binary" ] || fail 'The native binary is missing. Build it in the primary main checkout as the runbook describes.'
[ ! -L "$notifier_binary" ] || fail 'The install input must be a regular binary, without a symbolic link.'
[ -x "$notifier_binary" ] || fail 'The supplied native binary is not executable.'
for notifier_tool in go systemctl getent useradd install sha256sum cmp ln mv sed grep chmod rm stat mktemp awk readlink; do
    command -v "$notifier_tool" >/dev/null 2>&1 || fail "Missing installation tool: $notifier_tool. Install it before running this installer."
done
# These build records come from Go's embedded VCS data, not a caller's label.
notifier_source_digest=$(sha256sum "$notifier_binary" | sed 's/ .*//')
notifier_metadata=$(go version -m "$notifier_binary")
notifier_built_revision=$(printf '%s\n' "$notifier_metadata" | sed -n 's/^[[:space:]]*build[[:space:]]*vcs.revision=//p')
notifier_modified=$(printf '%s\n' "$notifier_metadata" | sed -n 's/^[[:space:]]*build[[:space:]]*vcs.modified=//p')
[ "$notifier_built_revision" = "$notifier_revision" ] || fail 'Binary VCS identity differs from --revision. Rebuild the exact reviewed commit natively.'
[ "$notifier_modified" = false ] || fail 'Binary inputs were dirty or lack VCS evidence. Build from the clean reviewed main checkout.'
printf '%s\n' "$notifier_metadata" | grep -q 'GOOS=linux' || fail 'The binary is not a Linux host build. Build it natively on the installation host.'
notifier_arch=$(go env GOHOSTARCH)
printf '%s\n' "$notifier_metadata" | grep -q "GOARCH=$notifier_arch"'$' || fail 'The binary architecture differs from this host. Build it natively on the installation host.'
printf '%s\n' "$notifier_metadata" | grep -q 'CGO_ENABLED=0' || fail 'The binary does not record CGO_ENABLED=0. Use the native launcher build.'
printf '%s\n' "$notifier_metadata" | grep -q -- '-tags=ze_le,' || fail 'The binary does not carry the le personality. Use the documented named launcher build.'
for notifier_path in "$notifier_prefix" "$notifier_prefix/releases" /etc/ze-github-notifier /var/lib/ze-github-notifier; do
    [ ! -L "$notifier_path" ] || fail 'An installation directory is a symbolic link. Resolve its ownership before installation.'
    if [ -e "$notifier_path" ]; then
        [ -d "$notifier_path" ] || fail 'An installation directory path is a non-directory. Inspect it before installation.'
    fi
done
for notifier_path in "$notifier_prefix" "$notifier_prefix/releases"; do
    if [ -e "$notifier_path" ]; then
        [ "$(stat -c %u "$notifier_path")" -eq 0 ] || fail 'An existing executable directory is not root-owned. Inspect it before installation.'
        [ "$(stat -c %a "$notifier_path")" = 755 ] || fail 'An existing executable directory has unexpected permissions. Inspect it before installation.'
    fi
done
if [ -L "$notifier_prefix/current" ]; then
    [ "$(stat -c %u "$notifier_prefix/current")" -eq 0 ] || fail 'The current-version link is not root-owned. Inspect it before installation.'
    notifier_previous=$(readlink "$notifier_prefix/current")
    case "$notifier_previous" in releases/*) ;; *) fail 'The current-version link leaves the release namespace. Inspect it before installation.';; esac
    notifier_previous_revision=${notifier_previous#releases/}
    [ "${#notifier_previous_revision}" -eq 40 ] || fail 'The current-version link has an unknown release identity. Inspect it before installation.'
    case "$notifier_previous_revision" in *[!0-9a-f]*) fail 'The current-version link has an invalid release identity. Inspect it before installation.';; esac
else
    [ ! -e "$notifier_prefix/current" ] || fail 'The current-version path is not the expected owned link. Inspect it before installation.'
fi
for notifier_unit in ze-github-notifier.service ze-github-notifier.timer; do
    notifier_target=/etc/systemd/system/$notifier_unit
    [ ! -L "$notifier_target" ] || fail 'An installed unit is a symbolic link. Inspect it before replacing notifier units.'
    if [ -e "$notifier_target" ]; then
        [ -f "$notifier_target" ] || fail 'An installed unit is not a regular file. Inspect it before installation.'
        [ "$(stat -c %u "$notifier_target")" -eq 0 ] || fail 'An installed unit is not root-owned. Inspect it before installation.'
    fi
    [ ! -L "$notifier_directory/$notifier_unit" ] || fail 'A source unit is a symbolic link. Use the reviewed regular template.'
    [ -f "$notifier_directory/$notifier_unit" ] || fail 'A source unit is missing. Use the reviewed installation directory.'
done
if ! getent passwd "$notifier_account" >/dev/null; then
    useradd --system --user-group --home-dir /var/lib/ze-github-notifier --shell /usr/sbin/nologin "$notifier_account"
fi
[ "$(id -u "$notifier_account")" -gt 0 ] || fail 'The notifier account must have a dedicated non-root UID. Correct the account before installation.'
getent passwd "$notifier_account" | awk -F: '$6 == "/var/lib/ze-github-notifier" && $7 ~ /\/(nologin|false)$/ { valid = 1 } END { exit !valid }' || fail 'The existing notifier account has a different home or login shell. Inspect the account rather than adopting it.'
notifier_uid=$(id -u "$notifier_account")
for notifier_path in /etc/ze-github-notifier /var/lib/ze-github-notifier; do
    if [ -e "$notifier_path" ]; then
        [ "$(stat -c %u "$notifier_path")" = "$notifier_uid" ] || fail 'An existing private directory has another owner. Inspect it rather than changing ownership of its files.'
        [ "$(stat -c %a "$notifier_path")" = 700 ] || fail 'An existing private directory has unsafe permissions. Inspect it before installation.'
    fi
done
systemctl stop ze-github-notifier.timer 2>/dev/null || {
    if systemctl cat ze-github-notifier.timer >/dev/null 2>&1; then fail 'The existing notifier timer could not stop. Inspect its systemd status before installation.'; fi
}
systemctl stop ze-github-notifier.service 2>/dev/null || {
    if systemctl cat ze-github-notifier.service >/dev/null 2>&1; then fail 'The existing notifier service could not stop. Inspect its systemd status before installation.'; fi
}
install -d -o root -g root -m 0755 "$notifier_prefix" "$notifier_prefix/releases"
notifier_release=$notifier_prefix/releases/$notifier_revision
[ ! -L "$notifier_release" ] || fail 'The selected release directory is a symbolic link. Inspect its owner before installation.'
if [ -e "$notifier_release" ]; then
    [ -d "$notifier_release" ] || fail 'The selected release path is not a directory. Inspect it before installation.'
    [ "$(stat -c %u "$notifier_release")" -eq 0 ] || fail 'The selected release directory is not root-owned. Inspect it before installation.'
    [ "$(stat -c %a "$notifier_release")" = 755 ] || fail 'The selected release directory has unexpected permissions. Inspect it before installation.'
fi
install -d -o root -g root -m 0755 "$notifier_release"
notifier_installed_executable_safe "$notifier_release/le" || exit 1
if [ -e "$notifier_release/le" ]; then
    cmp -s "$notifier_binary" "$notifier_release/le" || fail 'This revision already has different installed bytes. Preserve it and investigate the build identity.'
else
    install -o root -g root -m 0755 "$notifier_binary" "$notifier_release/le"
fi
notifier_installed_digest=$(sha256sum "$notifier_release/le" | sed 's/ .*//')
[ "$notifier_installed_digest" = "$notifier_source_digest" ] || fail 'Binary bytes changed during installation. Keep the timer stopped and investigate the input.'
notifier_source_build=$(printf '%s\n' "$notifier_metadata" | sed '1d')
notifier_installed_build=$(go version -m "$notifier_release/le" | sed '1d')
[ "$notifier_installed_build" = "$notifier_source_build" ] || fail 'Installed binary metadata differs from the checked input. Keep the timer stopped and inspect the build.'
install -d -o "$notifier_account" -g "$notifier_account" -m 0700 /etc/ze-github-notifier /var/lib/ze-github-notifier
notifier_digest=$(sha256sum "$notifier_release/le")
notifier_build_record_safe "$notifier_release/build.txt" || exit 1
notifier_record=$(mktemp "$notifier_release/.build.XXXXXX")
printf 'source-revision=%s\nbinary=%s\n%s\n' "$notifier_revision" "$notifier_release/le" "$notifier_digest" > "$notifier_record"
chmod 0644 "$notifier_record"
mv -Tf "$notifier_record" "$notifier_release/build.txt"
notifier_record=
notifier_candidate=$notifier_prefix/.current.$$
ln -s "releases/$notifier_revision" "$notifier_candidate"
notifier_link=$notifier_candidate
mv -Tf "$notifier_link" "$notifier_prefix/current"
install -o root -g root -m 0644 "$notifier_directory/ze-github-notifier.service" /etc/systemd/system/ze-github-notifier.service
install -o root -g root -m 0644 "$notifier_directory/ze-github-notifier.timer" /etc/systemd/system/ze-github-notifier.timer
systemctl daemon-reload
printf '%s\n' 'Installed the reviewed binary and units. The timer is stopped.' 'Provision and initialize private workflow files, run one poll, then enable the timer as docs/contributing/github-notifier.md describes.'
