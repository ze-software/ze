# Spec: bgp-open-session-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Four defects in how Ze builds and reports its BGP OPEN exchange, found by the
RFC re-read and the softver interop work of
`spec-rfc-requirement-quote-hand-backfill`. Two are RFC
defects: a peer misframes Ze's OPEN, and a per-peer override gives peers
different BGP Identifiers. Two are Ze behaviour defects that no RFC sentence
states, each recorded by its journal row: a changed capability does not reach
the wire after a reload, and a peer refusing Ze's OPEN leaves no line at the
default log level. Each was confirmed open in HEAD on 2026-09-27.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|--------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9072 Section 2 (RFC9072-2-1) | "The rules for encoding Optional Parameters are unchanged with respect to those given in [RFC4271], except that the Parameter Length field is extended to be a two-octet unsigned integer." | `internal/component/bgp/reactor/session_negotiate.go::buildOptionalParams` | The one-octet form is chosen while `capTotal <= 255`, so a capability total of 254 or 255 yields 256 or 257 octets of Optional Parameters with `ExtendedParams` false; `Open.WriteTo` (`internal/component/bgp/message/open.go`) then writes the extended envelope (optLen above 255) around a parameter with a one-octet length, which a conformant receiver misframes. The boundary is 2 plus capTotal | no verdict; read, not reproduced |
| D4 | RFC 6286 Section 2.1 (RFC6286-2.1-1) | "The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." | `internal/component/bgp/reactor/config.go::parsePeerSettings` ("Router ID from session > router-id (peer-level overrides global)") | A peer-level `router-id` gives that peer a BGP Identifier different from the others'; `docs/features/rfc-status.md` advertises the override as supported. The sentence carries no BCP 14 keyword and sits in the Section 2.1 definition; the row labels it MUST | weak (TestParsePeerFromTreeInvalid asserts only non-zero); OWNER DECISION |

### Ze behaviour defects (no RFC sentence)

Neither row is an RFC defect: no RFC sentence states the behaviour, so neither
carries an RFC tag, a discrimination record or a conformance count. Each is
owed because an operator meets it, and its journal row is its record.

| ID | Journal row | Quote | Producer | What Ze does wrong | Record |
|----|-------------|-------|----------|--------------------|--------|
| D2 | `plan/journal/unwired-feature.md`, 2026-09-27 | none: a reload defect | `sdk.Plugin.Run` capability declaration at stage 3; `internal/component/bgp/plugins/softver/softver.go` `OnConfigure`; same shape in `hostname`, `role`, `llnh`, `gr` | A SIGHUP reload that changes a plugin-declared capability (softver `encoding`) bounces the session, but the new OPEN keeps the startup capability (code reading; wire not captured) | journal row, Open |
| D3 | `plan/journal/silent-fall-through.md`, 2026-09-27 | none: an observability defect | `internal/component/bgp/reactor/session_handlers.go::Session.handleNotification`, the reconnect loop in `peer_run.go` | A NOTIFICATION received in answer to Ze's OPEN is logged only at DEBUG, so a peer that refuses the OPEN never comes up and no line says why (seen in the `frr-software-version` interop scenario) | journal row, Open |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a unit test at capTotal 253, 254, 255 and 256 asserting the envelope and the per-parameter length agree. D2: a test reloading a changed softver encoding and asserting the next OPEN. D3: a test asserting the WARN line |
| Both polarities | D1: 253 stays one-octet with no envelope; 254 upward uses the RFC 9072 form throughout. D4, if ruled a defect: a peer-level router-id refused at config load, the global one accepted |
| Discrimination | `./le rfc discriminate-record` for RFC9072-2-1, and for RFC6286-2.1-1 if D4 is ruled a defect; none for D2 and D3, which carry no RFC tag |
| Real entry point | `.ci` tests for D2 (reload) and D3 (peer refuses OPEN) |
| Interop | D1 against FRR with a capability set sized to 254 octets; D2 and D3 through `frr-software-version` |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | Re-declare capabilities on config apply, or refuse a capability change the reload cannot apply? |
| D4 | Remove the per-peer `router-id` override, or keep it as an owner-approved deviation from RFC 6286 Section 2.1, recorded with its reason, not counted as conformant, and the `docs/features/rfc-status.md` claim corrected? |

### Owner questions (design phase, 2026-10-13)

Each question asks which way to fix, never whether. The design below proceeds
under the recommended answer and names what changes under the other.

| # | Row | Question | Recommended | What the other answer changes |
|---|-----|----------|-------------|-------------------------------|
| Q1 | D2 | (a) Re-declare: the five capability plugins recompute their declaration on config apply and the engine replaces it before the peer restarts; (b) refuse: config-verify refuses a reload that changes a plugin-declared capability or adds a peer that would carry one, naming the field and "restart ze"; (c) restart the plugin process on such a reload (the `restart.go` path re-runs stage 3) | (a) | (b): AC-2 becomes the refusal (and a peer ADDED on reload with a plugin capability is refused too), no SDK/RPC change, phases 3-4 shrink to verify handlers in five plugins. (c): no SDK change, but every peer's OPEN validation in `role` stops while the plugin restarts, and a transaction now restarts processes |
| Q2 | D4 | Remove the peer- and group-level `session router-id` leaf (one BGP Identifier per speaker), or keep it as an owner-approved deviation | **Answered by the owner, 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows". Keep the leaf as an owner-approved deviation from RFC 6286 Section 2.1 (Deviation branch: AC-6 documents it, RFC6286-2.1-1 leaves the conformance count).** Original note: no recommendation, because both are coherent. Removal: 102 of the 160 interop `ze.conf` files that set a router-id set none at the `bgp` level, and about 92 `.ci` files carry more than one distinct router-id value, so removal rewrites a large share of the test corpus and interacts with OWNER RULING 5 of 2026-10-12 (`plan/pre-release/spec-config-yang-when-unique-choice.md`: a router-id every session carries counts as present) | Deviation: no config or test-corpus change; RFC6286-2.1-1 becomes `{partial}`/deviation in `rfc/short/rfc6286.md`, leaves the conformance count, and AC-4 cannot bring it to `enforced` (AC-4 then covers RFC9072-2-1 only, plus the deviation record) |
| Q3 | D4 | The same Section 2.1 sentence says the identifier is "determined on startup". Ze applies a changed global `router-id` on SIGHUP and restarts sessions (`bgp-reload-global-router-id`, `test/reload/reload-global-router-id.ci`). Is that live change part of the D4 ruling, or out of scope? | Out of scope: the reload re-runs startup for the affected sessions and every peer still sees one identifier | If in scope: refuse a reload that changes the global router-id, which reverses the shipped `reload-global-router-id` behaviour; owner must say so explicitly |
| Q4 | D1 | Fix the threshold in place in the two packers that carry the defect (reactor `buildOptionalParams`, chaos `buildOptionalParams`), or consolidate the four Optional Parameters packers (reactor, chaos, perf, bmp Loc-RIB) into one producer | In place | Consolidation moves `buildOptionalParams`, which RFC-tagged tests call by name (`rfc9072_param_type_test.go` RFC9072-3-2, `rfc6286_routerid_unique_test.go`, `rfc8050_migration_epoch_test.go`, `session_first_as_test.go`), so those tagged tests change and need owner approval and fresh discrimination records. That approval is given: **owner, 2026-10-13: "approve all RFC-tagged test changes"**, covering the tagged units consolidation would change, for whichever Q4 answer is recorded; it does not answer Q4. Commands under "Owner approval commands" in the TDD plan |

