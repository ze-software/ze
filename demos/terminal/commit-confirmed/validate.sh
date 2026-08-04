#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

state=/src/tmp/terminal-demos/state/commit-confirmed
config=${state}/ze.conf
rm -rf "${state}"
mkdir -p "${state}"
cp /src/demos/terminal/commit-confirmed/identity.conf "${config}"

output=$(
    python3 /src/demos/terminal/pty-session.py \
        --delay 2 \
        --command 'show system host' \
        --command 'set system host edge-trial' \
        --command 'show | compare' \
        --command 'commit confirmed 5' \
        --command '@wait Confirm within' \
        --command 'show system host' \
        --command '@wait automatically rolled back' \
        --command 'show system host' \
        --command 'set system host edge-confirmed' \
        --command 'commit confirmed 5' \
        --command '@wait Confirm within' \
        --command confirm \
        --command '@wait confirmed and saved permanently' \
        --command '@sleep 7' \
        --command 'show system host' \
        --command exit \
        --command '@wait operational\]' \
        --command '@escape' \
        --command '@wait Quit\?' \
        --command '@escape' \
        -- ze config edit -f "${config}"
)
assert_contains "${output}" "edge-original"
assert_contains "${output}" "edge-trial"
assert_contains "${output}" "Confirm within"
assert_contains "${output}" "automatically rolled back"
after_timeout=${output#*automatically rolled back}
assert_contains "${after_timeout}" "edge-original"
assert_not_contains "$(cat "${config}")" "edge-trial"
assert_contains "${output}" "confirmed and saved permanently"
after_confirm=${output#*confirmed and saved permanently}
assert_contains "${after_confirm}" "edge-confirmed"
assert_contains "$(cat "${config}")" "edge-confirmed"
finish_validation commit-confirmed
