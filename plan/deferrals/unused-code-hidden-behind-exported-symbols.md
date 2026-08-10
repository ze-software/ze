# Deferrals: unused-code-hidden-behind-exported-symbols

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-10 | `plan/spec-fixit-unexport-package-private-symbols.md`, bucket 3 (phase agent) | 18 `golangci-lint` `unused`/`unparam` findings that appear only after their symbol is unexported, because both linters skip an exported top-level declaration | The rename work does not depend on these declarations being cleaned up: every package still compiles and tests green with the finding left in place. Deleting or trimming a signature is a separate change outside the rename-only scope of the parent spec (`ai/rules/completion.md`) | `plan/spec-unused-code-hidden-behind-exported-symbols.md` | open |
