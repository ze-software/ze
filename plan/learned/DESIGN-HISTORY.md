# Design History

A chronological-by-subsystem map of how ze's code got to its current shape,
extracted from the 638 learned summaries in this directory. Read this
document first when the question is "why is X structured this way?" — it
points at the specific learned summaries that hold the full record, so
you need to read one file instead of 638.

See also: `METHODOLOGY.md` (how individual summaries are written),
`../../ai/LEARNED-INDEX.md` (curated index by topic).

## How to read this document

Each subsystem section has four fixed parts:

1. **Current shape** — one paragraph describing what exists today.
2. **Evolution** — the phases that produced the current shape, each
   pointing at one or more `plan/learned/NNN-*.md` entries.
3. **Abandoned approaches** — designs that were tried and removed.
   Important: if you are about to propose one of these, read the
   learned summary first to understand why it did not work.
4. **Load-bearing invariants** — facts about the code that are easy
   to break by accident. Each invariant names the code site that
   enforces it and the summary that records the reasoning.

When a section points at a numbered summary, read the summary for
the rejected alternatives and the gotchas. This document is the
index; the summaries are the authority.

---

## Era and phase summary (for orientation)

| Era | Range | Theme |
|-----|-------|-------|
| Foundation | 001-100 | BGP engine port from ExaBGP. Zero-copy, pool dedup, lazy parsing, config migration from ExaBGP syntax to YANG. |
| Buffer-first + Plugins | 101-200 | Pack() to WriteTo(buf, off), UpdateBuilder, hub separation, YANG as sole schema, plugin restructure (`ze.X` in-process plugins, 5-stage startup). |
| Plugin Architecture Maturity | 201-300 | YANG-driven IPC, two-socket RPC, SDK callback pattern, config reload coordinator, bgp-chaos, RIB plugin with per-attribute dedup + refcounted cache, RFC 7606 enforcement. |
| Arch-0 Restructuring + RIB Expansion | 301-400 | Tier-ordered startup, dependencies, UTP (unified text protocol), allocation reduction, 4-component system boundary (Engine, ConfigProvider, PluginManager, Subsystem), backpressure, panic recovery, LLGR, RPKI, editor modes, web UI. |
| Filter Framework + Ops | 401-500 | Prefix limits, filter framework, OTC/role/RPKI decorators, RBAC, per-user drafts with mtime conflict detection, ze-perf benchmarking, interface management, config archive, DNS, zefs blob namespaces. |
| Protocol Expansion | 501-640 | BART RIB, family registry, L2TP + PPP stack, BFD, VPP backend, firewall (nft), gokrazy appliance, sysctl, host inventory, RS fastpath, structured event bus migration (bus absorbed into stream system), TACACS, user login. |

---

## BGP engine: wire encoding and RIB

### Current shape

One reactor per `ze bgp` process, goroutine-per-peer, shared pools for
attribute deduplication. UPDATE reading is lazy via `WireUpdate`
iterators over raw wire bytes. Encoding is buffer-first via
`WriteTo(buf, off) int` into pooled buffers. A peer has two
`EncodingContext`s (recv, send), each identified by a `ContextID uint16`
in a global registry; zero-copy forwarding is a single integer
comparison. Best-path selection lives in the `bgp-rib` plugin, not the
engine; the engine has no Loc-RIB table.

### Evolution

- **[001](001-initial-implentation.md)** Foundation plan. Zero-copy
  forwarding via shared encoding context; goroutine-per-peer replaces
  ExaBGP's Python async reactor; per-attribute pools with mutex-based
  typed stores (later replaced by pool handles).
- **[003](003-pool-completion.md), [059](059-spec-pool-handle-migration.md),
  [124](124-unified-handle-nlri.md), [174](174-pool-restore-delete-pool.md),
  [176](176-per-attribute-deduplication.md), [332](332-pool-simplify.md)**
  Pool evolution. Single-pool → per-attribute-type pools (22 codes) →
  unified handle layout (bufferBit | poolIdx | flags | slot) → flags
  removed (slot widened to 26 bits) → double-buffer compaction scheduler
  wired into RIB plugin lifecycle.
- **[073](073-spec-buffer-writer.md), [075](075-nlri-writeto-zero-alloc.md),
  [092](092-pack-to-writeto-migration.md), [102](102-buffer-first-migration.md),
  [103](103-writeto-error-return.md), [114](114-pack-removal.md),
  [115](115-pack-removal.md), [116](116-wirewriter-unification.md)** Wire
  encoding migration. `Pack() []byte` allocated; replaced by
  `WriteTo(buf []byte, off int) int` with `CheckedWriteTo` variants for
  overflow detection. Session holds `writeBuf []byte` sized 4K pre-OPEN,
  resized to 65535 after Extended Message negotiation.
- **[076](076-spec-wire-update.md), [078](078-wireupdate-split.md),
  [079](079-spec-wireupdate-error-returns.md),
  [204](204-update-shared-parsing.md)** WireUpdate design. Raw wire
  bytes held, attributes lazy-parsed via AttributesWire. `UpdateSections`
  stores offsets (integers), not data slices.
- **[034](034-spec-addpath-encoding.md),
  [037](037-spec-asn4-packcontext.md),
  [038](038-spec-encoding-context-design.md),
  [039](039-spec-encoding-context-impl.md),
  [063](063-spec-afi-safi-map-refactor.md),
  [112](112-negotiated-composite-refactor.md),
  [113](113-encoding-context-consolidation.md)** Encoding context unification.
  Three Family types (nlri, capability, context) collapsed to one;
  `PackContext` → `WireContext` → `EncodingContext`. Per-peer recv/send
  contexts because ADD-PATH is the only asymmetric capability. Hash
  collision-resistant FNV-64a for registry dedup. `ContextID uint16`
  vs. pointer saves 6 MB at 1M routes.
- **[014](014-two-level-grouping.md),
  [030](030-spec-peer-encoding-extraction.md),
  [032](032-spec-update-builder.md),
  [061](061-spec-route-grouping.md),
  [062](062-spec-update-size-limiting.md),
  [097](097-api-bounds-safety.md)** UpdateBuilder. Routes grouped by
  non-AS_PATH attributes then by AS_PATH; MP_REACH NEXT_HOP lives in
  the attribute, not the base attribute section. Size limits enforced
  at the RIB-to-send boundary via `BuildGroupedUnicastWithLimit`
  (multi-route, splits) and `*WithMaxSize` (single route, errors).
- **[093](093-writeto-bounds-safety.md),
  [220](220-writeto-remaining-allocs.md),
  [377](377-chunknlri-zero-copy.md),
  [450](450-iter-elements.md)** Split-function zero-copy. `SplitMPNLRI`
  returns subslices of the original buffer. `ChunkMPNLRI` subsumed into
  `iter.Elements`.
- **[070](070-addpath-simplification.md),
  [071](071-nlri-wire-tests.md),
  [084](084-spec-multi-label-support.md),
  [086](086-spec-nlri-struct-embedding.md)** NLRI refactoring.
  `Len()` and `WriteTo()` return payload only; caller prepends path-id
  via `WriteNLRI` helper. `hasPath bool` on NLRI struct removed.
  `Bytes()` (identity/RIB keys) and `Pack()` (wire) are different
  concerns.
