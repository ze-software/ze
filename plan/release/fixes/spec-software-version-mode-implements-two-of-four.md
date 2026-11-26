# Spec: software-version-mode-implements-two-of-four

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Accepted `require` and `refuse` capability modes on `session/capability/software-version/mode` must enforce the promised RFC 5492 session outcomes instead of silently behaving as `enable` and `disable`. |
| RFC gaps | - |
| Depends | Owned by this spec per owner decision T-4 (2026-11-24) on `plan/release/fixes/spec-yang-single-source-of-truth.md`. Sibling `bgp/rib/adj-rib-*` half of the 2026-09-04 `unwired-feature.md` row is owned by `plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md`. Enum vocabulary gate sibling: `plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md`. |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** `session/capability/software-version/mode`
(`internal/component/bgp/plugins/softver/yang/ze-softver.yang`) uses the shared
`bgp:capability-mode` enumeration (`enable`, `disable`, `require`, `refuse`).
The capability container (`internal/component/bgp/yang/ze-bgp-conf.yang`) states
the contract for every capability under it: `enable` advertises with no
enforcement, `disable` neither advertises nor enforces, `require` advertises and
refuses the session with an RFC 5492 Section 3 Unsupported Capability
NOTIFICATION when the peer does not offer it, and `refuse` does not advertise
and refuses the session with Unsupported Capability when the peer offers it.

**What Ze does instead.** `extractSoftverCapabilities` and `parseSoftverSetting`
(`internal/component/bgp/plugins/softver/softver.go`) collapse the four values
into one boolean (`mode != modeDisable && mode != modeRefuse`). `require`
behaves as `enable` (a peer without code 75 stays up) and `refuse` behaves as
`disable` (a peer sending code 75 stays up). The reactor already enforces
`RequiredCapabilities` and `RefusedCapabilities` in `validateCapabilityModes`
(`internal/component/bgp/reactor/session_validation.go`), but `rpc.CapabilityDecl`
(`pkg/plugin/rpc/types.go`) carries only `Code`, `Encoding`, `Payload`, and
`Peers`, so no plugin-owned capability can declare an enforcement mode across
the plugin boundary. Furthermore, `Negotiated.CheckRequiredCodes`
(`internal/core/bgp/capability/negotiated.go`) maps only core-known capability
codes and defaults all other codes to `false`, so a plugin-owned code would
fail closed even when the peer advertised it.

**What closing it means.** Per owner decision T-4 (2026-11-24), implement
`require` and `refuse` on `software-version/mode` generically across the plugin
capability boundary:

1. Add an optional `mode` field (`CapMode`: `enable`, `require`, `refuse`;
   omitted means `enable`) to `rpc.CapabilityDecl` (`pkg/plugin/rpc/types.go`,
   aliased as `sdk.CapabilityDecl`) and `plugin.InjectedCapability`
   (`internal/component/plugin/registration.go`).
2. Carry `mode` through `capabilitiesFromRPC` and `CapabilityInjector`
   (refusing conflicting modes on the same code and scope; allowing omitted
   encoding/payload when `mode` is `refuse`, since refused capabilities are not
   advertised in local OPEN).
3. Propagate plugin-declared `required` and `refused` capability codes into
   `pluginCapabilitySnapshot`, `preparedOpen`, and `Session` OPEN validation
   (`validateCapabilityModes`), and include enforcement mode in
   `pluginDeclarationIndex` / `pluginDeclarationsKept` so live config reloads
   that tighten enforcement against a running session restart it cleanly.
4. In `Negotiated.CheckRequiredCodes` (`internal/core/bgp/capability/negotiated.go`),
   check core capabilities through their existing capability-specific negotiation
   rules and evaluate any non-core (plugin-owned) required code via
   `n.PeerAdvertised(code)`, without hardcoding code 75 in core.
5. Update `bgp-softver` (`softver.go` and `ze-softver.yang`) to parse all four
   modes (refusing unknown values), emit `CapModeRequire` with payload for
   `require` and `CapModeRefuse` without payload for `refuse`, and update the
   YANG `mode` description to state the enforced behavior.

This spec is the sole owner of `software-version/mode` runtime enforcement and
the `software-version/mode` half of the 2026-09-04 `unwired-feature.md` journal
row.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md`, Stage 3 `declare-capabilities`
  → Decision: `CapabilityDecl` is the published wire contract for plugin OPEN injection, internal and external alike
  → Constraint: adding an optional `mode` field with `omitempty` preserves wire compatibility for third-party plugins that omit it
- [ ] `docs/architecture/behavior/fsm-open-sent.md` and `docs/architecture/behavior/peer-lifecycle.md`
  → Decision: `validateCapabilityModes` enforces `RequiredCapabilities` and `RefusedCapabilities` on OPEN receipt; `pluginCapabilitiesKept` decides reload swap vs restart from the frozen OPEN proposal
  → Constraint: a reload that changes enforcement mode for a peer whose OPEN violates the new mode must restart the session rather than hot-swapping
- [ ] `docs/architecture/core-design.md`, Section 2 and Section 19
  → Decision: BGP core never hardcodes plugin capability codes; plugins register and declare capabilities generically
  → Constraint: `CheckRequiredCodes` evaluates plugin-owned codes generically via `PeerAdvertised` rather than adding a code-75 case
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`
  → Constraint: `pkg/plugin/rpc` and `pkg/plugin/sdk` are public API; external plugins must get identical enforcement over TLS RPC

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5492.md` - Sections 3 and 5, Unsupported Capability NOTIFICATION
  → Constraint: NOTIFICATION error code 2 (OPEN Message Error), subcode 7 (Unsupported Capability), with Data carrying the causing capability TLV(s) encoded as in OPEN (from local OPEN for missing `require`, from peer OPEN for present `refuse`)
- [ ] `rfc/short/draft-abraitis-bgp-version-capability.md` - Sections 3 and 4
  → Constraint: code 75 remains opt-in (default disabled when container absent); `draft` and `legacy` payload encodings remain unchanged

**Key insights:**
- `validateCapabilityModes` and `buildUnsupportedCapabilityDataCodes` already construct RFC 5492 Section 5 NOTIFICATION 2/7 data from `localCaps` (for missing required codes) and `peerCaps` (for present refused codes).
- `CheckRefusedCodes` already checks `n.peerCodes[code]` generically for any capability code; only `CheckRequiredCodes` had a closed map defaulting unlisted codes to `false`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/softver/softver.go` - `parseSoftverSetting` sets `enabled: mode != modeDisable && mode != modeRefuse`, collapsing `require` to `enable` and `refuse` to `disable`; `extractSoftverCapabilities` emits a `CapabilityDecl` only when `enabled` is true
- [ ] `internal/component/bgp/plugins/softver/yang/ze-softver.yang` - `leaf mode` uses `bgp:capability-mode` with default `enable`; its `description` currently documents that `require` acts as `enable` and `refuse` acts as `disable`
- [ ] `pkg/plugin/rpc/types.go` and `pkg/plugin/rpc/enums.go` - `CapabilityDecl` has `Code`, `Encoding`, `Payload`, `Peers` and no enforcement `Mode` field
- [ ] `internal/component/plugin/registration.go` - `InjectedCapability` has `Code`, `Value`, `Plugin`, `PeerAddr`; `CapabilityInjector.planLocked` decodes payloads and checks `(code, value)` conflicts per scope
- [ ] `internal/component/plugin/server/startup.go` - `capabilitiesFromRPC` converts `rpc.DeclareCapabilitiesInput` into `*plugin.PluginCapabilities`
- [ ] `internal/component/bgp/reactor/peer.go` and `session_plugin_capabilities.go` - `Peer.pluginCapabilitySnapshot` turns every `InjectedCapability` into a wire `capability.NewPlugin` and indexes `(code -> values)` into `pluginCapabilitySnapshot.declarations`
- [ ] `internal/component/bgp/reactor/session_open_proposal.go` - defines `preparedOpen` (`message`, `settings`, `pluginDeclarations`) with no `pluginRequired` or `pluginRefused` fields yet
- [ ] `internal/component/bgp/grmarker/grmarker.go` - `SetRBit` copies code-64 entries via a struct literal omitting `Mode`, whereas `SetFBit` copies the full `plugin.InjectedCapability` struct before replacing `Value`
- [ ] `internal/component/bgp/reactor/session_negotiate.go` and `session_handlers.go` - `prepareOpen` freezes `proposal`; `session_handlers.go` passes `proposal.settings.RequiredCapabilities` and `RefusedCapabilities` to `validateCapabilityModes`
- [ ] `internal/component/bgp/reactor/session_validation.go` - `validateCapabilityModes` calls `neg.CheckRequiredCodes` and `neg.CheckRefusedCodes`, sending NOTIFICATION 2/7 via `buildUnsupportedCapabilityDataCodes` and closing the connection on mismatch
- [ ] `internal/core/bgp/capability/negotiated.go` - `CheckRequiredCodes` checks a 10-entry core map and defaults unlisted codes to `false`; `CheckRefusedCodes` checks `n.peerCodes[code]` for any code
- [ ] `internal/component/bgp/reactor/peer_settings_negotiation.go` - `pluginDeclarationsKept` restarts a session only when a code's wire value changed and the peer advertised that code

