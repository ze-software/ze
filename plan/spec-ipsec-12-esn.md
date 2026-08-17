# Spec: ipsec-12-esn -- Extended Sequence Numbers (ESN) for ESP Child SAs

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ipsec-0-umbrella (native IKEv2 engine, XFRM/VPP dataplane) |
| Phase | - |
| Updated | 2026-06-20 |

Depends note (2026-07-22 plan review): `spec-ipsec-0-umbrella` is marked
skeleton, but the substance this spec actually depends on -- the native IKEv2
engine and XFRM/VPP dataplane -- IS on disk
(`internal/component/ike/{engine,dataplane,crypto,wire}`; ipsec children
landed as learned 734/735/739/742/1069/1072/1141/1215). The real dependency
is satisfied; this spec is implementable now. Anchors verified exact
(`crypto/transform.go`, `wire/payload_sa.go`,
`engine/initiator.go`); no `USE_ESN` yet.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc7296.md` (Section 3.3.2 Transform Type 5), `rfc/short/rfc4303.md` (Section 3.3.3 sequence numbers / anti-replay)
4. Source files: `internal/component/ike/engine/initiator.go` (`buildWireESPProposals`), `internal/component/ike/engine/auth.go`, `internal/component/ike/engine/fsm.go`, `internal/component/ike/engine/child.go`, `internal/component/ike/dataplane/{dataplane,xfrm_linux,vpp}.go`, `internal/component/ike/ipsec/{types,config}.go`, `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`

## Task

Add Extended Sequence Number (ESN) support to Ze's native IKEv2/IPsec implementation. ESN
(RFC 7296 Transform Type 5; 64-bit anti-replay sequence space per RFC 4303 Section 3.3.3)
extends the ESP anti-replay counter from 32 to 64 bits, which is required for high-throughput
SAs that would otherwise exhaust the 32-bit sequence space and force frequent rekeying.

Deliver three layers:
1. **Config surface** -- a per-proposal `esn` leaf (`required` | `optional` | `disabled`,
   default `disabled`) on both `esp-group/proposal` and `ike-group/proposal`.
2. **Negotiation** -- emit/parse the ESN transform (Transform Type 5, value 0 = No ESN,
   value 1 = ESN) in the ESP SA payload, replacing the current hardcoded "No ESN", and
   select an agreed value as initiator and responder.
3. **Dataplane** -- set the ESN flag on the installed Child SA in both backends: kernel
   XFRM (`XfrmState.ESN`) and VPP (`USE_ESN` flag on the SA entry).

The `esn` leaf on `ike-group/proposal` is accepted for configuration symmetry but is inert:
ESN is an ESP/AH-only transform and MUST NOT appear in an IKE SA proposal (RFC 7296
Section 3.3.2). This is a deliberate, documented limitation (see Known Limitations).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint: annotations. -->
- [ ] `plan/spec-ipsec-0-umbrella.md` -- IPsec scope, engine/dataplane split
  -> Constraint: IKEv2 engine is backend-agnostic; the SA/SP installer abstraction
     (`dataplane.Dataplane`) is the only path to the kernel/VPP. ESN flag must travel through
     `dataplane.SAParams`, not via a backend-specific side channel.
  -> Constraint: route-based (XFRM interface, if_id bound) tunnel mode only. ESN applies to
     the Child/ESP SA, which is always tunnel mode here.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` -- XFRM netlink backend
  -> Constraint: Child SA install builds `netlink.XfrmState`; anti-replay set via
     `state.ReplayWindow`. ESN requires `state.ESN = true`, and the kernel then expects the
     ESN replay structure (XFRMA_REPLAY_ESN_VAL) rather than the legacy replay value.