- **[173](173-plugin-rib-pool-storage.md),
  [253](253-nlri-plugin-extraction.md),
  [296](296-rib-02-adj-rib-in.md),
  [297](297-rib-03-rr-replay.md),
  [301](301-rib-04-plugin-dependencies.md),
  [303](303-seqmap.md),
  [304](304-seqmap-2-cache-contention.md),
  [317](317-refcounted-cache.md),
  [340](340-addpath-rib.md),
  [374](374-rib-05-best-path.md),
  [384](384-rib-pipeline-commands.md),
  [387](387-rib-06-pipe-filters.md),
  [534](534-rib-alloc.md),
  [607](607-rib-bart-bestprev.md),
  [618](618-rib-bestpath-pack.md)** RIB plugin arc. RIB commands
  originally engine builtins; moved to plugin-provided via registry
  dispatch. adj-rib-in plugin stores raw wire bytes for zero-copy
  replay. `seqmap` library gives O(log N + K) delta queries. Refcounted
  cache replaced TTL — time-based eviction silently drops routes when
  plugin is slow. RIB show/best moved from separate commands to
  pipeline iterator model. Storage backend: map (default) and BART
  (exact-match trie) for best-path store. `bestPrevStore` interner
  collapses per-route state to `uint16` indices.
- **[254](254-rfc7606-enforcement.md)** RFC 7606. Severity ordering via
  iota: None / AttributeDiscard / TreatAsWithdraw / SessionReset.
  Numeric comparison gives strength ordering. attribute-discard and
  zero-copy are incompatible — motivated `draft-mangin-idr-attr-discard-00`.
- **[275](275-spec-forward-pool.md),
  [276](276-rpc-multiplexing.md),
  [277](277-rr-ebgp-forward.md),
  [278](278-rr-flow-control.md),
  [289](289-rr-per-family-forward.md),
  [292](292-persistent-conn-reader.md),
  [316](316-buffered-writes.md),
  [392](392-forward-congestion-phase1.md),
  [394](394-forward-congestion-phase3.md),
  [424](424-forward-backpressure.md),
  [445](445-forward-congestion-phase4-5.md),
  [457](457-forward-congestion-phase2.md),
  [630](630-rs-fastpath-3-passthrough.md)** Forward path evolution. Per-destination-peer worker goroutines,
  MuxConn for concurrent RPCs (`#id <verb> [<json>]\n`), EBGP wire
  variants cached per-UPDATE (ASN4 vs ASN2 prepend), four-layer
  congestion response (bounded overflow, Prometheus metrics, read
  throttle, teardown), fire-and-forget `ForwardUpdatesDirect` typed
  path for internal plugins.
- **[239](239-rfc9234-role.md),
  [280](280-capability-mode.md),
  [408](408-dynamic-send-types.md),
  [442](442-filter-community.md),
  [464](464-role-otc.md),
  [541](541-policy-framework.md),
  [548](548-cmd-4-prefix-filter.md),
  [552](552-cmd-4-prefix-filter-phase2.md),
  [569](569-cmd-5-aspath-filter.md),
  [570](570-cmd-6-community-match.md),
  [571](571-cmd-7-route-modify.md),
  [590](590-cmd-1-rr-nexthop.md),
  [591](591-cmd-3-multipath.md),
  [593](593-cmd-2-session-policy.md)** Filter and policy framework.
  Capability mode (enable/disable/require/refuse) post-negotiation
  validation. Dynamic event/send type registration from plugins.
  OTC (RFC 9234), prefix-list, as-path, community-match, route-modify
  filter plugins. `apply_mods` attribute modification framework.

### Abandoned approaches

- **AS-PATH as NLRI indexing** (001) — proposed as novel index, listed
  as risk, never carried into production.
- **Span type** (102, 117) — introduced for compact offset storage,
  removed as over-engineered; native `[]byte` subslice suffices.
- **Adj-RIB-Out integration in engine** (060, 068) — reversed.
  Persistence delegated to external API programs (current RIB plugin).
- **`PathAttributes` intermediate struct** (105) — text → struct → wire
  bytes, replaced by `attribute.Builder` for text → wire bytes directly.
- **Cobra + Viper for CLI** (001) — rejected for stdlib `flag.FlagSet`.
- **Freeform config parsing** (001, 166, 281) — extracted content without
  schema; drove the YANG-as-sole-schema redesign.
- **`rib enter-llgr` / `rib depreference-stale` dedicated commands** (407)
  — rejected in favour of composable `attach-community` / `delete-with-community`
  / `mark-stale [level]`.
- **`forward-ebgp` command** (277) — rejected; engine already knows peer
  types, a single branch in `ForwardUpdate` replaces the command.
- **`BuildGroupedUnicast` for RIB routes** (171) — conversion cancelled;
  RIB routes have existing wire bytes (zero-copy forwarding), only
  locally-originated routes use `UpdateBuilder`.
- **Struct-based RIB routes** (321) — `StructuredEvent` wrapper was an
  identity wrapper pattern; rejected. Engine delivers `*RawMessage`
  directly via DirectBridge; plugin walks wire bytes with existing
  iterators.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| A peer has two `EncodingContext`s, never one | `peer.go` (`recvCtx`, `sendCtx`) | ADD-PATH is the only asymmetric capability; single context would conflate recv/send semantics. ([038](038-spec-encoding-context-design.md)) |
| `ContextID` is `uint16`, not a pointer | `internal/bgp/context/` | Saves ~6 MB at 1M routes, single-integer zero-copy comparison. ([038](038-spec-encoding-context-design.md), [039](039-spec-encoding-context-impl.md)) |
| `Bytes()` (identity/RIB key) and `Pack()`/`WriteTo()` (wire) are separate | NLRI types | Wire excludes path-id (caller prepends); RIB keys include path-id for uniqueness. Confusing them corrupted EVPN ADD-PATH once. ([070](070-addpath-simplification.md), [071](071-nlri-wire-tests.md)) |
| attribute-discard and zero-copy are incompatible | `rfc7606.go` | Stripping bytes from wire encoding breaks zero-copy forwarding. Solution is a new path attribute, not in-place stripping. ([254](254-rfc7606-enforcement.md)) |
| NLRI overrun → session reset, not treat-as-withdraw | `rfc7606.go` | The NLRI is not parseable, individual prefix withdrawal is impossible. ([254](254-rfc7606-enforcement.md)) |
| MP_REACH + MP_UNREACH can carry up to 3 distinct families per UPDATE | RFC 4760 | Per-family wire splitting not done in engine `ForwardUpdate`. ([289](289-rr-per-family-forward.md)) |
| Hold time 0 disables throttling entirely | `ReadThrottle.ThrottleSleep` | RFC 4271 §4.4: hold-time 0 means no timers, no safe sleep budget. ([394](394-forward-congestion-phase3.md)) |
| Best-path lives in bgp-rib plugin, on-demand | `internal/component/bgp/plugins/bgp-rib/` | Real-time deferred until export policy exists; engine has no Loc-RIB. ([374](374-rib-05-best-path.md)) |
| Per-source-peer worker pool serializes FIFO | bgp-rr `forwardWorker` | Cache ack protocol requires FIFO message-id order; unbounded goroutines caused ~98% route loss. ([269](269-rr-serial-forward.md)) |

---

## BGP engine: session, FSM, TCP

### Current shape

FSM is pure state transitions + timers. `session.go` orchestrates I/O
and delivers UPDATEs to the reactor's per-peer delivery goroutine.
Read loop uses `bufio.Reader` with 64K buffer and close-on-cancel
(cancel goroutine closes `net.Conn` to unblock `ReadFull`). Writes
are protected by a `writeMu` and batched via a 16K `bufio.Writer`
drained by the forward pool. Listener per local-address; peer lookup
by remote IP. RFC 9234 Role and collision detection in engine;
strict enforcement in the `bgp-role` plugin.

### Evolution

