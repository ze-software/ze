# Claude Hooks

Automated enforcement of `ai/rules/` requirements.

## Summary

**Total: 40 rows below** (26 blocking, 14 advisory)

> **This file is STALE and it is not the inventory.** Most rows name per-check
> shell scripts that no longer exist: they were folded into the three Python
> dispatchers (`pretool-bash.py`, `pretool-writeedit.py`,
> `posttool-writeedit.py`). Several registered hooks are missing from the table.
> **`ai/rules/repo-maintenance.md` is the real inventory, and `.claude/settings.json`
> is the authority on what actually runs.** Check registration there before you
> trust any row here.

## All Hooks

| Hook | Trigger | Rule | Mode |
|------|---------|------|------|
| `session-start.sh` | SessionStart | - | Advisory |
| `block-until-lsp.sh` | PreToolUse:.* | session-start.md | **Blocking** |
| `compaction-reminder.sh` | UserPromptSubmit | post-compaction.md | Advisory (stderr, which by convention does not reach the model) |
| `verify-claim-reminder.sh` | UserPromptSubmit | evidence.md | Advisory (stdout, which by convention reaches the model) |
| `delegation-reminder.sh` | UserPromptSubmit | planning.md | Advisory (stdout, which by convention reaches the model) |
| `pre-compact-save.sh` | PreCompact | post-compaction.md | Advisory |
| `block-premature-stop.sh` | Stop (first) | completion.md, planning.md, planning.md, session-start.md | **Blocking**. Re-registered 2026-07-31, after no event from 2026-06-29 (`41e5fa44f`). Phrase scan has two tiers, and the completion tier is state-gated. `stop_hook_active` skips the phrase scan alone, and the other three gates still run |
| `session-end-summary.sh` | Stop | - | Advisory |
| `block-destructive-git.sh` | PreToolUse:Bash | git-safety.md | **Blocking** |
| `block-claude-plans.sh` | PreToolUse:Write | writing.md | **Blocking** |
| `pre-write-go.sh` | PreToolUse:Write\|Edit | post-compaction.md | **Blocking** |
| `check-existing-patterns.sh` | PreToolUse:Write | architecture.md | **Blocking** |
| `block-legacy-log.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-panic-error.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-ignored-errors.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-nolint-abuse.sh` | PreToolUse:Write\|Edit | quality.md | **Blocking** |
| `require-test-first.sh` | PreToolUse:Write\|Edit | testing.md | **Blocking** |
| `block-layering.sh` | PreToolUse:Write\|Edit | no-layering.md | **Blocking** |
| `check-existing-tests.sh` | PreToolUse:Write | architecture.md | Advisory |
| `enforce-naming.sh` | PreToolUse:Write | writing.md | **Blocking** |
| `block-throwaway-tests.sh` | PreToolUse:Write | testing.md | **Blocking** |
| `require-docs-read.sh` | PreToolUse:Write | planning.md | **Blocking** |
| `block-version-config.sh` | PreToolUse:Write\|Edit | config.md | **Blocking** |
| `block-lint-exclusions.sh` | PreToolUse:Write\|Edit | quality.md | **Blocking** |
| `block-exabgp-in-engine.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-silent-ignore.sh` | PreToolUse:Write\|Edit | config.md | **Blocking** |
| `block-yagni-violations.sh` | PreToolUse:Write\|Edit | architecture.md | **Blocking** |
| `block-and-functions.sh` | PreToolUse:Write\|Edit | architecture.md | **Blocking** |
| `block-init-register.sh` | PreToolUse:Write\|Edit | architecture.md | **Blocking** |
| `block-utils-package.sh` | PreToolUse:Write | architecture.md | **Blocking** |
| `block-temp-debug.sh` | PreToolUse:Write\|Edit | go-standards.md | **Blocking** |
| `block-encoding-alloc.sh` | PreToolUse:Write\|Edit | performance.md | **Blocking** |
| `auto_linter.sh` | PostToolUse:Write\|Edit | go-standards.md | Advisory |
| `validate-spec.sh` | PostToolUse:Write\|Edit | planning.md | **Blocking** |
| `require-rfc-reference.sh` | PostToolUse:Write\|Edit | rfc-compliance.md | Advisory |
| `require-test-docs.sh` | PostToolUse:Write\|Edit | testing.md | Advisory |
| `require-fuzz-tests.sh` | PostToolUse:Write\|Edit | testing.md | Advisory |
| `block-vague-names.sh` | PostToolUse:Write\|Edit | architecture.md | Advisory |
| `require-boundary-tests.sh` | PostToolUse:Write\|Edit | testing.md | Advisory |

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