- [ ] `docs/architecture/ike/ipsec-6-ikev2-crypto.md` -- proposal negotiation
  -> Decision: `crypto.NegotiateESP` exists but is currently DEAD CODE; the engine uses
     `espGroup.Proposals[0]` directly for the Child SA. ESN intent is therefore taken from
     the first ESP proposal, not from a negotiated selection.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` Section 3.3.2 -- Transform Type 5 (ESN)
  -> Constraint: ESN transform IDs: 0 = "No Extended Sequence Numbers", 1 = "Extended
     Sequence Numbers". An AH/ESP proposal MUST contain an ESN transform. To offer a choice,
     include TWO ESN transforms (0 and 1) in the same proposal; the responder selects exactly
     one. Order of same-type transforms is not significant.
  -> Constraint: ESN transform MUST NOT appear in an IKE SA proposal (Transform Type 5 is
     valid only for AH and ESP). Existing code already omits it for IKE (`initiator.go`).
- [ ] `rfc/short/rfc4303.md` Section 3.3.3 -- ESP sequence numbers / anti-replay
  -> Constraint: With ESN, the high-order 32 bits of the 64-bit counter are maintained by
     each peer but not transmitted; both peers MUST agree on ESN use or anti-replay breaks.
- [ ] (optional) RFC 4304 -- ESN addendum (historical, IKEv1 DOI). No summary present; not
     required because the IKEv2/ESP behavior is fully specified by RFC 7296 + RFC 4303.
  -> Decision: do NOT block on creating a short RFC 4304 summary; cite 7296/4303 in code.

**Key insights:**
- Wire support for Transform Type 5 already exists (`transform.go`, `payload_sa.go`);
  only the ESP proposal builder hardcodes value 0, and inbound ESN transforms are ignored.
- The shared `buildChildSAPayloads` (used by both initiator SAi2 and responder SAr2 via
  `auth.go`) calls `buildWireESPProposals`, so ESN emission is centralized in one builder.
- `payload_sa_test.go:TestProposalTransformNested` proves the wire encoder round-trips multiple
  transforms in a proposal -- the `optional` two-ESN-transform offer is wire-safe.
- The Child SA uses `espGroup.Proposals[0]`; the agreed ESN value still depends on the peer's
  selection (especially for `optional`), so the chosen transform must be parsed from the peer
  message and carried to `installChildSA`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/crypto/transform.go` -- `TransformTypeESN TransformType = 5`
  -> Constraint: crypto-layer transform type constant already present.
- [ ] `internal/component/ike/wire/payload_sa.go` -- `TransformTypeESN uint8 = 5`; wire
  `Transform{Type, ID, Attrs}`; `Proposal.Transforms` is a slice (multiple same-type allowed).
  -> Constraint: encoder iterates `Transforms` in order; emitting two ESN transforms is valid.
- [ ] `internal/component/ike/engine/initiator.go` -- `buildWireESPProposals` (294-321)
  hardcodes `{Type: TransformTypeESN, ID: 0}` at line 310; `buildWireIKEProposals` (107-135)
  correctly omits ESN; `wireProposalsToIKE` (177-179) parses then discards ESN for IKE.
  -> Constraint: line 310 is the single point that currently forces No-ESN on every ESP SA.
- [ ] `internal/component/ike/engine/auth.go` -- responder builds SAr2 via
  `buildChildSAPayloads(sa)` (114); sets `sa.ChildInboundSPI` (118).
  -> Constraint: responder and initiator share the same ESP proposal builder.
- [ ] `internal/component/ike/engine/fsm.go` -- IKE_AUTH response handler (initiator) reads
  the response `PayloadSA` proposals (407-412) but extracts ONLY the SPI; transforms ignored.
  -> Constraint: this is where the initiator must read the responder's chosen ESN transform.
- [ ] `internal/component/ike/engine/child.go` -- `ChildSA` struct (46-58, no ESN field);
  `createFirstChildSA` uses `prop := espGroup.Proposals[0]` (99); `installChildSA` (180-275)
  builds inbound/outbound `dataplane.SAParams` with `ReplayWin: replayWindow` but no ESN.
  -> Constraint: ESN must be added to `ChildSA`, set from the negotiated value, and copied to
     both `SAParams`.
- [ ] `internal/component/ike/engine/rekey.go` -- also uses `espGroup.Proposals[0]` (94) and
  calls `installChildSA` (132).
  -> Constraint: rekey must preserve the SA's ESN value so the rekeyed SA matches.
- [ ] `internal/component/ike/engine/sa.go` -- `SA` struct (53); child negotiation fields
  `ChildInboundSPI`/`ChildOutboundSPI`/`NegotiatedTSi`/`NegotiatedTSr` (113-116).
  -> Constraint: add an analogous negotiated-ESN field here (the established pattern for
     "value learned during IKE_AUTH and consumed by Child SA creation").
