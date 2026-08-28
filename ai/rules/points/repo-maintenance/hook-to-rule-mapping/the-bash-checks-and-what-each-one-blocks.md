---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `bashWorktreeCopy` | `ai/INSTRUCTIONS.md` prohibition, no rule point | Bash | Blocks copies, moves, installs, rsync, and redirects from `.claude/worktrees`. BLOCKING. |
| `bashDestructiveGit` | `git-safety.md` | Bash | Blocks destructive and repository-changing git verbs. `git restore --staged` remains permitted. BLOCKING. |
| `bashRootBuild` | build hygiene, no rule point | Bash | Blocks `go build` that would place a binary at the repository root, while explicit session or `bin/` outputs pass. BLOCKING. |
| `bashLossyPipe` | `commands.md` | Bash | Blocks a lossy filter after an expensive command and directs the output to a log. BLOCKING. |
| `bashRawHeavy` | `commands.md` | Bash | Blocks raw Go tests, lint analysis, and functional runners outside `./le job run`. BLOCKING. |
| `bashPollLoop` | `commands.md` | Bash | Blocks an unbounded `while` or `until` polling loop with sleep or pgrep. BLOCKING. |
| `bashSystemTmp` | `testing.md` | Bash | Blocks access to `/tmp` and names the session scratch action. BLOCKING. |
| `bashScratch` | `commands.md` | Bash | Blocks ad-hoc writes at the project `tmp/` root while permitting owned subdirectories and governed shared names. BLOCKING. |
| `bashTestDeletion` | `testing.md` | Bash | Blocks deletion or checkout of test files outside `test/draft/`. BLOCKING. |
| `bashGovernedWrite` | `commands.md` | Bash | Blocks shell writes under `plan/` or `ai/rules/`, where the native Write/Edit checks own the policy. BLOCKING. |