### TDD (testing.md)
| Hook | What it enforces |
|------|------------------|
| `require-test-first.sh` | Test file before impl |
| `require-test-docs.sh` | VALIDATES/PREVENTS comments |
| `require-fuzz-tests.sh` | Fuzz tests for wire parsing |
| `require-boundary-tests.sh` | Boundary tests for numeric validation |

### Design Principles (architecture.md)
| Hook | What it blocks/warns |
|------|----------------------|
| `block-yagni-violations.sh` | "in case we need", "might be useful", etc. |
| `block-and-functions.sh` | `FooAndBar` functions (split into two) |
| `block-init-register.sh` | Auto-registration in `init()` |
| `block-utils-package.sh` | `utils/`, `helpers/`, `common/` packages |
| `block-vague-names.sh` | `Data`, `Info`, `Result`, `Item`, `Thing` names |

### Architecture (no-layering.md, architecture.md, go-standards.md, performance.md)
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
| `block-premature-stop.sh` | Blocks a stop on ownership-dodging, permission-seeking and premature handoff (exit 2). The phrase scan has two tiers. `PHRASES` always blocks. `COMPLETION_PHRASES` (`what next`, `what would you like`) blocks ONLY when a claimed spec is still in-progress, because `.claude/rules/session-start.md` requires that question once the task is done. Nothing is exempt from either tier, and nothing may become exempt by FILTERING the scan's input: an exemption written and removed on 2026-08-08 dropped the whole line carrying `ai/rules/completion.md`'s mandated `New spec: <path>. Implement it?`, so a banned phrase sharing that line ended the turn, and it defeated the unbalanced-backtick and all-markup fallbacks whose purpose is to scan MORE. That sentence needs no exemption: it matches no pattern in either list. Two fixtures pin it (`stop-phrase-mandated-spec-ask-allowed`, `stop-phrase-mandated-ask-does-not-cover-a-second-request`), so a future pattern that swallowed it would go red here rather than in a session. A phrase inside a fenced block or inline backticks is NAMED rather than used, and does not block. Four guards keep that filter scanning MORE rather than less, and guard 4 strips inline spans only on a line whose backticks balance. When the harness sets `stop_hook_active` the hook skips the PHRASE SCAN alone. That bounds a refusal loop, and the other three gates stay armed. It exits 0 on input it cannot parse. Also refuses a stop while a CLAIMED spec is implemented but not closed, and warns (exit 1) when a claimed spec was worked with no subagent. The state checks need a claimed spec, and they need it on every turn rather than only the first. Runs first at `Stop`, and no hook releases the claim: `spec-session.sh release` does, from `/ze-close`, so the marker survives the turn-by-turn `Stop` and every later one. It heartbeats TWO markers on the way past: the claim, and `tmp/session/.agent-spawned-<SID>`. Both calls use `touch -c`, so a missing marker is never created. Neither marker is ever deleted on a timer, so the mtime this heartbeat writes is what dates a live claim against one a dead session left behind. Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation` (36) |
| `session-end-summary.sh` | Append git state summary at session end |
| `compaction-reminder.sh` | Re-read reminder after compaction |
| `verify-claim-reminder.sh` | One line per turn: cite the producing `file:line` before a behavioral claim |
| `delegation-reminder.sh` | One line per turn: subagent delegation is pre-approved, and which skills stay in the main thread |

### Planning (planning.md, post-compaction.md)
| Hook | What it enforces |
|------|------------------|
| `pre-write-go.sh` | Session state for Go work |
| `require-docs-read.sh` | Arch docs before spec |
| `validate-spec.sh` | Spec format + Current Behavior section |

### Config (config.md)
| Hook | What it blocks |
|------|----------------|
| `block-version-config.sh` | Version fields in config |
| `block-silent-ignore.sh` | Must fail on unknown |

### Testing (testing.md, architecture.md)
| Hook | What it enforces |
|------|------------------|
| `check-existing-tests.sh` | Warn duplicate tests |
| `block-throwaway-tests.sh` | No /tmp test files |

### Naming (writing.md)
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

## Session Identity

Every hook that names a per-session marker under `tmp/session/`
(`.lsp-loaded-<sid>`, `.lsp-invoked-<sid>`, `.source-read-<sid>`,
`.source-read-<kind>-<sid>`, `.session-<sid>`), or the session directory that
holds the digest (`tmp/session/<YYYY-MM-DD>-<sid>/state/session-state-<stem>-<sid>.md`),
resolves `<sid>` through **one** resolver:
`.claude/hooks/lib/session_id.py`. It has two faces:

- an importable `session_id()` for the in-process Python callers
  (`pretool-writeedit.py`, `scripts/dev/commit_helper.py`); and
- a `__main__` that prints the id for the Bash hooks, which reach it through the
  one-line shim `.claude/hooks/lib/session-id.sh` (`_session_id`).

There is deliberately no second copy. Three independent derivations (Bash,
Python-hook, `commit_helper`) previously drifted for weeks despite a prose "MUST
stay identical" note, and a disagreement fails **closed**: the reader looks for a
marker nothing wrote and blocks work already done. A shim cannot drift from the
code it calls. See `plan/learned/` (spec `spec-fixit-session-id-collision`).

### Resolution precedence

| # | Source | Notes |
|---|--------|-------|
| 1 | `$CLAUDE_CODE_SESSION_ID` | The session UUID the CLI exports into every child process. Free, canonical, no walk. Forks/subagents inherit the **parent** session's value on purpose, so a fork sees the fail-closed markers its parent wrote. |
| 2 | `--session-id` in the process tree | The CLI's own flag, present only when launched with it. `/proc` on Linux, `ps` on macOS/BSD. |
| 3 | `CLAUDE_CODE_SESSION_ACCESS_TOKEN` JWT `session_id` claim | Empty for subscription auth. |
| 4 | Minted UUID at `tmp/session/.sid-by-pid-<clipid>-<starttime>` | Last resort: a UUID minted once and cached, keyed by the long-lived **CLI-ancestor PID and that process's start time** (never the id itself). Per-session unique **and** stable across the many short-lived hook subprocesses. A reused PID carries a different start time, so a new session never reads a dead one's id. Replaces the old shared constant `claude-session-fallback`, which every concurrent session collided on. |

An id from any source is used only when it is safe as a filename component
(`[A-Za-z0-9._-]`); anything else falls through, so the Bash and Python entry
points cannot disagree on the marker path.

The regression harness is `scripts/dev/hook-fixture-check.py` (section
`session-id`), run by `make ze-unit-hook-test`.

### The session directory

A session owns one directory, `tmp/session/<YYYY-MM-DD>-<sid>/`, holding `bin/`
(its binaries and the `etc/ze` they resolve), `scratch/` (ad-hoc logs and
probes) and `state/` (the per-spec digest). The flat marker files above sit
beside it, not inside it: `.sid-by-pid-<clipid>` mints the id the directory is
named for, and `.closure-ack-<stem>` is keyed by spec stem rather than by
session.

The directory is LOOKED UP, never recomputed: take the directory already
carrying this id, whatever its date, and name a new one with today's date only
on a miss, so a live session's directory does not move at midnight. The shell
definition is `.claude/hooks/lib/session-dir.sh` (`_session_dir`), used by
`lib/state-file.sh` and by `scripts/dev/session-scratch.sh`.
`pretool-writeedit.py` (`session_dir()`), `mk/session.mk` and
`internal/test/sessionpath` implement the same rule for their own callers, and
`TestMakeAndGoAgreeOnBinDir` (`scripts/dev/session_bin_dir_test.py`) is what
stops the copies drifting. Nothing under `tmp/session/` is ever removed
automatically; `make ze-sessions-clean BEFORE=<YYYY-MM-DD>` is the operator's
route.

Its regression harness is `hook-fixture-check.py` section
`session-state-location`.

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
