---
kind: table
level:
stage:
---
| The failing path | Whose red | What you do |
|------------------|-----------|-------------|
| In this commit's `--file` list | Yours | Fix it. A red you caused is never attributed away |
| Dirty in `git status --porcelain`, and not in your list | Another session's | Take that code as working. Name it in `--unverified` and commit |
| Clean and tracked, and your diff PRODUCES a symbol the failure names | Yours | Fix it. Ownership follows the producer, not the file that failed |
| Clean and tracked, and unrelated to your diff | Pre-existing | Attribute it against `git log`, name it in `--unverified`, and commit |
| Any deterministic structural gate | Yours until you prove otherwise | Fix it. Those read files rather than a moving tree. The helper drops the charge only when every file the failure groups name lies outside your commit |
