# Claude Hooks

Automated enforcement of `ai/rules/` requirements.

## Summary

**Total: 36 hooks** (25 blocking, 11 advisory)

## All Hooks

| Hook | Trigger | Rule | Mode |
|------|---------|------|------|
| `session-start.sh` | SessionStart | - | Advisory |
| `block-until-lsp.sh` | PreToolUse:.* | session-start.md | **Blocking** |
| `compaction-reminder.sh` | UserPromptSubmit | post-compaction.md | Advisory |
| `pre-compact-save.sh` | PreCompact | post-compaction.md | Advisory |
| `session-end-summary.sh` | Stop | - | Advisory |
| `block-destructive-git.sh` | PreToolUse:Bash | git-safety.md | **Blocking** |
| `block-claude-plans.sh` | PreToolUse:Write | documentation.md | **Blocking** |
| `pre-write-go.sh` | PreToolUse:Write\|Edit | post-compaction.md | **Blocking** |
| `check-existing-patterns.sh` | PreToolUse:Write | before-writing-code.md | **Blocking** |
| `block-legacy-log.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-panic-error.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-ignored-errors.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-nolint-abuse.sh` | PreToolUse:Write\|Edit | quality.md | **Blocking** |
| `require-test-first.sh` | PreToolUse:Write\|Edit | tdd.md | **Blocking** |
| `block-layering.sh` | PreToolUse:Write\|Edit | no-layering.md | **Blocking** |
| `check-existing-tests.sh` | PreToolUse:Write | before-writing-code.md | Advisory |
| `enforce-naming.sh` | PreToolUse:Write | documentation.md | **Blocking** |
| `block-throwaway-tests.sh` | PreToolUse:Write | testing.md | **Blocking** |
| `require-docs-read.sh` | PreToolUse:Write | planning.md | **Blocking** |
| `block-version-config.sh` | PreToolUse:Write\|Edit | config-design.md | **Blocking** |
| `block-lint-exclusions.sh` | PreToolUse:Write\|Edit | quality.md | **Blocking** |
| `block-exabgp-in-engine.sh` | PreToolUse:Write\|Edit | compatibility.md | **Blocking** |
| `block-silent-ignore.sh` | PreToolUse:Write\|Edit | config-design.md | **Blocking** |
| `block-yagni-violations.sh` | PreToolUse:Write\|Edit | design-principles.md | **Blocking** |
| `block-and-functions.sh` | PreToolUse:Write\|Edit | design-principles.md | **Blocking** |
| `block-init-register.sh` | PreToolUse:Write\|Edit | design-principles.md | **Blocking** |
| `block-utils-package.sh` | PreToolUse:Write | design-principles.md | **Blocking** |
| `block-temp-debug.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-encoding-alloc.sh` | PreToolUse:Write\|Edit | buffer-first.md | **Blocking** |
| `auto_linter.sh` | PostToolUse:Write\|Edit | go-standards.md | Advisory |
| `validate-spec.sh` | PostToolUse:Write\|Edit | planning.md | **Blocking** |
| `require-rfc-reference.sh` | PostToolUse:Write\|Edit | rfc-compliance.md | Advisory |
| `require-test-docs.sh` | PostToolUse:Write\|Edit | tdd.md | Advisory |
| `require-fuzz-tests.sh` | PostToolUse:Write\|Edit | tdd.md | Advisory |
| `block-vague-names.sh` | PostToolUse:Write\|Edit | design-principles.md | Advisory |
| `require-boundary-tests.sh` | PostToolUse:Write\|Edit | tdd.md | Advisory |

## By Category

### Go Code Quality (go-standards.md, quality.md)
| Hook | What it blocks/warns |
|------|----------------------|
| `block-legacy-log.sh` | `log.` package → use `slog` |
| `block-panic-error.sh` | `panic()` for error handling |
| `block-ignored-errors.sh` | `_, _ =` ignored errors |
| `block-nolint-abuse.sh` | nolint without `// reason` |
| `block-lint-exclusions.sh` | Adding linter exclusions |
| `block-silent-ignore.sh` | Silent ignore patterns |
| `block-temp-debug.sh` | `fmt.Println("DEBUG")` → use slogutil |

