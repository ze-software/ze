# Spec: wire-edit-3-aspath-fold -- AS_PATH prepend and ASN4 transcode become generate slots

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | `plan/spec-wire-edit-2-edit-apply.md` |
| Phase | 4/6 |
| Deferral shard | `plan/deferrals/spec-wire-edit-3-aspath-fold.md` |
| Updated | 2026-08-01 |

Child 3 of `plan/spec-wire-edit-0-umbrella.md`. It removes the second full
payload copy that every eBGP destination with a policy pays today.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

There are two rewrite mechanisms in the forward path and they do not compose.

| Mechanism | Producer | Expresses |
|-----------|----------|-----------|
| AS-path rewrite family | `internal/component/bgp/wireu/aspath_rewrite.go:35`, `internal/component/bgp/wireu/aspath_rewrite.go:52`, `internal/component/bgp/wireu/aspath_transcode.go:35` | AS_PATH prepend, ASN4 transcode, and the RFC 6793 derived AS4_PATH, AGGREGATOR and AS4_AGGREGATOR edits |
| Edit set and one-pass writer | `internal/component/bgp/filterapi/filterapi.go:98`, `internal/component/bgp/reactor/forward_build.go:58` | everything else: Set, Add, Remove, Prepend, Suppress, plus the NLRI and withdrawn overrides |

Because they are separate passes, an eBGP destination with any policy pays **two
full payload copies, back to back**. The closure at
`internal/component/bgp/reactor/reactor_api_forward.go:428` produces a rewritten
payload into a read-pool buffer, the destination loop takes it at
`internal/component/bgp/reactor/reactor_api_forward.go:712`, and
`buildModifiedPayload` at `internal/component/bgp/reactor/reactor_api_forward.go:793`
copies that rewritten payload again into the per-peer pool buffer. The second copy
is not amortised, because the edit set differs per destination.

The AS-path family also walks the attribute section repeatedly on its own.
`rewriteASPathPrepend` (`internal/component/bgp/wireu/aspath_rewrite.go:63`) scans
every attribute from `internal/component/bgp/wireu/aspath_rewrite.go:93`, and on
the slow path scans them all again from
`internal/component/bgp/wireu/aspath_rewrite.go:155` to find AS4_PATH, AGGREGATOR
and AS4_AGGREGATOR, then `rewritePrependASPathFull`
(`internal/component/bgp/wireu/aspath_rewrite.go:374`) walks a third time while
emitting, at `internal/component/bgp/wireu/aspath_rewrite.go:471`.

**Goal.** Make the AS_PATH family a **generate slot** in the edit set defined by
`plan/spec-wire-edit-2-edit-apply.md`, so the prepend, the transcode and their
RFC 6793 derived attributes are recorded as intent and emitted by the one-pass
writer, into the destination buffer, once.

The generate kind exists precisely for this: an ASN4 transcode re-encodes every
ASN in the path, so it cannot be a fragment list, and staging it through the
arena would reintroduce the double move. It already has a paired size function,
`LenWithASN4`, used at `internal/component/bgp/wireu/aspath_rewrite.go:224` and
`internal/component/bgp/wireu/aspath_rewrite.go:411`.

Once no intermediate rewritten payload exists, the per-update eBGP wire cache
(`internal/component/bgp/reactor/received_update.go:170`) and the forward-handle
adoption it requires (`internal/component/bgp/reactor/received_update.go:121`)
have nothing left to hold, and retire.

