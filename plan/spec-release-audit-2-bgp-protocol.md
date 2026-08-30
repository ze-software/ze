# Spec: release-audit-2-bgp-protocol

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-release-audit-1-surface-inventory.md |
| Phase | - |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-release-audit-0-umbrella.md` - release audit blocker policy and finding schema
3. `plan/spec-release-audit-1-surface-inventory.md` - release surface inventory and evidence matrix
4. `ai/rules/protocol.md` and `ai/rules/rfc-compliance.md` - protocol claim rules
5. `ai/rules/interop-and-goal-validation.md` - protocol interop evidence rule
6. `docs/architecture/wire/messages.md` - BGP message wire format
7. `docs/architecture/wire/capabilities.md` - capability parsing and negotiation model
8. `internal/component/bgp/message/open.go` - OPEN and RFC 9072 current behavior
9. `internal/core/bgp/capability/capability.go` - capability and optional-parameter parsing current behavior
10. `internal/component/bgp/message/rfc7606.go` - UPDATE error handling current behavior
11. `internal/component/bgp/reactor/session_read.go` and `session_handlers.go` - session validation and callback ordering

## Task

Audit Ze's BGP protocol surface before first user-facing release. The audit covers OPEN parsing, capability negotiation, UPDATE validation, route-refresh behavior, ADD-PATH, AS4 handling, RFC 7606 error handling, plugin/event delivery boundaries, ExaBGP compatibility, and live interop evidence.

This spec documents findings only. It does not fix product code, tests, schemas, generated files, Makefiles, or documentation. Future fix work must be approved separately and must include the verification requested by each finding.

## Audit Scope Boundary

This child audit records BGP protocol issues and evidence. It may run read-only or verification commands to prove findings, but it does not edit product code or tests.

Every BGP finding must include:
- The observed protocol, interop, or evidence issue.
- The user, peer, operator, or release impact.
- Source, RFC, test, or interop evidence proving the issue exists.
- The future owner area.
- Suggested fix direction, without implementing it here.
- Verification required from the future fix.

## Required Reading

### Architecture Docs and Rules

- [ ] `plan/spec-release-audit-0-umbrella.md` - release audit method and finding schema
  -> Decision: use the umbrella finding schema and route fixes to separate future work
  -> Constraint: this spec documents issues only and does not change product code
- [ ] `plan/spec-release-audit-1-surface-inventory.md` - BGP rows in release surface matrix
  -> Decision: consume the BGP protocol, NLRI, command, interop, and ExaBGP rows from the inventory
  -> Constraint: protocol findings must map to an entry point and evidence category
- [ ] `ai/rules/protocol.md` - RFC summaries before protocol claims
  -> Constraint: protocol findings cite repo RFC summaries, not memory
- [ ] `ai/rules/rfc-compliance.md` - BGP must be RFC 4271 compliant
  -> Constraint: future fixes must add RFC comments where code enforces MUST behavior
- [ ] `ai/rules/interop-and-goal-validation.md` - protocol features need external peer evidence
  -> Constraint: protocol behavior closes only with interop or justified packet-level evidence
- [ ] `ai/rules/testing.md` - user-visible wire behavior needs functional coverage
  -> Constraint: unit tests alone do not close reachable BGP behavior
- [ ] `docs/architecture/wire/messages.md` - BGP message formats and length rules
  -> Decision: audit message-specific parsing, length validation, and callback ordering
- [ ] `docs/architecture/wire/capabilities.md` - capability TLVs and negotiation
  -> Decision: split OPEN optional-parameter parsing from capability TLV parsing
- [ ] `docs/architecture/edge-cases/addpath.md` - ADD-PATH negotiation and path identifier handling
  -> Constraint: ADD-PATH is directional and negotiated per AFI/SAFI
- [ ] `docs/architecture/edge-cases/as4.md` - ASN4 and AS_TRANS behavior
  -> Constraint: AS4 findings must preserve RFC 6793 translation and discard rules
- [ ] `docs/architecture/testing/interop.md` - live interop scenarios and ExaBGP compatibility
  -> Decision: compare documented interop inventory with actual `test/interop/scenarios/`
  -> Constraint: docs can be stale and cannot be the only evidence source

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` - base BGP message, OPEN, UPDATE, NOTIFICATION, and FSM rules
  -> Constraint: malformed header, OPEN, and UPDATE errors must map to the correct NOTIFICATION behavior
- [ ] `rfc/short/rfc5492.md` - capability advertisement and unknown capability handling
  -> Constraint: unknown capabilities are ignored, malformed capability TLV boundaries are not silently treated as negotiated features
- [ ] `rfc/short/rfc9072.md` - extended optional parameters in OPEN
  -> Constraint: extended OPEN detection is based on Non-Ext OP Type 255, and extended optional parameters use two-octet parameter length fields
- [ ] `rfc/short/rfc4760.md` - MP_REACH_NLRI, MP_UNREACH_NLRI, and MP capability
  -> Constraint: MP attributes need AFI/SAFI and NLRI syntax validation and negotiated-family enforcement
- [ ] `rfc/short/rfc7606.md` - revised UPDATE error handling
  -> Constraint: use the strongest action, handle duplicate attributes, and validate NLRI before treat-as-withdraw
- [ ] `rfc/short/rfc2918.md` - route-refresh base capability and message
  -> Constraint: ROUTE-REFRESH body is fixed length and unadvertised families are ignored
