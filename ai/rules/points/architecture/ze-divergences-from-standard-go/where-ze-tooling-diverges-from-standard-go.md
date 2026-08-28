---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Ad-hoc scripts for tooling | Native Go package with a registered `./le` action | `ai/rules/go-standards.md` | One typed implementation serves local and CI callers |
| `/tmp` for scratch files | Per-session directory from `./le session scratch ensure` | `ai/rules/commands.md` | Concurrent sessions do not share names |
| `git add -A && git commit` | `./le commit create`, then the generated script | `ai/rules/git-safety.md` | The declared file population is checked before staging |
