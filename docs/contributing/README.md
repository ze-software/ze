# Contributing

Guidance for working on Ze itself: how to test, how to keep documentation
accurate, and how to implement RFC behaviour.

Ze development is expected on macOS or Linux. Windows is not a supported development platform.

| Document | Purpose |
|----------|---------|
| `writing-style.md` | Rule one: how every word in the repository is written. The six habits to avoid, the sentence limits, and how to check your own prose |
| `rule-authoring.md` | How to change an agent rule: the point files behind `ai/rules/`, the manifest, the generators, and how a hook check binds to one instruction |
| `testing.md` | How the test suites are organised and run (unit, functional, fuzz, race) |
| `documentation-testing.md` | The `make ze-doc-verify` drift and contract checks, and how to fix failures |
| `ci-test-coverage.md` | Functional `.ci`/`.et` coverage map and known gaps |
| `claude-code-cheatsheet.md` | Quick reference for the AI-assisted workflow and skills |
| `gh-pages.md` | Publishing the documentation site |
| `rfc-implementation-guide.md` | How to implement an RFC: reading, summarising, and compliance evidence |

For architecture-level testing design (CI format, interop, QEMU), see
`../architecture/testing/`.
