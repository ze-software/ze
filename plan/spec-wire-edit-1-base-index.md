# Spec: wire-edit-1-base-index -- immutable UPDATE base with an eagerly built attribute span index

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | `plan/spec-wire-edit-0-umbrella.md` |
| Phase | 1/6 |
| Deferral shard | `plan/deferrals/spec-wire-edit-1-base-index.md` |
| Updated | 2026-08-01 |

Child 1 of `plan/spec-wire-edit-0-umbrella.md`. It is the substrate every later
child stands on and the only one that is pure addition: **no emitted byte
changes**.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The attribute TLV sequence of one received UPDATE is walked by several scanners
that share nothing. On the receive path alone, `ValidateUpdateRFC7606AddPath`
(`internal/component/bgp/reactor/session_validation.go:108`) visits every
attribute, then `AttrFind` (`internal/component/bgp/reactor/session_validation.go:112`)
walks again for PrefixSID. Later, the first consumer to call
`Get`/`Has`/`GetRaw` builds a third index lazily under a write lock
(`internal/core/bgp/attribute/wire.go:291`), and each forward-path rewrite walks
the same bytes again.

Two costs follow:

| Cost | Where |
|------|-------|
| Repeated walks of bytes that never change | `internal/core/bgp/attribute/wire.go:291`, `internal/core/bgp/attribute/iterator.go:132`, `internal/component/bgp/reactor/session_validation.go:108` |
| A mutex on every read of an object nobody mutates after receive | `sync.RWMutex` (`internal/core/bgp/attribute/wire.go:35`), taken in `Get` (`internal/core/bgp/attribute/wire.go:63`), `Has` (`internal/core/bgp/attribute/wire.go:132`), `GetRaw` (`internal/core/bgp/attribute/wire.go:181`), `All` (`internal/core/bgp/attribute/wire.go:215`), `ForEach` (`internal/core/bgp/attribute/wire.go:244`) |

**Goal.** Build the attribute index exactly once, eagerly, on the receive
goroutine, as a by-product of the walk `enforceRFC7606`
(`internal/component/bgp/reactor/session_validation.go:38`) already performs, and
publish it as part of an immutable base. Once the index exists before
publication, `AttributesWire` becomes shared-immutable and its `sync.RWMutex`
(`internal/core/bgp/attribute/wire.go:35`) retires along with the lazy build.

The parsed-value cache is what forces the mutex to stay: the `parsed` field of
`attrIndex` (`internal/core/bgp/attribute/wire.go:19`) is filled on demand by
`parseAtLocked` (`internal/core/bgp/attribute/wire.go:336`). It moves to a side table used only by
the text, JSON and show paths, so the forward path never touches a lock and never
retains a heap pointer for the life of a cache entry.

**Non-goal.** No edit, no rewrite, no output-byte change. Producing edits over
this base is `plan/spec-wire-edit-2-edit-apply.md`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: the header is 3 bytes, or 4 when the Extended Length flag is set. A span must record enough to reconstruct the header size class, and it derives that from the flags byte rather than storing a separate width.
- [ ] `docs/architecture/wire/messages.md` - UPDATE body layout
  → Constraint: body is withdrawn-length(2) + withdrawn + attr-length(2) + attrs + NLRI. Span offsets are relative to the attribute section, which begins at a payload offset the sections value already knows.
- [ ] `docs/architecture/memory/lifetime-contracts.md` - the four buffer lifetime contracts
  → Constraint: contract A borrows the receive buffer; a consumer holding bytes past the boundary must Retain or Own first. The span index holds offsets, not bytes, so it inherits the base's boundary exactly and must never be published separately from it.
  → Decision: `Snapshot()` (`internal/component/bgp/wireu/wire_update.go:213`) is the eager copy that makes a borrow safe. Snapshotting must carry the index across, or the copy silently loses it and falls back to a rebuild.
- [ ] `docs/architecture/encoding-context.md` - per-peer encoding context
  → Constraint: the source context ID is part of the base's identity, as it already is on `AttributesWire` (`internal/core/bgp/attribute/wire.go:43`). Parsing an attribute is context-dependent, which is exactly why parsed values move to a side table rather than into the span.
- [ ] `ai/rules/buffer-first.md` - encoding writes into pooled bounded buffers
  → Constraint: the index builder must size from the attribute section it is handed. A growth-by-append index is acceptable only where the current code already does it; a fixed inline array plus a single exact-size spill is preferred.
- [ ] `ai/rules/design-principles.md` - abstraction gate, single responsibility
  → Decision: the gate is cleared. Three producers of the same index exist today, and every later child in this set consumes it.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: the current builder returns an error and leaves the index nil so the next call retries (`internal/core/bgp/attribute/wire.go:291`). An eager builder has no later retry, so an index that cannot be built must make the UPDATE fail the RFC 7606 path, not publish a base with a silently empty index.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format, path attributes
  → Constraint: Section 4.3 defines the attribute header and the Extended Length flag. Section 4.3 also makes a duplicate attribute a Malformed Attribute List error, which the current builder enforces at `internal/core/bgp/attribute/wire.go:309`; the eager builder must keep that verdict identical.
- [ ] `rfc/short/rfc7606.md` - error handling, revised
  → Constraint: the eager index is built inside the same pass that decides the RFC 7606 action, so index construction must not change any action the current validator returns. Section 3(g) keep-first duplicate stripping already runs in `enforceRFC7606` before the base would be published.
- [ ] `rfc/short/rfc8654.md` - extended message
  → Constraint: the body is capped at 65516 octets, so every span offset and length fits a 16-bit field with room to spare. This is what makes an 8-byte span sound.
- [ ] `rfc/short/rfc8669.md` - segment routing prefix SID
  → Constraint: the second receive-path walk exists only to find PrefixSID (`internal/component/bgp/reactor/session_validation.go:112`). A presence bitset answers that lookup without a walk at all.

