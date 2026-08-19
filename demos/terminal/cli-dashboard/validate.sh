#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/cli-dashboard/config
export ZE_SSH_PASSWORD=secret123
export SSHPASS=secret123
run=/src/demos/terminal/cli-dashboard/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" start >/dev/null

# The peer NAMES come from ze.conf, and the list prints every configured peer
# whether or not a session ever formed, so this waits on the three states
# instead. Sessions are what the dashboard displays.
peers=
for _ in {1..100}; do
    peers=$(ze cli -c 'show bgp peer list | raw' 2>&1 || true)
    if [[ $(grep -c '"state": "established"' <<<"${peers}") -eq 3 ]]; then
        break
    fi
    sleep 0.1
done
if [[ $(grep -c '"state": "established"' <<<"${peers}") -ne 3 ]]; then
    printf 'validation failed: expected three established sessions\n' >&2
    printf '%s\n' "${peers}" >&2
    exit 1
fi

# The demo's subject is `monitor bgp`: sort with `s`, select with an arrow key,
# Enter for the peer detail. Each `@wait` holds until its pattern appears in
# the output that followed the keystroke before it, and the session exits
# non-zero when one does not, so the sequence is asserted in order.
#
# `Peer Detail: 127.0.0.2` is where the two halves meet. `s` moves the sort
# column from Address to ASN (internal/component/cli/model_dashboard.go:389),
# and by ASN the order is 64496, 65001, 65002 -- 127.0.0.4, 127.0.0.2,
# 127.0.0.3. So one Down from the top selects 127.0.0.2 only if the sort took
# effect; in the address order it would select 127.0.0.3.
output=$(
    python3 /src/demos/terminal/pty-session.py --timeout 20 \
        --command exit \
        --command '@wait operational\]' \
        --command 'monitor bgp' \
        --command '@wait 127\.0\.0\.4' \
        --command '@type s' \
        --command '@wait ASN \^' \
        --command '@key down' \
        --command '@key enter' \
        --command '@wait Peer Detail: 127\.0\.0\.2' \
        --command '@escape' \
        --command '@type q' \
        --command '@wait operational\]' \
        --command exit \
        -- sshpass -e ssh ze-demo
)
assert_contains "${output}" 'ASN ^'
assert_contains "${output}" 'Peer Detail: 127.0.0.2'
finish_validation cli-dashboard
