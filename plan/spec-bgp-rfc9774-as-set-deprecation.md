# Spec: bgp-rfc9774-as-set-deprecation

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 |
| Handoff | verify |
| Updated | 2026-10-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement RFC 9774, "Deprecation of AS_SET and AS_CONFED_SET in BGP", in Ze, and
enrol the document (`rfc/short/rfc9774.md` through `/ze-rfc`).

Owner decision, 2026-10-18, verbatim: "Implement RFC 9774". This lifts the
2026-10-08 RFC enrolment freeze for RFC 9774 ONLY. No other RFC is unfrozen by it.

RFC 9774 Section 3 (read in `rfc/full/rfc9774.txt`), verbatim:

> Unless explicitly configured by a network operator to do otherwise (e.g., during a
> transition phase), BGP speakers:
> - MUST NOT advertise BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs and
> - MUST use the "treat-as-withdraw" error handling behavior per [RFC7606] upon
>   reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs in the
>   AS_PATH or AS4_PATH [RFC6793].

Today Ze accepts both segment types on receive and can transmit them. The goal is
that, by default, no route whose AS_PATH or AS4_PATH carries an AS_SET (type 1) or
an AS_CONFED_SET (type 4) is installed from a peer or advertised to one, whatever
produced it.

Section 5.2 ("the implementation MUST be configured to truncate the AS_PATH after the
right-most instance of the desired origin AS") and Section 6.1 bind an implementation
of brief aggregation. Ze has no aggregation feature (research below), so those rows are
`feature-out-of-scope` in the summary, never counted as met.

## Open Questions for the Owner

Each is "which way". The spec is written under the recommended answer; the delta for
the other answer is stated so the implementer does not have to guess.

Answered 2026-10-18. Thomas, verbatim: "For the rest it looks good to me", accepting
the recommendations put to him, with Q-4 answered (b) as recorded below.

| Q | Owner answer (2026-10-18) |
|---|---------------------------|
| Q-1 | (a) no knob: the rule is applied unconditionally |
| Q-2 | (a) treat-as-withdraw on every session, including AS4_PATH between two 4-octet speakers |
| Q-3 | (a) the writer guard only |
| Q-4 | (b) raw injection (`send bgp ... raw`) is exempt. This departs from the 2026-10-14 Q-3 (a) ruling that applied normalization to raw writes, because raw send is a test instrument |
| Q-5 | the spec stays in `plan/` |

| # | Question | Options | Recommendation | Delta if the other option is chosen |
|---|----------|---------|----------------|-------------------------------------|
| Q-1 | Does Ze expose the Section 3 operator override ("unless explicitly configured ... e.g., during a transition phase")? | (a) no knob: the behavior is unconditional; (b) a per-peer leaf under `session` (for example `as-set accept`) that disables BOTH the receive treat-as-withdraw and the send guard for that peer | (a). The RFC permits a knob and does not require one. Ze is pre-release, has no operator in a transition phase, and `docs/guide/bgp-policy.md` already sets the precedent for RFC 1997: "no configuration switch, because a switch here would be a switch to violate the RFC". A knob also forces the receive check out of the stateless message validator into session state | (b) moves the receive scan from `message/rfc7606.go` into `Session.validateRFC7606Attrs` (session_validation_receive.go, the AIGP-disabled precedent), adds the YANG leaf, its config reader, a `.ci` per direction for the knob, `docs/guide/configuration.md`, and a `{feature-out-of-scope}` row turns into a counted conditional row |
| Q-2 | AS_SET in AS4_PATH on a session where both ends are NEW (4-octet) speakers. RFC 6793 Section 4.1 has a NEW speaker ignore such an AS4_PATH; RFC 9774 Section 3 names AS4_PATH without that condition | (a) treat-as-withdraw on every session (literal Section 3); (b) treat-as-withdraw only where Ze consumes AS4_PATH (2-octet session), elsewhere keep today's discard | (a). Section 3 is unconditional and later than RFC 6793; the AS4_PATH is still in the UPDATE Ze received | (b) adds the `asn4` argument to the AS4_PATH set scan and one negative test on a 4-octet session |
| Q-3 | Where does the send-side MUST NOT get an operator-facing refusal, beyond the writer guard that enforces it? | (a) writer guard only: the route is converted to a withdrawal at write time, logged at warning level (rate-limited) and counted; (b) also refuse at config commit a static route whose raw `attribute` leaf carries an AS_PATH (code 2) or AS4_PATH (code 17) with a set segment, and refuse `update hex/b64 attr set` with such bytes at the command | (a). It is the one declaration that covers every producer (config, API, plugin raw wire, relay), so no central list of sources is written. (b) improves the message the operator sees but is a second, partial declaration | (b) adds a config validator under `internal/component/bgp/config/`, a refusal in `plugins/cmd/update/update_wire.go`, a `.ci` for each refusal, and `docs/guide/configuration.md` plus `docs/architecture/api/update-syntax.md` rows |
| Q-4 | Does the guard apply to the `send bgp ... raw` injection command (`ze-raw-cmd`, "testing only")? | (a) yes, as the 2026-10-14 owner decision Q-3 (a) applied RFC 4271/8092/8669 normalization to raw writes; (b) no, a raw injection is the "explicitly configured by a network operator" case | (b). The raw command exists to put operator-chosen bytes on the wire, which is the override Section 3 names, and it is Ze's only way to send an AS_SET to another implementation for testing | (a) moves the guard ahead of the `raw` branch in `writeUpdateBody` and adds one test on the raw path |
| Q-5 | Bucket | `plan/` (release ships without it) or `plan/pre-release/` | `plan/`: no operator meets the absence as a bug | move the file |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` (section "2. AS_PATH (Code 2)", segment type table at line 245)
  → Constraint: the segment table lists AS_SET and AS_CONFED_SET as accepted types; the implementation phase edits that section to state the RFC 9774 receive and send behavior, in the same change as the code.
- [ ] `docs/architecture/testing/interop.md` ("Scenario Structure", "Optional sidecars", "Prove a scenario discriminates")
  → Constraint: a scenario directory is named, never numbered; `inject.msg` (+ optional `inject-args`) drives Ze from the raw injector at 172.30.0.9; the pinned FRR image is `quay.io/frrouting/frr:10.3.1` (`internal/le/interoplab/bgp/run.go` defaultFRRImage), NOT 10.5.
  → Constraint: discrimination is proven by reverting the fix, rebuilding the Ze image, observing red, restoring, observing green, and recording the red.
- [ ] `docs/architecture/core-design.md` (declared by `session_validation_receive.go` and `forward_wellknown.go`: receive UPDATE coalescing, egress suppression on the forward rails)
  → Constraint: the coalescer batches only a verdict `batchable()` accepts; the set verdict MUST be reached before batching, which putting it in the shared validator guarantees. The page needs no edit unless the implementation changes the coalescing or forward-rail description; the implementer confirms that at Documentation row 12.
- [ ] `docs/guide/rpki.md` line 552
  → Constraint: the page said "AS_SET handling follows the receive path's RFC 9774 policy" before any such policy existed. Corrected in a separate commit during this design phase; the implementation phase rewrites it to the implemented behavior.
- [ ] `docs/guide/bgp-policy.md` line 153 (RFC 1997 well-known egress)
  → Decision: precedent for an RFC egress prohibition with no configuration switch and a counter as its only report. Informs Q-1 (a).

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc9774.txt` - the document itself (no `rfc/short/rfc9774.md` yet; enrolment is AC-14)
  → Constraint: Section 3 binds receive (treat-as-withdraw, AS_PATH and AS4_PATH) and send (MUST NOT advertise). Section 4 is descriptive (ROV never Valid, ASPA Invalid). Sections 5.2 and 6.1 bind aggregation, which Ze does not implement. Section 6.2 and 6.3 are operator advice.
- [ ] `rfc/short/rfc7606.md` - treat-as-withdraw procedure
  → Constraint: RFC 7606 Section 3(h): "the approach with the strongest action MUST be used", so a set in AS4_PATH (treat-as-withdraw) outranks the AS 0 attribute discard on the same attribute.
- [ ] `rfc/short/rfc6793.md` - AS4_PATH
  → Constraint: RFC 6793 Section 6 chooses attribute discard for a malformed AS4_PATH; RFC 9774 Section 3 overrides that for a set segment only. RFC6793-3-1 already forbids AS_CONFED_* in AS4_PATH on send.
- [ ] `rfc/short/rfc7607.md` - AS 0, the structural twin of this work
  → Decision: copy its shape. AS_PATH check sits in `validateASPath` after the structure walk; AS4_PATH check in `validateAS4PathAttr`; a shared scan helper in its own RFC file so the two validators cannot disagree; proven over `Session.enforceRFC7606` in a reactor test.
- [ ] `rfc/short/rfc4271.md` rows RFC4271-5.1.2-3, RFC4271-5.1.6-2, RFC4271-9.2-11, RFC4271-9.2.2.2-6
  → Constraint: RFC 9774 updates RFC 4271. The AS_SET branches of these rows become unreachable on Ze's default wire path; their unit-level proofs stay, and the summary records that RFC 9774 updates them. No row is deleted.
- [ ] `rfc/short/rfc5065.md`
  → Constraint: Ze configures no confederation (`docs/features/rfc-status.md` RFC 1997 row: "Ze configures no confederation"), so AS_CONFED_SET only ever arrives from a peer or a raw byte string; it is handled exactly like AS_SET.

**Key insights:**
- Receive: one stateless scan for segment types 1 and 4, added to the two existing attribute validators, lands on `RFC7606ActionTreatAsWithdraw`; the existing treat-as-withdraw rail (synthesized withdrawals, Section 6 log, diagnostics) does the rest.
- Send: one guard in `Session.writeUpdateBody`, sibling of the RFC 7311 Section 3.3 AIGP guard, writes the UPDATE's withdrawal form instead of the UPDATE. Every producer funnels through that writer.
- Ze originates no AS_SET of its own: the text and config AS_PATH parsers build only AS_SEQUENCE, and Ze has no aggregation. Only raw bytes (config `attribute` leaf, `update hex/b64`, plugin raw wire, `send bgp raw`) or a relayed received path can carry one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/aspath.go` - defines `ASSet`=1, `ASSequence`=2, `ASConfedSequence`=3, `ASConfedSet`=4; `ParseASPath` accepts types 1..4 (line 320).
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateASPath` (line 1072): structure walk (type 1..4 accepted, zero length, overrun, underrun) then the RFC 7607 AS 0 scan via `asPathHoldsASZero`; every fault is `RFC7606ActionTreatAsWithdraw` with `attrCodeASPath`. Registered in `attrValidators` by `init()` (line 655). Validator signature carries only (code, length, data, isIBGP, asn4): no session or config state.
- [ ] `internal/component/bgp/message/rfc7607.go` - `validateAS4PathAttr` (line 44) checks AS 0 only and answers `RFC7606ActionAttributeDiscard`; its comment states it deliberately does not judge AS4_PATH structure (RFC 6793 Section 6 gap). `asPathHoldsASZero` (line 93) is the shared scan pattern.
- [ ] `internal/component/bgp/reactor/session_validation_receive.go` - `classifyRFC7606` -> `validateRFC7606Attrs` -> `message.ValidateUpdateRFC7606AddPath`, then session policies (AIGP disabled, line 175). `batchable()` excludes any non-None verdict from the coalescer.
- [ ] `internal/component/bgp/reactor/session_validation.go` - `applyRFC7606` treat-as-withdraw branch logs (`logRFC7606Error`), records diagnostics and returns; `firstASMismatch`/`firstASPathMismatch` treat a leading set as a mismatch on eBGP.
- [ ] `internal/component/bgp/reactor/session_read.go` (lines 235-300) - on treat-as-withdraw, `message.SynthesizeWithdrawFamilies` turns the body into withdrawal bodies and dispatches them, so a previously-announced prefix is removed.
- [ ] `internal/component/bgp/reactor/session_write.go` `writeUpdateBody` (line 605) - every UPDATE body funnels here (`raw` flag for `send bgp raw`). First statement after the write-failed check is the RFC 7311 Section 3.3 guard: `!AIGPEnabled() && aigpPresent(body)` -> `writeUpdateWithoutAIGP`. Then adj-out ownership (`beginAdjOut`), `normalizeTransmittedUpdate`, and the ownership filter that suppresses what the peer already holds.
- [ ] `internal/component/bgp/reactor/session_aigp.go` - `aigpPresent` (header walk via `wireu.ParseUpdateSections` + `attribute.AttrFind`, no allocation) and `writeUpdateWithoutAIGP` (pool read buffer via `getReadBuf`, then re-enters the writer). This is the template for the send guard.
- [ ] `internal/component/bgp/message/rfc7606_withdraw.go` `SynthesizeWithdrawFamilies(body, accept, addPath)` - builds withdraw-only bodies from an announcing body; IPv4 NLRI move to Withdrawn Routes, MP_REACH becomes MP_UNREACH with the same NLRI bytes, one MP family per body.
- [ ] `internal/core/bgp/attribute/text.go` `ParseASPathText` and `internal/component/bgp/config/routeattr.go` `ParseASPath` - both return a flat `[]uint32`, encoded as one AS_SEQUENCE; no text syntax for a set exists.
- [ ] `internal/component/bgp/config/bgp_routes.go` line 316 and `config/peers.go` line 503 - the `attribute` leaf-list becomes `RawAttributes` with any code, so a configured static route CAN carry a raw AS_PATH with a set.
- [ ] `internal/component/bgp/plugins/cmd/update/update_wire.go` - `update hex|b64 attr set <bytes>` takes the attribute block verbatim.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` line 334 - API batches keep a caller's AS_PATH segments intact, AS_SET included.
- [ ] `internal/component/bgp/reactor/filter_delta.go` line 795 - egress prepend preserves each segment's type.
- [ ] `internal/component/bgp/plugins/rpki/aspa_verify.go` line 40/146, `plugins/rpki/validate.go` line 44, `plugins/rib/bestpath.go` lines 637/698 - AS_SET handling in ASPA, ROV and best path.

**Behavior to preserve:**
- Every existing RFC 7606 Section 7.2 description string and its precedence: a structurally malformed AS_PATH reports the structural fault, not the set.
- AS 0 handling (RFC 7607): AS_PATH treat-as-withdraw, AS4_PATH attribute discard when no set is present.
- ASPA, ROV and best-path AS_SET code stays as is: RFC 9774 Section 4 restates their behavior, and the Q-1 (b) knob or a future change could make them reachable again.
- Decoders and renderers (`ze bgp decode`, MRT, `[{1 2}]` debug rendering, ExaBGP bridge JSON `as-set`) still render a set: they show bytes, they advertise nothing.
- The `send bgp raw` path under Q-4 (b).

**Behavior to change:**
- Receive: an UPDATE whose AS_PATH or AS4_PATH holds a type 1 or type 4 segment becomes treat-as-withdraw.
- Send: an UPDATE whose AS_PATH or AS4_PATH holds a type 1 or type 4 segment is written as its withdrawal.
- `docs/guide/rpki.md` line 552 states the implemented behavior.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Receive: UPDATE wire bytes on a peer session (`session_read.go` processMessage).
- Send: an UPDATE body handed to `Session.writeUpdateBody` by any producer: relay (`reactor_api_forward.go`, `forward_rs.go`), API/plugin batch (`reactor_api_batch.go`), static routes, RIB replay.

### Transformation Path
1. Receive: `classifyRFC7606` -> `ValidateUpdateRFC7606AddPath` -> `validateAttribute` -> `validateASPath` (code 2) / `validateAS4PathAttr` (code 17) -> new set scan -> `RFC7606ActionTreatAsWithdraw`.
2. Receive: `applyRFC7606` treat-as-withdraw branch logs and records diagnostics; `session_read.go` synthesizes withdrawals and dispatches them; nothing is installed, the previous route is removed.
3. Send: `writeUpdateBody` -> new set check over the body's AS_PATH and AS4_PATH -> `SynthesizeWithdrawFamilies` with the session's send-side family and ADD-PATH predicates -> each withdrawal body re-enters `writeUpdateBody` (ownership filter suppresses a withdrawal of something the peer never held) -> counter and rate-limited warning.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> engine | RFC 7606 classification on the receive goroutine | No |
| Engine -> plugin | treat-as-withdraw dispatches synthesized withdrawals, so plugins see withdrawals only | No |
| Engine -> wire | `writeUpdateBody`, the one writer | No |

### Integration Points
- `attrValidators` registry (`message/rfc7606.go` init and `rfc7607.go` init) - the scan is added to the two validators already registered for codes 2 and 17; no new registration.
- `writeUpdateBody` - one guard beside the AIGP guard.
- `reactor_metrics.go` - one counter beside `ze_bgp_wellknown_community_suppressed_total`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | receive uses the RFC 7606 rail; send uses the one writer |
| No unintended coupling (components stay isolated) | Yes | message package stays stateless; reactor owns the send guard |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses treat-as-withdraw dispatch and `SynthesizeWithdrawFamilies` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | scans read the received slice; the send rewrite uses a pool buffer only on the AS_SET path |
| Registration over hardcoding, outbound | Yes | no new command, view or family |
| Registration over hardcoding, inbound | Yes | no list learns a new name; segment type constants already exist in `attribute` and `message` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze never originates an AS_SET itself | text/config AS_PATH parsers return flat `[]uint32`; no aggregation code under `internal/component/bgp` or `internal/plugins` (grep "aggregat" outside AGGREGATOR/ATOMIC) | send guard would also fire on Ze's own routes, which is still correct but changes the story | grep in phase 1, recorded in the summary | confirmed 2026-10-18: `git grep -i aggregat` over both trees, AGGREGATOR/ATOMIC excluded, finds pool/log wording and one RFC 4271 Section 5.1.7 quote in `update_text.go`, no route aggregation. But a configured raw `attribute "0x02 ..."` is written BESIDE the typed AS_PATH (two AS_PATH attributes, journal `wire-carries-both-arms-of-an-either-or.md`), so the guard judges every occurrence (`transmittedPathHoldsSet`) |
| A-2 | Every transmitted UPDATE, `raw` included, passes through `writeUpdateBody` | its comment and the AIGP guard placement; `writeRawUpdateBody` calls it | a producer bypassing it would advertise a set | `gopls call_hierarchy` on `writeUpdateBody` and grep for other `bufWriter` writers of type UPDATE | confirmed 2026-10-18: callers are `writeRawUpdateBody` (relay, `sendRawUpdateBody`), `writeOriginatedRawUpdate` (`writeMessageWithin` for every parsed UPDATE, `SendRawMessage` with raw=true) and the AIGP re-entry; the other `bufWriter.Write` sites carry non-UPDATE packets only (`SendRawMessage` full-packet branch routes UPDATE to `writeOriginatedRawUpdate`) |
| A-3 | A withdrawal body re-entering `writeUpdateBody` keeps Adj-RIB-Out ownership consistent, and a withdrawal of a prefix the peer never held is suppressed | `ownership.filter` / "announce suppressed: peer holds the final wire advertisement" in `writeUpdateBody` | spurious withdrawals on the wire, or `show` adj-rib-out listing a route the peer never got | unit test AC-9 (no UPDATE for a never-held prefix) and AC-10 (withdrawal for a held one) | confirmed 2026-10-18: `TestRFC9774SendASSetWrittenAsWithdrawal` never-held (no announcement on the wire) and held-then-replaced (exactly the clean announcement then one withdrawal). Partly broken 2026-10-18 on the config static-route rail: `test/encode/rfc9774-as-set-not-advertised.ci` shows the withdrawal of the never-held 10.0.1.0/24 IS written (that rail is not ownership-tracked). It announces nothing, so AC-9 ("No UPDATE announcing the prefix") holds; the .ci pins the withdrawal |
| A-4 | `SynthesizeWithdrawFamilies` can be driven with send-side predicates | its signature takes `accept` and `addPath` functions; receive passes `mpFamilyDispatchable`, `mpAddPathReceiveWire` | a send-side ADD-PATH body would be mis-framed | test with ADD-PATH negotiated for send | confirmed 2026-10-18: `TestRFC9774SendAddPathFraming` (IPv4, IPv6 unicast, IPv4 labeled unicast, path id 7). The predicate only matters for labeled NLRI (RFC 8277 Section 2.4 label rewrite); unicast is copied byte for byte. `mpAddPathSend` now reads the send encoding context (`bgpctx.Registry.Get(s.sendCtxID).AddPath`), the declaration the splitter and ownership filter use, not `s.negotiated`. Red: `mpAddPathSend` forced false turns the labeled case red only |
| A-5 | FRR 10.3.1 has `bgp reject-as-sets` and accepts an AS_SET when it is off | FRR command reference (not verified in this checkout) | scenario cannot use FRR as an accepting judge; fall back to GoBGP or BIRD | first scenario run; record FRR's `show bgp` with the knob off | confirmed 2026-10-19: both-off red walk run (receive scan and send guard overlaid off), FRR 10.3.1 with `no bgp reject-as-sets` installed `"10.97.74.0/24": [{"valid":true,"bestpath":true,...,"path":"65001 65004 {65005,65006}"...}]` from Ze's relay |
| A-6 | The caller's note "FRR 10.5 rejects them by default" does not bind the pinned 10.3.1 image | `run.go` pins 10.3.1 | none, the scenario sets the knob explicitly either way | scenario config states the knob | confirmed 2026-10-19: `test/interop/scenarios/bgp-rfc9774-as-set-frr/frr.conf` line 18 sets `no bgp reject-as-sets` explicitly, the lab image is `quay.io/frrouting/frr:10.3.1` (`defaultFRRImage`), and the both-off run (A-5) shows that image accepting a set with the knob off, so no default of any FRR release decides the verdict |
| A-7 | Scanning a set in AS4_PATH is safe without a full RFC 6793 Section 6 structure check | `validateAS4PathAttr` comment; `asPathHoldsASZero` bounded walk | a misframed AS4_PATH could be misread | scan judges only a fully framed AS4_PATH; a misframed one is left as today (unit test) | confirmed 2026-10-18: `TestValidateAS4PathRejectsSetSegments` misframed case draws no verdict |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Existing unit tests that drive an AS_SET through the receive or send rail go red (31 test files reference AS_SET; reactor ones: `filter_delta_test.go`, `forward_withhold_mixed_test.go`, `forward_policy_section_test.go`, `reactor_api_batch_attr_preserve_test.go`, `filter/loop_test.go`; `message/rfc7606_test.go`) | scoped package tests red after phase 2/3 | read each: a test of a pure function (prepend, loop detection, best path) is unaffected; a rail test asserting an AS_SET is accepted or sent is now asserting the RFC-violating behavior and is corrected to the new verdict, never deleted; a test that needs a set on the wire to reach another feature switches to the `raw` path (Q-4 b) |
| R-2 | RFC-tagged tests for RFC 4271 AS_SET rows (5.1.2-3, 9.1.2.2) depend on AS_SET reaching the wire | `./le rfc check` red | keep the unit-level proof; record "updated by RFC 9774" in the row's summary text; never weaken the tag |
| R-3 | `session_write.go` and `session_read.go` carry another session's uncommitted hunks (git status at design time) | `git status` dirty on those files at implementation start | add the guard in a new file `session_rfc9774.go` and a one-call insertion in `writeUpdateBody`; judge the foreign hunk against HEAD before committing (`ai/rules/git-safety.md`) |
| R-4 | Send guard hides a route silently from an operator who configured it | operator sees the prefix missing at the peer | warning line naming peer, prefix count, cause and fix (`ai/rules/user-facing-errors.md`), rate-limited like `wellKnownScanLogInterval`; counter |
| R-5 | Per-write cost: one more header walk on every UPDATE | `./le perf` delta on the forward path | fold the AS_PATH/AS4_PATH lookup into one `AttrFind`-style walk; no allocation on the clean path; benchmark in the existing `rfc7606_bench_test.go` style |
| R-6 | A relayed received UPDATE never carries a set after phase 2, so the send guard looks dead in relay tests | relay test passes with guard reverted | discriminate the send guard with config/API raw bytes (AC-9, AC-11), not with relay |
| R-7 | The coalescer batches clean UPDATEs before the set verdict | a batched UPDATE with a set reaches the RIB | the verdict lands in the shared validator, and `batchable()` refuses any non-None action; test with two UPDATEs, second carrying a set |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes with AS_SET from real peers are withdrawn (intended); a false positive on a clean path would withdraw legitimate routes |
| How is it reverted? | single commit revert |
| Who else touches this path? | RFC 7606 receive work, the in-flight reactor session work in `session_write.go`/`session_read.go`, RPKI ASPA |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE wire on a session | → | `validateASPath` set scan via `Session.enforceRFC7606` | `TestRFC9774ReceiveASSetTreatAsWithdraw` in `internal/component/bgp/reactor/rfc9774_receive_test.go` |
| UPDATE wire with AS4_PATH on a 2-octet session | → | `validateAS4PathAttr` set scan via `Session.enforceRFC7606` | `TestRFC9774ReceiveAS4PathSetTreatAsWithdraw` in the same file |
| Body handed to the writer | → | `writeUpdateBody` -> set guard -> withdrawal | `TestRFC9774SendASSetWrittenAsWithdrawal` in `internal/component/bgp/reactor/rfc9774_send_test.go` |
| Config static route with raw AS_PATH | → | writer guard | `test/encode/rfc9774-as-set-not-advertised.ci` |
| Peer sends AS_SET route | → | treat-as-withdraw, RIB empty | `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer sends an UPDATE whose AS_PATH holds an AS_SET segment, on a 4-octet session and on a 2-octet session | Treat-as-withdraw: no route installed, the session stays Established, the error log line names "RFC 9774 Section 3" and AS_PATH (code 2) |
| AC-2 | Same with an AS_CONFED_SET segment in AS_PATH, on eBGP and iBGP | Same as AC-1 |
| AC-3 | 2-octet session: AS_PATH clean, AS4_PATH holds an AS_SET or AS_CONFED_SET | Treat-as-withdraw (not the RFC 6793 attribute discard) |
| AC-4 | 4-octet session: AS4_PATH holds an AS_SET | Per Q-2 (a): treat-as-withdraw |
| AC-5 | Prefix P accepted with a clean path, then re-announced with an AS_SET | P is removed from the Adj-RIB-In and the withdrawal is relayed to peers that held it |
| AC-6 | IPv6 unicast (MP_REACH) UPDATE with an AS_SET | MP_UNREACH withdrawal synthesized for the same NLRI |
| AC-7 | Paths holding only AS_SEQUENCE and AS_CONFED_SEQUENCE segments | Accepted unchanged (negative of AC-1/AC-2) |
| AC-8 | AS_PATH with a set segment AND a structural fault (overrun) | Reports the RFC 7606 Section 7.2 structural description (precedence preserved) |
| AC-9 | Static route configured with a raw `attribute` AS_PATH holding an AS_SET, peer never held the prefix | No UPDATE announcing the prefix reaches the peer; `ze_bgp_as_set_withdrawn_total` increments by the route count; one warning line names the peer, the cause (RFC 9774 Section 3 forbids advertising AS_SET) and the fix (remove the set from the configured AS_PATH) |
| AC-10 | Prefix advertised clean, then the producer replaces it with an AS_SET variant | The peer receives a withdrawal of the prefix |
| AC-11 | `send bgp <peer> update hex attr set <AS_PATH with AS_SET> ...` | Same as AC-9 |
| AC-12 | AS_CONFED_SET in the transmitted AS_PATH, or a set in a transmitted AS4_PATH | Same as AC-9 |
| AC-13 | Interop: injector sends Ze one prefix with an AS_SET and one control prefix; Ze relays to FRR with `no bgp reject-as-sets`; separately Ze advertises a configured static route with a raw AS_SET path to the same FRR | FRR holds the control prefix and holds neither AS_SET prefix |
| AC-14 | Enrolment | `rfc/short/rfc9774.md` exists with `Implementation ze`, each Section 3 MUST as a verbatim row, Sections 5.2/6.1 as `feature-out-of-scope`, extraction sign-off, tagged tests with discrimination records; `docs/features/rfc-status.md` row regenerated; `./le rfc check` passes for the stem |
| AC-15 | `send bgp <peer> raw ...` carrying an AS_SET | Per Q-4 (b): written as given |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A peer advertises an aggregate with an AS_SET | wire -> classifyRFC7606 -> validateASPath -> treat-as-withdraw -> synthesized withdrawal -> RIB | `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci` |
| 2 | Operator configures a static route with a raw AS_SET path | config -> static route -> writeUpdateBody guard -> withdrawal/no UPDATE -> peer | `test/encode/rfc9774-as-set-not-advertised.ci` |
| 3 | Ze sits between an AS_SET sender and FRR | injector -> Ze receive -> relay -> FRR | interop `bgp-rfc9774-as-set-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateASPathRejectsSetSegments` | `internal/component/bgp/message/rfc9774_test.go` | types 1 and 4 in AS_PATH, 2- and 4-octet, first/middle/last segment -> treat-as-withdraw with the RFC 9774 description; types 2 and 3 accepted | |
| `TestValidateASPathStructureBeforeSet` | same | AC-8 precedence | |
| `TestValidateAS4PathRejectsSetSegments` | same | AS4_PATH set -> treat-as-withdraw; AS 0 without set -> still attribute discard; misframed AS4_PATH not judged (A-7) | |
| `TestRFC9774ReceiveASSetTreatAsWithdraw` | `internal/component/bgp/reactor/rfc9774_receive_test.go` | AC-1, AC-2, AC-7 through `Session.enforceRFC7606` | |
| `TestRFC9774ReceiveAS4PathSetTreatAsWithdraw` | same | AC-3, AC-4 | |
| `TestRFC9774ReceiveReannounceWithdraws` | same | AC-5, AC-6 | |
| `TestRFC9774ReceiveSetNotCoalesced` | same | R-7 | |
| `TestRFC9774SendASSetWrittenAsWithdrawal` | `internal/component/bgp/reactor/rfc9774_send_test.go` | AC-9, AC-10, AC-12, IPv4 and MP, ADD-PATH send framing (A-4) | |
| `TestRFC9774SendRawUntouched` | same | AC-15 | |
| `BenchmarkWriteUpdateBodyCleanPath` | same | R-5, zero allocation on a clean body | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AS_PATH segment type | 1..4 structurally; 2..3 accepted after RFC 9774 | 2 and 3 | 1 (AS_SET, treat-as-withdraw); 0 (unrecognized, already treat-as-withdraw) | 4 (AS_CONFED_SET, treat-as-withdraw); 5 (unrecognized, already treat-as-withdraw) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc9774-as-set-receive-treat-as-withdraw` | `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci` | test peer announces a clean prefix, then the same prefix with an AS_SET, then an AS_CONFED_SET prefix: `show bgp rib` holds none of them at the end, session up | done 2026-10-19 (commit 0a5c6a7b15): proven through the route-server relay rather than `show bgp rib`: the receiver sees P, P withdrawn, the fence, never Q or a set segment, and the stderr carries the RFC 9774 AS_SET verdict. Red with the receive scan off (the verdict line; the send guard alone keeps the frames clean) |
| `rfc9774-as-set-not-advertised` | `test/encode/rfc9774-as-set-not-advertised.ci` | static route with raw AS_SET AS_PATH plus a clean static route: test peer receives the clean one only | done (in HEAD via ca4eae70a0); red with the send guard off, re-observed 2026-10-19 |
| `rfc9774-as-set-api-not-advertised` | `test/plugin/rfc9774-as-set-api-not-advertised.ci` | `update hex attr set` with an AS_SET: peer receives nothing for it | done 2026-10-19 (commit 0a5c6a7b15): peer receives a withdrawal of the never-held prefix (the API rail is not ownership-tracked), then the fence; the warning names RFC 9774 Section 3. Red with the send guard off |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc9774-as-set-frr` | `test/interop/scenarios/bgp-rfc9774-as-set-frr/` (`ze.conf`, `frr.conf`, `inject.msg`, checker) | FRR 10.3.1 with `no bgp reject-as-sets`, raw injector | AC-13. Discriminates both halves: revert the receive scan and the injected AS_SET prefix appears at FRR via relay only if the send guard is also reverted, so the red runs are (1) both reverted -> injected prefix present, (2) send guard reverted -> static AS_SET prefix present | green 2026-10-19 (`interop: 1 passed, 0 failed`, no overlay, after f3cb2fe96f dropped the MED workaround; send-guard warning reads `routes=1`). Red walk below |

AC-13 red walk, 2026-10-19, each run `GOFLAGS=-overlay=<abs> VERBOSE=1 INTEROP_SCENARIO=bgp-rfc9774-as-set-frr ./le --name r9774c test integration interop`, one at a time, over f3cb2fe96f. Overlays: receive off makes `validateASPath`'s set scan (`message/rfc9774.go`) return nil; send off makes `writeUpdateBody`'s guard condition `!raw && false && ...` (`reactor/session_write.go` line 612).

| Overlay | Assertion that went red (verbatim) | What FRR or Ze held |
|---------|-----------------------------------|---------------------|
| receive scan off | `scenario bgp-rfc9774-as-set-frr assertion 6: ... peer output unexpectedly contains "10.97.74.0/24"` | Ze's `show bgp rib` held the injected set route; assertions 1-5 passed, so the send guard alone kept the relayed set off FRR (Ze logged the send warning, `routes=1`) |
| send guard off | `scenario bgp-rfc9774-as-set-frr assertion 4: ... peer output unexpectedly contains "10.97.76.0/24"` | FRR held the static set route as `"10.97.76.0/24": [{... "path":"65001" ...}]`: FRR kept the first of the two AS_PATH attributes (Ze's typed one), so the set is not shown, yet the UPDATE carrying it was advertised. The injected set route stayed off FRR (receive scan intact) |
| both off | `scenario bgp-rfc9774-as-set-frr assertion 4: ... peer output unexpectedly contains "10.97.74.0/24"` | FRR held `"10.97.74.0/24": [{... "path":"65001 65004 {65005,65006}" ...}]` beside 10.97.75/76/77; validates A-5 |
| none | green, `interop: 1 passed, 0 failed` | |

## Files to Modify
- `internal/component/bgp/message/rfc7606.go` - `validateASPath`: call the set scan after the structure walk, beside the AS 0 scan.
- `internal/component/bgp/message/rfc7607.go` - `validateAS4PathAttr`: call the set scan first (treat-as-withdraw outranks discard); rewrite its doc comment, which says it answers AS 0 only (`ai/rules/stale-comments.md`).
- `internal/component/bgp/reactor/session_write.go` - one guard call in `writeUpdateBody`, before the AIGP guard (a withdrawal carries no AIGP).
- `internal/component/bgp/reactor/reactor_metrics.go` - register `ze_bgp_as_set_withdrawn_total` (no label, or `afi-safi` if the sibling counters carry one).
- `rfc/short/rfc4271.md`, `rfc/short/rfc5065.md`, `rfc/short/rfc6793.md` - note RFC 9774 updates for the AS_SET rows (R-2).
- `docs/architecture/wire/attributes.md` - AS_PATH section: receive and send behavior.
- `docs/architecture/wire/messages.md` - RFC 7606 action table row for the set verdict.
- `docs/features/bgp-protocol.md` - table row (beside lines 308-310).
- `docs/guide/rpki.md` - line 552 states the implemented behavior.
- `docs/guide/monitoring.md` - counter row beside line 424.
- `features/bgp-protocol.md` - evidence entry.

## Files to Create
- `internal/component/bgp/message/rfc9774.go` - the shared set scan (`// RFC 9774 Section 3` quote above it), used by both validators, mirroring `asPathHoldsASZero`.
- `internal/component/bgp/message/rfc9774_test.go`
- `internal/component/bgp/reactor/session_rfc9774.go` - set detection over a transmitted body and the withdrawal write, mirroring `session_aigp.go`.
- `internal/component/bgp/reactor/rfc9774_receive_test.go`, `internal/component/bgp/reactor/rfc9774_send_test.go`
- `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci`, `test/encode/rfc9774-as-set-not-advertised.ci`, `test/plugin/rfc9774-as-set-api-not-advertised.ci`
- `test/interop/scenarios/bgp-rfc9774-as-set-frr/`
- `rfc/short/rfc9774.md`, `rfc/extraction/rfc9774.json`, `rfc/discrimination/rfc9774.json` (produced by the `/ze-rfc` and `./le rfc` tools, never by hand)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Q-1 (a): no knob. Under Q-1 (b): `internal/component/bgp/yang/ze-bgp-conf.yang` `container session` |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf; Q-3 (b) would add a config validator |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | Yes | `test/plugin/rfc9774-as-set-api-not-advertised.ci` covers the existing `update hex` API |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | Yes | `ze_bgp_as_set_withdrawn_total` in `reactor_metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/bgp-protocol.md` |
| 2 | Config syntax changed? | No | Q-1 (a) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | `update hex` syntax unchanged; behavior noted in `docs/architecture/api/update-syntax.md` wire mode section |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/rpki.md`, `docs/guide/monitoring.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md`, `docs/architecture/wire/messages.md` |
| 8 | Plugin SDK/protocol changed? | No | plugins see withdrawals they already handle |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9774.md` (new), `rfc/short/rfc4271.md`, `rfc/short/rfc5065.md`, `rfc/short/rfc6793.md`, regenerated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | uses the existing injector and FRR lab |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it carries an AS_SET or RFC 9774 row (check) |
| 12 | Internal architecture changed? | No | the RFC 7606 rail and writer gain one check each |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-bgp-rfc9774-as-set-deprecation.md` at implementation start |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/debugging-tools.md` line 91 (`[{1 2}]` decode rendering stays valid); `docs/architecture/api/update-syntax.md` hex example (no set, stays valid) |

## Implementation Steps

1. **Phase: Wiring** - write the five wiring tests; they fail because no scan or guard exists.
   - Tests: the Wiring Test table
   - Files: `rfc9774_receive_test.go`, `rfc9774_send_test.go`, the three `.ci`
   - Verify: each test fails on the missing verdict, not on setup
2. **Phase: Receive** - shared scan in `message/rfc9774.go`, called from `validateASPath` and `validateAS4PathAttr`; docs `attributes.md`, `messages.md` in the same phase.
   - Tests: message unit tests, receive reactor tests, receive `.ci`
   - Verify: R-1 inventory of reds walked and each one classified in the spec's Design Insights
3. **Phase: Send** - `session_rfc9774.go` and the `writeUpdateBody` call; counter; warning line; docs `monitoring.md`.
   - Tests: send reactor tests, encode and API `.ci`, benchmark
4. **Phase: Enrolment** - `/ze-rfc` for RFC 9774; walk both arithmetics; tag tests; `./le rfc discriminate-record` for each tag; update rfc4271/5065/6793 summaries; regenerate rfc-status.
5. **Phase: Interop** - `bgp-rfc9774-as-set-frr`, discrimination walk recorded.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The set scan judges only well-framed segments and never reads a length byte as a type |
| Correctness | Treat-as-withdraw outranks AS 0 attribute discard on AS4_PATH (RFC 7606 Section 3(h)) |
| Correctness | A withdrawal body never re-triggers the guard (it carries no AS_PATH) |
| Rule: user-facing-errors | The warning names peer, cause, RFC section and fix |
| Rule: rfc-compliance | `// RFC 9774 Section 3: "..."` quote above the scan and the guard; callers carry `// RFC 9774 Section 3` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Receive scan | `./le job run` scoped `go test ./internal/component/bgp/message/ -run RFC9774` |
| Send guard | scoped `go test ./internal/component/bgp/reactor/ -run RFC9774` |
| Enrolment | `./le rfc check` for stem rfc9774 |
| Interop | `INTEROP_SCENARIO=bgp-rfc9774-as-set-frr ./le test integration interop` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the scan is bounded by the attribute length; a 255-segment AS_PATH costs one pass |
| Resource exhaustion | the warning line is rate-limited; the peer controls its frequency |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `docs/guide/rpki.md` claimed an RFC 9774 receive policy that did not exist; corrected in a separate commit at design time.
- The caller's note named FRR 10.5; the lab pins FRR 10.3.1.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Receive scan inside the stateless attribute validators | a session-level policy in `validateRFC7606Attrs` | no knob (Q-1 a) means no session state is needed; mirrors RFC 7607; the coalescer and every reader inherit the verdict |
| Send guard in `writeUpdateBody`, writing the withdrawal form | refuse at each producer (config, API, relay, plugin); drop the UPDATE silently at the writer | one declaration covers every producer; a withdrawal removes a route the peer may still hold, a drop would leave it stale |
| No operator override | per-peer `session as-set accept` | Q-1; permitted, not required, by Section 3 |
| Sections 5.2 and 6.1 `feature-out-of-scope` | implement consistent brief aggregation | Ze has no aggregation; the owner's decision is RFC 9774, not an aggregation feature |

## Known Limitations
- No aggregation feature exists, so RFC 9774 Sections 5.2 and 6.1 are recorded as implementation gaps under `feature-out-of-scope`, not implemented.

## RFC Documentation (Scope: protocol)

Add `// RFC 9774 Section 3: "<quoted requirement>"` above the scan and the guard; each
caller carries `// RFC 9774 Section 3`.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| By default no route whose AS_PATH or AS4_PATH carries an AS_SET or AS_CONFED_SET is installed from a peer | interop + functional | `bgp-rfc9774-as-set-frr` green 2026-10-19, and its receive-off red at assertion 6 (Ze's RIB held 10.97.74.0/24, AC-13 red walk above); `test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci` |
| By default no such route is advertised to a peer, whatever produced it (relay, config, API, plugin) | interop + functional + unit | Same scenario green, send-off red at assertion 4 (FRR held static set route 10.97.76.0/24) and both-off red (FRR held relayed 10.97.74.0/24 with `{65005,65006}`); `test/encode/rfc9774-as-set-not-advertised.ci` (config), `test/plugin/rfc9774-as-set-api-not-advertised.ci` (API); `TestRFC9774SendGroupedStaticRoutesKeepCleanTwin` (grouped config rail withdraws the set route alone, red before f3cb2fe96f with counter 2) |
| A clean route is never withdrawn because a twin carries a set | unit + interop | `TestRFC9774SendGroupedStaticRoutesKeepCleanTwin`; scenario green with no MED workaround, FRR holds 10.97.77.0/24, send warning `routes=1` |
| Enrol RFC 9774 | gate | `rfc/short/rfc9774.md` (`Implementation ze`, two Section 3 rows); `./le rfc check` 2026-10-19 prints no rfc9774 line (exit 2 from other stems only) |
| Sections 5.2 and 6.1 recorded as out of scope, never counted as met | gate | `rfc/extraction/rfc9774.json` `feature-out-of-scope` exclusions, named in the summary's enrolment reason |

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-bgp-rfc9774-as-set-deprecation.md` only, in the same `./le commit create` script
