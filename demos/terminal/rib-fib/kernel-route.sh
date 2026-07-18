#!/usr/bin/env bash
set -euo pipefail

prefix=${1:?usage: kernel-route.sh PREFIX}
route=$(ip -details route show exact "${prefix}")
if [[ -n "${route}" ]]; then
    printf 'kernel FIB: %s\n' "${route}"
else
    printf 'kernel FIB: %s absent\n' "${prefix}"
fi