## Re-verification at HEAD `3e2770062b` (2026-10-13)

No row is fixed. Nothing is dropped.

| Row | Still open? | Evidence |
|-----|-------------|----------|
| D1 | Yes | `buildOptionalParams` still takes the one-octet branch on `capTotal <= 255` (`internal/component/bgp/reactor/session_negotiate.go`, the `if capTotal <= 255` line); the Optional Parameters field it returns is `2+capTotal` octets, so capTotal 254 and 255 give 256 and 257 octets with `ExtendedParams` false, and `Open.WriteTo` takes the extended envelope on `optLen > 255`. No commit touched the function since the skeleton (`git log` on the file: latest `12f2cd322a`, which changed reload bindings, not this function) |
| D1 sibling | Yes, new | `internal/chaos/peer/session.go::buildOptionalParams` writes one type-2 parameter per capability with one-octet lengths and never sets `ExtendedParams`, so above 255 total octets `Open.WriteTo` wraps one-octet parameters in the extended envelope: the same misframing, in the chaos test peer. `internal/perf/session.go` (three capabilities) and `internal/component/bgp/plugins/bmp/bmp_locrib.go::fabricateLocRIBOpen` (bounded set) cannot reach 254 |
| D2 | Yes | `sdk.Plugin.SetCapabilities` is documented "Must be called before Run()" and is read once at stage 3 (`pkg/plugin/sdk/sdk.go`, "Stage 3: declare-capabilities"); the engine has no runtime declare method (the `ze-plugin-engine:` method list holds `declare-capabilities` only as a startup stage); `CapabilityInjector` entries are removed only by `releasePluginRegistrations` (`internal/component/plugin/server/restart.go`). `softver`, `hostname`, `role` register `OnConfigure` only; `llnh` and `gr` add an `OnConfigVerify` refusal and no apply. A reload that edits a peer's capability block restarts the peer (`peerSettingsSwapPlan`, `CapabilityConfigJSON` differs), and `buildOpen` reads plugin capabilities through `pluginCapGetter`, i.e. the stage-3 store. `negotiatedCapabilitySettings`' comment (`peer_settings_negotiation.go`) records the same gap |
| D3 | Yes | `Session.handleNotification` logs only the RFC 8203 shutdown message (INFO) or an invalid one (WARN); the received code and subcode reach the counters and the report bus (`incrNotificationReceived`, `raiseNotificationError`: visible in `ze show errors`), and the reconnect loop logs `safeRunOnce returned` at DEBUG. `slogutil.Logger` defaults to WARN, so at the default level an OPEN refusal writes no log line. The skeleton's "WARN or INFO" is narrowed to WARN: an INFO line is invisible at the default level |
| D4 | Yes | `parsePeerSettings` (`internal/component/bgp/reactor/config.go`): "Router ID from session > router-id (peer-level overrides global)"; the `peer-fields` grouping of `ze-bgp-conf.yang` carries `leaf router-id` ("Override the router ID for this peer.") |
| RFC9072-2-1 verdict | weak | `rfc/audit/rfc9072.json`: "the procedure's two-octet Parameter Length per parameter is not asserted in this row's units" |
| RFC6286-2.1-1 verdict | weak | `rfc/audit/rfc6286.json`: "Ze accepts a per-peer router-id override ... nothing tagged goes red on it" |
| `plan/pre-release/spec-rfc-verdict-fix-bgp.md` | 2 rows point here | RFC6286-2.1-1 and RFC9072-2-1, both `weak`; the parent's Wiring note names this spec's AC-4 |
| `./le rfc check` | exit 0 | 4147 gated requirements; it prints no per-row verdict for these two, which the audit files above carry |
| False claim found | Yes | The `rfc/short/rfc6286.md` RFC6286-2.1-1 row annotation says the value "is read once at config load into reactor.Config.RouterID and used for every peer and every OPEN", which the per-peer override contradicts. `ai/rules/rfc-compliance.md` requires the correction immediately; AC-5 carries it |
| R48 (`plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`) | Not this spec | The RFC 4271 Section 8.2.2 "any other event" row was routed to the verdict-fix child, not here; `docs/architecture/behavior/fsm-open-sent.md` now documents `fsmMessageEvent` sending 5/0 for it. Named so it is not mistaken for a dropped row |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - reload order of plugin sections against BGP peer operations
  → Constraint: a plugin that declares the `bgp` root and does not decompose is a coarse node, placed in phase 5 BEFORE the decomposed peer starts ("the `bgp` root's peers start after the plugins that configure the RIB, the graceful-restart state and the filters have applied their sections"). A D2 re-declaration made in the plugin's config-apply therefore lands before the restarted peer builds its OPEN. Validated by the D2 `.ci`, not assumed (A-1)
