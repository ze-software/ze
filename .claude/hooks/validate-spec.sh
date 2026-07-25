#!/bin/bash
# PostToolUse hook: Validate plan/spec-*.md files against planning.md rules
# BLOCKING: Rejects invalid specs

set -e

INPUT=$(cat)

# This hook reads a PostToolUse JSON payload on STDIN. It takes no argv.
#
# "I checked and found nothing" and "I could not tell what to check" are
# different answers and MUST NOT share an exit code. An unparseable payload or
# an absent tool name means NOTHING WAS CHECKED, so a silent exit 0 there is
# read by the caller as "spec valid" -- a false green. Refuse loudly instead.
# A tool name that is present but simply not ours is the legitimate no-op and
# still exits 0 quietly, below. See ai/rules/fail-closed-guards.md.
#
# How it bit: invoked as `validate-spec.sh plan/spec-foo.md`, stdin is empty, jq
# yields an empty TOOL_NAME, and the old "not Write/Edit" test exited 0. Specs
# "validated" that way passed without a single check running.
usage_refusal() {
    echo -e "${RED:-}${BOLD:-}❌ validate-spec.sh: $1 -- NOTHING WAS CHECKED.${RESET:-}" >&2
    echo "  This is a PostToolUse hook: it reads a JSON payload on stdin and takes no argv." >&2
    echo "  A silent pass here would mean 'I could not tell what to check', not 'valid'." >&2
    echo "  Manual invocation:" >&2
    echo "    echo '{\"tool_name\":\"Write\",\"tool_input\":{\"file_path\":\"plan/spec-foo.md\"}}' \\" >&2
    echo "      | bash .claude/hooks/validate-spec.sh" >&2
    echo "  Fixtures: python3 scripts/dev/hook-fixture-check.py --only validate-spec" >&2
    exit 2
}

if ! TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>&1); then
    usage_refusal "unparseable hook payload on stdin (jq: ${TOOL_NAME})"
fi
if [[ -z "$TOOL_NAME" ]]; then
    usage_refusal "no tool name in the hook payload"
fi

FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# A tool name that is present but not one we handle: legitimate no-op.
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Only process real spec files: plan/spec-*.md at a path segment boundary.
# Anchor `plan` to `^` or `/` so per-session state files like
# tmp/session/session-state-plan/spec-<stem>-<SID>.md (created by the Go
# pre-write hook when the selected spec lives under plan/) are NOT treated
# as specs. The unanchored pattern matched the substring `plan/spec-...md`
# inside those state paths and emitted spurious validation errors.
if [[ ! "$FILE_PATH" =~ (^|/)plan/spec-[^/]*\.md$ ]]; then
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

# === METADATA TABLE CHECK ===
# Every spec must have the metadata table (Status, Depends, Phase, Updated)
if ! grep -q '| Status |' "$FILE_PATH"; then
    ERRORS+=("Missing metadata table. Add after title: | Field | Value | with Status, Depends, Phase, Updated rows")
else
    # Validate Status value
    SPEC_STATUS=$(sed -n 's/^| Status | *\([a-z-]*\).*/\1/p' "$FILE_PATH" | head -1)
    case "$SPEC_STATUS" in
        skeleton|design|ready|in-progress|blocked|deferred|done) ;;
        *) ERRORS+=("Invalid Status '$SPEC_STATUS'. Must be: skeleton, design, ready, in-progress, blocked, deferred, done") ;;
    esac
    # Check Updated field exists and has a date
    if ! grep -qE '^\| Updated \| *[0-9]{4}-[0-9]{2}-[0-9]{2}' "$FILE_PATH"; then
        WARNINGS+=("Metadata: Updated field should have a date (YYYY-MM-DD)")
    fi
    # Check Phase field exists
    if ! grep -q '| Phase |' "$FILE_PATH"; then
        WARNINGS+=("Metadata: Missing Phase field")
    fi
    # Check Depends field exists
    if ! grep -q '| Depends |' "$FILE_PATH"; then
        WARNINGS+=("Metadata: Missing Depends field")
    fi
fi

# A `skeleton` spec is ALLOWED to carry template placeholders: that is the
# documented shape of a deferral holder, which fills only `## Task` and leaves
# the rest for the session that picks the work up
# (ai/rules/deferral-tracking.md, "Creating the Deferral Spec"). Blocking those
# edits made a correctly-authored skeleton un-editable, so the placeholder
# guards below warn at `skeleton` and block from `design` onward, where the
# author IS claiming the section is written.
placeholder_problem() {
    if [[ "$SPEC_STATUS" == "skeleton" ]]; then
        WARNINGS+=("$1 (allowed while Status is skeleton)")
    else
        ERRORS+=("$1")
    fi
}