### TDD (tdd.md)
| Hook | What it enforces |
|------|------------------|
| `require-test-first.sh` | Test file before impl |
| `require-test-docs.sh` | VALIDATES/PREVENTS comments |
| `require-fuzz-tests.sh` | Fuzz tests for wire parsing |
| `require-boundary-tests.sh` | Boundary tests for numeric validation |

### Design Principles (design-principles.md)
| Hook | What it blocks/warns |
|------|----------------------|
| `block-yagni-violations.sh` | "in case we need", "might be useful", etc. |
| `block-and-functions.sh` | `FooAndBar` functions (split into two) |
| `block-init-register.sh` | Auto-registration in `init()` |
| `block-utils-package.sh` | `utils/`, `helpers/`, `common/` packages |
| `block-vague-names.sh` | `Data`, `Info`, `Result`, `Item`, `Thing` names |

### Architecture (no-layering.md, before-writing-code.md, compatibility.md, buffer-first.md)
| Hook | What it blocks |
|------|----------------|
| `block-layering.sh` | "backwards compat", "hybrid", "fallback" |
| `check-existing-patterns.sh` | Duplicate types/functions |
| `block-exabgp-in-engine.sh` | ExaBGP format in engine |
| `block-encoding-alloc.sh` | `append()`, `make([]byte`, `.Bytes()`, `.Pack()` in encoding paths |

### Session Lifecycle
| Hook | What it does |
|------|-------------|
| `session-start.sh` | Status summary at session start |
| `block-until-lsp.sh` | Refuses every tool call until `ToolSearch select:LSP` loads the LSP tool |
| `pre-compact-save.sh` | Auto-save session state before compaction |
| `session-end-summary.sh` | Append git state summary at session end |
| `compaction-reminder.sh` | Re-read reminder after compaction |

### Planning (planning.md, post-compaction.md)
| Hook | What it enforces |
|------|------------------|
| `pre-write-go.sh` | Session state for Go work |
| `require-docs-read.sh` | Arch docs before spec |
| `validate-spec.sh` | Spec format + Current Behavior section |

### Config (config-design.md)
| Hook | What it blocks |
|------|----------------|
| `block-version-config.sh` | Version fields in config |
| `block-silent-ignore.sh` | Must fail on unknown |

### Testing (testing.md, before-writing-code.md)
| Hook | What it enforces |
|------|------------------|
| `check-existing-tests.sh` | Warn duplicate tests |
| `block-throwaway-tests.sh` | No /tmp test files |

### Naming (documentation.md)
| Hook | What it enforces |
|------|------------------|
| `enforce-naming.sh` | File naming conventions |
| `block-claude-plans.sh` | Correct spec location |

### RFC Compliance (rfc-compliance.md)
| Hook | What it enforces |
|------|------------------|
| `require-rfc-reference.sh` | RFC comments in BGP code |

### Git Safety (git-safety.md)
| Hook | What it blocks |
|------|----------------|
| `block-destructive-git.sh` | force push, reset, clean |

## Exit Codes

| Code | Meaning | Effect |
|------|---------|--------|
| 0 | Success | Continue |
| 1 | Warning | Show message, continue |
| 2 | **Blocking** | Reject operation |

## Hook Input Format

Hooks receive JSON on stdin:

```json
{
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/path/to/file.go",
    "content": "..."
  }
}
```

For Edit tool:
```json
{
  "tool_name": "Edit",
  "tool_input": {
    "file_path": "/path/to/file.go",
    "old_string": "...",
    "new_string": "..."
  }
}
```

## Debugging

```bash
# Test a hook manually
echo '{"tool_name":"Write","tool_input":{"file_path":"test.go","content":"package main"}}' | .claude/hooks/block-legacy-log.sh
echo $?
```

## Creating New Hooks

1. Create script in `.claude/hooks/`
2. Make executable: `chmod +x .claude/hooks/your-hook.sh`
3. Add to `settings.json` under appropriate trigger
4. Document in this README

### Hook Template

```bash
#!/bin/bash
# PreToolUse hook: Description
# BLOCKING: What it blocks (rule.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process relevant tools/files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Your checks here...
ERRORS=()

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo "❌ BLOCKED: reason" >&2
    exit 2  # Blocking
fi

exit 0
```