- [ ] `internal/component/ike/dataplane/dataplane.go` -- `SAParams` (29-50, no ESN);
  `Dataplane` interface (73-80).
  -> Constraint: add `ESN bool` to `SAParams`; no interface signature change needed.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` -- `InstallSA` (21-66) sets
  `state.ReplayWindow` when `ReplayWin > 0`; no ESN.
  -> Constraint: vendored `netlink` v1.1.0 `XfrmState` has `ESN bool` (xfrm_state.go);
     set `state.ESN = p.ESN`.
- [ ] `internal/component/ike/dataplane/vpp.go` -- `InstallSA` (34-68); `ipsecSAEntry`
  (155-169) has NO `Flags` field; current code sets `Salt:0, UDPSrcPort:0, UDPDstPort:0`.
  -> Constraint: VPP `vl_api_ipsec_sad_entry_t` carries a `flags` field; adding it shifts the
     binary layout, which must match VPP's wire format exactly (see Risk R-1).
- [ ] `internal/component/ike/ipsec/types.go` -- `ESPProposal` (343-347) / `IKEProposal`
  (358-363); enum pattern (names map + `Parse*` + `String`), e.g. `PFSMode` (116-149).
  -> Constraint: add `ESNMode` enum following the existing pattern; add `ESN ESNMode` to both
     proposal structs. Zero value MUST equal `disabled` to preserve current behavior.
- [ ] `internal/component/ike/ipsec/config.go` -- `parseESPProposal` (179-213), `parseIKEProposal`
  (330-377) read leaves via `t.Get`.
  -> Constraint: parse `esn` leaf the same way; absent -> `ESNDisabled`.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` -- `typedef encryption-algo` (13),
  `typedef hash-algo` (24); `esp-group/proposal` (70-91); `ike-group/proposal` (160-190).
  -> Constraint: add `typedef esn-mode` near other typedefs; add `leaf esn` to both proposal
     lists with `default "disabled"`.

**Behavior to preserve:**
- Default (no `esn` configured) MUST produce identical wire output to today: a single ESN
  transform with value 0 on each ESP proposal, and No-ESN Child SAs. Zero-value enum = disabled.
- IKE SA proposals MUST continue to contain NO ESN transform, regardless of config.
- `peersEqual` / `IPsecConfig.Changed` semantics: an ESN change on a proposal MUST count as a
  peer change so the SA is reinstalled (proposals participate via the group reference).
- Existing AEAD/non-AEAD ESP transform construction (`initiator.go`) unchanged.

**Behavior to change:**
- `buildWireESPProposals` emits ESN transforms per the proposal's `ESN` mode instead of a
  hardcoded value 0.
- Initiator parses the responder's chosen ESN transform from the IKE_AUTH response SA.
- Responder selects an ESN value from the offered transforms against local policy and echoes a
  single chosen ESN transform in SAr2.
- `installChildSA` sets the ESN flag on both dataplane SAs.

## Data Flow (MANDATORY)

### Entry Point
- Operator config: `vpn ipsec esp-group <g> proposal <n> esn <required|optional|disabled>`
  (and the inert `ike-group ... esn`). Format at entry: YANG enum string.

### Transformation Path
1. Config tree -> `parseESPProposal` / `parseIKEProposal` -> `ESPProposal.ESN` /
   `IKEProposal.ESN` (`ESNMode`).
2. Initiator: `buildChildSAPayloads` -> `buildWireESPProposals(...)` emits ESN transforms:
   `disabled`=[0], `required`=[1], `optional`=[0,1]. (IKE builder still emits none.)
3. Initiator: IKE_AUTH response handler (`fsm.go`) reads the single ESN transform the responder
   selected in SAr2 -> stores agreed value on `SA` (e.g. `ChildESN bool` + a "set" marker).
4. Responder: IKE_AUTH request handler selects ESN from the offered transforms vs local policy
   (`esnNegotiate`), stores agreed value on `SA`, and `buildWireESPProposals` emits the single
   chosen ESN transform in SAr2.
5. `createFirstChildSA` / `rekey` -> `ChildSA.ESN` = agreed value.
6. `installChildSA` -> `dataplane.SAParams.ESN` (inbound + outbound).
7. XFRM backend: `state.ESN = p.ESN`. VPP backend: set `USE_ESN` flag on the SA entry.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> ipsec types | `t.Get("esn")` -> `ParseESNMode` | [ ] |
| Engine <-> wire (SA payload) | ESN `wire.Transform` in `Proposal.Transforms` | [ ] |
| Engine <-> dataplane | `SAParams.ESN bool` | [ ] |
| Dataplane <-> kernel | `netlink.XfrmState.ESN` (XFRMA_REPLAY_ESN_VAL) | [ ] |
| Dataplane <-> VPP | `ipsecSAEntry.Flags` USE_ESN bit | [ ] |

