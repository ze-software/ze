# Spec: as-path-prepend encodes at the negotiated width

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.


## Task

The shipped `as-path-prepend` filter action builds its AS_SEQUENCE segment with
four-octet AS numbers unconditionally. `ExtractASPathPrependOps`
(`internal/component/bgp/reactor/filter_delta.go`) carves `2 + n*4` bytes and
writes each copy of the local AS with a 32-bit big-endian store. `aspathHandler`
(`internal/component/bgp/reactor/filter_delta_handlers.go`) then splices that
segment in front of the AS_PATH value already in the payload, byte for byte and
with no transcode.

The payload it splices into is encoded at whatever width the relevant session
negotiated. When that width is two octets, the emitted attribute declares a
segment of four-octet AS numbers over a two-octet payload, and the result is no
longer one decodable AS_PATH.

The sibling extractor 30 lines below, `ExtractRemovePrivateASOps`, already takes
an explicit `asn4` parameter and is handed one at every call site. The prepend
extractor takes none, and none of its three call sites passes one.

The goal is that the prepend segment is encoded at the same width as the payload
it is spliced into, that a local AS above 65535 appears as AS_TRANS in a
two-octet AS_PATH, and that the real four-octet value is then carried in
AS4_PATH as RFC 6793 Section 4.2.2 requires.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` - the AS-path family as generate slots, declared by the `// Design:` header of `internal/component/bgp/wireu/aspath_slot.go`
  → Decision: the AS-path family for one destination is resolved by `ASPathEdit.Record` and recorded as attribute operations, so nothing pre-builds a whole payload.
  → Constraint: the AS4_PATH question has ONE owner, `internal/component/bgp/wireu/aspath_as4.go`, so that the rule "does this UPDATE need an AS4_PATH, and what goes in it" cannot drift between rails.
