#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh
run=/src/demos/terminal/traffic-anomaly/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT

# Bytes the eBPF accounting attributes to the lab source, both directions.
source_bytes() {
    printf '%s' "$1" | python3 -c '
import json, sys

usage = json.load(sys.stdin)
rows = usage.get("ingress-ips", []) + usage.get("egress-ips", [])
print(sum(row["bytes"] for row in rows if row["ip"] == "10.77.0.2"))
'
}

"${run}" prepare >/dev/null
"${run}" start >/dev/null
before=$("${run}" show 2>&1)
assert_contains "${before}" '"interface":"traffic0"'
# `traffic0` is the interface ze.conf names, so both snapshots carry it whether
# or not a packet ever arrives. The source address is what the accounting has
# to discover, and it is absent until the burst.
assert_not_contains "${before}" '10.77.0.2'

"${run}" generate >/dev/null
after=$("${run}" show 2>&1)
assert_contains "${after}" '"interface":"traffic0"'
assert_contains "${after}" '"ip":"10.77.0.2"'
assert_contains "${after}" '"port":8080'
assert_contains "${after}" '"protocol":"icmp"'

before_bytes=$(source_bytes "${before}")
after_bytes=$(source_bytes "${after}")
if [[ "${after_bytes}" -le "${before_bytes}" ]]; then
    printf 'validation failed: bytes attributed to 10.77.0.2 did not rise: %s -> %s\n' \
        "${before_bytes}" "${after_bytes}" >&2
    printf '%s\n%s\n' "${before}" "${after}" >&2
    exit 1
fi
finish_validation traffic-anomaly
