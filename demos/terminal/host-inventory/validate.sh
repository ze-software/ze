#!/usr/bin/env bash
set -euo pipefail
source /src/demos/terminal/validate-common.sh

kernel=$(ze show host kernel 2>&1)
cpu=$(ze show host cpu 2>&1)
memory=$(ze show host memory 2>&1)
nic=$(ze show host nic 2>&1)

# Every assertion names a VALUE, read back from the same kernel through a
# different interface. The keys alone used to be the whole check, so an
# inventory that answered every field empty passed it.
case "$(uname -m)" in
    x86_64) architecture=amd64 ;;
    aarch64) architecture=arm64 ;;
    *) architecture=$(uname -m) ;;
esac
assert_contains "${kernel}" "\"release\": \"$(uname -r)\""
assert_contains "${kernel}" "\"architecture\": \"${architecture}\""

# `nproc` reports the CPUs this process may run on, which a cgroup can narrow.
# The inventory reads /proc/cpuinfo, so the comparison reads it too.
assert_contains "${cpu}" "\"logical-cpus\": $(grep -c '^processor' /proc/cpuinfo)"
assert_contains "${cpu}" \
    "\"model-name\": \"$(sed -n 's/^model name[[:space:]]*: //p' /proc/cpuinfo | head -1)\""

meminfo_bytes() {
    local kilobytes
    kilobytes=$(sed -n "s/^$1:[[:space:]]*\([0-9]*\) kB\$/\1/p" /proc/meminfo)
    printf '%s' $((kilobytes * 1024))
}
assert_contains "${memory}" "\"total-bytes\": $(meminfo_bytes MemTotal)"

# Available memory moves between the two reads, so this asserts the range it
# must sit in rather than an exact number. A field the inventory left at zero,
# or one larger than the installed memory, fails.
inventory_bytes() {
    printf '%s\n' "${memory}" | sed -n "s/^[[:space:]]*\"$1\": \([0-9]*\),\?\$/\1/p"
}
available=$(inventory_bytes available-bytes)
total=$(inventory_bytes total-bytes)
if [[ -z "${available}" || -z "${total}" || "${available}" -le 0 || "${available}" -gt "${total}" ]]; then
    printf 'validation failed: available-bytes %q is not within (0, %q]\n' \
        "${available}" "${total}" >&2
    printf '%s\n' "${memory}" >&2
    exit 1
fi

[[ "${nic}" == \[*\] ]] || {
    printf 'validation failed: NIC inventory is not a JSON array\n%s\n' "${nic}" >&2
    exit 1
}
finish_validation host-inventory
