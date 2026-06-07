#!/bin/bash
# Enforce: ai/rules/registration-dispatch.md
# Block switch/case used for subcommand dispatch (args[0] pattern)

set -euo pipefail

file="${CLAUDE_FILE:-}"
[[ -z "$file" ]] && exit 0
[[ "$file" == *_test.go ]] && exit 0
[[ "$file" != *.go ]] && exit 0

# Look for the pattern: switch args[0] -- hand-rolled command dispatch
# (switch args[i] for flag parsing is fine)
if grep -nE 'switch\s+args\[0\]' "$file" | grep -vq '//.*nolint'; then
    matches=$(grep -nE 'switch\s+args\[0\]' "$file" | grep -v '//.*nolint')
    echo -e "\033[31m\033[1m❌ BLOCKED: switch-based command dispatch\033[0m"
    echo ""
    echo "$matches" | while read -r line; do
        echo -e "  \033[31m✗\033[0m $line"
    done
    echo ""
    echo -e "  \033[33mUse subdispatch.New() + Register() instead of switch on args.\033[0m"
    echo -e "  \033[33mRule: ai/rules/registration-dispatch.md\033[0m"
    exit 2
fi

exit 0
