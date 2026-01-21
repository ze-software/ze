#!/bin/bash
# PreToolUse hook: Block writes to .claude/plans/, enforce spec requirements
# BLOCKING: Rejects wrong plan location
# NON-BLOCKING: Reminds about required reading for spec files

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write tool
if [[ "$TOOL_NAME" != "Write" ]]; then
    exit 0
fi

# Check if writing to .claude/plans/ - BLOCK
if [[ "$FILE_PATH" =~ \.claude/plans/ ]]; then
    echo "❌ BLOCKED: Do not use .claude/plans/" >&2
    echo "" >&2
    echo "Create spec at: docs/plan/spec-<task>.md" >&2
    echo "Use template from: .claude/rules/planning.md" >&2
    exit 2  # BLOCKING
fi

# Check if writing to docs/plan/spec-*.md - REMIND about required reading
if [[ "$FILE_PATH" =~ docs/plan/spec-.*\.md ]]; then
    echo "📚 REQUIRED READING before writing spec:" >&2
    echo "" >&2
    echo "1. Re-read .claude/rules/planning.md (has keyword→doc mapping)" >&2
    echo "2. Read docs/architecture/core-design.md (always required)" >&2
    echo "3. Match task keywords to docs in planning.md table" >&2
    echo "4. Read ALL matched architecture docs" >&2
    echo "5. For protocol work: check rfc/short/ summaries exist" >&2
    echo "" >&2
    echo "Spec MUST include:" >&2
    echo "  ## Required Reading - with [ ] checkboxes for each doc" >&2
    echo "  ## 🧪 TDD Test Plan - tests BEFORE implementation" >&2
    echo "" >&2
    # Non-blocking - just a reminder
    exit 0
fi

exit 0