**Behavior to preserve:**
- `software-version` absent (no container configured): code 75 is neither advertised nor enforced (default disabled per draft Section 3).
- `mode enable` (default when container is present) and `mode disable`: exact existing OPEN advertisement, `draft`/`legacy` framing, and session behavior.
- Existing `CapabilityDecl` JSON payloads omitting `mode` behave identically to `enable`.
- Core capability `require`/`refuse` enforcement (`asn4`, `extended-message`, `route-refresh`, `graceful-restart`, `nexthop`, `add-path`) is unchanged.

**Behavior to change:**
- `software-version { mode require; }`: advertises code 75 and closes any session whose peer OPEN lacks code 75 with NOTIFICATION 2/7 carrying the local code-75 capability TLV.
- `software-version { mode refuse; }`: does not advertise code 75 and closes any session whose peer OPEN carries code 75 with NOTIFICATION 2/7 carrying the peer's code-75 capability TLV.
- Live config reload switching `software-version/mode` to `require` (when peer lacked code 75) or `refuse` (when peer advertised code 75) restarts the session so the new policy takes effect.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Reactor capability mode enforcement | `internal/component/bgp/reactor/session_validation.go` `validateCapabilityModes`, `buildUnsupportedCapabilityDataCodes` | Enforces `RequiredCapabilities` and `RefusedCapabilities`, emitting RFC 5492 Section 3/5 NOTIFICATION 2/7 | Extends: feeds plugin-declared required and refused codes into the same validator |
| Negotiated capability code checks | `internal/core/bgp/capability/negotiated.go` `CheckRequiredCodes`, `CheckRefusedCodes` | Checks required core codes against negotiated flags and refused codes against `n.peerCodes` | Extends: evaluates non-core required codes via `n.PeerAdvertised(code)` so plugin codes work without core hardcoding |
| Plugin capability injection | `pkg/plugin/rpc/types.go` `CapabilityDecl`; `internal/component/plugin/registration.go` `CapabilityInjector` | Carries plugin-declared capabilities from Stage 3 and reload verify into `Peer.pluginCapabilitySnapshot` | Extends: adds optional `Mode` (`CapMode`) to `CapabilityDecl` and `InjectedCapability`, with conflict checking and non-advertised `refuse` entries |
| Immutable OPEN proposal | `internal/component/bgp/reactor/session_open_proposal.go` `preparedOpen` | Freezes `message`, `settings`, and `pluginDeclarations` for OPEN admission and reload comparison | Extends: retains `pluginRequired` and `pluginRefused` slices alongside `pluginDeclarations` (with enforcement mode) so `validateCapabilityModes` and reload comparison read one frozen proposal |
| GR restart flag rewriting | `internal/component/bgp/grmarker/grmarker.go` `SetRBit`, `SetFBit` | Rewrites R and F bits on injected code-64 capabilities inside the restart window | Extends: updates `SetRBit` to copy the full `plugin.InjectedCapability` struct (matching `SetFBit`) so `Mode` is preserved across restart-flag rewriting |
| Reload capability swap decision | `internal/component/bgp/reactor/peer_settings_negotiation.go` `pluginCapabilitiesKept`, `pluginDeclarationsKept` | Decides whether a running session can keep running or must restart when plugin declarations change on reload | Extends: checks enforcement mode changes against `peerCaps` so tightening to `require` or `refuse` restarts violating sessions |
| Softver capability extractor | `internal/component/bgp/plugins/softver/softver.go` `parseSoftverSetting`, `extractSoftverCapabilities` | Parses `software-version` container and emits code 75 `CapabilityDecl` entries | Extends: maps all four `capability-mode` values (`enable`, `disable`, `require`, `refuse`) instead of collapsing to a boolean |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A change under `pkg/` is a change to a published contract. A type, a field, a callback name and the bytes on the wire all count. The SDK stays generic and carries no plugin-specific code" | agrees | Adding optional `Mode` (`omitempty`) to `rpc.CapabilityDecl` is generic across all plugins, carries no softver-specific logic, and updates `docs/plugin-development/protocol.md` and `docs/architecture/api/process-protocol.md` in the same change |
| `docs/architecture/api/ipc_protocol.md` | One encoding, both directions | the // Design: page of `pkg/plugin/rpc/types.go` and `pkg/plugin/rpc/enums.go` | "Stage 3 `declare-capabilities` carries the BGP capabilities a plugin injects into OPEN and names no wire shape" | agrees | Optional `mode` on `CapabilityDecl` preserves the Stage 3 framing and command-answer encoding unchanged |
| `docs/architecture/api/commands.md` | Overview | the // Design: page of `pkg/plugin/rpc/types.go` and `internal/component/plugin/server/startup.go` | - | silent | Covers CLI/API command dispatch and pipes; this spec changes no CLI or API command |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/enums.go`, `internal/component/bgp/reactor/peer.go` | - | silent | Covers bus participant filtering and API architecture; this spec changes no bus filter or event schema |
| `docs/architecture/api/process-protocol.md` | 5-Stage Startup Protocol | the // Design: page of `pkg/plugin/sdk/sdk_types.go`, `internal/component/plugin/registration.go`, `internal/component/plugin/server/startup.go`; a page this spec modifies | "A reload replaces the Stage 3 declaration through the config-verify answer." | agrees | Extends `CapabilityDecl` with optional `mode` (`omitempty`) on both Stage 3 `declare-capabilities` and reload `config-verify` answers, updating this page's Stage 3 `DeclareCapabilitiesInput` prose (while `docs/plugin-development/protocol.md` updates the `CapabilityDecl` field table) |
| `docs/architecture/api/wire-format.md` | Transport | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers binary wire serialization formats; `CapabilityDecl` uses existing JSON framing |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers config apply ordering across plugins; capability staging order is unchanged |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `pkg/plugin/rpc/types.go` and `internal/component/plugin/server/startup.go` | - | silent | Covers verify/apply/rollback transaction states; reload capability replacement already uses this protocol unchanged |
| `docs/architecture/core-design.md` | 18a. BGP Peer Reload: Swap or Restart | the // Design: page of `peer.go`, `session_negotiate.go`, `session_handlers.go`, `peer_settings_negotiation.go`, `softver.go`, `grmarker.go`; a page this spec modifies | "The Session's admitted remote OPEN is visible to this check before negotiation advances; a changed capability the remote did not advertise can remain." | departs | AC-4 tightens the reload rule: a changed capability whose mode becomes `require` when the remote did not advertise that code cannot remain and must restart the session so `validateCapabilityModes` rejects it with NOTIFICATION 2/7; this spec updates section 18a in the same change |
| `docs/architecture/doctor-and-health-checks.md` | BGP policy absences in the supplied configuration | anchored to `pkg/plugin/rpc/types.go` and `internal/component/plugin/server/startup.go` | - | silent | Covers `ze doctor` checks; this spec adds no doctor check or diagnostic code |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers firewall IRR prefix sets; unrelated to BGP capability enforcement |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers `route-install` RPC for forked protocol plugins; unrelated to OPEN capabilities |
| `docs/architecture/plugin-manager-wiring.md` | Problem | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers process spawn and handshake wiring; Stage 3 sequencing is unchanged |
| `docs/architecture/plugin/plugin-system.md` | Where each layer lives | anchored to `pkg/plugin/rpc/types.go`, `internal/component/plugin/server/startup.go`, `internal/component/bgp/reactor/peer.go` | - | silent | Covers plugin registration lifecycle; `bgp-softver` registration and process boundary stay unchanged |
| `docs/architecture/system-architecture.md` | System RIB process boundary | anchored to `pkg/plugin/rpc/types.go` | - | silent | High-level system architecture; no component boundary changes |
| `docs/architecture/update-cache.md` | Purpose | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers BGP UPDATE cache retention/release; unrelated to OPEN capabilities |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers NLRI wire encoding; this spec changes no NLRI codec |
| `ai/doors/plugin-infra.md` | Who depends on it from outside | the door covering `pkg/plugin/rpc/`, `pkg/plugin/sdk/`, `internal/component/plugin/` | "A field or method name in `rpc` is wire, not Go: renaming a JSON tag breaks every plugin written in another language." | agrees | Preserves all existing `CapabilityDecl` JSON fields and adds `"mode"` as optional with `omitempty` so existing plugins in any language continue working unchanged |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `pkg/plugin/rpc/enums.go` and `internal/component/bgp/reactor/peer.go` | - | silent | Covers RIB storage and best-path selection; unrelated to OPEN capability modes |
| `docs/architecture/hub-architecture.md` | Vision | anchored to `internal/component/plugin/registration.go` and `internal/component/plugin/server/startup.go` | - | silent | Covers daemon startup and orchestration; unchanged by capability mode enforcement |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers CLI tab completion; `ze-softver.yang` already declares all four `mode` enum values |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers nftables ownership and flush; unrelated to BGP capability modes |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers IKEv2 engine; unrelated to BGP capability modes |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | RFC obligations carried by this code | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers PPPoE client; unrelated to BGP capability modes |
| `docs/architecture/wire/capabilities.md` | Capability Defaults | the // Design: page of `internal/core/bgp/capability/negotiated.go` | ""Absent" means the capability is not advertised and has no enforcement — it's as if it doesn't exist. Setting it to `enable`, `require`, or `refuse` activates it." | agrees | Brings Software Version (code 75) into compliance with this documented contract so `require` and `refuse` enforce session outcomes while absent stays disabled |
| `docs/architecture/api/json-format.md` | Overview | anchored to `internal/core/bgp/capability/negotiated.go` and `internal/component/bgp/reactor/peer.go` | - | silent | Covers BGP JSON event format; no event fields change |
| `docs/architecture/edge-cases/addpath.md` | Overview | anchored to `internal/core/bgp/capability/negotiated.go` | - | silent | Covers ADD-PATH edge cases; `CodeAddPath` retains its existing entry in `CheckRequiredCodes` |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `internal/core/bgp/capability/negotiated.go` | - | silent | Covers 4-octet ASN edge cases; `CodeASN4` retains its existing entry in `CheckRequiredCodes` |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `internal/core/bgp/capability/negotiated.go` and `internal/component/bgp/reactor/session_negotiate.go` | - | silent | Covers RFC 8654 Extended Message; `CodeExtendedMessage` retains its asymmetric receive-permission check in `CheckRequiredCodes` |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `internal/core/bgp/capability/negotiated.go` | - | silent | Covers Docker interop suite discovery; `frr-software-version` scenario is preserved |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `internal/core/bgp/capability/` and `internal/component/bgp/reactor/` | "Never name a plugin package from the reactor." | agrees | Reactor reads generic `InjectedCapability.Mode` from the capability injector and `capability.CheckRequiredCodes` evaluates non-core codes generically with zero reference to `softver` or code 75 |
| `docs/architecture/behavior/peer-lifecycle.md` | One immutable proposal for each OPEN | the // Design: page of `internal/component/bgp/reactor/session_plugin_capabilities.go` and `internal/component/bgp/reactor/session_open_proposal.go`; a page this spec modifies | "Successful semantic admission retains the immutable remote OPEN under the Session lock before negotiation advances. Reload therefore judges a pending connection against its own accepted capabilities, including the rule that a changed declaration may remain when the peer did not advertise that code." | departs | Extends `pluginCapabilitySnapshot` and `preparedOpen` immutably, but AC-4 changes the unconditional rule that a changed declaration may remain when the peer did not advertise that code: reloading from `enable` to `require` when the peer omitted the code must restart the session; this spec updates this section in the same change |
| `docs/architecture/api/capability-contract.md` | Implementation Status | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers API-dependent GR/RR cache control; unchanged |
| `docs/architecture/api/update-syntax.md` | Status | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers `send bgp ... update text` grammar; unrelated to OPEN capability modes |
| `docs/architecture/behavior/fsm-open-sent.md` | What `handleOpen` actually validates before firing `EventBGPOpen` | anchored to `peer.go`, `session_negotiate.go`, `session_handlers.go` | "`validateCapabilityModes(...)` enforces `RequiredCapabilities` and `RefusedCapabilities`." | agrees | Feeds plugin-declared required and refused codes directly into `validateCapabilityModes` at this exact validation step |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `peer.go` and `session_handlers.go` | - | silent | Covers FSM state transition table; `validateCapabilityModes` fires the existing `EventBGPOpenMsgErr` transition |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers UPDATE forwarding pipeline; untouched |
| `docs/architecture/config/syntax.md` | Capability Section | anchored to `internal/component/bgp/reactor/peer.go` | "All capabilities support a four-mode vocabulary, declared once as the `capability-mode` typedef and reused by every mode leaf under `capability`" | agrees | Implements the documented four-mode `capability-mode` enforcement table (`enable`, `disable`, `require`, `refuse`, lines 1473–1478) for `software-version/mode` and honors the unknown-word refusal rule (lines 1550–1552: refusing any mode word outside that vocabulary rather than defaulting to `enable`) |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers `EncodingContext` and `ContextID`; code 75 does not affect UPDATE wire encoding |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers forward pool backpressure; untouched |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers RFC 9234 role metadata; untouched |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers `.ci` test file format; new functional test follows it |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/reactor/peer.go` | - | silent | Covers UPDATE message construction; untouched |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to `internal/component/bgp/reactor/session_negotiate.go` | - | silent | Covers protocol message capture; untouched |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `internal/component/bgp/reactor/session_handlers.go` | - | silent | Covers Established state handling; capability enforcement happens during OPEN processing before Established |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | anchored to `internal/component/bgp/reactor/session_handlers.go` | - | silent | Covers OpenConfirm state handling; capability enforcement happens before advancing to OpenConfirm |
| `docs/architecture/wire/messages.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/reactor/session_handlers.go` | - | silent | Covers BGP message header and framing; NOTIFICATION 2/7 uses existing `logNotifyErr` framing |
| `ai/doors/bgp-plugins.md` | Where it sits | the door covering `internal/component/bgp/plugins/softver/` | "The in-tree plugins hold no privilege over a third-party one" | agrees | `bgp-softver` declares `require` and `refuse` through the exact same `CapabilityDecl.Mode` wire field available to every third-party plugin |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/softver-capability-mode.ci` | "Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain." | agrees | Goal Gates and Deliverables use `./le test unit pkg path` and `./le test functional plugin` with no `./le verify` command |
| `docs/plugin-development/protocol.md` | Stage 3: Capabilities (Plugin to Engine) | a page this spec modifies | "Plugin sends `ze-plugin-engine:declare-capabilities` with a `DeclareCapabilitiesInput`" | agrees | Updates the `CapabilityDecl` field table in this section to document optional `"mode"` (`enable`, `require`, `refuse`; default `enable`) |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Does not add or alter any CLI command or pipe operation |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not alter command pipe overrides |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not alter pipe operator behavior |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` rendering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `bgp-softver` keeps its existing bus participant registration unchanged |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not modify bus `send > to` filtering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not modify bus `receive > from` filtering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not modify bus message allow-lists |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not modify bus filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Capability conflict checking in `CapabilityInjector` already refuses conflicting capability declarations per code/scope and does not add bus arbitration |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not modify bus display output |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Reuses the existing `ze-plugin-engine:declare-capabilities` engine operation without adding a new undeclared operation |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not modify event delivery queues or overflow handling |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | BGP capability enforcement is data-plane agnostic and leaves VPP/kernel gating untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not modify data-plane mismatch validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Unrelated to VPP firewall verification; untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch data-plane selector vocabulary |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch egress next-hop policy or `session { next-hop }` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch export chain next-hop configuration |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch `policychain.Chain` or `FilterInsertion` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch export chain folding or raw override provenance |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch `policychain.Sets.NextHop` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch route origination or forwarding next-hop rewriting |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied filter publication |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Preserves `ze start` behavior and keeps invalid reload candidates refused whole (via `CapabilityInjector` atomic replacement) |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Does not modify kernel/VPP startup mismatch behavior |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | `bgp-softver` Stage 1 declare-mode purity is preserved; `CapabilityDecl.Mode` is emitted only at Stage 3 / config-verify |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Does not modify Docker interop suite capability declarations |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not touch FlowSpec MP_REACH next-hop emission |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator configuration at `bgp > peer|group > session > capability > software-version > mode` (load, commit, or SIGHUP reload), delivered to `bgp-softver` as Stage 2 config or reload `config-verify` JSON.
- Peer BGP OPEN message arriving at `Session` (`internal/component/bgp/reactor/session_handlers.go`).

### Transformation Path
1. `bgp-softver` (`parseSoftverSetting`, `extractSoftverCapabilities`) parses `mode` (`enable`, `disable`, `require`, `refuse`) and `encoding` (`draft`, `legacy`), returning `[]sdk.CapabilityDecl` with `Mode` set for `require` (with payload) and `refuse` (without payload).
2. `sdk.Plugin` sends `DeclareCapabilitiesInput` at Stage 3 (`ze-plugin-engine:declare-capabilities`) and in reload `config-verify` answers.
3. `plugin/server` (`capabilitiesFromRPC`) maps `CapabilityDecl` into `plugin.PluginCapabilities`, and `CapabilityInjector` (`AddPluginCapabilities` / `ReplacePluginCapabilities`) validates mode and payload conflicts per scope (`global` or peer selector).
4. `Peer.pluginCapabilitySnapshot` extracts `declarations` (including enforcement mode), `required` codes (`CapModeRequire`), and `refused` codes (`CapModeRefuse`) before `restartFlagsFor`, preserves `Mode` inside `grmarker.SetRBit` (matching `SetFBit`), and builds wire `capabilities` from non-`refuse` entries.
5. `prepareOpen` freezes the snapshot on `preparedOpen`; `buildOpenFromSnapshot` puts only non-`refuse` wire capabilities into Ze's OPEN.
6. On peer OPEN receipt, `session_handlers.go` merges `proposal.pluginRequired`/`pluginRefused` with `proposal.settings.RequiredCapabilities`/`RefusedCapabilities` and runs `validateCapabilityModes`, which checks `neg.CheckRequiredCodes` (evaluating non-core codes via `n.PeerAdvertised`) and `neg.CheckRefusedCodes`, sending RFC 5492 NOTIFICATION 2/7 and closing the session on violation.
7. On live reload, `pluginCapabilitiesKept` checks whether any changed enforcement mode violates the live session's `peerCaps` (new `require` where peer lacks the code, or new `refuse` where peer advertised it) and restarts the session if so.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine (Stage 3 and reload verify) | JSON `CapabilityDecl` over `ze-plugin-engine:declare-capabilities` and `config-verify` output (internal DirectBridge and external TLS RPC) | No |
| Plugin infrastructure → BGP reactor | `Server.GetPluginCapabilitiesForSelectors` returning `[]plugin.InjectedCapability` | No |
| BGP speaker ↔ BGP peer (wire) | BGP OPEN (RFC 5492) and NOTIFICATION 2/7 Unsupported Capability | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `docs/`, `test/`; terms `CapabilityDecl`, `declare-capabilities`, `capabilitiesFromRPC`, `InjectedCapability`, `GetPluginCapabilitiesForSelectors`; positive controls: `softver.go:235`, `gr_capability.go:403`, `hostname.go:171`, `exabgp/main_sdk.go:60`, `plugin/server/startup.go:1533`, `plugin/registration.go:507`. Only the plugin IPC `CapabilityDecl` JSON object adds an optional field (`"mode",omitempty`); the BGP wire protocol (OPEN TLV and NOTIFICATION 2/7) follows existing RFC 5492 framing with no new wire format.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Plugin IPC `CapabilityDecl` (`ze-plugin-engine:declare-capabilities` and `config-verify` capability output) | writer | `pkg/plugin/rpc/types.go` (`CapabilityDecl`), `internal/component/bgp/plugins/softver/softver.go` (`extractSoftverCapabilities`), other in-tree capability plugins (`gr`, `hostname`, `role`, `exabgp`), and third-party external plugins | `git grep CapabilityDecl` found all in-tree writers and `docs/plugin-development/protocol.md` | Additive optional field `"mode"` (`omitempty`); `softver` writes `"require"` or `"refuse"`; all other in-tree and third-party writers omitting `"mode"` keep identical bytes meaning `"enable"` |
| Plugin IPC `CapabilityDecl` (`ze-plugin-engine:declare-capabilities` and `config-verify` capability output) | reader | `internal/component/plugin/server/startup.go` (`capabilitiesFromRPC`), `internal/component/plugin/server/config_tx_capabilities.go`, `internal/component/plugin/registration.go` (`CapabilityInjector.planLocked`), `internal/component/bgp/reactor/peer.go` (`Peer.pluginCapabilitySnapshot`) | `git grep capabilitiesFromRPC` and `git grep AddPluginCapabilities` confirmed all readers go through `capabilitiesFromRPC` into `CapabilityInjector` | Updated to validate `"mode"` (`""`/`"enable"`/`"require"`/`"refuse"`), refuse unknown or conflicting modes, allow empty payload for `"refuse"`, and propagate `required`/`refused` codes to the reactor |

### Integration Points
- `pkg/plugin/rpc/types.go` `CapabilityDecl` and `pkg/plugin/sdk/sdk_types.go` - public wire and SDK type.
- `internal/component/plugin/registration.go` `CapabilityInjector` - stores, conflict-checks, and atomically replaces plugin capabilities on startup and reload.
- `internal/component/bgp/reactor/session_handlers.go` `validateCapabilityModes` - single enforcement site for required/refused capabilities.
- `internal/core/bgp/capability/negotiated.go` `CheckRequiredCodes` - single negotiation check for required capability codes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | `bgp-softver` -> `CapabilityDecl` -> `CapabilityInjector` -> `pluginCapabilitySnapshot` -> `validateCapabilityModes` |
| No unintended coupling (components stay isolated) | No | `plugin` and `capability` hold no code-75 or softver knowledge; `mode` is generic across all plugin capabilities |
| No duplicated functionality (extends existing, does not recreate) | No | Reuses `validateCapabilityModes`, `buildUnsupportedCapabilityDataCodes`, `CheckRequiredCodes`, `CheckRefusedCodes`, and `CapabilityInjector` |
| Zero-copy preserved where applicable (refs, not copies) | No | OPEN negotiation path only; no UPDATE forwarding hot-path change |
| Registration over hardcoding, outbound | No | Plugins declare enforcement mode via `CapabilityDecl`; engine enforces generically |
| Registration over hardcoding, inbound | No | Searched `CheckRequiredCodes`, `CapabilityInjector.planLocked`, `pluginCapabilitySnapshot`; no code-75 special case added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A peer's code-75 capability arrives in `peerCaps` as `capability.Unknown` and sets `n.peerCodes[75] = true` | `internal/core/bgp/capability/negotiated.go:208` tracks every parsed capability's code in `peerCodes`, including `Unknown` | `CheckRequiredCodes` / `CheckRefusedCodes` would miss peer code 75 | Unit test `TestCheckRequiredCodesPluginOpaqueCode` + functional test `softver-capability-mode.ci` | unvalidated |
| A-2 | `buildUnsupportedCapabilityDataCodes` encodes `capability.Plugin` (from `localCaps`) and `capability.Unknown` (from `peerCaps`) into valid RFC 5492 Section 5 TLVs | `session_validation.go:1020-1042` calls `cap.Len()` and `cap.WriteTo()` on every matching `capability.Capability` | NOTIFICATION 2/7 Data would be empty or malformed | Unit test asserting NOTIFICATION Data bytes for both `require` and `refuse` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing `CheckRequiredCodes` fallback from `false` to `n.PeerAdvertised(code)` could loosen a future core capability added without a map entry | A new core capability with asymmetric negotiation rules only checks peer presence | Document clearly in `CheckRequiredCodes` that core capabilities with asymmetric rules register in the map while opaque plugin capabilities use `PeerAdvertised`; keep all 10 existing core map entries intact |
| R-2 | Live reload from `enable` to `refuse` leaves an established session up if `pluginDeclarationsKept` only compares wire bytes | Session stays Established after reload when peer sent code 75 | Include enforcement mode in `pluginCapabilitySnapshot` and check reload violations explicitly in `pluginDeclarationsKept` |
| R-3 | External plugin omitting `mode` fails JSON unmarshal if `CapMode` rejects empty string | External plugin conformance tests fail at Stage 3 | Treat omitted or `""` `mode` as `CapModeEnable` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Sessions with `software-version { mode require; }` or `refuse` fail to establish when valid or fail to drop when violating; a bug in `CapabilityDecl` unmarshaling breaks Stage 3 for capability plugins |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `gr`, `hostname`, `role`, and `exabgp` plugins use `CapabilityDecl`; `spec-yang-enum-vocabulary-agreement-gate.md` adds enum mapping checks over `softver.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `session { capability { software-version { mode require; } } }` with peer lacking code 75 and peer sending code 75 | → | `extractSoftverCapabilities` → `CapabilityInjector` → `Peer.pluginCapabilitySnapshot` → `validateCapabilityModes` | `test/plugin/softver-capability-mode.ci` |
| Config `session { capability { software-version { mode refuse; } } }` with peer sending code 75 and peer omitting code 75 | → | `extractSoftverCapabilities` → `CapabilityInjector` → `Peer.pluginCapabilitySnapshot` → `validateCapabilityModes` | `test/plugin/softver-capability-mode.ci` |
| Reload switching `software-version` from `enable` to `refuse` on an established session whose peer sent code 75 | → | `ReplacePluginCapabilities` → `pluginCapabilitiesKept` → session restart → `validateCapabilityModes` | `test/plugin/softver-capability-mode.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Peer configured with `software-version { mode require; }` connects to a peer whose OPEN omits code 75, and separately to a peer whose OPEN carries code 75 | Ze advertises code 75 in its OPEN; rejects the peer lacking code 75 with NOTIFICATION 2/7 carrying Ze's local code-75 TLV and closes the session; reaches Established with the peer sending code 75 | None |
| AC-2 | Peer configured with `software-version { mode refuse; }` connects to a peer whose OPEN carries code 75, and separately to a peer whose OPEN omits code 75 | Ze omits code 75 from its OPEN; rejects the peer sending code 75 with NOTIFICATION 2/7 carrying the peer's code-75 TLV and closes the session; reaches Established with the peer omitting code 75 | None |
| AC-3 | Peer configured with `software-version { mode enable; }` or `mode disable;` (or container absent) | Preserves exact existing behavior: `enable` advertises code 75 without rejecting peers lacking it; `disable` and absent neither advertise nor reject | Existing `draft` and `legacy` encodings unchanged |
| AC-4 | An established session whose peer advertised code 75 under `mode enable` reloads to `mode refuse` (and vice versa: peer lacking code 75 under `mode enable` reloads to `mode require`) | `pluginCapabilitiesKept` detects the mode violation and restarts the session; the reconnected session is rejected with NOTIFICATION 2/7 | A reload changing `mode` when the live peer already satisfies the new mode (for example `enable` to `require` when peer sent code 75) may keep or restart without violating policy |
| AC-5 | Two plugins declare conflicting `Mode` values for the same capability code and scope (or a plugin declares an unknown `Mode` string, or `require`/`enable` with invalid payload) | `CapabilityInjector` refuses the declaration atomically with an `opererr` naming the code, scope, and fix, leaving the live injector store untouched | Two plugins declaring identical `(code, mode, value)` share ownership as today |
| AC-6 | External plugin (over TLS RPC) declares a `CapabilityDecl` with `mode: "require"` or `mode: "refuse"`, or omits `"mode"` | External plugin receives identical OPEN advertisement and `validateCapabilityModes` enforcement as an internal plugin; omitting `"mode"` behaves as `"enable"` | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `software-version { mode require; }` on a peer | Config -> `bgp-softver` -> Stage 3 `CapabilityDecl{Mode: require}` -> `CapabilityInjector` -> OPEN + `validateCapabilityModes` -> NOTIFICATION 2/7 on missing code 75 | `test/plugin/softver-capability-mode.ci` |
| 2 | Sets `software-version { mode refuse; }` on a peer | Config -> `bgp-softver` -> Stage 3 `CapabilityDecl{Mode: refuse}` -> `CapabilityInjector` -> OPEN (no code 75) + `validateCapabilityModes` -> NOTIFICATION 2/7 on peer code 75 | `test/plugin/softver-capability-mode.ci` |
| 3 | Reloads a peer from `mode enable` to `mode refuse` while peer advertises code 75 | Reload verify -> `ReplacePluginCapabilities` -> `pluginCapabilitiesKept` returns false -> session restarts and rejects peer with NOTIFICATION 2/7 | `test/plugin/softver-capability-mode.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseSoftverModes` | `internal/component/bgp/plugins/softver/mode_test.go` | AC-1, AC-2, AC-3 (all 4 modes + unknown mode refusal) | |
| `TestCapabilityDeclModeJSONRoundTrip` | `pkg/plugin/rpc/types_test.go` | AC-5, AC-6 (`""` omitted = enable, require, refuse, unknown refused) | |
| `TestCapabilityInjectorModeConflictsAndRefuse` | `internal/component/plugin/registration_capability_test.go` | AC-5 (refuse allows empty payload, conflicting modes refused atomically) | |
| `TestCheckRequiredCodesPluginOpaqueCode` | `internal/core/bgp/capability/negotiated_test.go` | AC-1 (non-core code checked via `PeerAdvertised`, core map rules preserved) | |
| `TestSetRBitPreservesMode` | `internal/component/bgp/grmarker/rfc4724_grmarker_test.go` | AC-5 / reload safety (`SetRBit` preserves `InjectedCapability.Mode`, matching `SetFBit`) | |
| `TestSessionPluginCapabilityRequireAndRefuse` | `internal/component/bgp/reactor/session_handlers_test.go` | AC-1, AC-2 (NOTIFICATION 2/7 + exact Data TLV bytes for require and refuse) | |
| `TestReloadPluginCapabilityModeRestartsViolatingSession` | `internal/component/bgp/reactor/peer_settings_plugin_capability_test.go` | AC-4 (reload from enable to require/refuse restarts only when violated) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric config inputs added (`mode` is a 4-value enum) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `softver-capability-mode` | `test/plugin/softver-capability-mode.ci` | End-to-end daemon tests for `require` (rejects peer without code 75, accepts peer with code 75), `refuse` (rejects peer with code 75, accepts peer without code 75), and live reload enforcement | |

### Interop Tests (Scope: protocol)
N-A: OPEN wire format for code 75 is already proven against FRR in `test/interop/scenarios/frr-software-version`, and RFC 5492 Section 3/5 NOTIFICATION 2/7 framing uses the shared `validateCapabilityModes` / `buildUnsupportedCapabilityDataCodes` producer already covered by unit and functional tests against `internal/test/peer`.

## Files to Modify
- `pkg/plugin/rpc/types.go` - add optional `Mode CapMode` (`json:"mode,omitempty"`) to `CapabilityDecl`
- `pkg/plugin/rpc/enums.go` - define `CapMode` (`CapModeEnable`, `CapModeRequire`, `CapModeRefuse`) and validation
- `pkg/plugin/sdk/sdk_types.go` - alias `CapMode` and export mode constants
- `internal/component/plugin/registration.go` - carry `Mode` on `InjectedCapability`, check mode conflicts in `CapabilityInjector.planLocked`, allow empty payload on `CapModeRefuse`
- `internal/component/plugin/server/startup.go` - map `CapabilityDecl.Mode` in `capabilitiesFromRPC`
- `internal/core/bgp/capability/negotiated.go` - evaluate non-core required codes in `CheckRequiredCodes` via `n.PeerAdvertised(code)`
- `internal/component/bgp/grmarker/grmarker.go` - copy full `plugin.InjectedCapability` (preserving `Mode`) in `SetRBit` to match `SetFBit`
- `internal/component/bgp/reactor/session_plugin_capabilities.go` - add `required` and `refused` slices to `pluginCapabilitySnapshot`
- `internal/component/bgp/reactor/session_open_proposal.go` - add `pluginRequired` and `pluginRefused` slices to `preparedOpen`
- `internal/component/bgp/reactor/peer.go` - extract `required`/`refused` before `restartFlagsFor` and build wire caps (skipping `refuse`) in `Peer.pluginCapabilitySnapshot`
- `internal/component/bgp/reactor/session_negotiate.go` - populate `pluginRequired` and `pluginRefused` on `preparedOpen` in `prepareOpen`
- `internal/component/bgp/reactor/session_handlers.go` - pass union of settings and plugin required/refused codes to `validateCapabilityModes`
- `internal/component/bgp/reactor/peer_settings_negotiation.go` - include mode changes in `pluginDeclarationsKept`
- `internal/component/bgp/plugins/softver/softver.go` - parse all four modes and emit `CapModeRequire` / `CapModeRefuse`
- `internal/component/bgp/plugins/softver/yang/ze-softver.yang` - update `mode` leaf description to document enforced `require` and `refuse`
- `docs/architecture/api/process-protocol.md` - document optional `mode` field on `CapabilityDecl` in Stage 3 `DeclareCapabilitiesInput` prose
- `docs/architecture/behavior/peer-lifecycle.md` - update `One immutable proposal for each OPEN` to state that reloading to `require` restarts a session whose peer omitted that code
- `docs/architecture/core-design.md` - update `18a. BGP Peer Reload: Swap or Restart` to state that reloading to `require` restarts a session whose remote OPEN omitted that capability
- `docs/plugin-development/protocol.md` - document optional `mode` field in the `CapabilityDecl` field table
- `rfc/short/draft-abraitis-bgp-version-capability.md` - update Ze Implementation notes for `require` and `refuse`

## Files to Create
- `internal/component/bgp/plugins/softver/mode_test.go` - unit tests for `parseSoftverSetting` and `extractSoftverCapabilities` across all four modes
- `test/plugin/softver-capability-mode.ci` - functional test covering `require`, `refuse`, and reload enforcement

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/softver/yang/ze-softver.yang` (`mode` description updated; enum already declares all four values) |
| YANG validation constraints | No | Existing `bgp:capability-mode` enumeration already constrains the leaf |
| YANG custom validators | No | None |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | No | Config leaf only, no CLI command changed |
| Editor autocomplete | No | Already derives all four `bgp:capability-mode` values from YANG |
| Functional test for new RPC/API | Yes | `test/plugin/softver-capability-mode.ci` |
| Pipe completeness | No | No command answer added or changed |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | Session rejections use existing BGP NOTIFICATION logging (`logNotificationReceived`/FSM) |
| Prometheus counters/metrics | No | Existing NOTIFICATION counters already count sent 2/7 notifications |
| BGP family surface | No | Existing capability code 75; no new SAFI or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Existing capability mode contract fixed to match YANG |
| 2 | Config syntax changed? | No | Syntax unchanged; `require` and `refuse` already existed in YANG |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Existing `bgp-softver` plugin behavior fixed |
| 6 | Has a user guide page? | No | No dedicated softver guide page |
| 7 | Wire format changed? | No | Existing RFC 5492 OPEN and NOTIFICATION 2/7 framing |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md` (optional `mode` field on `CapabilityDecl`) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-abraitis-bgp-version-capability.md` (Ze Implementation notes) |
| 10 | Test infrastructure changed? | No | None |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/process-protocol.md` (Stage 3 `DeclareCapabilitiesInput` prose), `docs/architecture/behavior/peer-lifecycle.md` (`One immutable proposal for each OPEN`), `docs/architecture/core-design.md` (`18a. BGP Peer Reload: Swap or Restart`) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors spec plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`. Declared by changed files: `docs/architecture/api/process-protocol.md` (updated: Stage 3 `DeclareCapabilitiesInput` prose notes optional `CapabilityDecl.Mode`), `docs/architecture/behavior/peer-lifecycle.md` (updated: `One immutable proposal for each OPEN` notes the `require` reload-restart exception when the peer omitted the code), `docs/architecture/core-design.md` (updated: `18a. BGP Peer Reload: Swap or Restart` notes the `require` reload-restart exception when the remote OPEN omitted the capability), `docs/architecture/api/ipc_protocol.md` (unaffected: Stage 3 framing and command-answer encoding do not change), `docs/architecture/api/commands.md` (unaffected: no command or pipe behavior changes), `docs/architecture/wire/capabilities.md` (unaffected: already states `require`/`refuse` activation contract for Software Version code 75) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/plugin-development/protocol.md` (`CapabilityDecl` field table) and `docs/architecture/api/process-protocol.md` (Stage 3 `DeclareCapabilitiesInput` prose) updated |

## Design presentation

### What changes

Operators configuring `session { capability { software-version { mode require; } } }`
or `mode refuse;` get the session enforcement the YANG model promises instead of
silent fallback to `enable` and `disable`. Any capability plugin — `bgp-softver`
and third-party external plugins alike — can now declare an optional `mode`
(`enable`, `require`, `refuse`) on `CapabilityDecl`, and the BGP reactor enforces
it through the same RFC 5492 Unsupported Capability NOTIFICATION (2/7) path used
for core capabilities, both on initial OPEN and on live config reload.

### Before and after

1. **`software-version { mode require; }` against a peer without code 75.**
   Before: Ze advertises code 75 and the session reaches Established even though the peer omitted code 75.
   After: Ze advertises code 75, receives the peer's OPEN without code 75, sends NOTIFICATION Error 2 Subcode 7 carrying Ze's code-75 TLV, and closes the session.
2. **`software-version { mode refuse; }` against a peer sending code 75.**
   Before: Ze omits code 75 and the session reaches Established even though the peer advertised code 75.
   After: Ze omits code 75, receives the peer's OPEN carrying code 75, sends NOTIFICATION Error 2 Subcode 7 carrying the peer's code-75 TLV, and closes the session.
3. **Reloading `software-version` from `mode enable;` to `mode refuse;` on an established session whose peer advertised code 75.**
   Before: Reload succeeds and the established session remains up.
   After: Reload detects the enforcement violation against the peer's OPEN, restarts the session, and rejects reconnection with NOTIFICATION 2/7 until the peer stops advertising code 75.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Narrow `ze-softver.yang` `mode` enum to `enable` and `disable` | Removes operator control over one capability merely because it lives in a plugin; Thomas's T-4 ruling (2026-11-24) explicitly chose implementing `require` and `refuse` |
| Parse `software-version/mode` directly inside `reactor/config_capabilities.go` | Violates `ai/doors/bgp-engine.md` and `core-design.md` ("core knows nothing, and everything registers"): the reactor must not hardcode plugin-owned capability config leaves |
| Add a code-75 case inside `capability.CheckRequiredCodes` | Hardcodes a plugin-owned capability code inside `internal/core/bgp/capability`; evaluating non-core codes via `n.PeerAdvertised(code)` supports every plugin capability generically |

### Documents departed from

1. **`docs/architecture/behavior/peer-lifecycle.md` (`One immutable proposal for each OPEN`, lines 295–298)**
   - **Before:** "Successful semantic admission retains the immutable remote OPEN under the Session lock before negotiation advances. Reload therefore judges a pending connection against its own accepted capabilities, including the rule that a changed declaration may remain when the peer did not advertise that code."
   - **After:** Reload still judges a pending or established connection against its accepted remote OPEN, and a changed non-enforcing declaration whose code the peer did not advertise may still remain, except when the reload sets `mode require` for a code the peer did not advertise (or `mode refuse` for a code the peer did advertise), which MUST restart the session so `validateCapabilityModes` rejects it with RFC 5492 NOTIFICATION 2/7.
2. **`docs/architecture/core-design.md` (`18a. BGP Peer Reload: Swap or Restart`, lines 4018–4025)**
   - **Before:** "The Session's admitted remote OPEN is visible to this check before negotiation advances; a changed capability the remote did not advertise can remain."
   - **After:** The Session's admitted remote OPEN is visible to this check before negotiation advances; a changed capability the remote did not advertise can remain unless the reload requires that capability (making the peer's OPEN non-compliant and forcing a restart), just as refusing a capability the remote advertised forces a restart.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `Mode` to `rpc.CapabilityDecl`, `plugin.InjectedCapability`, and `pluginCapabilitySnapshot`; write failing unit tests and failing `test/plugin/softver-capability-mode.ci`.
2. **Phase: Plugin IPC and CapabilityInjector** -- implement `CapMode` JSON marshaling/validation, conflict checks, and non-advertised `CapModeRefuse` storage in `pkg/plugin/rpc`, `pkg/plugin/sdk`, `internal/component/plugin/registration.go`, and `internal/component/plugin/server/startup.go`.
3. **Phase: Reactor and Core Capability Enforcement** -- update `capability.CheckRequiredCodes`, `grmarker.SetRBit`, `Peer.pluginCapabilitySnapshot`, `session_open_proposal.go` (`preparedOpen`), `session_negotiate.go`, `session_handlers.go`, and `peer_settings_negotiation.go` for initial OPEN and live reload enforcement.
4. **Phase: Softver Plugin and Docs** -- update `softver.go` and `ze-softver.yang`, update `docs/architecture/api/process-protocol.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/core-design.md`, `docs/plugin-development/protocol.md`, and `rfc/short/draft-abraitis-bgp-version-capability.md`, verify all unit and functional tests pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | `refuse` never injects code 75 into local OPEN; `require` injects code 75 and includes local code-75 TLV in NOTIFICATION 2/7 Data; `refuse` includes peer's code-75 TLV in NOTIFICATION 2/7 Data |
| Naming | JSON field `"mode"` and values `"enable"`, `"require"`, `"refuse"` match `ze-bgp-conf.yang` `capability-mode` |
| Data flow | No softver or code-75 knowledge added to `plugin` or `core/bgp/capability` |
| Operator errors and doctor codes | Conflicting or unknown `CapMode` refusals use `opererr` naming code, scope, and fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `CapabilityDecl.Mode` + `CapabilityInjector` mode handling | `./le test unit pkg path ./pkg/plugin/rpc/ && ./le test unit pkg path ./internal/component/plugin/` |
| Generic `CheckRequiredCodes` + `grmarker.SetRBit` + reactor enforcement + reload restart | `./le test unit pkg path ./internal/core/bgp/capability/ && ./le test unit pkg path ./internal/component/bgp/grmarker/ && ./le test unit pkg path ./internal/component/bgp/reactor/` |
| Softver 4-mode parsing + functional proof | `./le test unit pkg path ./internal/component/bgp/plugins/softver/ && ./le test functional plugin` (`softver-capability-mode.ci`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Unknown `CapMode` strings from an external plugin are refused at Stage 3 / verify; `require` with empty/invalid payload is refused before OPEN encoding |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

Moved here on 2026-11-26 from `plan/release/fixes/spec-yang-single-source-of-truth.md` (sole owner of the `software-version/mode` half per T-4; the `bgp/rib/adj-rib-in`/`adj-rib-out` half is owned by `plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md`).

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-09-04 | `session/capability/software-version/mode` offers four values, implements two |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add generic optional `Mode` (`CapMode`) to `rpc.CapabilityDecl` | Narrow YANG enum to 2 values; parse `software-version/mode` in reactor | Preserves operator capability control, honors T-4, keeps BGP core ignorant of plugin capabilities, and works identically for internal and external plugins |
| Evaluate non-core required capability codes via `Negotiated.PeerAdvertised(code)` | Hardcode code 75 in `CheckRequiredCodes` | Registration over hardcoding: supports `bgp-softver` and any future or third-party plugin capability without core edits |
| Allow empty payload on `CapModeRefuse` and omit it from local OPEN | Require dummy payload on `refuse` | A refused capability is never advertised in local OPEN; carrying no payload avoids fabricating wire bytes |

## Known Limitations
- `ZeVersion = "Ze/0.1.0"` build-stamp tracking is outside this spec's scope (this spec enforces `mode` only).
- The `bgp/rib/adj-rib-in` and `adj-rib-out` config-false subtree mentioned in the same 2026-09-04 journal row is owned by `plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md`.

## RFC Documentation (Scope: protocol)

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 5492 §§3, 5 | Unrecognized capabilities ignored by default; when policy requires or refuses a capability, OPEN Message Error NOTIFICATION (2/7) carries the causing capability TLV(s) encoded as in OPEN | `mode enable` (advertise, no enforcement), `mode disable` (do not advertise, no enforcement), `mode require`, `mode refuse` | `TestSessionPluginCapabilityRequireAndRefuse`, `test/plugin/softver-capability-mode.ci` | `TestParseSoftverModes`, `test/plugin/softver-capability-mode.ci` |
| draft-abraitis-bgp-version-capability §§3, 4 | Capability 75 disabled by default when container is absent; `draft` UTF-8 framing is default when enabled | `encoding legacy` (1-octet length prefix for FRR/ExaBGP interop, owner deviation D-13) | Existing `softver` unit tests + `test/plugin/softver-capability-mode.ci` | Existing `frr-software-version` interop scenario + `softver` unit tests |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | - |
| `./le spec review check` | not run |
| Rounds | 0 |
| Reviewer lenses used | - |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./pkg/plugin/rpc/`, `./le test unit pkg path ./internal/component/plugin/`, `./le test unit pkg path ./internal/core/bgp/capability/`, `./le test unit pkg path ./internal/component/bgp/grmarker/`, `./le test unit pkg path ./internal/component/bgp/reactor/`, `./le test unit pkg path ./internal/component/bgp/plugins/softver/`, `./le test functional plugin`, and the suites `./le repo changed scope print both` reaches. No `./le verify` command
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests N-A (no numeric inputs)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A (OPEN encoding unchanged, already covered by `frr-software-version`)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal row marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-software-version-mode-implements-two-of-four.md` only