# === REQUIRED SECTIONS ===
REQUIRED_SECTIONS=(
    "## Task"
    "## Required Reading"
    "## Current Behavior"
    "## Data Flow"
    "## Wiring Test"
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
# Ensure source files were actually read (not just placeholders).
# Read the WHOLE section (not a fixed 30-line window): a long section's
# "Behavior to preserve" heading used to fall outside a `head -30` window and
# warn permanently, training readers to ignore validator output (T-1). sed's
# range already stops at the next `## ` heading.
CURRENT_BEHAVIOR_SECTION=$(sed -n '/^## Current Behavior/,/^## /p' "$FILE_PATH")
if [[ -n "$CURRENT_BEHAVIOR_SECTION" ]]; then
    # A cited source is a backticked path ending in a known source extension OR
    # named `Makefile`, OPTIONALLY followed by `:<line>` -- the exact citation
    # form ai/rules/no-fabrication.md mandates (path plus line number). `.sh`,
    # `.mk` and `Makefile` are included so a spec about shell/make tooling can
    # cite the file it is about. The `- [ ]`/`- [x]` checkbox anchor keeps prose
    # out: a sentence cannot match, so this accepts the mandated form without
    # accepting everything (ai/rules/fail-closed-guards.md).
    # A basename is required before the extension (`+`, not `*`) so a bare
    # `` `.go` `` -- an empty path component -- is rejected, not accepted as a
    # valid-looking zero (ai/rules/fail-closed-guards.md). The empty prefix is
    # allowed ONLY for the literal Makefile (which has no basename+extension).
    _CB_SRC='`([^`]+\.(go|py|rs|ts|js|sh|mk)|[^`]*Makefile)(:[0-9]+)?`'
    # Check for "Source files read:" with actual file paths
    if ! echo "$CURRENT_BEHAVIOR_SECTION" | grep -qE "^[[:space:]]*-[[:space:]]*\[[[:space:]]*\][[:space:]]*${_CB_SRC}"; then
        # No unchecked source files - check for checked ones
        if ! echo "$CURRENT_BEHAVIOR_SECTION" | grep -qE "^[[:space:]]*-[[:space:]]*\[x\][[:space:]]*${_CB_SRC}"; then
            ERRORS+=("Current Behavior section must list source files read (e.g., '- [ ] \`path/to/file.go\`' or '- [ ] \`scripts/dev/foo.py:42\`')")
        fi
    fi

    # Check for "Behavior to preserve:" with actual content
    if ! echo "$CURRENT_BEHAVIOR_SECTION" | grep -qiE 'behavior to preserve|preserve.*:'; then
        WARNINGS+=("Current Behavior should document 'Behavior to preserve'")
    elif echo "$CURRENT_BEHAVIOR_SECTION" | grep -qE 'Behavior to preserve.*:[[:space:]]*$'; then
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

    # Check Entry Point isn't just placeholder.
    # `\[Where data enters` is deliberately NOT anchored with a closing bracket:
    # the template's own placeholder is `[Where data enters: wire bytes, ...]`,
    # which the old `\[Where data enters\]` alternative never matched. The guard
    # only ever fired through `[Format at entry]`, so editing that ONE line while
    # leaving the other let a placeholder through (ai/rules/fail-closed-guards.md).
    ENTRY_CONTENT=$(echo "$DATA_FLOW_SECTION" | sed -n '/### Entry Point/,/### /p' | grep -v '^#' | head -5)
    if echo "$ENTRY_CONTENT" | grep -qE '\[Where data enters|\[Format at entry\]'; then
        placeholder_problem "Data Flow: Entry Point contains placeholder text. Document actual entry points!"
    fi

    # Check Transformation Path has actual stages (numbered list)
    TRANSFORM_CONTENT=$(echo "$DATA_FLOW_SECTION" | sed -n '/### Transformation Path/,/### /p' | grep -v '^#' | head -10)
    if ! echo "$TRANSFORM_CONTENT" | grep -qE '^[0-9]+\.[[:space:]]+'; then
        WARNINGS+=("Data Flow: Transformation Path should have numbered stages (1. ... 2. ...)")
    fi
    if echo "$TRANSFORM_CONTENT" | grep -qE '\[Stage [0-9N]+'; then
        placeholder_problem "Data Flow: Transformation Path contains placeholder text. Document actual stages!"
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
)

