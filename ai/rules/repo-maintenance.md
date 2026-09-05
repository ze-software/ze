# Repository Maintenance

**When:** adding or changing a feature, tool, gate, or generated file, or editing a canonical source whose generated files need a resync
**Severity:** blocking

## Directives

**A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the SAME work: `ai/INDEX.md` for a keyword or a project fact, `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` for a file or a page, the owning `docs/architecture/` page for a contract, and a registered `ze doctor` check for a new runtime dependency.** A private change owes none of that only when it changes no user or agent behavior, breaks no documented contract or invariant, makes no page stale, and sets no pattern later work follows.

**A new verification gate MUST be registered in `StagesForMode` (`internal/le/verify/engine/stages.go`), and a gate that runs in some modes only MUST be added to each one it belongs in: `fullStages`, `staticcheckStages` and `changedStages` are separate lists.** A gate absent from every list is code that compiles, passes its own unit test, and judges nothing, which is the fail-open shape `ai/rules/evidence.md` bans. Read the three functions and say which lists the gate joins, rather than adding it to the first one you find.

**A hook reminder that MUST land in the model's context writes to stdout; a banner that MUST cost no context tokens writes to stderr.** A `UserPromptSubmit` reminder fires on every turn, so each one MUST stay a single line.

## Canonical Sources and Sync Direction

**A generated file MUST NOT be edited. Edit its canonical source, then run its sync command.**

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `./le ai skills-sync` |
| `ai/skills/*.md` | the per-tool `SKILL.md` mirrors | `./le ai skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le rules render-update` |
| A rule's points or manifest | `TRIGGERS.md`, `CORE.md`, `INDEX.md` | `./le rules render-update`, then `condensed-update` and `index-update` |

**Before editing any file listed in the "Generates" column above, STOP. You MUST find its canonical source in the left column and edit that instead.**

- **Project-wide behavior, workflow and agent rules MUST live under `ai/rules/`, never under a tool-specific home such as `~/.claude/rules/`, so every agent discovers the same rule.** A tool-specific file carries only behavior that applies to that tool outside this repository, and `.claude/rules/*.md` are Claude-specific originals that MUST NOT hold shared Ze behavior.
- **`ai/rules/*.md` are RENDERED from `ai/rules/points/<rule>/` and MUST NOT be edited by hand.** One instruction is one file and its PATH is its id; the manifest carries the title, the trigger, the severity and the reading order. A point on disk that the manifest does not list is a hard render error, never a silent drop.
