# Spec: BGP one-off defects — 2: BGP plugins

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Each item repairs incorrect shipped BGP plugin behavior (RIB, Adj-RIB-In, RS, RR, BMP, RPKI, filter plugins, NLRI plugins, and BGP command plugins) owned under `ai/doors/bgp-plugins.md` |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-bgp-one-off-defects.md`. This child owns the 24 open BGP plugin journal rows under `ai/doors/bgp-plugins.md` (`internal/component/bgp/plugins/*`).

## Task

Repair the remaining open one-off defects across BGP plugins (`cmd/peer`, `cmd/announce`, `cmd/update`, `filter_community`, `filter_prefix`, `rib`, `adj_rib_in`, `rs`, `rr`, `bmp`, `rpki`, `nlri/*`), after retiring already-fixed plugin rows (`AC-6`, `AC-59`, `AC-60`, `rpki` cache-restart race) in the parent umbrella. Every defect follows TDD: reproduce red at HEAD, repair on the process-safe plugin rails, verify green, and close the journal row with `fixed: `.

Standing architectural rules applied to every item here:
- **Process-boundary parity (`docs/architecture/plugin/plugin-system.md`, `docs/architecture/api/process-protocol.md`)**: every plugin fix works both internal and forked. Transient issues raised by forkable plugins go through `sdk.Plugin.RaiseIssue` via `issuequeue` (never direct `internal/core/report` calls and never synchronous on hot or lock-holding loops), register every code with `diagnostic.Register` and `diagnostic.RegisterEventCode`, and avoid unguarded `dangerousCalls`.
- **EventBus `Sends` parity**: any EventBus event a plugin emits is declared in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`.
- **Ze always starts (2026-11-26 ruling, `docs/architecture/core-design.md`)**: no plugin configuration check added here exits `ze start`.
- **Functional tests live in `test/plugin/*.ci`**: plugin `.ci` tests live under `test/plugin/`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, public `pkg/` API, compile-out boundaries
  → Constraint: every plugin fix survives running forked over TLS RPC with zero shared-memory shortcuts.
- [ ] `ai/doors/bgp-plugins.md` - registration parity, process boundary, doctor check routing, filter insertions, and barriers
  → Constraint: `registry.Registration` and Stage 1 `sdk.Registration` declarations always agree; no plugin imports another plugin's implementation.
- [ ] `docs/architecture/plugin/plugin-system.md` - process boundary, communication patterns, and event/command contracts
  → Constraint: forkable plugins report transient issues through `sdk.Plugin.RaiseIssue` via `issuequeue` rather than calling `internal/core/report` directly.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB pool, VPN/labeled/SRv6 storage, and best-path formatting
  → Constraint: `ExtractSRv6SIDFull` and `formatNLRIAsPrefix` preserve family-specific RFC layouts.

### RFC Summaries (Scope: plugin)
- [ ] `rfc/short/rfc9003.md` - Section 2 & 3 (Shutdown Communication length and UTF-8 rules)
- [ ] `rfc/short/rfc7999.md` - Section 3.1 (BLACKHOLE community agreement)
- [ ] `rfc/short/rfc6514.md` - Section 4.4 (MVPN Leaf A-D type 4 layout without RD)
- [ ] `rfc/short/rfc9252.md` - Sections 5 & 7 (SRv6 L3 Service TLV vs L2 Service TLV)
- [ ] `rfc/short/rfc9552.md` - Table 7 (BGP-LS Node Flag Bits bit 6 `A`)
- [ ] `rfc/short/rfc9830.md` - Section 4.2.1 (SR Policy receive validation)

**Key insights:**
- `AC-63` (`declareShapes` in `internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go`) touches only `adj_rib_in` and delegates to the existing `command.PluginShapesFromDecls` helper.
- `AC-59` (`adj-rib-in` JSON ingest `handleReceived` VPN RD keying) and `AC-6` (`resendRoutesWithCursor` refused count) are already fixed at HEAD and retired in the parent umbrella.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go`, `admin.go`, `notify.go` - `handleTeardown` truncates a > 128-octet Shutdown Communication via `message.BuildShutdownData`, while `handlePeerNotify` refuses it via `message.BuildTeardownData`.
- [ ] `internal/component/bgp/plugins/filter_community/blackhole.go` - `blackholeReadvertiseRefused` inspects received payload bytes rather than the egress-modified attributes Ze will send.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `genericCommunityHandler` Add buffer validation gap.
- [ ] `internal/component/bgp/plugins/filter_prefix/config.go` - `ge`/`le` validation combination gap and `parsePrefixListEntries` multi-entry test input shape mismatch.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go`, `bmp_events.go` - `dedupState` per-peer hash set suppresses a contradicting UPDATE whose body matches an earlier one (A, B, A).
- [ ] `internal/component/bgp/plugins/rr/rr.go` - `isConnectionError` matches only three strings, so `no established peers to forward to` logs at `ERROR` in `reportUpdateFailure`.
- [ ] `internal/component/bgp/plugins/rs/source_lifetime.go` - `rs.unknownOutputs` keeps an `errAdjOutSession` refusal permanently, failing every later settlement and `request quiesce`.
- [ ] `internal/component/bgp/plugins/rib/pool/srv6sid.go` - `ExtractSRv6SIDFull` reads both `tlvTypeSRv6L3Service` and `tlvTypeSRv6L2Service` without distinguishing VPN (L3) from EVPN (L2).
- [ ] `internal/component/bgp/plugins/rib/rib.go`, `rib_recovery.go`, `rib_commands.go`, `rib_nlri.go` - JSON rail SAFI 4 label handling, `recoveryRoutes` destinations not attached to `bgp-rib`, `show bgp rib best` VPN RD formatting, and received `LARGE_COMMUNITY` display in `rib show`.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go`, `rib_shape_test.go` - `applyDecision` WARN on a withdrawn pending route, and `declareShapes` duplicating `command.PluginShapesFromDecls`.
- [ ] `internal/component/bgp/plugins/nlri/ls/attr_node.go` - `lsNodeFlagBits.ToJSON` emits `"T"` instead of RFC 9552 Table 7 `"A"` for bit 6.
- [ ] `internal/component/bgp/plugins/nlri/mvpn/types.go` - `parseBody` attempts to read an 8-octet RD for all non-`bodyParsed()` route types including Leaf A-D type 4.
- [ ] `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go`, `internal/component/bgp/plugins/nlri/evpn/plugin.go`, `internal/component/bgp/plugins/cmd/announce/announce.go`, `internal/component/bgp/plugins/cmd/update/update_text_test.go` - legacy `destination` keyword refusal, `DecodeNLRIHex` discarding trailing `rest` bytes, and `announceAndTrack` lifetime handling.

**Behavior to preserve:**
- All existing registered commands, YANG schemas, Stage 1 declarations, DirectBridge/external RPC parity, and already-retired plugin fixes (`AC-6`, `AC-59`).

**Behavior to change:**
- Only the 16 plugin ACs listed in `## Acceptance Criteria` below.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shutdown Communication builder | `internal/component/bgp/message` `BuildTeardownData` / `BuildShutdownData` | Validates or truncates RFC 9003 Shutdown Communication text | Extends: AC-1 unifies `handleTeardown` and `handlePeerNotify` onto one shared contract |
| Shared command answer shape conversion | `internal/component/command/answer_shape.go` `command.PluginShapesFromDecls` | Single conversion from plugin command declarations to answer shapes | Extends: AC-14 replaces the duplicate loop in `adj_rib_in/rib_shape_test.go` `declareShapes` |
| Plugin report-bus issue queue | `pkg/plugin/sdk` `Plugin.RaiseIssue`; `internal/core/diagnostic` `Register` & `RegisterEventCode` | Delivers transient plugin issues asynchronously across the process boundary | Extends: any new plugin `Warn` raising a transient issue routes through `RaiseIssue` via `issuequeue` with a registered code |
| Filter community egress evaluation | `internal/component/bgp/plugins/filter_community/blackhole.go` `blackholeReadvertiseRefused`; `handler.go` `genericCommunityHandler` | Enforces RFC 7999 BLACKHOLE agreement and community add/remove edits | Extends: AC-2 checks egress-modified attributes; AC-12 bounds `genericCommunityHandler` Add buffer |
| BGP-RS retained settlement | `internal/component/bgp/plugins/rs/source_lifetime.go`, `retained_settlement.go` | Tracks per-target unknown outcomes across quiesce and settlement | Extends: AC-15 distinguishes definite `errAdjOutSession` non-delivery from unknown outcomes |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Every fix keeps full internal and external process-boundary parity with zero shared-memory shortcuts. |
| `docs/architecture/api/commands.md` | Overview | the // Design: page of internal/component/bgp/plugins/cmd/peer/peer.go and anchored plugin command files | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Command repairs preserve verb-first paths, YANG parameter validation, and shared answer-shape conversion. |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to internal/component/bgp/plugins/cmd/peer/peer.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering internal/component/bgp/plugins/* | "**Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component.**" | agrees | Every BGP plugin fix communicates exclusively over the bus, DirectBridge, or text commands and keeps Stage 1 declarations symmetric. |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to internal/component/bgp/plugins/cmd/peer/notify.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/bgp/on-demand-origination.md` | The decisions | the // Design: page of internal/component/bgp/plugins/cmd/announce/announce.go | "**Announce uses the existing local build, export and socket path.**" | agrees | `announce ... for` lifetime repairs extend the existing origination path without adding a parallel path. |
| `docs/architecture/core-design.md` | Executive Summary | the // Design: page of internal/component/bgp/plugins/filter_community/blackhole.go and sibling plugin files | "`ze` and `le` are personalities of the single `cmd/ze` codebase." | agrees | All repairs stay inside the modular component/plugin architecture without breaking component isolation or startup resilience. |
| `docs/architecture/meta/filter-community.md` | Keys Set | anchored to internal/component/bgp/plugins/filter_community/blackhole.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | anchored to internal/component/bgp/plugins/filter_community/handler.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/birdwatcher-compat.md` | Status of this document | anchored to internal/component/bgp/plugins/bmp/bmp.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to internal/component/bgp/plugins/rr/rr.go; plugins/nlri/flowspec/plugin_encode_text.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to internal/component/bgp/plugins/rr/rr.go; plugins/rib/rib.go | "Ze plugins communicate with the engine via newline-framed YANG RPCs" | agrees | All plugin fixes stay on the newline-framed YANG RPC and DirectBridge contracts and route transient issues via `sdk.Plugin.RaiseIssue`. |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to internal/component/bgp/plugins/rr/rr.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/plugin/plugin-relationships.md` | 1. System Plugins | anchored to internal/component/bgp/plugins/rr/rr.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | the // Design: page of internal/component/bgp/plugins/rs/source_lifetime.go and anchored RIB files | "This is the **implementation reference** for Pool + Wire design in API programs:" | agrees | RIB storage, VPN RD rendering, SAFI 4 JSON rail, and SRv6 SID extraction preserve the existing pool and wire ownership design. |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to internal/component/bgp/plugins/rr/rr.go; rib_recovery.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to internal/component/bgp/plugins/rs/source_lifetime.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to internal/component/bgp/plugins/rib/rib.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | The Question | anchored to internal/component/bgp/plugins/rib/rib.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/doctor-and-health-checks.md` | BGP policy absences in the supplied configuration | anchored to internal/component/bgp/plugins/rib/rib.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to internal/component/bgp/plugins/rib/rib.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to internal/component/bgp/plugins/rib/rib.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/route-selection.md` | Overview | anchored to internal/component/bgp/plugins/rib/rib.go; rib_recovery.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/buffer-architecture.md` | Executive Summary | anchored to internal/component/bgp/plugins/rib/rib_recovery.go | "Ze uses a **buffer-first** architecture where BGP messages are represented as byte buffers with iterators and partial parsers." | agrees | Plugin repairs preserve buffer-first encoding and bounded pooled lifetimes. |
| `docs/architecture/mrt.md` | Packages | anchored to internal/component/bgp/plugins/rib/rib_recovery.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to internal/component/bgp/plugins/rib/rib_commands.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Where the SID comes from | the // Design: page of internal/component/bgp/plugins/rib/pool/srv6sid.go | "The SID reaches the FIB through the BGP **Prefix-SID attribute**, not through a" | agrees | AC-7 restricts VPN SRv6 SID extraction to the L3 Service TLV per RFC 9252 Section 5 without altering FIB programming. |
| `docs/architecture/bgp/structural-forwarding.md` | The ordering constraint | anchored to internal/component/bgp/plugins/adj_rib_in/rib_validation.go | "**`extractWireNLRIRecords` MUST run before forwarding.**" | agrees | Adj-RIB-In validation and RS settlement repairs preserve pre-forwarding extraction order. |
| `docs/architecture/wire/nlri-flowspec.md` | Wire Format Overview | the // Design: page of internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/nlri-bgpls.md` | MP_REACH next hop | the // Design: page of internal/component/bgp/plugins/nlri/ls/attr_node.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/wire/bgpls-attribute-naming.md` | Naming Rules | anchored to internal/component/bgp/plugins/nlri/ls/attr_node.go | - | silent | This page is anchored via shared source files and is silent on the specific defect repairs in this BGP plugin defect pass; its documented invariants are preserved untouched. |
| `docs/architecture/api/text-format.md` | Current Format | anchored to internal/component/bgp/plugins/nlri/mvpn/types.go | "All parsed text events start with `peer <address> remote as <asn>`." | agrees | MVPN Leaf A-D type 4 RD parsing aligns `MVPN.String()` with RFC 6514 Section 4.4 without altering text-event framing. |
| `docs/architecture/wire/nlri-evpn.md` | Common Header | the // Design: page of internal/component/bgp/plugins/nlri/evpn/plugin.go | "All EVPN NLRIs share this structure:" | agrees | EVPN `DecodeNLRIHex` refuses trailing unparsed bytes while preserving the documented common header and route type framing. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering test/plugin/blackhole-policy-add-refused.ci | "**A compiled Go fixture returns an error when an assertion fails** (`ai/patterns/functional-test.md`, `ai/rules/testing.md`)." | agrees | Every functional `.ci` test created or modified asserts on live state and fails closed on unmet assertions. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Preserved unchanged: no command path or pipe grammar changes here. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Preserved unchanged: pipe override semantics are untouched. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Preserved unchanged: pipe side-effect isolation is untouched. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | Preserved unchanged: ExaBGP scripts remain external bus endpoints. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | Preserved unchanged: ExaBGP base64 UPDATE transport is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Preserved unchanged: these BGP plugin repairs touch no data-plane backend selector. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Preserved unchanged (as amended for `ze start` by the 2026-11-26 decision below): these BGP plugin repairs touch no data-plane mismatch check. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Preserved unchanged: these BGP plugin repairs touch no VPP firewall verifier. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Preserved unchanged: these BGP plugin repairs touch no data-plane backend selector. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Preserved unchanged: export-chain next-hop rewriting remains untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Preserved unchanged: export-chain next-hop authoring forms remain untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Preserved unchanged: built-in export next-hop steps remain untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Preserved unchanged: next-hop folding and provenance checks remain untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Preserved unchanged: explicit next-hop import/export validation remains untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Preserved unchanged: locally originated `announce` routes preserve their explicit next hop. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Preserved unchanged: implied filter publication remains intact. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Honored directly: no BGP plugin configuration refusal added here exits `ze start`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Honored directly: startup resilience holds across all plugin repairs. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Preserved unchanged: `\| implied` rendering is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Preserved unchanged: all plugin repairs stay on existing bus participant registrations and keep `Sends` symmetric between `registry.Registration` and Stage 1 `sdk.Registration`. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Preserved unchanged: bus destination filtering semantics are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Preserved unchanged: bus receive-source filtering semantics are untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Preserved unchanged: bus message allow-list enforcement is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Preserved unchanged: bus filter compilation is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Preserved unchanged: bus participant non-arbitration is untouched. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Preserved unchanged: bus default/effective display is untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Preserved unchanged: engine operation message declarations and authorization profiles are untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Honored directly: plugin event consumers preserve lossless delivery without dropping routes. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Preserved unchanged: Stage 1 declare-mode purity is untouched. |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Preserved unchanged: FlowSpec text keyword refusal does not alter `flowspec-redirect-next-hop` opt-in behavior. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- CLI/API commands (`request peer teardown/notify`, `announce ... for`, `show bgp rib best`, `request quiesce`).
- EventBus/DirectBridge UPDATE and state deliveries into `bgp-rib`, `adj-rib-in`, `bgp-rs`, `bgp-rr`, `bgp-bmp`, `bgp-rpki`, and `filter_*` plugins.

### Transformation Path
1. Plugin receives command RPC, `filter-update` batch, or structured/JSON UPDATE event over DirectBridge or TLS RPC.
2. Plugin validates inputs (`DecodeNLRIHex`, `parsePrefixListEntries`, `BuildTeardownData`), updates plugin state (`RIBManager`, `AdjRIBInManager`, `routeServer`, `bmpPlugin`), and emits commands or BMP frames.
3. Any transient operator diagnostic is queued asynchronously via `sdk.Plugin.RaiseIssue` (`issuequeue`) under a registered `diagnostic.EventCode`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ BGP plugin (internal or forked) | DirectBridge / TLS RPC (`pkg/plugin/sdk`, `pkg/plugin/rpc`) | No |
| BMP plugin ↔ BMP collector | TCP BMP Route Monitoring messages (`plugins/bmp/`) | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/plugins/{bmp,nlri/ls,nlri/mvpn,nlri/flowspec,nlri/evpn,cmd/peer,cmd/update,rib}` for `dedupState`, `lsNodeFlagBits`, `parseBody`, `DecodeNLRIHex`, `BuildShutdownData`, and `destination-ipv4`; positive controls confirmed in `plugins/bmp/bmp_events.go`, `plugins/nlri/ls/attr_node.go`, `plugins/nlri/mvpn/types.go`, `plugins/cmd/peer/peer.go`, and `plugins/cmd/update/update_text_test.go`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| BMP Route Monitoring stream (RFC 7854) | writer | `internal/component/bgp/plugins/bmp/bmp_events.go` `handleSenderUpdate` | Emits Route Monitoring messages to collectors after dedup check | Change: emit contradicting A → B → A sequence without suppressing the second A |
| BMP Route Monitoring stream (RFC 7854) | reader | External BMP collectors and `test/plugin/` BMP tests | Collectors track latest route state per peer | Preserve: collectors require the restoring A message after B |
| BGP-LS Node Flag Bits JSON (`lsNodeFlagBits.ToJSON`) | writer | `internal/component/bgp/plugins/nlri/ls/attr_node.go` `lsNodeFlagBits.ToJSON` | Serializes TLV 1024 node flags into plugin JSON events and decode output | Change: rename bit 6 JSON key from `"T"` to `"A"` per RFC 9552 Table 7 |
| BGP-LS Node Flag Bits JSON (`lsNodeFlagBits.ToJSON`) | reader | External JSON API/event consumers and `internal/component/bgp/plugins/nlri/ls/*_test.go` | Reads `"node-flags"` map keys | Change: update tests and consumers expecting `"T"` to `"A"` in the same change |
| FlowSpec update-text grammar (`destination-ipv4`/`source-ipv4`) | writer | Operators, `internal/plugins/ddos/flowspec/responder.go`, `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` | Formats and sends FlowSpec update-text commands | Change: refuse bare `destination`/`source` with error naming `destination-ipv4`/`source-ipv4` |
| FlowSpec update-text grammar (`destination-ipv4`/`source-ipv4`) | reader | `internal/component/bgp/plugins/cmd/update/` `ParseUpdateText` and `plugins/nlri/flowspec/plugin_encode_text.go` | Tokenizes FlowSpec match keywords | Change: ensure consistent refusal message across both parsers |

### Integration Points
- `internal/component/command/answer_shape.go` `command.PluginShapesFromDecls` - reused by `adj_rib_in/rib_shape_test.go`.
- `pkg/plugin/sdk` `Plugin.RaiseIssue` & `internal/core/diagnostic` - process-safe diagnostic reporting.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | All plugin fixes stay inside `internal/component/bgp/plugins/*` |
| No unintended coupling (components stay isolated) | No | Zero cross-plugin package imports added (`./le arch tier check`) |
| No duplicated functionality (extends existing, does not recreate) | No | AC-14 deletes the duplicate loop in `adj_rib_in/rib_shape_test.go` |
| Zero-copy preserved where applicable (refs, not copies) | No | No new hot-path allocations in `rib`, `bmp`, `rs`, or `filter_community` |
| Registration over hardcoding, outbound | No | `registry.Registration` and Stage 1 `sdk.Registration` declarations stay symmetric |
| Registration over hardcoding, inbound | No | No core switch learns any plugin name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every listed plugin defect still reproduces at HEAD | Audited against HEAD source on 2026-11-26 | Retire any row whose red-first test passes at HEAD into `## Existing Mechanisms` | Red-first test per AC | unvalidated |
| A-2 | BMP dedup memo can track last-sent state per route/peer without unbounded memory | `maxDedupPerPeer` bound already exists in `plugins/bmp/bmp.go` | Evict or key per prefix state rather than an unordered body-hash set | Unit test driving A, B, A sequence | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing `lsNodeFlagBits.ToJSON` `"T"` to `"A"` breaks existing JSON tests | `go test ./internal/component/bgp/plugins/nlri/ls/...` fails | Update repository tests and fixtures in the same change per `ai/META-DECISIONS.md` (with `ze-test-judge` for any RFC-tagged test) |
| R-2 | Raising report-bus diagnostics directly from a plugin breaks forked mode | `./le plugin boundary check` fails | Always use `sdk.Plugin.RaiseIssue` via `issuequeue`, never direct `internal/core/report` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Incorrect BMP monitoring, RIB display/recovery, BLACKHOLE filtering, or route-server quiesce failures |
| How is it reverted? | Single commit revert per plugin AC increment |
| Who else touches this path? | `spec-dropped-error-2-bgp-plugins.md`, `spec-bmp-sflow-export-rfc-defects.md`, `spec-srv6-bestpath-resolvability.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `request peer <sel> teardown` and `notify` with 129-octet text | → | `handleTeardown`, `handlePeerNotify` (`plugins/cmd/peer/`) | `TestTeardownAndNotifyShareShutdownContract` |
| Policy adding `65535:666` toward unagreed eBGP peer | → | `blackholeReadvertiseRefused` (`plugins/filter_community/blackhole.go`) | `test/plugin/blackhole-policy-add-refused.ci` |
| BMP receiving UPDATE sequence A, B, A | → | `bmpPlugin.handleSenderUpdate` (`plugins/bmp/bmp_events.go`) | `TestBMPDedupEmitsContradictingSequenceABA` |
| `request quiesce` after a target returns `errAdjOutSession` | → | `routeServer` settlement (`plugins/rs/source_lifetime.go`) | `TestRSRetiredSessionRefusalDoesNotStickInUnknownOutputs` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Bare `destination`/`source` keyword passed to FlowSpec config or `update text` (`plugins/nlri/flowspec/plugin_encode_text.go`, `plugins/cmd/update/`, old AC-1 remaining rows) | Refused with an operator error naming `destination-ipv4`/`source-ipv4` (or `-ipv6`); all FlowSpec tests in both packages pass using the family-qualified keywords | Family-qualified keywords (`destination-ipv4`, `source-ipv4`, `destination-ipv6`, `source-ipv6`) continue to parse identically |
| AC-2 | Same 129-octet or invalid UTF-8 Shutdown Communication sent via `request peer teardown` and `request peer notify` (`plugins/cmd/peer/peer.go`, `admin.go`, `notify.go`, old AC-3) | Both commands apply the single RFC 9003 Shutdown Communication length/UTF-8 contract and return the same refusal | Valid <= 128-octet UTF-8 communications succeed on both commands |
| AC-3 | Export policy or filter adds `65535:666` (BLACKHOLE) toward an unagreed eBGP peer (`plugins/filter_community/blackhole.go` `blackholeReadvertiseRefused`, old AC-16) | RFC 7999 gate evaluates the egress attributes Ze will transmit and withholds the route; `test/plugin/blackhole-policy-add-refused.ci` observes no send | Agreed eBGP peers, iBGP peers, and route-server clients continue to receive BLACKHOLE routes per RFC 7999 §3.1 |
| AC-4 | Maximum-paths reload test (`bgp-rib-maximum-paths-reload.ci`, `plugins/rib/`, old AC-22) | Passes standalone on a clean HEAD build with root cause repaired in the plugin reload path | Existing single-path and unchanged multi-path sessions remain unaffected |
| AC-5 | BMP receives UPDATEs A, B, A for the same peer where A's body repeats after contradicting update B (`plugins/bmp/bmp_events.go`, `bmp.go`, old AC-25) | Emits all three Route Monitoring messages (A, B, A) so collectors reflect current route state | True consecutive duplicate bodies without an intervening contradicting change may still be deduplicated |
| AC-6 | `bgp-rr` forwards when no peers are established (`plugins/rr/rr.go` `isConnectionError`, `reportUpdateFailure`, old AC-27 rr half) | Records the transient empty-destination condition at `Debug` (or `Warn` with a registered doctor code) and returns cleanly rather than escalating to `ERROR` | Real transport or internal errors continue to log at `Warn`/`ERROR` with declared codes |
| AC-7 | VPN (L3) route carrying Prefix-SID with both L2 and L3 Service TLVs (`plugins/rib/pool/srv6sid.go` `ExtractSRv6SIDFull`, old AC-31) | Extracts the SRv6 Service SID for a VPN route from the L3 Service TLV only per RFC 9252 Section 5; EVPN (L2) routes use L2 Service TLV | Valid L3 TLVs on VPN routes and L2 TLVs on EVPN routes keep extracting their SIDs |
| AC-8 | Source peer goes down with routes previously sent to destinations not attached to `bgp-rib` (`plugins/rib/rib_recovery.go` `recoveryRoutes`, old AC-32) | Sends WITHDRAW to affected destinations or logs the omission under a declared doctor code (via `sdk.Plugin.RaiseIssue` when transient); `.ci` verifies withdrawal | Attached `bgp-rib` destinations retain their existing recovery path |
| AC-9 | JSON rail receives SAFI 4 labeled routes or an unsplit family (`plugins/rib/rib.go` `insertPoolNLRIs`, `removePoolNLRIs`, old AC-33) | JSON rail binds and withdraws labels matching the structured rail; a family with no splitter is refused or warned under a declared doctor code, never silently dropped | Structured-rail SAFI 4 behavior stays identical |
| AC-10 | `show bgp rib best` for VPN routes and `rib show` for received `LARGE_COMMUNITY` (`plugins/rib/rib_nlri.go`, `rib_commands.go`, old AC-36 VPN half & AC-43 LARGE_COMMUNITY) | VPN route renders as RD plus prefix (never misparsed as `/112` IPv4); `rib show` displays received `LARGE_COMMUNITY` values | Standard IPv4/IPv6 unicast rendering and other attribute fields remain unchanged |
| AC-11 | Plugin discarded values (`plugins/cmd/announce/announce.go` `announceAndTrack` lifetime, `plugins/nlri/*/DecodeNLRIHex` trailing `rest` bytes, `plugins/nlri/ls/attr_node.go` `lsNodeFlagBits.ToJSON`, old AC-44 plugin items) | `announce ... for` enforces the configured lifetime; `DecodeNLRIHex` refuses trailing unparsed bytes; `lsNodeFlagBits.ToJSON` emits RFC 9552 Table 7 key `"A"` for bit 6 | Valid exact-length hex inputs, non-timed announcements, and all other BGP-LS node flag keys (`O`, `E`, `B`, `R`, `V`) remain unchanged |
| AC-12 | Community Add buffer guard and prefix-list `ge`/`le` validation & multi-entry test shape (`plugins/filter_community/handler.go` `genericCommunityHandler`, `plugins/filter_prefix/config.go`, `config_test.go`, old AC-53 plugin items & AC-56) | `genericCommunityHandler` checks Add `Buf` bounds; prefix-list validates all `ge`/`le` combinations; multi-entry test feeds the exact shape `ParseBGPSubtree` produces | Valid community edits and prefix-list entries keep identical behavior |
| AC-13 | MVPN Leaf A-D route (type 4) decoded by `parseBody` (`plugins/nlri/mvpn/types.go`, old AC-58 mvpn) | Per RFC 6514 Section 4.4, `parseBody` reads an RD only for route types whose wire layout carries one, and `String()` reports no RD for type 4 | Route types whose layout carries an RD (types 1, 2, 3, 5, 6, 7) keep parsing and displaying their RD |
| AC-14 | `declareShapes` in `internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go` (old AC-63) | Calls `command.PluginShapesFromDecls` directly with no duplicated loop; existing adj-rib-in shape assertions stay green | All existing answer shapes and column assertions remain unchanged |
| AC-15 | `bgp-rs` target returns `errAdjOutSession` refusal during settlement (`plugins/rs/source_lifetime.go`, `retained_settlement.go`, journal 2026-11-20) | Treats definite retired-session refusal as non-delivery rather than keeping it permanently in `rs.unknownOutputs`, so subsequent settlements and `request quiesce` succeed | Genuine unknown writer outcomes remain recorded in `rs.unknownOutputs` until settled |
| AC-16 | Remaining open plugin journal rows (`adj-rib-in` `applyDecision` WARN on withdrawn pending route 2026-10-23, `rpki-aspa-*` EOR timeout 2026-10-31, and plugin weak RFC tags `RFC9830-4.2.1-2/-6` and `RFC9252-7-1/-2`) | `applyDecision` handles withdrawn pending routes without spurious `WARN`; `rpki-aspa-*` receives Ze's EOR promptly; RFC 9830 and RFC 9252 tests prove their exact RFC claims (with `ze-test-judge` approval for any RFC test change) | Genuine validation errors and live pending decisions remain logged and enforced |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show bgp rib best` with VPN routes and received `LARGE_COMMUNITY` | `cli` → `plugin/server` → `bgp-rib` `rib_commands.go` / `rib_nlri.go` | `TestShowBGPRIBBestFormatsVPNAndLargeCommunity` |
| 2 | Adds BLACKHOLE via export policy toward an unagreed eBGP peer | `policychain` → `filter_community` `blackholeReadvertiseRefused` | `test/plugin/blackhole-policy-add-refused.ci` |
| 3 | Runs `request quiesce` on a route server after a peer session rotated | `cmd` → `bgp-rs` `retained_settlement.go` | `TestRSRetiredSessionRefusalDoesNotStickInUnknownOutputs` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Red-first unit tests for AC-1..AC-16 | `internal/component/bgp/plugins/{cmd/peer,cmd/announce,cmd/update,filter_community,filter_prefix,bmp,rr,rs,rib,adj_rib_in,rpki,nlri/*}/*_test.go` | Each plugin AC fails before repair and passes under `-race` after | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RFC 9003 Shutdown Communication length | 0–128 octets | 128 | N/A | 129 |
| Prefix-list `ge`/`le` bounds (IPv4 / IPv6) | prefixLen–32 / prefixLen–128 | 32 / 128 | prefixLen - 1 | 33 / 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `blackhole-policy-add-refused.ci` | `test/plugin/blackhole-policy-add-refused.ci` | Policy-added BLACKHOLE withheld from unagreed eBGP peer | |
| `bgp-rib-maximum-paths-reload.ci` | `test/plugin/bgp-rib-maximum-paths-reload.ci` | Maximum-paths reload passes standalone | |
| `rpki-aspa-*.ci` | `test/plugin/rpki-aspa-*.ci` | RPKI ASPA tests receive Ze's EOR without timeout | |