- **[015](015-fsm-active-design.md)** FSM/Reactor split intentional: FSM
  = pure transitions + timers; Reactor = orchestration + I/O. Follows
  ExaBGP pattern. Reactor bloat was in `peer.go` encoding logic, not
  FSM design — addressed separately.
- **[023](023-spec-collision-detection.md)** RFC 4271 §6.8 collision
  detection: detection at peer/reactor level; two-phase (reject if
  Established, else wait for remote BGP ID in OPEN). OpenSent collision
  NOT handled — only OpenConfirm (the MUST case).
- **[049](049-spec-listener-per-local-address.md)** Per-peer listeners:
  listeners keyed by `netip.Addr`; `LocalAddress` mandatory.
  Self-referential peers and link-local IPv6 rejected.
- **[233](233-mandatory-local-address.md)** `local-address` mandatory,
  validated in Go (`reactor/config.go`), because YANG `mandatory true`
  only checks presence, not format.
- **[067](067-spec-peer-lifecycle-callbacks.md)** `PeerLifecycleObserver`
  interface; observers registered via `AddPeerObserver`. `OnPeerEstablished`
  fires BEFORE `sendInitialRoutes()` so plugins see Established before
  routes arrive.
- **[142](142-api-sync.md)** `plugin session ready` mandatory: Ze waits
  for all processes to signal ready before starting peer connections
  (5s timeout, then proceeds with warning). Same mechanism per-session
  on reconnect.
- **[244](244-reactor-interface-split.md),
  [247](247-plugin-restructure.md),
  [248](248-plugin-bgp-format-extraction.md),
  [250](250-move-require-bgp-reactor.md),
  [251](251-commit-manager-injection.md),
  [252](252-remove-type-aliases.md),
  [351](351-reactor-lifecycle-split.md)** ReactorInterface split.
  68-method monolith → `ReactorLifecycle` + `BGPReactor` → 5 focused
  sub-interfaces (ReactorIntrospector, ReactorPeerController,
  ReactorConfigurator, ReactorStartupCoordinator, ReactorCacheCoordinator).
  Adapter pattern: `ReactorLifecycle` is implemented by unexported
  `reactorAPIAdapter`, not `*Reactor` directly.
- **[272](272-session-read-pipeline.md),
  [279](279-session-write-mutex.md),
  [290](290-buffered-tcp-read.md),
  [316](316-buffered-writes.md),
  [376](376-backpressure.md),
  [382](382-sendctx-race.md)** Session I/O pipeline. Close-on-cancel for
  read interruption. `writeMu` added after missing synchronization was
  blamed on "externally synchronized" comments that no caller honored.
  `bufio.Reader` 64K (matches Extended Message). Forward pool batch
  drain. Atomic pointer for `sendCtx` (concurrent readers from plugin
  dispatch without RLock because `peer_initial_sync.go` already holds
  `p.mu.Lock`).
- **[288](288-ze-sim-abstractions.md),
  [264](264-bgp-chaos-inprocess.md),
  [265](265-bgp-chaos-selftest.md)** Clock/Dialer/ListenerFactory
  injection. `sim.Clock`, `sim.Dialer`, `sim.ListenerFactory` interfaces.
  VirtualClock for in-process chaos. ChaosClock for `--chaos-seed`
  self-test mode. Grep audit test forbids direct `time.*` and `net.*`
  in reactor/FSM.
- **[365](365-panic-recovery.md)** Per-peer panic recovery. `safeRunOnce`
  wraps peer lifecycle; panic becomes error, feeds backoff loop.
  Delivery goroutine recovery exits loop (session tears down anyway).
  4K stack capture via `runtime.Stack`.

### Abandoned approaches

- **`passive bool`** (271) — replaced with independent `connect` and
  `accept` booleans. Defense-in-depth: both checked at reactor.
- **`session reset` command** (170) — removed with API restructure.
- **`CBOR` plugin encoding** (170) — incompatible with line-delimited
  protocol.
- **Field names `Encoder` and `Serial`** on `CommandContext` (229) —
  dead fields, never read; removed.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `local-address` is mandatory for every peer | `reactor/config.go` | Explicit choice per peer; `auto` is allowed for OS-selected binding but must be explicit. ([049](049-spec-listener-per-local-address.md), [233](233-mandatory-local-address.md)) |
| Peer lookup on incoming connection is by remote IP, not listener address | `reactor/listener.go` | Listener address is used only for RFC-compliance validation. ([049](049-spec-listener-per-local-address.md)) |
| Keepalive timer fires via `time.AfterFunc` in independent goroutine | `session.go` | Writes through `sendKeepalive` → `writeMessage`; this is the least obvious concurrent caller. `writeMu` required. ([279](279-session-write-mutex.md)) |
| `connectionEstablished()` sends an OPEN | session setup | Tests that set up TCP sessions directly must use raw field assignment. ([415](415-prefix-data.md)) |
| Reactor code must use `sim.Clock` interface, not `time.*` | `reactor/*.go` | Chaos and virtual-time tests bypass `time.Now()` silently — audit test forbids direct calls. ([288](288-ze-sim-abstractions.md)) |
| Collision detection happens at OpenConfirm, not OpenSent | `peer.go collision check` | OpenSent collision is a MAY, never implemented. OpenConfirm is the MUST case. ([023](023-spec-collision-detection.md)) |

---

## Configuration: YANG, parser, editor, reload

### Current shape

YANG is the single source of schema truth. The parser tokenizes a
set+meta line-oriented format (write-through per session), validates
against YANG, and produces a `Tree` + `MetaTree` in memory. Config
travels through the pipeline as `File → Tree → ResolveBGPTree →
map[string]any → reactor.PeersFromTree`. The editor holds `Tree` as
canonical (not raw text). SIGHUP triggers a two-phase verify+apply
coordinator. Plugins declare `wants config <root>` and receive the
subtree at Stage 2; they parse the JSON themselves.

### Evolution

- **[008](008-config-migration-system.md),
  [009](009-neighbor-to-peer-rename.md),
  [041](041-spec-format-based-migration.md)** Config migration. Three-
  version chain (v1 ExaBGP main → v2 Ze intermediate → v3 `peer` +
  `template.match`). Heuristic detection (no version field). Named
  semantic transformations over numbered versions.
- **[065](065-spec-remove-version-numbers.md)** Version numbers
  removed from code (API versions, config fields, migration comments).
  Design for machine-transformable migration.
- **[050](050-spec-environment-config-block.md),
  [476](476-env-registry-consistency.md),
  [506](506-listener-6-compound-env.md),
  [628](628-env-cleanup.md)** `environment { }` block. Priority:
  OS env > config block > defaults. Strict validation (unknown keys
  fail at startup). Env vars registered via `env.MustRegister` silently
  overwrite duplicates; known two-site drift (`environment.go` +
  consumer) tracked in memory.
- **[166](166-yang-only-schema.md),
  [167](167-yang-schema-refactor.md),
  [180](180-native-update-syntax.md),
  [181](181-remove-exabgp-announce.md),
  [281](281-remove-ze-syntax.md)** YANG as sole schema. `ze:syntax`
  extensions removed — standard YANG `leaf-list` / presence container
  / list handle what `ze:syntax` used to annotate. Freeform parsing
  eliminated. `LegacyBGPSchema()` retained only for ExaBGP migration.
- **[151](151-hub-yang-modules.md),
  [334](334-yang-reorganisation.md),
  [488](488-lg-looking-glass.md),
  [556](556-bfd-1-wiring.md),
  [577](577-gokrazy-2-ntp.md)** YANG reorganisation. Each module lives
  with the package that owns it; `init()` registers via
  `yang.RegisterModule`; `yang_schema.go` hard-codes the module list
  (TWO registrations required — known duplication).
