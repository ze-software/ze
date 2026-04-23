# Learned Summaries Index

Curated index of `plan/learned/` summaries that capture structural decisions, patterns, and gotchas.
Task-completion-only summaries (the majority) are omitted. Full list: `ls plan/learned/`.

Three meta-summaries sit alongside the numbered per-spec summaries. Read the one that
matches your question first — each one points at the specific numbered summaries that
hold the full record, so one file of reading replaces hundreds.

| Question | File |
|----------|------|
| "Why is the code as it is?" | `plan/learned/DESIGN-HISTORY.md` — design evolution by subsystem, abandoned approaches, load-bearing invariants |
| "Am I about to fall into a known trap?" | `plan/learned/RECURRING-PATTERNS.md` — patterns that recurred 5+ times, with avoid-it-by and recover-if-you-hit-it |
| "Why did this hook reject my code?" | `plan/learned/HOOK-FRICTION.md` — every hook false positive with verified workaround |

## Core Architecture

System boundaries, component design, lifecycle patterns, subsystem separation.

- [001](plan/learned/001-initial-implentation.md) -- Zero-copy ContextID, per-type pools, lazy iterators, goroutine-per-peer
- [013](plan/learned/013-unified-commit-system.md) -- CommitService abstraction, grouping by config not command, implicit EOR
- [015](plan/learned/015-fsm-active-design.md) -- FSM active connect state machine design
- [133](plan/learned/133-internal-migration.md) -- internal/ package restructuring, reactor -> component/bgp
- [149](plan/learned/149-unified-subsystem-protocol.md) -- Unified subsystem protocol, in-process vs async
- [157](plan/learned/157-hub-separation-phases.md) -- Hub separation into 7 phases
- [165](plan/learned/165-reactor-service-separation.md) -- Reactor service separation from protocol
- [244](plan/learned/244-reactor-interface-split.md) -- Reactor interface split for testability
- [247](plan/learned/247-plugin-restructure.md) -- Plugin restructure, circular import resolution

## Wire/Encoding

Buffer-first, zero-copy, attribute pools, UPDATE building, NLRI parsing.

- [059](plan/learned/059-spec-pool-handle-migration.md) -- Pool handle migration from mutex stores
- [073](plan/learned/073-spec-buffer-writer.md) -- BufWriter WriteTo(buf, off) int pattern
- [076](plan/learned/076-spec-wire-update.md) -- WireUpdate lazy parsing design
- [092](plan/learned/092-pack-to-writeto-migration.md) -- Pack() to WriteTo() migration
- [102](plan/learned/102-buffer-first-migration.md) -- Buffer-first migration, Span type abandoned
- [105](plan/learned/105-pathattributes-removal.md) -- PathAttributes struct removal (lazy over eager)
- [176](plan/learned/176-per-attribute-deduplication.md) -- Per-attribute-type pool dedup design
- [204](plan/learned/204-update-shared-parsing.md) -- Shared UPDATE parsing for wire/API

## Plugin System

Registration, SDK, event flow, lifecycle, hook integration.

- [253](plan/learned/253-nlri-plugin-extraction.md) -- NLRI codec extraction to plugins
- [256](plan/learned/256-plugin-lifecycle-mgmt.md) -- Plugin lifecycle management patterns
- [300](plan/learned/300-plugin-service-pattern.md) -- Plugin service pattern (SDK callbacks)
- [301](plan/learned/301-plugin-sdk-interface.md) -- SDK public interface design
- [303](plan/learned/303-plugin-api-dispatch.md) -- Plugin API dispatch via text commands
- [325](plan/learned/325-plugin-rib-families.md) -- Plugin RIB family registration

## Configuration

YANG schema, migration, config reload, editor, environment variables.

- [008](plan/learned/008-config-migration-system.md) -- Heuristic version detection, 3-version migration chain
- [065](plan/learned/065-spec-remove-version-numbers.md) -- No version numbers in config (YANG-transformable)
- [166](plan/learned/166-yang-only-schema.md) -- YANG as sole schema source of truth
- [175](plan/learned/175-config-editor-validation.md) -- Config editor validation pipeline
- [184](plan/learned/184-exabgp-to-yang-migration.md) -- ExaBGP syntax to YANG migration
- [226](plan/learned/226-config-reload-6-remove-bgpconfig.md) -- BGPConfig removal, map[string]any
- [232](plan/learned/232-editor-tree-canonical.md) -- Editor tree canonical representation

## CLI/API

Command structure, text format, IPC, RPC dispatch.