- [ ] `docs/architecture/encoding-context.md` - what `ContextID` and `ASN4()` mean for a forwarded payload
  → Constraint: a payload carries the encoding context it was produced under; the same bytes forwarded to a peer with a different context need a transcode.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6793.md` - four-octet AS number space, AS4_PATH, AS_TRANS
  → Constraint: RFC 6793 Section 4.2.2, read at `rfc/full/rfc6793.txt`: "When communicating with an OLD BGP speaker, a NEW BGP speaker MUST send the AS path information in the AS_PATH attribute encoded with two-octet AS numbers. The NEW BGP speaker MUST also send the AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers), except for the case where all of the AS path information is composed of mappable four-octet AS numbers only. In this case, the NEW BGP speaker MUST NOT send the AS4_PATH attribute."
  → Constraint: RFC 6793 Section 4.2.2: "In the AS_PATH attribute encoded with two-octet AS numbers, non-mappable four-octet AS numbers are represented by the well-known two-octet AS number, AS_TRANS."
  → Constraint: RFC 6793 Section 4.2.3: "If the number of AS numbers in the AS_PATH attribute is larger than or equal to the number of AS numbers in the AS4_PATH attribute, then the AS path information SHALL be constructed by taking as many AS numbers and path segments as necessary from the leading part of the AS_PATH attribute, and then prepending them to the AS4_PATH attribute so that the AS path information has a number of AS numbers identical to that of the AS_PATH attribute."
- [ ] `rfc/short/rfc4271.md` - AS_PATH encoding and the prepend obligation
  → Constraint: RFC 4271 Section 4.3 encodes each AS_PATH segment as type(1) + length(1) + a list of AS numbers, and the segment length counts AS numbers rather than octets, so the reader's octet count comes from the negotiated width alone.

**Key insights:**
- The reconstruction rule in Section 4.2.3 takes the LEADING part of AS_PATH. A prepend therefore lands where the reconstruction reads, which is why a MAPPABLE local AS needs no AS4_PATH edit and a NON-MAPPABLE one needs the real value carried in AS4_PATH.
- `attribute.ASPath.WriteToWithASN4` is Ze's only implementation of AS_TRANS substitution inside AS_PATH.
- `wireu.as4PathForRewrite` is Ze's only implementation of "does this prepend need an AS4_PATH, and what goes in it".

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `ExtractASPathPrependOps` parses the `as-path-prepend N` directive, carves `2 + n*4` bytes, writes the segment type, the count, and N four-octet copies of `localAS`, and records one `AttrModPrepend` operation on attribute code 2. It takes no width parameter. `ExtractRemovePrivateASOps`, in the same file, takes `asn4 bool` and re-encodes AS_PATH through `attribute.ASPath.LenWithASN4` and `WriteToWithASN4`.
- [ ] `internal/component/bgp/reactor/filter_delta_handlers.go` - `aspathHandler` emits every `AttrModPrepend` buffer first, then the last Set value or the source value where it already sits, then the attribute header. It copies bytes; it does not parse, transcode, or check a width.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - the import call site (`runIngressPolicyChain`) passes `peer.settings.LocalAS` and nothing else; the line above it passes `srcASN4`, read from the encoding context of `wireUpdate.SourceCtxID()`, to the sibling. The export call site (`runEgressPolicyChainASN4`) passes `destLocalAS` and nothing else; the line above it passes the function's `asn4` parameter to the sibling.
- [ ] `internal/component/bgp/reactor/egress_inject_filter.go` - `exportFilterForBody` calls `runEgressPolicyChainASN4` with `facts.sendASN4`, and its own comment states why: "The body was encoded by the session write path in THIS peer's SEND context, so that is the context its attributes must be parsed under -- and likewise why asn4 is facts.sendASN4 rather than a source-context lookup."
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` - `computeWireChanges` is a third call site. It already holds an `asn4` parameter, hands it to the sibling, and hands the prepend extractor only `localAS`.
- [ ] `internal/component/bgp/reactor/session_write.go` - `writeUpdateGated` and the announce writer both call `egressRouteFilter` and, when it returns an override, replace `body` with it. `writeRawUpdateBody` copies `body` verbatim behind a BGP header into the session write buffer. Nothing between the filter and the socket parses or re-encodes the AS_PATH.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - on the forwarded rail the policy chain's override becomes `peerBaseWire`, and then, only when `facts.isEBGP`, `aspathEdit.Record` is called with `SrcASN4: peerBaseSrcASN4` and `DstASN4: facts.sendASN4`. `peerBaseSrcASN4` is read from the encoding context of `peerBaseWire.SourceCtxID()`, which the policy override preserves.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - on the import rail `res.modifiedPayload` replaces the payload, a new `WireUpdate` is built over it with the SOURCE context id, and that becomes `msg.RawBytes`, `msg.WireUpdate` and the cached `ReceivedUpdate`.
- [ ] `internal/core/bgp/attribute/aspath.go` - `WriteToWithASN4` delegates each segment to `writeSegmentWithSplit`, which writes four octets per AS number when `asn4` is true and two octets when it is false, substituting 23456 for any AS number above 65535. `ParseASPath(data, fourByte)` reads a segment header then `count` AS numbers of the stated width.
- [ ] `internal/component/bgp/wireu/aspath_as4.go` - `as4PathForRewrite(prepended, recvAS4, asns, srcASN4, dstASN4)` returns the AS4_PATH to emit after `asns` have been prepended, or nil when none is required. It is unexported.
- [ ] `internal/component/bgp/wireu/aspath_slot.go` - `ASPathEdit.recordPrepend` is the ordinary prepend path. It parses the existing AS_PATH at `SrcASN4`, prepends, encodes at `DstASN4`, and calls `as4PathForRewrite` for the AS4_PATH half.
- [ ] `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` - the `as-path-prepend` leaf is the operator-facing surface that reaches the extractor.

**Behavior to preserve:**
- The prepend stays an `AttrModPrepend` operation on attribute code 2, so `aspathHandler` keeps composing it with a Set from another extractor (prepends first, then the Set value or the source value).
- The count validation stays: an unparseable count, zero, or a count above 32 records nothing and logs one warning.
- An absent `as-path-prepend` directive still records nothing.
- The mappable-local-AS case at four-octet width keeps the exact bytes it emits today.