**Non-goal.** No change to which ASNs are prepended, when a transcode happens, or
which derived attributes RFC 6793 obliges. This child moves the same decisions
into a different execution position, byte for byte.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/encoding-context.md` - per-peer encoding context, ASN4 and ADD-PATH
  → Constraint: zero-copy forwarding is legal only when the source and destination context IDs match. A generate slot is bound to one destination context, and the resulting wire must carry the destination's ASN4 width, exactly as `fwdContextIDWithASN4` records today at `internal/component/bgp/reactor/received_update.go:199`.
- [ ] `docs/architecture/memory/lifetime-contracts.md` - buffer lifetime contracts
  → Constraint: `adoptFwdHandle` (`internal/component/bgp/reactor/received_update.go:121`) exists because an intermediate rewritten payload is aliased into async writes and must outlive the forward call. Removing the intermediate is what makes the adoption removable; removing the adoption before the intermediate is gone is a use-after-return.
  → Decision: retire the adoption only when a grep for intermediate variants is clean, never on the assumption that it is.
- [ ] `ai/rules/buffer-first.md` - encoding writes into pooled bounded buffers
  → Constraint: the resolver writes at an offset into the caller's buffer after answering an exact size. It must not build a path into a scratch slice first.
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: AS_PATH is a sequence of segments, each a type byte, a count byte and count ASNs of 2 or 4 octets. Re-encoding changes the value length, so the header size class can change with it.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: today a transcode failure returns a read buffer and skips the destination, at `internal/component/bgp/reactor/reactor_api_forward.go:729`. Under the edit set the same failure must still suppress that destination, never emit an untranscoded path to a two-octet peer.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format, eBGP AS_PATH prepend
  → Constraint: Section 9.1.2 requires prepending the local AS when propagating to an eBGP peer. Section 5 orders attribute type codes ascending on emission, which matters because AS4_PATH (17) may be created by this resolver.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN, AS4_PATH, AS_TRANS
  → Constraint: Section 4.2.2 obliges an AS4_PATH when the path is not composed only of mappable four-octet ASNs, and requires an existing AGGREGATOR to be set to AS_TRANS with the real value carried in AS4_AGGREGATOR. An AS_PATH edit can therefore *induce* three further attribute edits, so the resolver must own that derivation rather than leaving it to a producer.
  → Constraint: Section 6 requires a malformed AS4_PATH to be discarded while processing continues, which the current code does at the site noted in Current Behavior.
- [ ] `rfc/short/rfc7947.md` - route server
  → Constraint: Section 2.2.2 forbids a route server from modifying AS_PATH for an RS-client peer, so the prepend slot must be skippable per destination while the transcode slot still applies.
- [ ] `rfc/full/rfc7705.txt` - AS migration, dual-AS local-as. The implementation summary is written but parked at `rfc/pending/rfc7705.md`, out of `rfc/short/` until `plan/spec-bgp-as-migration.md` enrols it; read the source text meanwhile.
  → Constraint: Section 3.3 fixes the dual-AS prepend order: the speaker appends the globally configured ASN first and the "Local AS" value immediately after, which leaves the override outermost, closest to the peer. Section 3.2 works the same order through end to end (AS_PATH 64510 64500 64499, override then real ASN then origin). The prepend slot must therefore carry an ordered sequence of ASNs, never an unordered set, and the two local-as modifiers cut it down rather than reorder it: "Replace Old AS" drops the globally configured ASN and prepends only the override, while the base mechanism prepends both.
- [ ] `rfc/short/rfc8654.md` - extended message
  → Constraint: the body ceiling bounds the re-encoded path, so the size the resolver answers always fits a 16-bit field.

**Key insights:** (minimal context to resume after compaction)
- The generate kind and the one-pass writer already exist after `plan/spec-wire-edit-2-edit-apply.md`. This child registers a resolver, it does not extend the writer.
- The exact size function already exists: `LenWithASN4`, called at `internal/component/bgp/wireu/aspath_rewrite.go:224` and `internal/component/bgp/wireu/aspath_rewrite.go:411`.
- The AS-path family owns four codes, not one: AS_PATH (2), AGGREGATOR (7), AS4_PATH (17) and AS4_AGGREGATOR (18). One resolver owns all of them, because RFC 6793 makes them a single decision. (AGGREGATOR is code 7. An earlier draft of this line said 18, which is AS4_AGGREGATOR.)
- The tombstone transitive clear at the eBGP boundary (`internal/component/bgp/wireu/aspath_rewrite.go:542`) rides on the same pass today and must ride on the resolver afterwards.
- The dual-AS local-as mode prepends two ASNs in a defined order, the override outermost. `RewriteASPathDual` (`internal/component/bgp/wireu/aspath_rewrite.go:52`) encodes that order in the array it builds. RFC 7705 Section 3.3 is the authority for that order (append the globally configured ASN first, the "Local AS" value immediately after, so the override lands closest to the peer), and Section 3.2 shows the result as AS_PATH 64510 64500 64499. The slot must carry the order as ordered intent, never as a set.
- `getEBGPWire` (`internal/component/bgp/reactor/reactor_api_forward.go:428`) caches per key within one forward call, and `EBGPWire` (`internal/component/bgp/reactor/received_update.go:170`) caches across calls in two atomic slots. Both exist only to amortise the intermediate copy this child deletes.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:35` - `RewriteASPath`: the single-ASN entry point, stack-allocating a one-element array and delegating.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:52` - `RewriteASPathDual`: the local-as dual mode; the array order is load-bearing, since the last element prepended ends up outermost.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:63` - `rewriteASPathPrepend`: parses the body layout, then scans every attribute from `off := attrsStart` at `internal/component/bgp/wireu/aspath_rewrite.go:93` looking only for AS_PATH.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:155` - the second full scan, run only on the slow path, collecting AS4_PATH, AGGREGATOR and AS4_AGGREGATOR offsets before delegating.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:208` - `rewriteInsertASPath`: the no-AS_PATH case, inserting a complete attribute; it sizes with `LenWithASN4` at `internal/component/bgp/wireu/aspath_rewrite.go:224` and writes the header with `attribute.WriteHeaderTo` at `internal/component/bgp/wireu/aspath_rewrite.go:269`.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:289` - `tryDirectPrepend`: the fast path, which copies the whole payload with one attribute shifted, encoding `asn = 23456` for AS_TRANS at `internal/component/bgp/wireu/aspath_rewrite.go:354` when the destination is a two-octet speaker.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:374` - `rewritePrependASPathFull`: the slow path, sizing with `existingPath.LenWithASN4(dstASN4)` at `internal/component/bgp/wireu/aspath_rewrite.go:411`, walking the attributes a third time from `off := attrsStart` at `internal/component/bgp/wireu/aspath_rewrite.go:471`, deriving `attribute.AttrAS4Path` at `internal/component/bgp/wireu/aspath_rewrite.go:490`, setting AGGREGATOR to `AS_TRANS` at `internal/component/bgp/wireu/aspath_rewrite.go:503`, and clearing the tombstone transitive bit with `clearTombstoneTransitive` at `internal/component/bgp/wireu/aspath_rewrite.go:542`.
- [ ] `internal/component/bgp/wireu/aspath_transcode.go:35` - `TranscodeASPath`: the no-prepend RS-client case; returns 0 when the widths already match, and otherwise repeats the same body-layout parse and attribute scan.
- [ ] `internal/component/bgp/wireu/aspath_as4.go:31` - `hasNonMappableASN`: the RFC 6793 Section 4.2.2 test that decides whether an AS4_PATH is obliged.
- [ ] `internal/component/bgp/wireu/aspath_as4.go:59` - `as4PathForPath`, plus `as4PathForRewrite` at `internal/component/bgp/wireu/aspath_as4.go:80`: the two derivation entry points, one for transcode and one for prepend.
- [ ] `internal/component/bgp/wireu/aspath_as4.go:100` - `as4PathWireSize`, plus `writeAS4PathAttr` at `internal/component/bgp/wireu/aspath_as4.go:115`: the existing exact-size-then-write pair, which is the shape the resolver needs.
- [ ] `internal/component/bgp/reactor/received_update.go:170` - `EBGPWire`: lock-free hit on an atomic slot, double-checked miss, a read-pool buffer whose `dst.Buf` is checked at `internal/component/bgp/reactor/received_update.go:189`, and `wireu.RewriteASPath` into it at `internal/component/bgp/reactor/received_update.go:193`.
- [ ] `internal/component/bgp/reactor/received_update.go:209` - `ebgpSlot`: two slots, one per destination ASN width, so a fan-out to mixed peers materialises two intermediate payloads.
- [ ] `internal/component/bgp/reactor/received_update.go:121` - `adoptFwdHandle`: the buffer-lifetime repair that exists because those intermediate payloads are aliased into async writes; drained by `returnFwdHandles` at `internal/component/bgp/reactor/received_update.go:136`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:428` - `getEBGPWire`: the per-call closure that keys the intermediate wire and adopts its handle with `update.adoptFwdHandle(dst)` at `internal/component/bgp/reactor/reactor_api_forward.go:459` and `internal/component/bgp/reactor/reactor_api_forward.go:537`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:712` - the destination-loop `getEBGPWire` call producing the eBGP wire, gated on the peer being eBGP and not an RS client.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:719` - the RS-client branch: a transcode is still required when the source is four-octet and the destination is not, and it runs inline at `internal/component/bgp/reactor/reactor_api_forward.go:728` for an export-override wire, adopting the handle at `internal/component/bgp/reactor/reactor_api_forward.go:739`, or once per call at `internal/component/bgp/reactor/reactor_api_forward.go:753` with the handle adopted at `internal/component/bgp/reactor/reactor_api_forward.go:765`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:793` - the modify call that copies the already-rewritten payload a second time.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:1081` - `applyASOverride`, plus `rewriteASPathOverride` at `internal/component/bgp/reactor/reactor_api_forward.go:1107`: the AS-override producer, a third AS_PATH writer with its own rules.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go:374` - `aspathHandler`: the existing registered AS_PATH handler, which supports Prepend by inserting a new AS_SEQUENCE segment and otherwise falls back to the generic set handler.