- [ ] `docs/architecture/api/process-protocol.md` - the 5-stage handshake; stage 3 `ze-plugin-engine:declare-capabilities`
  → Constraint: capabilities are declared once, in stage 3, by `DeclareCapabilitiesInput`; a runtime re-declaration is a protocol change this page must describe in the same change (Q1 (a))
- [ ] `docs/architecture/behavior/fsm-open-sent.md` - received NOTIFICATION in OpenSent (Event 24 vs 25)
  → Constraint: Event 25 in OpenSent is answered with 5/0 by `fsmMessageEvent`; the D3 log line MUST NOT change that FSM behaviour, only add the WARN
- [ ] `docs/architecture/testing/interop.md` - scenario layout, typed checkers, `opWaitLogContains`, the revert-to-red rule
  → Constraint: every interop scenario is a named directory with a typed checker in `internal/le/interoplab/bgp`; a reload scenario follows `bgp-reload-global-router-id` (`ze-reload.conf`, `opExec` copy, `opSignal` HUP)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9072.md`, `rfc/full/rfc9072.txt` Section 2
  → Constraint: RFC 9072 Section 2: "However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below." The length is the WHOLE Optional Parameters field, parameter headers included, so with Ze's single type-2 parameter it is 2 + capTotal and the boundary is capTotal 254
  → Constraint: RFC 9072 Section 2: "The rules for encoding Optional Parameters are unchanged with respect to those given in [RFC4271], except that the Parameter Length field is extended to be a two-octet unsigned integer." The envelope and every parameter inside it must agree (`Open.ExtendedParams` exists for this)
  → Constraint: RFC 9072 Section 2: "In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration." So 255 octets of parameters (capTotal 253) stays one-octet: the negative polarity
- [ ] `rfc/short/rfc6286.md`, `rfc/full/rfc6286.txt` Section 2.1
  → Constraint: RFC 6286 Section 2.1: "For a BGP speaker that supports the AS-wide Unique BGP Identifier, the BGP Identifier is specified as the following: ... The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." The sentence carries no BCP 14 keyword (RFC 6286 has no BCP 14 boilerplate); the summary labels the row MUST. RFC 4271 Section 4.2 carries the same sentence ("The value of the BGP Identifier is determined upon startup and is the same for every local interface and BGP peer."); `rfc/short/rfc4271.md` has no row for it, and RFC scope is frozen (no new rows planned here)
- [ ] `rfc/full/rfc8654.txt` Section 6 - OPEN size
  → Constraint: RFC 8654 Section 6 keeps OPEN at 4096 octets ("This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages."), so the D1 boundary tests stay far below any message-size limit

**Key insights:**
- D1 is an off-by-two: the one-octet branch must hold only while 2 + capTotal <= 255
- D2 has no runtime path: declaration is stage 3 only, removal only on plugin restart
- D3 is visible today only in `ze show errors` and counters, never in the log at the default WARN level
- D4 is an owner decision with a large test-corpus blast radius for removal

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `buildOptionalParams` packs every capability into ONE type-2 parameter; one-octet form while `capTotal <= 255`, extended form (3 + capTotal, `ExtendedParams` true) above, `slog.Error` and nil parameters above 65535. `buildOpen` is the one OPEN producer, called by `sendOpen` and by the reload probe `negotiationOutcomeUnchanged`
  → Constraint: `buildOpen` must stay the single producer; the reload probe compares OPENs it builds
- [ ] `internal/component/bgp/message/open.go` - `Open.Len`/`Open.WriteTo` take the extended envelope when `ExtendedParams || optLen > 255`; `writeToExtended` writes 255/255 and a two-octet length
  → Constraint: preserve; the envelope decision is correct, the packer's framing choice is what is wrong
- [ ] `internal/chaos/peer/session.go` - `buildOptionalParams` (chaos test peer): one parameter per capability, one-octet lengths, no `ExtendedParams`
  → Decision: the sibling of D1, fixed in the same change (AC-1b)
- [ ] `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk.go` - `SetCapabilities` stores the set for stage 3; `OnConfigVerify`, `OnConfigApply`, `OnConfigRollback` exist for the reload pipeline
  → Constraint: an apply handler that fails MUST undo what it applied; the rollback handler runs after every accepted apply (`OnConfigApply` doc comment)
- [ ] `internal/component/plugin/server/startup.go` - `onCapabilities` converts and `capInjector.AddPluginCapabilities`; a conflict is an error
- [ ] `internal/component/plugin/registration.go` - `CapabilityInjector.AddPluginCapabilities`, `RemovePluginCapabilities`
  → Decision: Q1 (a) adds a replace operation keyed by plugin name that swaps one plugin's set atomically under the same conflict check
- [ ] `internal/component/bgp/plugins/softver/softver.go`, `hostname/hostname.go`, `role/role.go`, `llnh/llnh.go`, `gr/gr.go` - each extracts per-peer `CapabilityDecl` in `OnConfigure` and calls `SetCapabilities`; `llnh` and `gr` also refuse in `OnConfigVerify`
  → Constraint: `role` also keeps `filterPeerConfigs` for `OnValidateOpen`; a re-declaration must update that state in the same apply or a reload changes the advertised role without changing the validated one
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `peerSettingsSwapPlan`: a changed `CapabilityConfigJSON` forces a restart; `peer_settings_negotiation.go::negotiatedCapabilitySettings` documents why
  → Constraint: preserve the restart; D2 makes the restarted OPEN right, it does not turn the restart into a swap
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleNotification`: counters and report bus via `onNotifRecv`, INFO for an RFC 8203 message, FSM event, close, returns `ErrNotificationRecv`
- [ ] `internal/component/bgp/reactor/peer_run.go` - reconnect loop logs `safeRunOnce returned` at DEBUG; "session closed" INFO only on leaving Established
- [ ] `internal/core/slogutil/slogutil.go` - `Logger`: default level WARN
  → Constraint: D3's line must be WARN to be seen at the default level
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerSettings` reads `session router-id` per peer and group, overriding the global; `parseRouterID` refuses 0.0.0.0
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - global `leaf router-id` (mandatory unless every session carries one) and the `peer-fields` `leaf router-id` override
- [ ] `internal/test/peer/expect.go`, `internal/test/peer/peer.go` - ze-peer `option=open:value=inspect-open-message` checks the received OPEN against `expect=bgp` rules; `action=notification|sighup|rewrite` exist
- [ ] `test/reload/reload-capability-restart-names-both-fields.ci` - the model for a capability-block reload `.ci` (rewrite + sighup + Cease 6/6 + conn 2)
- [ ] `test/interop/scenarios/frr-software-version/`, `internal/le/interoplab/bgp/check_softver.go` - FRR refuses the draft form with 2/0; the checker reads Ze's `show bgp peer ... detail`

**Behavior to preserve:**
- OPENs whose Optional Parameters fit 255 octets stay byte-identical (every `test/encode` and `test/decode` OPEN expectation)
- The reload restart decision and its logged reason (`reload-capability-restart-names-both-fields.ci`)
- The FSM's answer to a received NOTIFICATION (`fsm-open-sent.md` table), the counters and the `ze show errors` entries
- `allow-shared-router-id` and the RFC 6286 Section 2.2/2.3 behaviour

**Behavior to change:** D1, D1 sibling, D2, D3, and D4 per Q2; the false RFC6286-2.1-1 annotation.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Session start: `sendOpen` → `buildOpen` (D1)
- SIGHUP / `ze config` commit: the config transaction (verify, apply, operation graph) (D2, D4)
- A NOTIFICATION read in OpenSent or OpenConfirm (D3)
- Config load: `parsePeerSettings` (D4)

### Transformation Path
1. Config load: plugins receive the `bgp` section at stage 2 and declare per-peer capabilities at stage 3; the engine stores them in `CapabilityInjector`
2. `buildOpen` merges configured capabilities with `pluginCapGetter` (injector), coalesces PATHS-LIMIT, calls `buildOptionalParams`, returns `message.Open`
3. `Open.WriteTo` chooses the envelope; `writeMessage` sends it
4. Reload: verify sections to plugins, apply coarse plugin sections, then peer operations; a changed capability block restarts the peer, and the new connection repeats 2-3
5. Receive: `handleNotification` counts, reports, runs the FSM event, closes; the peer loop reconnects after backoff

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | stage-3 `declare-capabilities` today; Q1 (a) adds a runtime re-declaration over the same RPC connection | No (design) |
| Engine ↔ Reactor | `pluginCapGetter` reads the injector per OPEN | Yes, by reading `buildOpen` |
| Ze ↔ peer | OPEN wire bytes | Yes, by reading `Open.WriteTo` |

### Integration Points
- `CapabilityInjector` - Q1 (a) replaces one plugin's set; conflict check unchanged
- `sdk.Plugin` - Q1 (a) adds a method usable in Running stage that sends the new set
- The five capability plugins - each gains verify/apply/rollback handlers that recompute the set from the candidate config

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | capabilities still reach the OPEN only through the injector and `buildOpen` |
| No unintended coupling | Yes (design) | the reactor does not learn plugin capability semantics; plugins still own their declarations |
| No duplicated functionality | Partly | four Optional Parameters packers remain (Q4) |
| Zero-copy preserved | Yes | the OPEN path allocates once per connection today; unchanged |
| Registration over hardcoding, outbound | Yes (design) | a re-declaring plugin uses the SDK call; no per-plugin case in the engine |
| Registration over hardcoding, inbound | Yes (design) | searched: `ze-plugin-engine:` method list (`startup_driver.go`), `CapabilityInjector`; the replace is keyed by the plugin name the plugin registered, no list learns a plugin name |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Session start with a capability set of 254 octets | → | `buildOptionalParams` → `Open.WriteTo` | `test/encode/open-optional-params-254.ci` (new: ze-peer `inspect-open-message` on the exact extended OPEN hex) |
| SIGHUP with a changed softver `encoding` | → | softver config-apply → SDK re-declaration → `CapabilityInjector` replace → `buildOpen` on the restarted connection | `test/reload/reload-softver-encoding-reaches-open.ci` (new) |
| SIGHUP adding a peer with `software-version` | → | same path, new peer | `test/reload/reload-add-peer-softver-capability.ci` (new) |
| ze-peer answers Ze's OPEN with NOTIFICATION 2/0 | → | `handleNotification` WARN line | `test/plugin/open-refused-notification-warn.ci` (new: `expect=stderr` on the WARN) |
| Config with a peer-level `router-id` (Q2 = remove) | → | YANG / `parsePeerSettings` refusal | `test/parse/peer-router-id-refused.ci` (new); under Q2 = deviation this row is deleted with the reason |

## Acceptance Criteria

AC-1 to AC-4 keep the skeleton's intent. AC-2's either/or is resolved by Q1
(recommended (a)); AC-3 is narrowed from "WARN or INFO" to WARN because the
default level is WARN.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `buildOpen` with capabilities totalling 254 or 255 octets (and 256) | The OPEN carries Non-Ext OP Len 255, Non-Ext OP Type 255, Extended Opt. Parm. Length = 3 + capTotal, and one type-2 parameter whose two-octet Parameter Length = capTotal; a conforming decode (`message.UnpackOpen` then `capability.ParseFromOptionalParams`) returns every capability unchanged |
| AC-1n | capabilities totalling 253 octets (and 0, 1) | The OPEN uses the RFC 4271 form: Opt Parm Len = 2 + capTotal (255 at 253), no 255/255 marker, one-octet Parameter Length; bytes identical to today |
| AC-1b | chaos test peer with a capability set whose parameters exceed 255 octets | The chaos peer's OPEN round-trips through `message.UnpackOpen` and `capability.ParseFromOptionalParams` with every capability intact (its parameters use the RFC 9072 two-octet framing whenever the envelope is extended) |
| AC-2 | SIGHUP that changes `capability software-version encoding` on a running peer (legacy → draft) | The peer restarts (Cease 6/6, unchanged) and the OPEN on the new connection carries capability 75 in the draft form; the startup form is absent |
| AC-2b | SIGHUP that adds a new peer with `software-version` | The new peer's first OPEN carries capability 75 |
| AC-2c | SIGHUP that removes `software-version` from a peer | The restarted OPEN carries no capability 75 |
| AC-2d | The same reload-then-OPEN check for `hostname`, `role`, `llnh` and `gr` capability changes | Each restarted OPEN carries the new value; `role`'s OPEN validation uses the new role on the same connection |
| AC-2e | A reload whose transaction fails after a plugin applied its new set | The plugin's declaration returns to the previous set (rollback), and the next OPEN carries the previous capability |
| AC-3 | A NOTIFICATION received in OpenSent or OpenConfirm (Ze's OPEN refused) | Exactly one WARN line per received NOTIFICATION naming the peer, the FSM state, the code and subcode as names and numbers, and the RFC 8203 message when present; visible with no `ze.log` override |
| AC-3n | A NOTIFICATION received in Established | The same one WARN line (one line per NOTIFICATION, whatever the state); the existing INFO "peer shutdown communication" line is folded into it, not duplicated (no-layering) |
| AC-4 | The `weak` verdicts of RFC6286-2.1-1 and RFC9072-2-1, after this spec's producer fix | Each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Under Q2 = deviation, RFC6286-2.1-1 instead carries the deviation record (`{partial}`, reason, not counted conformant) and its verdict is re-judged against that record. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (parent P-3, 2026-09-28) |
| AC-5 | `rfc/short/rfc6286.md` RFC6286-2.1-1 annotation and the `docs/features/rfc-status.md` RFC 6286 row | Neither claims the identifier is the same for every peer while the per-peer override exists; done FIRST, before any code, because `ai/rules/rfc-compliance.md` does not let a false claim wait |
| AC-6 (Q2 = remove) | A config with `session router-id` at peer or group level | Refused at load, `ze config validate` and editor commit, naming the leaf and RFC 6286 Section 2.1; the global `router-id` is accepted and used for every peer; a dynamic group takes the global identifier |
| AC-6 (Q2 = deviation) | A config with a peer-level `router-id` | Accepted as today; the YANG description and `docs/guide/configuration.md` say the override departs from RFC 6286 Section 2.1 |
| AC-7 | `plan/journal/unwired-feature.md` (2026-09-27) and `plan/journal/silent-fall-through.md` (2026-09-27) rows | Marked fixed with the commit, in the closing commit |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures enough families and capabilities that the set is 254 octets, peers with FRR | config → plugins stage 3 → `buildOpen` → `buildOptionalParams` → `Open.WriteTo` → FRR | `bgp-open-extended-params-boundary-frr` |
| 2 | Changes `software-version encoding` and sends SIGHUP | transaction → softver apply → re-declare → injector → peer restart → `buildOpen` | `reload-softver-encoding-reaches-open.ci`, `frr-software-version-reload` |
| 3 | Peers with a speaker that refuses Ze's OPEN, reads the log | wire NOTIFICATION → `handleNotification` → WARN | `open-refused-notification-warn.ci`, `frr-software-version` (log assertion added) |
| 4 | (Q2 = remove) writes a peer-level router-id | YANG load → refusal | `peer-router-id-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC9072OptionalParamsBoundary` (table: capTotal 0, 1, 253, 254, 255, 256, 300) | `internal/component/bgp/reactor/rfc9072_optional_params_boundary_test.go` (new) | AC-1, AC-1n: builds the OPEN through `buildOpen`-equivalent packing plus `Open.WriteTo`, reads the envelope octets and walks the parameters with the framing the envelope announces; tagged `RFC9072-2-1 positive` (254, 255, 256 extended with two-octet Parameter Length) and `RFC9072-2-1 negative` (253 and below carry no marker and one-octet length; the tag quotes the Section 2 SHOULD sentence, see the audit table below) | |
| `TestChaosPeerOpenRoundTripsAbove255` | `internal/chaos/peer/session_test.go` | AC-1b | |
| `TestCapabilityInjectorReplacePluginCapabilities` | `internal/component/plugin/registration_test.go` | replace swaps one plugin's set atomically, keeps the conflict check, leaves other plugins untouched (Q1 (a)) | |
| `TestSDKRedeclareCapabilitiesRunning` | `pkg/plugin/sdk/` | the running-stage call reaches the engine and is refused before Running | |
| `TestSoftverApplyRedeclares` and the same for `hostname`, `role`, `llnh`, `gr` | each plugin's `_test.go` | verify computes, apply re-declares, rollback restores (AC-2, AC-2d, AC-2e) | |
| `TestRoleApplyUpdatesValidateOpenConfig` | `internal/component/bgp/plugins/role/` | AC-2d role half | |
| `TestHandleNotificationLogsWarn` | `internal/component/bgp/reactor/session_handlers_test.go` | AC-3, AC-3n: a captured logger at WARN sees exactly one line with peer, state, code, subcode; with an RFC 8203 message the message is in that line and no second line exists | |
| (Q2 = remove) `TestParsePeerRouterIDRefused` | `internal/component/bgp/reactor/config_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| capTotal, one-octet form | 0-253 | 253 | N/A | 254 (must switch to RFC 9072 form) |
| capTotal, RFC 9072 form | 254-65532 (Extended Opt. Parm. Length = 3 + capTotal <= 65535) | 65532 | 253 (must stay RFC 4271 form) | 65533 (refused with an error, see R-4) |

