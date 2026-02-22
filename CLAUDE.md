# Ze - Claude Instructions

## What Ze Is

Ze is a **BGP daemon** written in Go. "Ze" = "The" with a French accent (predecessor: ExaBGP).

**Engine + Plugin architecture:** The engine handles BGP protocol (TCP, FSM, OPEN/UPDATE/NOTIFICATION parsing, capabilities negotiation). Plugins handle policy decisions (RIB storage, route reflection, graceful restart). Engine and plugins communicate over Unix socket pairs using JSON events (engine→plugin) and text commands (plugin→engine).

```
Engine: Peers (FSM) → Reactor (event loop, BGP cache)
   ║ JSON + base64 wire bytes (down) / text commands (up)
Plugin: bgp-rib, bgp-rr, bgp-gr, bgp-role, bgp-nlri-*, ...
```

**Key abstractions:**
- `WireUpdate` — lazy-parsed BGP UPDATE (zero-copy iterators over wire bytes)
- `PackContext` — negotiated capabilities (ASN4, ADD-PATH, ExtNH) that determine encoding
- `ContextID` — if source and dest peers share ContextID, forward wire bytes unchanged
- Pool-based dedup — per-attribute-type pools (ORIGIN, AS_PATH, etc.) with refcounted handles
- Buffer-first encoding — all wire writing uses `WriteTo(buf, off) int` into pooled buffers

**Config:** YANG-modeled (`ze-bgp-conf.yang`), parsed via `internal/config/`. File → Tree → ResolveBGPTree() → `map[string]any` → `reactor.PeersFromTree()`. ExaBGP configs auto-detected and migrated.

**Plugin registry:** `internal/plugin/registry/` — plugins register via `init()` in `register.go`. Engine discovers them through registry, never imports plugin packages directly.

## Key Paths

| Area | Location |
|------|----------|
| Engine core | `internal/plugins/bgp/` (reactor, FSM, wire, message, capability) |
| Plugin impls | `internal/plugins/bgp-rib/`, `bgp-rr/`, `bgp-gr/`, `bgp-nlri-*/ ` |
| Plugin infra | `internal/plugin/` (registry, process, hub, SDK) |
| Config | `internal/config/`, YANG schemas in `internal/plugins/bgp/schema/` |
| CLI | `cmd/ze/` (subcommands: bgp, validate, etc.) |
| IPC/Hub | `internal/hub/`, `internal/ipc/` |
| Tests | `test/` (.ci functional tests), `*_test.go` (unit) |
| Architecture docs | `docs/architecture/` (→ see `.claude/INDEX.md` for navigation) |
| Specs | `docs/plan/spec-*.md` (active), `docs/plan/done/` (completed) |
| RFCs | `rfc/short/` (summaries), `rfc/full/` |

## Navigation

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