- [072](plan/learned/072-cli-run-merge.md) -- CLI run command consolidation
- [081](plan/learned/081-update-text-parser.md) -- Unified text parser for API commands
- [110](plan/learned/110-consolidate-update-commands.md) -- Update command consolidation
- [132](plan/learned/132-spec-test-cmd-consolidation.md) -- Test command consolidation
- [143](plan/learned/143-api-command-restructure-step-3.md) -- API command restructure (8 steps)
- [209](plan/learned/209-yang-ipc-dispatch.md) -- YANG-driven IPC dispatch
- [229](plan/learned/229-command-context-server-refactor.md) -- CommandContext server refactor
- [245](plan/learned/245-rib-command-unification.md) -- RIB command unification

## Web Interface

Web UI, HTMX, templates, looking glass, chaos dashboard.

- [266](plan/learned/266-chaos-web-foundation.md) -- Chaos web foundation, SSE debounce, OOB swaps
- [268](plan/learned/268-chaos-web-route-matrix.md) -- Route matrix visualization pattern

## RIB/Routing

Route storage, selection, forwarding, communities, path selection.

- [010](plan/learned/010-rib-config-design.md) -- RIB config design, storage model
- [173](plan/learned/173-plugin-rib-pool-storage.md) -- RIB pool storage design
- [275](plan/learned/275-spec-forward-pool.md) -- Forward pool, per-peer worker goroutines
- [316](plan/learned/316-outbound-rib-initialization.md) -- Outbound RIB initialization sequence
- [395](plan/learned/395-local-rib-architecture.md) -- Local RIB architecture, index design
- [402](plan/learned/402-bgp-route-selection.md) -- Best-path selection algorithm

## Protocol/RFC

Graceful restart, route refresh, capability negotiation, session management.

- [007](plan/learned/007-family-negotiation.md) -- Four family modes (enable/disable/require/ignore)
- [033](plan/learned/033-spec-eor-handling.md) -- End-of-RIB handling (RFC 4724)
- [254](plan/learned/254-rfc7606-enforcement.md) -- RFC 7606 treat-as-withdraw enforcement
- [369](plan/learned/369-bgp-graceful-restart-design.md) -- Graceful restart state machine design
- [375](plan/learned/375-ebgp-route-refresh-design.md) -- Route refresh design (RFC 2918/7313)
- [574](plan/learned/574-bgp-4-bmp.md) -- BMP receiver + sender (RFC 7854), config-as-strings, synthetic OPENs
- [647](plan/learned/647-bmp-5-sender-compliance.md) -- BMP sender compliance: real OPENs, Route Mirroring, ribout dedup

## Observability

Metrics, telemetry, Prometheus exporters, third-party format compatibility.

- [653](plan/learned/653-netdata-os-collectors.md) -- Netdata-compatible OS collector framework, 138 metrics, counter-wrap protection, per-collector config via YANG, verify names against source not summaries

## Testing

Test patterns, infrastructure, chaos testing.

- [274](plan/learned/274-spec-test-diagnostics.md) -- Test diagnostic improvements
- [258](plan/learned/258-bgp-chaos-families.md) -- Chaos family fuzzing
- [265](plan/learned/265-bgp-chaos-selftest.md) -- Chaos self-test patterns
- [608](plan/learned/608-concurrent-test-patterns.md) -- Concurrent-test flake patterns (locked-write/unlocked-read, subscribe-before-broadcast, gate-handler, barrier FIFO, cleanup-drains-work)

## Gotchas

Reusable lessons extracted from gotchas sections across summaries.

- (001) Freeform config parsing without schema causes data extraction failures; schema-driven (YANG) prevents this
- (008) Preserve insertion order of conditional rules -- they apply sequentially, not by specificity
- (013) EOR semantics extend RFC 4724 beyond graceful restart; document RFC violations when accepting them
- (102) Over-engineered specialized types (e.g., Span) often lose to native types; prefer native until proven insufficient
- (133) Renaming packages to short common nouns causes variable shadowing in callers
- (149) Do not force async protocols for in-process communication; adds complexity for no benefit
- (165) Organizational separation is distinct from protocol redesign; reuse existing infrastructure
- (176) Preserve attribute flag values separately -- do not hardcode reconstruction flags
- (247) Check dependency graphs before large restructurings; circular imports are blocking
- (253) Import cycles in test files reveal over-tight coupling; use external test packages
- (266) SSE debounce in HTTP layer prevents blocking the main event loop
- (275) Concurrent sends racing with channel close require WaitGroup coordination to avoid panic
- (647) Early return in event handler blocks housekeeping (caching, cleanup) that must run regardless of sender state
- (647) KEEPALIVE has nil RawBytes; check RawMessage != nil, not RawBytes != nil, for messages with no body
- (652) Verify "does not exist" claims during child spec RESEARCH; umbrella assumed show interface was missing but it was fully implemented
- (652) subsystem-list was hardcoded to ["bgp"]; always check stub implementations before assuming real data flows
