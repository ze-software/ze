# Spec: forwarded-as-path-obeys-rfc6793-for-every-destination

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/8 |
| Handoff | - |
| Updated | 2026-09-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The AS-path family is resolved on TWO rails on the forward path, and the rail a
destination takes is decided by whether it is an eBGP peer.

`forwardUpdateCore` (`internal/component/bgp/reactor/reactor_api_forward.go`)
calls `aspathEdit.Record` inside `if facts.isEBGP`, and `reactorForwardRS`
(`internal/component/bgp/reactor/forward_rs.go`) carries the same guard. A
non-eBGP destination therefore never reaches `wireu.ASPathEdit`. Its AS-path
family is resolved later instead, by `wireu.TranscodeASPath` reached through
`fwdUpdateForDestination` and `buildFwdBody`
(`internal/component/bgp/reactor/forward_body.go`).

**The width is not the defect.** The commissioning brief expected a non-eBGP
destination to receive AS_PATH at the SOURCE session's width. It does not:
`buildFwdBody` takes its re-encode branch whenever the source and destination
context IDs differ, and `fwdUpdateForDestination` runs `wireu.TranscodeASPath`
whenever the two contexts disagree on ASN4. `TestForwardSplitConvertsASN4Context`
(`internal/component/bgp/reactor/forward_body_test.go`) pins that behavior today,
and its destination peer carries no eBGP marking at all.

What the two rails do NOT agree on is the AS4_PATH companion, and that
disagreement is two RFC 6793 defects.

**D-1, the reconstruction.** Neither rail performs the RFC 6793 Section 4.2.3
reconstruction when it widens a two-octet path to four octets. A route learned
from an OLD speaker arrives with AS_TRANS in AS_PATH and the real four-octet AS
numbers in AS4_PATH. `TranscodeASPath` parses AS_PATH at two octets, re-encodes
those same values at four octets, and DROPS the received AS4_PATH without
reading it; `ASPathEdit.recordTranscode` does the same through
`as4PathForPath`, which answers nil for any four-octet destination. Every
downstream speaker then sees AS 23456 where a real AS was, permanently. The
attribute's own doc comment states the omission and misplaces the repair: it
says the merge "is done at ingress by the receiving session", and the ingress
merge is `attribute.MergeAS4Path` called from
`internal/component/bgp/plugins/rib/storage/attrparse.go`, which builds the RIB
view and never touches the wire bytes the forward path relays.

**The shape below FIXES D-1, at the one place the merge belongs.** The collapse
runs once on the received payload, so the bytes Ze relays carry the same
four-octet truth the RIB already holds, and no forward rail needs a widening arm
at all.

**D-2, the relay between NEW speakers.** RFC 6793 Section 4.1 forbids carrying
AS4_PATH in an UPDATE between NEW speakers, and Section 6 obliges a NEW speaker
receiving one from another NEW speaker to discard it. Ze does neither on the
forward path when the two sessions are both four-octet. The eBGP prepend rail
drops it (`recordPrepend` reaches `recordAS4Path` with a nil derivation) and
`recordWithdrawOnly` drops it on purpose with the rule written out, but
`recordTranscode` returns early on equal widths and `TranscodeASPath` returns 0
on equal widths, so an RS client and every non-eBGP destination receive the
attribute verbatim. `rfc/short/rfc6793.md` already records this as
`RFC6793-4.1-6` and `RFC6793-4.1-7`, and both annotations cite
`internal/component/bgp/wireu/aspath_rewrite.go`, whose two exported entry
points have no non-test caller.

**The shape below FIXES the receive half of D-2 and DISSOLVES the relay half.**
The fix is at ingest: an AS4_PATH or AS4_AGGREGATOR arriving from a NEW speaker
is DISCARDED there rather than merged, which is the Section 4.1 receive
obligation `RFC6793-4.1-7` records as unmet, and which today's ingest code
misses because `canonicalizeASPath` merges a present AS4_PATH without consulting
the `asn4` flag it is given. The relay half then has nothing left to get wrong:
with no AS4_PATH surviving ingest, no forward rail can carry one between NEW
speakers, and no rail needs a rule saying so.

**The goal.** The AS path information Ze holds for a received route is
four-octet truth, reconstructed once at ingest under RFC 6793 Sections 4.1,
4.2.3 and 6, and every forwarded destination receives that truth encoded for its
own negotiated width under Section 4.2.2. A four-octet destination does no
AS-path work at all.

## The shape, decided by the owner on 2026-09-08

**Ze stores four-octet AS numbers and nothing else, and regenerates the
two-octet form when it builds the attribute.** No second stored encoding, and no
cached wire form to take a lock on: the owner's reasoning is that regenerating
at encode is cheaper than storing and retrieving a second copy, and the
conversion is a SLOW PATH that a fleet of four-octet speakers never enters.

This is what FRR and BIRD both do, read from their source on 2026-09-08 rather
than assumed:

| Implementation | Where the pair is reconciled | What is stored |
|----------------|------------------------------|----------------|
| FRR | ingest, `aspath_reconcile_as4` called from `bgp_attr_parse` (`bgpd/bgp_attr.c`) | one four-octet path; the AS4_PATH is uninterned "in order to save memory" |
| BIRD | ingest, `bgp_process_as4_attrs` (`proto/bgp/attrs.c`), which unsets `BA_AS4_PATH` before storing | one four-octet path |

Neither carries the two attributes past ingest, which is exactly why neither
needs a Section 4.2.3 step on its forward path. Both regenerate AS_PATH at the
peer's width and rebuild AS4_PATH at egress, and both omit AS4_PATH when no AS
number in the path exceeds 65535.

**What that means for Ze, whose relay forwards payload bytes rather than
encoding from a stored path.** The collapse happens ONCE, at ingest, on the
payload: an UPDATE that arrives on a two-octet session, or that carries an
AS4_PATH, has its AS-path family rewritten into four-octet truth before anything
caches or relays it. After that:

- Every four-octet destination is the fast path and does no AS-path work at all,
  because the bytes it forwards are already the bytes it owes.
- A two-octet destination is the slow path: narrow at encode, substitute
  AS_TRANS for a non-mappable AS number, and rebuild AS4_PATH from the same
  path. `attribute.ASPath.WriteToWithASN4` and `wireu.AS4PathForRewrite` both
  already do this half.
- D-2 stops being a case to handle. With no AS4_PATH past ingest, there is
  nothing for the relay to carry between NEW speakers by mistake.

The reconstruction rule itself is already declared once, in
`attribute.MergeAS4Path` (`internal/core/bgp/attribute/as4.go`), and
`reconstructASPath` (`internal/component/bgp/plugins/rib/storage/attrparse.go`)
already calls it to build the RIB view. What this spec adds is a second caller
on the ingest rail so that the bytes Ze relays carry the same truth the RIB
already holds. **This is a MOVE plus a caller, never a second implementation of
the rule.** Where the byte-level collapse lands, and whether `reconstructASPath`
then calls it rather than keeping its own copy, is an implementation question
this spec's design phase answers; the constraint is that one function owns it.

## Where the collapse lives (answers the constraint above)

Two levels, each declared once, because the two callers need different things.

| Level | Home | What it takes and answers | Why there |
|-------|------|---------------------------|-----------|
| The RULE, over attribute VALUE bytes | `internal/core/bgp/attribute` | AS_PATH value, AS4_PATH value, AGGREGATOR value, AS4_AGGREGATOR value, source ASN4 → canonical four-octet AS_PATH value, canonical AGGREGATOR value, and the RFC 6793 Section 6 discard reason | `MergeAS4Path` already lives there, the package imports only `internal/core/*`, and both callers already import it |
| The PAYLOAD rewrite, over a whole UPDATE body | `internal/component/bgp/wireu` | UPDATE payload plus source ASN4 → the collapsed payload, or the same slice unchanged when nothing is owed | Every UPDATE-payload rewrite in Ze lives in `wireu`, and the reactor already imports it |

The RULE is a MOVE, not a new function. `canonicalizeASPath`,
`reconstructASPath`, `widenASPath`, `expandASPath2to4`, `selectAggregator` and
`aggregatorASIsTrans` are unexported in
`internal/component/bgp/plugins/rib/storage/attrparse.go` today. They move up
into `internal/core/bgp/attribute`, exported, for the INGEST rail to call.
Nothing is copied.

`ParseAttributes` does not follow them. A-5, resolved 2026-09-08: every
`PeerRIB.Insert` and `ParseRouteEntry` caller passes a literal `true` except
`rib_structured.go`, whose value the relabel makes `true` as well, so the RIB's
`asn4` parameter becomes constant and is DELETED rather than defaulted
(`ai/rules/no-layering.md`). Post-collapse `ParseAttributes` reads a canonical
four-octet AS_PATH and reconstructs nothing, which is what makes this a move OUT
of `rib/storage` rather than a move up with the old caller retained.

**The move is forced by the tier direction, not chosen for tidiness.**
`internal/component/bgp/reactor` is a component and
`internal/component/bgp/plugins/rib/storage` is a plugin, so the reactor MUST
NOT import it (`ai/rules/architecture.md`: dependency direction decides
placement, and `./le tier check` refuses the reverse edge). The rule therefore
cannot stay where it is and be reached from ingest. `internal/core/bgp/attribute`
is legal for both: it imports `internal/core/bgp/wire`,
`internal/core/bgp/context`, `internal/core/textbuf`, `internal/core/stringsx`
and `internal/core/bgp/asn` and nothing above them, `rib/storage` already imports
it for `MergeAS4Path`, and `wireu` already imports it for the same function.