**Key insights:** (minimal context to resume after compaction)
- The walk is already unconditional on receive, so the index is close to free: `ValidateUpdateRFC7606AddPath` at `internal/component/bgp/reactor/session_validation.go:108` visits every attribute on every received UPDATE.
- The mutex exists solely to guard lazy construction and parsed-value caching. Remove both reasons and the lock has no remaining job.
- `attrIndex` (`internal/core/bgp/attribute/wire.go:19`) is 24 bytes because of its `parsed Attribute` interface field. Dropping that field to a side table is what shrinks a span to 8 bytes.
- The default recent-update cache ceiling is 1,000,000 entries (`internal/component/bgp/reactor/reactor.go:391`) against a soft limit that warns but never rejects (`internal/component/bgp/reactor/recent_cache.go:73`). Every per-UPDATE byte is multiplied by that number, which is what rules out a 512-byte code table.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/core/bgp/attribute/wire.go:19` - `attrIndex`: code, offset(uint16), length(uint16), hdrLen(uint8), and a `parsed Attribute` interface field that is nil until parsed on demand.
- [ ] `internal/core/bgp/attribute/wire.go:34` - `AttributesWire`: a `sync.RWMutex`, the packed bytes, a source context ID, and a nil-until-first-scan index.
- [ ] `internal/core/bgp/attribute/wire.go:63` - `Get`: read-locks, linear-scans the index, and on a cache miss drops the read lock and re-acquires the write lock through `getAndParse`.
- [ ] `internal/core/bgp/attribute/wire.go:132` - `Has`: same read-lock scan, then upgrades to a write lock purely to build the index.
- [ ] `internal/core/bgp/attribute/wire.go:181` - `GetRaw`: the zero-copy accessor the forward path uses, and it takes the lock too.
- [ ] `internal/core/bgp/attribute/wire.go:215` - `All`, plus `ForEach` at `internal/core/bgp/attribute/wire.go:244`: both take the write lock unconditionally because both may fill `parsed`.
- [ ] `internal/core/bgp/attribute/wire.go:291` - `ensureIndexLocked`: builds into a local slice with `make([]attrIndex, 0, 8)` at `internal/core/bgp/attribute/wire.go:298`, rejects a duplicate code on `seen[code]` at `internal/core/bgp/attribute/wire.go:309`, rejects a truncated attribute on `offset+hdrLen+int(length) > len(a.packed)` at `internal/core/bgp/attribute/wire.go:315`, and publishes only on success so a failure retries.
- [ ] `internal/core/bgp/attribute/wire.go:336` - `parseAtLocked`: resolves the source encoding context and parses one attribute value, caching the result in the index entry.
- [ ] `internal/core/bgp/attribute/iterator.go:132` - `AttrFind`: a standalone zero-allocation single-code scan, used where the caller wants no index at all.
- [ ] `internal/component/bgp/wireu/wire_update.go:40` - `WireUpdate`: payload, source context, message and source IDs, and three `sync.Once`-guarded lazy fields (sections, attributes, mixed-shape verdict).
- [ ] `internal/component/bgp/wireu/wire_update.go:100` - `Attrs`: wraps the attribute section in a fresh `AttributesWire` once, under `attrsOnce`.
- [ ] `internal/component/bgp/wireu/wire_update.go:213` - `Snapshot`: the contract A eager copy; it rebuilds a `WireUpdate` around owned bytes and carries only the message and source IDs.
- [ ] `internal/core/bgp/wire/update_sections.go:37` - `UpdateSections`: four int offsets plus a validity bool; accessors return zero-copy slices into the payload.
- [ ] `internal/core/bgp/wire/update_sections.go:56` - `ParseUpdateSections`: the single producer of those offsets, with the RFC 4271 minimum-length and containment checks.
- [ ] `internal/component/bgp/reactor/session_validation.go:38` - `enforceRFC7606`: runs on every received UPDATE before callback dispatch; parses section lengths by hand, validates both NLRI-bearing IPv4 fields, then hands the attribute section to the validator.
- [ ] `internal/component/bgp/reactor/session_validation.go:108` - the `ValidateUpdateRFC7606AddPath` call over the whole attribute section, plus the extra `AttrFind` walk at `internal/component/bgp/reactor/session_validation.go:112` for the RFC 8669 PrefixSID discard.
- [ ] `internal/component/bgp/reactor/received_update.go:52` - `ReceivedUpdate`: the cache entry that owns the published base, including the adopted forward handles appended by `adoptFwdHandle` (`internal/component/bgp/reactor/received_update.go:121`) and drained by `returnFwdHandles` (`internal/component/bgp/reactor/received_update.go:136`).
- [ ] `internal/component/bgp/reactor/recent_cache.go:73` - `maxEntries`: a soft limit that warns but never rejects, so entry lifetime is bounded by acknowledgement and eviction, not by time.
- [ ] `internal/component/bgp/reactor/reactor.go:391` - the 1,000,000-entry default when the config leaf is unset.

**Behavior to preserve:**
- Every RFC 7606 action `enforceRFC7606` returns today, for every input, byte for byte and verdict for verdict. Adding an index must not turn an accept into a reset or the reverse.
- The duplicate-attribute verdict at `internal/core/bgp/attribute/wire.go:309` and the truncation verdict at `internal/core/bgp/attribute/wire.go:315`, including their error text shape.
- `Get`, `Has`, `GetRaw`, `GetMultiple`, `All`, `ForEach`, `Packed`, `PackFor` keep their signatures and their return semantics, including the documented `(nil, nil)` "not present" contract.
- Zero-copy: `GetRaw` still returns a subslice of the packed bytes, never a copy.
- `Snapshot()` still yields a `WireUpdate` that owns its payload, per buffer lifetime contract A.
- Emitted wire bytes: this child adds no encoder and changes no output. Every existing `.ci` under `test/plugin/`, `test/policy/` and `test/encode/` passes unchanged.

**Behavior to change:**
- Index construction moves from lazy-under-write-lock to eager-on-receive, folded into the walk `enforceRFC7606` already performs.
- `sync.RWMutex` (`internal/core/bgp/attribute/wire.go:35`) is removed; reads become lock-free.
- The `parsed Attribute` field leaves the index entry for a side table owned by the text, JSON and show paths.
- A per-code presence bitset answers `Has` and the PrefixSID lookup without any scan.

## Data Flow (MANDATORY)

### Entry Point
- Peer-received UPDATE: TCP bytes land in a pooled read buffer, the session read loop frames one message, and `enforceRFC7606` (`internal/component/bgp/reactor/session_validation.go:38`) is called before any consumer sees it.
- API-originated UPDATE: bytes are produced by an announce rail and wrapped by `NewWireUpdate` (`internal/component/bgp/wireu/wire_update.go:60`); it reaches the same base type without the receive-path walk, so the index is built on first use there.

### Transformation Path
1. Frame the UPDATE and hand the body to `enforceRFC7606` (`internal/component/bgp/reactor/session_validation.go:38`).
2. Parse the four section offsets. Today this is open-coded inside the validator and again by `ParseUpdateSections` (`internal/core/bgp/wire/update_sections.go:56`) when a consumer first asks. **Proposed:** one parse, its result carried into the base.
3. Validate the attribute section (`internal/component/bgp/reactor/session_validation.go:108`). **Proposed:** the same pass emits one span per attribute plus the presence bitset.
4. Apply the RFC 8669 PrefixSID discard. **Proposed:** answered from the presence bitset, deleting the second walk at `internal/component/bgp/reactor/session_validation.go:112`.
5. Publish the base into a `ReceivedUpdate` (`internal/component/bgp/reactor/received_update.go:52`). After this point the base is immutable and shared.
6. Consumers read through the existing `AttributesWire` surface, now backed by the published spans and taking no lock.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine to reactor cache | the base is fully built before publication; nothing writes it afterwards | No |
| Reactor cache to forward loop (many goroutines) | shared immutable base, borrowed bytes, boundary is cache eviction | No |
| Base to parsed-value side table | the side table is the only mutable part and is never touched by the forward path | No |
| Base to `Snapshot()` copy | the copy owns new bytes, so spans must be rebased onto them or rebuilt | No |

### Integration Points
- `attribute.AttributesWire` keeps its whole read surface; only its internals change, so the text, JSON, show and RIB consumers are untouched.
- `wireu.WireUpdate` keeps `Payload`, `Attrs`, `NLRI`, `Withdrawn`, `MPReach`, `MPUnreach`, `MixesNLRIFields`, `IsEOR` as the read surface over the base.
- `AttrFind` (`internal/core/bgp/attribute/iterator.go:132`) stays in the `attribute` package for callers that hold bare attribute bytes with no base.
- `UpdateSections` (`internal/core/bgp/wire/update_sections.go:37`) is embedded in the base by value, unchanged.

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
| A-1 | The span index can be emitted from the existing RFC 7606 walk with no measurable added receive cost. | The walk is unconditional (`internal/component/bgp/reactor/session_validation.go:108`) and a second walk already runs at `internal/component/bgp/reactor/session_validation.go:112`. | Keep the lazy build and the mutex; every later child still works, it just pays a walk. | Go benchmark of the receive path before and after, same corpus. | **superseded** (2026-08-01) -- the index is not emitted from the RFC 7606 walk at all. A-6 made that unsound, so it is built in `NewAttributesWire` and published by `publishBase` at the end of `enforceRFC7606`. Net walk count is UNCHANGED (the lazy build already walked once on first access) and one heap allocation per UPDATE is removed (`TestSpanIndexZeroAllocUpToEight`). |
| A-2 | Removing the mutex is safe once the index is built before publication. | The lock exists only to guard lazy construction (`internal/core/bgp/attribute/wire.go:291`) and parsed-value caching (`internal/core/bgp/attribute/wire.go:336`). | Keep a lock on the side table only; the forward path still never takes one. | Race detector over the forward-path suites plus a concurrent-fanout unit test. | **confirmed** -- writer set enumerated and closed (see Deviations). `sync.RWMutex` gone; a `sync.Mutex` guards the parsed side table only. `make ze-race-reactor` reported 0 data races over 20 iterations; `TestBaseImmutableAfterPublication` drives 16 concurrent readers under `-race`. |
| A-3 | An 8-entry inline span array covers the common attribute count without a heap allocation. | The current builder pre-sizes to 8 (`internal/core/bgp/attribute/wire.go:298`), which is an existing judgement about the same distribution. | Raise the constant; the structure does not change. | Instrumented run recording the attribute-count distribution over a real feed. | **confirmed** -- the histogram already existed: `docs/architecture/wire/attributes.md` "Real-World Attribute Count Distribution" measures 112M MRT routes at 99.9% <= 8 attributes, maximum observed 10. `ze_bgp_update_span_spill_total` makes the same question answerable from a running daemon. |
| A-4 | No consumer relies on the parsed-value cache surviving on the index entry. | `parsed` is private and reachable only through `Get` (`internal/core/bgp/attribute/wire.go:63`), `All` and `ForEach`. | The side table gains the same caching semantics behind the same accessors. | Compile plus a grep for `parsed` across the tree at the start of implementation. | **confirmed** -- `parsed` was unexported and reachable only from those three accessors. The side table keeps identical caching semantics; the package's own 800-line `wire_test.go` passes unedited. |
| A-5 | API-originated UPDATEs tolerate a first-use index build, since they never pass through `enforceRFC7606`. | `NewWireUpdate` (`internal/component/bgp/wireu/wire_update.go:60`) is the announce-rail constructor and has no validation pass. | The announce rails call the builder explicitly at construction. | Unit test asserting an API-built base answers `Has` correctly with no prior accessor call. | **confirmed, and stronger than assumed** -- building in `NewAttributesWire` means every rail (announce, `update` command, API batch) is eager too, with no rail-side call. `TestAPIBuiltBaseAnswersHas`. |
| A-6 | **The bytes the RFC 7606 walk indexes are the bytes that get published.** | Assumed silently by "emit the span index as a by-product of the walk". NOT stated anywhere in this spec before 2026-08-01. | The index must be built AFTER the rebuild paths, or rebuilt on them. "Pure addition, no second walk" weakens. | Read `enforceRFC7606` end to end, past the validator call. | **broken** (2026-08-01) |

→ A-6 is BROKEN, and it is the load-bearing correction to this child. Found by a
research pass on 2026-08-01 and confirmed in the main thread against
`reactor/session_validation.go` `enforceRFC7606` and
`message/attr_discard.go`. The published base is NOT always the array the
validator walked. Two paths rebuild or mutate it AFTER the walk returns:

- **RFC 7606 Section 3.g keep-first strip.** When the validator records
  duplicate attributes, `enforceRFC7606` strips them and rebuilds the body, then
  wraps the result in a NEW `WireUpdate`. Every span offset after the first
  stripped range shifts, so an index built during the walk is wrong for the
  object that is actually published.
- **Attribute discard in place.** `ApplyAttrDiscard` takes an in-place branch for
  the single-occurrence case, which OVERWRITES the type-code byte with
  `AttrTombstone` and leaves the length alone. No new `WireUpdate` is built and
  nothing signals the change. Offsets survive, but `span.code` goes stale: a
  presence bitset built during the walk would report the original code as present
  after it has been tombstoned.

→ Constraint: build the index after those branches, or rebuild it on them. Both
are malformed-input paths so the cost is irrelevant, but it is unspecified work
and it means this child is not the "pure addition, zero output-byte change" the
umbrella's Implementation Steps promise. `StripAttrRanges` and `ApplyAttrDiscard`
are absent from this spec's Files to Modify and must be added.

→ Constraint: T1-5 (`plan/spec-hotpath-alloc-round-4.md`, 2026-08-01) changed
BOTH mechanisms while fixing an RFC 8669 Section 4 leak. `applyInPlace` now
declines when the attribute code recurs, so the multi-occurrence case takes the
rebuild path rather than the in-place one, and the Section 4 branch no longer
discards `DuplicateRanges`. Re-read both before implementing this child; the
descriptions above are current as of that change, not before it.

→ Correction to the umbrella, same date: AC-6's saving from folding the PrefixSID
walk does not exist on iBGP sessions. That second walk was always gated on
`!isIBGP && !AcceptSRv6PrefixSID`. The umbrella states it unconditionally.

→ **A-6 resolved by ORDERING, not by rebuilding (2026-08-01, implementation).** The
index is built in `attribute.NewAttributesWire`, which `wireu.WireUpdate.Attrs`
calls once under `attrsOnce`. `enforceRFC7606` now calls `Attrs` itself, from
`publishBase`, as the LAST thing it does -- after the Section 3.g strip and after
`ApplyAttrDiscard`. Both mutation branches therefore complete before any index
exists, so neither needs a rebuild:

| Branch | Why the index is right | Test |
|--------|------------------------|------|
| Section 3.g keep-first strip | wraps the deduped body in a NEW `WireUpdate`, whose `Attrs` indexes the new bytes | `TestStripRebuildIndexMatchesPublished` |
| `ApplyAttrDiscard` in place | mutates `pathAttrs` before `publishBase` runs, so the index sees ATTR_TOMBSTONE, not the original code | `TestInPlaceDiscardPrecedesIndexBuild` |

Verified at the producer: `processMessage` (`reactor/session_read.go`) constructs
the `WireUpdate` and calls `enforceRFC7606` immediately, with no `Attrs` call in
between; `ApplyAttrDiscard` has exactly one non-test caller, `enforceRFC7606`; and
every forward-path writer (`wireu.WriteTombstone`, `clearTombstoneTransitive`,
`rewriteASPathPrepend`) writes into a `dst` egress buffer, never into a received
payload. So `StripAttrRanges` and `ApplyAttrDiscard` needed no change and are
correctly absent from Files to Modify. The ordering invariant is documented on
`WireUpdate.Attrs` and on `publishBase`, and the two tests above fail if a future
edit calls `Attrs` earlier.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An eager builder that fails has no later retry, so a build error could publish a base with an empty index that reads as "no attributes". | A unit test feeding a truncated attribute section and asserting the RFC 7606 action, not the index shape. | Build failure routes into the existing RFC 7606 action path. There is no state in which a base is published with an index that did not build. |
| R-2 | `Snapshot()` copies the payload, so spans computed against the original bytes are offsets into a different array. | A test that snapshots and then reads every attribute. | Spans are offsets relative to the attribute section start, so they are valid against any byte array with identical contents. The snapshot carries them across verbatim, and a test pins that. |
| R-3 | Dropping the mutex while a consumer still mutates the object through some path not found by grep. | Race detector, and `-race` on the whole reactor package. | The base type exposes no setter. Any write path found during implementation blocks the mutex removal and is reported before proceeding. |
| R-4 | Behaviour drift in the duplicate and truncation verdicts when the check moves into the receive walk. | Golden verdict table over a malformed-attribute corpus. | The verdicts are pinned by a table-driven test before the move, and the same table runs after. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every consumer of path attributes reads the wrong bytes: the RIB installs wrong attributes, the show and JSON paths print wrong values, and a wrong duplicate verdict either resets a healthy session or accepts a malformed UPDATE. |
| How is it reverted? | Single commit revert. This child adds a structure and rewires readers to it; nothing outside the process has observed a changed byte, so revert is complete. |
| Who else touches this path? | `plan/spec-wire-edit-2-edit-apply.md` and `plan/spec-wire-edit-3-aspath-fold.md` consume this index; `plan/spec-hotpath-alloc-round-4.md` touches neighbouring allocation sites on the same receive path. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends any UPDATE carrying path attributes | → | `enforceRFC7606` emits spans and the presence bitset before publication | `TestReceivePathPublishesSpanIndex` |
| A consumer calls `Has` for an attribute the UPDATE does not carry | → | presence bitset answers with no scan and no lock | `TestPresenceBitsetAnswersHasWithoutScan` |
| An eBGP peer receives a route forwarded unchanged from a same-context peer | → | zero-copy passthrough still holds; the index changed no bytes | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| An iBGP peer receives a route forwarded unchanged | → | identity forward still byte-identical | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any received UPDATE with path attributes | The base carries one span per attribute, in wire order, with the same code, value offset and value length the lazy builder produces for the same bytes |
| AC-2 | An UPDATE whose attribute section holds a duplicate type code | The RFC 7606 action is identical to today's, and no base is published with a partially built index |
| AC-3 | An UPDATE whose last attribute is truncated | The RFC 7606 action is identical to today's |
| AC-4 | A base published to the forward path, read concurrently by many destination goroutines | No lock is taken on any read, and `-race` reports nothing |
| AC-5 | `grep -n "sync.RWMutex" internal/core/bgp/attribute/wire.go` after the change | Returns nothing |
| AC-6 | A base built from an eBGP UPDATE without PrefixSID | The RFC 8669 discard decision is reached from the presence bitset, with no second attribute walk |
| AC-7 | A `Snapshot()` of a base | Every attribute reads back identically from the snapshot, and the snapshot needs no index rebuild |
| AC-8 | An UPDATE with 8 or fewer attributes | The index costs zero heap allocations |
| AC-9 | The full existing functional corpus | Every `.ci` under `test/plugin/`, `test/policy/` and `test/encode/` passes with no expectation edited |
| AC-10 | The receive-path benchmark, before and after | No measurable regression; A-1 is resolved with numbers |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a neighbour and receives routes carrying communities and MP_REACH | wire, receive walk, span index, RIB install, `show` output | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| 2 | Runs a route server whose clients share one received route | wire, one base, many concurrent destination reads, TCP | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |
| 3 | Receives a malformed UPDATE from a misbehaving peer | wire, receive walk, RFC 7606 action, session or attribute handling | `TestRFC7606VerdictsUnchangedByEagerIndex` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSpanIndexMatchesAttributesWire` + `FuzzSpanIndexMatchesIterator` | `internal/core/bgp/attribute/span_test.go` | AC-1: the eager index matches an INDEPENDENT `AttrIterator` oracle on code, value offset, value length and header class. Renamed from `TestSpanIndexMatchesLazyIndex`: the lazy build is deleted, so comparing against it is impossible; an oracle that shares no code with the builder is the stronger test. 2.4M fuzz executions, no divergence | done |
| `TestSpanFitsEightBytes` | `internal/core/bgp/attribute/span_test.go` | the span budget. Renamed from `TestSpanIsEightBytes`: the realized span is **6** bytes (offset, length, code, hdrLen), so the test pins the <=8 budget and the 48-byte inline array rather than asserting a number that is not true | done |
| `TestSpanIndexRejectsDuplicateCode` | `internal/core/bgp/attribute/span_test.go` | AC-2: identical verdict to the old `seen[code]` check, and a failed build publishes no spans and no presence bits | done |
| `TestSpanIndexRejectsTruncatedAttribute` | `internal/core/bgp/attribute/span_test.go` | AC-3: identical verdict to the old containment check | done |
| `TestSpanIndexRejectsOversizeSection` | `internal/core/bgp/attribute/span_test.go` | the fail-closed 16-bit bound the old builder papered over with a `//nolint:gosec` | done (added) |
| `TestSpanIndexZeroAllocUpToEight` | `internal/core/bgp/attribute/span_test.go` | AC-8: zero heap allocations at or below the inline capacity, and the constructor allocates at most the struct | done |
| `TestSpanIndexSpillsPastInlineCapacity` | `internal/core/bgp/attribute/span_test.go` | the 8/9 span-count boundary across the inline/spill seam | done (added) |
| `TestPresenceBitsetAnswersHasWithoutScan` | `internal/core/bgp/attribute/span_test.go` | presence answers without walking and without allocating | done |
| `TestReceivePathPublishesSpanIndex` | `internal/component/bgp/reactor/session_span_index_test.go` | AC-1: after `enforceRFC7606`, a first `Attrs()` costs zero allocations, which only holds if the base was already published | done |
| `TestRFC7606VerdictsUnchangedByEagerIndex` | `internal/component/bgp/reactor/session_span_index_test.go` | AC-2, AC-3: nine verdict classes, each pinned by action, error-ness AND the exact emitted payload hex | done |
| `TestInPlaceDiscardPrecedesIndexBuild` | `internal/component/bgp/reactor/session_span_index_test.go` | A-6, in-place branch | done (added) |
| `TestStripRebuildIndexMatchesPublished` | `internal/component/bgp/reactor/session_span_index_test.go` | A-6, strip branch | done (added) |
| `TestBaseImmutableAfterPublication` | `internal/component/bgp/wireu/base_test.go` | AC-4: 16 concurrent readers under `-race` | done |
| `TestSnapshotCarriesSpanIndex` | `internal/component/bgp/wireu/base_test.go` | AC-7: a snapshot reads back identically and rebuilds nothing (zero allocations on its first `Attrs()`) | done |
| `TestSnapshotOfEmptyAndMalformedUpdates` | `internal/component/bgp/wireu/base_test.go` | AC-7 at its edges: no attribute section, and unparsable sections | done (added) |
| `TestAPIBuiltBaseAnswersHas` | `internal/component/bgp/wireu/base_test.go` | A-5 | done |
| ~~`BenchmarkReceivePathIndexBuild`~~ | ~~`session_validation_bench_test.go`~~ | ~~AC-10, A-1~~ | **not written.** A-1 was superseded: the index is not folded into the RFC 7606 walk, so there is no before/after receive-path delta to measure. What changed is one fewer heap allocation per UPDATE, which `TestSpanIndexZeroAllocUpToEight` asserts directly rather than by timing |
| `BenchmarkAttributeReadNoLock` | `internal/core/bgp/attribute/span_test.go` | AC-4, AC-5: concurrent read throughput with the lock gone | done |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attribute value length (non-extended) | 0-255 | 255 | N/A | 256 (requires the 4-byte header) |
| attribute value length (extended) | 0-65535 | 65535 | N/A | 65536 (unrepresentable on the wire) |
| UPDATE body length | 4-65516 | 65516 | 3 | 65517 |
| span index entries | 0-n | 8 inline | N/A | 9 (heap spill) |
| attribute type code | 0-255 | 255 | N/A | N/A (uint8 domain) |
| span offset into the attribute section | 0-65516 | 65516 | N/A | 65517 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-fastpath-ebgp-shared` | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | an eBGP peer still receives the shared fast-path wire, byte for byte | |
| `bgp-rs-fastpath-ibgp-identity` | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | an iBGP peer still receives an identical forward | |
| `community-strip` | existing `test/plugin/community-strip.ci` | community handling over the new index is unchanged | |
| `nexthop-self` | existing `test/plugin/nexthop-self.ci` | next-hop rewrite over the new index is unchanged | |

This child changes no emitted byte, so its functional evidence is the existing
corpus passing with no expectation edited. A new `.ci` here could not fail for
the right reason; the new coverage that can is the Go layer above.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing receive-side scenarios | `test/interop/scenarios/` | FRR, BIRD, GoBGP | a real peer's UPDATEs still parse to the same attribute set; no new scenario is warranted because no emitted byte changes | |

## Deviations from the design (2026-08-01, implementation)

| # | Design said | What was built | Why |
|---|-------------|----------------|-----|
| D-1 | Emit the index from the RFC 7606 walk | Build it in `attribute.NewAttributesWire`; `enforceRFC7606` calls `publishBase` last | A-6. The walk reads pre-strip, pre-tombstone bytes. Building at construction makes the index correct for whichever byte array is published, on the receive rail and on every API rail, with no second walk and no rebuild. |
| D-2 | New `internal/component/bgp/wireu/base.go` type | No new type. The base is `WireUpdate` + `AttributesWire`, now immutable with an eager index | A parallel base type duplicating `WireUpdate` would be a second read surface over the same bytes (`ai/rules/no-layering.md`), and every consumer already reaches the bytes through these two. Nothing in this child needs a field they do not have. |
| D-3 | Span is 8 bytes | Span is 6 bytes (`offset uint16`, `length uint16`, `code uint8`, `hdrLen uint8`) | 8 was a budget, not a requirement. The inline array is 48 bytes, not 64. |
| D-4 | The mutex "retires" | `sync.RWMutex` is gone; a `sync.Mutex` guards the parsed-value side table | Exactly what Known Limitations already allowed: the forward path (`Has`, `GetRaw`, `Packed`, `Count`, `Spilled`) is lock-free; only `Get`, `All`, `ForEach` lock, and only to fill the cache. AC-5's grep returns nothing. |
| D-5 | AC-6: delete the second PrefixSID walk | Already deleted by commit `8e67a9b03` before this child started | `grep AttrFind internal/component/bgp/reactor/session_validation.go` returns nothing; `result.PrefixSIDPresent` comes from the validator. Nothing to do. |
| D-6 | Spill counter named `bgp_update_span_spill_total` | `ze_bgp_update_span_spill_total`, labelled by peer, on `reactorMetrics` | The repo's counters all carry the `ze_` prefix, and the per-peer label matches its wire-layer siblings. A package-level counter in `internal/core/bgp/attribute` would be global mutable state in a leaf on the hot path. |

**Writer enumeration behind D-4 (the mutex removal).** Five post-construction
writers existed, all in `wire.go`: `a.index = index` in `ensureIndexLocked`, and
four `a.index[i].parsed = attr` sites (`getAndParse` twice, `All`, `ForEach`). The
first is gone -- the index is built by the constructor and never assigned again.
The other four collapse to one, `parseAtLocked`, writing the side table under
`parseMu`. The set is closed: `AttributesWire` exposes no setter, every field is
unexported, and every method on the type lives in `wire.go`. The bytes are not
written either -- `ApplyAttrDiscard` is the only in-place mutator of attribute
bytes in the tree, it has one non-test caller, and it runs before `publishBase`.

## Files to Modify
- `internal/core/bgp/attribute/wire.go` - index moves to the base; `sync.RWMutex` and the lazy build retire; parsed values move to a side table
- `internal/core/bgp/attribute/iterator.go` - `AttrFind` stays, and gains a note pointing bases at the index instead
- `internal/component/bgp/wireu/wire_update.go` - the read accessors sit over the base; `Snapshot` carries the index
- `internal/component/bgp/reactor/session_validation.go` - emit spans and the presence bitset from the existing walk; the PrefixSID lookup reads the bitset
- `internal/component/bgp/reactor/received_update.go` - publication takes the built base
- `docs/architecture/wire/attributes.md` - document the span index and its immutability boundary
- `docs/architecture/memory/lifetime-contracts.md` - contract A gains the rule that an index of offsets shares its base's boundary

## Files to Create
- `internal/core/bgp/attribute/span.go` - the span type, the inline array, the presence bitset, and the single eager builder
- `internal/core/bgp/attribute/span_test.go` - unit and benchmark coverage for the builder
- `internal/component/bgp/wireu/base.go` - the immutable base: payload, sections, spans, presence, source context
- `internal/component/bgp/wireu/base_test.go` - immutability, snapshot and API-origin coverage

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new configuration surface |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; the existing receive and forward corpus covers the path |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_update_span_spill_total`, incremented when an UPDATE exceeds the inline span capacity, so A-3 is answerable from a running daemon |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal representation only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | The handler contract is untouched by this child |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | No emitted byte changes; the parse representation does |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Existing RFC 4271 Section 4.3 and RFC 7606 verdicts are preserved, not changed |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/attributes.md` for the span index, `docs/architecture/memory/lifetime-contracts.md` for the offset-borrow rule |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the spill counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `wire.go`, `iterator.go`, `wire_update.go` and `session_validation.go`, and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No user-facing syntax here |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- pin today's behaviour before touching it
   - Tests: `TestSpanIndexMatchesLazyIndex` and `TestRFC7606VerdictsUnchangedByEagerIndex`, both written against the current code so they pass before any change
   - Files: `internal/core/bgp/attribute/span_test.go`, `internal/component/bgp/reactor/session_validation_test.go`, corpus under `internal/core/bgp/attribute/testdata/`
   - Verify: the equivalence harness runs and pins the current index and the current RFC 7606 verdict table
2. **Phase: the span type** -- add the structure with no consumer
   - Tests: `TestSpanIsEightBytes`, `TestSpanIndexRejectsDuplicateCode`, `TestSpanIndexRejectsTruncatedAttribute`, `TestSpanIndexZeroAllocUpToEight`
   - Files: `internal/core/bgp/attribute/span.go`
   - Verify: pure addition, nothing calls it yet, AC-1 through AC-3 and AC-8 pass
3. **Phase: the base** -- assemble payload, sections, spans and presence into one immutable value
   - Tests: `TestBaseImmutableAfterPublication`, `TestSnapshotCarriesSpanIndex`, `TestAPIBuiltBaseAnswersHas`
   - Files: `internal/component/bgp/wireu/base.go`, `internal/component/bgp/wireu/wire_update.go`
   - Verify: AC-4 and AC-7 pass; `Snapshot` carries the index; A-5 resolved
4. **Phase: build eagerly on receive** -- fold the builder into the existing walk
   - Tests: `TestReceivePathPublishesSpanIndex`, `BenchmarkReceivePathIndexBuild`
   - Files: `internal/component/bgp/reactor/session_validation.go`, `internal/component/bgp/reactor/received_update.go`
   - Verify: AC-1, AC-6 and AC-10 pass; the second PrefixSID walk is gone; A-1 resolved with numbers
5. **Phase: retire the lock** -- readers move to the published index, parsed values to the side table
   - Tests: `BenchmarkAttributeReadNoLock`, the whole reactor package under `-race`
   - Files: `internal/core/bgp/attribute/wire.go`
   - Verify: AC-5 passes, AC-9 holds, A-2 and A-4 resolved
6. **Phase: documentation and counters**
   - Tests: the existing docs gate
   - Files: `docs/architecture/wire/attributes.md`, `docs/architecture/memory/lifetime-contracts.md`
   - Verify: the Documentation checklist rows marked Yes are done with source anchors

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and every accessor listed in Behavior to preserve still has its documented return semantics |
| Feature completeness | Every user story has a passing test, including the malformed-UPDATE story |
| Correctness | The eager index equals the lazy index for every corpus input; the duplicate and truncation verdicts are unchanged; presence and spans never disagree about which codes are present |
| Naming | The base and span types follow `ai/rules/naming.md`; the package stays `wireu` per the recorded 2026-07-08 decision |
| Data flow | Nothing writes the base after publication; the side table is reachable only from the text, JSON and show paths |
| Registration over hardcoding | No per-attribute-code field, switch case or factory is added to a core package: the presence bitset is generic over all 256 codes and knows nothing about which ones matter |
| Rule: `ai/rules/fail-closed-guards.md` | A build failure routes into the RFC 7606 action; no path publishes a base whose index did not build |
| Rule: `docs/architecture/memory/lifetime-contracts.md` | Spans never outlive the payload they index; `Snapshot` rebases or carries them, never leaves them dangling |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Span is 8 bytes | `go test ./internal/core/bgp/attribute/ -run TestSpanIsEightBytes` |
| Eager index equals lazy index | `go test ./internal/core/bgp/attribute/ -run TestSpanIndexMatchesLazyIndex` |
| Mutex removed | `grep -n "sync.RWMutex" internal/core/bgp/attribute/wire.go` returns nothing |
| Second PrefixSID walk removed | `grep -n "AttrFind" internal/component/bgp/reactor/session_validation.go` returns nothing |
| No receive-path regression | `go test ./internal/component/bgp/reactor/ -bench BenchmarkReceivePathIndexBuild -benchmem` |
| No output-byte change | `make ze-functional-test` with no `.ci` expectation edited |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Spans are built from peer-controlled bytes. Every offset and length is bounds-checked at build time, so no later consumer can construct an out-of-range slice from an index that looks trustworthy |
| Integer handling | Offsets and lengths are 16-bit on the wire and used as Go ints for slicing. Every conversion is explicitly bounded by the RFC 8654 body ceiling |
| Resource exhaustion | The span count is bounded by the attribute count, which is bounded by the body size. The spill counter makes an attacker-driven spill visible |
| Fail-open risk | A failed build must never publish an empty index that reads as "this UPDATE has no attributes", which would silently bypass every attribute-based policy |
| Concurrency | Removing the lock is safe only if nothing writes after publication. A write path discovered later is a correctness bug, not a performance question |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| The equivalence harness reports an index difference | STOP. The builder is not equivalent. Back to design, do not adjust the harness |
| An RFC 7606 verdict changes | STOP. Receive-path behaviour is not in scope for this child |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | This child changes no bytes, so a `.ci` failure means a real regression. Back to the phase that introduced it |
| A write-after-publication path is found | Report it, keep the mutex on that path, and land the rest |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The index already exists three times over on the receive path. This child does not add a structure, it deletes two of the three and publishes the survivor.
- The mutex is not protecting shared mutable state that the design wants. It is protecting laziness. Removing the laziness removes the need.
- The `parsed Attribute` field is the reason a span is 24 bytes rather than 8, and it is also the reason a heap pointer is retained for the life of a cache entry whose ceiling is a million entries. Moving it out is a memory win and a cache-line win from one change.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build eagerly during the RFC 7606 walk | keep the lazy build under a mutex | The walk is already unconditional, and eager construction is what converts a shared-mutable object into a shared-immutable one |
| 8-byte span, parsed values in a side table | keep the 24-byte `attrIndex` with its interface field | One cache line covers a typical UPDATE instead of three, and no heap pointer is retained by a TTL-less cache entry |
| 32-byte presence bitset | a 512-byte per-code table | At the 1,000,000-entry ceiling the table is 512 MB against 32 MB, and an O(1) lookup buys nothing when the whole index fits in one cache line |
| Spans are offsets relative to the attribute section | absolute payload offsets | Section-relative offsets survive `Snapshot()` and any future re-basing without arithmetic |
| Keep `AttrFind` | route every caller through a base | Some callers hold bare attribute bytes with no base. Forcing a base on them would be coupling for its own sake |

## Known Limitations

- API-originated bases still build their index on first use, because they never pass through the receive walk. `plan/spec-wire-edit-4-api-origin.md` is where that converges.
- The inline capacity is derived from the existing pre-size judgement at `internal/core/bgp/attribute/wire.go:298`, not from a traffic histogram. A-3 is the measurement gate; only the constant would change.
- Parsed-value caching keeps whatever synchronisation the side table needs. This child does not claim the show and JSON paths become lock-free, only that the forward path does.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site after this child |
|-----|---------|-------------|----------------------|
| 4271 | 4.3 | path attribute header layout, including the Extended Length flag | `internal/core/bgp/attribute/span.go` builder |
| 4271 | 4.3 | duplicate attributes are a malformed attribute list | the duplicate check in `internal/core/bgp/attribute/span.go`, carried from `seen[code]` at `internal/core/bgp/attribute/wire.go:309` |
| 7606 | 3 | structural length conflicts route to session reset | unchanged at `internal/component/bgp/reactor/session_validation.go:38` |
| 8654 | - | extended message body ceiling, which bounds every span field | `internal/core/bgp/attribute/span.go` field widths |
| 8669 | 4 | discard PrefixSID from eBGP unless configured to accept | `internal/component/bgp/reactor/session_validation.go`, now reading the presence bitset |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
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
- [ ] Learned summary written to `plan/learned/NNN-wire-edit-1-base-index.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-wire-edit-1-base-index.md` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- `internal/core/bgp/attribute/span.go`: `Span` (6 bytes: offset, length, code, hdrLen), `SpanIndex` with an 8-entry inline array, a 32-byte presence bitset, a heap spill past 8, and one eager builder `SpanIndex.build`.
- `attribute.NewAttributesWire` builds the index at construction and stores `indexErr`. The lazy build and `sync.RWMutex` are gone; a `sync.Mutex` guards the parsed-value side table used only by the text, JSON and show paths.
- `Session.publishBase` (`internal/component/bgp/reactor/session_validation.go`) calls `WireUpdate.Attrs` as the LAST action of `enforceRFC7606`, after the RFC 7606 Section 3.g strip and after `ApplyAttrDiscard`. That ordering is what makes A-6 harmless.
- `AttributesWire.CarryOver` reuses an index across a byte-identical rebuild, falling back to a full rebuild on a length mismatch.
- `ze_bgp_update_span_spill_total` on `reactorMetrics`, labelled by peer.

### Bugs Found/Fixed

- The pooled `SpanIndex` discarded its spill slice on every rebuild, so a spilled UPDATE re-allocated on each reuse. Fixed in `f1f746fb6`; covered by `TestSpanIndexReuseKeepsSpillCapacity` and `TestSpanIndexErrorPathKeepsCapacityAndReadsEmpty`.
- The old builder papered over a 16-bit bound with a `//nolint:gosec`. The eager builder fails closed with `ErrAttrSectionTooLarge`; `TestSpanIndexRejectsOversizeSection`.