**Behavior to preserve:**
- Byte-identical output for every AS-path transform: single prepend, dual-AS prepend, no-prepend and replace-as local-as modes, ASN4 transcode in both directions the code supports, AS4_PATH derivation, AGGREGATOR set to AS_TRANS with AS4_AGGREGATOR carrying the real value, and the malformed-AS4_PATH discard.
- RFC 7947 Section 2.2.2: an RS-client destination never has its AS_PATH modified, while a required ASN4 transcode still applies.
- The AS-override behaviour at `internal/component/bgp/reactor/reactor_api_forward.go:1081`, including its interaction ordering with the prepend.
- The tombstone transitive clear at the eBGP boundary.
- Prepend order: the outermost ASN is the one closest to the peer, as `RewriteASPathDual` documents.
- A transcode or rewrite failure suppresses that destination; it never emits a path the destination cannot read.
- Every existing AS-path suite under `internal/component/bgp/wireu/` passes unchanged.

**Behavior to change:**
- The prepend and the transcode become generate slots on the per-destination edit set instead of a prior pass producing a whole payload.
- The intermediate rewritten payload disappears, and with it the two atomic eBGP wire slots, the per-call `getEBGPWire` cache and the forward-handle adoption.
- The AS4_PATH, AGGREGATOR and AS4_AGGREGATOR derivations become slots the AS_PATH resolver declares, so the one-pass writer merge-inserts them at their ascending code positions.
- The attribute scans at `internal/component/bgp/wireu/aspath_rewrite.go:93`, `internal/component/bgp/wireu/aspath_rewrite.go:155` and `internal/component/bgp/wireu/aspath_rewrite.go:471` are replaced by reads of the base's span index.

## Data Flow (MANDATORY)

### Entry Point
- A received UPDATE published as an immutable base with a span index (`plan/spec-wire-edit-1-base-index.md`).
- A destination peer whose forward facts state whether it is eBGP, whether it is an RS client, its local and secondary AS, and its negotiated ASN4 width.

### Transformation Path
1. The destination loop resets the hoisted edit set (`internal/component/bgp/reactor/reactor_api_forward.go:604`).
2. The eBGP prepend decision is taken exactly where it is taken today, at `internal/component/bgp/reactor/reactor_api_forward.go:712`, but records ordered prepend intent on the AS_PATH slot instead of producing a payload.
3. The RS-client transcode decision, taken today at `internal/component/bgp/reactor/reactor_api_forward.go:719`, records a transcode target on the same slot.
4. The AS-override producer (`internal/component/bgp/reactor/reactor_api_forward.go:1081`) records its intent on the same slot rather than building its own bytes.
5. The size query asks the AS_PATH resolver for its exact contribution, which it answers from the base's AS_PATH span plus the recorded intent, using `LenWithASN4`.
6. The resolver also declares whether RFC 6793 obliges AS4_PATH, AGGREGATOR and AS4_AGGREGATOR slots, so those enter the same size query.
7. The one-pass writer emits everything into the exactly-sized destination buffer, merge-inserting the derived attributes at their ascending code positions.
8. Dispatch to the forward pool is unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Destination facts to AS_PATH slot | ordered ASNs plus a transcode target, recorded as typed intent, not as bytes | No |
| AS_PATH resolver to the one-pass writer | an exact size, then a write at an offset, plus the derived slots it declares | No |
| Base span index to resolver | the AS_PATH, AS4_PATH, AGGREGATOR and AS4_AGGREGATOR spans are read, never re-scanned | No |
| Read-pool to forward path | removed by this child: no intermediate payload survives, so no handle is adopted | No |