**The moved rule reports its discards rather than logging them.** The
`attribute` package holds no logger and takes none, and adding one to a leaf on
the parse path is a dependency Ze does not owe. `reconstructASPath` logs two
warnings today, both required by RFC 6793 Section 6 ("The error SHOULD be logged
locally for analysis"). The moved function returns which attribute it discarded
and why, and each caller logs under its own subsystem: `rib/storage` keeps
`bgp.rib`, and the ingest collapse writes under the session subsystem. Both
returns MUST be read, and the doc comment says so on both sides: the canonical
AS path is always usable, and a non-nil discard reason is a report for the log
rather than a failure (`ai/rules/principles.md`).

## The ingest site (answers "before or after the import policy chain")

**`Session.processMessage` (`internal/component/bgp/reactor/session_read.go`),
immediately after the RFC 7606 block and before `validateUpdateFamilies`. That
is BEFORE the import policy chain.**

| Why this site | Evidence read at the producer |
|---------------|-------------------------------|
| After RFC 7606, because that enforcement judges what the PEER sent and rewrites the payload itself | `enforceRFC7606` returns a possibly rewritten `wireUpdate` (attribute-discard tombstoning), and the treat-as-withdraw arm replaces it again with a synthesized primary |
| Before the import policy chain, because the chain READS the AS path and must read the truth | `LoopIngress` (`internal/component/bgp/reactor/filter/loop.go`) iterates `attribute.AttrASPath` at `src.ASN4` and never reads AS4_PATH, so it compares Ze's own AS against AS_TRANS today |
| Before the `ReceivedUpdate` cache, because that cache is what both forward rails read | `notifyMessageReceiver` inserts `ReceivedUpdate` after the filter loop, and `forwardUpdateCore` and `reactorForwardRS` both take their payload from it |
| NOT inside `notifyMessageReceiver`, because that function returns early when no message receiver is registered, before the filter loop | The `receiver == nil` early return sits above the ingress loop; a collapse there would be conditional on a plugin being registered, which is the accidental-guard shape `docs/contributing/ze-go-style.md` names |
| The shape already exists at this site | `processMessage` replaces the `WireUpdate` with a rewritten one twice already, so the collapse joins an existing pattern rather than adding one |
| The negotiated width is readable without a cross-goroutine caveat | `processMessage` runs on the session read goroutine and reads `s.recvCtxID` under `s.mu`; `notifyMessageReceiver` documents its own `peer.session` read as a verified-benign unlocked one |
| Both reference implementations put it in the receive decode | FRR's `bgp_attr_parse` and BIRD's `bgp_process_as4_attrs` both run before import policy, which is `processMessage` and not the plugin fan-out |

**The relabel is part of the collapse and is not optional.** The collapsed
payload is four octets wide while the session's `recvCtxID` says two, and three
consumers read that context rather than the negotiated capability. `buildFwdBody`
compares source against destination context and would widen an already-widened
path; `fwdUpdateForDestination` reads `srcCtx.ASN4()`; and the RIB reads
`asn4 := ctx == nil || ctx.ASN4()` (`internal/component/bgp/plugins/rib/rib_structured.go`).
So a collapsed payload is carried by a `WireUpdate` labelled with
`fwdContextIDWithASN4(recvCtxID, true)`. That helper already exists in package
`reactor` (`forward_context.go`), reached today only from the two forward rails,
and it preserves the ADD-PATH map while flipping ASN4 alone.

**`PeerFilterInfo.ASN4` moves with the payload.** `notifyMessageReceiver` sets
`src.ASN4` from `peer.session.Negotiated().ASN4` today. Post-collapse that value
describes the SESSION and no longer describes the PAYLOAD the filters parse, so
it is read from the WireUpdate's encoding context instead. Leaving it would make
every in-process ingress filter parse a four-octet AS_PATH at two octets.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/edge-cases/as4.md` - the RFC 6793 Section 4.2.3 receive procedure as Ze implements it
  → Decision: this page is the one that DECLARES the reconstruction, and it already states it as an INGEST-path behavior owned by `selectAggregator` and `canonicalizeASPath`. The new shape makes that statement true of the whole daemon rather than of the RIB alone, so the page's claim survives and its OWNER changes.
  → Constraint: the page's "What it costs" section says the reconstruction runs only for an UPDATE carrying an AS4_PATH and that "the common path parses nothing and allocates nothing beyond the widening buffer a two-octet AS_PATH already needed". That sentence becomes the performance contract of the ingest collapse and is what AC-9's benchmark pins.
- [ ] `docs/architecture/wire/attributes.md` - AS_PATH, AS4_PATH and AS4_AGGREGATOR wire encoding, and the policy-prepend width rule
  → Decision: the AS4_PATH Ze emits carries the WHOLE path rather than the received one with local AS numbers on top; with no received AS4_PATH surviving ingest, that reduces to deriving the AS4_PATH from the outgoing path alone, which `as4PathForPath` already does.
  → Constraint: the page describes the ORIGINATING and PREPEND rails only. It is SILENT on the receive-side collapse, so this spec's page edit adds the ingest step and says what survives it.
- [ ] `docs/architecture/encoding-context.md` - `EncodingContext`, `ContextID` dedup, and the `Transcoder` interface
  → Constraint: `computeHash` (`internal/core/bgp/context/context.go`) folds ASN4, direction, ExtendedMessage, IsIBGP, LocalASN, PeerASN and the ADD-PATH map, so two peers of different widths can never share a context ID and the ASN4 relabel always produces a distinct ID.
  → Constraint: the page is SILENT on a received payload being relabelled at ingest. Every context relabel it describes is an egress one, so the page edit adds the receive-side relabel and names `fwdContextIDWithASN4` as its owner.
- [ ] `docs/architecture/behavior/fsm-established.md` - the established-state receive path `session_read.go` declares
  → Constraint: the page describes what `processMessage` does to a received UPDATE before dispatch. The collapse is a new step in that sequence and the page's ordering list gains it, between RFC 7606 enforcement and family validation.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the page `attrparse.go` declares
  → Constraint: it describes the RIB as the owner of the Section 4.2.3 reconstruction. After the move the RIB is a CALLER of the rule rather than its owner, and the page says which package holds it.
- [ ] `docs/architecture/testing/interop.md` - scenario structure, the container address table, and "Writing a New Scenario"
  → Constraint: a new scenario owes a directory under `test/interop/scenarios/`, an entry in `scenarioOperations` (`internal/le/interoplab/bgp/checkers.go`), and a `TestCheckerPopulationMatchesProducer` match. A scenario that reads the RIB needs `attach process rib` on the peer, not just the plugin instance.
  → Constraint: the raw injector is 172.30.0.9, FRR is 172.30.0.3, Ze is 172.30.0.2.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6793.md` - the requirement ledger this spec closes rows in
  → Constraint: `RFC6793-4.1-6` and `RFC6793-4.1-7` are recorded `{gap}`. `-4.1-7`'s annotation states the live defect precisely: "the discard is never conditioned on the negotiated capability ... `canonicalizeASPath` merges a present AS4_PATH into the AS path information without consulting the asn4 flag it is given". The collapse is where that condition goes, so the row is closed by ADDING the NEW-speaker discard rather than by moving code.
  → Constraint: `-4.1-6`'s annotation cites `aspath_rewrite.go` at five line numbers, a file with no non-test caller. Those citations are rewritten against the live producers, not merely un-gapped.
  → Constraint: `RFC6793-4.2.3-8`, `-9` and `-10` carry no `{gap}` because the RIB ingest path satisfies them. After the move they are satisfied by the same rule at a new address, so their test bindings are re-pointed rather than re-proven.
  → Constraint: line 15-19 (`Support`, `Support status`, `Support coverage`, `Support remaining`) and the generated `docs/features/rfc-status.md` row both change when a gap closes. `Support remaining` names four gaps today and loses two.

**Key insights:** (minimal context to resume after compaction)
- The collapse belongs at ingest and NOT on the forward path. Both forward rails become correct for free, so `Record` stays inside its `isEBGP` guard and neither rail is restructured.
- `wireu.TranscodeASPath` is NOT dead under this shape. It keeps its one non-test caller (`forward_body.go`) and becomes the narrowing encoder every non-eBGP two-octet destination takes; only its 2→4 widening arm dies.
- `wireu.RewriteASPath` and `wireu.RewriteASPathDual` have NO non-test caller and are deleted. `plan/journal/unwired-feature.md` records the same finding on 2026-09-05.
- The RIB reads ASN4 from the WireUpdate's encoding context (`rib_structured.go`, `asn4 := ctx == nil || ctx.ASN4()`), so the ingest relabel carries the RIB with it and needs no RIB-side change.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/session_read.go` - `processMessage` builds the `WireUpdate` from the session's `recvCtxID`, runs `enforceRFC7606` (which may return a rewritten UPDATE, or synthesize withdraw bodies and dispatch the extras itself), then `validateUpdateFamilies`, then `checkPrefixLimits`, then `onMessageReceived`, then the policy-teardown check. Nothing between the header parse and `onMessageReceived` reads the AS path.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` returns early when `r.messageReceiver` is nil; otherwise it builds the `RawMessage`, runs `r.orderedIngressSteps` (in-process filters plus the per-peer policy chain), rebuilds the `WireUpdate` over any modified payload, and inserts the `ReceivedUpdate` cache entry the forward rails read. `src.ASN4` is set from `peer.session.Negotiated().ASN4`.
- [ ] `internal/component/bgp/reactor/filter/loop.go` - `LoopIngress` iterates `attribute.AttrASPath` at `src.ASN4` through `NewASPathIterator` and `NewASNIterator`, and reads no AS4_PATH at all.
- [ ] `internal/component/bgp/plugins/rib/storage/attrparse.go` - `ParseAttributes(raw, asn4)` runs `selectAggregator` (the RFC 6793 Section 4.2.3 AGGREGATOR gate, with its own length check) and `canonicalizeASPath` (the count comparison, `reconstructASPath`, `widenASPath` and `expandASPath2to4`). The merge is NOT conditioned on `asn4`, so an AS4_PATH from a NEW speaker is merged where Section 4.1 obliges a discard.
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - `asn4 := ctx == nil || ctx.ASN4()` reads the width from the WireUpdate's encoding context, not from the negotiated capability, and threads it into `PeerRIB.Insert` and `storage.ParseRouteEntry`.
- [ ] `internal/core/bgp/attribute/as4.go` - `MergeAS4Path` is the single declaration of the Section 4.2.3 construction, with the count comparison at :383, the leading-segment prepend at :396 and the confederation adjacency rule at :408. The package imports only `internal/core/*` and holds no logger.
- [ ] `internal/component/bgp/reactor/filter_format.go` - `asPathForFilter` is the THIRD caller of `MergeAS4Path`, building the AS path every text-mode filter judges. It needs no edit: post-collapse it finds no AS4_PATH and takes the merge's no-op arm, which is the same outcome by a shorter route.
- [ ] `internal/core/bgp/attribute/span.go` - `SpanIndex` carries a `presence [4]uint64` bitmap and `SpanInline` inline spans, so "does this UPDATE carry attribute 17 or 18" is one walk and two bit tests, and a normal UPDATE never reaches the spill slice.
- [ ] `internal/component/bgp/reactor/forward_context.go` - `fwdContextIDWithASN4` copies the source encoding, flips ASN4, preserves the ADD-PATH map and the family list, and registers the result. It returns the source ID unchanged when the widths already agree.
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody` compares `peerWire.SourceCtxID()` with the destination's `sendCtxID`; a mismatch parses the UPDATE and calls `fwdUpdateForDestination`, which runs `wireu.TranscodeASPath` when the two contexts disagree on ASN4, then re-frames ADD-PATH NLRI and MP attributes.
- [ ] `internal/component/bgp/wireu/aspath_slot.go` - `ASPathEdit.Record` has exactly two non-test callers, `reactor_api_forward.go` and `forward_rs.go`, both inside `if facts.isEBGP`. It dispatches empty `Prepend` to `recordTranscode`, a non-empty `Prepend` over a payload advertising no NLRI to `recordWithdrawOnly`, and the rest to `recordPrepend`. `recordWithdrawOnly` is `recordTranscode` plus one extra rule: the equal-width Section 4.1 AS4_PATH drop.
- [ ] `internal/component/bgp/wireu/aspath_transcode.go` - `TranscodeASPath` returns 0 on equal widths, narrows 4→2 with AS_TRANS and a derived AS4_PATH from `as4PathForPath`, rewrites AGGREGATOR in both directions, and skips a received AS4_PATH in its output loop. Exactly one non-test caller: `forward_body.go`.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - `RewriteASPath` and `RewriteASPathDual` are exported and have NO non-test caller anywhere in `internal/`, `cmd/` or `pkg/`.
- [ ] `internal/component/bgp/wireu/aspath_as4.go` - `as4PathForPath` owns "does this destination get an AS4_PATH, and what is in it". `AS4PathForRewrite` adds one branch for a received AS4_PATH, reached through `MergeAS4Path` and `joinSequences`.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - the egress policy prepend reads a received AS4_PATH out of the payload it is editing and passes it to `AS4PathForRewrite` with one width for both ends.
- [ ] `internal/perf/allocgate.go` - a benchmark name maps to an allocation ceiling, and `./le verify deps alloc` enforces every registered row. `BenchmarkForwardDirect` is 6, `BenchmarkFilterDispatch_ZeroAlloc` is 0.

**Behavior to preserve:**
- RFC 7606 enforcement judging what the PEER sent, ahead of any Ze rewrite, including the treat-as-withdraw synthesis and its extra per-family dispatches.
- The pcap and message-observer view of the received bytes: `processMessage` passes the original `body` as `rawBytes` to `onMessageReceived`, and `r.rawCapture` appends those bytes.
- The eBGP prepend order: the globally configured AS innermost and the RFC 7705 Section 3.3 "Local AS" override outermost, with `applyASOverride` writing AS_PATH after `Record`.
- RFC 7947 Section 2.2.2 for an RS client: AS_PATH is never MODIFIED, and at equal widths the AS_PATH value stays byte-identical to the one Ze holds.
- RFC 4271 Section 5.1.2 b: a withdraw-only UPDATE gets no prepend.
- The RFC 4456 iBGP suppression and the ORIGINATOR_ID / CLUSTER_LIST injection that follows it.
- The zero-copy raw-split branch of `buildFwdBody`, `fwdUpdateForDestination`'s ADD-PATH and MP re-framing, its single-ownership contract for the read-pool `BufHandle`, and the `fwdBodyCache` key.
- `Record` staying inside the `isEBGP` guard on both forward rails. Nothing about the rail split changes.

**Behavior to change:**
- A received UPDATE is collapsed to four-octet truth once, in `processMessage`, and the collapsed payload is carried by a `WireUpdate` relabelled with `fwdContextIDWithASN4(recvCtxID, true)`.
- An AS4_PATH or AS4_AGGREGATOR received from a NEW speaker is DISCARDED at ingest instead of merged, closing `RFC6793-4.1-7`'s receive half.
- `PeerFilterInfo.ASN4` is read from the WireUpdate's encoding context rather than the negotiated capability, so every ingress filter parses the payload it was given.
- The Section 4.2.3 rule MOVES from `internal/component/bgp/plugins/rib/storage/attrparse.go` to `internal/core/bgp/attribute`, exported, and reports its discards instead of logging them. `ParseAttributes` becomes one of its two callers.
- `wireu` gains one exported payload-level collapse that calls the moved rule, and returns the input slice unchanged when nothing is owed.
- `TranscodeASPath` loses its 2→4 widening arm, which no caller can reach once every source payload is four-octet.
- `aspath_rewrite.go` and its test are deleted, and the test fixtures that build payloads with `RewriteASPath` are rebuilt against `ASPathEdit.Record`.
- `recordWithdrawOnly`'s equal-width AS4_PATH drop, and with it the function and its arm of the `Record` dispatch, are deleted: `spans.Has(attribute.AttrAS4Path)` cannot be true for a payload that reached the forward path.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP UPDATE arrives on an established session. `Session.ReadAndProcess` frames it and `processMessage` receives the header and the body, held zero-copy in a pool `BufHandle`. Format at entry: the raw UPDATE body exactly as the peer sent it, labelled with the session's `recvCtxID`, whose ASN4 is what the OPEN negotiated.
- The forward path is no longer an entry point for this feature. It is a consumer of what the entry point produces.

### Transformation Path
1. `processMessage` builds the `WireUpdate` over the received body at `recvCtxID`.
2. `enforceRFC7606` judges the attributes the peer sent and may return a rewritten UPDATE or synthesize withdrawals.
3. **NEW: the collapse.** One span-index walk answers whether any AS-path work is owed. Owed when the session is two-octet, or when attribute 17 or 18 is present. When nothing is owed the payload is returned unchanged and steps 4 and 5 do not run.
4. **NEW:** the AS-path family is rewritten into four-octet truth: AS_PATH canonicalized under RFC 6793 Section 4.2.3 (or the received AS4_PATH discarded under Section 4.1 when the source is a NEW speaker), AGGREGATOR resolved through the Section 4.2.3 gate, AS4_PATH and AS4_AGGREGATOR removed. The result is a new heap payload, exactly as the RFC 7606 rewrite and the ingress-filter modification already produce.
5. **NEW:** a fresh `WireUpdate` is built over the collapsed payload at `fwdContextIDWithASN4(recvCtxID, true)`, and it replaces the one the remaining steps carry.
6. `validateUpdateFamilies` and `checkPrefixLimits` run as they do today.
7. `onMessageReceived` reaches `notifyMessageReceiver`, which runs the ordered ingress steps over the collapsed payload with `src.ASN4` read from its context, then inserts the `ReceivedUpdate` cache entry.
8. The RIB plugin parses the collapsed attributes at `ctx.ASN4()`, which is now true, so the moved rule finds no AS4_PATH and returns the AS_PATH value unchanged.
9. A forwarded destination reads the collapsed payload from the cache. A four-octet destination forwards it with no AS-path work. A two-octet destination narrows: an eBGP one through `ASPathEdit.Record`, a non-eBGP one through `fwdUpdateForDestination` and `TranscodeASPath`. Both derive the AS4_PATH from `as4PathForPath`, which is their single shared owner.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Received wire bytes ↔ collapsed payload | A new heap slice built by the `wireu` collapse; the pool `BufHandle` keeps its own ownership and the original bytes still reach pcap and the message observers | No |
| Session read path ↔ wireu | Payload plus source ASN4 in, collapsed payload plus a changed flag plus a discard reason out; no reactor type crosses | No |
| wireu ↔ attribute | The moved value-level rule, taking and returning attribute value bytes | No |
| Encoding context ↔ collapsed payload | `fwdContextIDWithASN4(recvCtxID, true)`, registered once per source context and then found by hash | No |
| Collapsed payload ↔ RIB | `rib_structured.go` reads `ctx.ASN4()` from the WireUpdate the reactor dispatched, so the relabel carries the RIB with no RIB-side change | No |
| Forward pool ↔ session writer | Unchanged: `fwdItem` carrying `rawBodies` or `updates` plus a pool handle | No |

### Integration Points
- `Session.processMessage` - the single site where a received UPDATE is normalized.
- `internal/core/bgp/attribute` - the moved Section 4.2.3 rule, with `MergeAS4Path` beside it and `ParseAttributes` as its second caller.
- `internal/component/bgp/wireu` - the payload-level collapse, calling that rule.
- `fwdContextIDWithASN4` - the relabel, reached from a third site.
- `filterapi.PeerFilterInfo.ASN4` - now describes the payload, not the session.
- `internal/perf/allocgate.go` - the ceiling row that proves the fast path allocates nothing.
- `internal/le/interoplab/bgp/checkers.go` - `scenarioOperations`, where the new scenario's ordered assertions register.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The collapse sits on the one receive path every consumer reads from, ahead of the filters, the cache, the RIB and both forward rails. |
| No unintended coupling (components stay isolated) | Yes | The reactor calls `wireu`, which it already imports. It does NOT reach into `rib/storage`, which is why the rule moves to `internal/core/bgp/attribute`. |
| No duplicated functionality (extends existing, does not recreate) | Yes | One rule, moved once, with two callers. `MergeAS4Path` stays the single declaration of the construction, and `as4PathForPath` stays the single declaration of the egress AS4_PATH question. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The collapse returns the input slice unchanged when nothing is owed, and a four-octet session carrying no AS4_* attribute is that case. AC-9 pins it at zero allocations. |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No registry is touched. The interop scenario registers through `scenarioOperations`, and the benchmark ceiling registers in the existing `allocgate` map. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every consumer of a received UPDATE reads its width from the WireUpdate's encoding context, never from the negotiated capability | `rib_structured.go` (`asn4 := ctx == nil \|\| ctx.ASN4()`), `forward_body.go` (`srcCtx.ASN4()`), `buildFwdBody`'s context comparison | A consumer parses a four-octet AS_PATH at two octets and corrupts every path it reads | A grep for `Negotiated().ASN4` and `neg.ASN4` over every path reached from a received UPDATE, recorded in the spec at implementation time; `PeerFilterInfo.ASN4` is already known to be one such site and is fixed by this spec | CONFIRMED 2026-09-08, and it found a FOURTH relabel site the spec did not name. `resolveRelaySource` (`reactor_api_relay.go`) stamps `srcPeer.recvContextID()`, the SESSION's receive context, onto the wire `buildRelayUpdate` reconstructs from stored `adj_rib_in` bytes. Those bytes are post-collapse four-octet while `recvCtxID` still says two octets, so the relay would label them wrong and every egress filter behind it would parse at the wrong width. The relabel is FOUR sites, and the two comments in that file asserting the stored bytes are still in the source's encoding are made false by this work. Two reads deliberately KEEP the negotiated width: `enforceRFC7606` (`session_validation.go`) judges what the peer sent and runs before the collapse, and every `sendASN4` reader is outgoing. `PeerFilterInfo.ASN4` has exactly one reader, `LoopIngress` |
| A-2 | Two peers of different negotiated widths can never share a `ContextID`, so the relabel always yields a distinct registered context | `computeHash` folds ASN4 (`internal/core/bgp/context/context.go`) and `ContextRegistry` dedups by that hash | The relabel could return a context another peer reads, and a four-octet label would reach a two-octet session | `TestEncodingContextIDDiffersOnASN4Alone`, registering two contexts that differ only in ASN4 | CONFIRMED 2026-09-09. The test is written and passes in `internal/core/bgp/context/registry_test.go`: two contexts sharing an identity and an ADD-PATH map and differing only in ASN4 register to different ids, and each id resolves back to a context of its own width |
| A-3 | `attribute.MergeAS4Path` implements RFC 6793 Section 4.2.3 correctly, including the count comparison and the confederation adjacency rule | `internal/core/bgp/attribute/as4.go` carries the quoted requirement at :383, :396 and :408, and `RFC6793-4.2.3-8`, `-9`, `-10` carry no `{gap}` | The ingest collapse inherits a wrong reconstruction and spreads it to the relayed bytes as well as the RIB | Reading `MergeAS4Path` and its tests, plus the new ingest tests asserting the exact reconstructed path | unvalidated |
| A-4 | The egress policy prepend (`ExtractASPathPrependOps`, `filter_delta.go`) runs over the pre-narrowing payload, so its `recvAS4` is nil once no AS4_PATH survives ingest | `AS4PathForRewrite`'s doc comment says that caller "edits ONE payload rather than transcoding between two", and passes one width for both ends | The `MergeAS4Path` branch of `AS4PathForRewrite` and `joinSequences` stay reachable and MUST NOT be deleted | Tracing the payload `ExtractASPathPrependOps` is given back to its producer at implementation time, before the deletion in step 6 | BROKEN 2026-09-08, in the safe direction. `exportFilterForBody` (`egress_inject_filter.go`) passes `facts.sendASN4`, so a two-octet destination still reaches the extractor with `asn4` false, and `computeWireChanges` takes an operator-supplied width. So `AS4PathForRewrite`'s merged branch and `joinSequences` are NOT deleted by this spec. What they lose is every FORWARDING caller: an originated body carries no AS4_PATH (no encoder in `internal/component/bgp/message/update_build*.go` emits one), so `recvAS4` is nil on that arm, and the merged branch survives only on the `policy dry-run` path with an operator-supplied body. `AS4PathForRewrite`'s doc comment explains itself in terms of an OLD-speaker SOURCE and is made wrong for the forwarding rails by this work, so it is corrected in the same change (`ai/rules/stale-comments.md`) |
| A-5 | `ParseAttributes` is reached only with payloads that already passed the ingest collapse, or with injected attributes that are already four-octet | `PeerRIB.Insert` call sites pass a literal `true` except `rib_structured.go`, which passes `ctx.ASN4()` | The RIB's call to the moved rule is still load-bearing for a real two-octet input, and the parameter cannot be reasoned away | A grep over every `Insert` and `ParseRouteEntry` call site, including `rib_commands.go`, `rib_inject.go` and `capture_replay.go`, recorded in the spec | BROKEN 2026-09-08, and it makes the move SIMPLER than the spec said. `storage.ParseAttributes` has no caller outside its own package, and every `PeerRIB.Insert` / `ParseRouteEntry` caller passes a literal `true` except `rib_structured.go`, which the relabel turns into `true` as well. So post-collapse the `asn4` parameter is constant and is DELETED rather than defaulted (`ai/rules/no-layering.md`), and `ParseAttributes` calls none of the moved helpers: it reads a canonical four-octet AS_PATH. The helpers move OUT of `rib/storage` for the ingest rail to use, rather than moving up for the RIB to keep calling. `capture_replay.go` is a writer only and its replay re-enters `processMessage`, so it gets the collapse |
| A-6 | Deleting `aspath_rewrite.go` breaks only test fixtures | `wireu.RewriteASPath` and `wireu.RewriteASPathDual` appear outside `wireu` only in `filter_delta_test.go` and `forward_body_test.go`; `plan/journal/unwired-feature.md` records the same finding | The deletion grows into product work and belongs in its own spec | `grep -rn "wireu.RewriteASPath"` over `internal/`, `cmd/` and `pkg/` at implementation time | unvalidated |
| A-7 | The MRT dump records the received bytes rather than the collapsed WireUpdate | `processMessage` passes the original `body` as `rawBytes`, and `r.rawCapture` appends those bytes | An MRT archive would record what Ze normalized rather than what the peer sent, which is a fidelity regression an operator meets as wrong data | Reading the observer chain end to end | CONFIRMED 2026-09-08, and it is now a CONSTRAINT rather than an assumption. `Component.OnBGPMessage` (`internal/plugins/mrt/component.go`) takes a `rawBytes []byte` it never derives itself; `notifyObservers` (`reactor_notify.go`) hands every observer the `rawBytes` argument it was given, and the receive path passes `body`, the bytes read off the socket, beside the `WireUpdate` (`session_read.go`). So the collapse MUST replace the payload that storage and the relay read, and MUST NOT touch the `body` handed to the observers. The same argument feeds `r.rawCapture`, so pcap is covered by the same constraint |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The collapse allocates on every received UPDATE, turning the receive hot path into a per-message allocation | `BenchmarkCollapseAS4FastPath` reports more than zero allocations, or `./le verify deps alloc` fails its new row | The fast path returns the input slice and a false changed flag. The span-index walk uses the `presence` bitmap and the inline span array, so a normal UPDATE reaches no spill slice. The ceiling row makes the regression a red gate rather than a review question. |
| R-2 | The relabel is forgotten or partial, so a collapsed payload is parsed at two octets somewhere downstream | A forwarded AS_PATH holds garbage AS numbers, or the RIB reports a path twice as long as the one received | A-1's grep enumerates every width reader before the code lands, and the relabel is written in the same function as the collapse so neither can appear without the other. |
| R-3 | A collapsed payload is larger than the received one and no longer fits an assumption downstream | An UPDATE that split cleanly before now fails, or a size check rejects it | Widening doubles each AS number, which the forward path already paid for on every 2→4 transcode. The collapse allocates a fresh slice sized from the computed value lengths rather than writing into the pool buffer. Boundary test at the maximum message size. |
| R-4 | A peer sends an AS4_PATH longer than its AS_PATH and the collapse lengthens the path | A path-length assertion or a max-AS-path filter fires on a received route | RFC 6793 Section 4.2.3 first row: when the AS_PATH count is less than the AS4_PATH count, the AS4_PATH is IGNORED. `MergeAS4Path` implements it and the collapse MUST NOT bypass that arm. Explicit negative test. |
| R-5 | Discarding an AS4_PATH from a NEW speaker changes behavior for a peer that sends one wrongly but usefully | A route's path shortens for a peer that previously merged | This is the Section 4.1 obligation `RFC6793-4.1-7` records as unmet, and the RFC gives no discretion. The discard is conditioned on the SOURCE being four-octet, which is what "from another NEW BGP speaker" states. |
| R-6 | Deleting `TranscodeASPath`'s widening arm loses the coverage its tests carried | `TestTranscodeASPath_2to4_Aggregator` has no successor | The 2→4 cases move to the ingest collapse's own tests BEFORE the arm is deleted, not after. The 4→2 cases stay where they are, because the narrowing arm stays. |
| R-7 | The collapse lands before RFC 7606 by mistake, so Ze judges its own bytes | An RFC 7606 test asserting on a received malformed AS4_PATH changes verdict | The site is stated as after `enforceRFC7606` and before `validateUpdateFamilies`, and the wiring test asserts the order by feeding an UPDATE that is both malformed and mixed-width. |
| R-8 | An ingress policy filter returns a modified payload that reintroduces an AS4_PATH | An AS4_PATH appears in a forwarded payload on a NEW-to-NEW session | The invariant "no AS4_PATH past ingest" is documented at the collapse and asserted by a test over the filter rail's modified-payload arm. A filter that violates it is a defect in that filter, and the equal-width drop is not restored to compensate (`ai/rules/no-layering.md`). |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every received route is affected, not only a forwarded one: the RIB view, best-path selection, the looking glass, the ingress filters and both forward rails all read the collapsed payload. Wrong in the AS_PATH direction is a silently wrong path rather than a session reset, because a well-formed AS_PATH holding AS_TRANS is not malformed under RFC 4271 Section 6.3, so the receiver accepts it, runs best-path over a path whose members are wrong, and re-advertises the corruption. A wrong AS_PATH LENGTH does change route selection (RFC 4271 Section 9.1.2.2). The one shape that DOES reset a session is a malformed attribute value, which a length or offset error in the collapse would produce, and it would produce it toward every peer at once. This is a wider blast radius than the forward-path shape this spec previously carried, and the fast-path early return is what keeps a four-octet fleet outside it entirely. |
| How is it reverted? | Single commit revert. No config migration, no on-disk format. Routes already stored carry the collapsed path until they are re-advertised or withdrawn. |
| Who else touches this path? | `plan/immediate/spec-tombstone-forwarding-policy.md` edits the per-destination loop in `reactor_api_forward.go`, which this spec no longer changes. `plan/journal/unwired-feature.md` holds the standing record on `aspath_rewrite.go`. `plan/journal/gate-excludes-part-of-its-population.md` holds a live row on `LoopIngress` reading AS_PATH without AS4_PATH; the ingest site chosen here resolves that row as a consequence, and its Fix cell says so. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Session.processMessage` with an UPDATE received on a two-octet session carrying AS_TRANS in AS_PATH and the real AS in AS4_PATH | → | `processMessage` → the `wireu` collapse → the moved `attribute` rule → `MergeAS4Path` → relabelled `WireUpdate` → `onMessageReceived` | `TestReceiveCollapsesAS4PathIntoASPath` |
| `Session.processMessage` with an UPDATE received on a FOUR-octet session that wrongly carries an AS4_PATH | → | `processMessage` → the collapse's Section 4.1 discard → relabelled `WireUpdate` | `TestReceiveDiscardsAS4PathFromNewSpeaker` |
| `Session.processMessage` with an ordinary UPDATE on a four-octet session carrying no AS4_* attribute | → | `processMessage` → the collapse's fast-path return → the SAME payload slice and the SAME context ID | `TestReceiveKeepsPayloadWhenNoAS4WorkIsOwed` |
| `reactorAPIAdapter.ForwardUpdate` with a four-octet destination, over a route received on a two-octet session | → | `ReceivedUpdate` cache → `forwardUpdateCore` → `buildFwdBody` → dispatched `fwdItem` | `TestForwardUpdateCarriesReconstructedPathToNewSpeaker` |
| `reactorAPIAdapter.ForwardUpdate` with a TWO-octet destination, over a route received on a two-octet session | → | `ReceivedUpdate` cache → `forwardUpdateCore` → `fwdUpdateForDestination` → `wireu.TranscodeASPath` → dispatched `fwdItem` | `TestForwardUpdateNarrowsReconstructedPathToOldSpeaker` |
| `reactorForwardRS` with a four-octet route-server client, over a route received on a two-octet session | → | `ReceivedUpdate` cache → `reactorForwardRS` → dispatched `fwdItem` | `TestForwardRSCarriesReconstructedPathToClient` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An UPDATE received on a two-octet session, AS_PATH `[65100, 23456]`, AS4_PATH `[65100, 4200000123]` | After `processMessage`, the dispatched `WireUpdate` carries AS_PATH `[65100, 4200000123]` encoded at four octets, carries no AS4_PATH attribute, and its context ID reports ASN4 true |
| AC-2 | An UPDATE received on a two-octet session carrying AS_PATH but no AS4_PATH | The AS_PATH is widened to four octets, no AS4_PATH is invented, and the context is relabelled |
| AC-3 | An UPDATE received on a FOUR-octet session that carries an AS4_PATH or an AS4_AGGREGATOR | Both attributes are removed and neither is merged; the AS_PATH value is unchanged and the context ID is unchanged |
| AC-4 | An UPDATE received on a two-octet session whose AGGREGATOR carries a value other than AS_TRANS, beside an AS4_AGGREGATOR | The AGGREGATOR is the aggregating node, the AS4_AGGREGATOR and the AS4_PATH are both ignored, and the AS_PATH is widened rather than merged |
| AC-5 | An UPDATE received on a two-octet session whose AGGREGATOR carries AS_TRANS beside an AS4_AGGREGATOR | The AS4_AGGREGATOR is the aggregating node, the AS_PATH is merged with the AS4_PATH, and the collapsed payload carries a four-octet AGGREGATOR and no AS4_AGGREGATOR |
| AC-6 | An UPDATE whose AS4_PATH holds MORE AS numbers than its AS_PATH, received on a two-octet session | The AS4_PATH is ignored and the AS_PATH is taken as the AS path information, so the collapsed AS_PATH holds the same AS numbers the received AS_PATH held, widened to four octets |
| AC-7 | An UPDATE received on a two-octet session whose AS4_PATH carries an AS_CONFED_SEQUENCE or AS_CONFED_SET segment | Those segments are prepended only where RFC 6793 Section 4.2.3 allows, and the collapsed AS_PATH matches what `MergeAS4Path` produces for the same inputs |
| AC-8 | An UPDATE received on a two-octet session carrying a malformed AS4_PATH | The AS4_PATH is discarded, the AS_PATH is widened and taken as the AS path information, the UPDATE continues to be processed, and one log line under the session subsystem names the peer and the parse error |
| AC-9 | An UPDATE received on a four-octet session carrying no AS4_PATH and no AS4_AGGREGATOR | The collapse returns the SAME payload slice and the SAME context ID, allocates nothing, and parses no AS_PATH. `BenchmarkCollapseAS4FastPath` reports 0 allocations and its `internal/perf/allocgate.go` ceiling is 0 |
| AC-10 | The route of AC-1 forwarded to any destination whose session negotiated four octets, whether eBGP, iBGP, route-reflector client or route-server client | The dispatched bytes carry AS_PATH `[65100, 4200000123]` at four octets plus any eBGP prepend, and no AS4_PATH attribute |
| AC-11 | The route of AC-1 forwarded to a destination whose session negotiated two octets | The dispatched bytes carry a two-octet AS_PATH with AS_TRANS for each non-mappable AS, and an AS4_PATH derived from the outgoing path; a mappable-only path carries no AS4_PATH |
| AC-12 | An AGGREGATOR carrying a non-mappable AS, forwarded to a two-octet destination | AGGREGATOR carries AS_TRANS and an AS4_AGGREGATOR carries the real AS; a mappable AGGREGATOR gets neither |
| AC-13 | An ingress filter reads the AS_PATH of a route received on a two-octet session from an OLD speaker whose path holds Ze's own four-octet AS | `LoopIngress` sees the real AS number rather than AS_TRANS and rejects the route as a loop, and `PeerFilterInfo.ASN4` reports true |
| AC-14 | A grep over `internal/`, `cmd/` and `pkg/` for `RewriteASPath` and `RewriteASPathDual` after the change | No match, and `internal/component/bgp/wireu/aspath_rewrite.go` does not exist |
| AC-15 | A grep over `internal/` for the Section 4.2.3 construction after the change | Exactly one declaration, in `internal/core/bgp/attribute`, and `internal/component/bgp/plugins/rib/storage/attrparse.go` holds no copy of `canonicalizeASPath`, `reconstructASPath`, `widenASPath`, `expandASPath2to4`, `selectAggregator` or `aggregatorASIsTrans` |
| AC-16 | `INTEROP_SCENARIO=as-path-mixed-width-relay-frr ./le integration interop` | FRR reports the prefix with the real four-octet AS in its AS path and does not report AS 23456 |
| AC-17 | The UPDATE of AC-1, with a message observer registered (`mrt`) and raw capture enabled | The observer and the capture each receive the bytes the peer sent, AS_PATH two-octet with 23456 and the AS4_PATH still present, while the dispatched `WireUpdate` carries the collapsed payload. An archive records the wire, never Ze's normalization |
| AC-18 | A route received from an OLD speaker, stored in `adj_rib_in`, then relayed through `buildRelayUpdate` | The relay's `WireUpdate` carries a context reporting ASN4 true, and every egress filter behind it parses the reconstructed AS_PATH at four octets. A two-octet label on those bytes is the failure this row exists to catch |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a legacy router that does not support four-octet AS numbers, and relays its routes to a modern iBGP core | wire → session read → `processMessage` collapse → `ReceivedUpdate` → `ForwardUpdate` → `buildFwdBody` → destination session write | `TestReceiveCollapsesAS4PathIntoASPath`, `TestForwardUpdateCarriesReconstructedPathToNewSpeaker`, and the interop scenario `as-path-mixed-width-relay-frr` |
| 2 | Reads `show bgp` for a route learned from that legacy router | wire → `processMessage` collapse → RIB parse at `ctx.ASN4()` → route view | `TestRIBStoresReconstructedPathFromCollapsedPayload` |
| 3 | Runs Ze as a route server between clients of mixed four-octet support | wire → `processMessage` collapse → `reactorForwardRS` → client session write | `TestForwardRSCarriesReconstructedPathToClient`, `TestForwardUpdateNarrowsReconstructedPathToOldSpeaker` |
| 4 | Peers with a router that wrongly sends AS4_PATH on a four-octet session | wire → `processMessage` collapse discard → every consumer | `TestReceiveDiscardsAS4PathFromNewSpeaker` |
| 5 | Runs a fleet of four-octet speakers and pays nothing for the transition machinery | wire → `processMessage` fast-path return → unchanged payload | `TestReceiveKeepsPayloadWhenNoAS4WorkIsOwed`, `BenchmarkCollapseAS4FastPath` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReceiveCollapsesAS4PathIntoASPath` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-1, entry point `processMessage` | PASS |
| `TestReceiveWidensASPathWithoutAS4Path` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-2 | |
| `TestReceiveDiscardsAS4PathFromNewSpeaker` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-3, the RFC 6793 Section 4.1 receive obligation | PASS |
| `TestReceiveKeepsPayloadWhenNoAS4WorkIsOwed` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-9, asserting the returned slice and the context ID are the SAME values, not merely equal | PASS |
| `TestReceiveCollapseRunsAfterRFC7606` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | R-7, an UPDATE that is both malformed and mixed-width | PASS |
| `TestReceiveCollapseLogsDiscardedMalformedAS4Path` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-8, both the log line and the continued processing | PASS |
| `TestCollapseAS4SelectsAggregatorPerSection423` | `internal/component/bgp/wireu/aspath_collapse_test.go` | AC-4 and AC-5, both arms of the AGGREGATOR gate | PASS |
| `TestCollapseAS4IgnoresOversizedAS4Path` | `internal/component/bgp/wireu/aspath_collapse_test.go` | AC-6, the RFC 6793 Section 4.2.3 count comparison | PASS |
| `TestCollapseAS4AppliesConfedAdjacencyRule` | `internal/component/bgp/wireu/aspath_collapse_test.go` | AC-7 | PASS |
| `TestCollapseAS4RewritesAggregatorToFourOctets` | `internal/component/bgp/wireu/aspath_collapse_test.go` | AC-5's AGGREGATOR half and the 2→4 cases moving off `aspath_transcode_test.go` (R-6) | PASS |
| `FuzzCollapseAS4` | `internal/component/bgp/wireu/aspath_collapse_test.go` | The collapse never panics on peer-supplied input, replacing `FuzzRewriteASPath`'s coverage | PASS, 2.7M execs clean |
| `TestCanonicalASPathMatchesMergeAS4Path` | `internal/core/bgp/attribute/as4_test.go` | AC-15, the moved rule producing what `MergeAS4Path` produces for the same inputs | |
| `TestRIBStoresReconstructedPathFromCollapsedPayload` | `internal/component/bgp/plugins/rib/storage/attrparse_test.go` | Story 2, the RIB reading a collapsed payload at `ctx.ASN4()` true | |
| `TestForwardUpdateCarriesReconstructedPathToNewSpeaker` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-10, entry point `ForwardUpdate`, eBGP and iBGP and RR-client as subtests | PASS |
| `TestForwardUpdateNarrowsReconstructedPathToOldSpeaker` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-11, both the non-mappable and mappable-only arms | PASS |
| `TestForwardRSCarriesReconstructedPathToClient` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-10, entry point `reactorForwardRS` | PASS |
| `TestForwardUpdateNarrowsAggregatorToOldSpeaker` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-12 | |
| `TestLoopIngressSeesReconstructedASPath` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-13, and the `plan/journal/gate-excludes-part-of-its-population.md` row | |
| `TestEncodingContextIDDiffersOnASN4Alone` | `internal/core/bgp/context/registry_test.go` | A-2 | PASS |
| `TestReceivedBytesReachTheObserversUncollapsed` | `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` | AC-17. It drives `processMessage` and asserts the `rawBytes` argument is the received slice ITSELF, still two-octet and still carrying attribute 17, while the dispatched `WireUpdate` carries neither. `notifyObservers` (`reactor_notify.go`) hands every observer and `r.rawCapture` that same argument, so the assertion covers the observer view at the site the collapse touches | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AS number mappability | 0 - 4294967295 | 65535 is the last mappable value; 65536 is the first that becomes AS_TRANS on egress | N/A | N/A |
| AS_PATH segment AS count | 1 - 255 | 255 per segment (`attribute.MaxASPathSegmentLength`); a reconstruction crossing it splits into a second segment | 0 is a malformed segment | 256 cannot be encoded |
| AS_PATH attribute value length | 0 - 65535 | 255 is the last value taking a 3-octet header; 256 takes a 4-octet header | N/A | 65535 is the wire ceiling |
| Collapsed UPDATE body size | 0 - 4096, or 65535 with RFC 8654 | 4076 for a standard peer, 65516 with extended message | N/A | One octet over is refused by the collapse rather than truncated |
| AS4_PATH AS count against AS_PATH AS count | equal or fewer means reconstruct; more means ignore | equal counts reconstruct | N/A | one more AS number in AS4_PATH ignores it |
| AS4_AGGREGATOR value length | 8 octets exactly | 8 | 7 is malformed and discarded | 9 is malformed and discarded |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc6793-ingest-collapse` | `test/decode/rfc6793-ingest-collapse.ci` | An operator receives a route from a two-octet peer and reads the AS path Ze holds, asserted as hex, with no AS4_PATH present | |
| `rfc6793-no-as4path-from-new-speaker` | `test/decode/rfc6793-no-as4path-from-new-speaker.ci` | An operator receives an UPDATE that wrongly carries AS4_PATH on a four-octet session, and the attribute is absent from everything Ze holds and sends | |
| `rfc6793-narrow-to-old-speaker` | `test/decode/rfc6793-narrow-to-old-speaker.ci` | An operator relays that route to a two-octet peer and reads the AS_TRANS AS_PATH and the derived AS4_PATH Ze put on the wire, asserted as hex | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `as-path-mixed-width-relay-frr` | `test/interop/scenarios/as-path-mixed-width-relay-frr/` | FRR | The mixed-width relay end to end, judged by a second implementation. The raw injector at 172.30.0.9 holds a session Ze pins to two octets (`session { capability { asn4 false; } }`) and announces a prefix whose AS_PATH carries AS_TRANS and whose AS4_PATH carries a non-mappable AS. FRR at 172.30.0.3 negotiates four octets and its own route view must show the real four-octet AS and must NOT show 23456, which only a peer daemon can judge. Two FRR-side facts are reused from `test/interop/scenarios/as-path-prepend-two-octet-peer/frr.conf`, both measured on 2026-09-08: `dont-capability-negotiate` suppresses only what FRR SENDS, so `remote-as` must carry the real four-octet ASN or FRR answers every OPEN with "2/2 (OPEN Message Error/Bad Peer AS)"; and `enforce-first-as` must be off per neighbor, because it compares the leading AS of the received AS_PATH against the real ASN and the per-neighbor form is the only one FRR 10.3 parses. | |

## Files to Modify
- `internal/component/bgp/reactor/session_read.go` - `processMessage` calls the collapse after `enforceRFC7606` and before `validateUpdateFamilies`, replaces `wireUpdate` with one built over the collapsed payload at `fwdContextIDWithASN4(recvCtxID, true)`, and logs a discarded attribute under the session subsystem
- `internal/component/bgp/reactor/reactor_notify.go` - `src.ASN4` is read from the WireUpdate's encoding context rather than `peer.session.Negotiated().ASN4`, and the comment explaining the unlocked read is rewritten because the read is gone
- `internal/component/bgp/reactor/reactor_api_relay.go` - `resolveRelaySource` stamps `srcPeer.recvContextID()` onto the wire `buildRelayUpdate` reconstructs from stored `adj_rib_in` bytes. Those bytes are collapsed four-octet ones, so the label becomes `fwdContextIDWithASN4(srcPeer.recvContextID(), true)`, and the two comments in the file asserting the stored bytes are still in the source peer's encoding are corrected in the same edit (`ai/rules/stale-comments.md`). Found by A-1's enumeration on 2026-09-08; without it the relay hands every egress filter a four-octet path labelled two-octet
- `internal/component/bgp/wireu/aspath_as4.go` - gains the payload-level collapse, or a sibling file beside it, calling the moved rule; `AS4PathForRewrite` loses its `MergeAS4Path` branch and `joinSequences` goes with it, subject to A-4
- `internal/component/bgp/wireu/aspath_transcode.go` - the 2→4 widening arm is deleted; the 4→2 narrowing arm stays and is now the only direction any caller can reach
- `internal/component/bgp/wireu/aspath_slot.go` - `recordWithdrawOnly` and its arm of the `Record` dispatch are deleted, because the equal-width AS4_PATH drop cannot fire on a payload that reached the forward path
- `internal/core/bgp/attribute/as4.go` - receives the moved Section 4.2.3 rule beside `MergeAS4Path`, exported, returning the canonical AS_PATH value, the canonical AGGREGATOR value and the discard reason
- `internal/component/bgp/plugins/rib/storage/attrparse.go` - loses its six unexported copies, and does NOT call the moved rule: post-collapse it reads a canonical four-octet AS_PATH, so its `asn4` parameter is constant and goes with them (A-5). `familyrib.go` and `attrFingerprint` follow the same deletion
- `internal/perf/allocgate.go` - the ceiling rows for the two new benchmarks, the fast path at 0
- `internal/component/bgp/wireu/aspath_transcode_test.go` - the 2→4 cases move to the collapse's tests before the arm is deleted
- `internal/component/bgp/reactor/forward_body_test.go` - `TestForwardDoesNotRetranscodeASN2RewrittenWire` builds its fixture with `RewriteASPath` and must build it another way
- `internal/component/bgp/reactor/filter_delta_test.go` - the same fixture change
- `internal/component/bgp/wireu/tombstone_test.go`, `internal/component/bgp/wireu/tombstone_forward_test.go`, `internal/component/bgp/wireu/wire_bench_test.go`, `internal/component/bgp/wireu/aspath_aggregator_probe_test.go`, `internal/component/bgp/wireu/rfc6793_as4_test.go` - each drives `RewriteASPath` or `RewriteASPathDual` and is re-expressed against `ASPathEdit.Record` or the collapse before the deletion lands
- `rfc/short/rfc6793.md` - `RFC6793-4.1-6` and `RFC6793-4.1-7` lose their `{gap}` and their stale `aspath_rewrite.go` citations; `RFC6793-4.2.3-8`, `-9` and `-10` re-point to the moved rule and gain ingest-path test bindings; the `## Meta` `Support remaining` row loses two of its four gaps and `Support coverage` names the ingest collapse
- `docs/architecture/edge-cases/as4.md` - the reconstruction is a daemon-wide ingest step rather than a RIB-storage one; the "What it costs" section states the fast-path contract
- `docs/architecture/wire/attributes.md` - the AS_PATH and AS4_PATH sections state that no AS4_PATH survives ingest, so an egress AS4_PATH is always derived rather than relayed
- `docs/architecture/encoding-context.md` - the receive-side ASN4 relabel and its owner
- `docs/architecture/behavior/fsm-established.md` - the collapse's position in the `processMessage` sequence
- `docs/architecture/core-design.md` - declared by `reactor_notify.go`, `forward_context.go`, `filter_delta.go` and `filter/loop.go`: the received-UPDATE dispatch sequence gains the collapse ahead of the ingress filters, the context derivation gains a receive-side caller, and the loop filter reads a reconstructed AS_PATH
- `docs/functional-tests.md` - declared by `internal/perf/allocgate.go`: read first, and edited if it enumerates the registered allocation-ceiling benchmarks, because this spec adds two
- `docs/architecture/plugin/rib-storage-design.md` - the RIB is a caller of the rule, not its owner
- `docs/architecture/testing/interop.md` - the Scenario Inventory row for `as-path-mixed-width-relay-frr`
- `docs/features/rfc-status.md` - regenerated from the `rfc/short/` rows
- `internal/le/interoplab/bgp/checkers.go` - the new scenario's ordered assertions in `scenarioOperations`
- `internal/le/interoplab/bgp/names.go` - the prefix and AS-number constants the new assertions read
- `plan/journal/unwired-feature.md` - the 2026-09-05 row's Fix cell records that the `aspath_rewrite.go` deletion happened and names this spec
- `plan/journal/gate-excludes-part-of-its-population.md` - the `LoopIngress` row's Fix cell records that the ingest site resolves it and names this spec

## Files to Create
- `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go` - the entry-point tests for AC-1 through AC-3, AC-8 through AC-13
- `internal/component/bgp/wireu/aspath_collapse.go` - the payload-level collapse, if it does not land in `aspath_as4.go`
- `internal/component/bgp/wireu/aspath_collapse_test.go` - the value-level and payload-level tests, including the fuzz target and the two benchmarks
- `test/decode/rfc6793-ingest-collapse.ci` - the functional test for story 1
- `test/decode/rfc6793-no-as4path-from-new-speaker.ci` - the functional test for story 4
- `test/decode/rfc6793-narrow-to-old-speaker.ci` - the functional test for story 3
- `test/interop/scenarios/as-path-mixed-width-relay-frr/ze.conf` - Ze between a two-octet injector and a four-octet FRR
- `test/interop/scenarios/as-path-mixed-width-relay-frr/frr.conf` - FRR as the NEW speaker, with `remote-as` carrying the real four-octet ASN and `enforce-first-as` off per neighbor
- `test/interop/scenarios/as-path-mixed-width-relay-frr/inject.msg` - the raw UPDATE carrying AS_TRANS in AS_PATH and the real AS in AS4_PATH
- `test/interop/scenarios/as-path-mixed-width-relay-frr/inject-args` - the injector's AS

## Files to Delete
- `internal/component/bgp/wireu/aspath_rewrite.go` and `internal/component/bgp/wireu/aspath_rewrite_test.go` - `RewriteASPath` and `RewriteASPathDual` are the only exported entry points and neither has a non-test caller. Evidence: a grep for both names over `internal/`, `cmd/` and `pkg/` returns their two declarations, their doc references inside `wireu`, and `_test.go` files only. `ASPathEdit.Record` replaced them, its own file header says so twice, and `plan/journal/unwired-feature.md` recorded the same finding on 2026-09-05. `ai/rules/no-layering.md`: X was never deleted after Y was written. The test fixtures that drive them are re-expressed FIRST, in the same change

**Not deleted, and the previous shape was wrong about it.**
`internal/component/bgp/wireu/aspath_transcode.go` stays. `TranscodeASPath` has
exactly one non-test caller, `internal/component/bgp/reactor/forward_body.go`,
and under this shape it becomes the narrowing encoder every non-eBGP two-octet
destination takes. Only its 2→4 widening arm dies, because no payload reaching
it can be two-octet once the collapse has run. The file's 4→2 tests stay where
they are.

Two smaller deletions are named in Files to Modify rather than here, because
each is a function inside a surviving file: `recordWithdrawOnly`
(`aspath_slot.go`), whose only reason to exist is a drop that can no longer
fire, and `joinSequences` with the `MergeAS4Path` branch of `AS4PathForRewrite`
(`aspath_as4.go`), which are NOT deleted: A-4 was resolved on 2026-09-08 and
broke in the safe direction. `exportFilterForBody` still hands the extractor a
two-octet body for a two-octet destination, and the operator dry-run still takes
a width of its own, so the merged branch keeps a caller. What it loses is every
FORWARDING caller, because no originated body carries an AS4_PATH and nothing
else survives ingest with one. `recordWithdrawOnly` still goes.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf is added. The behavior is unconditional RFC conformance, and `ai/rules/simplicity.md` forbids an option nobody asked for. |
| YANG validation constraints | N-A | No new leaf. |
| YANG custom validators | N-A | No new leaf. |
| CLI commands/flags | N-A | No command changes. The behavior is observable through `show bgp` output that already exists. |
| CLI grammar (keyword before value) | N-A | No command added. |
| Editor autocomplete | N-A | No leaf added. |
| Functional test for new RPC/API | Yes | `test/decode/rfc6793-ingest-collapse.ci`, `test/decode/rfc6793-no-as4path-from-new-speaker.ci`, `test/decode/rfc6793-narrow-to-old-speaker.ci` |
| Pipe completeness | N-A | No command output added. |
| Env var registration | N-A | No env var added. |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, kernel module, binary or certificate is added. |
| Prometheus counters/metrics | No | A per-UPDATE "AS path collapsed" counter would sit on the receive hot path with no operator question behind it. The allocation ceiling in `internal/perf/allocgate.go` is what watches this path, and it is a gate rather than a metric. |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability or attribute code. AS_PATH (2), AGGREGATOR (7), AS4_PATH (17) and AS4_AGGREGATOR (18) all exist and are registered. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | This is a conformance repair to an existing behavior; `docs/features.md` already claims RFC 6793 support. |
| 2 | Config syntax changed? | No | No leaf added or renamed. |
| 3 | CLI command added/changed? | No | No command touched. |
| 4 | API/RPC added/changed? | No | `ForwardUpdate`'s signature and contract are unchanged, and so is `onMessageReceived`'s. |
| 5 | Plugin added/changed? | No | No plugin surface changes. The RIB plugin loses six unexported functions and gains one import. |
| 6 | Has a user guide page? | Yes | `docs/architecture/edge-cases/as4.md` is the AS4 page `ai/INDEX.md` routes "ASN4, AS4" to, and it states the Section 4.2.3 reconstruction as RIB-owned. |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md`, the AS_PATH and AS4_PATH sections. |
| 8 | Plugin SDK/protocol changed? | No | No SDK type or event changes. A plugin receiving an UPDATE now receives the collapsed payload, which is a value change rather than a contract change, and row 6's page states it. |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc6793.md` (`RFC6793-4.1-6`, `RFC6793-4.1-7`, `RFC6793-4.2.3-8`, `-9`, `-10`, and the `## Meta` rows), and the regenerated `docs/features/rfc-status.md`. |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` Scenario Inventory gains `as-path-mixed-width-relay-frr`. |
| 11 | Affects daemon comparison? | No | The comparison table claims RFC 6793 support, which stays true and becomes better proven. |
| 12 | Internal architecture changed? | Yes | `docs/architecture/encoding-context.md` (the receive-side relabel), `docs/architecture/behavior/fsm-established.md` (the collapse's position in `processMessage`), `docs/architecture/plugin/rib-storage-design.md` (the RIB is a caller, not the owner). |
| 13 | Route metadata keys added/changed? | No | No metadata key touched. |
| 14 | Prometheus counters added/changed? | No | No counter added. |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entry changes. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/immediate/spec-forwarded-as-path-obeys-rfc6793-for-every-destination.md` must be run at implementation time and its output reconciled here. Known already from `ai/CODE-TO-DOCS.md`: `attrparse.go` declares `docs/architecture/edge-cases/as4.md`, whose `<!-- source: -->` anchors name `selectAggregator`, `canonicalizeASPath`, `MergeAS4Path`, `appendLeadingSegments` and `countASNs`, and the first two move package, so those anchors move with them. `as4.go` declares four pages, of which `docs/architecture/edge-cases/as4.md` and `docs/architecture/wire/attributes.md` change and `docs/architecture/api/process-protocol.md` and `docs/architecture/bgp/filter-path-asn.md` are read before being called unaffected. `session_read.go` declares five pages, of which `docs/architecture/behavior/fsm-established.md` changes; the other four are read for an AS_PATH claim before that verdict is accepted. `docs/architecture/core-design.md` is declared by `reactor_notify.go` ("peer lifecycle events and message receiver dispatch"), `forward_context.go` ("egress BGP context derivation"), `filter_delta.go` and `internal/component/bgp/reactor/filter/loop.go` ("route loop detection ingress filter"), and it DOES change: the received-UPDATE sequence it describes gains the collapse ahead of the ingress filters, the context derivation it describes gains a receive-side caller, and the loop filter it describes now reads a reconstructed AS_PATH (AC-13). `docs/functional-tests.md` is declared by `internal/perf/allocgate.go` ("allocation-ceiling verification") and is read before the verdict: it changes if it enumerates the registered benchmarks, because two rows are added, and is otherwise unaffected. `forward_body.go` declares `docs/architecture/bgp/structural-forwarding.md`, which keeps its ASN4 transcode step because `TranscodeASPath` survives. `aspath_rewrite.go` declares `docs/architecture/wire/attributes.md` and is DELETED, so that page's anchors into it are removed rather than re-pointed. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/edge-cases/as4.md` carries a worked AS_PATH and AS4_PATH example and a reconstruction walk-through; both are verified against the ingest behavior and the "Reconstruction Algorithm" section states where it runs. |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove the entry point reaches the collapse, with the tests RED
   - Tests: the six Wiring Test rows, the first three written against `Session.processMessage` and the last three against `adapter.ForwardUpdate` and `reactorForwardRS`, asserting on the dispatched `WireUpdate` and the dispatched `fwdItem` bytes
   - Files: `internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go`
   - Verify: `TestReceiveCollapsesAS4PathIntoASPath` fails because AS_PATH holds 23456; `TestReceiveDiscardsAS4PathFromNewSpeaker` fails because attribute 17 survives. Record both reds. Validate A-1 here with the grep it names, and A-7 by reading the MRT handler.
2. **Phase: Move the rule** - one declaration, in a package both callers may import
   - Tests: `TestCanonicalASPathMatchesMergeAS4Path`, and the existing `attrparse` tests re-pointed at the moved names
   - Files: `internal/core/bgp/attribute/as4.go`, `internal/component/bgp/plugins/rib/storage/attrparse.go`
   - Verify: `./le tier check` clean; AC-15's grep shows one declaration; the RIB's `bgp.rib` log line still fires for a malformed AS4_PATH, now from the returned discard reason
3. **Phase: The payload collapse** - the fast path first
   - Tests: `TestCollapseAS4*`, `FuzzCollapseAS4`, `BenchmarkCollapseAS4FastPath`
   - Files: `internal/component/bgp/wireu/aspath_collapse.go`, `internal/perf/allocgate.go`
   - Verify: the fast path returns the input slice itself and allocates nothing; `./le verify deps alloc` green with the new 0 ceiling; each arm of the Section 4.2.3 gate has a test that fails when that arm is removed
4. **Phase: The ingest site** - `processMessage`, with the relabel
   - Tests: Wiring rows 1 to 3, `TestReceiveCollapseRunsAfterRFC7606`, `TestReceiveCollapseLogsDiscardedMalformedAS4Path`, `TestEncodingContextIDDiffersOnASN4Alone`
   - Files: `internal/component/bgp/reactor/session_read.go`, `internal/component/bgp/reactor/reactor_notify.go`
   - Verify: the collapsed payload carries a context reporting ASN4 true; `src.ASN4` reads from that context; `TestLoopIngressSeesReconstructedASPath` and `TestRIBStoresReconstructedPathFromCollapsedPayload` pass with no change to the loop filter or the RIB
5. **Phase: The forward path, unchanged and now correct** - prove it rather than edit it
   - Tests: Wiring rows 4 to 6, `TestForwardUpdateNarrowsAggregatorToOldSpeaker`, and the existing forward suite
   - Files: none expected. A diff here means the collapse did not do its job.
   - Verify: `Record` is still inside the `isEBGP` guard on both rails; the existing forward suite stays green; the four-octet destination takes the same branch it took before
6. **Phase: Delete what the shape killed** - `ai/rules/no-layering.md`
   - Tests: every fixture driving `RewriteASPath` or `RewriteASPathDual` is re-expressed first; the 2→4 transcode cases move to `aspath_collapse_test.go` first
   - Files: delete `aspath_rewrite.go` and `aspath_rewrite_test.go`; edit `aspath_transcode.go`, `aspath_transcode_test.go`, `aspath_slot.go`, `aspath_as4.go`, `forward_body_test.go`, `filter_delta_test.go`, `tombstone_test.go`, `tombstone_forward_test.go`, `wire_bench_test.go`, `aspath_aggregator_probe_test.go`, `rfc6793_as4_test.go`
   - Verify: AC-14's grep; `joinSequences` and the `MergeAS4Path` branch STAY (A-4), and the doc comment of `AS4PathForRewrite` is corrected in this phase because it explains itself in terms of an OLD-speaker source the forwarding rails no longer have; `./le verify lint run` clean
7. **Phase: Functional and interop** - a user reaches the behavior
   - Tests: the three `.ci` files, then the interop scenario with its discrimination walk
   - Files: `test/decode/*.ci`, the scenario directory, `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/names.go`
   - Verify: revert phase 3, rebuild the Ze container, watch FRR report AS 23456 and the scenario go RED, restore, watch it go GREEN, and record the red
8. **Phase: Docs and the RFC ledger** - the pages and rows this change makes wrong
   - Tests: `./le docs-to-code index-check`, `./le doc check links`, `./le rfc check`
   - Files: the seven pages and the two ledger files in Files to Modify, plus the two journal rows
   - Verify: the two `{gap}` annotations are gone AND their replacement text names a producer that exists; the anchors that moved package point at the new one

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-1 through AC-3 and AC-8 through AC-13 each have a test whose entry point is `processMessage`, `ForwardUpdate` or `reactorForwardRS`, never the collapse helper alone |
| Feature completeness | The collapse is reached from the receive path and no consumer bypasses it. A diff that changes a forward rail means the ingest site was wrong |
| Correctness | The collapse runs AFTER RFC 7606 and BEFORE the ingress filter chain, and the relabel is in the same function as the rewrite |
| Correctness | The Section 4.1 discard is conditioned on the SOURCE session being four-octet, which is what "from another NEW BGP speaker" states, and never on the destination |
| Correctness | The RFC 6793 Section 4.2.3 first arm is present: an AS4_PATH holding more AS numbers than the AS_PATH is IGNORED, not merged |
| Performance | The four-octet fast path returns the input slice and the input context ID, and `BenchmarkCollapseAS4FastPath` carries a 0 ceiling in `internal/perf/allocgate.go` |
| Naming | The interop scenario directory carries no numeric prefix and names the behavior, and the same string appears in the directory, `scenarioOperations` and every citation |
| Data flow | After the change, exactly one function performs the Section 4.2.3 construction and exactly one performs the payload collapse. Grep proves it |
| Rule: `ai/rules/no-layering.md` | `aspath_rewrite.go` is deleted in the same change that lands the collapse, and the 2→4 transcode arm goes with it |
| Rule: `ai/rules/architecture.md` | The reactor does not import `internal/component/bgp/plugins/rib/storage`, and `./le tier check` is green |
| Rule: `ai/rules/rfc-compliance.md` | Every MUST and MUST NOT enforced in the new code carries the quoted RFC sentence above it, read from `rfc/full/rfc6793.txt` rather than from `rfc/short/` |
| Rule: `ai/rules/evidence.md` | The rewritten `{gap}` annotations in `rfc/short/rfc6793.md` name producers that exist and are reachable; the current ones name a file with no non-test caller |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| One collapse on the receive path | `grep -rn "TranscodeASPath\|RewriteASPath" internal/ cmd/ pkg/` returns only the surviving `TranscodeASPath` narrowing caller |
| One declaration of the Section 4.2.3 construction | `grep -rn "canonicalizeASPath\|reconstructASPath\|selectAggregator" internal/` names `internal/core/bgp/attribute` only |
| Tier direction legal | `./le tier check` |
| Entry-point tests, not helper tests | Each new test body in `rfc6793_ingest_collapse_test.go` contains `processMessage(`, `ForwardUpdate(` or `reactorForwardRS(` |
| The fast path costs nothing | `./le verify deps alloc` green with `BenchmarkCollapseAS4FastPath` at 0 |
| Interop scenario registered | `go test ./internal/le/interoplab/bgp/ -run TestCheckerPopulationMatchesProducer` |
| Interop scenario discriminates | `./le rfc discriminate-record` output, or the recorded RED from the revert-and-rebuild walk |
| RFC ledger updated | `./le rfc check`, and `grep -n "RFC6793-4.1-6" rfc/short/rfc6793.md` shows no `{gap}` |
| Docs updated in the same work | `./le spec citation anchors spec plan/immediate/spec-forwarded-as-path-obeys-rfc6793-for-every-destination.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | AS_PATH, AS4_PATH, AGGREGATOR and AS4_AGGREGATOR are peer-supplied and the collapse is the FIRST code to rewrite them. Every length, segment count and segment type is checked before it is read, and a malformed value produces a discard the caller logs, never a panic. `attribute.ParseASPath`, `attribute.ParseAS4Path` and `attribute.BuildSpanIndex` own that, and the collapse must not bypass them. The fuzz target is not optional: this code runs on every received UPDATE from every peer. |
| Resource exhaustion | An oversized AS4_PATH must not lengthen the stored path: the RFC 6793 Section 4.2.3 count comparison is the bound and R-4's test is the proof. Without it a peer sending a 255-segment AS4_PATH inflates every path Ze holds AND relays, which is a wider reach than the forward-path shape had. |
| Buffer bounds | The collapse writes into a slice it sized from the computed value lengths, never into the pool read buffer, which is shorter than the widened payload. A body that would exceed the wire ceiling is refused rather than truncated. |
| Error leakage | The discard log names the peer and the parse error, which is operator-facing and carries no key material. |
| Fail closed | A collapse that cannot run refuses the UPDATE rather than dispatching a payload whose AS path is half-rewritten. A partially rewritten payload MUST NOT reach any consumer. |
| Denial of service | The collapse runs on the session read goroutine, so its cost is charged to the peer that caused it. The fast path is an early return, so a hostile peer cannot make a four-octet fleet pay for the transition machinery. |

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

- The commissioning premise was that a non-eBGP destination receives AS_PATH at the source's width. It does not. `buildFwdBody` re-encodes for any destination whose context ID differs from the source's, and a context ID folds the peer AS, so almost every forward takes that branch. The lesson: a guard that looks like it gates a behavior may only gate WHICH of two implementations performs it, and the difference between the two is where the defect lives.
- The first design fixed the defect on the forward path, where it was OBSERVED. The owner moved it to ingest, where it is CAUSED. Fixing at the observation point needed a widening arm in two encoders, a rail unification and a per-destination cost; fixing at the cause needs one function, called once, and both encoders become correct without being edited. The tell that a fix is at the wrong layer is that it has to be written twice.
- `wireu.TranscodeASPath`'s own doc comment names the gap ("does NOT merge an existing AS4_PATH") and routes the repair to a surface that does not perform it ("that merge is done at ingress by the receiving session"). The comment was not wrong about WHERE the repair belongs. It was wrong that Ze had one. This spec makes the comment true.
- The RIB has performed the correct reconstruction since before this spec existed, and it was invisible because it produced the STORED route rather than the RELAYED bytes. One rule, two consumers, one of them served: that is what "declare every fact once" prevents, and the copy that was served is the one that made the gap hard to see.
- Three defects on three surfaces turned out to be one defect at one site. `LoopIngress` reading AS_PATH without AS4_PATH (`plan/journal/gate-excludes-part-of-its-population.md`), the forward path relaying AS_TRANS, and the RIB's merge running for a NEW speaker are all resolved by the same collapse, purely because it sits upstream of all three.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Collapse the AS-path family once at ingest | Add a Section 4.2.3 widening arm to both egress encoders and unify the two forward rails | Owner decision, 2026-09-08. FRR (`aspath_reconcile_as4` from `bgp_attr_parse`) and BIRD (`bgp_process_as4_attrs`) both do exactly this, which is why neither needs a Section 4.2.3 step on its forward path. Regenerating at encode is cheaper than storing a second copy, and a four-octet fleet then pays nothing. |
| The site is `Session.processMessage`, before the import policy chain | Inside `notifyMessageReceiver` before the ingress loop | `notifyMessageReceiver` returns early when no message receiver is registered, so the normalization would depend on a plugin being present. `processMessage` is upstream of the filters, the cache, the RIB and both forward rails, already owns the payload-replacement shape, and matches where FRR and BIRD put it. |
| The rule moves to `internal/core/bgp/attribute` | Leave it in `rib/storage` and call it from the reactor; or copy it into `wireu` | The reactor is a component and `rib/storage` is a plugin, so the import is illegal and `./le tier check` refuses it. Copying is what `ai/rules/principles.md` bans. `attribute` already holds `MergeAS4Path`, imports only `internal/core/*`, and is already imported by both callers. |
| The moved rule reports its discards instead of logging them | Give the `attribute` package a logger | A leaf on the parse path should not take a logging dependency, and the two callers write under different subsystems (`bgp.rib` and the session). Returning the reason lets each log correctly and lets a test assert the reason without capturing output. |
| The collapse returns the input slice unchanged when nothing is owed | Always return a fresh payload | The fast path is the whole point of the owner's shape. Returning the same slice makes "no AS-path work" checkable by identity and by a 0-allocation ceiling, rather than by reading the code. |
| `Record` stays inside the `isEBGP` guard | Lift it, as the previous shape required | With the collapse at ingest both rails are already correct, so lifting it would be machinery the problem no longer needs (`ai/rules/simplicity.md`). The rail split is then a duplication question, not a correctness one, and it is named in Known Limitations. |
| No config option to disable the collapse | A `capability asn4-reconstruct` leaf | `ai/rules/simplicity.md`: an option nobody asked for is a permanent branch, a schema entry, a doc line and a test matrix. Conformance is not optional (`ai/rules/rfc-compliance.md`). |
| One interop scenario, plus three `.ci` files | Two interop scenarios, one per direction | The mixed-width relay is one behavior with one observable outcome in FRR's route view. The absences (no AS4_PATH between NEW speakers, no AS4_PATH for a mappable-only path) are not reportable by a peer daemon, so a `.ci` with a hex assertion proves them directly. |

## Known Limitations

- Two encoders still narrow a four-octet path for a two-octet destination: `ASPathEdit.recordTranscode` for an eBGP destination and `wireu.TranscodeASPath` for every other one. Both are CORRECT after this change and both derive their AS4_PATH from the one owner, `as4PathForPath`, so this is duplication rather than divergence. Removing it means lifting `Record` out of the `isEBGP` guard, which the owner's shape did not commission and which this spec deliberately does not do. It needs its own spec.
- `RFC6793-6-5` (a malformed AS4_AGGREGATOR is not discarded on every path) is narrowed but not closed. The collapse discards a malformed one at ingest, so no relayed payload can carry it, but no RFC 7606 validator is registered for attribute code 18 and `ParseAS4Aggregator`'s only production reachability still returns the error to its caller rather than discarding and continuing. That half is a different surface and needs its own spec.
- The RIB commit rail (`internal/component/bgp/rib/commit.go`, `packAttributesWithASPath`) is a separate place AS_PATH is written, for routes Ze ORIGINATES rather than relays. It is out of scope: an originated route has no source AS4_PATH to reconstruct from, and its two-octet encoding is already the Section 4.2.2 narrowing this spec leaves alone.
- An MRT archive and a pcap export both keep recording what the peer sent, and that is a constraint this spec carries rather than an open question (A-7, confirmed). The observers are handed the socket's own `body`; the collapse replaces the payload that storage and the relay read. An implementation that collapses `body` in place would make every archive record Ze's normalization instead, which is why the ACs assert on the observer's bytes as well as on the stored ones.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. Read
each sentence from `rfc/full/rfc6793.txt` and `rfc/full/rfc7947.txt` before
quoting it; `rfc/short/` is a derived artifact and is never the authority.

| Requirement | Section | Where it is enforced |
|-------------|---------|----------------------|
| "The new attributes, AS4_PATH and AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers." | RFC 6793 Section 4.1 | The collapse removing both attributes at ingest, which leaves nothing for any forward rail to carry |
| "A NEW BGP speaker that receives the AS4_PATH attribute or the AS4_AGGREGATOR attribute in an UPDATE message from another NEW BGP speaker MUST discard the path attribute and continue processing the UPDATE message." | RFC 6793 Section 4.1 | The collapse's source-width condition: from a four-octet source both attributes are discarded rather than merged |
| "If the number of AS numbers in the AS_PATH attribute is less than the number of AS numbers in the AS4_PATH attribute, then the AS4_PATH attribute SHALL be ignored, and the AS_PATH attribute SHALL be taken as the AS path information." | RFC 6793 Section 4.2.3 | The first arm of the moved rule in `internal/core/bgp/attribute`, delegating to `MergeAS4Path` |
| "the AS path information SHALL be constructed by taking as many AS numbers and path segments as necessary from the leading part of the AS_PATH attribute, and then prepending them to the AS4_PATH attribute so that the AS path information has a number of AS numbers identical to that of the AS_PATH attribute." | RFC 6793 Section 4.2.3 | The second arm, delegating to `MergeAS4Path` |
| "Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is either the leading path segment or is adjacent to a path segment that is prepended." | RFC 6793 Section 4.2.3 | The confederation adjacency rule inside `MergeAS4Path` |
| The AGGREGATOR versus AS4_AGGREGATOR choice, and what it decides about the AS4_PATH | RFC 6793 Section 4.2.3 | The moved AGGREGATOR gate; the sentences are quoted verbatim from `rfc/full/rfc6793.txt` at implementation time |
| "A NEW BGP speaker that receives a malformed AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST discard the attribute and continue processing the UPDATE message. The error SHOULD be logged locally for analysis." | RFC 6793 Section 6 | The discard reason the moved rule returns, and the session-subsystem log line the ingest caller writes |
| "When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers." | RFC 6793 Section 4.2.2 | The narrowing arms of `recordTranscode` and `TranscodeASPath`, both of which already exist and both of which now see a four-octet source |
| "Whenever the AS path information contains the AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP speaker MUST exclude such path segments from the AS4_PATH attribute being constructed." | RFC 6793 Section 4.2.2 | `hasNonMappableASN` and `AS4Path.WriteTo`, which already exist |
| The route server MUST NOT modify an RS client's AS_PATH | RFC 7947 Section 2.2.2 | The empty `Prepend` on the RS rail; the sentence is quoted verbatim from `rfc/full/rfc7947.txt` at implementation time |

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Review Gate

### Round 1
| Finding | Severity | File | Resolution |
|---------|----------|------|------------|

### Round 2
| Finding | Severity | File | Resolution |
|---------|----------|------|------------|