- **[293](293-yang-validation.md),
  [356](356-editor-modes.md),
  [410](410-validate-completion.md),
  [551](551-filter-non-cidr-families.md)** YANG `ze:validate` extension.
  Registry in `internal/yang/`; validators in other packages register
  via explicit `RegisterValidators()`. `CompleteFn` provides autocompletion
  in the editor. Runtime-determined sets (plugin families) use
  `ze:validate`; compile-time sets use YANG native constraints.
- **[212](212-inline-config-reader.md),
  [213](213-config-yang-validation.md),
  [214](214-pluggable-config-frontend.md),
  [345](345-hub-phase2-config-reader.md),
  [346](346-hub-phase3-yang-integration.md)** Config reader. Standalone
  binary → inline library. Tokenizer produces `map[string]any` directly
  (JSON roundtrip removed). `ValidateContainer` (flat, per-block) and
  `ValidateTree` (recursive, at load time).
- **[175](175-config-editor-validation.md),
  [232](232-editor-tree-canonical.md),
  [349](349-load-command-redesign.md),
  [369](369-editor-4-workflow-tests.md),
  [370](370-editor-5-reload-probe.md),
  [391](391-concurrent-config.md),
  [427](427-per-user-drafts.md),
  [428](428-command-history.md)** Editor. Text surgery (findFullContextPath,
  setValueInConfig) deleted. `Tree` is canonical; `WorkingContent()` =
  `Serialize(tree)`. Per-user change files (`config.conf.change.<user>`)
  replace shared draft. Session identity is `user@origin:unix-ts`.
  Live conflict (same path, two sessions) and stale conflict (committed
  value changed since last set) detected explicitly.
- **[222](222-config-reload-1-rpc.md) through
  [234](234-reload-peer-add-remove.md),
  [342](342-reload-test-framework.md)** Config reload. Two-phase
  verify+apply coordinator. Plugins register `WantsConfigRoots`; engine
  sends `config-verify` → all plugins OK → `config-apply`. Any verify
  failure aborts. SIGHUP wired through coordinator; editor triggers
  reload via RPC (not signal).
- **[380](380-config-archive.md)** Config archive. VyOS-inspired fan-out:
  all locations attempted, errors collected per-location, non-fatal to
  commit. `file://` and `http(s)://` protocols.
- **[426](426-blob-namespaces.md),
  [456](456-zefs-integration.md),
  [463](463-zefs-socket-locking.md),
  [477](477-zefs-key-registry.md)** Zefs blob store. Two namespaces:
  `meta/` (instance metadata), `file/active/` (current config).
  `file/draft/` and `file/<date>/` qualifiers planned. No flock (all
  sessions in-process); `Storage.AcquireLock` returns a `WriteGuard`.
- **[537](537-config-tx-protocol.md),
  [538](538-report-bus.md)** Transaction protocol. Orchestrator
  migrated from the bus to the stream system during the larger
  bus-removal work. Participating plugins declare via `WantsConfig`;
  orchestrator sends verify → all ok → apply → all ok → commit.

### Abandoned approaches

- **`ConfigVersion` type and numbered migrations** (041) — replaced
  with named `Transformation` registry.
- **`ze:syntax` YANG extensions** (281) — ALL of them were display
  artifacts over standard YANG types. Removed.
- **Flat shared draft** (427) — replaced with per-user change files.
- **Socket locking with flock** (463) — went through three design
  iterations (new RPC protocol → flock on socket → "SSH already exists").
- **`UpdateSections.parsed bool`** (204) — replaced with
  `sections.Valid()`.
- **`freeform` parser mode** (281) — stored word sequences as opaque
  map keys, prevented schema-level validation.
- **Config push from hub to plugins** (160) — pull model: hub notifies,
  plugins query `query config live|edit path`.
- **`SetParser.ValidateValue`** (214) — YANG is the sole validator.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| YANG is the single source of schema truth | `internal/component/config/yang/` | `BGPSchema()` removed; `LegacyBGPSchema()` only for ExaBGP migration. ([166](166-yang-only-schema.md)) |
| Every YANG `environment/<name>` leaf needs `ze.<name>.<leaf>` registered via `env.MustRegister` | plugin `init()` | Env vars are part of the config interface, not follow-up work. ([050](050-spec-environment-config-block.md), CLAUDE.md rule) |
| Every new top-level config block requires TWO registrations | `init()` + `yang_schema.go:YANGSchemaWithPlugins()` | `yang.RegisterModule()` makes module available to loader; explicit call in `yang_schema.go` builds the schema. Parser does not discover. ([488](488-lg-looking-glass.md), [556](556-bfd-1-wiring.md), [577](577-gokrazy-2-ntp.md)) |
| `ApplyConfigDiff` in production re-reads disk via `reloadFn` | `reactor/config.go` | Verify and apply must see the same on-disk state at their respective times. ([466](466-set-with.md), [535](535-config-tx-consumers.md)) |
| `GetConfigTree` returns live map reference, not a copy | `reactor/config.go` | Mutating outside locks races. ([466](466-set-with.md)) |
| `ze config validate` does NOT invoke plugin `OnConfigVerify` | `cmd/ze/config/cmd_validate.go` | Parser + YANG type check only; plugin-side validation runs only in running daemon. ([413](413-prefix-limit.md), [557](557-iface-tunnel.md), [621](621-backend-feature-gate.md), [627](627-fw-7-traffic-vpp.md)) |
| Plugin config tree delivered wrapped as `{"bgp":{...}}` | `server/reload.go` | Plugins must unwrap before accessing their subtree. ([185](185-config-json-delivery.md), [538](538-report-bus.md)) |
| YANG list = JSON map keyed by list key | type conversion | `list server { key "name"; }` → `"server": {"default": {...}}`, not `[{"name":"default",...}]`. ([574](574-bgp-4-bmp.md)) |
| Config values survive JSON roundtrip as strings | type conversion | `"enabled": "true"` not `"enabled": true`; `strconv.ParseUint` needed for numeric. ([213](213-config-yang-validation.md), [556](556-bfd-1-wiring.md), [574](574-bgp-4-bmp.md)) |

---

## Plugin system: architecture

### Current shape

A plugin is an `init()` function that calls `registry.Register(name,
Registration{...})`. The registration declares: capabilities the plugin
handles, address families, config roots (`WantsConfig`), event types
it emits (`EventTypes`), send types it provides (`SendTypes`),
command handlers (in a `-cmd.yang` module via `ze:command`), and
dependencies. A plugin runs in one of three modes: **Fork** (external
binary or path), **Internal** (same binary, goroutine +
`net.Pipe()` pair), **Direct** (synchronous in-process call via
DirectBridge zero-copy transport). All use the same 5-stage startup
protocol over Socket A (plugin → engine) and Socket B (engine →
plugin), with auto-detected JSON or text framing (first byte).

### Evolution

- **[001](001-initial-implentation.md)** Original plugin protocol:
  JSON events down, text commands up, stdin/stdout.
- **[069](069-spec-api-command-serial.md),
  [168](168-api-command-serial.md)** Serial correlation. `#N` numeric
  prefix (plugin → engine), `#abc` alpha prefix (engine → plugin),
  `@serial` response echo. Empty serial (`""`) for unsolicited events.
- **[142](142-api-sync.md),
  [152](152-hub-phase1-schema-infrastructure.md),
  [172](172-api-capability-contract.md),
  [305](305-plugin-startup-ordering.md)** 5-stage startup protocol.
  Stage 1 declarations (capabilities, families, schema, commands),
  Stage 2 config delivery, Stage 3 capabilities, Stage 4 registry,
  Stage 5 ready. Tier-ordered per Kahn topological sort; dependencies
  resolved pre-startup.