### Integration Points
- `filterapi` gains the AS-path intent on the edit set; the accumulator's exported surface is unchanged, so plugins compile.
- `wireu` keeps its AS-path parsing and encoding helpers; `as4PathWireSize` and `writeAS4PathAttr` become the resolver's size-then-write pair.
- `reactor.buildModifiedPayload` needs no change: the generate kind already exists after `plan/spec-wire-edit-2-edit-apply.md`.
- `filter_delta_handlers.aspathHandler` folds into the resolver, so one AS_PATH writer remains instead of three.
- `wireu.SplitWireUpdate` still consumes materialised output, unchanged.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every AS-path transform in the tree reduces to ordered prepend ASNs plus a transcode target plus a remove-private flag. | The three producers are `internal/component/bgp/wireu/aspath_rewrite.go:35`, `internal/component/bgp/wireu/aspath_rewrite.go:52` and `internal/component/bgp/wireu/aspath_transcode.go:35`, plus the override at `internal/component/bgp/reactor/reactor_api_forward.go:1107`. | The transform that does not fit keeps a terminal raw override, and the child lands without it. | A mapping table with one row per producer, each with a byte-identity test. | unvalidated |
| A-2 | The RFC 6793 derived attributes can be declared by the resolver rather than by the producer. | The derivation already lives inside the rewrite, at `internal/component/bgp/wireu/aspath_rewrite.go:490` and `internal/component/bgp/wireu/aspath_rewrite.go:503`, not in any caller. | The producers would have to declare them, which reintroduces the coupling this child removes. | A test asserting that a producer recording only a prepend still yields AS4_PATH when the path requires it. | unvalidated |
| A-3 | Deleting the eBGP wire cache does not regress a large fan-out. | The cache amortises a copy that this child deletes outright; the remaining per-destination work is a resolver write into a buffer that must be written anyway. | Keep the cache keyed on the edit-set fingerprint instead, which is `plan/spec-wire-edit-5-fanout-dedup.md`. | A fan-out benchmark at 1, 10 and 100 eBGP destinations, before and after. | unvalidated |
| A-4 | No intermediate wire variant survives, so `adoptFwdHandle` can be deleted. | Its call sites are `internal/component/bgp/reactor/reactor_api_forward.go:459`, `internal/component/bgp/reactor/reactor_api_forward.go:537`, `internal/component/bgp/reactor/reactor_api_forward.go:739` and `internal/component/bgp/reactor/reactor_api_forward.go:765`, all of them adopting a rewritten payload. | Keep the adoption for whatever survives, and delete only what is provably gone. | Grep for the adoption and for read-pool acquisition on the forward path, which must both come back clean. | unvalidated |
| A-5 | The tombstone transitive clear can ride on the resolver without changing when it fires. | It fires inside `rewritePrependASPathFull`, at the `clearTombstoneTransitive` call at `internal/component/bgp/wireu/aspath_rewrite.go:542`, which runs exactly when the eBGP prepend runs. | It becomes its own slot on the tombstone attribute code. | A test over an UPDATE carrying the tombstone marker forwarded to an eBGP peer. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The AS-path code is the most intricate in the package: the slow path interleaves offset arithmetic across four attributes. A translation error mis-encodes paths. | The golden byte-identity harness from `plan/spec-wire-edit-2-edit-apply.md`, extended with an AS-path transform matrix. | The old implementation stays in the tree, exercised by the harness as the reference, until every matrix cell is byte-identical. |
| R-2 | Retiring the intermediate variants changes buffer lifetimes, which is the exact area a prior fix had to repair. | Debug-build poison reads on returned pool buffers. | Keep `adoptFwdHandle` until the grep in A-4 is clean; delete it in its own commit. |
| R-3 | A two-octet peer receives a path that was not transcoded, or an AS4_PATH that does not match. | Interop scenario against a two-octet-ASN speaker. | Interop is a gate for this child, not an optional extra. |
| R-4 | RS-client handling is subtle: no prepend, but a transcode may still be required. | The existing route-server suites. | AC-4 pins both halves, and the existing route-server `.ci` files run unchanged. |
| R-5 | The AS-override producer and the prepend interact; folding both into one slot could change their order. | A test that configures AS-override and an eBGP prepend on the same destination. | The slot records ordered intent, and the order today is fixed by the call sequence in the destination loop. That sequence is preserved verbatim. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Wrong AS_PATH on the wire: a loop that should have been detected is not, a two-octet peer reads a four-octet path as garbage, or an AS4_PATH contradicts its AS_PATH. Peers may reset the session or install wrong forwarding state. |
| How is it reverted? | Single commit revert while the golden harness and the old implementation are both still present. After the old implementation is deleted, revert means restoring it. |
| Who else touches this path? | `plan/spec-wire-edit-5-fanout-dedup.md` replaces the caching this child deletes; `plan/spec-bgp-as-notation.md` and `plan/spec-bgp-deferred-confederation-otc.md` touch neighbouring AS_PATH concerns. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends an UPDATE that is forwarded to an eBGP peer with an export policy | → | prepend recorded as a generate slot, one materialisation, no intermediate payload | `test/plugin/wire-edit-single-materialise.ci` |
| A four-octet path is forwarded to a two-octet-ASN speaker | → | transcode generate slot plus derived AS4_PATH, written once | `test/plugin/wire-edit-asn4-transcode-single-copy.ci` |
| A route server forwards to an RS client that is a two-octet speaker | → | no prepend slot, transcode slot still applied | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| An eBGP peer with local-as dual mode receives a route | → | two ordered ASNs on one slot, outermost closest to the peer | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The AS-path transform matrix replayed over a corpus of received UPDATEs | Byte-identical to the current implementation for every cell, EXCEPT where merge-insert corrects attribute order. A newly derived AS4_PATH (17) or AS4_AGGREGATOR (18) used to be appended after every source attribute; the one-pass writer inserts it at its ascending type-code position instead. RFC 4271 Section 5 orders attributes ascending on emission, and `plan/spec-wire-edit-2-edit-apply.md` already made that correction for other codes, so AS4 follows rather than staying an exception (Thomas, 2026-08-01). A base carrying LARGE_COMMUNITY (32) or OTC (35) plus a derived AS4 attribute therefore reaches the wire in a different byte ORDER, with the same content. Any golden that moves for a reason OTHER than AS4 ordering is a stop-and-report |
| AC-2 | An eBGP destination with an export policy that modifies attributes | Exactly one full payload copy occurs for that destination, down from two, asserted by a copy counter in a benchmark |
| AC-3 | A four-octet AS_PATH forwarded to a two-octet-ASN destination | AS_PATH carries AS_TRANS where required and AS4_PATH carries the real values, both emitted by one writer pass |
| AC-4 | An RS-client destination | AS_PATH is not modified, and an ASN4 transcode still applies when the widths differ |
| AC-5 | An UPDATE carrying AGGREGATOR with a non-mappable ASN, forwarded to a two-octet destination | AGGREGATOR is set to AS_TRANS and AS4_AGGREGATOR carries the real value, both merge-inserted at their ascending positions |
| AC-6 | An UPDATE with no AS_PATH forwarded to an eBGP peer | A complete AS_PATH attribute is created at its ascending position |
| AC-7 | An UPDATE carrying a malformed AS4_PATH | It is discarded and processing continues, exactly as today |
| AC-8 | An UPDATE carrying the tombstone marker forwarded across the eBGP boundary | The transitive bit is cleared, as it is today |
| AC-9 | After this child lands | `grep -rn "adoptFwdHandle" internal/` returns nothing, and the two atomic eBGP wire slots are gone |
| AC-10 | An eBGP destination whose prepend or transcode cannot be produced | The route is suppressed for that destination, never forwarded with an unmodified path |
| AC-11 | A local-as dual-mode destination | The override ASN is outermost and the real AS sits behind it, matching today's byte output |
| AC-12 | A destination with AS-override configured together with an eBGP prepend | The emitted path matches today's byte output, including the relative order of the two edits |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a two-octet-ASN speaker while carrying four-octet paths | wire, AS_PATH generate slot, AS4_PATH derivation, one materialisation, TCP | `test/plugin/wire-edit-asn4-transcode-single-copy.ci` |
| 2 | Configures an export policy on an eBGP peer and receives a route | wire, prepend slot plus policy slots, one materialisation, TCP | `test/plugin/wire-edit-single-materialise.ci` |
| 3 | Runs a route server whose clients include a two-octet speaker | wire, no prepend, transcode slot, TCP | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| 4 | Configures local-as dual mode on an eBGP session | wire, ordered prepend intent, TCP | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |
| 5 | Configures AS-override on an eBGP session | wire, override intent on the same slot, TCP | existing `test/plugin/community-strip.ci` running unchanged alongside a new Go byte-identity case |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestASPathSlotGoldenByteIdentity` | `internal/component/bgp/reactor/forward_build_golden_test.go` | AC-1: the transform matrix is byte-identical to the current implementation | |
| `TestASPathSlotSizeIsExact` | `internal/component/bgp/wireu/aspath_slot_test.go` | the resolver's size equals the bytes it writes, for both ASN widths and both header classes | |
| `TestASPathSlotDerivesAS4Path` | `internal/component/bgp/wireu/aspath_slot_test.go` | AC-3, A-2: a producer recording only a prepend still yields AS4_PATH when RFC 6793 obliges it | |
| `TestASPathSlotDerivesAggregatorASTrans` | `internal/component/bgp/wireu/aspath_slot_test.go` | AC-5: AGGREGATOR to AS_TRANS with AS4_AGGREGATOR carrying the real value | |
| `TestASPathSlotInsertsWhenAbsent` | `internal/component/bgp/wireu/aspath_slot_test.go` | AC-6: a created AS_PATH lands at its ascending position | |
| `TestASPathSlotDiscardsMalformedAS4Path` | `internal/component/bgp/wireu/aspath_slot_test.go` | AC-7: discard and continue, per RFC 6793 Section 6 | |
| `TestASPathSlotDualOrder` | `internal/component/bgp/wireu/aspath_slot_test.go` | AC-11: the override ASN is outermost | |
| `TestASPathSlotRSClientSkipsPrepend` | `internal/component/bgp/reactor/forward_aspath_test.go` | AC-4: no prepend for an RS client, transcode still applied | |
| `TestASPathSlotASOverrideOrder` | `internal/component/bgp/reactor/forward_aspath_test.go` | AC-12: override and prepend compose in today's order | |
| `TestTombstoneTransitiveClearedAtEBGP` | `internal/component/bgp/reactor/forward_aspath_test.go` | AC-8, A-5: the clear fires exactly when it fires today | |
| `TestASPathFailureSuppressesDestination` | `internal/component/bgp/reactor/forward_aspath_test.go` | AC-10: a failed resolve suppresses, never forwards an unmodified path | |
| `TestNoIntermediateWireVariantRemains` | `internal/component/bgp/reactor/received_update_test.go` | AC-9, A-4: no read-pool buffer is acquired on the forward path and no handle is adopted | |
| `BenchmarkEBGPForwardCopiesPerDestination` | `internal/component/bgp/reactor/forward_build_bench_test.go` | AC-2, A-3: copies and allocations per destination at fan-out 1, 10 and 100 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AS_PATH segment ASN count | 0-255 | 255 | N/A | 256 (a new segment is required) |
| ASN value | 0-4294967295 | 4294967295 | N/A | N/A (uint32 domain) |
| mappable ASN for a two-octet peer | 0-65535 | 65535 | N/A | 65536 (encoded as AS_TRANS, real value into AS4_PATH) |
| prepended ASN count | 1-2 | 2 (dual-AS) | 0 (refused) | 3 (no producer emits it) |
| AS_PATH value length after re-encode | 0-65535 | 65535 | N/A | 65536 (suppress the destination) |
| UPDATE body length after rewrite | 4-65516 | 65516 | 3 | 65517 (suppress the destination) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wire-edit-asn4-transcode-single-copy` | `test/plugin/wire-edit-asn4-transcode-single-copy.ci` | a two-octet peer receives a correctly transcoded path with AS4_PATH, built once | |
| `wire-edit-single-materialise` | existing `test/plugin/wire-edit-single-materialise.ci` | an eBGP peer with policy now pays one copy, not two | |
| `bgp-rs-fastpath-ebgp-shared` | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | the route-server fast path is unchanged | |
| `bgp-rs-fastpath-ibgp-identity` | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | the identity forward is unchanged | |
| `bgp-rs-reactor-fastpath` | existing `test/plugin/bgp-rs-reactor-fastpath.ci` | the reactor fast path is unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-wire-edit-asn4-transcode` | `test/interop/scenarios/` | FRR | a two-octet-ASN speaker accepts the transcoded path together with its AS4_PATH | |
| `NN-wire-edit-ebgp-prepend` | `test/interop/scenarios/` | BIRD | a real eBGP peer sees the local AS prepended exactly once, in the right position | |

## Files to Modify
- `internal/component/bgp/wireu/aspath_rewrite.go` - becomes the AS_PATH generate resolver; the payload-copying entry points retire once the harness is clean
- `internal/component/bgp/wireu/aspath_transcode.go` - folds into the same resolver
- `internal/component/bgp/wireu/aspath_as4.go` - the AS4_PATH derivation moves under the resolver and keeps its size-then-write pair
- `internal/component/bgp/filterapi/filterapi.go` - the edit set carries ordered AS-path intent
- `internal/component/bgp/reactor/reactor_api_forward.go` - the prepend and transcode decisions record intent; `getEBGPWire` and its adoptions retire
- `internal/component/bgp/reactor/forward_rs.go` - the route-server rail records the same intent
- `internal/component/bgp/reactor/received_update.go` - the two atomic eBGP wire slots and the forward-handle adoption retire
- `internal/component/bgp/reactor/filter_delta_handlers.go` - `aspathHandler` folds into the resolver
- `docs/architecture/wire/attributes.md` - the AS-path generate slot and its derived attributes
- `docs/architecture/memory/lifetime-contracts.md` - remove the adoption rule once the intermediate is gone
- `docs/features/rfc-status.md` - re-anchor the RFC 4271 Section 9.1.2 and RFC 6793 Section 4.2.2 rows

## Files to Create
- `internal/component/bgp/wireu/aspath_slot.go` - the resolver: intent, exact size, write, and the derived-slot declaration
- `internal/component/bgp/wireu/aspath_slot_test.go` - sizing, derivation, ordering and discard coverage
- `internal/component/bgp/reactor/forward_aspath_test.go` - RS-client, override, tombstone and suppression coverage
- `test/plugin/wire-edit-asn4-transcode-single-copy.ci` - transcode without a second copy

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new configuration surface; local-as, AS-override and RS-client leaves already exist |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; the new `.ci` covers the transcode path |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_update_aspath_resolve_failed_total`, labelled by peer, so AC-10 suppressions are observable |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code; AS4_PATH and AS4_AGGREGATOR already exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal execution position changes, not behaviour |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | The AS_PATH handler is internal, not plugin-registered |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` for the generate slot and the derived attributes |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` rows for RFC 4271 Section 9.1.2, RFC 6793 Section 4.2.2 and RFC 7947 Section 2.2.2, re-anchored to the resolver |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/memory/lifetime-contracts.md`, since the adoption rule it documents no longer has a subject |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the resolve-failure counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `aspath_rewrite.go`, `aspath_transcode.go`, `received_update.go` and `reactor_api_forward.go`, and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The local-as and AS-override syntax is unchanged |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- extend the golden harness with the AS-path matrix
   - Tests: `TestASPathSlotGoldenByteIdentity`, initially calling the current implementation on both sides
   - Files: `internal/component/bgp/reactor/forward_build_golden_test.go`, corpus additions covering four-octet paths, AGGREGATOR, existing AS4_PATH and the tombstone marker
   - Verify: every matrix cell is pinned before anything moves
2. **Phase: the resolver** -- intent, exact size, write, derived-slot declaration
   - Tests: `TestASPathSlotSizeIsExact`, `TestASPathSlotDerivesAS4Path`, `TestASPathSlotDerivesAggregatorASTrans`, `TestASPathSlotInsertsWhenAbsent`, `TestASPathSlotDiscardsMalformedAS4Path`, `TestASPathSlotDualOrder`
   - Files: `internal/component/bgp/wireu/aspath_slot.go`, `internal/component/bgp/wireu/aspath_as4.go`
   - Verify: A-1 and A-2 resolved; nothing consumes the resolver yet; the old path still runs
3. **Phase: record intent in the destination loop** -- switch the producers over
   - Tests: `TestASPathSlotRSClientSkipsPrepend`, `TestASPathSlotASOverrideOrder`, `TestTombstoneTransitiveClearedAtEBGP`, `TestASPathFailureSuppressesDestination`
   - Files: `internal/component/bgp/reactor/reactor_api_forward.go`, `internal/component/bgp/reactor/forward_rs.go`, `internal/component/bgp/filterapi/filterapi.go`
   - Verify: AC-1, AC-4, AC-8, AC-10, AC-11 and AC-12 pass; the golden harness is still byte-identical
4. **Phase: delete the intermediate** -- retire the caches and the adoption
   - Tests: `TestNoIntermediateWireVariantRemains`, `BenchmarkEBGPForwardCopiesPerDestination`
   - Files: `internal/component/bgp/reactor/received_update.go`, `internal/component/bgp/reactor/reactor_api_forward.go`, `internal/component/bgp/reactor/filter_delta_handlers.go`
   - Verify: AC-2, AC-9 pass; A-3 and A-4 resolved with a clean grep and numbers
5. **Phase: retire the old writers**
   - Tests: the full `wireu` AS-path suites, unchanged
   - Files: `internal/component/bgp/wireu/aspath_rewrite.go`, `internal/component/bgp/wireu/aspath_transcode.go`
   - Verify: one AS_PATH writer remains; the harness still reports byte identity against the recorded reference
6. **Phase: interop and documentation**
   - Tests: the two interop scenarios
   - Files: `test/interop/scenarios/`, the doc targets above
   - Verify: a real two-octet speaker and a real eBGP peer both accept the output

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line; every AS-path producer in the census maps to recorded intent with a byte-identity test |
| Feature completeness | Every user story has a passing test, including the RS-client and two-octet-peer paths |
| Correctness | The resolver emits the RFC 6793 derived attributes in every case the current slow path does; prepend order is preserved; the tombstone clear fires at the same boundary |
| Naming | The resolver and intent types follow `ai/rules/naming.md`; the package stays `wireu` per the recorded 2026-07-08 decision |
| Data flow | No intermediate payload survives; no read-pool buffer is acquired on the forward path; the resolver reads spans rather than re-scanning |
| Registration over hardcoding | The resolver is selected by attribute code through the same slot mechanism as every other edit; no AS-path special case is added to the writer |
| Rule: `ai/rules/buffer-first.md` | The resolver answers an exact size, then writes at an offset; it never builds a path into a scratch slice |
| Rule: `docs/architecture/memory/lifetime-contracts.md` | The adoption is deleted only after the grep proves no intermediate variant remains |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| AS-path byte identity | `go test ./internal/component/bgp/reactor/ -run TestASPathSlotGoldenByteIdentity -v` |
| One copy per eBGP destination | `go test ./internal/component/bgp/reactor/ -bench BenchmarkEBGPForwardCopiesPerDestination -benchmem` shows the copy count halved |
| Intermediate wire variants removed | `grep -rn "adoptFwdHandle" internal/` returns nothing |
| eBGP wire cache removed | `grep -n "ebgpSlot" internal/component/bgp/reactor/received_update.go` returns nothing |
| One AS_PATH writer remains | `grep -rn "func rewritePrependASPathFull" internal/component/bgp/wireu/` returns nothing |
| Existing AS-path suites pass | `go test ./internal/component/bgp/wireu/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The resolver reads AS_PATH, AS4_PATH, AGGREGATOR and AS4_AGGREGATOR spans built from peer-controlled bytes. Segment counts and lengths must be validated before any ASN is read |
| Integer handling | ASNs are 32-bit and encoded as 16-bit for old speakers. The AS_TRANS substitution must be the only narrowing path, and it must be unconditional for a non-mappable ASN |
| Resource exhaustion | A path with the maximum segment count re-encoded to four octets doubles in size. The size query must catch the resulting body overflow before any write |
| Fail-open risk | A resolve failure must suppress the destination. Emitting the unmodified path to a two-octet peer, or to an eBGP peer without the prepend, is both a protocol violation and a routing-correctness bug |
| Loop prevention | The prepend is what makes eBGP loop detection work. A missing prepend is a routing-loop risk, not a cosmetic difference |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Golden harness reports a byte difference | STOP. The transform is not equivalent. Back to design, do not adjust the golden |
| A derived attribute is missing or extra | STOP. RFC 6793 Section 4.2.2 is not satisfied; this is not a tuning question |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Interop peer rejects an UPDATE | STOP and present. A real peer disagreeing is stronger evidence than any unit test |
| The adoption grep is not clean | Do not delete the adoption. Report what still holds a buffer |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The AS-path family is not a special case, it is the case that proves the generate kind is needed. A transcode re-encodes every ASN, so no fragment list over the base can express it, and staging it in the arena would restore exactly the double move this child exists to delete.
- The RFC 6793 derivations belong to the resolver, not to the caller. That is already true in the code: the derivation lives inside the rewrite, and no caller mentions AS4_PATH. Preserving that ownership is what keeps the producers simple.
- Two caches exist here (a per-call one and a per-update one, in two ASN-width variants) purely to amortise a copy. Deleting the copy deletes the reason for both, which is a larger simplification than the copy saving itself.
- `as4PathWireSize` and `writeAS4PathAttr` are already a size-then-write pair. The resolver contract is not new to this package, it is generalised from what AS4_PATH already does.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| AS_PATH becomes a generate slot | keep the prior pass and accept two copies | The second copy is per destination and cannot be amortised, because the edit set differs per destination |
| One resolver owns AS_PATH, AS4_PATH, AGGREGATOR and AS4_AGGREGATOR | one slot per code, each declared by the producer | RFC 6793 makes them a single decision. Splitting them puts protocol knowledge in every producer |
| Ordered prepend intent rather than pre-encoded bytes | record the encoded segment | The destination's ASN width is not known to every producer, and AS_TRANS substitution depends on it |
| Delete the eBGP wire caches | key them on the edit set instead | With no intermediate payload there is nothing to cache. Fan-out sharing moves to `plan/spec-wire-edit-5-fanout-dedup.md`, which caches the materialisation, not the intermediate |
| Keep the old implementation until the harness is clean | migrate and delete in one step | The AS-path slow path is the most intricate code in the package; the reference implementation is the only trustworthy oracle |