for item in "${REQUIRED_CHECKLIST[@]}"; do
    if ! grep -q "$item" "$FILE_PATH"; then
        ERRORS+=("Missing checklist item: $item")
    fi
done

# === VERIFICATION COMMAND ===
# ONE command names the pre-commit gate, and it is `make ze-verify`
# (ai/rules/git-safety.md Step 1). This gate used to demand the literal string
# `make ze-test`, which is `ze-lint ze-unit-test ze-functional-test
# ze-exabgp-test ze-fuzz-test` (Makefile) -- the fuzz-inclusive target that the
# commit rule does NOT use. The template shipped all three spellings at once.
# Accept the legacy string so the 50 specs predating this change keep
# validating, but warn, so new specs converge on the real gate.
VERIFY_CMD='make ze-'"verify"
LEGACY_CMD='make ze-'"test"
if grep -q "$VERIFY_CMD" "$FILE_PATH"; then
    :
elif grep -q "$LEGACY_CMD" "$FILE_PATH"; then
    WARNINGS+=("Checklist names '$LEGACY_CMD' (lint+unit+functional+exabgp+fuzz). The pre-commit gate is '$VERIFY_CMD' (ai/rules/git-safety.md) -- prefer it")
else
    ERRORS+=("Missing verification checklist item: '$VERIFY_CMD' (the pre-commit gate, ai/rules/git-safety.md)")
fi

# === RFC CONSTRAINT DOCS CHECK ===
# If protocol work (references RFCs), should have RFC Documentation section
if [[ -n "$RFC_REFS" ]]; then
    if ! grep -q "## RFC Documentation" "$FILE_PATH"; then
        WARNINGS+=("Protocol work should have '## RFC Documentation' section")
    fi
fi

# === FEATURE INTEGRATION CHECK ===
# Ensure Files to Modify includes actual codebase files (feature code), not just tests
FILES_SECTION=$(sed -n '/^## Files to Modify/,/^##/p' "$FILE_PATH" | grep -E '^[[:space:]]*-[[:space:]]*`' || true)
if [[ -n "$FILES_SECTION" ]]; then
    # Check if ANY file is feature code (not _test.go, not in test/, not .ci, not qa/)
    FEATURE_FILES=$(echo "$FILES_SECTION" | grep -vE '_test\.go|test/|\.ci`|qa/' || true)
    if [[ -z "$FEATURE_FILES" ]]; then
        ERRORS+=("Files to Modify contains only test files. Feature code must be integrated into the codebase (internal/*, cmd/*)")
    fi
fi

# === FUNCTIONAL TEST CHECK (BLOCKING) ===
# Every spec with user-facing features MUST have .ci functional tests.
# A `.ci` drives the ze daemon, so only a spec that changes daemon Go
# (internal/*.go or cmd/*.go) can have one. A spec that only edits hooks, dev
# scripts, or docs cannot -- its real functional surface is a driving test in
# that language (e.g. scripts/dev/hook-fixture-check.py). Scope the requirement
# to daemon specs rather than dropping it (T-5): the `.ci` rule exists because
# unit tests alone let 30 tests ship that never bound a peer, and daemon specs
# still owe one.
# pkg/ is the plugin SDK compiled into the daemon (pkg/plugin, pkg/ze), so it
# counts as daemon code exactly as T-4's evidence set does (internal/ pkg/ cmd/).
# Omitting it would let a pkg/-only user-facing spec skip the .ci the old gate
# required.
FILES_TO_MODIFY=$(sed -n '/^## Files to Modify/,/^## /p' "$FILE_PATH")
TOUCHES_DAEMON=false
if echo "$FILES_TO_MODIFY" | grep -qE '`[^`]*(internal|pkg|cmd)/[^`]*\.go`'; then
    TOUCHES_DAEMON=true
fi
FUNC_TEST_SECTION=$(sed -n '/^### Functional Tests/,/^###\|^##/p' "$FILE_PATH" | head -20)
if [[ -z "$FUNC_TEST_SECTION" ]]; then
    ERRORS+=("Missing '### Functional Tests' section. User-facing features MUST have .ci tests (see rules/integration-completeness.md)")
elif echo "$FUNC_TEST_SECTION" | grep -qiE 'N/A|not applicable|no new .* features|no user-facing|cosmetic|existing test|no regressions|test suite passes'; then
    : # Explicit opt-out for internal/cosmetic specs — allowed
elif echo "$FUNC_TEST_SECTION" | grep -qE '\.ci'; then
    : # names a .ci functional test — always acceptable
