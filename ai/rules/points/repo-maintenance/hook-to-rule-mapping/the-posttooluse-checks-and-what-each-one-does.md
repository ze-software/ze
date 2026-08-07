---
kind: table
level:
stage:
---
| Check | File | Enforces | Triggers on | What it does |
|---|---|---|---|---|
| mark-lsp-invoked | `mark-lsp-invoked.sh` | `session-start.md` | LSP | Writes `.lsp-invoked` freshness marker for the design-without-lsp gate. |
| mark-source-read | `mark-source-read.sh` | `evidence.md` | Read | Writes `.source-read` freshness marker when a `.go` under `internal/`/`pkg/`/`cmd/` is read, so reading the producing code satisfies the design-without-lsp gate. Non-blocking. |
| mark-agent-spawned | `mark-agent-spawned.sh` | `planning.md` | Agent, Task | Writes `.agent-spawned-<sid>` so the Stop hook can tell a supervising main thread from one that ran the phase inline. Fires in the PARENT (subagents inherit its session id), so the marker always lands on the supervising session. Non-blocking. |
| auto-lint | `posttool-writeedit.py` | `go-standards.md` | `.go` Write/Edit | `gofmt`/`goimports -w`, then **one** `golangci-lint --new-from-rev=HEAD` pass (flags only issues this edit introduced). BLOCKING on lint failure. |
| auto-py-format | `posttool-writeedit.py` | (code style) | `.py` Write/Edit | `ruff format` + `ruff check`. Non-blocking. |
| validate-spec | `validate-spec.sh` | `planning.md` | `plan/spec-*.md` | Validates required sections/format. Exit 2 blocks a structurally invalid spec; both `→` and `->` wiring rows accepted. |
| file-size | `posttool-writeedit.py` | `go-standards.md` | `.go` | Warns >1000 lines. Advisory. |
| warn-deferral | `posttool-writeedit.py` | `planning.md` | `.md` | Warns on deferral language in doc edits. Advisory. |
| require-rfc-reference | `posttool-writeedit.py` | `go-standards.md` | `.go` | Suggests `// RFC:` header. Advisory. |
| require-test-docs | `posttool-writeedit.py` | `testing.md` | `_test.go` | Warns about missing `VALIDATES:`/`PREVENTS:`. Advisory. |
| require-fuzz-tests | `posttool-writeedit.py` | `testing.md` | wire `.go` | Warns about `Parse*` without `Fuzz*` tests. Advisory. |
| vague-names | `posttool-writeedit.py` | `architecture.md` | `.go` | Warns about `Data`/`Info`/`Result`/... names. Advisory. |
| boundary-tests | `posttool-writeedit.py` | `testing.md` | `.go` | Warns about numeric validation without boundary tests. Advisory. |
