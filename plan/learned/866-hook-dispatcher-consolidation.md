# 866 -- hook-dispatcher-consolidation

## Context

The Claude Code hook system had grown to 69 shell hooks: up to 11 PreToolUse hooks on every Bash call and 40 on every Write/Edit, each spawning `bash` + several `jq` processes. A PostToolUse agent-review hook spawned a Sonnet subagent (up to 120s) after every `make ze-verify`, which was the real source of multi-minute per-command latency on a 352M / 360-file working tree. Read-only tools paid a hook spawn on every call.

## Decisions

- Consolidated 58 per-check shell hooks into 3 Python dispatchers (`pretool-bash.py`, `pretool-writeedit.py`, `posttool-writeedit.py`): one process per tool call instead of dozens, each check a function gated exactly as its original `.sh`.
- Validated every port by exit-code parity against the original `.sh` (PreToolUse: 3280 per-hook isolated comparisons + aggregate; PostToolUse: 22), THEN converted the regression test to self-contained golden values (run in a fresh temp project dir) so the originals could be deleted without losing the test.
- Removed the PostToolUse agent-review hook -- the actual source of the "minutes"; `/code-review` covers it on demand.
- Narrowed `block-until-lsp` from `.*` to mutating/executing tools + `ToolSearch`, so reads incur zero hooks.
- Trimmed `auto_linter` to a single `golangci-lint --new-from-rev=HEAD` pass (only issues the current edit introduced, not pre-existing ones in untouched code).
- `-S` (skip-site) on the dispatcher shebangs via `#!/usr/bin/env -S python3 -S`: ~20ms faster startup each (42%).

## Consequences

- PreToolUse 43->4 entries, PostToolUse 11->5; reads hook-free; per-edit lint roughly halved.
- Behaviour is preserved exactly (parity-proven), so the ~100 rules and learned summaries that name a check by behaviour stay accurate without edits.
- To change a check: edit the dispatcher function, run `python3 scripts/dev/hook-parity-check.py`, then `--bless` only if behaviour intentionally changed.

## Gotchas

- `block-format-alloc.sh` was silently DEAD in production: it used bash-4 `declare -A`, but the `#!/bin/bash` shebang resolves to macOS bash 3.2.57, which errors on the associative-array assignment ("invalid arithmetic operator") and falls through to `exit 0`. The format-file append-idiom guard never fired once. The port preserves the no-op; the real check sits disabled in `pretool-writeedit.py`, ready to enable.
- PostToolUse checks read the post-edit file from DISK (not the payload `content`/`new_string`), so their parity/golden tests must create on-disk fixtures in the temp project dir.
- Aggregate parity can mask an individual broken check when a dominant hook (`pre-write-go`, `require-design-ref`) also fires on the same file; per-hook ISOLATED comparison is required to catch a silently-no-op'd port.

## Files

- `.claude/hooks/pretool-bash.py`, `pretool-writeedit.py`, `posttool-writeedit.py` (created) -- the 3 dispatchers
- `.claude/hooks/auto_linter.sh` (modified) -- single golangci pass
- `.claude/settings.json` (modified) -- 58 hook entries -> 3 dispatchers, narrowed LSP gate, agent hook removed
- `scripts/dev/hook-parity-check.py` (created) -- self-contained golden regression test (`--bless` to regenerate)
- `ai/rules/repo-maintenance.md` (modified) -- rewritten for the dispatcher architecture
- `ai/rules/git-safety.md` (modified) -- fixed a deleted-hook example reference
- 56 `.claude/hooks/*.sh` (removed) -- consolidated into the dispatchers