### Interop Tests (Scope: plugin)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Existing BMP / BLACKHOLE / SRv6 interop scenarios | `test/interop/scenarios/` | FRR / GoBGP | No regression on wire-visible plugin behaviors; BMP A-B-A and BLACKHOLE policy withholding hold end-to-end | |

## Files to Modify
- `internal/component/bgp/plugins/cmd/peer/peer.go` - unify Shutdown Communication contract with `notify`
- `internal/component/bgp/plugins/cmd/peer/admin.go` - shared Shutdown Communication validation helper
- `internal/component/bgp/plugins/cmd/peer/notify.go` - shared Shutdown Communication validation helper
- `internal/component/bgp/plugins/cmd/announce/announce.go` - apply `announce ... for` lifetime
- `internal/component/bgp/plugins/cmd/update/update_text_test.go` - assert bare `destination`/`source` refusal naming qualified keywords
- `internal/component/bgp/plugins/filter_community/blackhole.go` - evaluate egress-modified attributes
- `internal/component/bgp/plugins/filter_community/handler.go` - check `genericCommunityHandler` Add `Buf` bounds
- `internal/component/bgp/plugins/filter_prefix/config.go` - validate every `ge`/`le` combination
- `internal/component/bgp/plugins/bmp/bmp.go` - bounded route-aware dedup state
- `internal/component/bgp/plugins/bmp/bmp_events.go` - emit contradicting A → B → A Route Monitoring sequence
- `internal/component/bgp/plugins/rr/rr.go` - log `no established peers` below `ERROR`
- `internal/component/bgp/plugins/rs/source_lifetime.go` - treat `errAdjOutSession` as definite non-delivery, not permanent `unknownOutputs`
- `internal/component/bgp/plugins/rib/rib.go` - JSON rail SAFI 4 labels and unsplit family refusal/warning
- `internal/component/bgp/plugins/rib/rib_recovery.go` - `recoveryRoutes` for non-`bgp-rib` destinations
- `internal/component/bgp/plugins/rib/rib_commands.go` - `rib show` received `LARGE_COMMUNITY`
- `internal/component/bgp/plugins/rib/rib_nlri.go` - `formatNLRIAsPrefix` VPN RD plus prefix rendering
- `internal/component/bgp/plugins/rib/pool/srv6sid.go` - `ExtractSRv6SIDFull` family-aware L3 vs L2 Service TLV
- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - drop spurious WARN on withdrawn pending route
- `internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go` - delegate `declareShapes` to `command.PluginShapesFromDecls`
- `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` - refuse bare `destination`/`source` naming `destination-ipv4`/`source-ipv4`
- `internal/component/bgp/plugins/nlri/ls/attr_node.go` - emit RFC 9552 Table 7 key `"A"` for bit 6
- `internal/component/bgp/plugins/nlri/mvpn/types.go` - skip RD parse in `parseBody` for Leaf A-D type 4
- `internal/component/bgp/plugins/nlri/evpn/plugin.go` - refuse trailing `rest` bytes in `DecodeNLRIHex`