## Known Limitations

- The two-to-four-octet direction still does not merge an existing AS4_PATH, matching the current documented behaviour of `TranscodeASPath`; that merge stays an ingress concern.
- Splitting still runs after materialisation, so a path that only overflows after the prepend is split rather than pre-planned.
- The remove-private-AS intent is defined on the slot but has no producer in this child; it exists so a future producer needs no contract change.
- Fan-out sharing is not restored in this child. Between this child and `plan/spec-wire-edit-5-fanout-dedup.md`, a large eBGP fan-out does per-destination work that the deleted cache used to share. A-3 is the gate that says whether that gap is acceptable to land on its own.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site after this child |
|-----|---------|-------------|----------------------|
| 4271 | 9.1.2 | prepend the local AS when propagating to an eBGP peer | the prepend branch of `internal/component/bgp/wireu/aspath_slot.go` |
| 6793 | 4.2.2 | AS4_PATH obligation, AGGREGATOR set to AS_TRANS | the derivation in `internal/component/bgp/wireu/aspath_slot.go`, carried from `attribute.AttrAS4Path` at `internal/component/bgp/wireu/aspath_rewrite.go:490` and `AS_TRANS` at `internal/component/bgp/wireu/aspath_rewrite.go:503` |
| 6793 | 6 | discard a malformed AS4_PATH and continue | the discard branch of `internal/component/bgp/wireu/aspath_slot.go` |
| 7947 | 2.2.2 | a route server must not modify AS_PATH for RS clients | the RS-client gate at `internal/component/bgp/reactor/reactor_api_forward.go:712` |
| 7705 | 3.3 | dual-AS local-as ordering: globally configured ASN appended first, "Local AS" immediately after, so the override is outermost | the ordered intent, carried from `internal/component/bgp/wireu/aspath_rewrite.go:52` |
| 8654 | - | extended message body ceiling, which bounds the re-encoded path | the resolver's size query |
| draft-mangin-idr-attr-tombstone-00 | 5.3 | clear the transitive bit at the eBGP boundary | carried from `internal/component/bgp/wireu/aspath_rewrite.go:542` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-wire-edit-3-aspath-fold.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-wire-edit-3-aspath-fold.md` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- `internal/component/bgp/wireu/aspath_slot.go`: `ASPathIntent` (ordered prepend ASNs, transcode target, RS-client and remove-private flags) and `ASPathEdit.Record`, which registers AS_PATH, AS4_PATH, AGGREGATOR and AS4_AGGREGATOR as generate slots on the edit set from child 2.
- Five generators implement the size-then-write contract: `asPathShiftGen`, `asPathEncodeGen`, `as4PathGen`, `aggregatorGen`, `as4AggregatorGen`. `tryShift` is the same-width fast path.
- The RFC 6793 derivations stay inside the resolver: no producer mentions AS4_PATH or AS_TRANS.
- The second full payload copy is gone. A destination with a policy now takes one materialisation, not two (three on the route-server rail).

### Bugs Found/Fixed

Two live defects, both pre-existing and both found while translating the slow path:

- **AGGREGATOR was destroyed on every same-width slow-path prepend.** Fixed; `TestPrependKeepsValidAggregatorOnEveryPath` and `TestTranscodeNoOpLeavesAggregatorAlone`.
- **`clearTombstoneTransitive` fired on only one of three prepend paths.** Fixed; `TestTombstoneTransitiveClearedOnEveryPrependPath`.

### Documentation Updates

- `docs/architecture/wire/attributes.md` -- the generate slot and the AS4 derivation ownership.
- The RFC Documentation table above now points at `aspath_slot.go` rather than `aspath_rewrite.go`.

### Deviations from Plan

| # | Plan said | What was built | Why |
|---|-----------|----------------|-----|
| D-1 | AC-1 byte-identical for every matrix cell | Byte-identical except AS4 ORDERING | A derived AS4_PATH (17) or AS4_AGGREGATOR (18) used to be appended after every source attribute. The one-pass writer merge-inserts it at its ascending position, so a base carrying LARGE_COMMUNITY (32) or OTC (35) reaches the wire in a different byte order with identical content. RFC 4271 Section 5 orders attributes ascending on emission, and Thomas approved this on 2026-08-01 |
| D-2 | AC-9: the eBGP wire slots are gone | They are unreachable but NOT deleted | `EBGPWire`, `ebgpSlotASN4`, `ebgpSlotASN2` and `ebgpWireSlot` have zero non-test callers, so they are dead code with no behavioural effect. The deletion touches read-pool lifetimes in `recent_cache.go`, which is child 3's own R-2 and R-3 area, and wants an implementation phase rather than a closure-time edit. Homed in `plan/spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal.md` |
| D-3 | AC-9: `adoptFwdHandle` is gone | It survives, with one production caller | Deliberate. It is the correct mechanism for the one remaining borrow (a cross-context transcode buffer), and child 1's `TestForwardAdoptedHandleHeldUntilLastWrite` now proves its release ordering rather than asserting it vacuously |
| D-4 | `test/plugin/wire-edit-asn4-transcode-single-copy.ci` | `test/plugin/asn4-transcode-pooled-buffer.ci` | The existing file already drives the transcode through a real session; it was extended rather than duplicated |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The AS-path slow path was assumed to be correct and only in need of translation | Two of its branches were wrong: AGGREGATOR was destroyed on a same-width prepend, and the tombstone transitive clear fired on one path of three | Writing the per-path matrix test forced every branch to be enumerated | Both fixed in `ddf04953a`, each with a test named after the branch it covers |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| AS_PATH becomes a generate slot on the edit set | Done | `wireu/aspath_slot.go` `ASPathEdit.Record` | |
| One resolver owns AS_PATH, AS4_PATH, AGGREGATOR, AS4_AGGREGATOR | Done | `recordPrepend`, `recordTranscode`, `recordAS4Path`, `recordAggregator` | |
| Ordered prepend intent rather than pre-encoded bytes | Done | `ASPathIntent` | The destination's ASN width is resolved inside the slot |
| Delete the second payload copy | Done | `reactor_api_forward.go` | AC-2 |
| Delete the eBGP wire caches | Partial | `received_update.go` | D-2: unreachable, not deleted |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Changed | the AS-path transform matrix in `internal/component/bgp/wireu/aspath_slot_test.go` | Byte-identical except AS4 ordering (D-1), approved by Thomas 2026-08-01 |
| AC-2 | Done | read-pool borrow deltas measured per destination: 2 copies to 1 on the general rail, 3 to 1 on the route-server rail | |
| AC-3 | Done | `TestASPathSlotDerivesAS4Path`, `TestASPathSlotSuppressesAS4PathWhenNotObliged` | One writer pass |
| AC-4 | Done | `TestASPathSlotRSClientSkipsPrepend`; existing route-server `.ci` corpus | Transcode still applies |
| AC-5 | Done | `TestASPathSlotDerivesAggregatorASTrans` | Merge-inserted at ascending positions |
| AC-6 | Done | `TestASPathSlotInsertsWhenAbsent` | |
| AC-7 | Done | `TestASPathSlotDiscardsMalformedAS4Path` | |
| AC-8 | Done | `TestTombstoneTransitiveClearedOnEveryPrependPath` | This is where the one-of-three defect was found |
| AC-9 | **Partial** | `grep -rn "\.EBGPWire(" internal/ cmd/ pkg/` returns test files only; `adoptFwdHandle` has one production caller at `reactor_api_forward.go:743` | D-2 and D-3. NOT met as written. Homed in `plan/spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal.md`. **Needs Thomas's sign-off** (`ai/rules/no-partial-completion.md`) |
| AC-10 | Done | `TestASPathSlotRefusesTruncatedPayload`, `TestASPathSlotEmptyPrependIsRefused`; the `modifyFailure` suppression path | Never forwarded with an unmodified path |
| AC-11 | Done | `TestASPathSlotDualOrder` | RFC 7705 Section 3.3 order preserved |
| AC-12 | Done | the AS-override plus prepend row of the matrix test | Call sequence preserved verbatim |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the AS-path transform matrix | Done | `wireu/aspath_slot_test.go`, 10 `Test` functions | |
| the two defect regressions | Done | `wireu/aspath_aggregator_probe_test.go`, 4 `Test` functions | Added, not planned |
| the fan-out benchmark for A-3 | Changed | `BenchmarkFanoutDedup`, `BenchmarkFanoutFloor`, `BenchmarkFanoutRebuildOnly` | Child 5 owns the measurement; A-3 is resolved from its numbers |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/wireu/aspath_slot.go` + `_test.go` | Done | |
| `test/plugin/wire-edit-asn4-transcode-single-copy.ci` | Changed | D-4: `test/plugin/asn4-transcode-pooled-buffer.ci` extended instead |
| every "Files to Modify" row | Done | see the diff of `ddf04953a` and `e2037e598` |

