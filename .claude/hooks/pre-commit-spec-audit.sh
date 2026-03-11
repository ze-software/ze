#!/bin/bash
# BLOCKING HOOK: Verify spec obligations before git commit
# Prevents committing when selected spec has unfulfilled obligations:
# 1. Wiring Test / Functional Test .ci files listed but not created on disk
# 2. Implementation Audit tables empty (no data rows)
# 3. Learned summary contains "not done" language
#
# Bypass: clear .claude/selected-spec to commit unrelated work.
# Exit code 2 = BLOCK the commit.

COMMAND="$CLAUDE_TOOL_INPUT_command"

# Only trigger on git commit commands
if [[ "$COMMAND" != *"git commit"* ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

# Check if there's a selected spec
if [[ ! -f .claude/selected-spec ]]; then
    exit 0
fi

SPEC_NAME=$(tr -d '[:space:]' < .claude/selected-spec)
if [[ -z "$SPEC_NAME" ]]; then
    exit 0
fi

SPEC_FILE="docs/plan/$SPEC_NAME"
if [[ ! -f "$SPEC_FILE" ]]; then
    # Spec file doesn't exist (already deleted/moved) — skip
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# === CHECK 1: Wiring Test .ci files exist on disk ===
WIRING_SECTION=$(sed -n '/^## Wiring Test/,/^## [^W]/p' "$SPEC_FILE")
WIRING_CI=$(echo "$WIRING_SECTION" | grep -oE 'test/[a-zA-Z0-9_./-]+\.ci' | sort -u)

for ci_file in $WIRING_CI; do
    if [[ ! -f "$ci_file" ]]; then
        ERRORS+=("Wiring Test promises '$ci_file' — file does not exist")
    fi
done

# === CHECK 2: Functional Tests .ci files exist on disk ===
FUNC_SECTION=$(sed -n '/^### Functional Tests/,/^###\|^## /p' "$SPEC_FILE")
FUNC_CI=$(echo "$FUNC_SECTION" | grep -oE 'test/[a-zA-Z0-9_./-]+\.ci' | sort -u)

for ci_file in $FUNC_CI; do
    if [[ ! -f "$ci_file" ]]; then
        ERRORS+=("Functional Tests promises '$ci_file' — file does not exist")
    fi
done

# === CHECK 3: Implementation Audit tables have data rows ===
AUDIT_SECTION=$(sed -n '/^## Implementation Audit/,/^## [^I]/p' "$SPEC_FILE")

# Requirements from Task
REQ_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Requirements from Task/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*Requirement|^\|.*---' | wc -l)
if [[ "$REQ_ROWS" -lt 1 ]]; then
    ERRORS+=("Implementation Audit: 'Requirements from Task' table is empty")
fi

# Acceptance Criteria
AC_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Acceptance Criteria/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*AC ID|^\|.*---' | wc -l)
if [[ "$AC_ROWS" -lt 1 ]]; then
    ERRORS+=("Implementation Audit: 'Acceptance Criteria' table is empty")
fi

# Tests from TDD Plan
TEST_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Tests from TDD Plan/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*Test |^\|.*---' | wc -l)
if [[ "$TEST_ROWS" -lt 1 ]]; then
    ERRORS+=("Implementation Audit: 'Tests from TDD Plan' table is empty")
fi

# Files from Plan
FILE_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Files from Plan/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*File |^\|.*---' | wc -l)
if [[ "$FILE_ROWS" -lt 1 ]]; then
    ERRORS+=("Implementation Audit: 'Files from Plan' table is empty")
fi

# Audit Summary — must have actual numbers, not template placeholders
SUMMARY_SECTION=$(echo "$AUDIT_SECTION" | sed -n '/^### Audit Summary/,/^## \|^### /p')
if echo "$SUMMARY_SECTION" | grep -qE 'Total items:\s*$'; then
    ERRORS+=("Implementation Audit: Audit Summary has no totals — complete the audit")
fi

# === CHECK 4: Learned summary does not claim incomplete ===
SPEC_BASENAME=$(basename "$SPEC_FILE" .md | sed 's/^spec-//')
LEARNED_FILE=$(ls docs/learned/*-${SPEC_BASENAME}*.md 2>/dev/null | head -1)

if [[ -z "$LEARNED_FILE" ]]; then
    ERRORS+=("No learned summary for '$SPEC_BASENAME' in docs/learned/")
elif grep -qiE 'not yet implemented|not wired|infrastructure only|library only|not yet wired|wiring not.*implemented' "$LEARNED_FILE"; then
    ERRORS+=("Learned summary '$LEARNED_FILE' says work is incomplete — fix it or finish the work")
fi

# === OUTPUT ===
if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ Spec obligations not met — commit blocked (${#ERRORS[@]} issues):${RESET}" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "${YELLOW}Fix these, or clear .claude/selected-spec for unrelated commits.${RESET}" >&2
    echo -e "See: rules/implementation-audit.md, rules/integration-completeness.md" >&2
    exit 2
fi

exit 0
