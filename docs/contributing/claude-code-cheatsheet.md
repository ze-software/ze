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
| `/ze-audit` | Pre-implementation: what code already exists for this spec? |
| `/ze-implement` | Implement a spec end-to-end (TDD, review loops, docs) |
| `/./le verify current mode full` | Run `./le verify current mode full` and report results |
| `/ze-debug` | Investigate failing tests with parallel hypotheses |
| `/ze-review` | Quick single-pass code review of uncommitted changes |
| `/ze-review-deep` | Multi-agent exhaustive review (9 specialized agents) |
| `/ze-review-spec` | Post-implementation: did we build what the spec says? |
| `/ze-review-docs` | Documentation accuracy, completeness, and quality |
| `/ze-commit` | Prepare a scoped commit script through `internal/le/commit.Answer` (does not commit directly) |
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
/./le verify current mode full                 -- run full test suite
/ze-review-deep            -- multi-agent review before submission
/ze-commit                 -- prepare the commit
```

### Fixing a bug

```
/ze-debug                  -- paste test output, get parallel investigation
/./le verify current mode full                 -- confirm fix doesn't break anything
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

## Notes

- `/ze-commit` does **not** run `git commit`. It generates a message file and executable commit script through `internal/le/commit.Answer`; you run the script yourself.
<!-- source: internal/le/commit/actions.go -- Answer -->
- `/./le verify current mode full` is the same as `./le verify current mode full` but formats the output as a structured report.
- `/ze-review-deep` accepts arguments: path scope, agent names, or `branch` for branch review.
- `/ze-hunt` differs from the review family: the reviews look at the **diff**, `/ze-hunt` sweeps the **existing tree** for the recurring bug classes recorded in `plan/learned/RECURRING-PATTERNS.md`. Use it for periodic latent-bug sweeps, not for checking a change.
- All commands are read-only unless explicitly stated. Reviews report findings without making changes.
