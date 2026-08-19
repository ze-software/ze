#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/rbac/config
run=/src/demos/terminal/rbac/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" start >/dev/null

export ZE_SSH_PASSWORD=noc-secret
allowed=$(ze cli --user noc -c 'show version' 2>&1)
# The version STRING, not the field name: `version` is a label in every
# rendering, so the old assertion held over an answer that carried no version
# at all. The daemon runs this same binary, so `ze version` is the value it
# must report.
assert_contains "${allowed}" "$(ze version)"
set +e
denied=$(ze cli --user noc -c 'clear interface counters' 2>&1)
status=$?
set -e
if [[ ${status} -eq 0 ]]; then
    printf 'validation failed: denied RBAC command succeeded\n' >&2
    exit 1
fi
assert_contains "${denied}" "restricted by access control"
finish_validation rbac
