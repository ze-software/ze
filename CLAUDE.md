# Ze - Claude Instructions

All rules live in `.claude/rules/` and are auto-loaded. This file explains what each rule does and why it exists.

For architecture docs and RFC navigation, see `.claude/INDEX.md`.

## Session Discipline

| Rule | File | Why It Exists |
|------|------|---------------|
| **Session Start** | `session-start.md` | The TOP 6 rules that prevent the most expensive mistakes: reading specs first, understanding before coding, TDD, preserving behavior |
| **Post-Compaction** | `post-compaction.md` | After context compaction you lose memory of what you read. Forces re-reading source files and specs before resuming |
| **Before Writing Code** | `before-writing-code.md` | Prevents writing code that duplicates existing patterns or conflicts with architecture |

## Code Quality

| Rule | File | Why It Exists |
|------|------|---------------|
| **TDD** | `tdd.md` | Tests must fail before implementation — proves they actually validate something |
| **Go Standards** | `go-standards.md` | slog logging, error wrapping, fail-early — the coding baseline |
| **Quality** | `quality.md` | Never disable linters, never skip reviews, paste proof of completion |
| **Design Principles** | `design-principles.md` | YAGNI, no premature abstraction, single responsibility, 3-fix escalation rule |
| **Anti-Rationalization** | `anti-rationalization.md` | Pre-addresses known excuses for skipping TDD, ignoring test failures, or claiming completion without evidence |

## BGP Protocol

| Rule | File | Why It Exists |
|------|------|---------------|
| **RFC Compliance** | `rfc-compliance.md` | Ze must be a fully RFC 4271 compliant BGP speaker. All protocol code needs RFC citations with quoted MUST requirements |
| **Buffer-First** | `buffer-first.md` | Wire encoding must use `WriteTo(buf, off)`, never allocate and return — prevents GC pressure on hot paths |
| **JSON Format** | `json-format.md` | All JSON output uses kebab-case, follows ze-bgp IPC protocol envelope |
| **Architecture Summary** | `architecture-summary.md` | Condensed system overview always in context: Engine/Plugin boundary, negotiated capabilities, WireUpdate vs RIB |

## Planning & Specs

| Rule | File | Why It Exists |
|------|------|---------------|
| **Planning** | `planning.md` | Full planning workflow: research → design → implement → verify. Keyword→doc mapping, spec template, completion checklist |
| **Spec No Code** | `spec-no-code.md` | Specs describe WHAT/WHY in tables and prose. Code in specs becomes stale and misleading |
| **Spec Preservation** | `spec-preservation.md` | Completed specs are institutional memory — strip scaffolding, keep knowledge |
| **Implementation Audit** | `implementation-audit.md` | Line-by-line verification that every spec item was implemented. Tests passing ≠ spec complete |
| **Integration Completeness** | `integration-completeness.md` | Features must be proven integrated end-to-end, not just tested in isolation |
| **Data Flow Tracing** | `data-flow-tracing.md` | Trace data through all layers before modifying specs — prevents architectural boundary violations |

## Infrastructure

| Rule | File | Why It Exists |
|------|------|---------------|
| **Plugin Design** | `plugin-design.md` | Plugin architecture: registry, 5-stage protocol, SDK callbacks, registration via `init()` |
| **CLI Patterns** | `cli-patterns.md` | Consistent CLI dispatch, flag handling, exit codes, usage text |
| **Config Design** | `config-design.md` | No version numbers in config, fail on unknown keys |
| **Naming** | `naming.md` | Ze naming convention: `ze-bgp-conf`, `ZeBGPConf*`, "ze" where "the" works |

## Process

| Rule | File | Why It Exists |
|------|------|---------------|
| **Git Safety** | `git-safety.md` | Only commit when asked, scope commits to task, never force-push, save before destructive actions. Includes Codeberg CLI (`tea`) |
| **No Layering** | `no-layering.md` | When replacing X with Y, delete X first. No "keep both for safety" |
| **No Backwards Compat** | `compatibility.md` | Ze has never been released — no users to break, no legacy shims needed |
| **No Test Deletion** | `no-test-deletion.md` | Ask before deleting tests — prevents hiding bugs instead of fixing them |
| **Testing** | `testing.md` | No throw-away tests, functional test locations, make targets, linter list |
| **Documentation** | `documentation.md` | File naming, doc placement, single source of truth — never duplicate content across files |
| **Hook Errors** | `hook-errors.md` | Hook validation errors must be fixed before proceeding — never ignore exit codes |
| **Memory** | `memory.md` | Project knowledge: YANG conventions, flaky test policy, config pipeline, file splits |

## Reference

- **Architecture docs:** `.claude/INDEX.md` for full doc→topic and RFC→keyword mappings
- **RFC summaries:** `rfc/short/` (implementation-ready markdown)
- **Full RFCs:** `rfc/full/` (text files)
- **ExaBGP reference:** `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`

## Commands

```bash
make unit-test         # Unit tests with race detector
make lint              # 26 linters via golangci-lint
make functional-test   # Functional tests (encode, plugin, decode, parse, reload, editor)
make exabgp-test       # ExaBGP compatibility tests
make fuzz-test         # Fuzz tests (10s per target)
make chaos-test        # In-process BGP chaos simulation
make verify            # lint + unit-test + functional-test
make test-all          # lint + unit-test + functional-test + exabgp-test
```