### RFC-tagged tests that change (owner approval)

| Test | Tag | Changes under | Why |
|------|-----|---------------|-----|
| none | - | Q1 (a), Q4 in place, Q2 deviation | D1 adds new tagged tests only; existing tagged tests (`TestOpenPackExtendedParams`, `TestTheExtendedEnvelopeAndItsParametersAgree`, `TestRFC9072ParameterType255OnlyAsTheExtendedMarker`) use sets of 14 and 300+ octets and stay as they are |
| `internal/component/bgp/reactor/config_test.go::TestParsePeerFromTreeInvalid` (`zero_router_id_rejected` case) | RFC6286-2.1-1 | Q2 = remove | the case feeds a peer-level `router-id 0.0.0.0`, which removal makes an unknown leaf; the case moves to the global leaf |
| Every RFC-tagged test whose config text or tree sets a peer- or group-level `router-id` | various (86 tagged test files mention `router-id` at some level; the peer-level subset is enumerated in Phase 5 step 1 by `./le` grep before any edit) | Q2 = remove | input rewrite only; each needs owner approval and, where its claim's unit changes, a fresh discrimination record |
| `rfc9072_param_type_test.go`, `rfc6286_routerid_unique_test.go`, `rfc8050_migration_epoch_test.go`, `session_first_as_test.go` | RFC9072-3-2, RFC6286-2.1-2, RFC8050, RFC6793 rows | Q4 = consolidate | they call `buildOptionalParams` by name |