## Files to Create
- `test/plugin/blackhole-policy-add-refused.ci` - end-to-end policy-added BLACKHOLE refusal test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new config leaves or RPCs |
| YANG validation constraints | No | Existing schemas unchanged |
| YANG custom validators | No | Existing validators unchanged |
| CLI commands/flags | No | Existing commands repaired in place |
| CLI grammar (keyword before value) | No | No new verbs or keywords |
| Editor autocomplete | No | No schema additions |
| Functional test for new RPC/API | Yes | `test/plugin/blackhole-policy-add-refused.ci` |
| Pipe completeness | No | Existing pipe paths unchanged |
| Env var registration | No | No new env vars |
| Doctor check for runtime dependencies | No | No new runtime dependencies |
| Doctor code per problem class | Yes | Any newly added `Warn` (`rib_recovery.go`, `rib.go`, `rr.go`) carries a declared doctor code and uses `sdk.Plugin.RaiseIssue` via `issuequeue` when raising transient issues |
| Prometheus counters/metrics | No | Existing counters remain sufficient |
| BGP family surface (new SAFI / capability / attribute) | No | No new family or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Bug fixes only |
| 2 | Config syntax changed? | No | Existing FlowSpec qualified keywords preserved |
| 3 | CLI command added/changed? | No | Existing commands repaired |
| 4 | API/RPC added/changed? | No | Existing RPCs repaired |
| 5 | Plugin added/changed? | No | Existing BGP plugins repaired |
| 6 | Has a user guide page? | No | Existing guides remain accurate |
| 7 | Wire format changed? | No | Wire codecs brought into RFC compliance |
| 8 | Plugin SDK/protocol changed? | No | Uses existing SDK and process protocol |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9003.md`, `rfc/short/rfc7999.md`, `rfc/short/rfc6514.md`, `rfc/short/rfc9252.md`, `rfc/short/rfc9552.md`, `rfc/short/rfc9830.md` |
| 10 | Test infrastructure changed? | No | Existing functional runner used |
| 11 | Affects daemon comparison? | No | No comparison change |
| 12 | Internal architecture changed? | No | Repairs inside existing plugin architecture |
| 13 | Route metadata keys added/changed? | No | No metadata key changes |
| 14 | Prometheus counters added/changed? | No | No new counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors spec plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md`; declared pages checked and unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `docs/architecture/wire/nlri-bgpls.md` matches RFC 9552 Table 7 key `"A"` |