### Documentation Updates

- `docs/architecture/wire/attributes.md` -- the span index, the inline/spill boundary and the immutability boundary.
- `docs/architecture/memory/lifetime-contracts.md` -- contract A gains the rule that an index of offsets shares its base's lifetime boundary.
- `make ze-doc-test` result recorded in Pre-Commit Verification below.

### Deviations from Plan

D-1 to D-6 are recorded in full in "Deviations from the design (2026-08-01, implementation)" above and are not repeated here.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-6 assumed the bytes the RFC 7606 walk indexes are the bytes that get published, so the index could be emitted from that walk | Two branches rebuild or mutate the attribute array AFTER the walk: the Section 3.g keep-first strip wraps a NEW `WireUpdate`, and `ApplyAttrDiscard` overwrites a type code in place | A research pass on 2026-08-01 read `enforceRFC7606` past the validator call | The index moved to `NewAttributesWire` and publication moved to the end of `enforceRFC7606`. A-1 was superseded by the same correction |
| approach | A-1's plan to fold the index into the RFC 7606 walk, and its before/after receive benchmark | There is no fold, so there is no before/after delta to measure. The net walk count is unchanged and the win is one fewer heap allocation per UPDATE | Fell out of A-6 | `BenchmarkReceivePathIndexBuild` was not written; `TestSpanIndexZeroAllocUpToEight` asserts the real change directly |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Build the attribute index exactly once, eagerly, before publication | Changed | `attribute.NewAttributesWire`, `Session.publishBase` | Built at construction rather than inside the RFC 7606 walk (D-1, A-6) |
| Publish it as part of an immutable base | Done | `attribute.AttributesWire`, `wireu.WireUpdate.Attrs` under `attrsOnce` | No new base type (D-2) |
| Retire the `sync.RWMutex` | Done | `internal/core/bgp/attribute/wire.go` | AC-5 grep is empty |
| Move parsed values to a side table | Done | `AttributesWire.parseAtLocked` under `parseMu` | Four writers collapsed to one |
| No emitted byte changes | Done | existing `test/plugin/` corpus unedited | The child is pure addition on the wire |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSpanIndexMatchesAttributesWire`, `FuzzSpanIndexMatchesIterator` (`internal/core/bgp/attribute/span_test.go`) | Oracle is an independent `AttrIterator`, not the retired lazy builder |
| AC-2 | Done | `TestSpanIndexRejectsDuplicateCode`, `TestRFC7606VerdictsUnchangedByEagerIndex` | Nine verdict classes pinned by action, error-ness and emitted payload hex |
| AC-3 | Done | `TestSpanIndexRejectsTruncatedAttribute`, `TestRFC7606VerdictsUnchangedByEagerIndex` | |
| AC-4 | Done | `TestBaseImmutableAfterPublication` (`internal/component/bgp/wireu/base_test.go`), `make ze-race-reactor` | 16 concurrent readers under `-race` |
| AC-5 | Done | `grep -n "sync.RWMutex" internal/core/bgp/attribute/wire.go` returns nothing | |
| AC-6 | Changed | `grep AttrFind internal/component/bgp/reactor/session_validation.go` returns nothing; `test/plugin/prefixsid-ebgp-discard-single-walk.ci` | The second walk was already deleted by `8e67a9b03` before this child started (D-5). The umbrella's unconditional claim is also wrong: that walk was always gated on `!isIBGP && !AcceptSRv6PrefixSID` |
| AC-7 | Done | `TestSnapshotCarriesSpanIndex`, `TestSnapshotOfEmptyAndMalformedUpdates` | Zero allocations on the snapshot's first `Attrs()` |
| AC-8 | Done | `TestSpanIndexZeroAllocUpToEight` | |
| AC-9 | Done | `make ze-test-bgp`, `make ze-functional-test` | No `.ci` expectation edited by this child |
| AC-10 | Changed | superseded with A-1 | No fold into the walk means no receive-path before/after delta. The measurable change is one fewer heap allocation, asserted by AC-8 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every unit row in the TDD table | Done | `span_test.go`, `session_span_index_test.go`, `base_test.go` | Each row's Status column is already filled in the table above |
| `BenchmarkReceivePathIndexBuild` | Skipped | -- | Superseded with A-1; recorded in the Mistake Log |
| four functional rows | Done | `test/plugin/` | Existing files, unedited, still green |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/bgp/attribute/span.go` | Done | |
| `internal/core/bgp/attribute/span_test.go` | Done | |
| `internal/component/bgp/wireu/base.go` | Changed | Not created. D-2: the base is `WireUpdate` + `AttributesWire`; a parallel type would be a second read surface over the same bytes |
| `internal/component/bgp/wireu/base_test.go` | Done | |