#### Independent audit of the tagged tests (2026-10-13, Q2 answered: deviation)

Source: `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/rfc-tagged-test-compliance-review.md`, section 11.
Re-checked against `config_test.go` at HEAD, `rfc/full/rfc6286.txt` Section 2.1 and `rfc/full/rfc9072.txt` Section 2; agreed on both rows.

| Tagged unit | Audit verdict | What the change owes |
|-------------|---------------|----------------------|
| `config_test.go::TestParsePeerFromTreeInvalid`, case `zero_router_id_rejected` (RFC6286-2.1-1 positive) | COMPLIANT, unchanged under the deviation | Its claim is limited to "non-zero" ("The BGP Identifier is a 4-octet, unsigned, non-zero integer", RFC 6286 Section 2.1), which the body proves by refusing a peer-level `0.0.0.0`. Under the deviation it is the Tested scope of the `{partial}` row in `rfc/short/rfc6286.md`, "the same for every local interface and every BGP peer" is the declared deviation gap, and the row contributes nothing to the conformance count. Body and tag unchanged |
| new `TestRFC9072OptionalParamsBoundary` (RFC9072-2-1 positive and negative) | COMPLIANT once the negative's prose is fixed | The boundary at capTotal 254 is right: the field length is 2 + capTotal, and the MUST binds when it "does exceed 255". The negative (capTotal 253 and below stay in the base form) is NOT proved by the MUST sentence: the same paragraph says "an implementation MUST accept an OPEN message that uses the encoding of this specification even if the length of the Optional Parameters is 255 or less", so the extended form at or below 255 is permitted. The base form there is the SHOULD sentence plus Ze's choice. The negative tag line must quote it: RFC 9072 Section 2, "In the event that the length of the Optional Parameters in the BGP OPEN message does not exceed 255, the encodings of the base BGP specification [RFC4271] SHOULD be used without alteration." That sentence is enrolled as its own row, RFC9072-2-7 [SHOULD]; the same 253 case may also carry an `RFC9072-2-7 positive` tag, with its own discrimination record, since it proves that row directly |