elif [[ "$TOUCHES_DAEMON" == false ]] && echo "$FUNC_TEST_SECTION" | grep -qE '`?[A-Za-z0-9_./-]+\.(py|sh)`?'; then
    : # tooling-only spec (no daemon code) naming a concrete .py/.sh driving surface — allowed
else
    ERRORS+=("Functional Tests section must reference .ci test files (this spec touches daemon code). A Go unit test is NOT a substitute for a .ci functional test")
fi

# === NO CODE IN SPECS CHECK ===
# Specs must NOT contain code blocks (Go, Python, etc.)
# Exception: Markdown tables and examples of text output are allowed
CODE_BLOCKS=$(grep -cE '```(go|python|rust|java|c|cpp|javascript|typescript)' "$FILE_PATH" 2>/dev/null | head -1 || echo "0")
if [[ "$CODE_BLOCKS" -gt 0 ]]; then
    ERRORS+=("Specs MUST NOT contain code blocks. Found $CODE_BLOCKS code block(s). Use tables/prose instead (see ai/rules/spec-no-code.md)")
fi

# Also check for inline code that looks like function definitions (outside of code blocks)
# This catches func/def/fn at line start which shouldn't appear in prose
FUNC_DEFS=$(grep -cE '^func[[:space:]]+[[:alnum:]_]+|^def[[:space:]]+[[:alnum:]_]+|^fn[[:space:]]+[[:alnum:]_]+' "$FILE_PATH" 2>/dev/null | head -1 || echo "0")
if [[ "$FUNC_DEFS" -gt 0 ]]; then
    ERRORS+=("Specs MUST NOT contain function definitions. Use tables/prose to describe behavior")
fi

# === WIRING TEST CHECK (BLOCKING) ===
# Every spec MUST have a Wiring Test section with concrete test names
# (Note: "## Wiring Test" is also in REQUIRED_SECTIONS for the basic check above;
#  this block adds detailed validation of table content)
if ! grep -q "^## Wiring Test" "$FILE_PATH"; then
    : # Already caught by REQUIRED_SECTIONS check above
else
    WIRING_SECTION=$(sed -n '/^## Wiring Test/,/^## /p' "$FILE_PATH" | head -30)
    # Must have a table. Accept BOTH arrow conventions: the Unicode arrow (→,
    # plan/TEMPLATE.md) and ASCII (->, .claude/rules/post-compaction.md). Both are
    # institutionalized; matching only → mislabeled ~40 legacy specs as tableless.
    if ! echo "$WIRING_SECTION" | grep -qE '\|.*(→|->).*\|'; then
        ERRORS+=("Wiring Test section must have table with Entry Point -> Feature Code -> Test columns")
    fi
    # Check for placeholder/deferred/empty test cells
    # Table rows look like: | entry | -> | code | test |  (or → )
    # Extract the last column (test name) from data rows (skip header + separator).
    # The trailing `|| true` is load-bearing: under `set -e`, a grep pipeline that
    # selects no lines exits 1 and aborts the whole script BEFORE the output stage,
    # swallowing every queued ERROR. `|| true` keeps WIRING_ROWS empty instead.
    WIRING_ROWS=$(echo "$WIRING_SECTION" | grep -E '\|.*(→|->).*\|' | grep -v '^|.*Entry Point' | grep -v '^|.*---' || true)
    if [[ -n "$WIRING_ROWS" ]]; then
        HAS_CI_TEST=false
        # Check each row's test column for deferred/TODO/placeholder/empty
        while IFS= read -r row; do
            TEST_CELL=$(echo "$row" | awk -F'|' '{print $NF}' | sed 's/^[ \t]*//;s/[ \t]*$//')
            # If $NF is empty (trailing |), get second to last
            if [[ -z "$TEST_CELL" ]]; then
                TEST_CELL=$(echo "$row" | awk -F'|' '{print $(NF-1)}' | sed 's/^[ \t]*//;s/[ \t]*$//')
            fi
            if [[ -z "$TEST_CELL" ]] || echo "$TEST_CELL" | grep -qiE 'deferred|TODO|TBD|\[test name|^\?\?\?$|^$'; then
                placeholder_problem "Wiring Test: every row must have a concrete test name. Found deferred/empty: '$TEST_CELL'"
                break
            fi
            # Track if any test references a .ci file
            if echo "$TEST_CELL" | grep -qE '\.ci'; then
                HAS_CI_TEST=true
            fi
        done <<< "$WIRING_ROWS"
        # User-facing features need .ci tests in wiring table (unless all rows reference existing Go test suite)
        if [[ "$HAS_CI_TEST" == false ]]; then
            # Check if spec explicitly opts out (cosmetic/internal work)
            if ! echo "$WIRING_SECTION" | grep -qiE 'existing test|no new feature|cosmetic|N/A'; then
                WARNINGS+=("Wiring Test: user-facing features should reference .ci functional tests, not just Go unit tests")
            fi
        fi
    fi
