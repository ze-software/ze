# Claude Code Cheat Sheet

Ze uses [Claude Code](https://claude.ai/code) for development. These slash commands
are available inside Claude Code when working in the ze repository.

## Quick Reference

| Command | What it does |
|---------|-------------|
| `/ze-status` | What needs my attention? Dashboard with suggested next actions |
| `/ze-explore <topic>` | Find and read all files related to a topic |
| `/ze-spec` | Create or resume a spec (design document for a task) |
| `/ze-design` | Stress-test a design through structured decisions |
| `/ze-disposition [stem\|stale]` | Before picking up an old spec: does it still belong in `main`? One verdict per spec, recorded in it |
| `/ze-audit` | Pre-implementation: what code already exists for this spec? |
| `/ze-implement` | Implement a spec end-to-end (TDD, review loops, docs) |
| `/ze-verify` | Main thread only: run the scoped tests for what changed, in the main checkout, and report results; it never runs any `./le verify` command |
| `/ze-debug` | Investigate failing tests with parallel hypotheses |
| `/ze-review` | Quick single-pass code review of uncommitted changes |
| `/ze-review-deep` | Multi-agent exhaustive review (9 specialized agents) |
| `/ze-review-spec` | Post-implementation: did we build what the spec says? |
| `/ze-review-docs` | Documentation accuracy, completeness, and quality |
| `/ze-commit` | Commit the named changes through a scoped commit script from `internal/le/commit.Answer`, run at once; no verification (`/ze-commit-check` adds it) |
| `/ze-debrief` | Summarize current session state |
| `/ze-handoff` | Generate a handoff document for the next session |
| `/ze-rfc` | Generate an implementation summary from an RFC |
| `/ze-find-alloc` | Scan encoding paths for allocations that should use buffers |
| `/ze-fix-alloc <file:line>` | Convert a specific allocation to buffer-writing |
| `/ze-extract <src> <dst> <symbols>` | Move Go symbols between files |
| `/ze-hunt [path]` | Sweep the tree for recorded bug classes (silent fall-through, seqnum ordering, unenforced lock contracts, count-asserts) and triage each hit |
| `/ze-weekly-update` | Draft the Zeledon weekly update, update gh-pages, and post the approved `ze-news` message |


<!-- source: ai/skills/ze-commit.md -- scoped commit workflow -->
<!-- source: internal/le/commit/actions.go -- Answer -->
<!-- source: ai/skills/ze-weekly-update.md -- weekly update drafting, site update, and Discord post workflow -->
## Typical Workflows

### Contributing a new feature

```
/ze-explore <area>         -- understand what exists
/ze-spec                   -- write the spec (design, ACs, test plan)
/ze-audit                  -- check what's already implemented
/ze-implement              -- TDD implementation with built-in reviews
/ze-verify                 -- run the scoped tests the change reaches
/ze-review-deep            -- multi-agent review before submission
/ze-commit                 -- prepare the commit
```

### Fixing a bug

```
/ze-debug                  -- paste test output, get parallel investigation
/ze-verify                 -- confirm the fix with the tests it reaches
/ze-review                 -- quick review of the fix
/ze-commit                 -- prepare the commit
```

### Publishing the weekly update

```
/ze-weekly-update          -- draft from shipped work, update the website, then post after approval
```

The command owns the Discord approval gate, the Zeledon voice rules, the gh-pages regeneration path, and the homepage/feed checks.
<!-- source: ai/skills/ze-weekly-update.md -- hard gates, publish phase, and site update phase -->

### Reviewing changes

```
/ze-review                 -- quick single-pass (~2 min)
/ze-review-deep            -- exhaustive multi-agent review
/ze-review-deep branch     -- review current branch vs main
/ze-review-spec            -- check implementation matches spec
/ze-review-docs            -- check docs match code
```

### Session management

```
/ze-status                 -- what needs attention across everything?
/ze-debrief                -- where am I in the current task?
/ze-handoff                -- prepare state for a new session
```

## Review Family

The four review commands serve different purposes:

| Command | Scope | Depth | When to use |
|---------|-------|-------|-------------|
| `/ze-review` | Uncommitted changes | Single pass, ~2 min | Quick sanity check |
| `/ze-review-deep` | Uncommitted or branch | 9 parallel agents | Before merge or commit of significant work |
| `/ze-review-spec` | Selected spec | Spec vs code comparison | After implementing a spec |
| `/ze-review-docs` | `docs/` directory | Accuracy and completeness | Periodic documentation audit |

## Where a Skill Lives

Each command above is a skill, and its text is one file: `ai/skills/<name>.md`.
Edit that file. A change to its body takes effect the next time the skill is
loaded, in every tool, with nothing to run.

Claude Code, Codex, OpenCode and Pi each look for skills in their own directory
(`.claude/skills/`, `.codex/skills/`, `.agents/skills/`). Those directories are
generated and git-ignored. Each holds one stub per skill: the skill's name and
description, and an instruction to read `ai/skills/<name>.md` in full. Never
edit a stub.

What each tool was seen to do with a stub, on 2026-11-13:

| Tool | Reads its stubs from | Follows the stub to the source |
|------|----------------------|--------------------------------|
| Claude Code | `.claude/skills/` | Yes: the next action after the stub was a full read of `ai/skills/<name>.md`, in the main session and in two fresh agents |
| Codex CLI 0.169.0 | `.agents/skills/` | Yes: it read the stub, then the whole source, and bound the argument it was given |
| OpenCode 1.19.4 | `.agents/skills/` and `.claude/skills/`, each skill listed once | Not run: it discovered all 34 stubs, and no model was available to it that day |
| Pi 0.73.1 | `.agents/skills/` (by its source) | Not run: no model was available to it that day |

| You did | Run |
|---------|-----|
| Edited a skill's body | Nothing |
| Added, removed or renamed a skill, or changed its `name` or `description` | `./le ai sync update` |
| Cloned the repository, or saw "generated agent files are stale" at session start | `./le ai sync update` |

The sync refuses a skill file whose frontmatter lacks a `description`, or a
`name` equal to its file name, and says which line to add or edit. It removes
the stub of a skill that is gone. Any other file it finds in those directories
it leaves in place and names, with the command that deletes it if it is a
leftover. `./le ai sync check` names what is stale and writes nothing.
<!-- source: internal/le/ai/sync/ai.go -- stubOf, clearLeftovers, Check -->
<!-- source: internal/le/ai/sync/report.go -- Report.Text -->

## Notes

- `/ze-commit` does **not** run `git commit` or `git add` as direct calls. It generates a message file and executable commit script through `internal/le/commit.Answer` and runs that script, which is what commits.
<!-- source: internal/le/commit/actions.go -- Answer -->
- `/ze-verify` runs in the main thread, which schedules every test on the shared machine: no subagent runs it. It runs the scoped tests for what changed and formats the output as a structured report. It never runs any `./le verify` command: GitHub CI verifies a pushed commit, and `./le verify current` is Thomas's release check.
- `/ze-review-deep` accepts arguments: path scope, agent names, or `branch` for branch review.
- `/ze-hunt` differs from the review family: the reviews look at the **diff**, `/ze-hunt` sweeps the **existing tree** for the recurring bug classes recorded in `plan/learned/RECURRING-PATTERNS.md`. Use it for periodic latent-bug sweeps, not for checking a change.
- All commands are read-only unless explicitly stated. Reviews report findings without making changes.
