# Contributing

Guidance for working on Ze itself: how to test, how to keep documentation
accurate, and how to implement RFC behaviour.

Ze development is expected on macOS or Linux. Windows is not a supported development platform.

| Document | Purpose |
|----------|---------|
| `writing-style.md` | Rule one: how every word in the repository is written. The six habits to avoid, the sentence limits, and how to check your own prose |
| `ze-go-style.md` | Rule one for code: how every line of Go in the repository is written. The reasoning behind the Go rules, adapted from TigerStyle |
| `go-conventions.md` | The Ze-specific Go reference: the package-naming glossary, file headers and cross-references, the `internal/core/env` accessors, typed-numeric-over-string, and API contract comments |
| `ze-python-style.md` | Rules for references to external Python programs. First-party commands, hooks, test drivers, and generators are Go |
| `rule-authoring.md` | How to change an agent rule: the point files behind `ai/rules/`, the manifest, the generators, and how a hook check binds to one instruction |
| `spec-workflow.md` | The formats around a spec: its status vocabulary, the deferral shard, the closure gates and what each one reads, the executive summary, and the session handoff |
| `testing.md` | How the test suites are organised and run (unit, functional, fuzz, race) |
| `committing.md` | How `./le commit create` works: its keywords, what it refuses, what the generated script contains, and what to do when a commit or a push fails |
| `running-commands.md` | How the `./le` action surface behaves: feature build tags, the changed-set selector, session scratch and binaries, verify logs, the job registry, the Bash guard |
| `documentation-testing.md` | The `./le doc check verify` drift and contract checks, and how to fix failures |
| `ci-test-coverage.md` | Functional `.ci`/`.et` coverage map and known gaps |
| `navigating-the-code.md` | How to answer a question without reading a whole file: the `gopls` routes for a Go symbol, and which generated index answers which question |
| `claude-code-cheatsheet.md` | Quick reference for the AI-assisted workflow and skills |
| `gh-pages.md` | Publishing the documentation site |
| `rfc-implementation-guide.md` | How to implement an RFC: reading, summarising, and compliance evidence |
| `rfc-conformance-gates.md` | What `./le rfc check` measures: the artifacts, the eight ratchets, the public ledger guards, the superseded marker, and the extraction sign-off |

For architecture-level testing design (CI format, interop, QEMU), see
`../architecture/testing/`.