### Audit Summary
- **Total items:** 21
- **Done:** 16
- **Partial:** 1 (AC-9, needs Thomas's sign-off)
- **Skipped:** 0
- **Changed:** 4 (D-1 to D-4)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Remove the second full payload copy every eBGP destination with a policy pays | benchmark | Read-pool borrow deltas per destination: 2 to 1 on the general rail, 3 to 1 on the route-server rail. `BenchmarkFanoutRebuildOnly` decomposes what remains: 416 ns rebuild against 2.1 ns copy |
| The two rewrite mechanisms compose | unit | `ASPathEdit.Record` registers into the same `EditSet` child 2 built; `TestASPathSlotShiftPrependIsExact` and the transform matrix pass through one writer |
| RFC 6793 derivations stay with the resolver | unit | `TestASPathSlotDerivesAS4Path`, `TestASPathSlotDerivesAggregatorASTrans`; no producer names AS4_PATH |
| RFC 7947 route-server transparency survives | functional | existing route-server `.ci` corpus, unedited; `TestASPathSlotRSClientSkipsPrepend` |
| Two live wire defects closed | unit | `TestPrependKeepsValidAggregatorOnEveryPath`, `TestTombstoneTransitiveClearedOnEveryPrependPath` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| AC-9: the unreachable eBGP wire cache (`EBGPWire`, `ebgpSlotASN4`, `ebgpSlotASN2`, `ebgpWireSlot`, and their release branches in `recent_cache.go`) is not deleted | deferred | `plan/spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal.md`, created 2026-08-02 and carrying the full task |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/wire-edit-3-aspath-fold-<session-id>.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | correctness/wire/lifetime; tests/security/coverage (two agents over `bbd53bf22^..b1fa7ab1e`, 2026-08-02) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The read-buffer leak tests asserted `before == after` over zero borrows, so the adopted-handle release ordering this child's deletions depend on was untested | `reactor/forward_readbuf_leak_test.go` | `f1f746fb6`, `TestForwardAdoptedHandleHeldUntilLastWrite` over an IBGP destination on a 2-byte send context |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/wireu/aspath_slot.go` | Yes | `grep -n "^func \|^type "` lists `ASPathIntent`, `ASPathEdit`, `Record`, `recordPrepend`, `recordTranscode`, `recordAS4Path`, `recordAggregator`, `tryShift` and five generators |
| `internal/component/bgp/wireu/aspath_slot_test.go` | Yes | 10 `Test` functions |
| `internal/component/bgp/wireu/aspath_aggregator_probe_test.go` | Yes | 4 `Test` functions |
| `test/plugin/asn4-transcode-pooled-buffer.ci` | Yes | `ls test/plugin/asn4-transcode-pooled-buffer.ci` |
| `test/plugin/bgp-rs-relay-aspath-transparency.ci` | Yes | `ls test/plugin/bgp-rs-relay-aspath-transparency.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-3 | AS_TRANS plus AS4_PATH from one pass | `grep -n "func TestASPathSlotDerivesAS4Path" internal/component/bgp/wireu/aspath_slot_test.go`; `make ze-test-bgp` green |
| AC-4 | RS clients keep their AS_PATH | `grep -n "func TestASPathSlotRSClientSkipsPrepend" internal/component/bgp/wireu/aspath_slot_test.go` |
| AC-8 | the tombstone clear fires on every prepend path | `grep -n "func TestTombstoneTransitiveClearedOnEveryPrependPath" internal/component/bgp/wireu/aspath_aggregator_probe_test.go` |
| AC-9 | NOT met | `grep -rn "\.EBGPWire(" internal/ cmd/ pkg/` -> test files only; `grep -rn "adoptFwdHandle" internal/ \| grep -v _test` -> one call site, `reactor_api_forward.go:743` |
| AC-11 | dual-AS order | `grep -n "func TestASPathSlotDualOrder" internal/component/bgp/wireu/aspath_slot_test.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| eBGP forward with an export policy | `test/plugin/nexthop-self.ci` | Yes -- read: real session, modified wire pinned. The "one materialisation" half is the borrow-delta measurement (D-3 of child 2 applies here too) |
| Four-octet path to a two-octet speaker | `test/plugin/asn4-transcode-pooled-buffer.ci` | Yes -- read: drives the transcode through a real session and asserts the emitted wire |
| RS client that is a two-octet speaker | `test/plugin/bgp-rs-relay-aspath-transparency.ci` | Yes -- read: asserts the relayed AS_PATH is untouched for an RS client |
| eBGP with local-as dual mode | `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | Yes -- read: identity forward unchanged; the ordering itself is `TestASPathSlotDualOrder` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Every AS-path transform reduces to `ASPathIntent`: ordered prepend ASNs, a transcode target, RS-client and remove-private flags. The override at `reactor_api_forward.go` maps to the ordered prepend list; no producer needed a terminal raw override |
| A-2 | confirmed | The derivation lives in `recordAS4Path` and `recordAggregator`, inside the resolver. `TestASPathSlotDerivesAS4Path` records only a prepend and still gets AS4_PATH when the path requires it |
| A-3 | confirmed with a caveat | Deleting the caches does not regress fan-out where a policy group is shared: `BenchmarkFanoutDedup` shows -14.4% at (10,2) and -28.6% at (100,2) once child 5 lands. Where every destination is its own group the digest costs about 3%, which is child 5's AC-11 and is homed there |
| A-4 | **broken** | An intermediate variant DOES survive: the cross-context transcode buffer, adopted at `reactor_api_forward.go:743`. `adoptFwdHandle` is correctly kept (D-3), and its release ordering is now proven rather than assumed. The unreachable eBGP slots are the part that should have gone and did not (D-2) |
| A-5 | confirmed, and it exposed a defect | The tombstone clear rides on the resolver. It also turned out to have been firing on one prepend path of three before this child; `TestTombstoneTransitiveClearedOnEveryPrependPath` now covers all three |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Wire format: the generate slot | `docs/architecture/wire/attributes.md` checked against `asPathEncodeGen.GenLen`/`GenWrite`, the size-then-write pair | Yes |
| RFC 6793 Section 4.2.2 site moved | The RFC Documentation table above points at `aspath_slot.go`; `grep -n "AS4Path" internal/component/bgp/wireu/aspath_slot.go` confirms the derivation is there | Yes |
| RFC 7947 Section 2.2.2 site | `TestASPathSlotRSClientSkipsPrepend` plus the RS-client gate in `reactor_api_forward.go` | Yes |
| Categories answered No | `ddf04953a` and `e2037e598` touch no `.yang`, no CLI command, no plugin registration | Yes |

## Core Insight

Two caches existed here purely to amortise a copy. Deleting the copy deleted the
reason for both, which is a larger simplification than the copy saving itself --
and translating the slow path branch by branch is what surfaced two live wire
defects that had survived because no test enumerated the branches.