No existing tagged test changes under Q1 (a), Q2 deviation and Q4 in place: confirmed.

#### Owner approval commands

Approved by the owner on 2026-10-13: "approve all RFC-tagged test changes". Under the recommended answers (Q1 (a), Q4 in place) and the answered Q2 (deviation), no existing tagged unit changes and none of these is needed. Q2 = remove is moot since Q2 is answered, so `TestParsePeerFromTreeInvalid` and the router-id rewrite rows need no command. The two commands below apply only if Q4 is answered "consolidate". They are the tagged units at HEAD `051876c3f4` that call `buildOptionalParams` in the four files Q4 names: in `rfc6286_routerid_unique_test.go` the call sits in the untagged `TestOpenAdvertisedASReadsAS4Capability`, and `session_first_as_test.go` carries no tag, so neither file needs a command. The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer, so the owner runs these at the prompt of the session that commits the consolidation. A unit is `<package directory>.<top-level test function>`.

2 commands (Q4 = consolidate only):

```
./le rfc approve unit reactor.TestRFC9072ParameterType255OnlyAsTheExtendedMarker reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 consolidate only: buildOptionalParams call moves)'
./le rfc approve unit reactor.TestMRTMigrationEpochUsesActualLocalOPEN reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-bgp-open-session-rfc-defects, Q4 consolidate only: buildOptionalParams call moves)'
```

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `open-optional-params-254` | `test/encode/open-optional-params-254.ci` | Ze's OPEN with a 254-octet capability set is the exact RFC 9072 form | |
| `reload-softver-encoding-reaches-open` | `test/reload/` | legacy → draft on SIGHUP; conn 2 OPEN inspected (AC-2) | |
| `reload-add-peer-softver-capability` | `test/reload/` | AC-2b | |
| `reload-softver-removed-leaves-open` | `test/reload/` | AC-2c | |
| `reload-hostname-reaches-open`, `reload-role-reaches-open` | `test/reload/` | AC-2d (the two with a value payload; `llnh` and `gr` are covered by unit tests plus one `.ci` each if ze-peer can inspect their codes, A-3) | |
| `open-refused-notification-warn` | `test/plugin/` | ze-peer answers the OPEN with 2/0; `expect=stderr:contains=` the WARN with peer, state, code, subcode; no `ze.log` override | |
| (Q2 = remove) `peer-router-id-refused` | `test/parse/` | AC-6 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-open-extended-params-boundary-frr` (new) | `test/interop/scenarios/` | FRR | Ze's OPEN at capTotal 254 (sized with `software-version` padding plus families) reaches Established with FRR and FRR reports Ze's capabilities (D1). Red phase: revert the threshold, rebuild the Ze image, FRR refuses the OPEN | |
| `frr-software-version-reload` (new, or `ze-reload.conf` added to `frr-software-version` if the checker can run both halves) | `test/interop/scenarios/` | FRR | SIGHUP flips peer legacy to draft; FRR then refuses with 2/0 (its known draft-form refusal), which proves the new form reached the wire (D2) | |
| `frr-software-version` (extended) | `test/interop/scenarios/frr-software-version/` | FRR | `opWaitLogContains` on Ze's WARN naming FRR's 2/0 for peer draft (D3) | |

## Files to Modify
- `internal/component/bgp/reactor/session_negotiate.go` - D1 threshold, RFC quote above it
- `internal/chaos/peer/session.go` - D1 sibling
- `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk.go` - running-stage re-declaration (Q1 (a))
- `pkg/plugin/rpc/` - the RPC input type if a new method name is used (A-2)
- `internal/component/plugin/server/` (dispatch of the runtime declaration), `internal/component/plugin/registration.go` - replace in `CapabilityInjector`
- `internal/component/bgp/plugins/softver/softver.go`, `hostname/hostname.go`, `role/role.go`, `llnh/llnh.go`, `gr/gr.go` - verify/apply/rollback re-declaration
- `internal/component/bgp/reactor/peer_settings_negotiation.go` - the `negotiatedCapabilitySettings` comment that records the stale plugin store (stale-comments rule)
- `internal/component/bgp/reactor/session_handlers.go` - D3 WARN
- (Q2 = remove) `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/bgp/reactor/config.go`, `internal/component/config/validate_mandatory_bgp.go` (`EverySessionCarries`), test configs; (Q2 = deviation) the YANG description only
- `rfc/short/rfc6286.md`, `rfc/short/rfc9072.md`, `rfc/audit/rfc6286.json`, `rfc/audit/rfc9072.json`, `rfc/discrimination/rfc9072.json` (and `rfc6286.json` under removal)
- `plan/journal/unwired-feature.md`, `plan/journal/silent-fall-through.md`

## Files to Create
- the unit tests, `.ci` tests and interop scenarios named above, each with its typed checker in `internal/le/interoplab/bgp`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Q2 = remove: Yes | `ze-bgp-conf.yang` drops the `peer-fields` `router-id` leaf; otherwise No |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | No | no new leaf |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | Q2 = remove: Yes, derived | the leaf leaves completion with the schema |
| Functional test for new RPC/API | Yes | the plugin re-declaration is exercised by the `test/reload/` `.ci` files |
| Pipe completeness | N-A | no command output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary added |
| Prometheus counters/metrics | No | the NOTIFICATION counters exist; D3 adds a log line only |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new capability code; existing capabilities change only in when they are declared |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope changed? | No | defects |
| 2 | Config syntax changed? | Q2 = remove: Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | the engine RPC is plugin protocol, row 8 |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (capability plugins now apply a reload) |
| 6 | Has a user guide page? | Yes | `docs/guide/monitoring.md` (the NOTIFICATION WARN line) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/capabilities.md` (the 254-octet boundary) |
| 8 | Plugin SDK/protocol changed? | Yes (Q1 (a)) | `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md` if it states declare-once |
| 9 | RFC behavior changed or proven? | Yes | `rfc/short/rfc9072.md`, `rfc/short/rfc6286.md`, `docs/features/rfc-status.md` rows (generated from the summaries' Meta) |
| 10 | Test infrastructure changed? | Only if A-3 fails | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/apply-ordering.md` if the re-declaration ordering needs a sentence; `docs/architecture/behavior/fsm-open-sent.md` (WARN on Event 24/25) |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered plugin/capability inventory changed? | No | - |
| 16 | Changed sources referenced by doc anchors? | Derived at implementation | `./le spec citation anchors spec plan/immediate/spec-bgp-open-session-rfc-defects.md`. Declared by `// Design:` headers of files this spec changes: `docs/architecture/chaos-web-dashboard.md` (`internal/chaos/peer/session.go`): unaffected, it describes the chaos dashboard, not the chaos peer's OPEN framing; `docs/architecture/core-design.md` (`internal/component/bgp/reactor/config.go`): unaffected by D1-D3 (it states no capability-declaration or router-id rule); under Q2 = remove re-read it for any per-peer router-id statement |
| 17 | Existing docs show examples for this area? | Q2 = remove: Yes | every doc example with a peer-level `router-id` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Plugin config-apply for the `bgp` root runs before the restarted peer sends its OPEN | `docs/architecture/config/apply-ordering.md` phase 5 coarse-node paragraph | The restarted OPEN still carries the old capability; the re-declaration must instead be pushed in config-verify-accepted order or the peer restart deferred | `reload-softver-encoding-reaches-open.ci` | unvalidated |
| A-2 | The existing `ze-plugin-engine:declare-capabilities` method can be accepted in Running stage, or a sibling method name is cleaner | `startup_driver.go` handles it as a stage, `methodDeclareCapabilities` | A new RPC name and YANG for it | design review in Phase 3 | unvalidated |
| A-3 | ze-peer `inspect-open-message` can check the OPEN of connection 2 after a reload | `peer.go` per-connection handling | ze-peer extension, `docs/functional-tests.md` row | first D2 `.ci` | unvalidated |
| A-4 | ze-peer can answer Ze's OPEN with a NOTIFICATION before or instead of a KEEPALIVE (`action=notification` at conn 1 seq 1) | `expect.go` action list | a ze-peer `open` option to refuse | first D3 `.ci` | unvalidated |
| A-5 | FRR accepts an RFC 9072 OPEN of 254+ octets and reaches Established | FRR implements RFC 9072 (`bgp_open.c` extended opt params) | interop scenario uses another daemon (GoBGP, BIRD) | `bgp-open-extended-params-boundary-frr` | unvalidated |
| A-6 | `hostname`, `role`, `llnh`, `gr` derive their declaration only from the config the reload delivers | each plugin's `OnConfigure` | a plugin with runtime-derived capability state needs its own design | Phase 4 per plugin | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A re-declaration conflicts with another plugin's capability code for the same peer mid-transaction | `AddPluginCapabilities` conflict error at apply | apply fails and the plugin undoes; conflict also checked in verify so the transaction refuses first |
| R-2 | Rollback after a plugin re-declared leaves the new set in the injector | AC-2e test red | rollback handler re-declares the previous set; test AC-2e |
| R-3 | `role` advertises one role and validates another after reload | `TestRoleApplyUpdatesValidateOpenConfig` red | update both in one apply |
| R-4 | capTotal above 65532 (or an OPEN above RFC 8654's 4096) today logs ERROR and sends an OPEN with no capabilities: a silently wrong OPEN | code reading of the `maxExtendedParamLen` branch | out of this spec's four rows; one row in `plan/journal/silent-fall-through.md` at implementation start, not fixed here (rule-precedence: closing first) |
| R-5 | The D3 WARN floods the log when a peer refuses every retry | ConnectRetry cadence in `frr-software-version` (nine refusals) | one line per received NOTIFICATION is bounded by the reconnect backoff; FRR and BIRD log every received NOTIFICATION at their default level too |
| R-6 | Q2 = remove rewrites ~100 interop and ~90 `.ci` configs and touches RFC-tagged tests | Phase 5 grep count | owner decides Q2 with this cost stated; deviation path has none |
| R-7 | Reload probe (`negotiationOutcomeUnchanged`) now sees a re-declared plugin set if the plugin applied before the probe runs, and could turn a restart into a swap | `reload-capability-restart-names-both-fields.ci` reason string changes | the `CapabilityConfigJSON` field still forces the restart; the test pins it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | D1: OPENs of 254-255 octets (today broken) and, if the threshold were moved wrongly, every OPEN; the encode/decode `.ci` corpus pins the common sizes. D2: capability plugins on reload. D3: log only. D4 removal: every config using a per-peer router-id is refused |
| How is it reverted? | Single commit per row; D4 removal needs operator configs to regain the leaf |
| Who else touches this path? | `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` (OPEN and session handling), `plan/immediate/spec-bgp-live-receive-binding-and-bfd-keep-session.md` (reload swap plan in `peer_settings_apply.go`, in flight), `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (verdict rows), `plan/pre-release/spec-config-yang-when-unique-choice.md` (router-id mandatory ruling) |

## Implementation Steps

1. **Phase: Wiring and the false claim (FIRST)** - AC-5 edit of `rfc/short/rfc6286.md` and the rfc-status row; write the failing wiring tests of the Wiring Test table
   - Verify: each new test fails for the stated reason
2. **Phase: D1** - `TestRFC9072OptionalParamsBoundary` red at 254/255, fix the threshold with the RFC 9072 Section 2 quote above it, green; chaos sibling; `./le rfc discriminate-record` for each new RFC9072-2-1 tag; `open-optional-params-254.ci`
3. **Phase: D2 engine** - injector replace, SDK running-stage call, server dispatch; unit tests
4. **Phase: D2 plugins** - softver first (the journal row), then hostname, role, llnh, gr; reload `.ci` per AC-2..AC-2e; correct the `negotiatedCapabilitySettings` comment
5. **Phase: D4 per Q2** - removal: grep the peer- and group-level `router-id` uses, list the RFC-tagged ones for owner approval before editing, then YANG, parser, `EverySessionCarries`, configs; deviation: summary `{partial}` with the reason, YANG and guide wording
6. **Phase: D3** - `TestHandleNotificationLogsWarn` red, WARN line, fold the RFC 8203 INFO into it, `.ci`
7. **Phase: interop** - the three scenarios with revert-rebuild-red records
8. **Phase: verdicts** - AC-4: fresh-agent `./le rfc audit-stamp ... mode rejudge` for both rows; journal rows marked fixed (AC-7)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the threshold compares the Optional Parameters field length (2 + capTotal), not capTotal; 253 stays byte-identical |
| Correctness | the WARN carries code and subcode names and numbers and the FSM state; one line, not two, when an RFC 8203 message exists |
| Data flow | capabilities still enter the OPEN only through `buildOpen`; no reactor code learns softver/hostname/role semantics |
| Rule: rfc-compliance | RFC 9072 Section 2 quote above the threshold; new tags carry discrimination records; AC-5 done first |
| Rule: no-layering | the INFO RFC 8203 line is replaced, not kept beside the WARN; the stage-3-only declaration is replaced, not doubled |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| D1 fixed | `TestRFC9072OptionalParamsBoundary` and `open-optional-params-254.ci` pass; revert red recorded |
| D2 fixed | reload `.ci` files pass; `frr-software-version-reload` passes with red recorded |
| D3 fixed | `open-refused-notification-warn.ci` passes |
| D4 decided and applied | per Q2 |
| Verdicts | `rfc/audit/rfc9072.json` RFC9072-2-1 `enforced`; RFC6286-2.1-1 per Q2 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a plugin's runtime re-declaration is accepted only from that plugin's own connection and only for its own name; a NOTIFICATION's RFC 8203 message is logged through the existing sanitising path (`ShutdownMessage`), never raw bytes |
| Resource exhaustion | a re-declaration size is bounded by the same checks as stage 3 |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A-1 broken (apply after peer restart) | DESIGN: revisit Q1 with the owner |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D1: compare 2 + capTotal with 255 in the existing packer | consolidate four packers (Q4) | minimum change, no RFC-tagged test touched |
| D2: plugins re-declare on apply (Q1 (a)) | refuse the reload (b); restart the plugin (c); build capabilities in the engine from `RawCapabilityConfig` | (a) keeps the reload useful and the plugin the owner of its capability; (b) is simplest but refuses ordinary operator edits and must also refuse a peer added with the capability; (c) restarts processes inside a transaction and stops `role` validation meanwhile; the engine option breaks the plugin boundary |
| D3: one WARN per received NOTIFICATION in `handleNotification` | WARN only before Established; raise the reconnect-loop DEBUG to WARN | the producer is the one place that knows code, subcode and state; one rule for all states matches FRR and BIRD defaults; the reconnect loop sees only an error string |

## Known Limitations
- R-4 (OPEN with capabilities above the two-octet or 4096-octet limit) is recorded as a journal row, not fixed here
- RFC 4271 Section 4.2 carries the same "same for every local interface and BGP peer" sentence and has no row; RFC scope is frozen, so no row is added

## RFC Documentation (Scope: protocol)

Add `// RFC 9072 Section 2: "However, if the length of the Optional Parameters in the BGP OPEN message does exceed 255, the OPEN message MUST be encoded according to the procedure below."` above the threshold, and the RFC 6286 Section 2.1 quote above the removal refusal under Q2 = remove.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for capTotal
- [ ] Functional `.ci` tests
- [ ] Interop tests

### Verification
- [ ] `./le verify worktree`

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-open-session-rfc-defects.md`