- [ ] `rfc/short/rfc7313.md` - enhanced route-refresh BoRR/EoRR markers
  -> Constraint: invalid BoRR/EoRR length requires Route-Refresh NOTIFICATION; unknown subtypes are ignored
- [ ] `rfc/short/rfc7911.md` - ADD-PATH capability and path identifier encoding
  -> Constraint: Send/Receive values are only 1, 2, or 3; other values should make the capability not understood and ignored
- [ ] `rfc/short/rfc6793.md` - four-octet AS numbers and AS4_PATH handling
  -> Constraint: AS4_PATH/AS4_AGGREGATOR discard behavior must not reset sessions
- [ ] `rfc/short/rfc8654.md` - extended BGP message length
  -> Constraint: extended messages are accepted only if the local speaker advertised the capability

**Key insights:**
- The audit must distinguish unknown capabilities, which RFC 5492 says to ignore, from malformed known capability encodings that can silently downgrade or mis-negotiate a session.
- RFC 9072 affects both OPEN message framing and optional-parameter length fields. Ze currently has code for the marker but the capability parsing path still assumes standard optional parameter lengths.
- RFC 7606 validation exists and has broad unit coverage, but some validations are pure helper tests and need session-level evidence to prove bad UPDATEs do not reach RIB/plugin consumers incorrectly.
- Live interop scenarios cover many happy paths but not malformed OPEN/capability and negative RFC 7606 paths.
- Documentation drift in interop inventory is a release issue because release engineers use the docs to decide what evidence exists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/open.go` - writes extended OPEN marker when `OptionalParams` length exceeds 255 and detects extended format only when `data[9] == 255` and `data[10] == 255`
- [ ] `internal/component/bgp/message/open_test.go` - tests RFC 9072 raw OPEN body round-trip and truncation, but not capability negotiation through extended optional parameters
- [ ] `internal/core/bgp/capability/capability.go` - parses capability TLVs, extracts capabilities from OPEN optional parameters, and silently stops on malformed optional parameter boundaries
- [ ] `internal/core/bgp/capability/capability_test.go` - tests normal, empty, truncated, unknown, and valid ADD-PATH capabilities, but not overlong known capabilities or invalid ADD-PATH modes
- [ ] `internal/core/bgp/capability/negotiated.go` - tracks peer capability codes and performs directional ADD-PATH negotiation for modes 1, 2, and 3
- [ ] `internal/component/bgp/message/header.go` - validates ROUTE-REFRESH minimum length but exact body length is enforced later in the session handler
- [ ] `internal/component/bgp/message/routerefresh.go` - parses ROUTE-REFRESH body after length check, including subtype field
- [ ] `internal/component/bgp/reactor/session_read.go` - validates UPDATE with RFC 7606 before callback dispatch, then calls message callback for all message types before type-specific handlers
- [ ] `internal/component/bgp/reactor/session_handlers.go` - handles OPEN, UPDATE, NOTIFICATION, and ROUTE-REFRESH; route-refresh exact length and subtype checks happen after callback dispatch
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - outbound BoRR/EoRR send path checks Enhanced Route Refresh capability before sending markers
- [ ] `internal/component/bgp/reactor/session_validation.go` - enforces RFC 7606 and rejects non-negotiated MP families with NOTIFICATION in `processMessage()`
- [ ] `internal/component/bgp/message/rfc7606.go` - validates UPDATE attributes, duplicate MP attributes, next-hop lengths, and NLRI helper syntax
- [ ] `internal/component/bgp/message/rfc7606_test.go` - contains many RFC 7606 unit tests, including duplicate ORIGIN/MED, MP_REACH next-hop length, classic NLRI syntax, and MP_UNREACH minimum length
- [ ] `internal/core/bgp/attribute/wire.go` - lazy attribute index rejects duplicate attributes when consumers parse attributes later
- [ ] `internal/component/bgp/reactor/session_handlers_test.go` - covers route-refresh invalid length, unknown subtype, no capability, non-negotiated family, and update family mismatch paths
- [ ] `internal/component/bgp/reactor/session_test.go` - covers normal route refresh, BoRR, EoRR, unknown subtype, reserved subtype, and bad route-refresh body length with an established test session
- [ ] `test/interop/scenarios/*/check.py` - actual interop scenario tree currently includes scenarios through `bgp-remove-private-as-as4path-frr`
- [ ] `test/interop/scenarios/bgp-addpath-frr/check.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - checks ADD-PATH capability negotiation and multiple path receipt with FRR
- [ ] `test/interop/scenarios/bgp-route-refresh-frr/check.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - checks normal route refresh via FRR soft-in clear and route stability
- [ ] `docs/architecture/testing/interop.md` - lists scenarios through 32 and says BFD is not covered
- [ ] `docs/features/interoperability-testing.md` - says there are 32 interop scenarios and lists only 01 through 32

**Behavior to preserve:**
- Unknown capability codes remain non-fatal per RFC 5492.
- Normal route-refresh for negotiated families remains non-fatal.
- Unknown route-refresh subtypes remain ignored per RFC 7313.
- RFC 7606 treat-as-withdraw and attribute-discard paths continue to reduce session resets when safe.
- ADD-PATH negotiation remains directional per AFI/SAFI.
- AS4 translation and attribute-discard behavior remain RFC 6793 compatible.
- Existing ExaBGP compatibility and live interop suites remain separate evidence sources.

**Audit documentation goal:**
- Record BGP protocol bugs, evidence gaps, stale docs, and future verification needs before release.
- Keep code and tests unchanged in this audit phase.
- Make each future protocol fix prove its behavior with unit, functional, and interop or packet-level evidence.

## Data Flow (MANDATORY)

### Entry Point

- BGP wire bytes enter through the TCP session read loop in `internal/component/bgp/reactor/session_read.go`.
- Local configuration enters OPEN construction through `session_negotiate.go` and capability objects.
- Plugin/API consumers receive BGP messages through `onMessageReceived` callbacks and the plugin/RPC event path.
- Release evidence enters through Go unit tests, functional BGP tests, ExaBGP compatibility tests, and Docker interop scenarios.

### Transformation Path

1. Session reads and validates the BGP header in `session_read.go`.
2. OPEN bodies are unpacked by `message.UnpackOpen()`.
3. OPEN optional parameters are converted to capabilities by `capability.ParseFromOptionalParams()`.
4. Capabilities are negotiated by `capability.Negotiate()` and copied into session encoding context.
5. UPDATE bodies are wrapped as `WireUpdate`, validated by `enforceRFC7606()`, then delivered or rejected.
6. Route-refresh messages pass header minimum length, then callback delivery, then exact body length and subtype checks.
7. Valid messages reach FSM handling and plugin/API event delivery.
8. Tests and interop scenarios observe negotiation, route exchange, route refresh, malformed input behavior, and session stability.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> message parser | Header and type-specific body parsers | `message/header.go`, `message/open.go`, `message/routerefresh.go` |
| OPEN optional params -> capability negotiation | `ParseFromOptionalParams()` then `Negotiate()` | `capability.go`, `negotiated.go` |
| Session validation -> plugin event stream | `processMessage()` callback ordering | `session_read.go` |
| UPDATE validation -> RIB/plugin consumers | `enforceRFC7606()` before callback for UPDATE only | `session_validation.go`, `session_read.go` |
| Route refresh -> API/plugin route-refresh behavior | `onMessageReceived` callback plus `handleRouteRefresh()` | `session_read.go`, `session_handlers.go` |
| Protocol code -> release evidence | Go tests, ExaBGP compat, live interop | `test/`, `test/interop/scenarios/` |

### Integration Points

- `spec-release-audit-5-plugins-rib.md` owns future fixes that involve RIB/plugin consumer handling of duplicate or discarded attributes.
- `spec-release-audit-8-docs-onboarding.md` owns future documentation updates for stale interop inventories.
- `spec-release-evidence-gate.md` owns future release target wiring for heavy interop evidence.

### Architectural Verification

- [ ] No bypassed layers: findings name the BGP wire entry point and runtime path.
- [ ] No unintended coupling: suggested fixes keep parser, negotiation, session, and plugin/RIB concerns separate.
- [ ] No duplicated functionality: future fixes should centralize optional-parameter parsing rather than adding ad hoc session checks.
- [ ] Zero-copy preserved where applicable: future UPDATE fixes must preserve `WireUpdate` and buffer ownership semantics.

## Protocol Audit Matrix

| Surface | Current Evidence | Release Risk | Finding |
|---------|------------------|--------------|---------|
| OPEN optional parameters and RFC 9072 | `open.go`, `open_test.go`, RFC 9072 summary | Extended OPEN markers exist but parameter length handling is inconsistent | RA-BGP-001 |
| OPEN malformed optional parameter handling | `capability.ParseFromOptionalParams()`, `handleOpen()` | Malformed params can be silently ignored and downgrade session capability state | RA-BGP-002 |
| Known capability length validation | `capability.go`, `capability_test.go`, RFC summaries | Overlong and non-zero-length known capabilities lack negative coverage and enforcement | RA-BGP-003 |
| ADD-PATH invalid modes | `parseAddPath()`, `negotiated.go`, RFC 7911 summary | Invalid mode is parsed as present despite comment saying it should be ignored | RA-BGP-004 |
| Route refresh and Enhanced RR | `session_read.go`, `session_handlers.go`, `session_test.go`, interop scenario 12 | Event callback and validation ordering can expose malformed or unsupported marker messages | RA-BGP-005 |
| RFC 7606 duplicate attributes | `rfc7606.go`, `rfc7606_test.go`, `attribute/wire.go` | Validator reports no error for duplicates but later consumers may reject duplicate attributes | RA-BGP-006 |
| MP_REACH/MP_UNREACH syntax | `rfc7606.go`, `rfc7606_test.go`, `session_validation.go` | MP next-hop overrun and MP NLRI syntax are not fully enforced in the UPDATE validator | RA-BGP-007 |
| Live interop inventory | `test/interop/scenarios/*/check.py`, interop docs | Scenario docs stale and negative malformed-wire coverage absent | RA-BGP-008 |

## Initial Findings

| ID | Severity | Surface | File/line | User Impact | Reproduction | Expected | Actual | Missing Test | Suggested Direction | Owner | Verification Requested |
|----|----------|---------|-----------|-------------|--------------|----------|--------|--------------|---------------------|-------|------------------------|
| RA-BGP-001 | Critical | OPEN/RFC 9072 | `internal/component/bgp/message/open.go`, `:100-135`, `:185-217`; `internal/component/bgp/reactor/session_negotiate.go`; `internal/core/bgp/capability/capability.go`; `rfc/short/rfc9072.md`, `:229-230` | Peers using valid extended OPEN optional parameters may fail capability negotiation, and Ze may emit malformed extended optional parameters when total optional parameters exceed 255 bytes | Inspect RFC 9072 summary against `Open.WriteTo()`, `UnpackOpen()`, `buildOptionalParams()`, and `ParseFromOptionalParams()` | Extended OPEN detection accepts any non-zero Non-Ext OP Len when Non-Ext OP Type is 255, and extended optional parameters use two-octet parameter lengths | `UnpackOpen()` only enters extended mode when `data[9] == 255`; `buildOptionalParams()` and `ParseFromOptionalParams()` use one-octet parameter lengths even when `Open.WriteTo()` emits the extended marker | No test proves capability negotiation through RFC 9072 extended optional parameters, no encode test asserts two-octet optional-parameter length fields, no test covers Non-Ext OP Len not equal to 255 with type 255 | Future fix should normalize optional parameters after unpacking or represent parameter length width explicitly, encode extended parameters with two-octet lengths, and accept RFC 9072's non-zero Non-Ext OP Len rule | BGP message/capability | Unit tests for extended OPEN encode/decode, session negotiation test with valid extended capability parameter, packet-level test that capabilities survive extended OPEN |
| RA-BGP-002 | Major | OPEN malformed optional params | `internal/core/bgp/capability/capability.go`; `internal/component/bgp/reactor/session_handlers.go`; `rfc/short/rfc5492.md`; `rfc/short/rfc4271.md`, `:615` | A peer can send malformed optional parameters and Ze may silently drop them, causing capability downgrade or acceptance of a session that should have failed clearly | Inspect `ParseFromOptionalParams()` and `handleOpen()` | Unknown capabilities are ignored, but malformed optional parameter boundaries and malformed known capability TLVs should produce explicit OPEN error behavior or a documented exact ignore policy | `ParseFromOptionalParams()` breaks on short parameter headers or length overrun and ignores `Parse()` errors; `handleOpen()` cannot distinguish no capabilities from malformed parameters | No session-level malformed optional parameter tests assert NOTIFICATION or documented ignore behavior | Future fix should make optional-parameter extraction return structured errors while preserving RFC 5492 unknown-capability ignore semantics | BGP capability/session | Unit tests for short parameter header, parameter length overrun, malformed capability TLV, and session-level NOTIFICATION behavior |
| RA-BGP-003 | Major | capability length validation | `internal/core/bgp/capability/capability.go`, `:272-316`, `:319-373`; `rfc/short/rfc2918.md`; `rfc/short/rfc7313.md`; `rfc/short/rfc8654.md`; `rfc/short/rfc4760.md`; `rfc/short/rfc6793.md` | Malformed known capabilities can be accepted as valid, causing wrong negotiation state or interop failures with strict peers | Inspect parsers and RFC summary length fields | Known capabilities with fixed lengths are validated against their exact RFC-defined lengths or ignored as not understood according to the capability's RFC | Multiprotocol and ASN4 accept `len(data) >= 4`; Route Refresh, Extended Message, and Enhanced Route Refresh ignore non-zero payloads | No tests for overlong MP/ASN4 or non-zero payload Route Refresh, Enhanced Route Refresh, or Extended Message | Future fix should define one policy for malformed known capabilities, implement exact length checks, and preserve unknown capability tolerance | BGP capability | Unit tests for exact length, overlong length, non-zero zero-length capabilities, and session-level behavior for malformed known capabilities |
| RA-BGP-004 | Major | ADD-PATH capability | `internal/core/bgp/capability/capability.go`; `internal/core/bgp/capability/negotiated.go`, `:250-291`; `rfc/short/rfc7911.md`, `:128-140` | Invalid ADD-PATH modes can be recorded as a peer-advertised capability and influence capability presence checks, even though RFC 7911 says invalid values should make the capability not understood and ignored | Send ADD-PATH capability data with mode 0 or 4 into `Parse()` | Invalid Send/Receive values are ignored as an unrecognized capability and do not affect negotiation or refused-capability logic | `parseAddPath()` stores any byte as `Mode`; negotiation ignores unknown modes for send/receive, but `peerCodes` still records ADD-PATH as present | No invalid ADD-PATH mode tests | Future fix should reject or ignore the entire ADD-PATH capability instance when any tuple has a mode outside 1, 2, or 3, then document how multiple ADD-PATH capability instances are handled | BGP capability/ADD-PATH | Unit tests for mode 0 and 4, negotiation tests showing no ADD-PATH state or refused-cap side effect, interop or packet-level negative test |
| RA-BGP-005 | Major | route-refresh and Enhanced RR | `internal/component/bgp/reactor/session_read.go`, `:218-237`; `internal/component/bgp/reactor/session_handlers.go`; `internal/component/bgp/reactor/reactor_api_forward.go`; `internal/component/bgp/reactor/session_test.go`; `rfc/short/rfc7313.md`, `:221-235` | API/plugin consumers may see malformed or unsupported route-refresh marker messages before validation, and Enhanced RR inbound behavior lacks capability and state evidence | Send ROUTE-REFRESH with body length greater than 4 or BoRR/EoRR without negotiated Enhanced Route Refresh in a session with an `onMessageReceived` callback | Invalid length is rejected before consumers treat the message as usable; BoRR/EoRR handling is gated or clearly documented when Enhanced RR was not negotiated | Header validation allows ROUTE-REFRESH length >= 23, callback runs before `handleRouteRefresh()` exact length and subtype checks, and inbound BoRR/EoRR is not checked against `neg.EnhancedRouteRefresh` in the handler | Tests cover handler/session no-error paths and outbound Enhanced RR gating, but not callback delivery for invalid length or BoRR/EoRR without Enhanced RR | Future fix should decide whether raw monitor callbacks intentionally receive invalid messages; if yes, mark invalid/unsupported state in events. If not, move exact route-refresh validation before callback and add Enhanced RR state handling | BGP reactor/route-refresh | Session tests with callback assertions for invalid length, BoRR/EoRR without Enhanced RR, Enhanced RR negotiated, and normal refresh interop retained |
| RA-BGP-006 | Major | RFC 7606 duplicate attributes | `internal/component/bgp/message/rfc7606.go`; `internal/component/bgp/message/rfc7606_test.go`; `internal/core/bgp/attribute/wire.go`; `internal/component/bgp/reactor/session_read.go`; `rfc/short/rfc7606.md`, `:141-148` | UPDATEs with duplicate non-MP attributes may pass RFC 7606 validation but fail later when RIB or plugin consumers build an attribute index | Send UPDATE with duplicate ORIGIN or MED through session into a consumer that calls `AttributesWire.ensureIndexLocked()` | RFC 7606 duplicate non-MP attributes are discarded after the first occurrence before route selection or consumer parsing | Validator skips duplicate non-MP attributes and returns no error, but raw duplicate bytes remain; lazy attribute index treats any duplicate as an error | Existing tests only assert `ValidateUpdateRFC7606()` returns none for duplicate ORIGIN/MED; no session/RIB consumer test proves first-wins behavior after dispatch | Future fix should either physically remove or mark duplicate non-MP attributes before dispatch, or make all consumers enforce the same first-wins rule | BGP message plus plugin/RIB | Session-level duplicate attribute test proving RIB/plugin receives usable first-wins attributes, plus unit tests for duplicate MP still causing session reset |
| RA-BGP-007 | Major | MP_REACH/MP_UNREACH RFC 7606 syntax | `internal/component/bgp/message/rfc7606.go`, `:688-721`, `:786-841`; `internal/component/bgp/message/rfc7606_test.go`, `:1045-1083`; `internal/component/bgp/reactor/session_validation.go`; `rfc/short/rfc7606.md`; `rfc/short/rfc4760.md` | Malformed MP attributes can pass validation when next-hop length is valid for the AFI/SAFI but exceeds the actual attribute body, or when MP NLRI syntax is malformed | Inspect `validateMPReachNextHop()` and `validateMPUnreachAttr()` against RFC 7606 and RFC 4760 summary rules | MP_REACH validates that next-hop length fits the attribute body and MP_REACH/MP_UNREACH validate their NLRI syntax, with correct session-reset or AFI/SAFI handling when the field cannot be parsed | `validateMPReachNextHop()` checks allowed length values but not `len(data) >= 5 + nhLen`; `ValidateNLRISyntax()` is used for traditional NLRI/withdrawn routes in session validation but not for MP attribute NLRI bytes | Tests cover next-hop invalid lengths, classic NLRI syntax, and MP_UNREACH minimum length, but not MP next-hop overrun or malformed MP NLRI carried inside MP_REACH/MP_UNREACH | Future fix should validate MP_REACH next-hop bounds, reserved-byte availability, and MP NLRI syntax using family-specific max prefix lengths or registered NLRI validators | BGP message/RFC7606 | Unit tests for valid nhLen with truncated body, MP_REACH malformed NLRI, MP_UNREACH malformed NLRI, and session-level NOTIFICATION or treat-as-withdraw behavior |
| RA-BGP-008 | Minor | interop evidence/docs | `test/interop/scenarios/*/check.py`; `docs/architecture/testing/interop.md`, `:240-244`; `docs/features/interoperability-testing.md`, `:17-52` | Release engineers and users may underestimate or misread protocol evidence; malformed-wire coverage is absent from live interop inventory | Compare actual scenario tree with docs | Interop docs match current scenario inventory and identify both positive and negative protocol evidence | Actual tree includes scenarios through `bgp-remove-private-as-as4path-frr`; docs list 32 scenarios and one doc still says BFD is not covered while `bfd-frr` exists | No inventory check prevents docs drift; no live interop scenario covers malformed OPEN/capability/RFC 7606 negative paths | Future fix should derive or check interop inventory from the scenario tree and decide which negative protocol tests belong in interop versus packet-level functional tests | Docs/onboarding plus release evidence | Documentation update backed by inventory check, plus negative malformed-wire scenario plan or explicit non-interop justification |

## Wiring Test (MANDATORY)

This audit spec has no runtime code. Its wiring test is that each protocol surface maps to the source path and future evidence path.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Remote OPEN with optional parameters | -> | `message.UnpackOpen()`, `capability.ParseFromOptionalParams()`, `capability.Negotiate()` | Future tests requested by RA-BGP-001 and RA-BGP-002 |
| Remote OPEN with known capabilities | -> | capability parsers and negotiation | Future tests requested by RA-BGP-003 and RA-BGP-004 |
| Remote ROUTE-REFRESH | -> | `processMessage()`, `onMessageReceived`, `handleRouteRefresh()` | Existing route-refresh tests plus future tests requested by RA-BGP-005 |
| Remote UPDATE | -> | `enforceRFC7606()`, `validateUpdateFamilies()`, plugin/RIB consumers | Existing RFC 7606 tests plus future tests requested by RA-BGP-006 and RA-BGP-007 |
| Live peer interop | -> | `test/interop/scenarios/` | Existing scenarios 01 through 37 plus future inventory check requested by RA-BGP-008 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP protocol audit starts | OPEN, capability, UPDATE, route-refresh, ADD-PATH, AS4, interop, and docs evidence surfaces are mapped |
| AC-2 | Confirm a protocol finding | Finding includes RFC/source/test evidence, user or peer impact, suggested future direction, and requested verification |
| AC-3 | Review OPEN/capabilities | Audit identifies malformed OPEN, RFC 9072, known capability, unknown capability, and ADD-PATH evidence gaps |
| AC-4 | Review UPDATE validation | Audit identifies RFC 7606 behavior, duplicate attribute handling, MP NLRI syntax, and consumer boundary risks |
| AC-5 | Review route-refresh | Audit identifies normal route refresh, Enhanced RR marker, capability gating, validation ordering, and interop coverage |
| AC-6 | Review interop evidence | Audit compares scenario tree with docs and records positive coverage plus negative coverage gaps |
| AC-7 | Keep audit-only scope | No production source, tests, schemas, docs, generated files, or Makefiles are modified by this spec |

## 🧪 TDD Test Plan

This protocol audit records evidence expected from future fix work. It does not add or change tests itself.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| RFC 9072 extended OPEN encode/decode | `internal/component/bgp/message/open_test.go` | Extended optional-parameter detection and two-octet parameter lengths | Suggested for RA-BGP-001 |
| Extended OPEN capability negotiation | `internal/component/bgp/reactor/*_test.go` or `capability/*_test.go` | Capabilities survive RFC 9072 OPEN through session negotiation | Suggested for RA-BGP-001 |
| Malformed optional parameter extraction | `internal/core/bgp/capability/*_test.go` | Short header, length overrun, malformed capability TLV | Suggested for RA-BGP-002 |
| Known capability exact length | `internal/core/bgp/capability/capability_test.go` | MP, ASN4, Route Refresh, Enhanced RR, Extended Message length policy | Suggested for RA-BGP-003 |
| Invalid ADD-PATH mode | `internal/core/bgp/capability/capability_test.go` | Mode 0 and 4 ignored or rejected per chosen policy | Suggested for RA-BGP-004 |
| Route-refresh callback ordering | `internal/component/bgp/reactor/session_test.go` | Invalid route-refresh body and unsupported BoRR/EoRR event delivery behavior | Suggested for RA-BGP-005 |
| Duplicate non-MP consumer behavior | `internal/component/bgp/reactor` or RIB tests | Duplicate attributes produce first-wins usable attributes downstream | Suggested for RA-BGP-006 |
| MP NLRI bounds and syntax | `internal/component/bgp/message/rfc7606_test.go` | MP_REACH next-hop overrun, MP_REACH NLRI syntax, MP_UNREACH NLRI syntax | Suggested for RA-BGP-007 |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OPEN Non-Ext OP Len | non-zero when type 255 | 1 and 255 | 0 with no params | N/A |
| Optional parameter length width | 0-65535 in RFC 9072 | 65535 | truncated body | body shorter than declared length |
| Multiprotocol capability length | exactly 4 | 4 | 3 | 5 |
| ASN4 capability length | exactly 4 | 4 | 3 | 5 |
| zero-length capabilities | exactly 0 | 0 | N/A | 1 |
| ADD-PATH mode | 1-3 | 3 | 0 | 4 |
| ROUTE-REFRESH body length | exactly 4 | 4 | 3 | 5 |
| MP_REACH next-hop bounds | `len(data) >= 5 + nhLen` | exact fit | missing reserved byte | declared next-hop longer than body |

### Functional Tests

This audit adds no tests itself (audit-only scope): the rows below are evidence suggested for future fix specs, and existing test suites plus the interop scenarios remain the current evidence.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Packet-level malformed OPEN | `test/decode` or BGP session functional suite | Peer sends malformed OPEN and observes NOTIFICATION/session behavior | Suggested for RA-BGP-001 to RA-BGP-003 |
| Packet-level invalid ADD-PATH | `test/decode` or BGP session functional suite | Peer sends invalid ADD-PATH mode and Ze ignores or rejects as specified | Suggested for RA-BGP-004 |
| Packet-level malformed route-refresh | `test/decode` or BGP session functional suite | Peer sends invalid body length and consumers do not treat it as valid | Suggested for RA-BGP-005 |
| Packet-level malformed MP UPDATE | `test/decode` or BGP session functional suite | Peer sends malformed MP_REACH/MP_UNREACH and Ze applies RFC 7606 action | Suggested for RA-BGP-007 |

### Interop Tests

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-addpath-frr` | `test/interop/scenarios/bgp-addpath-frr` | FRR | ADD-PATH happy-path negotiation and multiple paths | Existing |
| `bgp-route-refresh-frr` | `test/interop/scenarios/bgp-route-refresh-frr` | FRR | Normal route refresh happy path and session stability | Existing |
| `bgp-remove-private-as-as4path-frr` | `test/interop/scenarios/bgp-remove-private-as-as4path-frr` | FRR plus BIRD | AS4_PATH handling with remove-private-as policy | Existing |
| Malformed OPEN/capability negative path | TBD | ze-peer, ExaBGP harness, FRR/BIRD/GoBGP if possible | Session behavior for malformed wire input | Suggested, choose packet-level or interop during future fix |
| Enhanced route-refresh marker behavior | TBD | FRR if exposed, otherwise packet-level peer | BoRR/EoRR capability and state behavior | Suggested for RA-BGP-005 |

### Future

- No deferrals are approved by this audit. Future fix specs must choose the concrete test type for each finding and prove closure before claiming completion.

## Files to Modify

- `plan/spec-release-audit-2-bgp-protocol.md` - this audit spec
- No production files in this audit phase
- Future fix directions are recorded in Initial Findings; product edits happen outside this audit spec

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A for audit spec |
| CLI commands/flags | [ ] | N/A for audit spec |
| CLI grammar | [ ] | N/A for audit spec |
| Editor autocomplete | [ ] | N/A for audit spec |
| Functional test for new behavior | [ ] | Document expected evidence only |
| Doctor check for runtime dependencies | [ ] | N/A for audit spec |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `spec-release-audit-8-docs-onboarding.md` owns doc fixes |
| 7 | Wire format changed? | [ ] | Future BGP fix specs decide |
| 8 | Plugin SDK/protocol changed? | [ ] | Future BGP/plugin fix specs decide |
| 9 | RFC behavior implemented? | [ ] | Future BGP fix specs decide |
| 10 | Test infrastructure changed? | [ ] | Future release evidence work decides |
| 11 | Affects daemon comparison? | [ ] | Future docs/onboarding work decides |
| 12 | Internal architecture changed? | [ ] | Future BGP fix specs decide |

## Files to Create

- None beyond this spec.

## Implementation Steps

Despite the template heading, these are audit documentation steps only. They do not authorize product code changes.

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Protocol Audit Matrix and Initial Findings |
| 3. Wiring phase | Data Flow and Wiring Test |
| 4. Document findings | No production implementation in this spec |
| 5. Review gate | Verify source/RFC/test evidence for each finding |
| 6. Full verification | `./le spec status`, `git diff --check`, and targeted source checks |
| 7. Critical review | Critical Review Checklist below |
| 8. Route issues | Record suggested direction and owner for future fix work |
| 9. Re-verify audit evidence | Re-check source references and reproductions, not product fixes |
| 10. Repeat | Until BGP audit findings are source-backed |
| 11. Deliverables review | Finding list and requested verification complete |
| 12. Security review | Malformed input and resource exhaustion concerns routed |
| 13. Re-verify | Final spec visibility and formatting checks |
| 14. Present summary | BGP protocol audit report |

### Implementation Phases

1. **Phase: OPEN and capability audit** - verify RFC 4271, RFC 5492, RFC 9072, fixed capability lengths, and ADD-PATH parsing against source and tests.
2. **Phase: UPDATE and RFC 7606 audit** - verify malformed UPDATE handling, duplicate attributes, MP attributes, route family validation, and consumer boundaries.
3. **Phase: Route-refresh audit** - verify normal route refresh, Enhanced RR markers, callback ordering, and interop evidence.
4. **Phase: Interop evidence audit** - compare actual scenario tree with docs and identify negative protocol coverage gaps.
5. **Phase: Finding triage** - classify findings, route owners, and document verification requested from future fixes.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | OPEN, capabilities, UPDATE, route-refresh, ADD-PATH, AS4, interop, and docs are represented |
| Correctness | Every finding has source/RFC/test evidence and avoids unsupported protocol claims |
| Reproducibility | Every finding names a command, source comparison, or packet/test scenario for future reproduction |
| Scope | Spec remains audit-only and does not implement product fixes |
| Interop | Wire-visible findings request external peer or packet-level evidence |
| Severity | Severity reflects release and peer impact, not implementation difficulty |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Protocol audit spec exists | `plan/spec-release-audit-2-bgp-protocol.md` |
| Protocol audit matrix exists | Protocol Audit Matrix section |
| Initial findings exist | Initial Findings section |
| RFC summaries cited | Required Reading and findings reference `rfc/short/` files |
| Interop drift captured | RA-BGP-008 |
| Spec visible to status tool | `./le spec status` shows `release-audit-2-bgp-protocol` |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Malformed OPEN | Silent capability downgrade, unexpected session acceptance, missing NOTIFICATION |
| Malformed UPDATE | Bad route entering RIB/plugin path, incorrect session reset, missing treat-as-withdraw |
| Resource exhaustion | Excessive optional parameters, large messages, repeated route-refresh markers |
| Event leakage | Invalid wire messages exposed as valid plugin/API events |
| Interop trust | Docs claiming evidence that is stale or incomplete |

### Failure Routing

| Failure | Route To |
|---------|----------|
| OPEN/RFC 9072 bug | Future BGP message/capability fix spec |
| Capability parser bug | Future BGP capability fix spec |
| ADD-PATH parser/negotiation bug | Future BGP ADD-PATH fix spec |
| Route-refresh state or callback bug | Future BGP reactor/route-refresh fix spec |
| RFC 7606 duplicate/MP bug | Future BGP UPDATE validation fix spec, with plugin/RIB coordination if needed |
| Interop inventory docs drift | `spec-release-audit-8-docs-onboarding.md` |
| Heavy interop gate missing | `spec-release-evidence-gate.md` |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| RFC 7606 duplicate non-MP attribute tests were absent | `rfc7606_test.go` already includes duplicate ORIGIN and MED helper tests | Grep and read of `rfc7606_test.go` | Finding changed from missing unit tests to missing downstream first-wins enforcement evidence |
| RFC 9072 support only needed a strictness check | The TX and RX paths both still assume one-octet optional-parameter lengths around the extended marker | Read `open.go`, `session_negotiate.go`, `capability.go`, and `rfc/short/rfc9072.md` | RA-BGP-001 raised to Critical |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Treat every missing malformed-wire interop test as a blocker | Some malformed inputs are better covered by packet-level functional tests than external daemon interop | Findings request the future fix to choose interop or packet-level evidence explicitly |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| `~/.claude/rules/session-start.md` is referenced but absent in this environment | Seen in this session | Add OpenCode-compatible session-start location or remove stale pointer | Report to user in session summary |

## Design Insights

- RFC 9072 cannot be safely represented as raw optional-parameter bytes without also tracking whether the parameter length field was standard or extended.
- The current BGP session path validates UPDATE before callback dispatch, but non-UPDATE message types use callback-before-handler behavior. Future fixes must decide whether monitors are raw-wire monitors or validated-message consumers.
- RFC 7606 duplicate handling needs consumer-level proof, because a validator returning `none` is not enough if later attribute readers reject the same bytes.
- Interop inventory should be mechanically checked because both architecture and feature docs drifted behind the scenario tree.

## RFC Documentation

No RFC behavior is implemented in this audit spec. Future BGP fix specs must add direct RFC comments above code that enforces MUST or MUST NOT behavior.

## Audit Summary

### What Was Documented

- Created the BGP protocol release audit child spec.
- Recorded source-backed findings for RFC 9072, OPEN/capability parsing, ADD-PATH mode validation, route-refresh event ordering, RFC 7606 duplicate/MP handling, and interop docs drift.

### Findings Recorded

- RA-BGP-001 through RA-BGP-008 recorded in Initial Findings.
- No bugs are repaired in this protocol audit spec. Product code is outside this audit scope.

### Documentation Updates

- None yet. Documentation drift findings route to `spec-release-audit-8-docs-onboarding.md`.

### Deviations from Plan

- None.

## Implementation Audit

For this audit spec, "implementation" means audit documentation only. It does not mean product code changes.

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Audit BGP protocol release risk | Partial | Protocol Audit Matrix, Initial Findings | Source-backed findings recorded, future fixes not implemented |
| Cite RFC/source/test evidence | Partial | Required Reading, Initial Findings | RFC summaries and source files cited |
| Preserve audit-only scope | Met for this spec | Files to Modify | No product files are in scope |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Partial | Protocol Audit Matrix | Protocol surfaces mapped, not all future fixes executed |
| AC-2 | Partial | Initial Findings | Findings include evidence and requested verification |
| AC-3 | Partial | RA-BGP-001 through RA-BGP-004 | OPEN and capability findings recorded |
| AC-4 | Partial | RA-BGP-006, RA-BGP-007 | UPDATE/RFC 7606 findings recorded |
| AC-5 | Partial | RA-BGP-005 | Route-refresh finding recorded |
| AC-6 | Partial | RA-BGP-008 | Interop docs drift and negative coverage gap recorded |
| AC-7 | Met for this spec | Files to Modify | Audit-only scope documented |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Future RFC 9072 tests | Not started | TBD | Suggested verification only |
| Future malformed OPEN tests | Not started | TBD | Suggested verification only |
| Future capability length tests | Not started | TBD | Suggested verification only |
| Future route-refresh callback tests | Not started | TBD | Suggested verification only |
| Future duplicate consumer tests | Not started | TBD | Suggested verification only |
| Future MP NLRI syntax tests | Not started | TBD | Suggested verification only |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `plan/spec-release-audit-2-bgp-protocol.md` | Created | This file |

### Audit Summary

- **Total items:** 7 ACs, 8 initial findings
- **Done:** 0 product fixes
- **Partial:** BGP audit documentation created and source-backed findings recorded
- **Skipped:** None
- **Changed:** None

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Audit BGP protocol surface | Source/RFC evidence | Protocol Audit Matrix and Initial Findings cite BGP source files and RFC summaries |
| Make release protocol risk visible | Finding evidence | RA-BGP-001 through RA-BGP-008 record impact, reproduction, direction, and requested verification |
| Preserve audit-only scope | Spec evidence | Files to Modify and Audit Scope Boundary state no product changes in this spec |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Protocol audit records findings but does not execute future fixes | This file | Acknowledged |
| 2 | NOTE | Some malformed-wire evidence should be packet-level rather than external daemon interop | TDD Test Plan | Future fix specs must decide evidence type explicitly |

### Spec Edits Applied

- None.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-7 demonstrated (protocol surfaces mapped, findings evidence-backed, audit-only scope held)
- [ ] Every finding RA-BGP-001..RA-BGP-008 cites source/RFC/test evidence, owner, and requested verification
- [ ] `./le spec status` shows `release-audit-2-bgp-protocol`
- [ ] `./le verify worktree` unaffected (audit-only: this spec changes no product code or tests)

### TDD

<!-- Audit-only spec: no tests are added or changed here. These items are satisfied
     by the future fix specs that the Initial Findings request; each fix spec pastes
     its own failing/passing output. -->
- [ ] Tests written (N/A here -- owned by the future fix specs named in Initial Findings)
- [ ] Tests FAIL (N/A here -- paste output in each future fix spec)
- [ ] Tests PASS (N/A here -- paste output in each future fix spec)

### Completion

- [ ] Findings routed to their future owner areas (see Failure Routing)
- [ ] No production source, tests, schemas, docs, generated files, or Makefiles modified by this spec
