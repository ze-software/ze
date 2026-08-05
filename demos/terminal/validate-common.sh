#!/usr/bin/env bash
set -euo pipefail

assert_contains() {
    local value=$1
    local expected=$2
    if [[ "${value}" != *"${expected}"* ]]; then
        printf 'validation failed: expected output containing %q\n' "${expected}" >&2
        printf '%s\n' "${value}" >&2
        return 1
    fi
}

assert_not_contains() {
    local value=$1
    local unexpected=$2
    if [[ "${value}" == *"${unexpected}"* ]]; then
        printf 'validation failed: output unexpectedly contained %q\n' "${unexpected}" >&2
        printf '%s\n' "${value}" >&2
        return 1
    fi
}

finish_validation() {
    printf 'validated %s output\n' "$1"
}

# A failing validator must say why. run.sh sends `ze init`, `ze config import`
# and the daemon output to log files under the demo state directory, so a
# failure in any of them exits non-zero with an empty terminal. The trap prints
# what the run captured.
#
# It hangs on ERR, not on EXIT: a validator that starts a demo installs its own
# EXIT trap to stop it, and that would replace this one. `errtrace` carries the
# trap into functions and subshells, which an ERR trap does not reach by
# default.
set -o errtrace
demo_report_logs() {
    local demo_id state log
    demo_id=$(basename "$(dirname "$0")")
    state="${ZE_DEMO_STATE_ROOT:-/src/tmp/terminal-demos/state}/${demo_id}"
    for log in "${state}"/*.log; do
        [[ -s "${log}" ]] || continue
        printf -- '--- %s\n' "${log}" >&2
        tail -n 20 "${log}" >&2
    done
}

trap demo_report_logs ERR
