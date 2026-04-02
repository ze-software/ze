#!/bin/bash
# BLOCKING HOOK: Verify spec obligations before git commit
# Prevents committing when selected spec has unfulfilled obligations.
# This is the last line of defense — Claude cannot rationalize past exit 2.
#
# Checks:
# 1. Every .ci file in Wiring Test / Functional Tests exists on disk
# 2. Every file in "Files to Create" exists on disk
# 3. Every test file in TDD Plan exists on disk
# 4. Implementation Audit tables have data rows (not empty templates)
# 5. Every AC row in audit has non-empty "Demonstrated By" evidence
# 6. Audit Summary has actual numbers
# 7. Pre-Commit Verification section exists and has data rows
# 8. Learned summary exists and does not claim incompleteness
#
# Bypass: clear .claude/selected-spec to commit unrelated work.
# Exit code 2 = BLOCK the commit.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only trigger on git commit commands
if [[ "$COMMAND" != *"git commit"* ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

# Check if there's a selected spec
if [[ ! -f .claude/selected-spec ]]; then
    exit 0
fi

SPEC_NAME=$(grep -v '^#' .claude/selected-spec 2>/dev/null | grep -v '^$' | tail -1 | tr -d '[:space:]')
if [[ -z "$SPEC_NAME" ]]; then
    exit 0
fi

SPEC_FILE="plan/$SPEC_NAME"
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
        ERRORS+=("Wiring Test: '$ci_file' does not exist on disk")
    fi
done

# === CHECK 2: Functional Tests .ci files exist on disk ===
FUNC_SECTION=$(sed -n '/^### Functional Tests/,/^###\|^## /p' "$SPEC_FILE")
FUNC_CI=$(echo "$FUNC_SECTION" | grep -oE 'test/[a-zA-Z0-9_./-]+\.ci' | sort -u)

for ci_file in $FUNC_CI; do
    if [[ ! -f "$ci_file" ]]; then
        ERRORS+=("Functional Tests: '$ci_file' does not exist on disk")
    fi
done

# === CHECK 3: Files to Create — all exist on disk ===
CREATE_SECTION=$(sed -n '/^## Files to Create/,/^## /p' "$SPEC_FILE")
# Extract paths: lines like "- `internal/component/authz/authz.go`" or "- `test/parse/foo.ci`"
CREATE_FILES=$(echo "$CREATE_SECTION" | grep -oE '`[a-zA-Z0-9_./-]+\.(go|ci|yang|md)`' | tr -d '`' | sort -u)

for file in $CREATE_FILES; do
    if [[ ! -f "$file" ]]; then
        ERRORS+=("Files to Create: '$file' does not exist on disk")
    fi
done

# === CHECK 4: TDD Plan test files exist ===
TDD_SECTION=$(sed -n '/^### Unit Tests/,/^### Boundary\|^### Functional\|^## /p' "$SPEC_FILE")
# Extract test file paths from table: second column typically has the file path
TDD_FILES=$(echo "$TDD_SECTION" | grep -oE '`[a-zA-Z0-9_./-]+_test\.go`' | tr -d '`' | sort -u)

for test_file in $TDD_FILES; do
    if [[ ! -f "$test_file" ]]; then
        ERRORS+=("TDD Plan: test file '$test_file' does not exist on disk")
    fi
done

# === CHECK 5: Implementation Audit tables have data rows ===
AUDIT_SECTION=$(sed -n '/^## Implementation Audit/,/^## Pre-Commit\|^## Checklist/p' "$SPEC_FILE")

# Requirements from Task
REQ_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Requirements from Task/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*Requirement|^\|.*---' | wc -l)
if [[ "$REQ_ROWS" -lt 1 ]]; then
    ERRORS+=("Audit: 'Requirements from Task' table is empty")
fi

# Acceptance Criteria — table must have rows
AC_TABLE=$(echo "$AUDIT_SECTION" | sed -n '/^### Acceptance Criteria/,/^### /p')
AC_ROWS=$(echo "$AC_TABLE" | grep -E '^\|' | grep -vE '^\|.*AC ID|^\|.*---' | wc -l)
if [[ "$AC_ROWS" -lt 1 ]]; then
    ERRORS+=("Audit: 'Acceptance Criteria' table is empty")
fi

# === CHECK 6: Every AC row has non-empty "Demonstrated By" ===
# Table format: | AC-N | Status | Demonstrated By | Notes |
# "Demonstrated By" is the 3rd column (4th field with awk -F'|')
if [[ "$AC_ROWS" -gt 0 ]]; then
    EMPTY_EVIDENCE=0
    while IFS= read -r row; do
        # Skip header and separator
        if echo "$row" | grep -qE '^\|.*AC ID|^\|.*---'; then
            continue
        fi
        # Extract "Demonstrated By" column (3rd data column = field 4)
        EVIDENCE=$(echo "$row" | awk -F'|' '{print $4}' | sed 's/^[ \t]*//;s/[ \t]*$//')
        if [[ -z "$EVIDENCE" ]]; then
            AC_ID=$(echo "$row" | awk -F'|' '{print $2}' | sed 's/^[ \t]*//;s/[ \t]*$//')
            ERRORS+=("Audit: $AC_ID has empty 'Demonstrated By' — every AC needs evidence")
            EMPTY_EVIDENCE=$((EMPTY_EVIDENCE + 1))
            # Stop after 3 to avoid flooding
            if [[ "$EMPTY_EVIDENCE" -ge 3 ]]; then
                ERRORS+=("Audit: ... and more ACs with empty evidence")
                break
            fi
        fi
    done <<< "$(echo "$AC_TABLE" | grep -E '^\|')"
fi

# Tests from TDD Plan
TEST_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Tests from TDD Plan/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*Test |^\|.*---' | wc -l)
if [[ "$TEST_ROWS" -lt 1 ]]; then
    ERRORS+=("Audit: 'Tests from TDD Plan' table is empty")
fi

# Files from Plan
FILE_ROWS=$(echo "$AUDIT_SECTION" | sed -n '/^### Files from Plan/,/^### /p' \
    | grep -E '^\|' | grep -vE '^\|.*File |^\|.*---' | wc -l)
if [[ "$FILE_ROWS" -lt 1 ]]; then
    ERRORS+=("Audit: 'Files from Plan' table is empty")
fi

# Audit Summary — must have actual numbers
SUMMARY_SECTION=$(echo "$AUDIT_SECTION" | sed -n '/^### Audit Summary/,/^## \|^### /p')
if echo "$SUMMARY_SECTION" | grep -qE 'Total items:\s*$'; then
    ERRORS+=("Audit: Audit Summary has no totals")
fi

# === CHECK 7: Pre-Commit Verification section exists and is filled ===
VERIFY_SECTION=$(sed -n '/^## Pre-Commit Verification/,/^## Checklist/p' "$SPEC_FILE")

if [[ -z "$VERIFY_SECTION" ]]; then
    ERRORS+=("Missing '## Pre-Commit Verification' section — add it from TEMPLATE.md")
else
    # Files Exist table must have data rows
    FILES_EXIST_ROWS=$(echo "$VERIFY_SECTION" | sed -n '/^### Files Exist/,/^### /p' \
        | grep -E '^\|' | grep -vE '^\|.*File |^\|.*Exists|^\|.*---' | wc -l)
    if [[ "$FILES_EXIST_ROWS" -lt 1 ]]; then
        ERRORS+=("Pre-Commit Verification: 'Files Exist' table is empty — verify files with ls")
    fi

    # AC Verified table must have data rows
    AC_VERIFY_ROWS=$(echo "$VERIFY_SECTION" | sed -n '/^### AC Verified/,/^### /p' \
        | grep -E '^\|' | grep -vE '^\|.*AC ID|^\|.*Claim|^\|.*---' | wc -l)
    if [[ "$AC_VERIFY_ROWS" -lt 1 ]]; then
        ERRORS+=("Pre-Commit Verification: 'AC Verified' table is empty — re-verify each AC independently")
    fi

    # Wiring Verified table must have data rows
    WIRING_VERIFY_ROWS=$(echo "$VERIFY_SECTION" | sed -n '/^### Wiring Verified/,/^### \|^## /p' \
        | grep -E '^\|' | grep -vE '^\|.*Entry Point|^\|.*Verified|^\|.*---' | wc -l)
    if [[ "$WIRING_VERIFY_ROWS" -lt 1 ]]; then
        ERRORS+=("Pre-Commit Verification: 'Wiring Verified' table is empty — verify each wiring path")
    fi
fi

# === CHECK 8: Learned summary exists and is accurate ===
SPEC_BASENAME=$(basename "$SPEC_FILE" .md | sed 's/^spec-//')
LEARNED_FILE=$(ls plan/learned/*-${SPEC_BASENAME}*.md 2>/dev/null | head -1)

if [[ -z "$LEARNED_FILE" ]]; then
    ERRORS+=("No learned summary for '$SPEC_BASENAME' in plan/learned/")
elif grep -qiE 'not yet implemented|not wired|infrastructure only|library only|not yet wired|wiring not.*implemented' "$LEARNED_FILE"; then
    ERRORS+=("Learned summary '$LEARNED_FILE' says work is incomplete — fix or finish the work")
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