### Audit Summary
- **Total items:** 22
- **Done:** 18
- **Partial:** 0
- **Skipped:** 1 (`BenchmarkReceivePathIndexBuild`, superseded with A-1)
- **Changed:** 3 (AC-6, AC-10, `wireu/base.go`) -- each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Build the attribute index exactly once instead of three times | functional + unit | `TestReceivePathPublishesSpanIndex`: a first `Attrs()` after `enforceRFC7606` costs zero allocations, which holds only if the base was already published |
| The index is correct for the bytes actually published | unit | `TestStripRebuildIndexMatchesPublished` and `TestInPlaceDiscardPrecedesIndexBuild` cover both post-walk mutation branches; both fail if `Attrs` is called earlier |
| The forward path takes no lock on any attribute read | unit + race | `TestBaseImmutableAfterPublication` (16 readers, `-race`); `BenchmarkAttributeReadNoLock`; AC-5 grep empty |
| No emitted byte changes | functional | `make ze-functional-test` green with no `.ci` expectation edited by this child |
| One fewer heap allocation per received UPDATE | unit | `TestSpanIndexZeroAllocUpToEight` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none -- `plan/deferrals/spec-wire-edit-1-base-index.md` was never created | done | `ls plan/deferrals/ \| grep wire-edit-1` returns nothing; this child deferred no work |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/wire-edit-1-base-index-<session-id>.md` |
| `review_gate.py check` | clean |
| Reviewer lenses used | correctness/wire/lifetime; tests/security/coverage (two independent agents over `bbd53bf22^..b1fa7ab1e`, 2026-08-02) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The pooled `SpanIndex` discarded its spill slice on every rebuild, so a spilled UPDATE re-allocated on each reuse | `internal/core/bgp/attribute/span.go` `SpanIndex.reset` | `f1f746fb6`, with `TestSpanIndexReuseKeepsSpillCapacity` and `TestSpanIndexErrorPathKeepsCapacityAndReadsEmpty` |
| 2 | ISSUE | The read-buffer leak tests were vacuous at the early-release ORDERING invariant: every assertion was `before == after` over zero borrows | `internal/component/bgp/reactor/forward_readbuf_leak_test.go` | `f1f746fb6`, `TestForwardAdoptedHandleHeldUntilLastWrite` over an IBGP destination on a 2-byte send context |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/bgp/attribute/span.go` | Yes | `grep -n "^func \|^type " internal/core/bgp/attribute/span.go` lists `Span`, `SpanIndex`, `BuildSpanIndex`, `Rebuild`, `reset`, `build`, `add`, `has`, `mark`, `Len`, `Spilled`, `Has`, `At`, `Find`, `findIndex` |
| `internal/core/bgp/attribute/span_test.go` | Yes | holds `TestSpanIndexMatchesAttributesWire` and 9 further `Test`/`Fuzz`/`Benchmark` functions |
| `internal/component/bgp/wireu/base_test.go` | Yes | holds `TestBaseImmutableAfterPublication`, `TestSnapshotCarriesSpanIndex`, `TestSnapshotOfEmptyAndMalformedUpdates`, `TestAPIBuiltBaseAnswersHas` |
| `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Yes | `ls test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |
| `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | Yes | `ls test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |
| `internal/component/bgp/wireu/base.go` | No | Deliberate (D-2). `ls internal/component/bgp/wireu/base.go` returns "No such file"; the base is `WireUpdate` + `AttributesWire` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | spans match an independent oracle | `grep -rl "func TestSpanIndexMatchesAttributesWire(" internal/` -> `internal/core/bgp/attribute/span_test.go`; `make ze-test-core` green |
| AC-4 | no lock on any forward read | `grep -rl "func TestBaseImmutableAfterPublication(" internal/` -> `internal/component/bgp/wireu/base_test.go`; `make ze-test-bgp` green |
| AC-5 | the mutex is gone | `grep -n "sync.RWMutex" internal/core/bgp/attribute/wire.go` prints nothing (re-run 2026-08-02) |
| AC-6 | the PrefixSID lookup does not walk twice | `grep -n "AttrFind" internal/component/bgp/reactor/session_validation.go` prints nothing (re-run 2026-08-02) |
| AC-8 | zero heap allocations at or below 8 spans | `grep -rl "func TestSpanIndexZeroAllocUpToEight(" internal/` -> `internal/core/bgp/attribute/span_test.go`; `make ze-test-core` green |
| AC-9 | the functional corpus is unedited | `git diff --name-only bbd53bf22^..bbd53bf22 -- test/` prints nothing |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A peer sends any UPDATE carrying path attributes | `internal/component/bgp/reactor/session_span_index_test.go` `TestReceivePathPublishesSpanIndex` | Yes -- read: it runs `enforceRFC7606` and asserts the first `Attrs()` allocates nothing, so the base must already be published |
| A consumer calls `Has` for an absent attribute | `internal/core/bgp/attribute/span_test.go` `TestPresenceBitsetAnswersHasWithoutScan` | Yes -- read: asserts the answer with no walk and no allocation |
| eBGP forward unchanged from a same-context peer | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | Yes -- read: pins the shared fast-path wire by hex; unedited by this child |
| iBGP identity forward | `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | Yes -- read: pins an identical forward; unedited by this child |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken (superseded by A-6) | The index is not emitted from the RFC 7606 walk. Net walk count unchanged; the real gain is one heap allocation, asserted by `TestSpanIndexZeroAllocUpToEight`. Mistake Log row filed |
| A-2 | confirmed | Writer set enumerated and closed (five writers, four collapsed into `parseAtLocked`); `make ze-race-reactor` 0 races over 20 iterations; `TestBaseImmutableAfterPublication` under `-race` |
| A-3 | confirmed | `docs/architecture/wire/attributes.md` "Real-World Attribute Count Distribution": 112M MRT routes, 99.9% at or below 8 attributes, maximum 10. `ze_bgp_update_span_spill_total` makes it answerable live |
| A-4 | confirmed | `parsed` was unexported and reachable only from `Get`, `All`, `ForEach`; the package's own `wire_test.go` passes unedited |
| A-5 | confirmed, stronger than assumed | Building in `NewAttributesWire` makes every rail eager with no rail-side call; `TestAPIBuiltBaseAnswersHas` |
| A-6 | broken | The published base is not always the array the validator walked. Resolved by ORDERING, not by rebuilding; `TestStripRebuildIndexMatchesPublished` and `TestInPlaceDiscardPrecedesIndexBuild` fail if `Attrs` is called earlier |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #12 internal architecture: the span index | `docs/architecture/wire/attributes.md` checked against `internal/core/bgp/attribute/span.go` `SpanIndex` -- inline capacity 8, spill past it, 6-byte `Span` | Yes |
| #12 internal architecture: the offset-borrow rule | `docs/architecture/memory/lifetime-contracts.md` checked against `AttributesWire.CarryOver`, which requires byte-identical contents and rebuilds on a length mismatch | Yes |
| #14 Prometheus counters: the spill counter | `ze_bgp_update_span_spill_total` on `reactorMetrics` (`internal/component/bgp/reactor/reactor_metrics.go`), per-peer label | Yes |
| #1-#11, #13, #15, #17 answered No | This child emits no changed byte and adds no config, command, RPC, plugin, family or capability surface; `git diff --name-only bbd53bf22^..bbd53bf22` touches no `.yang` and no `cmd/` | Yes |
| #16 stale source anchors | `grep -rn "wire.go\|iterator.go\|wire_update.go\|session_validation.go" docs/` reviewed; the two architecture pages above are the anchors that named the changed files, and both were updated | Yes |

## Core Insight

The mutex was never protecting shared mutable state the design wanted. It was protecting laziness. Removing the laziness removed the reason for the lock, and the only remaining writer -- the parsed-value cache -- turned out to belong to the show and JSON paths, not to the forward path at all.
