#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

run=/src/demos/terminal/web-config/run.sh
trap '"${run}" stop >/dev/null 2>&1 || true' EXIT
"${run}" start >/dev/null
base=https://127.0.0.1:8443
auth=admin:secret123
origin='Origin: https://127.0.0.1:8443'

editor=$(curl --fail --insecure --silent --show-error --user "${auth}" \
    "${base}/show/system/identity/")
assert_contains "${editor}" "System Identity"

curl --fail --insecure --silent --show-error --user "${auth}" \
    --header "${origin}" --data-urlencode 'field:system/host=edge-demo' \
    "${base}/config/form/" >/dev/null
diff=$(curl --fail --insecure --silent --show-error --user "${auth}" \
    "${base}/config/diff")
assert_contains "${diff}" "host edge-demo"

curl --fail --insecure --silent --show-error --user "${auth}" \
    --header "${origin}" --request POST "${base}/config/commit" >/dev/null
active=$(curl --fail --insecure --silent --show-error --user "${auth}" \
    "${base}/show/system/identity/")
assert_contains "${active}" 'value="edge-demo"'
finish_validation web-config