## Design presentation

### What changes

This child spec repairs the 24 remaining open BGP plugin defects under `ai/doors/bgp-plugins.md`: unifying `request peer teardown` and `notify` onto one RFC 9003 Shutdown Communication contract, checking egress-modified attributes in the RFC 7999 BLACKHOLE gate, fixing BMP Route Monitoring dedup on contradicting A → B → A sequences, lowering normal `no established peers` logs in `bgp-rr`, preventing retired-session refusals from sticking in `bgp-rs` `unknownOutputs`, scoping `ExtractSRv6SIDFull` by L3 vs L2 family, repairing JSON-rail SAFI 4 labels, VPN RD rendering, received `LARGE_COMMUNITY` display, MVPN Leaf A-D type 4 RD parsing, BGP-LS Node Flag bit 6 (`A`), `DecodeNLRIHex` trailing bytes, `announce ... for` lifetime, `filter_prefix`/`filter_community` guards, and removing the duplicate shape loop in `adj_rib_in`.

### Before and after

1. **BMP A → B → A sequence (AC-5):** Before, `bmpPlugin.dedupState` hashed every sent body into an unordered set, so re-advertising A after B silently suppressed the second A. After, BMP tracks per-route state and emits all three messages A, B, A.
2. **BGP-RS quiesce after session rotation (AC-15):** Before, `errAdjOutSession` stayed in `rs.unknownOutputs` forever, causing every subsequent `request quiesce` to fail. After, definite retired-session non-delivery is settled cleanly and `request quiesce` succeeds.
3. **VPN route display in `show bgp rib best` (AC-10):** Before, `formatNLRIAsPrefix` misread the leading label/RD octets and rendered an IPv4 VPN route as an invalid `/112` IPv4 prefix. After, it renders `<RD> <prefix>`.
4. **`request peer teardown` vs `notify` (AC-2):** Before, a 129-octet Shutdown Communication was silently truncated by `teardown` and refused by `notify`. After, both apply the same RFC 9003 contract and answer identically.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Call `internal/core/report` directly from `bgp-rib`, `bgp-rr`, or `bgp-rs` | Breaks process-boundary parity when the plugin runs forked (`ai/doors/system.md`, `ai/doors/bgp-plugins.md`) |
| Disable BMP dedup globally without bounding memory | Risks memory growth or noisy duplicate frames; route-scoped state keeps dedup bounded and correct |
| Keep the local shape loop in `adj_rib_in/rib_shape_test.go` | Duplicates `command.PluginShapesFromDecls` that all other plugins already share |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — write red-first unit and wiring tests (`TestTeardownAndNotifyShareShutdownContract`, `TestBMPDedupEmitsContradictingSequenceABA`, `TestRSRetiredSessionRefusalDoesNotStickInUnknownOutputs`, `test/plugin/blackhole-policy-add-refused.ci`).
2. **Phase: Commands & Filters (AC-1, AC-2, AC-3, AC-11 `announceAndTrack`, AC-12)** — FlowSpec keyword refusal, `teardown`/`notify` contract, BLACKHOLE egress attributes, `announce ... for` lifetime, `filter_community` Add `Buf`, and `filter_prefix` `ge`/`le` & test shape.
3. **Phase: RIB, Adj-RIB-In, RS, RR, BMP, RPKI (AC-4, AC-5, AC-6, AC-7, AC-8, AC-9, AC-10, AC-14, AC-15, AC-16)** — maximum-paths reload, BMP dedup, `rr` log level, `ExtractSRv6SIDFull`, `recoveryRoutes`, SAFI 4 JSON rail, VPN RD & `LARGE_COMMUNITY` display, `declareShapes`, `rs.unknownOutputs`, `adj-rib-in` `applyDecision`, and `rpki-aspa` EOR.
4. **Phase: NLRI plugins (AC-11 `DecodeNLRIHex` & `lsNodeFlagBits`, AC-13)** — trailing `rest` refusal, BGP-LS Node Flag `"A"`, MVPN Leaf A-D type 4 RD omission, and RFC 9830 / 9252 proof tags (via `ze-test-judge`).

