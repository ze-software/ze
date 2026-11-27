# Spec: bgp-ls-receiver-fault-management

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Validate untrusted BGP-LS input and prove receiver fault-management controls end to end against the live daemon and GoBGP. |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Dedicated `consumer-facing` per-peer YANG role (`RFC9552-8.2.6-2`) | Drop the redundant `consumer-facing` per-peer YANG role; operators use existing `family-filter` (`action remove`) to drop incoming BGP-LS updates (matching `rfc/short/rfc9552.md`, which already marks `RFC9552-8.2.6-2` `{not-applicable}`). Rebase and close this spec around the remaining live daemon `.ci` and GoBGP interop proof gaps, since NLRI syntactic validation, unit tests, and RFC ledger rows are already landed at HEAD. |

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| BGP-LS NLRI syntactic validator and registry registration | `internal/component/bgp/message/rfc7606_bgpls_nlri.go` (`judgeBGPLSNLRISection`, `RetainWellFormedNLRI`), `internal/component/bgp/message/register_nlrisyntax.go` (`init`), `internal/component/bgp/message/rfc7606.go` (`validateMPNLRISyntax`) | Validates BGP-LS (AFI 16388 / SAFI 71 and 72) NLRI framing and per-NLRI TLV/sub-TLV structure, ordering, and Node Descriptor uniqueness under RFC 9552 Section 8.2.2; non-skipable framing overruns reset the session with NOTIFICATION 3/9 and skipable errors discard only the malformed NLRI | Preserved as already landed at HEAD; this spec adds live daemon `.ci` and GoBGP interop proof covering this path |
| BGP-LS attribute validator | `internal/component/bgp/message/rfc7606_bgpls.go` (`validateBGPLSAttr`) | Registered under code 29 in `attrValidators`; checks BGP-LS Attribute TLV framing and returns `RFC7606ActionAttributeDiscard` on length mismatch | Preserved as already landed at HEAD; exercised in the live daemon `.ci` test |
| Session RFC 7606 and typed-NLRI receive enforcement | `internal/component/bgp/reactor/session_validation.go` (`Session.enforceRFC7606`, `applyRFC7606`, `rfc7606ResetNotification`), `internal/component/bgp/reactor/session_validation_nlritype.go` (`applyTypedNLRIDiscard`, `typedNLRIEdit`) | Runs `validateMPNLRISyntax` (`Judge`), strips skipable malformed Link-State NLRIs via `syntax.Retain`, drops emptied UPDATEs without relaying, applies `ATTR_TOMBSTONE` (code 252) on Attribute Discard, and sends NOTIFICATION 3/9 on non-skipable framing errors | Preserved as already landed at HEAD; exercised end to end through live sessions |
| Family import filter | `internal/component/bgp/plugins/filter_family/config.go` (`parseFamilyFilters`) | Lets operators configure `bgp/policy/family-filter` (`action remove` or `tear-down`) to drop inbound BGP-LS updates on Consumer-facing peers (`RFC9552-8.2.6-1`) | Preserved; per the 2026-11-24 owner decision, replaces the dropped dedicated `consumer-facing` YANG role |
| Compiled plugin test fixtures | `internal/test/fixture/plugin_fixture_13.go` (`routeServerReplay13`) | Registers compiled Go observer fixtures (`le test fixture plugin/<name>`) that verify route-server delivery and shut the daemon down cleanly after the processing fence | Extends by registering `plugin/bgp-ls-malformed-nlri` for the live daemon `.ci` test |
| Docker BGP interop lab and GoBGP RIB reader | `internal/le/interoplab/bgp/` (`names.go`, `checkers.go`, `check_extras.go`), `test/interop/Dockerfile.gobgp` (`gobgp-paths`) | Runs multi-daemon Docker scenarios against pinned GoBGP v4.11.0 with raw injectors (`bgp-sr-policy-malformed-receive-gobgp`) and reads non-CLI AFI/SAFI tables via `gobgp-paths <afi> <safi>` | Extends with scenario `bgp-ls-receive-gobgp` reading GoBGP's AFI 16388 / SAFI 71 RIB |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order" | agrees | The `.ci` and GoBGP interop proofs assert exact RFC 9552 Section 8.2.2 and RFC 4760 Section 7 receive behavior on the wire without altering any published API or config contract. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/test/fixture/plugin_fixture_13.go`, `internal/le/interoplab/bgp/names.go`, `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/check_extras.go`, `test/plugin/bgp-ls-malformed-nlri.ci`, and `test/interop/scenarios/bgp-ls-receive-gobgp/*` | "A compiled Go fixture returns an error when an assertion fails" | agrees | The new `plugin/bgp-ls-malformed-nlri` registration reuses `routeServerReplay13` in `plugin_fixture_13.go`, which reports assertion failures before shutdown, and the new `.ci` lives in `test/plugin/` where live `le test peer` + `ze -` runs are supported. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | the `// Design:` page of `internal/le/interoplab/bgp/names.go`, `checkers.go`, `check_extras.go`; anchored to all three; a page this spec modifies | "A scenario directory carries only declarative inputs its runner reads" | agrees | `test/interop/scenarios/bgp-ls-receive-gobgp/` holds only declarative `ze.conf`, `gobgp.toml`, `inject.msg`, and `inject-args`; all assertions live in the typed Go checker registry in `internal/le/interoplab/bgp/`. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/testing/interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | `bgp-ls-receive-gobgp` is a pure userspace BGP scenario in the BGP suite and declares no extra kernel capabilities, running without requiring unrelated SRv6 or IPsec kernel features. |
| `docs/architecture/testing/ci-format.md` | The compiled observer API | anchored to `internal/test/fixture/plugin_fixture_13.go` | "`runPlugin13` supplies that barrier when its shutdown flag is true." | agrees | `plugin/bgp-ls-malformed-nlri` registers with `routeServerReplay13` and `StopsDaemon`, preserving the quiesce-before-shutdown observer barrier. |
| `docs/architecture/ospf/ospf-4-component-config.md` | Decisions | anchored to `internal/le/interoplab/bgp/check_extras.go` | - | silent | Anchored to `check_extras.go` only for `ospf-auto-cost-frr`; this spec adds a BGP-LS scenario entry to `scenarioExtras` and does not touch OSPF component registration or configuration. |
| `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md` | Decisions | anchored to `internal/le/interoplab/bgp/checkers.go` | - | silent | Anchored to `checkers.go` only for `ospfIPsecPeerSetup`; this spec adds a BGP-LS scenario entry to `scenarioOperations` and does not touch OSPFv3 IPsec authentication. |
| `docs/functional-tests.md` | Overview | a page this spec modifies | "Functional tests exercise release-gate behavior across BGP wire encoding and decoding, plugin behavior" | agrees | `test/plugin/bgp-ls-malformed-nlri.ci` adds release-gate live-daemon coverage in the `plugin` functional suite and is documented in `docs/functional-tests.md`. |
| `ai/doors/bgp-engine.md` | Gates | added: governing BGP-LS receive-path design | "Wire output, decoding, config parsing and plugin paths through the real daemon" | agrees | The landed unit tests cover `validateMPNLRISyntax` and `enforceRFC7606`; this spec closes the remaining real-daemon `.ci` and Docker GoBGP interop gates. |
| `docs/architecture/wire/nlri-bgpls.md` | Receive-Path NLRI Validation (RFC 9552 Section 8.2.2) | added: governing BGP-LS receive-path design | "Ze implements no AFI/SAFI disable, so the session resets." | agrees | AC-7 and `test/plugin/bgp-ls-malformed-nlri.ci` match this landed behavior: a non-skipable Link-State NLRI framing error resets even a mixed-family session with NOTIFICATION 3/9. |
| `docs/architecture/wire/nlri.md` | NLRI Receive Validation: the Per-Family Syntax Registry | added: governing BGP-LS receive-path design | "No receive-path file names a family." | agrees | The existing `nlrisyntax` registration in `register_nlrisyntax.go` is preserved untouched; no family-specific branch is added to `rfc7606.go` or `session_validation.go`. |

## Task

RFC 9552 Section 8.2.2 requires a BGP-LS speaker to syntactically validate Link-State NLRIs and BGP-LS Attributes on the receive path and take the error-class action the RFC prescribes ('NLRI discard' for skipable NLRI errors, 'Attribute Discard' for skipable BGP-LS Attribute errors, and 'session reset' with NOTIFICATION 3/9 for non-skipable NLRI framing errors when 'AFI/SAFI disable' is not implemented).

At HEAD, the producer implementation, boundary and receive-path unit tests, RFC ledger entries, and wire architecture documentation are already landed:

| Item | State at HEAD |
|------|---------------|
| Link-State NLRI syntactic validator (`judgeBGPLSNLRISection`, `RetainWellFormedNLRI`) | Landed in `internal/component/bgp/message/rfc7606_bgpls_nlri.go` and registered in `register_nlrisyntax.go` |
| BGP-LS Attribute syntactic validator (`validateBGPLSAttr`) | Landed in `internal/component/bgp/message/rfc7606_bgpls.go` |
| Receive-path enforcement (`enforceRFC7606`, `typedNLRIEdit`) | Landed in `internal/component/bgp/reactor/session_validation.go` and `session_validation_nlritype.go` |
| Boundary, receive-path, and semantic-tolerance unit tests | Landed in `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go`, `rfc7606_bgpls_nlri_fuzz_test.go`, `internal/component/bgp/reactor/rfc9552_nlri_test.go`, `rfc9552_test.go`, and `rfc9552_semantic_test.go` |
| RFC 9552 ledger rows (`8.2.2-4`, `8.2.2-5`, `8.2.2-6`, `8.2.2-9`, `8.2.2-10`, `8.2.6-2`) | Already updated in `rfc/short/rfc9552.md` (`8.2.6-2` marked `{not-applicable}`; 2026-11-24 owner decision dropped the dedicated `consumer-facing` YANG role) |
| Wire architecture doc (`Receive-Path NLRI Validation`) | Already documented in `docs/architecture/wire/nlri-bgpls.md` and `docs/architecture/wire/nlri.md` |
| Live daemon functional `.ci` proof | **Missing** -- no `.ci` test drives malformed Link-State NLRIs (skipable discard + non-skipable session reset) and malformed BGP-LS Attribute through a running `ze` daemon |
| GoBGP Docker interop proof (`bgp-ls-receive-gobgp`) | **Missing** -- no interop scenario proves zero false positives on valid BGP-LS NLRIs and live skipable-NLRI discard upstream of real GoBGP v4.11.0 |

