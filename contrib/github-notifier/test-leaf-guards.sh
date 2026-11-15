#!/bin/sh
# Execute the actual installation guards against isolated Linux file fixtures.
# This proof never reads or changes /opt, /etc, accounts, units or private state.
set -eu
notifier_test_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$notifier_test_directory/leaf-guards.sh"
if [ "$#" -ne 2 ]; then
    printf '%s\n' 'Usage: sudo sh contrib/github-notifier/test-leaf-guards.sh --scratch /absolute/task-scratch' >&2
    exit 2
fi
if [ "$1" != --scratch ]; then printf '%s\n' 'Specify --scratch with the existing task-owned scratch directory.' >&2; exit 2; fi
case "$2" in /*) ;; *) printf '%s\n' 'Scratch must be absolute.' >&2; exit 2;; esac
if [ "$(id -u)" -ne 0 ]; then printf '%s\n' 'This Linux ownership proof needs sudo; it changes only its own scratch fixtures.' >&2; exit 2; fi
[ -d "$2" ] || { printf '%s\n' 'The task scratch directory must already exist.' >&2; exit 2; }
notifier_test_root=$(mktemp -d "$2/notifier-leaf-proof.XXXXXX")
trap 'rm -rf "$notifier_test_root"' EXIT HUP INT TERM
printf 'matching fixture bytes\n' > "$notifier_test_root/control"
chmod 0755 "$notifier_test_root/control"
notifier_installed_executable_safe "$notifier_test_root/control"
notifier_installed_executable_safe "$notifier_test_root/missing"

notifier_expect_refusal() {
    notifier_test_guard=$1
    notifier_test_path=$2
    notifier_test_message=$3
    if "$notifier_test_guard" "$notifier_test_path" > "$notifier_test_root/output" 2>&1; then
        printf '%s\n' "FAIL: accepted $notifier_test_message" >&2
        exit 1
    fi
    grep -q "$notifier_test_message" "$notifier_test_root/output" || {
        printf '%s\n' "FAIL: refusal did not identify $notifier_test_message" >&2
        exit 1
    }
}

cp "$notifier_test_root/control" "$notifier_test_root/user-owned"
chown 65534 "$notifier_test_root/user-owned"
notifier_expect_refusal notifier_installed_executable_safe "$notifier_test_root/user-owned" 'not root-owned'
cp "$notifier_test_root/control" "$notifier_test_root/writable"
chmod 0777 "$notifier_test_root/writable"
notifier_expect_refusal notifier_installed_executable_safe "$notifier_test_root/writable" 'unexpected permissions'
ln -s "$notifier_test_root/control" "$notifier_test_root/alias"
notifier_expect_refusal notifier_installed_executable_safe "$notifier_test_root/alias" 'symbolic link'
ln -s "$notifier_test_root/missing" "$notifier_test_root/dangling"
notifier_expect_refusal notifier_installed_executable_safe "$notifier_test_root/dangling" 'symbolic link'
mkdir "$notifier_test_root/directory"
notifier_expect_refusal notifier_installed_executable_safe "$notifier_test_root/directory" 'regular file'
printf 'PASS: executable control and five unsafe leaves\n'

printf 'retained record target\n' > "$notifier_test_root/record-target"
chmod 0644 "$notifier_test_root/record-target"
cp "$notifier_test_root/record-target" "$notifier_test_root/original-target"
notifier_build_record_safe "$notifier_test_root/record-target"
notifier_build_record_safe "$notifier_test_root/absent-record"
ln -s "$notifier_test_root/record-target" "$notifier_test_root/build.txt"
notifier_expect_refusal notifier_build_record_safe "$notifier_test_root/build.txt" 'symbolic link'
ln -s "$notifier_test_root/absent-record" "$notifier_test_root/dangling-record"
notifier_expect_refusal notifier_build_record_safe "$notifier_test_root/dangling-record" 'symbolic link'
notifier_expect_refusal notifier_build_record_safe "$notifier_test_root/directory" 'regular file'
cp "$notifier_test_root/record-target" "$notifier_test_root/user-record"
chown 65534 "$notifier_test_root/user-record"
notifier_expect_refusal notifier_build_record_safe "$notifier_test_root/user-record" 'not root-owned'
cp "$notifier_test_root/record-target" "$notifier_test_root/writable-record"
chmod 0666 "$notifier_test_root/writable-record"
notifier_expect_refusal notifier_build_record_safe "$notifier_test_root/writable-record" 'unexpected permissions'
cmp "$notifier_test_root/record-target" "$notifier_test_root/original-target"
printf 'PASS: build-record control, five unsafe leaves, unchanged alias target\n'