**Behavior to change:**
- The prepend segment is encoded at the width of the payload it is spliced into, rather than always at four octets.
- A local AS above 65535 prepended at two-octet width appears as AS_TRANS (23456) in AS_PATH, and the real value is carried in a recorded AS4_PATH operation.

## Data Flow (MANDATORY)

### Entry Point
- An operator configures `as-path-prepend N` under a `filter_modify` policy (`internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang`).
- The plugin returns the directive as the synthetic token `as-path-prepend N` inside the modified filter text.

### Transformation Path
1. `PolicyFilterChain` returns modified text; `parseFilterAttrsInto` parses it into `filterAttrs`.
2. `ExtractASPathPrependOps` reads the directive and records attribute operations on a `filterapi.ModAccumulator`.
3. `buildModifiedPayload` runs the registered handlers; `aspathHandler` emits AS_PATH, and the generic set handler emits AS4_PATH.
4. The rebuilt payload becomes either the import chain's replacement payload or the export chain's wire override.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ reactor | Filter text carrying `as-path-prepend N` | Yes -- `filter_modify.go` writes the token, `filter_chain.go` names it `policyAttrASPathPrepend` |
| Reactor ↔ wire | Rebuilt UPDATE body handed to `writeRawUpdateBody` | Yes -- `session_write.go` copies the override verbatim |
| Reactor ↔ local store | Rebuilt UPDATE body cached as `ReceivedUpdate` | Yes -- `reactor_notify.go` builds the cached entry from `res.modifiedPayload` |
| Reactor ↔ `wireu` | The AS4_PATH derivation rule | No -- `as4PathForRewrite` is unexported today; this spec exports it |