This spec closes those two remaining proof gaps so the P0 release fix can be verified end to end and closed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - Link-State NLRI framing, Receive-Path Fault Management, and Receive-Path NLRI Validation
  → Decision: non-skipable framing boundary is drawn at Total NLRI Length (`judgeBGPLSNLRISection` -> session reset because Ze implements no AFI/SAFI disable); everything inside a well-framed NLRI is skipable (`RetainWellFormedNLRI` -> NLRI discard)
  → Constraint: BGP-LS TLV header is 4 octets with no 4-octet padding; unknown NLRI types stay opaque and propagate whole; no TLV value octet is interpreted semantically
- [ ] `docs/architecture/wire/nlri.md` - NLRI Receive Validation: the Per-Family Syntax Registry
  → Decision: BGP-LS (`bgp-ls/bgp-ls` and `bgp-ls/bgp-ls-vpn`) registers its `Judge` and `Retain` callbacks in `nlrisyntax`; no receive-path file names a family
  → Constraint: every MP NLRI session reset carries NOTIFICATION 3/9 (Optional Attribute Error) with the whole MP attribute as Data per RFC 4760 Section 7
- [ ] `docs/architecture/testing/interop.md` - Docker interop scenario discovery, GoBGP v4.11.0 `gobgp-paths`, and malformed-receive checker conventions
  → Decision: scenario directories contain only declarative config/injector inputs; assertions live in typed Go checkers under `internal/le/interoplab/bgp/`
  → Constraint: absence assertions require a live positive control (`proof`) and unchanged session counters so a dropped session cannot pass vacuously
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` functional test format and suite separation
  → Constraint: `test/decode/` strictly runs single-command offline `ze bgp decode` tests and rejects `le test peer` / `ze -`; live daemon peer tests belong in `test/plugin/`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9552.md` - Sections 5.1, 5.2, 5.2.1.4, 8.2.2, and 8.2.6
  → Constraint: "A Link-State NLRI MUST NOT be considered malformed or invalid based on the inclusion/exclusion of TLVs or contents of the TLV fields (i.e., semantic errors)"; structural length, §5.1 Type/Length/opaque-Value ordering, and §5.2.1.4 Node Descriptor sub-TLV uniqueness still apply
  → Constraint: skipable NLRI errors take 'NLRI discard'; unprocessable framing errors take 'session reset' (because Ze implements no 'AFI/SAFI disable'); skipable BGP-LS Attribute errors take 'Attribute Discard'
  → Constraint: `RFC9552-8.2.6-2` is `{not-applicable}` (operator policy obligation satisfied by existing `bgp/policy/family-filter` under `RFC9552-8.2.6-1`)

**Key insights:**
- All producer validation code and unit tests are already landed and wired through `nlrisyntax` (`register_nlrisyntax.go`) and `attrValidators` (`rfc7606_bgpls.go`).
- Live daemon `.ci` tests using `le test peer` + `ze -` must live under `test/plugin/` (`test/plugin/bgp-ls-malformed-nlri.ci`), not `test/decode/`, because `internal/test/runner/decoding_strict_test.go` enforces that `test/decode/` accepts only `ze bgp decode` commands.
- `test/interop/Dockerfile.gobgp` already ships `gobgp-paths <afi> <safi>`, which queries GoBGP v4.11.0's global RIB over gRPC `ListPath` with `EnableOnlyBinary` and decodes NLRIs via `bgp.NLRIFromSlice`, matching the pattern proven by `bgp-sr-policy-malformed-receive-gobgp`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/rfc7606_bgpls_nlri.go` - already landed: implements `RetainWellFormedNLRI` (zero-copy when nothing is dropped; handles SAFI 71, SAFI 72 8-octet RD, RFC 7911 ADD-PATH, unknown NLRI types kept opaque, TLV length walk, §5.1 Type/Length/Value ordering, §5.2.1.4 Node Descriptor 256/257 sub-TLV uniqueness and ascending order) and `judgeBGPLSNLRISection` (returns `ActionSessionReset` when Total NLRI Length overruns the section or ends inside an NLRI header).
- [ ] `internal/component/bgp/message/register_nlrisyntax.go` - already landed: registers `judgeBGPLSNLRISection` and `RetainWellFormedNLRI` in `nlrisyntax` for both `bgp-ls/bgp-ls` and `bgp-ls/bgp-ls-vpn`.
- [ ] `internal/component/bgp/message/rfc7606.go` - already landed: `validateMPNLRISyntax` queries `nlrisyntax.Get(fam)` first and maps `ActionSessionReset` to `RFC7606ActionSessionReset` with RFC 4760 Section 7 `MPAttributeErrorNotification(attr)` (NOTIFICATION 3/9).
- [ ] `internal/component/bgp/message/rfc7606_bgpls.go` - already landed: registers `validateBGPLSAttr` for attribute code 29 returning `RFC7606ActionAttributeDiscard` (`DiscardReasonInvalidLength`) on TLV overrun or trailing header octets.
- [ ] `internal/component/bgp/reactor/session_validation.go` and `internal/component/bgp/reactor/session_validation_nlritype.go` - already landed: `enforceRFC7606` runs `classifyRFC7606` (`validateMPNLRISyntax`) before plugin dispatch, applies `applyTypedNLRIDiscard` -> `typedNLRIEdit` (`syntax.Retain`), drops emptied UPDATEs without relaying, applies `ATTR_TOMBSTONE` (code 252) on Attribute Discard, and calls `rfc7606ResetNotification` on session reset.
- [ ] `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go`, `internal/component/bgp/reactor/rfc9552_nlri_test.go`, `internal/component/bgp/reactor/rfc9552_test.go`, `internal/component/bgp/reactor/rfc9552_semantic_test.go` - already landed: full boundary, receive-path, and semantic-tolerance unit coverage for RFC 9552 Section 8.2.2.
- [ ] `internal/test/fixture/plugin_fixture_13.go` - registers compiled Go observer fixtures (`routeServerReplay13`) for live `test/plugin/*.ci` tests including `plugin/rfc7606-54-bgpls-override-propagates`.
- [ ] `internal/le/interoplab/bgp/names.go`, `checkers.go`, `check_extras.go`, and `test/interop/scenarios/bgp-sr-policy-malformed-receive-gobgp/` - established GoBGP v4.11.0 + raw injector pattern using `gobgp-paths <afi> <safi>`, IPv4 fence synchronization, and Ze `show bgp peer <addr> detail` session-stability assertions.
- [ ] `rfc/short/rfc9552.md` - rows `RFC9552-8.2.2-4`, `8.2.2-5`, `8.2.2-6`, `8.2.2-9`, `8.2.2-10` already proven; `RFC9552-8.2.6-2` already marked `{not-applicable}`.

**Behavior to preserve:**
- All already-landed BGP-LS NLRI and Attribute syntactic validation in `internal/component/bgp/message/` and `internal/component/bgp/reactor/` (no product code changes unless a live `.ci` or GoBGP interop test exposes a defect).
- Zero-copy fast path for conforming BGP-LS UPDATEs (`RetainWellFormedNLRI` returns the input slice unchanged when `dropped == 0`).
- Preservation and opaque propagation of unknown Link-State NLRI types (`test/plugin/rfc7606-54-bgpls-override-propagates.ci`) and semantically odd TLVs (`rfc9552_semantic_test.go`).
- Existing `bgp/policy/family-filter` import-drop behavior for Consumer-facing peers.

**Behavior to change:**
- Add live daemon functional test `test/plugin/bgp-ls-malformed-nlri.ci` (with fixture `plugin/bgp-ls-malformed-nlri` in `internal/test/fixture/plugin_fixture_13.go`) proving the complete wire-to-relay and wire-to-NOTIFICATION receive path on a running `ze` daemon.
- Add Docker GoBGP interop scenario `bgp-ls-receive-gobgp` (`test/interop/scenarios/bgp-ls-receive-gobgp/` + checker in `internal/le/interoplab/bgp/`) proving valid BGP-LS acceptance with zero false positives and live skipable-NLRI discard upstream of GoBGP v4.11.0.
- Document the new functional test and interop scenario in `docs/functional-tests.md`, `docs/architecture/testing/interop.md`, and `docs/features/rfc-status.md`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire bytes: BGP UPDATE messages carrying `MP_REACH_NLRI` or `MP_UNREACH_NLRI` for AFI 16388 / SAFI 71 (and optional BGP-LS Attribute code 29) arriving over TCP from `le test peer` (functional `.ci`) or the raw injector container (Docker GoBGP interop) into an established `ze` BGP session.

### Transformation Path
1. `Session.enforceRFC7606` (`internal/component/bgp/reactor/session_validation.go`) classifies the received `wireu.WireUpdate` before plugin dispatch.
2. `validateMPNLRISyntax` (`internal/component/bgp/message/rfc7606.go`) looks up AFI 16388 / SAFI 71 in `nlrisyntax` and calls `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go`). If Total NLRI Length overruns the MP attribute or leaves a truncated header, `applyRFC7606` sends NOTIFICATION 3/9 (`MPAttributeErrorNotification`) and resets the session.
3. For a walkable MP section, `validateBGPLSAttr` (`rfc7606_bgpls.go`) validates attribute code 29 TLVs and returns `RFC7606ActionAttributeDiscard` if any TLV length overruns or leaves trailing header octets.
4. `applyTypedNLRIDiscard` -> `typedNLRIEdit` (`session_validation_nlritype.go`) invokes `RetainWellFormedNLRI` (`rfc7606_bgpls_nlri.go`) to discard skipable malformed Link-State NLRIs (TLV/sub-TLV overrun, §5.1 ordering violation, §5.2.1.4 duplicate/descending Node Descriptor sub-TLV) while preserving well-formed NLRIs and unknown NLRI types.
5. Surviving NLRIs (with `ATTR_TOMBSTONE` code 252 if code 29 was discarded) reach `bgp-rs` / `bgp-adj-rib-in` and are relayed to the downstream peer (`le test peer` receiver connection or GoBGP v4.11.0).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer TCP ↔ `message` (`nlrisyntax` / `attrValidators`) | Raw BGP UPDATE frames validated in-process before plugin dispatch | Yes (`TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly`, `TestRetainWellFormedNLRIBoundaries`, `test/plugin/bgp-ls-malformed-nlri.ci`) |
| `message` ↔ `reactor` (`enforceRFC7606` / `typedNLRIEdit`) | `RFC7606ValidationResult`, `nlrisyntax.Judgement`, and `Retain` byte slice drive session reset, NLRI discard, or Attribute Discard | Yes (`TestRFC9552*` in `internal/component/bgp/reactor/`, `test/plugin/bgp-ls-malformed-nlri.ci`, counterfactual red on `RetainWellFormedNLRI`) |
| `reactor` ↔ `bgp-rs` / downstream peer (`le test peer` or GoBGP) | Surviving NLRIs relayed over BGP TCP; GoBGP RIB queried via `gobgp-paths 16388 71` | Yes (`test/plugin/bgp-ls-malformed-nlri.ci` PASS 3.1s + 80/80 stress-repro; `bgp-ls-receive-gobgp` PASS 6m8s against real GoBGP v4.11.0) |

