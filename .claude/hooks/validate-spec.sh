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
    "## Current Behavior"
    "## Data Flow"
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

# === CURRENT BEHAVIOR CHECK ===
# Ensure source files were actually read (not just placeholders)
CURRENT_BEHAVIOR_SECTION=$(sed -n '/^## Current Behavior/,/^##/p' "$FILE_PATH" | head -30)
if [[ -n "$CURRENT_BEHAVIOR_SECTION" ]]; then
    # Check for "Source files read:" with actual file paths
    if ! echo "$CURRENT_BEHAVIOR_SECTION" | grep -qE '^\s*-\s*\[\s*\]\s*`[^`]+\.(go|py|rs|ts|js)`'; then
        # No unchecked source files - check for checked ones
        if ! echo "$CURRENT_BEHAVIOR_SECTION" | grep -qE '^\s*-\s*\[x\]\s*`[^`]+\.(go|py|rs|ts|js)`'; then
            ERRORS+=("Current Behavior section must list source files read (e.g., '- [ ] \`path/to/file.go\`')")
        fi
    fi

    # Check for "Behavior to preserve:" with actual content
    if ! echo "$CURRENT_BEHAVIOR_SECTION" | grep -qiE 'behavior to preserve|preserve.*:'; then
        WARNINGS+=("Current Behavior should document 'Behavior to preserve'")
    elif echo "$CURRENT_BEHAVIOR_SECTION" | grep -qE 'Behavior to preserve.*:\s*$'; then
        # Empty behavior to preserve
        ERRORS+=("Current Behavior: 'Behavior to preserve' section is empty. Document existing behavior!")
    fi
fi

# === DATA FLOW CHECK ===
# Ensure Data Flow section has required subsections and isn't placeholder
DATA_FLOW_SECTION=$(sed -n '/^## Data Flow/,/^## /p' "$FILE_PATH" | head -50)
if [[ -n "$DATA_FLOW_SECTION" ]]; then
    # Check for required subsections
    if ! echo "$DATA_FLOW_SECTION" | grep -q "### Entry Point"; then
        ERRORS+=("Data Flow section missing '### Entry Point' subsection")
    fi
    if ! echo "$DATA_FLOW_SECTION" | grep -q "### Transformation Path"; then
        ERRORS+=("Data Flow section missing '### Transformation Path' subsection")
    fi
    if ! echo "$DATA_FLOW_SECTION" | grep -q "### Boundaries Crossed"; then
        ERRORS+=("Data Flow section missing '### Boundaries Crossed' subsection")
    fi
    if ! echo "$DATA_FLOW_SECTION" | grep -q "### Integration Points"; then
        ERRORS+=("Data Flow section missing '### Integration Points' subsection")
    fi

    # Check Entry Point isn't just placeholder
    ENTRY_CONTENT=$(echo "$DATA_FLOW_SECTION" | sed -n '/### Entry Point/,/### /p' | grep -v '^#' | head -5)
    if echo "$ENTRY_CONTENT" | grep -qE '\[Where data enters\]|\[Format at entry\]'; then
        ERRORS+=("Data Flow: Entry Point contains placeholder text. Document actual entry points!")
    fi

    # Check Transformation Path has actual stages (numbered list)
    TRANSFORM_CONTENT=$(echo "$DATA_FLOW_SECTION" | sed -n '/### Transformation Path/,/### /p' | grep -v '^#' | head -10)
    if ! echo "$TRANSFORM_CONTENT" | grep -qE '^[0-9]+\.\s+'; then
        WARNINGS+=("Data Flow: Transformation Path should have numbered stages (1. ... 2. ...)")
    fi
    if echo "$TRANSFORM_CONTENT" | grep -qE '\[Stage [0-9N]+'; then
        ERRORS+=("Data Flow: Transformation Path contains placeholder text. Document actual stages!")
    fi

    # Check Boundaries Crossed has table with content
    BOUNDARY_CONTENT=$(echo "$DATA_FLOW_SECTION" | sed -n '/### Boundaries Crossed/,/### /p' | grep -v '^#' | head -10)
    if ! echo "$BOUNDARY_CONTENT" | grep -q '|.*|.*|'; then
        ERRORS+=("Data Flow: Boundaries Crossed must use table format")
    fi
fi

# === RFC SUMMARY CHECK ===
# Extract referenced RFC summaries and check they exist
RFC_REFS=$(grep -oE '\rfc/short/rfc[0-9]+\.md' "$FILE_PATH" 2>/dev/null || true)
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

# === FEATURE INTEGRATION CHECK ===
# Ensure Files to Modify includes actual codebase files (feature code), not just tests
FILES_SECTION=$(sed -n '/^## Files to Modify/,/^##/p' "$FILE_PATH" | grep -E '^\s*-\s*`' || true)
if [[ -n "$FILES_SECTION" ]]; then
    # Check if ANY file is feature code (not _test.go, not in test/, not .ci, not qa/)
    FEATURE_FILES=$(echo "$FILES_SECTION" | grep -vE '_test\.go|test/|\.ci`|qa/' || true)
    if [[ -z "$FEATURE_FILES" ]]; then
        ERRORS+=("Files to Modify contains only test files. Feature code must be integrated into the codebase (internal/*, cmd/*)")
    fi
fi

# === FUNCTIONAL TEST CHECK ===
# Ensure spec includes functional tests for end-user verification
FUNC_TEST_SECTION=$(sed -n '/^### Functional Tests/,/^###\|^##/p' "$FILE_PATH" | head -20)
if [[ -z "$FUNC_TEST_SECTION" ]]; then
    WARNINGS+=("Missing '### Functional Tests' section. Features need functional tests to verify end-user behavior")
elif ! echo "$FUNC_TEST_SECTION" | grep -qE '\.ci|test/'; then
    WARNINGS+=("Functional Tests section should reference .ci files or test/ locations for end-user verification")
fi

# === NO CODE IN SPECS CHECK ===
# Specs must NOT contain code blocks (Go, Python, etc.)
# Exception: Markdown tables and examples of text output are allowed
CODE_BLOCKS=$(grep -cE '\`\`\`(go|python|rust|java|c|cpp|javascript|typescript)' "$FILE_PATH" 2>/dev/null | head -1 || echo "0")
if [[ "$CODE_BLOCKS" -gt 0 ]]; then
    ERRORS+=("Specs MUST NOT contain code blocks. Found $CODE_BLOCKS code block(s). Use tables/prose instead (see .claude/rules/spec-no-code.md)")
fi

# Also check for inline code that looks like function definitions (outside of code blocks)
# This catches func/def/fn at line start which shouldn't appear in prose
FUNC_DEFS=$(grep -cE '^func\s+\w+|^def\s+\w+|^fn\s+\w+' "$FILE_PATH" 2>/dev/null | head -1 || echo "0")
if [[ "$FUNC_DEFS" -gt 0 ]]; then
    ERRORS+=("Specs MUST NOT contain function definitions. Use tables/prose to describe behavior")
fi

# === OUTPUT RESULTS (compact) ===
if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}❌ Spec invalid (${#ERRORS[@]} errors):${RESET}" >&2
    for err in "${ERRORS[@]:0:5}"; do  # Max 5 errors
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    [[ ${#ERRORS[@]} -gt 5 ]] && echo -e "  ... +$((${#ERRORS[@]}-5)) more" >&2
    exit 2
fi

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}⚠ Spec: ${#WARNINGS[@]} warnings${RESET}" >&2
fi

exit 0
