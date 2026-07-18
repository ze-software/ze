#!/usr/bin/env bash
set -euo pipefail

root=/src/demos/terminal/config-views
state=/src/tmp/terminal-demos/state/config-views

prepare() {
    rm -rf "${state}"
    mkdir -p "${state}"
    ze config validate "${root}/router.conf" >/dev/null
    ze config migrate --format set -o "${state}/router.set" "${root}/router.conf" >/dev/null
    ze config migrate --format hierarchical -o "${state}/roundtrip.conf" "${state}/router.set" >/dev/null
    ze config migrate --format set -o "${state}/roundtrip.set" "${state}/roundtrip.conf" >/dev/null
    echo "Configuration views prepared"
}

case "${1:-}" in
    prepare) prepare ;;
    *) echo "usage: $0 prepare" >&2; exit 2 ;;
esac