### Changed-Wire Census

No wire format, field layout, encoding, or RPC schema changes in this spec: all RFC 9552 Section 8.2.2 receive-path validation is already landed at HEAD (`internal/component/bgp/message/rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`, `register_nlrisyntax.go`, `internal/component/bgp/reactor/session_validation_nlritype.go`), and this spec adds only live functional (`.ci`) and Docker interop (`bgp-ls-receive-gobgp`) tests plus documentation proving the existing wire behavior end to end.

### Integration Points
- `internal/test/fixture/plugin_fixture_13.go` (`init`, `routeServerReplay13`) - registers `plugin/bgp-ls-malformed-nlri` for clean fence-driven shutdown in `test/plugin/bgp-ls-malformed-nlri.ci`.
- `internal/le/interoplab/bgp/names.go`, `checkers.go`, `check_extras.go` - registers `bgp-ls-receive-gobgp` in the BGP interop suite's `scenarioOperations` and `scenarioExtras` maps so `Discover` (`internal/le/interoplab/discover.go`) discovers and validates it automatically.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Live `.ci` (`test/plugin/bgp-ls-malformed-nlri.ci`) and Docker interop (`bgp-ls-receive-gobgp`) drive raw TCP BGP UPDATEs through `Session.enforceRFC7606` (`session_validation.go`), `validateMPNLRISyntax` (`rfc7606.go`), `nlrisyntax` (`register_nlrisyntax.go`), `validateBGPLSAttr` (`rfc7606_bgpls.go`), and `typedNLRIEdit` (`session_validation_nlritype.go`) before `bgp-rs` relay or NOTIFICATION 3/9. |
| No unintended coupling (components stay isolated) | Yes | No product package edited; `internal/test/fixture/plugin_fixture_13.go` and `internal/le/interoplab/bgp/` stay strictly inside test/tooling packages and respect `TestNormalZeLinksNoInternalLe`. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `routeServerReplay13` (`plugin_fixture_13.go`), `gobgp-paths` (`test/interop/Dockerfile.gobgp`), `opWaitContains`/`opRequireAbsent`/`opRequireJSONFields` (`internal/le/interoplab/bgp/`), and the existing `nlrisyntax`/`attrValidators` registries. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `RetainWellFormedNLRI` (`rfc7606_bgpls_nlri.go:166-168`) returns `section` unchanged when `dropped == 0`, pinned by `TestRetainWellFormedNLRIIsZeroCopyWhenNothingIsDropped`; `validateBGPLSAttr` allocates nothing. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | BGP-LS syntax rules register via `nlrisyntax.Register` (`register_nlrisyntax.go:23-30`) and `attrValidators[29]` (`rfc7606_bgpls.go:32`); `plugin/bgp-ls-malformed-nlri` registers via `fixture.Register` (`plugin_fixture_13.go:18`); `bgp-ls-receive-gobgp` registers in `scenarioOperations`/`scenarioExtras` discovered by `interoplab.Discover`. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No core switch, CLI parser, YANG schema, or completion table names `bgp-ls-malformed-nlri` or `bgp-ls-receive-gobgp`; `.ci` runner discovers `test/plugin/*.ci` from disk, `fixture.Run` resolves `plugin/bgp-ls-malformed-nlri` from `fixture.Register`, and `interoplab.Discover` joins `test/interop/scenarios/bgp-ls-receive-gobgp/` against `scenarioOperations`/`scenarioExtras` (`TestCheckerPopulationMatchesProducer` PASS). |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | GoBGP v4.11.0 accepts `afi-safi-name = "ls"` in `gobgp.toml` and `gobgp-paths 16388 71` decodes relayed BGP-LS Node/Link NLRIs via `bgp.NLRIFromSlice` | `test/interop/Dockerfile.gobgp` builds `github.com/osrg/gobgp/v4@v4.11.0` with `gobgp-paths <afi> <safi>` calling `bgp.NLRIFromSlice(bgp.NewFamily(uint16(afi), uint8(safi)), path.NlriBinary)` under `EnableOnlyBinary: true` | Adjusted `bgplsReceived*` tokens in `names.go` to match GoBGP's exact `LsNodeNLRI.String()` / `LsLinkNLRI.String()` format | `INTEROP_SCENARIO=bgp-ls-receive-gobgp ./le test integration interop` PASS (6m8s): GoBGP v4.11.0 negotiated `ls`, `gobgp-paths 16388 71` decoded `NODE 10.95.52.1`, `LINK 10.95.52.11->10.95.52.12`, and `NODE 10.95.52.3`, omitted malformed `10.95.52.9`, with `connections-established: 1`, `connections-dropped: 0` | confirmed |
| A-2 | The already-landed BGP-LS validator in `rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`, and `session_validation_nlritype.go` needs no product code changes to pass the live `.ci` and GoBGP interop proofs | All unit tests in `rfc7606_bgpls_nlri_test.go`, `rfc9552_nlri_test.go`, `rfc9552_test.go`, and `rfc9552_semantic_test.go` are landed at HEAD | None -- no product defect surfaced | `test/plugin/bgp-ls-malformed-nlri.ci` PASS (3.1s, 80/80 under 64 CPU burners, counterfactual red proved) and `INTEROP_SCENARIO=bgp-ls-receive-gobgp ./le test integration interop` PASS (6m8s) with zero edits to `internal/component/bgp/` | confirmed |
| A-3 | `test/plugin/` is the right functional suite for live malformed BGP-LS session tests | `internal/test/runner/decoding_strict_test.go` and `docs/functional-tests.md` prove `test/decode/` rejects any command other than `ze bgp decode`, while `test/plugin/rfc7606-54-bgpls-override-propagates.ci` and `test/plugin/rfc7606-reset.ci` run `le test peer` + `ze -` in `test/plugin/` | None -- verified directly in runner source | Read `internal/test/runner/decoding_strict_test.go` and `test/plugin/rfc7606-54-bgpls-override-propagates.ci`; `TestCIFrameLengthsWellFormed`, `TestCIFrameStructureWellFormed`, and `TestPluginObserversWaitForTheScriptedPeer` PASS on `test/plugin/bgp-ls-malformed-nlri.ci` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The validator false-positives on a valid Link-State NLRI or BGP-LS Attribute that real GoBGP exchanges or accepts | `bgp-ls-receive-gobgp` fails to deliver valid Node/Link NLRIs to GoBGP or drops the session | Run `bgp-ls-receive-gobgp` against real GoBGP v4.11.0 and assert `connections-established: 1`, `connections-dropped: 0` alongside exact decoded NLRI fields |
| R-2 | Interop absence check passes vacuously if GoBGP's session drops or the injector never sends | GoBGP loses the malformed NLRI only because the session reset or no routes arrived | Pair every malformed NLRI with a well-formed survivor NLRI plus an untouched control NLRI (`proof`) and assert `connections-established: 1`, `connections-dropped: 0` on Ze's GoBGP peer |
| R-3 | Live `.ci` test has timing flake between skipable-discard relay and non-skipable session-reset phases | Receiver misses relayed survivor before reset or daemon exits early | Use deterministic TCP-order processing fence (`10.0.0.0/24` IPv4 UPDATE after BGP-LS UPDATEs, as in `rfc7606-54-bgpls-override-propagates.ci`) and a dedicated peer connection for the session-reset NOTIFICATION 3/9 assertion |
| R-4 | A peer stalls the TLV walk with zero-length TLVs or 65535-octet claims | Hang or timeout on malformed input | Already bounded in `rfc7606_bgpls_nlri.go` (every loop advances by at least `bgplsTLVHeaderLen = 4`) and fuzzed in `rfc7606_bgpls_nlri_fuzz_test.go` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A broken `.ci` or interop test causes false red or vacuous green in release gates; no production forwarding or config path changes unless a live test uncovers a real defect in the landed validator. |
| How is it reverted? | Single commit revert removes the new `.ci` file, fixture registration, interop scenario, checker entries, and doc notes; no config migration or state change. |
| Who else touches this path? | `internal/test/fixture/plugin_fixture_13.go` and `internal/le/interoplab/bgp/{names,checkers,check_extras}.go` are shared test registries; changes are additive single-scenario entries. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an UPDATE with AFI 16388 and a malformed Link-State NLRI (already landed) | → | `validateMPNLRISyntax` / `judgeBGPLSNLRISection` / `RetainWellFormedNLRI` | `TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly` (`internal/component/bgp/message/rfc7606_bgpls_nlri_test.go`) |
| malformed and valid Link-State NLRIs on a session (already landed) | → | `Session.enforceRFC7606` / `typedNLRIEdit` | `TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`, `TestRFC9552LinkStateNLRILengthOverrunResetsSession`, `TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated` (`internal/component/bgp/reactor/rfc9552_nlri_test.go`) |
| malformed BGP-LS Attribute on a session (already landed) | → | `Session.enforceRFC7606` / `validateBGPLSAttr` | `TestRFC9552BGPLSAttributeTLVOverrunDiscarded` (`internal/component/bgp/reactor/rfc9552_test.go`) |
| live BGP peer sending skipable malformed NLRI, malformed BGP-LS Attribute, and non-skipable Total NLRI Length overrun to running `ze` daemon | → | full daemon receive path (`session_read` → `enforceRFC7606` → `bgp-rs` relay / NOTIFICATION 3/9) | `test/plugin/bgp-ls-malformed-nlri.ci` |
| upstream injector + `ze` route server + GoBGP v4.11.0 exchanging valid and skipable-malformed BGP-LS NLRIs | → | full daemon receive + relay path judged by real GoBGP v4.11.0 | `bgp-ls-receive-gobgp` (`test/interop/scenarios/bgp-ls-receive-gobgp/`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A Link-State NLRI whose TLV lengths do not sum to its Total NLRI Length (body truncated inside fixed fields or trailing TLV header) beside a well-formed NLRI | `RetainWellFormedNLRI` discards the malformed NLRI ('NLRI discard') and delivers the well-formed survivor byte-identical with session remaining established | Well-formed NLRIs in the same MP attribute survive because RFC 9552 Section 8.2.2 makes per-NLRI framing errors inside a valid Total NLRI Length skipable |
| AC-2 | `MP_REACH_NLRI` whose Link-State Total NLRI Lengths do not sum to the attribute length (overruns section or truncated 4-octet NLRI header) | `judgeBGPLSNLRISection` answers `ActionSessionReset` and `enforceRFC7606` sends NOTIFICATION 3/9 (Optional Attribute Error carrying the MP attribute) and resets the session | No NLRIs from that UPDATE survive because NLRI boundaries after the error cannot be located (RFC 9552 Section 8.2.2, RFC 7606 Section 3(j)) |
| AC-3 | `MP_UNREACH_NLRI` whose Link-State Total NLRI Lengths do not sum to the attribute length | `judgeBGPLSNLRISection` answers `ActionSessionReset` and `enforceRFC7606` sends NOTIFICATION 3/9 and resets the session | No NLRIs from that UPDATE survive because the withdraw-side MP attribute cannot be framed (RFC 9552 Section 8.2.2 bullet 2) |
| AC-4 | A TLV or recognized Node Descriptor (256/257) sub-TLV whose declared length overruns its container, beside a well-formed NLRI | `RetainWellFormedNLRI` discards that NLRI ('NLRI discard') and relays the well-formed survivor unchanged while keeping the session established | Well-formed NLRIs in the same UPDATE survive; the overrunning NLRI is bounded by its own valid Total NLRI Length and is therefore skipable |
| AC-5 | A Link-State NLRI whose TLVs violate RFC 9552 Section 5.1 ordering (descending Type, or equal Type with descending Length, or equal Type and Length with descending opaque Value), beside a well-formed NLRI | `RetainWellFormedNLRI` discards the out-of-order NLRI ('NLRI discard') and relays the well-formed survivor unchanged while keeping the session established | Correctly ordered NLRIs (including unknown TLV types and identical repeated TLVs) survive because Section 5.1 forbids only ordering violations |
| AC-6 | A Local (256) or Remote (257) Node Descriptor carrying a repeated sub-TLV type or descending sub-TLV types (RFC 9552 Section 5.2.1.4), beside a well-formed NLRI | `RetainWellFormedNLRI` discards that NLRI ('NLRI discard') and relays the well-formed survivor unchanged while keeping the session established | Well-formed NLRIs survive; Node Descriptors with strictly ascending unique sub-TLV types (including unknown sub-TLV types) survive |
| AC-7 | A non-skipable Total NLRI Length overrun on a mixed-family session negotiating both IPv4 unicast and BGP-LS | `enforceRFC7606` sends NOTIFICATION 3/9 (Optional Attribute Error carrying the MP attribute) and resets the session, matching RFC 9552 Section 8.2.2's mandatory fallback when `'AFI/SAFI disable'` is not implemented (`rfc7606_bgpls_nlri.go`) | No route from that malformed UPDATE survives and the session resets; Ze implements no `'AFI/SAFI disable'`, so Section 8.2.2's "Alternately, the router MUST perform a 'session reset'" applies |
| AC-8 | A non-skipable Total NLRI Length overrun on a BGP-LS-only session | `enforceRFC7606` sends NOTIFICATION 3/9 (Optional Attribute Error carrying the MP attribute) and resets the session | No route from that malformed UPDATE survives; Section 8.2.2 requires session reset for a BGP-LS-only session |
| AC-9 | A syntactically valid Link-State NLRI or BGP-LS Attribute with semantic oddities (unknown NLRI type, unknown/unexpected TLV type, missing mandatory descriptor, unregistered Protocol-ID, non-standard fixed-length TLV size, or unordered BGP-LS Attribute TLVs) | `enforceRFC7606` takes `RFC7606ActionNone` and propagates the NLRI and attribute byte-identical to plugins and downstream peers | All syntactically valid NLRIs and attributes survive unchanged because RFC 9552 Section 8.2.2 forbids semantic rejection by a Propagator |
| AC-10 | Valid Link-State NLRIs (Node and Link) and a skipable malformed BGP-LS Attribute (TLV overrun) sent through a live `ze` daemon and against GoBGP v4.11.0 | Valid NLRIs reach downstream peers (`le test peer` and GoBGP `gobgp-paths 16388 71`) intact with zero session drops (`connections-established: 1`, `connections-dropped: 0`); a malformed BGP-LS Attribute is replaced by `ATTR_TOMBSTONE` (code 252) while its Link-State NLRI still propagates; a skipable malformed NLRI injected beside a valid control is discarded before reaching GoBGP | Well-formed NLRIs and unaffected control routes survive on both sessions; only the malformed NLRI or malformed attribute code 29 is removed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers `ze` with a BGP-LS speaker that sends skipable malformed NLRIs (TLV overrun, out-of-order TLVs, duplicate Node Descriptor sub-TLV), a malformed BGP-LS Attribute, and a non-skipable truncated NLRI section | `le test peer` → TCP → `enforceRFC7606` → `validateMPNLRISyntax` / `validateBGPLSAttr` / `typedNLRIEdit` → `bgp-rs` relays survivors or sends NOTIFICATION 3/9 | `test/plugin/bgp-ls-malformed-nlri.ci` |
| 2 | Runs `ze` as a BGP-LS route server in front of a real GoBGP v4.11.0 peer receiving valid BGP-LS NLRIs and a skipable malformed NLRI beside a valid survivor | injector → `ze` (`enforceRFC7606` + `bgp-rs`) → GoBGP v4.11.0 (`gobgp-paths 16388 71`) | `bgp-ls-receive-gobgp` (`test/interop/scenarios/bgp-ls-receive-gobgp/`) |

## 🧪 TDD Test Plan

### Unit Tests (already landed at HEAD)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRetainWellFormedNLRIBoundaries` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-1, AC-4, AC-5, AC-6, AC-9 | Landed at HEAD |
| `TestRetainWellFormedNLRIIsZeroCopyWhenNothingIsDropped` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | Zero-copy fast path | Landed at HEAD |
| `TestRetainWellFormedNLRISkipsThePathIdentifier` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | ADD-PATH framing | Landed at HEAD |
| `TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly` | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | AC-2, AC-7, AC-8 | Landed at HEAD |
| `TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-9, AC-10 | Landed at HEAD |
| `TestRFC9552LinkStateNLRITLVOverrunIsDiscarded` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-1, AC-4 | Landed at HEAD |
| `TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-6 | Landed at HEAD |
| `TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-5 | Landed at HEAD |
| `TestRFC9552LinkStateSameTypeTLVsOutOfOrderAreDiscarded` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-5 | Landed at HEAD |
| `TestRFC9552LinkStateSameTypeTLVsInOrderArePropagated` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-5 | Landed at HEAD |
| `TestRFC9552LinkStateRecognizedTLVSubTLVOverrunIsDiscarded` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-4 | Landed at HEAD |
| `TestRFC9552LinkStateNLRIUnknownTLVOverrunIsDiscarded` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-4, AC-9 | Landed at HEAD |
| `TestRFC9552LinkStateNLRILengthOverrunResetsSession` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-2, AC-7, AC-8 | Landed at HEAD |
| `TestRFC9552LinkStateUnreachLengthOverrunResetsSession` | `internal/component/bgp/reactor/rfc9552_nlri_test.go` | AC-3, AC-7, AC-8 | Landed at HEAD |
| `TestRFC9552LinkStateNLRISemanticErrorsAreNotMalformed` | `internal/component/bgp/reactor/rfc9552_semantic_test.go` | AC-9 | Landed at HEAD |
| `TestRFC9552BGPLSAttributeWellFormedIsKept` / `TestRFC9552BGPLSAttributeTLVOverrunDiscarded` / `TestRFC9552BGPLSAttributeTrailingOctetsDiscarded` | `internal/component/bgp/reactor/rfc9552_test.go` | AC-10 (`RFC9552-8.2.2-6`) | Landed at HEAD |

### Boundary Tests (numeric inputs — landed in `rfc7606_bgpls_nlri_test.go`)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP-LS TLV Length | 0 - 65535 | remaining octets in NLRI body | N/A | 1 octet past NLRI body |
| Node Descriptor sub-TLV Length | 0 - 65535 | remaining octets in TLV 256/257 | N/A | 1 octet past TLV 256/257 |
| Total NLRI Length (SAFI 71) | 9 - 65535 (known types 1-4) | exact sum of fixed 9 octets + TLVs | 8 octets | 1 octet past MP section (session reset) |
| Total NLRI Length (SAFI 72) | 17 - 65535 (known types 1-4) | exact sum of 8-octet RD + 9 fixed + TLVs | 7 octets (short of RD) | 1 octet past MP section (session reset) |
| BGP-LS NLRI / TLV header | fixed 4 octets | 4 octets | 3 octets of tail | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-ls-malformed-nlri` | `test/plugin/bgp-ls-malformed-nlri.ci` | Running `ze` daemon receives (1) skipable malformed Link-State NLRIs (TLV overrun, descending TLV order, duplicate Node Descriptor sub-TLV) paired with a well-formed survivor and relays only the survivor while keeping the session up, (2) a malformed BGP-LS Attribute (code 29) and relays the Link-State NLRI without code 29 (tombstone 252), and (3) a non-skipable Total NLRI Length overrun on a mixed IPv4+BGP-LS session and resets that session with NOTIFICATION 3/9 | PASS (3.1s; 80/80 under 64 CPU burners; counterfactual red proved) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-ls-receive-gobgp` | `test/interop/scenarios/bgp-ls-receive-gobgp/` | GoBGP v4.11.0 | Real GoBGP v4.11.0 receives Ze-relayed valid BGP-LS Node/Link NLRIs via `gobgp-paths 16388 71` with zero false positives and stable session (`connections-established: 1`, `connections-dropped: 0`); a skipable malformed Link-State NLRI sent beside a valid survivor is discarded by Ze and absent from GoBGP while the survivor and control remain present | PASS (6m8s against real GoBGP v4.11.0) |

## Files to Modify
- `internal/test/fixture/plugin_fixture_13.go` - register `plugin/bgp-ls-malformed-nlri` using `routeServerReplay13` (`StopsDaemon`)
- `internal/le/interoplab/bgp/names.go` - add `scenarioBGPLSReceiveGoBGP`, `gobgpAFIBGPLS` (`"16388"`), and `gobgpSAFIBGPLS` (`"71"`)
- `internal/le/interoplab/bgp/checkers.go` - register `scenarioBGPLSReceiveGoBGP` session establishment and IPv4 fence operations
- `internal/le/interoplab/bgp/check_extras.go` - register `scenarioBGPLSReceiveGoBGP` GoBGP `gobgp-paths 16388 71` presence/absence assertions and Ze peer stability check
- `docs/architecture/testing/interop.md` - document `bgp-ls-receive-gobgp` with source anchors
- `docs/functional-tests.md` - document `test/plugin/bgp-ls-malformed-nlri.ci` in the `plugin` suite coverage
- `docs/features/rfc-status.md` - refresh RFC 9552 proof counts if `./le rfc` updates functional/interop totals

## Files to Create
- `test/plugin/bgp-ls-malformed-nlri.ci` - live daemon functional test proving skipable NLRI discard, Attribute Discard, and non-skipable mixed-session NOTIFICATION 3/9 reset
- `test/interop/scenarios/bgp-ls-receive-gobgp/ze.conf` - Ze route-server configuration peering with raw injector and GoBGP over `ipv4/unicast` and `bgp-ls/bgp-ls`
- `test/interop/scenarios/bgp-ls-receive-gobgp/gobgp.toml` - GoBGP v4.11.0 neighbor configuration negotiating `ipv4-unicast` and `ls`
- `test/interop/scenarios/bgp-ls-receive-gobgp/inject.msg` - raw BGP injector sending valid Link-State NLRIs and a skipable malformed NLRI beside a well-formed survivor, synchronized by GoBGP's IPv4 fence
- `test/interop/scenarios/bgp-ls-receive-gobgp/inject-args` - raw injector CLI flags (`--asn 65004`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Consumer-facing YANG role dropped by 2026-11-24 owner decision; operators use existing `bgp/policy/family-filter` |
| YANG validation constraints | N-A | No YANG schema change |
| YANG custom validators | N-A | No YANG schema change |
| CLI commands/flags | No | No CLI command added or changed |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No config or CLI addition |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-ls-malformed-nlri.ci` proves the live receive-path enforcement end to end |
| Pipe completeness | N-A | No command output added or changed |
| Env var registration | No | No new environment variable |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary |
| Doctor code per problem class | No | Existing RFC 7606 / RFC 9552 session diagnostics and logs already cover malformed BGP-LS receive paths |
| Prometheus counters/metrics | No | No new metric added in this closure pass; existing session drop and RFC 7606 diagnostic surfaces remain intact |
| BGP family surface (new SAFI / capability / attribute) | No | `bgp-ls/bgp-ls` and `bgp-ls/bgp-ls-vpn` and code 29 are already registered |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | BGP-LS fault management is already implemented and `features/` requires no new feature id; consumer-facing role was dropped |
| 2 | Config syntax changed? | No | No config or YANG change after dropping consumer-facing role |
| 3 | CLI command added/changed? | No | No CLI verb changed |
| 4 | API/RPC added/changed? | No | No RPC changed |
| 5 | Plugin added/changed? | No | `bgp-nlri-ls` plugin unchanged |
| 6 | Has a user guide page? | No | No new user-facing knob; existing `family-filter` already documented |
| 7 | Wire format changed? | No | Wire validation is already landed and unchanged |
| 8 | Plugin SDK/protocol changed? | No | No SDK or process protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | Refresh `docs/features/rfc-status.md` if `./le rfc` updates RFC 9552 proof totals; `rfc/short/rfc9552.md` rows `8.2.2-4..10` and `8.2.6-2` are already current |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md` for `bgp-ls-malformed-nlri.ci` and `bgp-ls-receive-gobgp` |
| 11 | Affects daemon comparison? | No | BGP-LS fault management is already landed at HEAD |
| 12 | Internal architecture changed? | No | `docs/architecture/wire/nlri-bgpls.md` and `docs/architecture/wire/nlri.md` already document the landed `nlrisyntax` and `validateBGPLSAttr` architecture; `docs/architecture/testing/interop.md` is updated under row 10 |
| 13 | Route metadata keys added/changed? | No | No metadata key changed |
| 14 | Prometheus counters added/changed? | No | No counter added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No product registration changed |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | Checked via `./le spec citation anchors`: `docs/architecture/testing/interop.md` (updated), `docs/architecture/testing/ci-format.md` (unaffected: reuses existing `routeServerReplay13` / `runPlugin13` barrier unchanged), `docs/architecture/ospf/ospf-4-component-config.md`, `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md`, `docs/guide/bmp.md`, and `docs/guide/vrrp.md` (all unaffected: unrelated OSPF/BMP/VRRP scenario anchors in shared `checkers.go`/`check_extras.go`); re-run at closure |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config, CLI, or API syntax changes |

## Design presentation

### What changes

All RFC 9552 Section 8.2.2 BGP-LS receive-path validation code (`rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`, `register_nlrisyntax.go`, `session_validation_nlritype.go`), its unit tests, `rfc/short/rfc9552.md`, and `docs/architecture/wire/nlri-bgpls.md` are already landed at HEAD, and the 2026-11-24 owner decision dropped the dedicated `consumer-facing` per-peer YANG role in favor of the existing `family-filter`. This spec closes the two remaining end-to-end proof gaps: a live daemon functional test (`test/plugin/bgp-ls-malformed-nlri.ci`) and a Docker GoBGP v4.11.0 interop scenario (`bgp-ls-receive-gobgp`), plus their documentation in `docs/functional-tests.md`, `docs/architecture/testing/interop.md`, and `docs/features/rfc-status.md`.

### Before and after

1. Live daemon functional coverage:
   - Before: RFC 9552 Section 8.2.2 BGP-LS NLRI and Attribute fault management is proven only by in-process unit tests (`rfc7606_bgpls_nlri_test.go`, `rfc9552_nlri_test.go`, `rfc9552_test.go`, `rfc9552_semantic_test.go`); no `.ci` test exercises skipable NLRI discard, Attribute Discard, or non-skipable mixed-session NOTIFICATION 3/9 reset through a running `ze` daemon.
   - After: `./le test bgp plugin bgp-ls-malformed-nlri` runs `test/plugin/bgp-ls-malformed-nlri.ci` against the real `ze` binary and `le test peer`, proving skipable malformed NLRI discard with survivor relay, BGP-LS Attribute Discard with NLRI preservation, and NOTIFICATION 3/9 session reset on a mixed IPv4 + BGP-LS session.
2. Live GoBGP interop coverage:
   - Before: No interop scenario tests Ze receiving and relaying BGP-LS NLRIs to or from GoBGP v4.11.0 (`./le test integration interop` has no `bgp-ls-receive-gobgp` scenario).
   - After: `./le test integration interop` runs `bgp-ls-receive-gobgp`, proving via `gobgp-paths 16388 71` and Ze's peer detail counters that GoBGP accepts Ze-relayed valid BGP-LS NLRIs with zero false positives and zero session drops while Ze strips a skipable malformed NLRI before it reaches GoBGP.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep the live `.ci` test under `test/decode/bgp-ls-malformed-nlri.ci` | `internal/test/runner/decoding_strict_test.go` enforces that `test/decode/` accepts only single-command offline `ze bgp decode` directives and rejects `le test peer` and `ze -`. Live receive-path tests belong in `test/plugin/`. |
| Implement per-family `'AFI/SAFI disable'` for AC-7 | `rfc7606_bgpls_nlri.go` and `docs/architecture/wire/nlri-bgpls.md` already land RFC 9552 Section 8.2.2's explicit `"Alternately, the router MUST perform a 'session reset' ... if 'AFI/SAFI disable' action is not possible"` fallback; adding mid-session capability teardown is a separate feature, not a P0 closure fix. |
| Add a dedicated `consumer-facing` per-peer YANG role (`ze-bgp-ls-role.yang`, `role.go`) | Explicitly dropped by Thomas's 2026-11-24 owner decision; operators use existing `bgp/policy/family-filter` (`action remove`) and `rfc/short/rfc9552.md` already marks `RFC9552-8.2.6-2` `{not-applicable}`. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the live `.ci` fixture and interop scenario skeleton
   - Tests: `test/plugin/bgp-ls-malformed-nlri.ci`, `bgp-ls-receive-gobgp`
   - Files: `internal/test/fixture/plugin_fixture_13.go`, `internal/le/interoplab/bgp/names.go`, `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/check_extras.go`
   - Verify: fixture name `plugin/bgp-ls-malformed-nlri` is registered and `Discover` finds `bgp-ls-receive-gobgp` with a registered checker (fails until scenario files are populated).
2. **Phase: Live daemon functional test (`test/plugin/bgp-ls-malformed-nlri.ci`)** -- prove skipable NLRI discard, Attribute Discard, and non-skipable mixed-session reset through real `ze`
   - Tests: `test/plugin/bgp-ls-malformed-nlri.ci` (drafted outside `test/`, proved under load, then promoted per `/ze-test`)
   - Files: `test/plugin/bgp-ls-malformed-nlri.ci`, `internal/test/fixture/plugin_fixture_13.go`
   - Verify: receiver peer observes only the well-formed Link-State survivor NLRI (malformed TLV overrun, descending TLV order, and duplicate Node Descriptor sub-TLV discarded), observes the Link-State NLRI surviving malformed BGP-LS Attribute code 29 discard, and the offending mixed-family peer receives exact NOTIFICATION 3/9 carrying the overrunning `MP_REACH_NLRI` attribute; prove red when each check is temporarily reverted.
3. **Phase: GoBGP Docker interop scenario (`bgp-ls-receive-gobgp`)** -- prove zero false positives and live skipable-NLRI discard upstream of GoBGP v4.11.0
   - Tests: `bgp-ls-receive-gobgp` via `./le test integration interop`
   - Files: `test/interop/scenarios/bgp-ls-receive-gobgp/{ze.conf,gobgp.toml,inject.msg,inject-args}`, `internal/le/interoplab/bgp/{names.go,checkers.go,check_extras.go}`
   - Verify: GoBGP receives valid BGP-LS Node/Link NLRIs in `gobgp-paths 16388 71`, loses no session (`connections-established: 1`, `connections-dropped: 0`), and never receives the skipable malformed NLRI injected beside the survivor; prove red when the malformed-NLRI discard is reverted.
4. **Phase: Documentation and RFC ledger refresh** -- update test documentation and verify anchors and RFC gate
   - Tests: `./le doc check all`, `./le rfc check`
   - Files: `docs/architecture/testing/interop.md`, `docs/functional-tests.md`, `docs/features/rfc-status.md`
   - Verify: all doc anchors resolve and RFC 9552 checks pass cleanly.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-10 maps to landed unit proof and/or new `.ci` / GoBGP interop proof at file:line |
| Feature completeness | Both user stories run through the real `ze` daemon with no skipped or stubbed assertions |
| Correctness | Live `.ci` test verifies exact surviving NLRI wire bytes, `ATTR_TOMBSTONE` (252) on Attribute Discard, and exact NOTIFICATION 3/9 with full `MP_REACH_NLRI` Data on mixed-session reset |
| Correctness | `bgp-ls-receive-gobgp` asserts positive controls (`proof`) and `connections-established: 1`, `connections-dropped: 0` so a dropped session cannot pass vacuously |
| Naming | Scenario directory `bgp-ls-receive-gobgp` matches `scenarioBGPLSReceiveGoBGP` in `internal/le/interoplab/bgp/names.go` with no numeric prefix |
| Data flow | Tests exercise the existing `nlrisyntax` and `attrValidators` paths without adding family branches to `rfc7606.go` or `session_validation.go` |
| Operator errors and doctor codes | Existing RFC 7606 / RFC 9552 diagnostics and NOTIFICATIONs remain untouched |
| Rule: `ai/rules/interop-and-goal-validation.md` | Prove both `test/plugin/bgp-ls-malformed-nlri.ci` and `bgp-ls-receive-gobgp` fail when the behavior under test is reverted (counterfactual discrimination) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Live daemon `.ci` test exists and passes | `ls test/plugin/bgp-ls-malformed-nlri.ci` and `./le test bgp plugin bgp-ls-malformed-nlri` PASS |
| GoBGP interop scenario exists and passes | `ls test/interop/scenarios/bgp-ls-receive-gobgp/` and `./le test integration interop` with `bgp-ls-receive-gobgp` PASS |
| RFC 9552 ledger stays clean | `./le rfc check` exits 0 on `rfc/short/rfc9552.md` |
| Docs and source anchors stay clean | `./le doc check all` exits 0 |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Landed validator bounds-checks every peer-controlled length before slicing (`rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`); live `.ci` proves untrusted overrunning TLVs and Total NLRI Lengths never crash or bypass validation |
| Resource exhaustion | Zero-length TLV and 65535-octet length claims terminate in bounded time (verified by `rfc7606_bgpls_nlri_fuzz_test.go` and live `.ci`) |
| Error leakage | Non-skipable reset emits standard RFC 4760 Section 7 NOTIFICATION 3/9 with the offending MP attribute; no panic or fatal error on stderr |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read `rfc7606_bgpls_nlri.go` and `session_validation_nlritype.go`; if a real product defect surfaces, add a regression unit test and fix the producer |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- RFC 9552 Section 8.2.2 splits syntactic failures cleanly at the Total NLRI Length boundary: framing errors that prevent locating subsequent NLRI boundaries reset the session with NOTIFICATION 3/9 (since Ze has no AFI/SAFI disable), while errors inside a well-framed NLRI discard only that NLRI so surviving routes and the session remain intact.
- `test/decode/` is strictly for offline single-command `ze bgp decode` tests (`internal/test/runner/decoding_strict_test.go`); live daemon peer tests belong in `test/plugin/`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Place live malformed BGP-LS test at `test/plugin/bgp-ls-malformed-nlri.ci` | `test/decode/bgp-ls-malformed-nlri.ci` | The `decode` suite runner strictly refuses any directive other than a single `ze bgp decode` command; `test/plugin/` is the suite that runs live `le test peer` + `ze -` daemon sessions (matching `rfc7606-54-bgpls-override-propagates.ci` and `rfc7606-reset.ci`). |
| Use `gobgp-paths 16388 71` and a raw injector upstream of Ze in `bgp-ls-receive-gobgp` | Rely only on GoBGP originating valid BGP-LS routes | Combining valid Node/Link NLRIs with a skipable malformed NLRI beside a well-formed survivor proves both zero false positives against GoBGP v4.11.0 and live receive-path discard before GoBGP, with `connections-established: 1` / `connections-dropped: 0` preventing vacuous passes. |
| Keep AC-7 aligned with landed session-reset behavior | Build per-family `'AFI/SAFI disable'` | RFC 9552 Section 8.2.2 explicitly mandates session reset when `'AFI/SAFI disable'` is not possible; `rfc7606_bgpls_nlri.go` and `docs/architecture/wire/nlri-bgpls.md` already establish this contract. |
| Drop the `consumer-facing` per-peer YANG role | Add `ze-bgp-ls-role.yang` and `role.go` | Thomas's 2026-11-24 owner decision: operators already have `bgp/policy/family-filter` (`action remove`), and `rfc/short/rfc9552.md` marks `RFC9552-8.2.6-2` `{not-applicable}`. |

## Known Limitations

- Ze implements no per-family `'AFI/SAFI disable'` mechanism; non-skipable Link-State NLRI framing errors take RFC 9552 Section 8.2.2's mandatory `'session reset'` fallback on both BGP-LS-only and mixed-family sessions.
- Recognized sub-TLV lengths (§8.2.2 bullet 4) are checked for the two recognized Node Descriptor containers (TLVs 256 and 257); semantic fixed-length TLV checks are deliberately omitted per RFC 9552 Section 8.2.2.
- RFC 9552 Section 8.2.3 SHOULD-level export controls (advertisement rate limits, abstracted topologies) are outside this receiver fault-management spec.

## RFC Documentation (Scope: protocol)

The enforcing code in `internal/component/bgp/message/rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`, `rfc7606.go`, and `internal/component/bgp/reactor/session_validation_nlritype.go` already carries verbatim `// RFC 9552 Section 8.2.2` / `Section 5.1` / `Section 5.2.1.4` and `// RFC 4760 Section 7` quotes.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 9552 §8.2.2 (NLRI syntactic validation & actions) | Default BGP-LS session validates every received Link-State NLRI on ingress: discards skipable malformed NLRIs (`RetainWellFormedNLRI`), resets session with NOTIFICATION 3/9 on non-skipable framing overruns (`judgeBGPLSNLRISection`) | Unknown NLRI types propagate opaque (§5.2); unknown/semantically odd TLVs survive (§5.1, §8.2.2); ADD-PATH path IDs skipped when negotiated | `rfc9552_nlri_test.go`, `rfc7606_bgpls_nlri_test.go`, `test/plugin/bgp-ls-malformed-nlri.ci`, `bgp-ls-receive-gobgp` | `TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`, `TestRFC9552LinkStateNLRISemanticErrorsAreNotMalformed`, `TestRetainWellFormedNLRISkipsThePathIdentifier`, `test/plugin/rfc7606-54-bgpls-override-propagates.ci` |
| RFC 9552 §8.2.2 (BGP-LS Attribute syntactic validation) | Default BGP-LS session discards malformed code 29 BGP-LS Attributes with `ATTR_TOMBSTONE` (252) while preserving the Link-State NLRI | Unordered and semantically odd BGP-LS Attribute TLVs survive unchanged | `rfc9552_test.go`, `test/plugin/bgp-ls-malformed-nlri.ci` | `TestRFC9552BGPLSAttributeSemanticErrorsAreNotMalformed` |
| RFC 9552 §8.2.6 (Consumer-facing inbound drop) | Operator configures existing `bgp/policy/family-filter` with `action remove` on Consumer-facing peers (`RFC9552-8.2.6-1`); `RFC9552-8.2.6-2` is `{not-applicable}` | Existing `family-filter` (`remove` and `tear-down`) preserved unchanged | Existing `filter_family` unit and functional tests | Existing `filter_family` tests |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names code under `internal/*`, not only docs
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist answered Yes/No/N-A on every row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, `test/plugin/bgp-ls-malformed-nlri.ci`, and `bgp-ls-receive-gobgp` via `./le test integration interop`. No agent runs any `./le verify` command
- [ ] Feature and test code integrated (`internal/*`, `test/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket (none remaining after 2026-11-24 owner decision)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (already landed in `rfc7606_bgpls_nlri_test.go`)
- [ ] Functional `.ci` test (`test/plugin/bgp-ls-malformed-nlri.ci`) for end-to-end behavior
- [ ] Interop test (`bgp-ls-receive-gobgp`) for protocol behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Registered compiled observer fixture `plugin/bgp-ls-malformed-nlri` (`routeServerReplay13("shutdown-after-up", "10.0.0.0/24", false)`, `StopsDaemon`) in `internal/test/fixture/plugin_fixture_13.go` (`init`).
- Created live daemon functional test `test/plugin/bgp-ls-malformed-nlri.ci` (drafted under `test/draft/plugin/`, verified 80/80 under 64 CPU burners, counterfactual red verified by short-circuiting `RetainWellFormedNLRI`, then promoted) covering non-skipable Total NLRI Length overrun on a mixed IPv4 + BGP-LS session (NOTIFICATION 3/9 with full offending `MP_REACH_NLRI` Data), skipable malformed Link-State NLRI discard (TLV length overrun, Section 5.1 descending TLV order, Section 5.2.1.4 duplicate Node Descriptor sub-TLV) with exact byte-identical survivor relay, malformed BGP-LS Attribute code 29 Attribute Discard via `ATTR_TOMBSTONE` (code 252) while preserving the Link-State NLRI, and the `10.0.0.0/24` processing fence.
- Registered Docker GoBGP interop scenario `bgp-ls-receive-gobgp` in `internal/le/interoplab/bgp/names.go` (`scenarioBGPLSReceiveGoBGP`, `gobgpAFIBGPLS`, `gobgpSAFIBGPLS`, `bgplsReceivedNodeControl`, `bgplsReceivedLinkControl`, `bgplsReceivedNodeSurvivor`, `bgplsReceivedNodeMalformed`), `internal/le/interoplab/bgp/checkers.go` (`scenarioOperations`), and `internal/le/interoplab/bgp/check_extras.go` (`scenarioExtras`), backed by declarative scenario inputs `test/interop/scenarios/bgp-ls-receive-gobgp/{ze.conf,gobgp.toml,inject-args,inject.msg}`.
- Documented `bgp-ls-receive-gobgp` in `docs/architecture/testing/interop.md` (row 72 + source anchors) and `test/plugin/bgp-ls-malformed-nlri.ci` in `docs/functional-tests.md` (`Plugin` suite inventory + source anchor).
- Cleared all external citations to `plan/release/fixes/spec-bgp-ls-receiver-fault-management.md` across `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md`, `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md`, and `plan/release/fixes/spec-bgp-ls-receiver-accept-and-limit-controls.md` using the bare stem `spec-bgp-ls-receiver-fault-management` and durable doc pointers.

### Bugs Found/Fixed
- None in product code: the already-landed BGP-LS validators (`internal/component/bgp/message/rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`, `register_nlrisyntax.go`) and receive-path enforcement (`internal/component/bgp/reactor/session_validation.go`, `session_validation_nlritype.go`) passed both live daemon `.ci` and real GoBGP v4.11.0 interop proofs without edits.
- During Phase 3 authoring, refined `bgplsReceived*` tokens in `internal/le/interoplab/bgp/names.go` to GoBGP's exact `LsNodeNLRI.String()` / `LsLinkNLRI.String()` format so `"10.95.52.1"` cannot match `"10.95.52.11"` by substring prefix, and added the distinct trailing `10.10.0.0/24` (`injectPrefixFirst`) fence in `checkers.go` / `inject.msg` separate from GoBGP's locally originated `10.20.0.0/24` (`peerPrefixFence`) startup barrier.

### Documentation Updates
- `docs/architecture/testing/interop.md`: added row 72 (`bgp-ls-receive-gobgp`) in the BGP interop scenario table and `<!-- source: ... -->` anchors for `internal/le/interoplab/bgp/names.go`, `checkers.go`, and `check_extras.go`.
- `docs/functional-tests.md`: updated `Functional Suite Inventory` `Plugin` row documenting `test/plugin/bgp-ls-malformed-nlri.ci` beside `test/plugin/rfc7606-54-bgpls-override-propagates.ci` and added `<!-- source: internal/test/fixture/plugin_fixture_13.go -- routeServerReplay13 -->`.
- `docs/features/rfc-status.md`: unchanged (generated by `./le rfc update`; `rfc/short/rfc9552.md` rows `RFC9552-8.2.2-4..10` and `8.2.6-2` were already current with 0 stale/unproven rows under `./le rfc check`).
- `./le doc check all`: 0 errors on any file touched by this spec (`./le doc ste check` OK on both edited doc pages; the single unrelated drift item in the shared tree is another session's `show fleet alarm` wiki catalog entry).

### Deviations from Plan
- `docs/features/rfc-status.md` was listed conditionally ("if `./le rfc` updates functional/interop totals") and required no edit because `rfc/short/rfc9552.md` proof rows were already landed and verified clean by `./le rfc check`.
- Per the 2026-11-24 owner decision, no `consumer-facing` per-peer YANG role was built; operators use existing `bgp/policy/family-filter` (`action remove`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Landed Link-State NLRI & Attribute syntactic validation preserved unchanged | Done | `internal/component/bgp/message/rfc7606_bgpls_nlri.go`, `rfc7606_bgpls.go`, `register_nlrisyntax.go`, `internal/component/bgp/reactor/session_validation_nlritype.go` | Verified against source; unit tests PASS (`TestRetainWellFormedNLRI`, `TestValidateMPNLRISyntax`, `TestRFC9552`) |
| Live daemon `.ci` functional proof for skipable NLRI discard, Attribute Discard, and non-skipable mixed-session NOTIFICATION 3/9 reset | Done | `test/plugin/bgp-ls-malformed-nlri.ci`, `internal/test/fixture/plugin_fixture_13.go` | PASS (3.1s), 80/80 under 64 CPU burners, counterfactual red proved |
| GoBGP v4.11.0 Docker interop proof for zero false positives and live skipable-NLRI discard with session stability | Done | `test/interop/scenarios/bgp-ls-receive-gobgp/*`, `internal/le/interoplab/bgp/{names,checkers,check_extras}.go` | PASS (6m8s) against real GoBGP v4.11.0 (`connections-established: 1`, `connections-dropped: 0`) |
| Test documentation and source anchors updated | Done | `docs/architecture/testing/interop.md`, `docs/functional-tests.md` | `./le doc ste check` OK; `./le doc check all` 0 errors on our files |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `RetainWellFormedNLRI` / `bgplsNLRIWellFormed` (`rfc7606_bgpls_nlri.go:140,227`), `TestRetainWellFormedNLRIBoundaries`, `TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`, `test/plugin/bgp-ls-malformed-nlri.ci` (Case 1a), `bgp-ls-receive-gobgp` (Frame 3) | Exact survivor relay + counterfactual red proved |
| AC-2 | Done | `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`), `validateMPNLRISyntax` (`rfc7606.go:1101`), `TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly`, `TestRFC9552LinkStateNLRILengthOverrunResetsSession`, `test/plugin/bgp-ls-malformed-nlri.ci` (Case 3) | Exact NOTIFICATION 3/9 carrying full 25B `MP_REACH_NLRI` Data |
| AC-3 | Done | `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`), `validateMPNLRISyntax` (`rfc7606.go:1101`), `TestRFC9552LinkStateUnreachLengthOverrunResetsSession` | Withdraw-side Total NLRI Length overrun resets session with NOTIFICATION 3/9 |
| AC-4 | Done | `bgplsNLRIWellFormed` / `bgplsNodeDescriptorWellFormed` (`rfc7606_bgpls_nlri.go:265,347`), `TestRFC9552LinkStateNLRITLVOverrunIsDiscarded`, `TestRFC9552LinkStateRecognizedTLVSubTLVOverrunIsDiscarded`, `TestRFC9552LinkStateNLRIUnknownTLVOverrunIsDiscarded`, `test/plugin/bgp-ls-malformed-nlri.ci` (Case 1a), `bgp-ls-receive-gobgp` (Frame 3) | TLV and Node Descriptor 256/257 sub-TLV overrun discarded; survivor preserved |
| AC-5 | Done | `bgplsTLVOrdered` (`rfc7606_bgpls_nlri.go:307`), `TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded`, `TestRFC9552LinkStateSameTypeTLVsOutOfOrderAreDiscarded`, `TestRFC9552LinkStateSameTypeTLVsInOrderArePropagated`, `test/plugin/bgp-ls-malformed-nlri.ci` (Case 1b) | Descending TLV order (257 before 256) discarded live; survivor Node 2 relayed |
| AC-6 | Done | `bgplsNodeDescriptorWellFormed` (`rfc7606_bgpls_nlri.go:335`), `TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded`, `test/plugin/bgp-ls-malformed-nlri.ci` (Case 1c) | Duplicate sub-TLV 512 in TLV 256 discarded live; survivor Node 3 relayed |
| AC-7 | Done | `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`), `mpNLRIJudgementResult` (`rfc7606.go:1130`), `TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly`, `TestRFC9552LinkStateNLRILengthOverrunResetsSession`, `test/plugin/bgp-ls-malformed-nlri.ci` (Case 3, mixed IPv4 + BGP-LS `reset-peer`) | Mixed-family session resets with NOTIFICATION 3/9 (no AFI/SAFI disable) |
| AC-8 | Done | `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`), `TestRFC9552LinkStateNLRILengthOverrunResetsSession`, `TestRFC9552LinkStateUnreachLengthOverrunResetsSession` | BGP-LS-only session resets with NOTIFICATION 3/9 |
| AC-9 | Done | `bgplsNLRIWellFormed` (`rfc7606_bgpls_nlri.go:229`), `validateBGPLSAttr` (`rfc7606_bgpls.go:84`), `TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated`, `TestRFC9552LinkStateNLRISemanticErrorsAreNotMalformed`, `TestRFC9552BGPLSAttributeWellFormedIsKept`, `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | Unknown NLRI types and semantic oddities propagate byte-identical |
| AC-10 | Done | `validateBGPLSAttr` (`rfc7606_bgpls.go:84`), `RetainWellFormedNLRI` (`rfc7606_bgpls_nlri.go:140`), `test/plugin/bgp-ls-malformed-nlri.ci` (Case 2: `ATTR_TOMBSTONE` 252 replaces code 29 while Node 4 propagates), `bgp-ls-receive-gobgp` (GoBGP v4.11.0 receives valid Node + Link + survivor, omits malformed `10.95.52.9`, `connections-established: 1`, `connections-dropped: 0`) | Both live daemon `.ci` and Docker GoBGP interop pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRetainWellFormedNLRI*` / `TestValidateMPNLRISyntax*` | Done | `internal/component/bgp/message/rfc7606_bgpls_nlri_test.go` | PASS (landed at HEAD, re-verified by main thread) |
| `TestRFC9552*` | Done | `internal/component/bgp/reactor/rfc9552_nlri_test.go`, `rfc9552_test.go`, `rfc9552_semantic_test.go` | PASS (landed at HEAD, re-verified by main thread) |
| `bgp-ls-malformed-nlri` | Done | `test/plugin/bgp-ls-malformed-nlri.ci` | PASS (3.1s), 80/80 under load, counterfactual red proved; runner & fixture static checks PASS |
| `bgp-ls-receive-gobgp` | Done | `test/interop/scenarios/bgp-ls-receive-gobgp/`, `internal/le/interoplab/bgp/` | PASS (6m8s) against real GoBGP v4.11.0; `internal/le/interoplab/bgp` unit tests PASS (186.3s) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/fixture/plugin_fixture_13.go` | Done | Registered `plugin/bgp-ls-malformed-nlri` (`routeServerReplay13`, `StopsDaemon`) |
| `internal/le/interoplab/bgp/names.go` | Done | Added `scenarioBGPLSReceiveGoBGP`, `gobgpAFIBGPLS`, `gobgpSAFIBGPLS`, and GoBGP decoded NLRI tokens |
| `internal/le/interoplab/bgp/checkers.go` | Done | Added `scenarioOperations[scenarioBGPLSReceiveGoBGP]` with GoBGP session, startup barrier, and trailing IPv4 fence |
| `internal/le/interoplab/bgp/check_extras.go` | Done | Added `scenarioExtras[scenarioBGPLSReceiveGoBGP]` with `gobgp-paths 16388 71` presence/absence + proof and Ze peer stability counters |
| `test/plugin/bgp-ls-malformed-nlri.ci` | Done | Created and promoted live daemon functional test |
| `test/interop/scenarios/bgp-ls-receive-gobgp/ze.conf` | Done | Created Ze route-server config |
| `test/interop/scenarios/bgp-ls-receive-gobgp/gobgp.toml` | Done | Created GoBGP v4.11.0 config (`afi-safi-name = "ls"`) |
| `test/interop/scenarios/bgp-ls-receive-gobgp/inject-args` | Done | Created raw injector CLI flags (`--asn 65004`) |
| `test/interop/scenarios/bgp-ls-receive-gobgp/inject.msg` | Done | Created raw BGP-LS UPDATE injector script |
| `docs/architecture/testing/interop.md` | Done | Added row 72 + source anchors |
| `docs/functional-tests.md` | Done | Documented `test/plugin/bgp-ls-malformed-nlri.ci` + source anchor |
| `docs/features/rfc-status.md` | Changed | Conditional on RFC proof count changes; no change needed (`./le rfc check` clean on RFC 9552) |

### Audit Summary
- **Total items:** 26 (4 Task requirements, 10 ACs, 4 TDD test groups, 12 planned files, deduplicated across sections)
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`docs/features/rfc-status.md` needed no hand edit or regeneration because `rfc/short/rfc9552.md` was already current and verified clean)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Prove RFC 9552 Section 8.2.2 receive-path NLRI discard, Attribute Discard, and non-skipable NOTIFICATION 3/9 session reset through a running `ze` daemon | functional test + counterfactual red | `test/plugin/bgp-ls-malformed-nlri.ci` PASS (3.1s), 80/80 clean under 64 CPU burners (`stress-repro`); counterfactual red proved by short-circuiting `RetainWellFormedNLRI` (`internal/component/bgp/message/rfc7606_bgpls_nlri.go`) to return `section, 0, true`, causing immediate failure (`Expected UPDATE len=73 ... Received UPDATE len=94` leaking malformed NLRI `0001001102...aabbccdd`), then green on restore |
| Prove zero false positives on valid BGP-LS NLRIs and live skipable-NLRI discard upstream of real GoBGP v4.11.0 with zero session drops | Docker interop test | `INTEROP_SCENARIO=bgp-ls-receive-gobgp ./le test integration interop` PASS (6m8s) against GoBGP v4.11.0: `gobgp-paths 16388 71` returned `NODE 10.95.52.1`, `LINK 10.95.52.11->10.95.52.12`, and `NODE 10.95.52.3`, omitted malformed `10.95.52.9`, and Ze `show bgp peer 172.30.0.5 detail` reported `connections-established: 1`, `connections-dropped: 0` |
| Preserve already-landed RFC 9552 Section 8.2.2 unit coverage and RFC ledger cleanliness | unit tests + RFC gate | `TestRetainWellFormedNLRI\|TestValidateMPNLRISyntax` PASS, `TestRFC9552` PASS, `./le rfc discriminate stem rfc9552` & `./le rfc check` 0 stale/unproven rows on `RFC9552-8.2.2-4..10` and `8.2.6-2` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All in-scope deliverables completed (dedicated `consumer-facing` per-peer YANG role dropped by Thomas's 2026-11-24 owner decision in favor of existing `bgp/policy/family-filter`) | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-ls-receiver-fault-management-7c8b4e11-2a0f-4b5d-9c31-6e2f8a10d4b2.md` |
| `./le spec review check` | clean (`review_gate: OK (5 code files, clean, hashes match tmp/review/bgp-ls-receiver-fault-management-7c8b4e11-2a0f-4b5d-9c31-6e2f8a10d4b2.md)`) |
| Rounds | 1 |
| Reviewer lenses used | logic+wiring, security+edge-cases+allocations, style (`docs/contributing/ze-go-style.md` 7-question pass)+design-documents+RFC-compliance+spec-completeness (`/ze-review` + `/ze-review-spec`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE, 0 NOTE in Round 1 over all 11 spec files (`./le repo check`, `./le test weakened check`, `./le go style check`, `./le doc check all` all 0 findings on spec files; style pass clean on all 4 Go files; all 7 design pages + 1 decision row verified agreeing) | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/bgp-ls-malformed-nlri.ci` | Yes | `ls -la`: `-rw-r--r-- 1 thomas thomas 11511 Nov 27 04:38 test/plugin/bgp-ls-malformed-nlri.ci` |
| `test/interop/scenarios/bgp-ls-receive-gobgp/ze.conf` | Yes | `ls -la`: `-rw-r--r-- 1 thomas thomas 1463 Nov 27 04:12 ze.conf` |
| `test/interop/scenarios/bgp-ls-receive-gobgp/gobgp.toml` | Yes | `ls -la`: `-rw-r--r-- 1 thomas thomas 509 Nov 27 04:12 gobgp.toml` |
| `test/interop/scenarios/bgp-ls-receive-gobgp/inject-args` | Yes | `ls -la`: `-rw-r--r-- 1 thomas thomas 12 Nov 27 04:13 inject-args` |
| `test/interop/scenarios/bgp-ls-receive-gobgp/inject.msg` | Yes | `ls -la`: `-rw-r--r-- 1 thomas thomas 4419 Nov 27 04:47 inject.msg` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Skipable TLV sum mismatch discarded; survivor relayed byte-identical | Verified in `RetainWellFormedNLRI` / `bgplsNLRIWellFormed` (`rfc7606_bgpls_nlri.go:140,227`); `TestRetainWellFormedNLRIBoundaries` PASS; `test/plugin/bgp-ls-malformed-nlri.ci` Case 1a (`conn=2 seq=1` -> `conn=3 seq=1` exact 73B) PASS + counterfactual red proved |
| AC-2 | `MP_REACH_NLRI` Total NLRI Length overrun resets session with NOTIFICATION 3/9 | Verified in `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`) + `mpNLRIJudgementResult` (`rfc7606.go:1130`); `TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly` & `TestRFC9552LinkStateNLRILengthOverrunResetsSession` PASS; `test/plugin/bgp-ls-malformed-nlri.ci` Case 3 (`conn=1 seq=2` exact 46B NOTIFICATION 3/9) PASS |
| AC-3 | `MP_UNREACH_NLRI` Total NLRI Length overrun resets session with NOTIFICATION 3/9 | Verified in `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`) + `validateMPNLRISyntax` (`rfc7606.go:1101`); `TestRFC9552LinkStateUnreachLengthOverrunResetsSession` PASS |
| AC-4 | TLV / Node Descriptor 256/257 sub-TLV overrun discarded; survivor relayed | Verified in `bgplsNLRIWellFormed` / `bgplsNodeDescriptorWellFormed` (`rfc7606_bgpls_nlri.go:265,347`); `TestRFC9552LinkStateRecognizedTLVSubTLVOverrunIsDiscarded` PASS; `test/plugin/bgp-ls-malformed-nlri.ci` Case 1a & `bgp-ls-receive-gobgp` Frame 3 PASS |
| AC-5 | RFC 9552 Section 5.1 TLV ordering violation discarded; survivor relayed | Verified in `bgplsTLVOrdered` (`rfc7606_bgpls_nlri.go:307`); `TestRFC9552LinkStateNLRIOutOfOrderTLVsAreDiscarded` PASS; `test/plugin/bgp-ls-malformed-nlri.ci` Case 1b (`conn=2 seq=2` -> `conn=3 seq=2` exact 73B) PASS |
| AC-6 | RFC 9552 Section 5.2.1.4 duplicate/descending Node Descriptor sub-TLV discarded; survivor relayed | Verified in `bgplsNodeDescriptorWellFormed` (`rfc7606_bgpls_nlri.go:335`); `TestRFC9552LinkStateNodeDescriptorDuplicateSubTLVIsDiscarded` PASS; `test/plugin/bgp-ls-malformed-nlri.ci` Case 1c (`conn=2 seq=3` -> `conn=3 seq=3` exact 73B) PASS |
| AC-7 | Non-skipable Total NLRI Length overrun on mixed IPv4 + BGP-LS session sends NOTIFICATION 3/9 | Verified in `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`); `test/plugin/bgp-ls-malformed-nlri.ci` Case 3 (`reset-peer` negotiates `ipv4/unicast` + `bgp-ls/bgp-ls`, receives exact NOTIFICATION 3/9) PASS |
| AC-8 | Non-skipable Total NLRI Length overrun on BGP-LS-only session sends NOTIFICATION 3/9 | Verified in `judgeBGPLSNLRISection` (`rfc7606_bgpls_nlri.go:378`); `TestRFC9552LinkStateNLRILengthOverrunResetsSession` PASS |
| AC-9 | Semantic oddities and unknown NLRI types propagate byte-identical | Verified in `bgplsNLRIWellFormed` (`rfc7606_bgpls_nlri.go:229`) and `validateBGPLSAttr` (`rfc7606_bgpls.go:84`); `TestRFC9552LinkStateNLRISemanticErrorsAreNotMalformed` & `TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated` PASS |
| AC-10 | Valid Node/Link NLRIs reach GoBGP with 0 drops; malformed code 29 replaced by `ATTR_TOMBSTONE` 252 while NLRI propagates; malformed NLRI omitted at GoBGP | `test/plugin/bgp-ls-malformed-nlri.ci` Case 2 (`conn=3 seq=4` exact 88B with `80FC0C1D02...`) PASS; `bgp-ls-receive-gobgp` PASS (6m8s: `NODE 10.95.52.1`, `LINK 10.95.52.11->10.95.52.12`, `NODE 10.95.52.3` present; `10.95.52.9` absent; `connections-established: 1`, `connections-dropped: 0`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Live BGP peers sending skipable malformed Link-State NLRIs, malformed BGP-LS Attribute code 29, and non-skipable Total NLRI Length overrun to running `ze` daemon | `test/plugin/bgp-ls-malformed-nlri.ci` | Yes (read all 250 lines; invokes `le test fixture plugin/bgp-ls-malformed-nlri` registered in `internal/test/fixture/plugin_fixture_13.go:18`; PASS 3.1s, 80/80 stress-repro, counterfactual red proved) |
| Upstream raw injector + `ze` route server + real GoBGP v4.11.0 exchanging valid and skipable-malformed BGP-LS NLRIs | `test/interop/scenarios/bgp-ls-receive-gobgp/` (`internal/le/interoplab/bgp/{names,checkers,check_extras}.go`) | Yes (read all 4 declarative scenario files and checker registrations; `TestCheckerPopulationMatchesProducer` PASS; Docker interop PASS 6m8s) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `INTEROP_SCENARIO=bgp-ls-receive-gobgp ./le test integration interop` PASS (6m8s) against GoBGP v4.11.0: `afi-safi-name = "ls"` negotiated and `gobgp-paths 16388 71` decoded `NODE 10.95.52.1`, `LINK 10.95.52.11->10.95.52.12`, and `NODE 10.95.52.3` while omitting malformed `10.95.52.9` |
| A-2 | confirmed | Zero product code changes required under `internal/component/bgp/`; both `test/plugin/bgp-ls-malformed-nlri.ci` (PASS + counterfactual red) and `bgp-ls-receive-gobgp` (PASS 6m8s) succeeded against landed HEAD code |
| A-3 | confirmed | `test/plugin/bgp-ls-malformed-nlri.ci` passed `TestCIFrameLengthsWellFormed`, `TestCIFrameStructureWellFormed`, and `TestPluginObserversWaitForTheScriptedPeer` in `test/plugin/` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Test infrastructure: `docs/architecture/testing/interop.md` row 72 (`bgp-ls-receive-gobgp`) | Matches `names.go` (`scenarioBGPLSReceiveGoBGP`, `gobgpAFIBGPLS`, `gobgpSAFIBGPLS`, `bgplsReceived*`), `checkers.go` (`scenarioOperations`), `check_extras.go` (`scenarioExtras`), and `test/interop/scenarios/bgp-ls-receive-gobgp/*`; `./le doc ste check` OK | Yes |
| Test infrastructure: `docs/functional-tests.md` `Plugin` suite row (`test/plugin/bgp-ls-malformed-nlri.ci`) | Matches `test/plugin/bgp-ls-malformed-nlri.ci` and `internal/test/fixture/plugin_fixture_13.go` (`routeServerReplay13`); `./le doc ste check` OK | Yes |
| RFC compliance: `rfc/short/rfc9552.md` and `docs/features/rfc-status.md` | `./le rfc discriminate stem rfc9552` and `./le rfc check` reported 0 stale/unproven rows on `RFC9552-8.2.2-4..10` and `8.2.6-2`; no hand edit to generated `docs/features/rfc-status.md` | Yes |
| Existing doc source anchors (`grep -rn "source:.*(plugin_fixture_13|interoplab/bgp/(names|checkers|check_extras))" docs/`) | All 21 anchor hits checked: `docs/architecture/testing/interop.md` and `docs/functional-tests.md` updated; `ci-format.md` (`runPlugin13`), `ospf-4-component-config.md`, `ospf-ext-16-ipsec-auth.md`, `bmp.md`, and `vrrp.md` unaffected; `./le doc check all` 0 errors on our files | Yes |
| Other categories (feature list, config/YANG, CLI, API/RPC, plugin SDK, wire format, metrics, comparison table) | Confirmed No: diff touches only test fixtures, interop lab checkers, `.ci` / interop scenario files, and test docs (`git diff --name-only` verified) | Yes |