### Integration Points
- `buildWireESPProposals` (single ESN emission point, shared initiator/responder).
- `SA` negotiated-value fields (same pattern as `ChildOutboundSPI`/`NegotiatedTSi`).
- `dataplane.SAParams` (single struct both backends consume).

### Architectural Verification
- [ ] No bypassed layers (ESN travels config -> engine -> SAParams -> backend)
- [ ] No unintended coupling (backends read only `SAParams.ESN`)
- [ ] No duplicated functionality (one ESN builder; one negotiation helper)
- [ ] Zero-copy preserved where applicable (no new per-event allocation in hot path)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `netlink` v1.1.0 sets the kernel ESN replay structure (XFRMA_REPLAY_ESN_VAL) when `XfrmState.ESN=true` and `ReplayWindow>0` | xfrm_state.go has `ESN bool`; kernel requires ESN replay attr | XFRM SA add fails or anti-replay misbehaves | QEMU: `ip xfrm state` shows `flag esn` + replay-window; SA add returns nil | unvalidated |
| A-2 | VPP `USE_ESN` flag = bit 0x01 and `flags` field position in `vl_api_ipsec_sad_entry_t` is known | VPP IPsec API `ipsec_sad_flags` | wrong offset corrupts ALL VPP SA installs | govpp binding / VPP `.api.json`; VPP interop SA add retval==0 | unvalidated |
| A-3 | Child SA ESN intent can be taken from `espGroup.Proposals[0].ESN` (no real per-proposal ESP negotiation) | child.go, rekey.go; `NegotiateESP` dead | multi-proposal ESN policies misbehave | code read confirmed; documented as Known Limitation | confirmed |
| A-4 | strongSwan (interop peer) includes an ESN transform and selects exactly one | RFC 7296 Section 3.3.2 | `optional` negotiation ambiguous | strongSwan interop: tunnel up with `esn=yes` both ends | unvalidated |
| A-5 | The wire encoder emits multiple same-type (ESN) transforms in `Proposal.Transforms` order and a peer accepts the proposal | payload_sa.go; payload_sa_test.go:TestProposalTransformNested | `optional` offer rejected | unit roundtrip test + interop | unvalidated |
| A-6 | An `esn` change on a proposal triggers SA reinstall via existing change detection | types.go `Changed`/`peersEqual` operate on peer refs to groups | stale SA keeps old ESN | unit test: changing esn marks peer changed | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding `Flags` to `ipsecSAEntry` shifts the VPP binary-API layout; mismatch corrupts every SA install | VPP `ipsec sa add` retval != 0; all VPP tunnels break | Verify field order against govpp-generated `vl_api_ipsec_sad_entry_t`; add a struct-layout/encode test; gate behind VPP interop before claiming done |
| R-2 | ESN replay-window semantics: with ESN the kernel needs the ESN replay struct; legacy replay value rejected | `XfrmStateAdd` error or replay drops in QEMU | Set `state.ESN` AND ensure replay window expressed via ESN attr; verify with `ip xfrm state`; if lib insufficient, set replay window explicitly for ESN |
| R-3 | `optional` mode: peer echoes both/zero ESN transforms -> ambiguous agreed value | Initiator parses != 1 ESN transform in SAr2 | Default to No-ESN when the response does not carry exactly one ESN transform; log a warning |
| R-4 | ESN mismatch installed silently (one side ESN, other not) -> anti-replay drops all packets | Tunnel up but no traffic passes; replay-fail counters climb | Derive the dataplane flag strictly from the negotiated transform, never from raw local config; interop test asserts traffic flows |
| R-5 | Inert `ike-group esn` leaf confuses operators | Support questions; config that "does nothing" | YANG description states it applies to the Child SA only; AC-9 proves IKE wire carries no ESN; documented Known Limitation |
| R-6 | Responder selection placed in the wrong handler (no `responder.go`; responder path is in `auth.go`/`fsm.go`) | Responder ignores offered ESN, always No-ESN | Locate the IKE_AUTH request handler that processes incoming SAi2; add negotiation there; unit-test the pure `esnNegotiate` helper independently |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `esp-group g proposal 1 esn required` in config | -> | `parseESPProposal` sets `ESN=ESNRequired` -> `buildWireESPProposals` emits ESN transform id=1 -> `installChildSA` -> `SAParams.ESN=true` -> `XfrmState.ESN=true` | `test-ipsec-esn-required` (QEMU functional: `ip xfrm state` shows `flag esn`) |
| `esp-group g proposal 1 esn required` both ends, strongSwan peer | -> | full IKE_AUTH ESN negotiation + ESP SA install | interop `NN-ipsec-esn-strongswan` (tunnel up, traffic flows) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `esp-group/proposal` with `esn required` | `ESPProposal.ESN == ESNRequired` after parse; absent leaf -> `ESNDisabled` |
| AC-2 | `ike-group/proposal` with `esn required` | `IKEProposal.ESN == ESNRequired` after parse (stored, inert) |
| AC-3 | ESP proposal `esn disabled` (or unset) | `buildWireESPProposals` emits exactly one ESN transform with value 0 (byte-identical to current output) |
| AC-4 | ESP proposal `esn required` | `buildWireESPProposals` emits exactly one ESN transform with value 1 |
| AC-5 | ESP proposal `esn optional` | `buildWireESPProposals` emits two ESN transforms, values 0 and 1, in one proposal |
| AC-6 | Initiator receives SAr2 with ESN transform value 1 | agreed ESN stored on `SA`; `ChildSA.ESN == true`; both `SAParams.ESN == true` |
| AC-7 | Responder receives SAi2 offering [0,1], local policy `optional` | selects value 1 (prefer ESN), echoes a single ESN transform value 1, installs Child SA with ESN |
| AC-7b | Responder local policy `disabled`, peer offers only value 1 | proposal rejected (no ESN-compatible match) |
| AC-8 | `installChildSA` with `ChildSA.ESN==true` (XFRM backend) | `netlink.XfrmState.ESN == true` for inbound and outbound SA |
| AC-8b | `installChildSA` with `ChildSA.ESN==true` (VPP backend) | `ipsecSAEntry.Flags` has the `USE_ESN` bit set |
| AC-9 | Any `ike-group/proposal esn` value | `buildWireIKEProposals` output contains ZERO ESN transforms (RFC 7296) |
| AC-10 | Changing a proposal's `esn` value in config | `IPsecConfig.Changed` lists the affected peer(s) so the SA is reinstalled |
| AC-11 | ESN `required` end to end vs strongSwan with `esn=yes` | tunnel establishes and bidirectional traffic passes (interop) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `esp-group g proposal 1 esn required` and brings up a site-to-site tunnel | config -> parse -> buildWireESPProposals(id=1) -> IKE_AUTH -> installChildSA -> XfrmState.ESN | `test-ipsec-esn-required` (QEMU) |
| 2 | Peers with strongSwan configured `esn=yes` | IKE_AUTH ESN negotiation -> agreed ESN -> ESP SA on both ends -> traffic | interop `NN-ipsec-esn-strongswan` |
| 3 | Runs `show vpn ipsec sa` on an ESN tunnel | SA state surface reports ESN enabled | functional/QEMU check of show output (if SA display includes ESN; else `ip xfrm state`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseESNMode` | `internal/component/ike/ipsec/types_test.go` | enum <-> string roundtrip; unknown -> false; zero value = disabled | |
| `TestParseESPProposalESN` | `internal/component/ike/ipsec/config_test.go` | `esn` leaf parsed; absent -> ESNDisabled | |
| `TestParseIKEProposalESN` | `internal/component/ike/ipsec/config_test.go` | `esn` leaf parsed into IKEProposal | |
| `TestConfigChangedESN` | `internal/component/ike/ipsec/types_test.go` | esn change marks peer changed (AC-10) | |
| `TestBuildWireESPProposalsESN` | `internal/component/ike/engine/initiator_test.go` | disabled->[0], required->[1], optional->[0,1] (AC-3/4/5) | |
| `TestBuildWireIKEProposalsNoESN` | `internal/component/ike/engine/initiator_test.go` | IKE proposals never carry ESN (AC-9) | |
| `TestESNNegotiate` | `internal/component/ike/engine/*_test.go` | pure helper: (localMode, offeredIDs) -> chosen/ok matrix (AC-7/7b) | |
| `TestInitiatorParseAgreedESN` | `internal/component/ike/engine/fsm_test.go` | reads single ESN transform from SAr2 -> SA.ChildESN (AC-6, R-3) | |
| `TestInstallChildSAESNFlag` | `internal/component/ike/dataplane/dataplane_test.go` | fake dataplane captures `SAParams.ESN` for in+out (AC-8) | |
| `TestVPPSAEntryESNFlag` | `internal/component/ike/dataplane/vpp_test.go` | USE_ESN bit set; struct encodes at expected layout (AC-8b, R-1) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `esn` leaf | enum {required, optional, disabled} | N/A (enum, not numeric) | rejected non-enum string | rejected non-enum string |
| ESN transform value | {0, 1} | 1 | N/A | value >=2 treated as unknown/ignored on parse |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ipsec-esn-required` | `test/` QEMU integration (`mk/test-integration.mk`) | esn required -> `ip xfrm state` shows `flag esn` on both SAs | |
| `test-ipsec-esn-default-noesn` | `test/` QEMU integration | no esn config -> SAs have no ESN flag (regression for default) | |
| `ipsec-esn-config` (added 2026-07-10) | `test/parse/ipsec-esn-config.ci` | `esn` leaf accepted on both proposal lists, invalid enum rejected | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-ipsec-esn-strongswan` | `test/interop-ipsec/scenarios/` (new, mirror `test/interop-pppoe` layout) | strongSwan `esn=yes` | ESN negotiated, tunnel up, bidirectional traffic (AC-11, R-4) | |
| `NN-ipsec-esn-optional-fallback` | same | strongSwan `esn=no` | Ze `optional` falls back to No-ESN, tunnel still up (R-3) | |

### Future (if deferring any tests)
- None planned. QEMU is mandatory for the XFRM (linux-only) path per `ai/rules/platform-linux.md`.

## Files to Modify
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` -- `typedef esn-mode`; `leaf esn` on
  `esp-group/proposal` and `ike-group/proposal` (default "disabled", description notes child-SA scope)
- `internal/component/ike/ipsec/types.go` -- `ESNMode` enum (+names/`ParseESNMode`/`String`);
  `ESN ESNMode` on `ESPProposal` and `IKEProposal`; include ESN in any proposal-equality used by `Changed`
- `internal/component/ike/ipsec/config.go` -- parse `esn` in `parseESPProposal` + `parseIKEProposal`
- `internal/component/ike/engine/initiator.go` -- `buildWireESPProposals` ESN-mode emission;
  keep `buildWireIKEProposals` ESN-free; `wireProposalsToIKE` unchanged (still ignores ESN for IKE)
- `internal/component/ike/engine/fsm.go` -- initiator: parse responder's chosen ESN transform
  in the IKE_AUTH response SA handler (currently 407-412); responder: select ESN in the request handler
- `internal/component/ike/engine/auth.go` -- responder SAr2 path emits the single chosen ESN value
- `internal/component/ike/engine/sa.go` -- negotiated child-ESN field(s) on `SA`
- `internal/component/ike/engine/child.go` -- `ChildSA.ESN`; set from SA; copy to both `SAParams`
- `internal/component/ike/engine/rekey.go` -- preserve ESN across Child SA rekey
- `internal/component/ike/dataplane/dataplane.go` -- `ESN bool` on `SAParams`
- `internal/component/ike/dataplane/xfrm_linux.go` -- `state.ESN = p.ESN` (+ ESN replay handling, R-2)
- `internal/component/ike/dataplane/vpp.go` -- ~~`Flags` field on `ipsecSAEntry`; set `USE_ESN` (R-1)~~ mechanism superseded 2026-07-10: vendor `binapi/ipsec` and use the generated SA-entry type with its flags field instead of extending the hand-rolled struct (see Post-wave corrections)
- `internal/component/ike/crypto/proposal.go` -- (optional) add ESN to `crypto.ESPProposal` +
  `NegotiateESP` match, only if the responder path is implemented through `NegotiateESP`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config leaf) | [ ] yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` |
| YANG validation constraints | [ ] yes | `typedef esn-mode` enumeration (required/optional/disabled), `default "disabled"` -- native enum, no custom validator needed |
| YANG custom validators | [ ] no | enum is fully constrained natively |
| CLI commands/flags | [ ] no | config-driven; editor autocomplete is automatic for YANG enum leaves |
| Editor autocomplete | [ ] yes (automatic) | YANG enum -> completion provided by config editor |
| Functional test for new behavior | [ ] yes | QEMU integration + interop |
| Doctor check for runtime dependencies | [ ] no | no new path/socket/binary; ESN is a flag on an existing SA |
| Prometheus counters/metrics | [ ] maybe | if SA telemetry distinguishes ESN; otherwise N/A (note in implementation) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` / IPsec feature page |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, IPsec config doc |
| 7 | Wire format changed? | [ ] yes | IPsec/IKEv2 wire doc -- ESN transform now emitted |
| 9 | RFC behavior implemented? | [ ] yes | cite RFC 7296 Section 3.3.2 + RFC 4303 Section 3.3.3 in code; note in `rfc/short/rfc7296.md` if a gap |
| 10 | Test infrastructure changed? | [ ] yes (if new interop dir) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` (ESN support row) |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors on the changed files |

## Files to Create
- `test/interop-ipsec/...` -- strongSwan ESN interop scenario(s) (new dir mirroring `test/interop-pppoe`)
- QEMU integration test entry under `test/` wired into `mk/test-integration.mk`
- (optional) a short RFC 4304 summary under rfc/short/ only if the team wants the historical addendum summarized

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-precommit-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 14. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- add `esn` YANG leaves + `ESNMode` type + parse, and
   a failing QEMU wiring test `test-ipsec-esn-required`.
   - Tests: `test-ipsec-esn-required` (fails: flag not set yet)
   - Files: yang, types.go, config.go
   - Verify: config parses; wiring test fails because ESN never reaches the kernel
2. **Phase: Wire emission** -- `buildWireESPProposals` emits ESN transforms per mode; IKE stays ESN-free.
   - Tests: `TestBuildWireESPProposalsESN`, `TestBuildWireIKEProposalsNoESN`
   - Files: initiator.go
   - Verify: AC-3/4/5/9
3. **Phase: Negotiation** -- responder `esnNegotiate` + SAr2 echo; initiator parses agreed ESN from SAr2.
   - Tests: `TestESNNegotiate`, `TestInitiatorParseAgreedESN`
   - Files: fsm.go, auth.go, sa.go (+ optional crypto/proposal.go)
   - Verify: AC-6/7/7b, R-3/R-6
4. **Phase: Dataplane** -- `SAParams.ESN`; XFRM `state.ESN` (+replay, R-2); VPP `Flags`/USE_ESN (R-1).
   - Tests: `TestInstallChildSAESNFlag`, `TestVPPSAEntryESNFlag`
   - Files: dataplane.go, xfrm_linux.go, vpp.go, child.go, rekey.go
   - Verify: AC-8/8b; wiring test now passes
5. **Functional + interop** -- QEMU `ip xfrm state` checks; strongSwan interop scenarios.
   - Verify: AC-11, R-4; A-1/A-2/A-4 validated
6. **RFC refs** -- `// RFC 7296 Section 3.3.2` / `// RFC 4303 Section 3.3.3` on enforcing code
7. **Full verification + complete spec** -- `make ze-precommit-verify`; fill audit; learned summary
   `plan/learned/NNN-ipsec-12-esn.md`; two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Default path byte-identical to pre-change (AC-3); agreed flag derives from negotiated transform only (R-4) |
| RFC compliance | IKE proposals carry no ESN (AC-9); ESN transform IDs 0/1 correct |
| Naming | YANG `esn-mode` enums kebab/lower; Go `ESNMode` follows existing enum pattern |
| Data flow | ESN flows config->engine->SAParams->backend; backends read only `SAParams.ESN` |
| VPP layout | `ipsecSAEntry` field order matches VPP binary API (R-1) |
| Rule: no-workarounds | Responder selection implemented at the source, test not weakened |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `esn` leaf on both proposal lists | `grep "leaf esn" internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` (2 hits) |
| ESN emission per mode | `go test ./internal/component/ike/engine/ -run BuildWireESPProposalsESN` |
| XFRM ESN flag | QEMU: `ip xfrm state` shows `flag esn` |
| VPP USE_ESN | `go test ./internal/component/ike/dataplane/ -run VPPSAEntryESNFlag` |
| Interop passes | run `NN-ipsec-esn-strongswan` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `esn` constrained to enum by YANG; unknown transform IDs ignored, never panic |
| Negotiation integrity | agreed ESN derived from the peer's selected transform, not spoofable into a mismatch that silently drops traffic (R-4) |
| No key/material leak | ESN is a boolean flag; no new secret handling |
| Resource | no new unbounded allocation; at most two ESN transforms per proposal |

### Failure Routing
| Failure | Route To |
|---------|----------|
| VPP SA add retval != 0 after adding Flags | R-1: re-check struct layout vs govpp binding |
| QEMU `ip xfrm state` lacks esn flag | R-2: ESN replay handling in xfrm_linux.go |
| `optional` interop ambiguous | R-3: default No-ESN when response lacks exactly one ESN transform |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- ESN emission is centralized in `buildWireESPProposals` because initiator (SAi2) and responder
  (SAr2) share `buildChildSAPayloads` -- one change point covers both directions.
- The 32->64 bit anti-replay benefit only materializes if BOTH peers agree; the implementation
  therefore treats the negotiated transform, not the local config, as the source of truth for
  the dataplane flag.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Emit a single ESN transform per mode (0 / 1) and two transforms for `optional` | per-proposal-name string variants | RFC 7296 native mechanism (multiple same-type transforms) is cleaner and backend-agnostic; Ze builds transforms structurally, not as strings |
| `esn` leaf on IKE proposals is inert | Reject non-disabled IKE esn at validation; or omit the IKE leaf entirely | User chose config-symmetry across both proposal types; RFC forbids ESN in IKE SA, so the leaf is stored but never emitted, with explicit docs + AC-9 |
| Both XFRM and VPP in one spec | XFRM first, VPP follow-up | User chose full backend parity; VPP binary-layout risk tracked as R-1 with a verification gate |
| ESN intent from `Proposals[0]` | Revive `NegotiateESP` for full per-proposal ESP selection | The engine already uses `Proposals[0]`; full ESP negotiation is out of scope and a separate concern |

## Known Limitations
- `esn` on `ike-group/proposal` has no protocol effect: ESN (Transform Type 5) is valid only in
  AH/ESP proposals (RFC 7296 Section 3.3.2). The leaf is accepted and stored for configuration
  symmetry only; the IKE SA proposal never carries an ESN transform (AC-9).
- ESN intent is taken from the first ESP proposal (`Proposals[0]`); per-proposal ESP
  negotiation (`NegotiateESP`) remains unused, consistent with current engine behavior.
- IKEv1 is out of scope; ESN here is IKEv2/ESP only.

## RFC Documentation
Add above enforcing code:
- `// RFC 7296 Section 3.3.2: Transform Type 5 (ESN); value 0 = No ESN, value 1 = ESN.`
- `// RFC 7296 Section 3.3.2: ESN transform MUST NOT appear in an IKE SA proposal.`
- `// RFC 4303 Section 3.3.3: 64-bit extended sequence number; high-order 32 bits not transmitted.`

## Implementation Summary
### What Was Implemented
- (to fill during /implement)
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ESP Child SA uses 64-bit ESN when configured | QEMU functional | `ip xfrm state` shows `flag esn` |
| Interoperates with a standard IKEv2 peer | interop test | strongSwan `esn=yes` tunnel up + traffic |
| Both dataplanes support ESN | unit + interop | XFRM `state.ESN`; VPP USE_ESN flag |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | file:line | |
### Fixes applied
### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for enum/transform values
- [ ] Functional tests (QEMU) for end-to-end behavior
- [ ] Interop tests (strongSwan)
- [ ] Goal Validation table filled

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-12-esn.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-ipsec-12-esn.md`

### Post-wave corrections (2026-07-10)

NEW CONSTRAINT for the VPP dataplane phase (AC-8b): implement the ESN flag change via the vendored govpp binapi, not by expanding hand-rolled structs.

- The followup-vpp-iface wave established the govpp binapi vendoring pattern under `vendor/go.fd.io/govpp/binapi/` (28 packages vendored: gre, ipip, vxlan, span, lcp, wireguard, sr, and others). `binapi/ipsec` is NOT yet vendored (verified 2026-07-10: absent from the vendor tree); vendoring it is now a mechanical step following the same pattern.
- The hand-rolled types this spec planned to extend carry a comment anticipating exactly this migration, verified at `internal/component/ike/dataplane/vpp.go`: when govpp/binapi/ipsec is vendored, replace these with the generated types. The `ipsecSAEntry` struct that follows (:163 onward) has no Flags field, as the Current Behavior section records.
- Consequences: the Dataplane phase should vendor `binapi/ipsec` and set the USE_ESN flag on the GENERATED SA-entry type. This retires the hand-layout hazard tracked as R-1 and assumption A-2 (the generated types carry the exact wire layout and CRC, so no manual field-order verification is needed); the corresponding row in Failure Routing ("re-check struct layout vs govpp binding") becomes "regenerate/re-vendor binapi/ipsec". The `TestVPPSAEntryESNFlag` unit test then asserts the flag on the generated type rather than a hand-encoded layout.
- The Files to Modify bullet for vpp.go is struck above accordingly; the file is still modified, only the mechanism changes.