fi

# === ACCEPTANCE CRITERIA CHECK ===
# New specs should have Acceptance Criteria section with AC-N table rows
if ! grep -q "^## Acceptance Criteria" "$FILE_PATH"; then
    WARNINGS+=("Missing '## Acceptance Criteria' section. Define testable AC-N assertions before implementation")
else
    AC_SECTION=$(sed -n '/^## Acceptance Criteria/,/^##/p' "$FILE_PATH" | head -20)
    if ! echo "$AC_SECTION" | grep -qE 'AC-[0-9]+'; then
        WARNINGS+=("Acceptance Criteria section should have AC-N table rows (e.g., AC-1, AC-2)")
    elif ! echo "$AC_SECTION" | grep -q '|.*|.*|'; then
        WARNINGS+=("Acceptance Criteria section should use table format (| AC ID | Input / Condition | Expected Behavior |)")
    fi
fi

# === RISKS & ASSUMPTIONS CHECK ===
# New specs should record design-time assumptions and risks (WARNING only:
# specs created before the section existed are exempt -- see ai/rules/planning.md).
if ! grep -q "^## Risks & Assumptions" "$FILE_PATH"; then
    WARNINGS+=("Missing '## Risks & Assumptions' section. Record assumptions (A-N) and risks (R-N) from design gates (see plan/TEMPLATE.md)")
else
    RA_SECTION=$(sed -n '/^## Risks & Assumptions/,/^## /p' "$FILE_PATH" | head -40)
    if ! echo "$RA_SECTION" | grep -qE 'A-[0-9]+'; then
        WARNINGS+=("Risks & Assumptions: Assumptions table should have A-N rows with Basis + validation method")
    fi
fi

# === CONTEXT CHECKPOINT CHECK ===
# Required Reading entries should have → Decision: or → Constraint: checkpoint lines
REQ_READING_SECTION=$(sed -n '/^## Required Reading/,/^## /p' "$FILE_PATH" | head -40)
if [[ -n "$REQ_READING_SECTION" ]]; then
    # Count doc entries (checkbox lines: - [ ] or - [x])
    DOC_ENTRIES=$(echo "$REQ_READING_SECTION" | grep -cE '^[[:space:]]*-[[:space:]]*\[[[:space:]]*[x ][[:space:]]*\]' || true)
    DOC_ENTRIES=${DOC_ENTRIES:-0}
    # Count checkpoint lines
    CHECKPOINT_LINES=$(echo "$REQ_READING_SECTION" | grep -cE '^[[:space:]]*→[[:space:]]*(Decision|Constraint):' || true)
    CHECKPOINT_LINES=${CHECKPOINT_LINES:-0}
    if [[ "$DOC_ENTRIES" -gt 0 && "$CHECKPOINT_LINES" -eq 0 ]]; then
        WARNINGS+=("Required Reading entries should have '→ Decision:' or '→ Constraint:' checkpoint annotations")
    fi
fi

# === GOAL GATES CHECK ===
# New specs should split checklist into Goal Gates and Quality Gates
if grep -q "^## Checklist" "$FILE_PATH"; then
    if ! grep -q "### Goal Gates" "$FILE_PATH"; then
        WARNINGS+=("Checklist should use '### Goal Gates' and '### Quality Gates' split")
    fi
fi

# === REGISTRATION-OVER-HARDCODING CHECK ===
# Every spec should carry the registration-over-hardcoding review item: new
# features register and the core discovers them, instead of adding a per-feature
# field/switch/case/factory to a core or shared package (small-core/registration;
# ai/rules/plugin-self-containment.md). WARNING only -- specs predating this rule
# are exempt; plan/TEMPLATE.md adds it to every newly authored spec.
if ! grep -qi 'Registration over hardcoding' "$FILE_PATH"; then
    WARNINGS+=("Missing 'Registration over hardcoding' review item (add to Critical Review Checklist + Architectural Verification). New commands/views/families/handlers must register and be core-discovered, not hardcoded into a core/shared package (ai/rules/plugin-self-containment.md)")
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
