#!/usr/bin/env bash
# Disposable wrapper around existing native proofs. Invoke through le job run.
# Run from the checkout root; no proof, build, or environment setup runs implicitly.
set -euo pipefail

if [[ $# != 2 ]]; then
    printf 'usage: bash run.sh {l2tp|pppoe-ac|pppoe-client} NEW_ARTIFACT_DIRECTORY\n' >&2
    exit 2
fi
mode=$1
out=$2
root=$PWD
le="$root/bin/le-testing-baseline-final/le"
ze=${ZE_EVIDENCE_ZE_BINARY:-"$root/ze-host"}
if [[ ! -f "$root/feature-gates.txt" || ! -x "$le" ]]; then
    printf 'Run from the checkout root with bin/le-testing-baseline-final/le present.\n' >&2
    exit 2
fi
for utility in jq timeout sha256sum cp mkdir uname date; do
    if ! command -v "$utility" >/dev/null; then
        printf 'Missing prerequisite: %s\n' "$utility" >&2
        exit 2
    fi
done
scenario=''
case "$mode" in
    l2tp)
        if [[ ! -x "$ze" ]]; then
            printf 'Missing existing host daemon: %s (do not rebuild implicitly).\n' "$ze" >&2
            exit 2
        fi
        export ZE_EVIDENCE_ZE_BINARY="$ze"
        action=l2tp-ppp-test
        bound=300
        ;;
    pppoe-ac)
        scenario=02-ze-ac-pppd-client
        action=docker-pppoe-accel-test
        bound=600
        ;;
    pppoe-client)
        scenario=01-pppoe-chap-ipv4
        action=docker-pppoe-accel-test
        bound=600
        ;;
    *) printf 'Unknown mode: %s\n' "$mode" >&2; exit 2 ;;
esac
if [[ -e "$out" ]]; then
    printf 'Refusing to overwrite artifact directory: %s\n' "$out" >&2
    exit 2
fi
umask 077
mkdir -p "$out"
cp -- "${BASH_SOURCE[0]}" "$out/driver.sh"
cp -- "${BASH_SOURCE[0]%/*}/manifest.json" "$out/manifest.json"
uname -a >"$out/kernel.txt"
date -u +%FT%TZ >"$out/started-at.txt"
sha256sum "$le" >"$out/native-binary.sha256"
if [[ "$mode" == l2tp ]]; then
    sha256sum "$ze" >"$out/daemon-binary.sha256"
    sha256sum internal/le/deployment/l2tpppp*.go internal/le/deployment/pppstate.go \
        internal/le/deployment/hostkernel.go >"$out/proof-source.sha256"
else
    if ! command -v docker >/dev/null; then
        printf 'Missing prerequisite: docker\n' >&2
        exit 2
    fi
    # Reuse requires independently recorded image freshness. Never silently build.
    export NO_BUILD=1
    export ZE_PPPOE_INTEROP_SCENARIO="$scenario"
    export ZE_PPPOE_INTEROP_SUFFIX="baseline-$$"
    timeout --signal=TERM --kill-after=5s 30s docker image inspect ze-pppoe-interop ze-pppoe-accel ze-pppoe-client \
        >"$out/images-before.json" 2>"$out/images-before.stderr"
    sha256sum internal/le/interoplab/pppoe/*.go internal/le/interoplab/lab.go \
        "test/interop-pppoe/scenarios/$scenario/ze.conf" >"$out/proof-source.sha256"
    cp -- "test/interop-pppoe/scenarios/$scenario/ze.conf" "$out/ze.conf"
fi
command=("$le" le deployment "$action" '|' json)
jq -n --arg mode "$mode" --arg scenario "$scenario" --arg cwd "$root" \
    --arg binary "$le" --arg action "$action" --argjson bound "$bound" \
    --arg ze "$ze" --arg suffix "${ZE_PPPOE_INTEROP_SUFFIX:-}" \
    '{mode:$mode,scenario:$scenario,cwd:$cwd,"timeout-seconds":$bound,
      environment:(if $mode == "l2tp" then {ZE_EVIDENCE_ZE_BINARY:$ze}
        else {NO_BUILD:"1",ZE_PPPOE_INTEROP_SCENARIO:$scenario,ZE_PPPOE_INTEROP_SUFFIX:$suffix} end),
      argv:[$binary,"le","deployment",$action,"|","json"]}' >"$out/invocation.json"
code=0
timeout --signal=TERM --kill-after=20s "${bound}s" "${command[@]}" \
    >"$out/native-report.json" 2>"$out/native.stderr" || code=$?
assertion=1
if [[ "$code" == 0 ]]; then
    if [[ "$mode" == l2tp ]]; then
        jq -e '(.data // .) | .proven == true and .peer == "xl2tpd" and
            (."ze-interface" | startswith("ppp")) and
            (."lac-interface" | startswith("ppp")) and
            ."local-address" == "10.100.0.1" and ."peer-address" == "10.100.0.2"' \
            "$out/native-report.json" >"$out/assertion.txt" 2>"$out/assertion.stderr" && assertion=0
    else
        jq -e --arg scenario "$scenario" '(.data // .) |
            .code == 0 and .passed == 1 and .failed == 0 and
            (.scenarios | length) == 1 and .scenarios[0].name == $scenario and
            .scenarios[0].passed == true and
            ((.scenarios[0]."cleanup-errors" // []) | length) == 0' \
            "$out/native-report.json" >"$out/assertion.txt" 2>"$out/assertion.stderr" && assertion=0
    fi
fi
date -u +%FT%TZ >"$out/finished-at.txt"
jq -n --arg mode "$mode" --argjson code "$code" --argjson assertion "$assertion" \
    '{mode:$mode,"native-exit-code":$code,"report-assertion-exit-code":$assertion,
      status:(if $code == 0 and $assertion == 0 then "implemented-tested"
              else "implemented-unverified" end),
      "cleanup-observation":(if $code == 124 or $code == 137 then
          "outer timeout: inspect owned namespaces/containers; cleanup is unverified"
          else "see native report and stderr" end)}' >"$out/result.json"
printf 'Subscriber evidence: %s/result.json\n' "$out"
if [[ "$code" != 0 ]]; then exit "$code"; fi
exit "$assertion"