- **[184](184-plugin-yang-discovery.md),
  [198](198-plugin-invocation.md),
  [210](210-yang-ipc-plugin.md),
  [264](264-bgp-chaos-inprocess.md),
  [294](294-inprocess-direct-transport.md),
  [459](459-plugin-tcp-transport.md)** Plugin invocation modes.
  `ze.X` prefix = internal (goroutine + `io.Pipe` or `net.Pipe`),
  path/cmd = fork. Same protocol both. DirectBridge skips JSON +
  socket I/O for internal plugins (415× faster UPDATE delivery).
  TLS plugin hub server (fleet-config use case).
- **[209](209-yang-ipc-dispatch.md),
  [215](215-yang-ipc-cleanup.md),
  [395](395-yang-command-tree.md)** YANG-driven dispatch. Text
  `RegisterBuiltin()` replaced by YANG RPC metadata extraction.
  `ze:command "wire-method"` YANG extension binds tree nodes to
  handlers. `"bgp "` prefix removed from all commands.
- **[218](218-plugin-auto-registration.md),
  [247](247-plugin-restructure.md),
  [248](248-plugin-bgp-format-extraction.md),
  [253](253-nlri-plugin-extraction.md),
  [282](282-capa-plugins.md),
  [283](283-softver-plugin.md),
  [329](329-watchdog-plugin.md),
  [375](375-handler-split.md)** Plugin extraction. Watchdog, Hostname,
  FlowSpec, EVPN, VPN, BGP-LS, RouteRefresh, SoftwareVersion, GR, role
  all moved out of engine. `internal/component/bgp/plugins/bgp-*`
  convention. NLRI plugins use `bgp-nlri-*` prefix (9 plugins, 4 tiers).
- **[291](291-batched-ipc-delivery.md),
  [292](292-persistent-conn-reader.md),
  [298](298-event-delivery-batching.md),
  [299](299-text-event-format.md),
  [321](321-alloc-4-structured-delivery.md),
  [322](322-alloc-0-umbrella.md),
  [422](422-structured-delivery.md),
  [606](606-eventbus-typed.md)** Event delivery performance.
  JSON-RPC `deliver-batch` replaces per-event writes. Persistent reader
  goroutine replaces 5 per-RPC goroutines. Text format opt-in per
  plugin. DirectBridge delivers `*RawMessage` directly; plugin reads
  wire bytes via existing `NLRIIterator`. `ze.EventBus` interface typed.
- **[315](315-utp-3-handshake.md),
  [318](318-utp-0-umbrella.md),
  [397](397-unified-rpc-framing.md)** Unified text protocol.
  Auto-detect JSON vs text from first byte. Single framing
  (`#<id> <verb> [<json>]\n`) replaces dual protocol. Heredoc
  `<<EOF` for multiline content.
- **[323](323-arch-1-interfaces.md) through
  [328](328-arch-6-eliminate-hooks.md),
  [419](419-arch-7-subsystem-wiring.md) through
  [425](425-arch-0-system-boundaries.md),
  [531](531-config-inline-container.md),
  [533](533-bgp-boundary-cleanup.md)** Arch-0 restructuring.
  Four components (originally five): Engine, ConfigProvider,
  PluginManager, Subsystem. Interfaces in `pkg/ze/`. Subsystem ≠
  Plugin (BGP daemon owns TCP/FSM; bgp-rib/rs/gr are plugins).
  BGPHooks eliminated — replaced by typed `EventDispatcher` in
  `bgp/server/`. Bus absorbed into the stream system during config-tx
  protocol work; `ze.EventBus` is now backed by `Server.Emit` and
  `Server.Subscribe`.
- **[301](301-rib-04-plugin-dependencies.md)** Plugin dependencies
  declared in registration. Two-layer validation: Go registry for
  pre-startup auto-loading of internal plugins; protocol Stage 1 for
  runtime validation of all plugins. Fail-loud on missing dependency.
- **[536](536-family-registry.md)** Family registry. Runtime-registered
  families (dynamic), AFI/SAFI indexed. `family.Family.String()`
  requires the family to be registered; tests call
  `family.RegisterTestFamilies()`. Single atomic state; old dual
  (mutable + snapshot cache) collapsed.
- **[390](390-rbac.md),
  [452](452-ssh-server.md),
  [484](484-unified-cli.md),
  [601](601-tacacs.md),
  [598](598-aaa-registry.md),
  [600](600-user-login.md)** Auth. SSH server with bcrypt. RBAC
  (Nokia-inspired). AAA registry (`aaa.Default.Register`,
  `BackendRegistry`). TACACS+ plugin. Timing side-channel fixed
  (always run dummy bcrypt for unknown users).

### Abandoned approaches

- **Unified subsystem protocol with full async 5-stage for
  internal handlers** (149) — over-engineered. `init()` self-
  registration works for in-process calls.
- **Plugin YANG opt-in loading** (201) — chicken-and-egg; internal
  plugin YANG always loaded at engine startup.
- **Central `RegisterDefaultHandlers()`** (149) — replaced with
  `init()` self-registration + `LoadBuiltins(d)`.
- **Hooks (BGPHooks)** (328) — replaced by typed `EventDispatcher`.
- **SSH server using own TLS session** (484) — discarded when the
  insight "SSH already provides auth" simplified the design.
- **Standalone `ze-config-reader` binary** (212) — replaced with
  in-process library.
- **`sync.Once` in `OnceValue` callbacks that may error** (079) —
  second call returns cached `(nil, nil)`; must use explicit state.
- **Bus (standalone `internal/bus/`)** (324, 425) — absorbed into
  the stream system during config-tx protocol work. The stream
  system already provided in-process pub/sub with schema validation,
  DirectBridge zero-copy for internal plugins, and TLS delivery
  for external plugins; the bus was the weaker of the two. `ze.EventBus`
  remains as the public interface, backed by `Server.Emit` and
  `Server.Subscribe`.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| Internal plugin full 5-stage startup can deadlock | `DirectBridge` startup | Engine blocks waiting for plugin `ready`; plugin blocks waiting for engine config. Decode-only path skips stages. ([198](198-plugin-invocation.md), [264](264-bgp-chaos-inprocess.md)) |
| `net.Pipe()` writes block until reader is ready | test helpers | Zero-buffering. Tests must start readers before writes OR wrap writes in goroutines. ([210](210-yang-ipc-plugin.md), [264](264-bgp-chaos-inprocess.md), [459](459-plugin-tcp-transport.md)) |
| Plugin subprocess stderr consumed by `relayStderrFrom()` | `process.go` | Never reaches test runner's `expect=stderr:contains=`. ([451](451-rib-show-filters.md)) |
| Strict role (RFC 9234) enforcement stays in plugin | `bgp-role` plugin | Engine is policy-free. ([239](239-rfc9234-role.md)) |
| Plugin declares what it handles; engine never enumerates | registry API | Families, events, send types all dynamic. ([187](187-family-plugin-infrastructure.md), [188](188-flowspec-plugin.md), [404](404-rpki-decorator.md), [408](408-dynamic-send-types.md)) |
| Registry lookup is by name (exact, case-sensitive post-normalisation) | `registry.go` | Case normalized at write time, not read time. ([187](187-family-plugin-infrastructure.md), [412](412-rpki-test-isolation.md)) |
| Auto-linter strips imports between Edits | `auto_linter.sh` hook | `goimports` runs after every Edit/Write; import + first usage must be in same Edit call. ([288](288-ze-sim-abstractions.md), [422](422-structured-delivery.md), many others) |