### Integration Points
- `attribute.ASPath.LenWithASN4` / `WriteToWithASN4` - the AS_TRANS substitution the sibling extractor already reuses.
- `wireu.as4PathForRewrite` - the AS4_PATH derivation, exported by this spec and left as the single declaration.
- `filterapi.ModAccumulator` - the operation list both extractors record on.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The prepend stays an accumulator operation resolved by the registered handler; nothing writes a payload directly |
| No unintended coupling (components stay isolated) | Yes | `reactor` already imports `wireu` (`filter_ordered.go`, `reactor_api_forward.go`); exporting one function adds no new edge |
| No duplicated functionality (extends existing, does not recreate) | Yes | AS_TRANS comes from `writeSegmentWithSplit`, the AS4_PATH rule from `as4PathForRewrite`; this spec writes neither |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The AS_PATH operation still carries only the new segment, so `aspathHandler` keeps the source value where it already sits |
| Registration over hardcoding | N-A | No new command, view, family, or handler; the AS_PATH and AS4_PATH handlers are already registered in `attrModHandlersWithDefaults` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three call sites each hold, at the point of the call, the width of the payload the prepend is spliced into | `filter_ordered.go` reads `srcASN4` from `wireUpdate.SourceCtxID()` on import and takes `asn4` as a parameter on export; `policy_dryrun.go` takes `asn4` as a parameter | The fix would encode against the wrong width and change nothing | Reading the producer of each value, then the unit tests at both widths | unvalidated |
| A-2 | Nothing between the export filter override and the socket re-encodes the AS_PATH | `session_write.go` `writeRawUpdateBody` copies `body` behind a header; `peer_run.go` installs `exportFilterForBody` as `egressRouteFilter` | The wire claim in Blast Radius is too strong and must be reduced to a local-store claim | Reading `writeRawUpdateBody` and both `egressRouteFilter` call sites | unvalidated |
| A-3 | A mappable local AS prepended at two-octet width needs no AS4_PATH edit | RFC 6793 Section 4.2.3 reconstructs by taking the LEADING part of AS_PATH, which is where the prepend lands | The fix would leave a count skew for every mappable prepend | `as4PathForRewrite` returning nil for that case, asserted in the unit test | unvalidated |
| A-4 | `ExtractRemovePrivateASOps` records an AS4_PATH operation only when the payload already carries an AS4_PATH | `filter_delta.go` guards that branch on `len(rawAS4Path) > 0` | The two extractors could both record an AS4_PATH Set and the last one would win | Reading the guard, plus the combination test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The prepend's AS4_PATH Set overrides an AS4_PATH Set that `ExtractRemovePrivateASOps` recorded first, restoring a private ASN the policy stripped | A combination test where both directives are configured shows the private ASN back in AS4_PATH | The prepend derives its AS4_PATH from the accumulator's last AS_PATH / AS4_PATH Set when one exists, and from the wire attributes otherwise |
| R-2 | Changing the extractor signature breaks an RFC-tagged test | `./le rfc check` refuses the commit | None of the twelve test references to `ExtractASPathPrependOps` carries an `RFC requirement:` tag; they are all in `filter_delta_test.go`, which carries none |
| R-3 | The four-octet width case changes bytes it should not | The existing `TestExtractASPathPrependOps` subtests go red | Those subtests are kept and re-pointed at the four-octet width, so the current bytes stay asserted |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Three distinct failures, with different reach. **On the wire:** an originated, injected, or `update text` re-advertised route leaving through `exportFilterForBody` toward a peer that did not negotiate four-octet support carries an AS_PATH the peer cannot parse. RFC 4271 Section 6.3 makes a malformed AS_PATH an UPDATE Message Error, so the peer answers with a NOTIFICATION and the session resets. That is a session drop, not a cosmetic wrongness. **In the local store:** a route arriving on a session with a two-octet source and rewritten by an import chain is cached as a `ReceivedUpdate` whose AS_PATH does not decode at its own context's width, and every consumer and every per-destination re-encode reads it. **As a blackhole:** on the forwarded rail toward an EBGP destination, `aspathEdit.Record` parses the spliced payload at the source width, fails, and the route is dropped for that destination with one warning |
| How is it reverted? | A single commit revert. No config migration, no on-disk format, and no peer-visible state that survives a session reset |
| Since when? | 2026-04-12, commit `2e32a59efc` "feat(bgp): wire AS-path prepend and export-path modify", which introduced the extractor with `wireLen := 2 + n*4` and the 32-bit store on day one of the feature |
| Who else touches this path? | `internal/component/bgp/reactor` has taken six commits on 2026-09-06 alone. The baseline for this work is `e5e3e84dccc9b32aa4f9d097e6979522b7fe51d9`, with `2175296427` the last commit touching the package. `filter_delta_handlers.go` carries an uncommitted hunk from another session changing the AIGP flag byte, so it is not named in this spec's commits |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Import chain: `runIngressPolicyChain` passes the source session width | → | `ExtractASPathPrependOps` | `TestImportPrependEncodesAtTheSourceASNWidth` |
| Export chain: `runEgressPolicyChainASN4` passes the width of the payload it edits | → | `ExtractASPathPrependOps` | `TestExportPrependEncodesAtTheDestinationASNWidth` |
| Dry run: `computeWireChanges` passes its own `asn4` | → | `ExtractASPathPrependOps` | `TestPolicyDryRunPrependReportsAtTheSessionWidth` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `as-path-prepend 3`, local AS 65000, payload width four octets | The recorded AS_PATH operation carries 14 bytes: segment type 2, count 3, and three four-octet copies of 65000. No AS4_PATH operation is recorded |
| AC-2 | `as-path-prepend 3`, local AS 65000, payload width two octets | The recorded AS_PATH operation carries 8 bytes: segment type 2, count 3, and three two-octet copies of 65000. No AS4_PATH operation is recorded, because every AS number in the path is mappable |
| AC-3 | `as-path-prepend 2`, local AS 4200000000, payload width two octets, payload AS_PATH holds one mappable AS number and no AS4_PATH | The AS_PATH operation carries segment type 2, count 2, and two copies of 23456. An AS4_PATH operation is recorded carrying the whole path with the real 4200000000 at its leading edge |
| AC-4 | `as-path-prepend 1`, local AS 4200000000, payload width two octets, payload already carries an AS4_PATH | The recorded AS4_PATH holds the real local AS prepended to the received AS4_PATH, so the AS number count of AS_PATH minus that of AS4_PATH is unchanged by the prepend |
| AC-5 | `as-path-prepend 1`, local AS 4200000000, payload width four octets | The AS_PATH operation carries the real 4200000000 in four octets, and no AS4_PATH operation is recorded |
| AC-6 | The import chain runs a filter returning `as-path-prepend N` on a session whose source context did not negotiate four-octet support | The rebuilt payload's AS_PATH parses at two-octet width and its leading segment holds N copies of the local AS |
| AC-7 | The export chain runs a filter returning `as-path-prepend N` for a destination whose payload is two-octet | The wire override's AS_PATH parses at two-octet width and its leading segment holds N copies of the local AS |
| AC-8 | Both `remove-private` and `as-path-prepend N` are configured, local AS above 65535, payload width two octets, payload carries an AS4_PATH holding a private ASN | The recorded AS4_PATH holds neither the private ASN nor a restored copy of it, and holds the real local AS at its leading edge |
| AC-9 | Count zero, count above 32, an unparseable count, or no directive at all | Nothing is recorded, at either width |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `as-path-prepend` toward a peer that did not negotiate four-octet support, and the peer keeps the session up | filter text → `ExtractASPathPrependOps` → `aspathHandler` → `buildModifiedPayload` → `writeRawUpdateBody` | `TestExportPrependEncodesAtTheDestinationASNWidth` |
| 2 | Configures `as-path-prepend` on an import policy on a two-octet session, and the stored route still decodes | filter text → `ExtractASPathPrependOps` → `buildModifiedPayload` → `ReceivedUpdate` | `TestImportPrependEncodesAtTheSourceASNWidth` |
| 3 | Runs a four-octet local AS and prepends toward a two-octet peer | filter text → `ExtractASPathPrependOps` → AS_PATH holding AS_TRANS plus AS4_PATH holding the real value | `TestPrependAtTwoOctetWidthCarriesTheRealASNInAS4Path` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractASPathPrependOps` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-1, AC-9 -- the existing subtests, re-pointed at the four-octet width | |
| `TestPrependAtTwoOctetWidthEncodesTwoOctetASNs` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-2 | |
| `TestPrependAtTwoOctetWidthCarriesTheRealASNInAS4Path` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-3, AC-4, AC-5 | |
| `TestPrependAS4PathDoesNotUndoRemovePrivateAS` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-8 | |
| `TestImportPrependEncodesAtTheSourceASNWidth` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-6, wiring | |
| `TestExportPrependEncodesAtTheDestinationASNWidth` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-7, wiring | |
| `TestPolicyDryRunPrependReportsAtTheSessionWidth` | `internal/component/bgp/reactor/filter_delta_test.go` | The third call site passes its width | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `as-path-prepend` count | 1-32 | 32 | 0 | 33 |
| local AS at two-octet width | 0-4294967295 | 65535 encodes as itself | N/A | 65536 encodes as 23456 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | The directive's end-to-end reach is already covered by the policy `.ci` suite; this spec changes the ENCODING of an existing action rather than adding a surface, and the encoding is what a byte-level unit test asserts | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `as-path-prepend-two-octet-peer` | `test/interop/scenarios/` | FRR | An `as-path-prepend` policy toward a peer that did not send the four-octet AS capability keeps the session established and produces a path FRR decodes | not run |

## Files to Modify
- `internal/component/bgp/reactor/filter_delta.go` - `ExtractASPathPrependOps` takes the wire attributes and the width, encodes at that width, and records the AS4_PATH operation when RFC 6793 Section 4.2.2 requires one
- `internal/component/bgp/reactor/filter_ordered.go` - both call sites pass the width they already hold
- `internal/component/bgp/reactor/policy_dryrun.go` - the third call site passes its `asn4`
- `internal/component/bgp/wireu/aspath_as4.go` - `as4PathForRewrite` becomes exported so the reactor reuses the one declaration
- `internal/component/bgp/wireu/aspath_slot.go` - its call of that function follows the rename
- `internal/component/bgp/reactor/filter_delta_test.go` - the new tests, and the existing ones re-pointed at the four-octet width

## Files to Create
- None

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The `as-path-prepend` leaf already exists in `ze-filter-modify.yang`; this changes how its value is encoded, not the surface |
| YANG validation constraints | No | The 1-32 range is unchanged |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | No command added or changed |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | No | No new RPC |
| Pipe completeness | N-A | No command output added |
| Env var registration | No | No environment leaf |
| Doctor check for runtime dependencies | No | No new path, socket, service, module, port, or binary |
| Prometheus counters/metrics | No | No new observable state; the existing modify-failure counter already covers a rebuild that fails |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability, or attribute code; AS_PATH and AS4_PATH are both already handled |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The action exists; its encoding is corrected |
| 2 | Config syntax changed? | No | The leaf and its range are unchanged |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | `filter_modify` is unchanged |
| 6 | Has a user guide page? | No | The prepend action's guide text describes the action, not its octet width |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` -- the AS-path family page, declared by the `// Design:` header of `aspath_slot.go`, gains the statement that the policy prepend is encoded at the width of the payload it edits |
| 8 | Plugin SDK/protocol changed? | No | The filter text token is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc6793.md` and the `docs/features/rfc-status.md` row for RFC 6793, if and only if the tagged-test decision below adds a proof |
| 10 | Test infrastructure changed? | No | No runner or harness change |
| 11 | Affects daemon comparison? | No | No feature gained or lost |
| 12 | Internal architecture changed? | No | The extractor keeps its shape and its accumulator |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors spec plan/immediate/spec-as-path-prepend-encodes-at-the-negotiated-width.md`. Two pages are DECLARED by the `// Design:` headers of the files this spec changes, and both are named here as unaffected. `docs/architecture/core-design.md` is declared by `filter_delta.go` ("policy filter wire-level dirty tracking") and by `policy_dryrun.go` ("policy filter chain"): it describes the dirty-tracking mechanism and the chain's ordering, and this spec changes neither, only the octet width one extractor encodes at. `docs/architecture/api/architecture.md` is declared by `filter_ordered.go` ("unified BGP route filter pipeline"): it describes the stage-ordered pipeline and the extractors it runs, and this spec adds an argument to one of those calls without adding, removing, or reordering a stage |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The prepend examples show the directive, not its bytes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the three call sites reach the extractor with a width
   - Tests: `TestImportPrependEncodesAtTheSourceASNWidth`, `TestExportPrependEncodesAtTheDestinationASNWidth`, `TestPolicyDryRunPrependReportsAtTheSessionWidth`
   - Files: `filter_ordered.go`, `policy_dryrun.go`, `filter_delta.go`
   - Verify: each test fails against the current code because the emitted segment is four-octet whatever the width
