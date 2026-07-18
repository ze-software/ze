#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

kernel=$(ze show host kernel 2>&1)
cpu=$(ze show host cpu 2>&1)
memory=$(ze show host memory 2>&1)
nic=$(ze show host nic 2>&1)
assert_contains "${kernel}" '"release"'
assert_contains "${kernel}" '"architecture"'
assert_contains "${cpu}" '"logical-cpus"'
assert_contains "${cpu}" '"vendor"'
assert_contains "${memory}" '"total-bytes"'
assert_contains "${memory}" '"available-bytes"'
[[ "${nic}" == \[*\] ]] || {
    printf 'validation failed: NIC inventory is not a JSON array\n%s\n' "${nic}" >&2
    exit 1
}
finish_validation host-inventory