---

## CLI, web, lookings glass, monitoring

### Current shape

One binary, `ze`, with subcommand dispatch. Interactive CLI over SSH
(Wish library) with Bubble Tea TUI. Dual mode: edit (config editing)
and command (operational RPCs). Shell completion for bash/zsh/fish/nu
with dynamic YANG-driven callbacks (`ze completion words show/run`).
Web UI (HTMX + SSE) provides config view/edit, admin command
execution, CLI modes (form/terminal), live updates. Looking glass
with birdwatcher-compatible API and topology graph. `ze-perf` for
benchmarking; `ze-chaos` for chaos testing; `ze-test` for functional
tests. Prometheus metrics at `/metrics`.

### Evolution

- **[004](004-edit-command.md),
  [175](175-config-editor-validation.md),
  [205](205-editor-testing-framework.md),
  [232](232-editor-tree-canonical.md),
  [339](339-config-completion-cli.md),
  [349](349-load-command-redesign.md),
  [356](356-editor-modes.md),
  [366](366-editor-1-exit-discard.md) through
  [370](370-editor-5-reload-probe.md)** Editor. Bubble Tea TUI with
  schema-driven completion. `.et` file-based headless test framework.
  Dual-mode (edit/command). Daemon socket probe at startup enables
  reload-on-commit.
- **[072](072-cli-run-merge.md),
  [199](199-cli-restructure.md),
  [372](372-show-routes.md),
  [373](373-shell-completion.md),
  [381](381-shell-completion.md),
  [383](383-command-package-extraction.md),
  [395](395-yang-command-tree.md),
  [440](440-bgp-dashboard.md),
  [446](446-feature-inventory-ci-gaps.md),
  [496](496-cli-dispatch.md),
  [518](518-shell-completion-v2.md),
  [625](625-rs-fastpath-1-profile.md)** CLI. `ze bgp run` + interactive
  merged into `ze cli`. `ze show` (read-only) and `ze run` (all) share
  `BuildCommandTree` from YANG. Shared `cmdutil` package. Pipe
  operators (`| json`, `| table`, `| text`, `| yaml`, `| count`)
  executed server-side where possible.
- **[388](388-cli-metrics.md),
  [389](389-cli-log.md),
  [396](396-bgp-monitor.md),
  [502](502-signal-status-command.md)** CLI commands for metrics/log/
  monitor/signal. `bgp metrics show/list`, `bgp log show/set`,
  `bgp monitor` streaming, `ze status`.
- **[266](266-chaos-web-foundation.md) through
  [268](268-chaos-web-route-matrix.md),
  [307](307-chaos-syncing-state.md) through
  [314](314-chaos-ux-0-umbrella.md),
  [357](357-chaos-web-dashboard.md),
  [358](358-chaos-web-controls.md)** Chaos web dashboard. HTMX + SSE.
  ~40-peer active set with adaptive TTL decay. 200 ms SSE debounce.
  Peer grid (alternative to table), donut chart, event toasts, chaos
  pulse, peer filter, convergence trend, chaos rate, trigger buttons.
- **[454](454-web-htmx-architecture.md) through
  [475](475-web-0-umbrella-retrospective.md),
  [474](474-web-admin-finder.md),
  [486](486-cli-nav-sync.md),
  [498](498-lg-overhaul.md)** Main web UI. Server-rendered Go HTML,
  HTMX, SSE. CSP-strict (no `unsafe-eval`). Config view/edit with
  `EditorManager` injected. Admin finder columns, CLI bar (form +
  terminal modes), live update banner, self-signed cert with SAN for
  every interface IP. Looking glass: birdwatcher API, topology graph,
  ASN decorators via `SetDecoratorRegistry`.
- **[417](417-perf.md),
  [433](433-interop-coverage.md),
  [565](565-bfd-3b-frr-interop.md)** `ze-perf` standalone benchmarking
  binary. Cross-implementation (Ze, GoBGP, FRR, BIRD, rustbgpd).
  NDJSON history, stddev-aware regression detection, Docker-orchestrated
  runs.
- **[386](386-prometheus-metrics.md),
  [453](453-prometheus-deep.md),
  [482](482-prometheus-plugin-health.md),
  [542](542-plugin-metrics.md),
  [561](561-bfd-4-operator-ux.md)** Prometheus metrics. Map-based
  idempotent `PrometheusRegistry`. Per-instance registry (not global)
  for test isolation. Counter metric families invisible until first
  `.Inc()`.

### Abandoned approaches

- **Standalone `ze bgp run` command** (072) — merged into `ze cli`.
- **Separate `rib show in/out/best`** (384, 387) — unified pipeline
  iterator with filters.
- **Custom JS charting for web dashboard** (266) — HTMX + CSS + inline
  SVG only. No JS framework.
- **WebSockets for dashboard** (266, 454) — SSE (server-push only)
  is simpler.
- **Template files for dashboard** (266, 454) — inline Go rendering.
- **`CLICommand` field in RPCRegistration** (395) — YANG `ze:command`
  extension is single source of truth.
- **`bgp` prefix on all commands** (395) — removed. User types
  `peer list`, not `bgp peer list`.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `EventSource` (SSE) does not support custom headers | `web/htmx.js` | Drives session-cookie auth over Basic Auth. ([468](468-web-1-foundation.md)) |
| HTMX SSE extension inserts via `innerHTML` | web handlers | Requires pre-rendering through `html/template` for XSS safety. ([473](473-web-6-live-updates.md)) |
| HTMX filter expressions in `hx-trigger` use eval internally | CSP config | Requires `unsafe-eval`; replace with plain JS event listener to keep CSP strict. ([454](454-web-htmx-architecture.md)) |
| Pipe operators default table when no format specified | `ProcessPipesDefaultTable` | `| json`, `| table`, `| text`, `| yaml` are format; `| count` is transform. Multiple formatters are an error. ([383](383-command-package-extraction.md)) |
| `ze config validate` invokes YANG type check only | `cmd/ze/config/cmd_validate.go` | Plugin `OnConfigVerify` runs only when daemon loads or reloads — parser tests cannot verify plugin-specific rules. ([413](413-prefix-limit.md), others) |

---

## Sub-protocols: BFD, L2TP, VPP, Firewall, Interface, Host, Gokrazy

### Current shape

- **Interface management** (`internal/component/iface`): backend-split
  (netlink/mock/VPP), monitor via netlink subscription, manage
  (add/remove/addr/unit/create/delete), BGP react on addr events, DHCP
  client for DHCPv4/v6 (DHCPv6 does not install default route — only
  Router Advertisements do), WireGuard via `wgctrl`. Per-protocol backend gate.
- **BFD** (`internal/component/bfd`): RFC 5880 plus RFC 5881/5882/5883
  variants. V4 and V6 transports (IPv6 uses `IPV6_RECVHOPLIMIT`). MD5/SHA1
  authentication. Echo mode. FRR interop scenarios in `test/interop/`.
  BGP client that brings BFD up alongside BGP peer.
- **L2TP + PPP** (`internal/component/l2tp`, `internal/component/ppp`):
  RFC 2661 wire, reliable delivery with `seqBefore` unsigned-distance
  comparison, tunnel + session FSM, kernel PPPoL2TP sockets via ioctl,
  LCP/PAP/CHAP/MSCHAPv2 + IPCP/IPv6CP NCPs.
- **VPP backend** (`internal/plugins/fibvpp`, `internal/component/vpp`):
  GoVPP binapi, AsyncConnect, stats socket for telemetry, PCI
  bind/unbind, classify/policer/QoS for traffic control.
