#!/bin/bash
# PostToolUse hook: Validate plan/spec-*.md files against planning.md rules
# BLOCKING: Rejects invalid specs

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit tools
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Only process plan/spec-*.md files
if [[ ! "$FILE_PATH" =~ plan/spec-.*\.md$ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

RED='\033[31m'
YELLOW='\033[33m'
GREEN='\033[32m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()
WARNINGS=()

# Read file content
CONTENT=$(cat "$FILE_PATH")

# === REQUIRED SECTIONS ===
REQUIRED_SECTIONS=(
    "## Task"
    "## Required Reading"
    "## 🧪 TDD Test Plan"
    "### Unit Tests"
    "## Files to Modify"
    "## Implementation Steps"
    "## Checklist"
)

for section in "${REQUIRED_SECTIONS[@]}"; do
    if ! grep -q "^${section}" "$FILE_PATH"; then
        ERRORS+=("Missing required section: $section")
    fi
done

# === RFC SUMMARY CHECK ===
# Extract referenced RFC summaries and check they exist
RFC_REFS=$(grep -oE '\.claude/zebgp/rfc/rfc[0-9]+\.md' "$FILE_PATH" 2>/dev/null || true)
for ref in $RFC_REFS; do
    if [[ ! -f "$ref" ]]; then
        ERRORS+=("RFC summary not found: $ref (run /rfc-summarisation first)")
    fi
done

# === TABLE FORMAT CHECK ===
# Check Unit Tests section uses table (has | characters)
UNIT_TEST_SECTION=$(sed -n '/^### Unit Tests/,/^###/p' "$FILE_PATH" | head -20)
if [[ -n "$UNIT_TEST_SECTION" ]] && ! echo "$UNIT_TEST_SECTION" | grep -q '|.*|.*|'; then
    ERRORS+=("Unit Tests section must use table format (| Test | File | Validates |)")
fi

# Check Functional Tests section uses table if present
FUNC_TEST_SECTION=$(sed -n '/^### Functional Tests/,/^##/p' "$FILE_PATH" | head -20)
if [[ -n "$FUNC_TEST_SECTION" ]] && ! echo "$FUNC_TEST_SECTION" | grep -q '|.*|.*|'; then
    WARNINGS+=("Functional Tests section should use table format")
fi

# === CHECKLIST ITEMS ===
REQUIRED_CHECKLIST=(
    "Tests written"
    "Tests FAIL"
    "Tests PASS"
    "make lint"
    "make test"
    "make functional"
)

for item in "${REQUIRED_CHECKLIST[@]}"; do
    if ! grep -q "$item" "$FILE_PATH"; then
        ERRORS+=("Missing checklist item: $item")
    fi
done

# === RFC CONSTRAINT DOCS CHECK ===
# If protocol work (references RFCs), should have RFC Documentation section
if [[ -n "$RFC_REFS" ]]; then
    if ! grep -q "## RFC Documentation" "$FILE_PATH"; then
        WARNINGS+=("Protocol work should have '## RFC Documentation' section")
    fi
fi

# === OUTPUT RESULTS ===
if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ Spec validation FAILED:${RESET} $(basename "$FILE_PATH")" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    if [[ ${#WARNINGS[@]} -gt 0 ]]; then
        echo "" >&2
        for warn in "${WARNINGS[@]}"; do
            echo -e "  ${YELLOW}⚠${RESET} $warn" >&2
        done
    fi
    echo "" >&2
    echo -e "  ${YELLOW}See: .claude/rules/planning.md${RESET}" >&2
    exit 1
fi

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Spec warnings:${RESET} $(basename "$FILE_PATH")" >&2
    for warn in "${WARNINGS[@]}"; do
        echo -e "  ${YELLOW}⚠${RESET} $warn" >&2
    done
fi

echo -e "${GREEN}✅ Spec valid:${RESET} $(basename "$FILE_PATH")"
exit 0