2. **Phase: AS_PATH at the negotiated width**
   - Tests: `TestPrependAtTwoOctetWidthEncodesTwoOctetASNs`, `TestExtractASPathPrependOps`
   - Files: `filter_delta.go`
   - Verify: the segment is built through `attribute.ASPath.LenWithASN4` and `WriteToWithASN4`, so AS_TRANS comes from the one place that already implements it
3. **Phase: AS4_PATH where RFC 6793 Section 4.2.2 requires it**
   - Tests: `TestPrependAtTwoOctetWidthCarriesTheRealASNInAS4Path`, `TestPrependAS4PathDoesNotUndoRemovePrivateAS`
   - Files: `wireu/aspath_as4.go`, `wireu/aspath_slot.go`, `filter_delta.go`
   - Verify: the decision and the value both come from the exported `as4PathForRewrite`, and no second implementation of the rule appears in the reactor

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and all three call sites pass a width |
| Feature completeness | The dry-run site is not forgotten: it reports to an operator what the runtime will do, so a width mismatch there is a lie in the CLI |
| Correctness | The AS4_PATH decision is `as4PathForRewrite`'s and not a re-derived condition; the mappable case records no AS4_PATH operation |
| Naming | The new parameter is named `asn4`, matching the sibling extractor rather than inventing a second name for the same fact |
| Data flow | The prepend stays an `AttrModPrepend` operation, so the source AS_PATH is still kept where it sits rather than copied |
| Rule: `ai/rules/principles.md` | AS_TRANS is declared once (`writeSegmentWithSplit`) and the AS4_PATH rule once (`as4PathForRewrite`); neither is restated in the reactor |
| Rule: `ai/rules/rfc-compliance.md` | Each enforced MUST carries a comment naming the RFC section and quoting it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The extractor takes a width | `gopls symbols internal/component/bgp/reactor/filter_delta.go \| grep ExtractASPathPrependOps` |
| All three call sites pass one | `grep -rn 'ExtractASPathPrependOps' --include=*.go internal/ \| grep -v _test.go` |
| No second AS_TRANS implementation | `grep -rn '23456' --include=*.go internal/component/bgp/reactor/filter_delta.go` returns nothing |
| Tests pass | `go test ./internal/component/bgp/reactor/... -run 'Prepend'` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The count comes from filter text a plugin produced. It stays bounded to 1-32 before any buffer is carved, so the carve size stays bounded at both widths |
| Resource exhaustion | The AS4_PATH derivation parses the received AS_PATH and AS4_PATH, both already length-bounded by `ParseASPath` and `ParseAS4Path`, which enforce `MaxASPathTotalLength` |
| Fail closed | A parse failure while deriving the AS4_PATH records no operation and logs, matching the sibling extractor. It never records a half-formed AS-path family |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| An existing RFC-tagged test changes | STOP and report; a row in `test/rfc-changed.md` is the owner's decision |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The `asn4` a call site passes is not "import versus export" and not "source versus destination". It is **the width of the payload being edited**, and each call site derives it from wherever that payload came from. `runEgressPolicyChain` reads the SOURCE context because a forwarded wire is still in the source's encoding. `exportFilterForBody` reads `facts.sendASN4` because the session write path has already encoded the body in the destination's send context. The import site reads the source context for the same reason as the first. That single reading explains all three, and it is the reading the fix must copy.
- RFC 6793 Section 4.2.3 reconstructs from the LEADING part of AS_PATH, which is exactly where a prepend lands. That is why a mappable local AS needs no AS4_PATH edit at all: the reconstruction picks the prepended AS numbers up from AS_PATH by itself.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep the AS_PATH edit as an `AttrModPrepend` operation carrying only the new segment | Emit a full `AttrModSet` carrying the whole re-encoded path | The Set form would make `aspathHandler`'s prepend branch dead code, which `ai/rules/no-layering.md` would then require deleting along with its test, for no correctness gain. The Prepend form also keeps the source AS_PATH bytes where they already sit |
| Export `as4PathForRewrite` from `wireu` rather than deriving the AS4_PATH in the reactor | Re-derive the condition in `filter_delta.go` from `localAS > 65535` and the presence of a received AS4_PATH | A second statement of the rule is a second declaration of the same fact (`ai/rules/principles.md`). The exported function also returns nil for exactly the cases that need no edit, so the caller needs no condition of its own |
| Reuse `attribute.ASPath.WriteToWithASN4` for the segment | Write a two-octet loop with an inline 23456 substitution | `writeSegmentWithSplit` is Ze's only AS_TRANS implementation inside AS_PATH, and the sibling extractor already reaches it the same way |
| Derive the AS4_PATH from the accumulator's last AS_PATH / AS4_PATH Set when one exists | Always derive from the wire attributes | `ExtractRemovePrivateASOps` runs first at all three sites and can Set both attributes. Deriving from the wire would override its work and restore a private ASN |

## Known Limitations

- The forwarded rail toward a NON-EBGP destination does not call `aspathEdit.Record` at all (`reactor_api_forward.go` guards it on `facts.isEBGP`), so a width difference between an iBGP source and an iBGP destination is not transcoded on that rail. That is a separate question from this spec's defect, it predates it, and this spec neither fixes nor relies on it.
- The interop scenario named above is written but has never been run in this session.

## RFC Documentation (Scope: protocol)

`// RFC 6793 Section 4.2.2: "..."` above the two-octet encoding choice and above
the AS4_PATH recording. `// RFC 6793 Section 4.2.3: "..."` above the comment
explaining why a mappable local AS needs no AS4_PATH edit.

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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

## Progress, 2026-09-06

The spec landed in `9140ccc610`. The implementation is in flight and
incomplete: the extractor signature changed and three call sites do not
compile, at `filter_ordered.go:227`, `filter_ordered.go:372` and
`policy_dryrun.go:240`.