- **Firewall** (`internal/component/firewall`): backend-split (nft on
  Linux). Linux uses netfilter rules via `vishvananda/netlink`.
- **Host inventory** (`internal/component/host`): stdlib parsing of
  `/proc/cpuinfo`, `/proc/meminfo`, `/proc/stat`, sysfs. `/proc/meminfo`
  values in kB converted to bytes at parse time.
- **Gokrazy** (`gokrazy/` + iface-dhcp + ntp): ze owns the appliance
  clock (ze's NTP), DHCP wiring per-interface. `ExtraFileContents`
  seeds config at build time.

### Evolution

Interface: [489](489-iface-0-umbrella.md),
[490](490-iface-1-monitor.md), [491](491-iface-2-manage.md),
[492](492-iface-3-bgp-react.md), [493](493-iface-4-advanced.md),
[494](494-iface-5-vm-tests.md),
[522](522-fib-0-umbrella.md),
[523](523-iface-mac-discovery.md),
[524](524-fib-config-autoload.md),
[526](526-iface-backend-split.md),
[557](557-iface-tunnel.md),
[566](566-iface-wireguard.md),
[567](567-iface-tunnel-mac-per-case.md),
[568](568-listener-dynamic-walk.md),
[576](576-gokrazy-1-dhcp-wiring.md),
[582](582-iface-route-priority.md),
[589](589-iface-ipv6-default-route.md),
[615](615-vpp-4-iface.md).

BFD: [555](555-bfd-skeleton.md),
[556](556-bfd-1-wiring.md), [559](559-bfd-2-transport-hardening.md),
[560](560-bfd-3-bgp-client.md), [561](561-bfd-4-operator-ux.md),
[562](562-bfd-5-authentication.md), [563](563-bfd-6-echo-mode.md),
[564](564-bfd-2b-ipv6-transport.md), [565](565-bfd-3b-frr-interop.md).

L2TP + PPP: [594](594-l2tp-1-wire.md),
[595](595-l2tp-2-reliable.md), [596](596-l2tp-3-tunnel.md),
[597](597-l2tp-4-session.md), [599](599-l2tp-5-kernel.md),
[602](602-l2tp-6a-lcp-base.md),
[609](609-l2tp-6b-auth.md), [616](616-l2tp-6c-ncp.md),
[620](620-l2tp-7-subsystem.md), [622](622-l2tp-7b-ci-coverage.md).

VPP: [610](610-vpp-7-test-harness.md),
[611](611-vpp-1-lifecycle.md), [612](612-vpp-6-telemetry.md),
[613](613-vpp-2-fib.md), [614](614-fmt-0-append.md),
[615](615-vpp-4-iface.md), [623](623-fw-9-traffic-lifecycle.md),
[627](627-fw-7-traffic-vpp.md), [629](629-fw-7b-backend-hardening.md).

Firewall: [584](584-fw-1-data-model.md),
[585](585-fw-4-yang-config.md), [586](586-fw-2-firewall-nft.md),
[587](587-fw-3-traffic-netlink.md), [588](588-fw-5-cli.md),
[635](635-fw-10-linux-gaps.md).

Host + Gokrazy: [577](577-gokrazy-2-ntp.md),
[578](578-gokrazy-3-build.md), [579](579-gokrazy-4-resilience.md),
[580](580-gokrazy-0-umbrella.md),
[581](581-sysctl-0-plugin.md), [583](583-sysctl-1-profiles.md),
[631](631-host-0-inventory.md).

### Abandoned approaches

- **BFD `Session`, `Engine`, `Clock` generic type names** (555) —
  collided with existing types project-wide; renamed.
- **L2TP signed-subtraction for sequence ordering** (595) —
  `int16(a-b)<0` mis-classifies diff=32768; use unsigned distance.
- **L2TP XOR loop shared between encrypt and decrypt** (594) — chain
  key is `MD5(secret || prev_ciphertext)`, decrypt needs different
  `prev_ciphertext` source than encrypt.
- **MAC on tunnel kinds applied generically** (567) — non-tunnel
  interfaces use MAC from list-level; tunnel-specific MAC clearing.
- **`prometheus/procfs` for host inventory** (631) — pure stdlib
  parsing of `/proc/*` is ~50 lines and clearer.
- **Host `cpu_capacity` hardcoded values** (631) — use ratio (max vs
  lower), not hardcoded 1024/768.
- **Gokrazy `WaitForClock: true`** (580) — ze owns the clock; waiting
  for it causes boot hang.
- **Firewall `DefaultBackendName` exported** (633) — collided with
  iface/traffic; removed export.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `IP_RECVTTL` cmsg arrives as `IP_TTL` | BFD UDP parser | Setsockopt uses `IP_RECVTTL` (enable flag); cmsg type is `IP_TTL` (value carrier). ([559](559-bfd-2-transport-hardening.md)) |
| DHCPv6 does not install default route | `iface/dhcp.go` | Default gateway comes from Router Advertisements. ([576](576-gokrazy-1-dhcp-wiring.md)) |
| `runtime.LockOSThread()` is mandatory before netns switch | iface tests | Without it, scheduler can move goroutine out of the target namespace. ([494](494-iface-5-vm-tests.md)) |
| `accept_ra` must be `2` not `1` when `forwarding=true` | `iface/sysctl_linux.go` | The kernel ignores Router Advertisements at `accept_ra=1` when forwarding is on. ([489](489-iface-0-umbrella.md), [491](491-iface-2-manage.md)) |
| VLAN composite names must fit IFNAMSIZ (15 chars) | `iface/validateIfaceName` | Combined (parent + `.` + vlan-id), not parent alone. ([489](489-iface-0-umbrella.md), [491](491-iface-2-manage.md)) |
| `SetKernelWorker` must be called BEFORE `reactor.Start()` | L2TP kernel worker | Reactor goroutine reads `r.kernelErrCh`; write after Start races. ([599](599-l2tp-5-kernel.md)) |
| `/proc/meminfo` values are in kB, not bytes | host inventory | Convert at parse time; field names carry `-bytes` suffix. ([631](631-host-0-inventory.md)) |
| `unsafe.Pointer` for RTC ioctl triggers gosec G103 | ntp plugin | Unavoidable for kernel ioctl. Document with `//nolint:gosec` + reason. ([577](577-gokrazy-2-ntp.md)) |

---

## Testing infrastructure

### Current shape

Three test flavors:

- **`test/*/*.ci`** — functional: `stdin=config`, `tmpfs=script.py`,
  `cmd=background/foreground`, `expect=bgp:hex/json/text/contains=`,
  `action=sighup/rewrite`. Python plugin scripts via `test/scripts/ze_api.py`.
- **`test/editor/**/*.et`** — headless TUI replay with keystrokes and
  expectations.
- **Go unit/integration tests** — `_test.go` alongside sources.

Plus fuzz corpora in many packages, `ze-chaos` for chaos, `ze-perf`
for benchmarks.

### Evolution

- **[026](026-spec-self-check-rewrite.md),
  [043](043-spec-functional-test-diagnostics.md),
  [044](044-spec-selfcheck-count.md),
  [045](045-spec-functional-decoding-parsing.md),
  [131](131-ci-format-cleanup.md),
  [132](132-spec-test-cmd-consolidation.md),
  [135](135-tmpfs-format.md),
  [206](206-unify-test-tools.md),
  [339](339-config-completion-cli.md)** `.ci` format evolution.
  ExaBGP-inspired runner rewritten in Go. `stdin=`, `tmpfs=`,
  `cmd=background/foreground` for self-contained tests. Test IDs
  alphanumeric (0-9, A-Z, a-z) stable across runs. `ze-test bgp
  parse/ui/encode/decode/plugin`.
- **[205](205-editor-testing-framework.md),
  [428](428-command-history.md)** `.et` framework. Key sequences
  + expected viewport/completions. Extended with `option=session:user=X`,
  `expect=file:path=X:contains=Y`, `restart=` for multi-session and
  persistence tests.
- **[255](255-bgp-chaos-session.md) through
  [265](265-bgp-chaos-selftest.md)** `ze-chaos` tool.
  Seed-deterministic. Multi-peer scenarios, chaos events (flap, timer
  expiry, malformed message), validation model, NDJSON event log,
  property-based testing with shrinking, in-process mode with
  VirtualClock.
- **[274](274-spec-test-diagnostics.md)** `.ci` diagnostics. Field-
  level JSON diff, named suite failures, parse test reproduction
  commands, full hex in debug commands.
- **[338](338-codebase-review-test-coverage.md),
  [354](354-nlri-test-coverage.md),
  [355](355-test-coverage.md),
  [393](393-ci-gaps.md),
  [446](446-feature-inventory-ci-gaps.md),
  [550](550-ci-observer-exit-code-fix.md),
  [558](558-ci-observer-per-test-audit.md)** Test coverage audits.
  Fuzz targets across wire parsers. Observer-exit antipattern
  (`sys.exit(1)` after `daemon shutdown` → test framework sees
  "success") replaced with `runtime_fail(msg)` pattern.
- **[608](608-concurrent-test-patterns.md)** Concurrent-test flake
  patterns. Locked-write/unlocked-read, subscribe-before-broadcast,
  gate-handler, barrier FIFO, cleanup-drains-work.

### Abandoned approaches

- **`sys.exit(1)` after `daemon shutdown` in Python observers** (550,
  558) — test framework sees the daemon's successful exit, observer
  failure is lost. Use `runtime_fail(msg)` which emits a valid slog
  line at ERROR level.
- **Drops as "acceptable flake"** (562, 604) — race detector missing a
  race does not prove the race does not exist. Fix the code when memory
  model says "race".
- **Count-only assertions** (340, 360, 400, 446, etc.) — parsing can
  produce accidentally-matching counts through data corruption.
  Assert on content (keys, values, wire bytes).
- **`go test ./...` from module root when `tmp/*.go` exists** (557,
  610, 619) — scratch files break unit-test phase. Research subagents
  must use `.txt` or build-tagged directories.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| `cmd=api` lines in `.ci` files are documentation metadata, not execution directives | `internal/test/runner/` | The route must come from the plugin. ([362](362-flaky-watchdog-test.md), [414](414-forward-barrier.md), [483](483-exabgp-bridge-muxconn.md)) |
| `expect=stderr:contains=` only fires inside `ExpectExitCode != nil` branch | `runner_exec.go` | Without `expect=exit:code=`, runner falls through to peer-wait path. ([623](623-fw-9-traffic-lifecycle.md)) |
| Plugin subprocess stderr is consumed by `relayStderrFrom()` | `process.go` | Never reaches test runner's stderr match. ([451](451-rib-show-filters.md)) |
| Background `.ci` processes do NOT get `ZE_READY_FILE` | `runner_exec.go:705-717` | Only foreground path writes daemon.pid + daemon.ready. ([623](623-fw-9-traffic-lifecycle.md)) |
| `parse/` test runner only extracts `stdin=config` and runs `ze validate` | `internal/test/runner/` | `cmd=foreground`, `tmpfs=`, `expect=stdout:contains=` are silently skipped. ([449](449-strip-private.md)) |
| Python test library (`test/scripts/ze_api.py`) must track Go protocol | Python SDK | Changing engine RPC without updating Python hangs 129+ tests. ([291](291-batched-ipc-delivery.md), [397](397-unified-rpc-framing.md), [497](497-check-ci-slowness.md)) |
| Stored test state (registry, `sync.Once`) must Snapshot/Restore | `t.Cleanup` | `Reset` empties all globally registered decoders; fresh registry breaks tests. ([240](240-plugin-engine-decode.md), [533](533-bgp-boundary-cleanup.md)) |
| `net.Pipe()` deadlocks sequential write-then-read | test setup | Zero buffering. Wrap writes in goroutines or start reader first. ([210](210-yang-ipc-plugin.md), [264](264-bgp-chaos-inprocess.md), [459](459-plugin-tcp-transport.md), [609](609-l2tp-6b-auth.md)) |

---

## Migration: ExaBGP to Ze

### Current shape

Two separate tools under `internal/exabgp/`, isolated from engine:

- `ze exabgp migrate` — one-shot config converter (ExaBGP → Ze).
- `ze exabgp plugin` — runtime bridge wrapping ExaBGP plugins for use
  with Ze's 5-stage protocol.

Engine code: zero ExaBGP format awareness.

### Evolution

- **[001](001-initial-implentation.md),
  [008](008-config-migration-system.md),
  [041](041-spec-format-based-migration.md),
  [096](096-exabgp-migration-tool.md),
  [125](125-exabgp-compat.md),
  [219](219-exabgp-feature-parity.md)** Migration tool. Heuristic
  version detection. Three-version chain. Named transformations.
  Plugin bridge: handles 5-stage internally, switches to JSON
  translation after `ready`.
- **[179](179-remove-unrequested-features.md),
  [181](181-remove-exabgp-announce.md),
  [183](183-remove-exabgp-syntax.md),
  [281](281-remove-ze-syntax.md),
  [344](344-remove-announce-route.md)** ExaBGP syntax removal from
  engine. `announce { }`, `static { }`, `operational { }` blocks
  deleted. `multi-session`, `operational`, `aigp` capabilities
  rejected during migration.

### Abandoned approaches

- **ExaBGP tests built with Ze syntax as input** (125) — tests passed
  without exercising migration. Rebuilt with real ExaBGP fixtures.
- **Full `dual-registration` for `daemon-*` RPCs** (228) — replaced
  atomically; ze has no users, no compat needed.

### Load-bearing invariants

| Invariant | Site | Why (first occurrence) |
|-----------|------|------------------------|
| All ExaBGP-aware code lives in `internal/exabgp/` | package boundary | No imports from other packages. ([181](181-remove-exabgp-announce.md)) |
| Complex NLRI families (FlowSpec, MVPN, MUP) are API-only, not config | config schema | Removed from config for simplicity. ([181](181-remove-exabgp-announce.md)) |
| ExaBGP text protocol plugins bridged at the boundary | `ze exabgp plugin` | Incompatible with Ze's NUL-framed JSON-RPC. ([219](219-exabgp-feature-parity.md), [483](483-exabgp-bridge-muxconn.md)) |

---

## Cross-cutting: reading this document in practice

- **"Why is component X structured this way?"** — find X in the
  section index above; the Evolution subsection lists the
  summaries in order. Read the oldest first to see what was being
  rejected; read the newest first to see the current state.
- **"Can I propose approach Y?"** — search "Abandoned approaches"
  for Y. If Y appears, the learned summary will explain why it did
  not work. Read it before proposing.
- **"Why does the code require invariant Z?"** — search "Load-bearing
  invariants" for Z. The table names the enforcement site and the
  summary that records the reasoning.
- **Still unclear?** — fall back to `../../ai/LEARNED-INDEX.md`
  for the curated topic index, then read the specific summary.

The summaries under `plan/learned/NNN-*.md` remain the authority.
This document exists so a future session does not need to read all
638 to understand why the code looks the way it does.
