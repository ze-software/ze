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
