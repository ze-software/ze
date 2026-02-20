# Ze - Claude Instructions

Rules: `.claude/rules/` (auto-loaded). Rationale: `.claude/rationale/` (on-demand).
Architecture + RFC navigation: `.claude/INDEX.md`.
Spec template: `docs/plan/TEMPLATE.md`.

## Rules

| Category | Rules |
|----------|-------|
| **Session** | `session-start.md`, `post-compaction.md`, `before-writing-code.md` |
| **Code Quality** | `tdd.md`, `go-standards.md`, `quality.md`, `design-principles.md`, `anti-rationalization.md`, `goroutine-lifecycle.md` |
| **BGP Protocol** | `rfc-compliance.md`, `buffer-first.md`, `json-format.md`, `architecture-summary.md` |
| **Planning** | `planning.md`, `spec-no-code.md`, `spec-preservation.md`, `implementation-audit.md`, `integration-completeness.md`, `data-flow-tracing.md` |
| **Infrastructure** | `plugin-design.md`, `cli-patterns.md`, `config-design.md`, `naming.md` |
| **Process** | `git-safety.md`, `no-layering.md`, `compatibility.md`, `no-test-deletion.md`, `testing.md`, `documentation.md`, `design-doc-references.md`, `hook-errors.md`, `memory.md` |

## Commands

```bash
make ze-unit-test          # Unit tests with race detector
make ze-functional-test    # Functional tests
make ze-lint               # 26 linters
make ze-verify             # lint + unit + functional
make ze-ci                 # lint + unit + build
make ze-fuzz-test          # Fuzz tests (10s per target)
make ze-exabgp-test        # ExaBGP compatibility
make ze-test               # All ze tests
make chaos-test            # Chaos unit + functional
make test-all              # lint + all ze tests
```