## Work Not Done

- BGP engine defects (`ai/doors/bgp-engine.md`) are owned by `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md`.
- Functional `.ci` reds, Docker interop scenarios, and `rfc/short/` enrollment (`ai/doors/le-tooling.md`) are owned by `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md`.
- Already-fixed defects (33 journal rows, including AC-6, AC-59, AC-60, and all MRT rows) are retired in `plan/release/fixes/spec-bgp-one-off-defects.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `bulk-rename-corruption` | 2026-09-06 | flowspec `destination` keyword |
| `bulk-rename-corruption` | 2026-09-04 | `plugins/cmd/update` |
| `conflicting-feature-contracts` | 2026-10-21 | `request peer <sel> teardown` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-21 | `blackholeReadvertiseRefused` |
| `listener-refuses-the-peer-its-config-names` | 2026-10-17 | `bgp-rib-maximum-paths-reload.ci` |
| `memo-suppresses-a-change-it-cannot-see` | 2026-08-30 | bmp `handleSenderUpdate` dedup |
| `normal-state-logged-at-error` | 2026-10-22 | `updateRouteWithMeta` |
| `normal-state-logged-at-error` | 2026-10-23 | adj-rib-in `applyDecision` WARN for a verdict on a withdrawn pending route |
| `silent-fall-through` | 2026-10-14 | `pool.ExtractSRv6SIDFull` |
| `silent-fall-through` | 2026-10-10 | `recoveryRoutes` |
| `silent-fall-through` | 2026-10-04 | `insertPoolNLRIs` SAFI 4 |
| `gate-excludes-part-of-its-population` | 2026-08-10 | `genericCommunityHandler` Add `Buf` unchecked |
| `gate-excludes-part-of-its-population` | 2026-08-18 | prefix-list `ge`/`le` missing combination |
| `green-that-could-not-have-been-red` | 2026-08-23 | `parsePrefixListEntries` multi-entry test feeds a shape no producer emits |
| `silent-fall-through` | 2026-10-14 | `show bgp rib best` VPN |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-09-21 | `lsNodeFlagBits.ToJSON` bit 6 `T` not `A` |
| `reference-checked-claim-unchecked` | 2026-10-13 | RFC9830-4.2.1-2/-6 `rfc9830_test.go` |
| `reference-checked-claim-unchecked` | 2026-10-13 | RFC9252-7-1/-2 `TestRFC9252FirstServiceTLVWins` |
| `unwired-feature` | 2026-08-12 | received LARGE_COMMUNITY in `rib show` |
| `validated-value-discarded-by-its-caller` | 2026-09-03 | `announceAndTrack` lifetime |
| `validated-value-discarded-by-its-caller` | 2026-09-20 | `DecodeNLRIHex` discards `rest` |
| `zero-value-as-valid-answer` | 2026-09-01 | mvpn `parseBody` reads a Leaf A-D Route Key as an RD (zero-RD face fixed by `rdPresent`) |
| `expected-message-never-sent` | 2026-10-31 | `test/plugin/rpki-aspa-*` time out waiting for Ze's EOR |
| `definite-non-delivery-kept-as-unknown-outcome` | 2026-11-20 | bgp-rs keeps an `errAdjOutSession` refusal in `rs.unknownOutputs` for good, so every later settlement and `request quiesce` fails |

## Review Gate

Pending implementation and independent review.
