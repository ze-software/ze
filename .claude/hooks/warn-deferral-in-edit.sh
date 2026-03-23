#!/bin/bash
# PostToolUse hook: Warn when deferral language appears in spec/doc edits
# ADVISORY (exit 1): Immediate reminder to log the deferral, not blocking.

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Only check spec and doc files (markdown)
if [[ ! "$FILE_PATH" =~ \.md$ ]]; then
    exit 0
fi

# Skip the deferrals file itself
if [[ "$FILE_PATH" =~ plan/deferrals\.md$ ]]; then
    exit 0
fi

# Skip memory and session state files
if [[ "$FILE_PATH" =~ \.claude/(memory|session-state|plan)/ ]]; then
    exit 0
fi

# Skip learned summaries (they document completed deferrals)
if [[ "$FILE_PATH" =~ plan/learned/ ]]; then
    exit 0
fi

# Get the content that was written/edited
if [[ "$TOOL_NAME" == "Write" ]]; then
    CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')
else
    CONTENT=$(echo "$INPUT" | jq -r '.tool_input.new_string // empty')
fi

if [[ -z "$CONTENT" ]]; then
    exit 0
fi

# Check for deferral phrases
DEFERRAL_PATTERNS=(
    "deferred to"
    "deferred for"
    "defer to"
    "out of scope"
    "future work"
    "future spec"
    "handle later"
    "address later"
    "skip for now"
    "skipping for now"
    "postpone"
    "not yet implemented"
    "not yet wired"
)

for pattern in "${DEFERRAL_PATTERNS[@]}"; do
    if echo "$CONTENT" | grep -qi "$pattern"; then
        YELLOW='\033[33m'
        BOLD='\033[1m'
        RESET='\033[0m'

        echo -e "${YELLOW}${BOLD}  Deferral language detected in $(basename "$FILE_PATH")${RESET}" >&2
        echo -e "  ${YELLOW}Pattern: '$pattern'${RESET}" >&2
        echo -e "  ${YELLOW}Record in plan/deferrals.md if this is deferred work.${RESET}" >&2
        exit 1
    fi
done

exit 0
