# Ze - Claude Instructions

## What Ze Is

Ze is a **BGP daemon** written in Go. "Ze" = "The" with a French accent (predecessor: ExaBGP).

**BGP Subsystem + Plugin architecture:** The BGP Subsystem handles protocol (TCP, FSM, OPEN/UPDATE/NOTIFICATION parsing, capabilities negotiation). Plugin Infrastructure manages plugin lifecycle and message routing. Plugins handle policy decisions (RIB storage, route reflection, graceful restart). Communication over Unix socket pairs using JSON events (engine→plugin) and text commands (plugin→engine).

```
BGP Subsystem (internal/plugins/bgp/):
  Peers (FSM) → Wire Layer → Reactor (event loop, BGP cache) → EventDispatcher
   ║ formatted events (down) / commands (up)
Plugin Infrastructure (internal/plugin/):
  Registry · Process Manager · Hub · SDK · DirectBridge
   ║ JSON events + base64 wire bytes (down) / text commands (up)
Plugins: bgp-rib, bgp-rs, bgp-gr, bgp-role, bgp-nlri-*, ...
```

**Key abstractions:**
- `WireUpdate` — lazy-parsed BGP UPDATE (zero-copy iterators over wire bytes)
- `PackContext` — negotiated capabilities (ASN4, ADD-PATH, ExtNH) that determine encoding
- `ContextID` — if source and dest peers share ContextID, forward wire bytes unchanged
- Pool-based dedup — per-attribute-type pools (ORIGIN, AS_PATH, etc.) with refcounted handles
- Buffer-first encoding — all wire writing uses `WriteTo(buf, off) int` into pooled buffers

**Config:** YANG-modeled (`ze-bgp-conf.yang`), parsed via `internal/component/config/`. File → Tree → ResolveBGPTree() → `map[string]any` → `reactor.PeersFromTree()`. ExaBGP configs auto-detected and migrated.

**Plugin registry:** `internal/plugin/registry/` — plugins register via `init()` in `register.go`. Engine discovers them through registry, never imports plugin packages directly.

## Key Paths

| Area | Location |
|------|----------|
| Engine core | `internal/plugins/bgp/` (reactor, FSM, wire, message, capability) |
| Plugin impls | `internal/plugins/bgp-rib/`, `bgp-rs/`, `bgp-gr/`, `bgp-nlri-*/ ` |
| Plugin infra | `internal/plugin/` (registry, process, hub, SDK) |
| Config | `internal/component/config/`, YANG schemas in `internal/component/bgp/schema/` |
| CLI | `cmd/ze/` (subcommands: bgp, validate, etc.) |
| IPC/Hub | `internal/hub/`, `internal/ipc/` |
| Tests | `test/` (.ci functional tests), `*_test.go` (unit) |
| Architecture docs | `docs/architecture/` (→ see `.claude/INDEX.md` for navigation) |
| Specs | `plan/spec-*.md` (active), `plan/learned/` (summaries) |
| RFCs | `rfc/short/` (summaries), `rfc/full/` |

## Navigation

Rules: `.claude/rules/` (auto-loaded). Rationale: `.claude/rationale/` (on-demand).
Architecture + RFC navigation: `.claude/INDEX.md`.
Spec template: `plan/TEMPLATE.md`.

## Rules

| Category | Rules |
|----------|-------|
| **Session** | `session-start.md`, `post-compaction.md`, `before-writing-code.md` |
| **Code Quality** | `tdd.md`, `go-standards.md`, `quality.md`, `design-principles.md`, `anti-rationalization.md`, `goroutine-lifecycle.md`, `file-modularity.md`, `api-contracts.md` |
| **BGP Protocol** | `rfc-compliance.md`, `buffer-first.md`, `json-format.md`, `architecture-summary.md` |
| **Planning** | `planning.md`, `spec-no-code.md`, `spec-preservation.md`, `implementation-audit.md`, `integration-completeness.md`, `data-flow-tracing.md` |
| **Infrastructure** | `plugin-design.md`, `cli-patterns.md`, `config-design.md`, `naming.md` |
| **Process** | `git-safety.md`, `no-layering.md`, `compatibility.md`, `no-test-deletion.md`, `testing.md`, `documentation.md`, `design-doc-references.md`, `related-refs.md`, `hook-errors.md`, `memory.md`, `friction-reporting.md` |

## Commands

```bash
make ze-unit-test          # Unit tests with race detector
make ze-functional-test    # Functional tests
make ze-lint               # 26 linters
make ze-verify             # All tests except fuzz (development)
make ze-ci                 # lint + unit + build
make ze-fuzz-test          # Fuzz tests (15s per target)
make ze-exabgp-test        # ExaBGP compatibility
make ze-chaos-test         # Chaos unit + functional
make ze-test               # All tests: lint + unit + functional + exabgp + chaos + fuzz (before commits)
make ze-inventory          # Project inventory: plugins, YANG, RPCs, families, tests, packages
make ze-inventory-json     # Same as above, machine-readable JSON
make ze-spec-status        # Spec progress table
```
