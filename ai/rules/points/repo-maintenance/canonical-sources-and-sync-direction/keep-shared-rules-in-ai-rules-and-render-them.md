---
kind: directive
level: MUST
stage:
---
- **Project-wide behavior, workflow and agent rules MUST live under `ai/rules/`, never under a tool-specific home such as `~/.claude/rules/`, so every agent discovers the same rule.** A tool-specific file carries only behavior that applies to that tool outside this repository, and `.claude/rules/*.md` are Claude-specific originals that MUST NOT hold shared Ze behavior.
- **`ai/rules/*.md` are RENDERED from `ai/rules/points/<rule>/` and MUST NOT be edited by hand.** One instruction is one file and its PATH is its id; the manifest carries the title, the trigger, the severity and the reading order. A point on disk that the manifest does not list is a hard render error, never a silent drop.
