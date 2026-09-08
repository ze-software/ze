# Spec: test-peer-open-mirrors-five-more-sender-facts

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-test-peer-open-inherits-zes-identity` states one property: **every fact
ze-peer's OPEN asserts about ze-peer is resolved once from the test's own
configuration, and no octet of that fact is inherited from ze's OPEN.** It
implemented that property for the AS, the BGP Identifier, the Role (capability
9), the ADD-PATH directions (69) and the FQDN (73). Five sender-describing
values are still mirrored, so the property does not yet hold, and this spec is
the rest of it. This spec's provenance is that one, and it exists because an
in-scope item a spec does not do becomes a spec of its own rather than a note
(`ai/rules/planning.md`).

The five are Graceful Restart (64), Long-Lived Graceful Restart (70 and 71),
software version (75), PATHS-LIMIT (76), and the two-octet Hold Time of the OPEN
body, which no `option=` can set today.

Graceful Restart is the one verified at its producer. `Negotiate`
(`internal/core/bgp/capability/negotiated.go`) stores the REMOTE speaker's
Graceful Restart capability whole, and `runPeer`
(`internal/component/bgp/reactor/peer_run.go`) feeds
`neg.GracefulRestart.RestartTime` into `startEORTimer`. A mirrored capability
therefore makes ze time the PEER's restart by ze's OWN configured restart time,
and every graceful-restart `.ci` in the tree is measuring ze against itself.

The other four are NOT producer-verified, and the first work this spec owes is
that verification: for each one, read the function that consumes the received
capability and state what a mirrored value makes ze believe. Software version
(75) is the loudest by inspection, because ze-peer reports ze's own build as its
own, but "loudest" is a reading of the code rather than a reading of its
consumer.

The reason none of the five was done in the first spec is recorded there and is
not a scope judgement to repeat here: no acceptance criterion reached them, and
changing the restart time changes what `startEORTimer` waits for in every
graceful-restart test. That blast radius is what this spec has to size and
absorb.

The class is recorded in
`plan/journal/mirrored-field-asserts-the-wrong-sender.md`, which stays: the
journal holds the pattern, this spec holds the work.

### Corrections established in research (2026-09-08)

The Task text above is kept as written. Three of its statements are wrong or are
now superseded, and the design below follows these corrections rather than those
paragraphs.

| Task statement | Correction | Evidence |
|----------------|-----------|----------|
| "Long-Lived Graceful Restart (70 and 71)" | LLGR is code **71 only**. Code 70 is Enhanced Route Refresh, which describes the session rather than the sender | `CodeEnhancedRouteRefresh`, declared as 70 with an RFC 7313 citation, in `internal/core/bgp/capability/capability.go`. That package holds no LLGR constant at all: the `gr` plugin owns code 71 |
| "The other four are NOT producer-verified, and the first work this spec owes is that verification" | All five are now producer-verified. The verification is DONE and is written into Current Behavior below. This spec's work starts at the design | the consumer table in Current Behavior, each row read at its producing function |
| "the blast radius ... is what this spec has to size and absorb" | Sized, and the answer changes the spec's shape: the graceful-restart receiving path is not merely mirrored, it is UNREACHABLE from every functional test. Swapping five field values leaves the tests green over dead code | the measurement in Current Behavior, "The GR receiving path is unreachable" |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` format reference, and
  the "Capability Control" section that states which values ze-peer owns
  → Constraint: the section is ACCURATE today, naming 9, 65, 69 and 73 as the
  resolved facts, so it becomes wrong the moment the first fact of this spec
  lands. Its edit belongs in the same change as the code
  (`ai/rules/documentation.md`), never at closure
  → Decision: `add-capability` REPLACES a resolved capability rather than adding a
  second one of the same code. Every newly owned code inherits that rule, which is
  what keeps a `.ci` that drops a code and adds it back unchanged
- [ ] `ai/rules/principles.md` - the zero-value and single-declaration directives
  → Constraint: single-declaration is the whole spec. A fact declared in ze's
  configuration and re-read out of ze's OPEN is a second declaration with nothing
  to arbitrate it
  → Decision: an empty family list read as "no families to mark stale" is the
  zero-value failure that rule names. `onSessionDown` returns false on it, and no
  caller can tell that from "GR ran and found nothing"
- [ ] `ai/patterns/functional-test.md` - the structure the new `.ci` take
  → Constraint: each new `.ci` asserts what ze DOES with the peer's value, never
  the octets ze-peer wrote. An OPEN-hex assertion passes over a dead consumer,
  which is the failure this spec exists to remove

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart, the Restart Time field, and what
  a receiver does with it
  → Constraint: the Restart Time is the SENDER's, and Section 3 pairs it with a
  list of `<AFI, SAFI, Flags>` tuples. The timer value alone describes nothing a
  receiver can act on, because the tuples decide which families go stale
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart and its stale time
  → Constraint: code 71 carries 7-octet per-family tuples, holding AFI, SAFI,
  flags and a 24-bit stale time. The F-bit and the stale time are both the
  sender's declarations
- [ ] `rfc/short/rfc4271.md` - the OPEN Hold Time field and its negotiation
  → Constraint: Section 4.2 makes the negotiated hold time the smaller of the two
  advertised values. A mirrored Hold Time makes the two values equal, so the
  min-selection has no work to do and has never been exercised
- [ ] `rfc/short/draft-abraitis-idr-addpath-paths-limit.md` - PATHS-LIMIT (76)
  → Constraint: the limit is the SENDER's declaration of how many paths it will
  accept, and the receiver is the side that must respect it. Mirrored, ze
  constrains its own sending by its own limit
- [ ] `rfc/short/rfc9072.md` - extended OPEN parameter framing
  → Constraint: `encodeOpen` already handles it. Owned capabilities grow the
  parameter block, so the 255-octet threshold is now reachable by tests that never
  reached it

**Key insights:** (minimal context to resume after compaction)
- ze-peer builds its OPEN by MIRRORING ze's OPEN. `ownedCapabilities`
  (`internal/test/peer/open.go`) resolves codes 65, 9, 69 and 73;
  `reconcileParams` copies every other TLV verbatim; `encodeOpen` copies the Hold
  Time out of ze's body.
- Five facts are therefore ze's own, asserted in ze-peer's name.
- The GR receiving path is not merely mirrored: it is unreachable. Ze's own code
  64 carries ZERO families, so `onSessionDown` returns at its empty-family guard
  and `retain-routes`, `mark-stale`, `purge-stale` and LLGR are never dispatched.
- The work is therefore larger than five field swaps: for code 64 the harness must
  own the FAMILIES and FLAGS as well as the timer, or the tests stay green either
  way.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE the design is written)
- [ ] `internal/test/peer/open.go` - `ownedCapabilities` resolves codes 65, 9, 69
  and 73 and mirrors every other capability; `reconcileParams` copies each unowned
  TLV; `encodeOpen` writes the fixed body and copies the Hold Time out of ze's,
  under a comment reading "mirrored: no .ci declares one"
  → Constraint: `openIdentity` holds only `as` and `routerID`. Every newly owned
  fact needs a home, and `openIdentity` is where the resolved sender-facts live
  → Decision: `reconcileParams` skips a code the `.ci` stated, so an
  `add-capability` already replaces a resolved capability. Newly owned codes need
  no new precedence rule
- [ ] `internal/test/peer/peer.go` - `Config` carries `OpenAS`, `Port`, `Mode`,
  `Linger`, `Silent`, `CapabilityOverrides` and the rest. It has NO hold-time field
  and no graceful-restart, LLGR, software-version or paths-limit field
  → Constraint: five new facts need five `Config` declarations plus the `option=`
  parsing that fills them
- [ ] `internal/test/peer/expect.go` - the `option=open:` switch accepts exactly
  `send-unknown-capability`, `inspect-open-message`, `send-unknown-message`,
  `drop-capability`, `router-id` and `add-capability`
  → Constraint: the Hold Time is unreachable from a `.ci` by any route. It is not a
  mirrored value with an escape hatch; there is no hatch
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate` stores the remote
  Graceful Restart capability whole; `negotiatePathsLimit` fills `pathsLimitSend`,
  whose own comment reads "Remote's limits (constrains our send)"
  → Constraint: PATHS-LIMIT has a real consumer, so a mirrored 76 makes ze police
  its own sending by its own number
- [ ] `internal/component/bgp/reactor/peer_run.go` - `runPeer` feeds
  `neg.GracefulRestart.RestartTime` into `sessionHealth.startEORTimer`
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `reactorAPIAdapter.Peers`
  copies `neg.GracefulRestart.RestartTime` into `GRRestartTime`, which is what
  `show bgp peer` prints
  → Decision: a display consumer, so a mirrored 64 makes the CLI report ze's own
  restart time as the peer's. No test asserts it
- [ ] `internal/component/bgp/plugins/gr/gr.go` - `handleStateEvent` routes peer
  up and down into the state manager; `parseGRCapValue` builds the code-64 value ze
  ADVERTISES; `decodeGR` parses a received one
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionDown`,
  `onSessionReestablished` and `enterLLGRLocked`
- [ ] `internal/component/bgp/plugins/gr/gr_llgr.go` - `parseLLGRCapValue` builds
  the code-71 value as 7-octet tuples: AFI, SAFI, a flags octet with the F-bit set,
  and a 24-bit stale time, one tuple per negotiated family
  → Decision: code 71 DOES carry families and the F-bit. Only code 64 carries an
  empty list, so the LLGR row of this spec is about the stale time and the F-bit,
  and the family LIST is a separate inheritance: ze's negotiated set asserted as
  the peer's
- [ ] `internal/component/bgp/plugins/softver/softver.go` - `decodeSoftwareVersion`
  → Decision: reached only from `capabilityToZeJSON`
  (`internal/component/bgp/cli/decode_open.go`), which serves the offline
  `ze bgp decode` CLI. `capability.Parse` has no code 75 arm, so a received 75
  never reaches a session decision
- [ ] `internal/component/bgp/rib/commit.go` - `CommitService.enforcePathsLimit`
  drops paths past the PEER's declared limit
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `RIBManager.handleState` calls
  `collectPeerUpReplay` and `replayRoutesWithCursor` on the down-to-up edge
  → Constraint: this is why the graceful-restart `.ci` pass with GR dead. The
  re-announcement they assert comes from the `bgp-rib` replay, not from GR

### The five mirrored facts and their consumers

| Fact | What ze does with the received value | Tests over it today | Reachable from a `.ci` |
|------|--------------------------------------|---------------------|------------------------|
| GR restart time (64) | `runPeer` to `sessionHealth.startEORTimer` (a warning only); `grPlugin.handleStateEvent` to `grStateManager.onSessionDown` (the restart timer and `mark-stale`); `reactorAPIAdapter.Peers` to `show bgp peer` | 32 `.ci` put a mirrored 64 on the wire. 13 name a `restart-time`, 19 take the YANG default of 120 | Yes and unused: `add-capability:code=64:hex=` replaces the mirror, because `reconcileParams` skips a stated code |
| LLGR stale time and F-bit (71) | `enterLLGRLocked` arms one `time.AfterFunc` per family on the stale time; `onSessionReestablished` reads the F-bits | 5, all under `test/plugin/`: `llgr-transition`, `llgr-rib-stale`, `llgr-readvertise`, `llgr-readvertise-multipeer`, `llgr-egress-state-unloaded` | Yes and unused |
| Software version (75) | Display only, and only offline. `decodeSoftwareVersion` is reached from `capabilityToZeJSON` for `ze bgp decode`; `capability.Parse` has no code 75 arm | None | Yes and unused |
| PATHS-LIMIT (76) | Real: `Negotiate` to `negotiatePathsLimit` to `pathsLimitSend` to `EncodingContext.PathsLimit` to `CommitService.enforcePathsLimit`, which drops paths past the PEER's limit | None. `test/decode/bgp-paths-limit.ci`, `test/encode/paths-limit.ci` and `test/exabgp-compat/encoding/conf-paths-limit.ci` are decode and encode only | Yes and unused |
| Hold Time (OPEN body) | `session_negotiate` takes the minimum of `ReceiveHoldTime` and the peer's, then calls `timers.SetHoldTime` | Every ze-peer session. The minimum is always ze's own value, so RFC 4271 Section 4.2's min-selection has never been exercised | **No.** `encodeOpen` copies the two Hold Time octets out of ze's body verbatim, `Config` has no hold field, and the `option=open:` switch accepts no such value |

### The GR receiving path is unreachable (measured 2026-09-07)

| Run | Result |
|-----|--------|
| `test/plugin/gr-mark-stale.ci`, verbatim | PASS, 7.2s |
| the same file, with GR events made unreachable | **PASS, 9.3s** |
| `test/plugin/llgr-transition.ci`, verbatim | PASS, 7.1s |
| the same file, with GR events made unreachable | PASS, 7.0s |
| control: one NLRI octet changed | FAIL, on exactly that assertion |

The control proves the harness discriminates, so the two PASSes are the finding
rather than a broken measurement.

The mechanism: `parseGRCapValue` returns the restart time formatted as four hex
characters and nothing else, so the code-64 value ze advertises is two octets
with no `<AFI, SAFI, Flags>` tuples. `Peer.getPluginCapabilities`
(`internal/component/bgp/reactor/peer.go`) puts that on the wire verbatim through
`capability.NewPlugin`, and the wire confirms it:

```
4002 0078
```

The mirrored 64 therefore carries zero families, `decodeGR` yields a nil family
list, and `onSessionDown` returns false at its empty-`staleFamilies` guard.
`retain-routes`, `mark-stale` and `purge-stale` are never dispatched, and
`enterLLGRLocked` is never reached.

What those tests actually assert is connection 2's re-announcement, and that
comes from `RIBManager.handleState`: on the down-to-up edge it calls
`collectPeerUpReplay`, `collectGroupedRibOutRoutesFiltered` and
`replayRoutesWithCursor`. `ribOut` is deleted only in the withdraw path, never on
peer-down, and `retainedPeers` guards the Adj-RIB-In release alone. The replay
happens whether GR ran or not.

Corroboration from the shipped documentation: the `ze:help` on `restart-time` in
`internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` already states
"It does not set the timer Ze runs when the peer restarts: that timer takes the
Restart Time the peer itself advertised." The mirror makes that documented
distinction untestable.

**Behavior to preserve:**
- Every existing `.ci` keeps its verdict. A file that declares none of the new
  facts gets the harness's documented default and behaves as it does today.
- `add-capability` and `drop-capability` keep their precedence: a stated code
  replaces the resolved one, and no second capability of that code is sent.
- The capability SET ze-peer advertises stays a mirror of ze's. This spec changes
  which VALUES describe the sender, never which codes are present.
- `encodeOpen`'s RFC 9072 framing and its 4096-octet OPEN bound.
- Ze's own wire behavior. Nothing outside `internal/test/peer/` and the `.ci` files
  changes, and no shipped binary links that package.

**Behavior to change:**
- The Hold Time, the code-64 value (restart time, families and flags), the code-71
  value (stale time and F-bits), the code-75 value and the code-76 value are each
  resolved from the test's own configuration.
- The `option=open:` switch gains four values, so a `.ci` can declare a fact that
  DIFFERS from ze's, which is what makes each consumer observable.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` peer block: `option=open:value=<fact>:...` lines, parsed by `expect.go`.
- Ze's OPEN, read off the wire by ze-peer, which supplies the capability SET.

### Transformation Path
1. `expect.go` parses each `option=open:` line into a `Config` field.
2. `buildOpen` reads ze's OPEN into its parameter list and resolves `openIdentity`.
3. `ownedCapabilities` builds a TLV for every code whose value describes ze-peer,
   now including 64, 71, 75 and 76, from `Config` and its defaults.
4. `reconcileParams` substitutes each owned TLV for the mirrored one, and leaves a
   code the `.ci` stated or dropped alone.
5. `encodeOpen` writes the fixed body, taking the Hold Time from the resolved
   configuration rather than from ze's body.
6. Ze reads the OPEN: `capability.Parse`, then `Negotiate`, then the per-consumer
   paths in the table above.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` file ↔ ze-peer | the `option=open:value=...` key/value grammar, parsed in `expect.go` | No |
| ze-peer ↔ ze | the OPEN message on the wire | No |
| ze reactor ↔ `bgp-gr` plugin | the peer-state event carrying the decoded capability, `handleStateEvent` | No |
| ze reactor ↔ `bgp-rib` plugin | the same peer-state event, `RIBManager.handleState` | No |

### Integration Points
- `ownedCapabilities` - each new fact is one more case, resolved from `Config`.
- `openIdentity` - the resolved sender-facts struct grows the new fields.
- `Config` plus the `option=open:` switch in `expect.go` - the declaration surface.
- `docs/architecture/testing/ci-format.md`, "Capability Control" - the published
  list of owned facts.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Filled at implementation: every new fact reaches the wire through `ownedCapabilities` then `reconcileParams`, never written into `encodeOpen` directly. The Hold Time is the exception, because it is a body field rather than a capability |
| No unintended coupling (components stay isolated) | No | Filled at implementation: `internal/test/peer/` gains no import of the `gr` plugin. The capability values are built from `internal/core/bgp/capability` and from octets, as codes 9, 65, 69 and 73 already are |
| No duplicated functionality (extends existing, does not recreate) | No | Filled at implementation: the four new options extend the existing `option=open:` switch |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A for the harness: `buildOpen` already allocates per OPEN, once per session |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | Filled at implementation: the new switch cases sit in the harness's own option parser, which is where the option vocabulary is declared. No core or shared package gains a case |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Four of the five mirrored values have a consumer that acts on them, as Graceful Restart does | only Graceful Restart is producer-verified; the other four are read from the capability list by inspection | some of the five are inert, and reconciling them changes nothing an operator or a test can observe | read the consumer of each received capability and name what it decides | **broken, and refined**: three act (64, 71, 76) and one is display-only and offline (75, reached from `capabilityToZeJSON` for `ze bgp decode`, absent from `capability.Parse`). The Hold Time acts, in `session_negotiate` |
| A-2 | Giving code 64 a non-empty family list makes `decodeGR` yield families and `onSessionDown` pass its empty-`staleFamilies` guard, so `retain-routes` and `mark-stale` are dispatched for the first time | the guard's own text in `gr_state.go`, and the measurement showing the path dead with an empty list | the GR receiving path stays unreachable and the five `llgr-*.ci` stay vacuous after the change, which is this spec failing at its purpose | run `test/plugin/gr-mark-stale.ci` after the change with the GR plugin's dispatch broken, and require RED | **confirmed.** With `handleStructuredState` returning immediately, the three new files go RED and `gr-mark-stale`, `llgr-transition` and `llgr-rib-stale` stay GREEN. The RED names the guard. The break had to be put in `handleStructuredState`: the same break in `handleStateEvent`, the JSON path, changed no verdict at all |
| A-3 | A fixed harness default hold time, chosen above the value ze advertises in every existing `.ci`, leaves the negotiated hold time exactly what it is today | RFC 4271 Section 4.2 takes the minimum, so a larger harness value is never selected | ze's KEEPALIVE cadence changes across the suite and timing-sensitive tests flake | run the full `.ci` suite and compare verdicts | **confirmed.** The default is 65535, the largest the field states, so the minimum is ze's own in every file. `plugin` was run twice, once with the resolution and once with the mirror restored: 14 failures common to both runs, 2 unique to one and 12 to the other, and every one of the 14 re-ran green in isolation. `encode` 61/61, `decode` 39/39, `runner` 10/10 |
| A-4 | No `.ci` asserts ze's software version inside ze-peer's OPEN | measured: no test covers code 75 at all | a `.ci` breaks when ze-peer stops reporting ze's build as its own | the same suite run | **confirmed.** No verdict moved in any suite |
| A-5 | `enforcePathsLimit` is observable from a `.ci`: a ze-peer declaring a limit of 1 for a family receives one path where ze holds several | `CommitService.enforcePathsLimit` drops past the peer's limit, and `pathsLimitSend` is the remote's | AC-5 has no functional test, and PATHS-LIMIT ownership is provable only by unit test | write the `.ci` and require it RED with the limit mirrored | **broken.** `enforcePathsLimit`'s drop cannot fire from any producer. `CommitService.Commit` has one caller, `commitToPeer`, reached only from `handleNamedCommitEnd` over `tx.Routes()`, and a `Transaction` keys its announcements by `nlriIndex`, which is AFI, SAFI and `NLRI.WriteTo` with no Path Identifier, so a second path for a prefix replaces the first before the commit ends. `update text` is not a second producer: it reaches `AnnounceNLRIBatch`. Recorded in `plan/journal/unwired-feature.md`. PATHS-LIMIT ownership is proven by unit test and on the wire (`4C05 0001010001` against ze's `4C05 0001010 00A`) |
| A-6 | The added capability octets keep every OPEN under the one-octet parameter length, or `encodeOpen`'s RFC 9072 framing absorbs the rest | `encodeOpen` already switches to extended framing above 255 octets and refuses above 65535 | an OPEN silently changes framing in a test that asserts OPEN hex | the suite run, plus the existing `encodeOpen` bound test | **confirmed.** The largest OPEN observed on the wire in these runs is 0x0038, 56 octets, and no framing changed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconciling the Graceful Restart restart time changes what `startEORTimer` waits for in every graceful-restart `.ci` | those files change verdict | read each one before landing, and size the change against them rather than against the capability |
| R-2 | Owning the code-64 family list makes the GR receiving path REACHABLE in 32 `.ci` that were green over dead code. Some will now fail for real | any of the 32 changes verdict | this is the spec working, not a regression to suppress. A file that goes red is read at its assertion, and the defect it exposes is a product defect governed by `ai/rules/completion.md`. Weakening the assertion to restore green is banned |
| R-3 | A harness hold time smaller than ze's changes ze's KEEPALIVE cadence everywhere and produces timing flakes | intermittent failures in unrelated `.ci` | the default is fixed and chosen ABOVE what ze advertises, so the minimum stays ze's value. Only a `.ci` that declares a smaller one opts into the change |
| R-4 | A newly owned code collides with an existing `add-capability` in some `.ci`, and the file gets the harness value instead of the octets it stated | a capability-mode or malformed-capability test changes verdict | `reconcileParams` already skips a stated code. The implementation adds a test per newly owned code proving the stated octets win |
| R-5 | Declaring an LLGR family ze did not negotiate changes what `onSessionReestablished` returns | an `llgr-*.ci` changes verdict | the family list defaults to the set ze-peer would otherwise have mirrored, so a `.ci` that declares nothing sees no change |
| R-6 | The four new option values overlap in grammar with `add-capability:code=64`, and a reader cannot tell which wins | review confusion, or a `.ci` author using both | `ci-format.md` states the precedence in the same sentence that states the existing one: a stated capability replaces a resolved one, for every owned code |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every functional test that drives a BGP session. Nothing an operator can reach; the shipped daemon is not touched |
| How is it reverted? | A single commit revert. The harness has no state and no on-disk format |
| Who else touches this path? | Any session writing a `.ci`. `plan/journal/mirrored-field-asserts-the-wrong-sender.md` carries the class |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a `.ci` peer block against a ze that offers Graceful Restart | → | `ownedCapabilities` code-64 resolution | `TestPeerOpenGracefulRestartIsTheHarnessOwn` |
| `option=open:value=graceful-restart:restart-time=N:family=F` | → | the `option=open:` switch filling `Config` | `TestPeerOptionGracefulRestartParses` |
| `option=open:value=hold-time:seconds=N` | → | `encodeOpen`'s fixed-body Hold Time | `TestPeerOpenHoldTimeIsDeclared` |
| `option=open:value=llgr:stale-time=N:family=F` | → | `ownedCapabilities` code-71 resolution | `TestPeerOpenLLGRIsTheHarnessOwn` |
| `option=open:value=paths-limit:family=F:limit=N` | → | `ownedCapabilities` code-76 resolution | `TestPeerOpenPathsLimitIsTheHarnessOwn` |
| a `.ci` declaring a peer restart time ze must act on | → | `grStateManager.onSessionDown` dispatching `mark-stale` | `test/plugin/gr-peer-restart-time-drives-timer.ci` |
| a `.ci` declaring a peer hold time below ze's | → | `session_negotiate` min-selection then `timers.SetHoldTime` | `test/plugin/open-hold-time-peer-lower-wins.ci` |
| a `.ci` declaring a peer paths limit of 1 | → | `CommitService.enforcePathsLimit` | `test/plugin/paths-limit-peer-declared.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze's OPEN carries a Graceful Restart capability with a restart time, and the `.ci` declares a different one | The OPEN ze-peer sends carries ze-peer's OWN restart time, and `startEORTimer` waits on that value rather than on ze's |
| AC-2 | The `.ci` declares one or more graceful-restart families for ze-peer | Ze-peer's code-64 value carries an `<AFI, SAFI, Flags>` tuple per declared family, `decodeGR` yields a non-empty family list, and `onSessionDown` passes its empty-family guard so `retain-routes` and `mark-stale` are dispatched |
| AC-3 | The `.ci` declares an LLGR stale time and F-bit for ze-peer, differing from ze's | Ze-peer's code-71 tuples carry the declared stale time and flags, and `enterLLGRLocked` arms its per-family timer on the declared value |
| AC-4 | Ze advertises a software version capability | Ze-peer's code-75 value is ze-peer's own version string. `ze bgp decode` over that OPEN prints it, and no octet of ze's build reaches the wire in ze-peer's name |
| AC-5 | The `.ci` declares a paths limit of 1 for a family in which ze holds several paths to one prefix | Ze-peer's code-76 value carries the declared limit, and ze sends one path for that prefix: `enforcePathsLimit` drops the rest |
| AC-6 | The `.ci` declares a hold time below ze's advertised value | The OPEN ze-peer sends carries the declared Hold Time, and ze's negotiated hold time is the declared value rather than ze's own |
| AC-7 | A `.ci` declares none of the five facts | It gets the harness's documented default for each, and its verdict is what it is today. No octet of any of the five comes from ze's OPEN |
| AC-8 | A `.ci` states `add-capability` for a newly owned code | The stated octets reach the wire unchanged, and ze-peer sends no second capability of that code |
| AC-9 | The `bgp-gr` plugin's state dispatch is broken deliberately | At least one `test/plugin/gr-*.ci` and one `test/plugin/llgr-*.ci` go RED. The recorded before-and-after is the proof that those tests stopped being vacuous |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerOpenGracefulRestartIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-1 | green |
| `TestPeerOpenGracefulRestartCarriesDeclaredFamilies` | `internal/test/peer/open_test.go` | AC-2 | green |
| `TestPeerOpenLLGRIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-3 | green |
| `TestPeerOpenSoftwareVersionIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-4 | green |
| `TestPeerOpenPathsLimitIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-5 | green |
| `TestPeerOpenHoldTimeIsDeclared` | `internal/test/peer/open_test.go` | AC-6 | green |
| `TestPeerOpenDefaultsInheritNoOctetFromZe` | `internal/test/peer/open_test.go` | AC-7 | green |
| `TestPeerOpenStatedCapabilityBeatsOwnedValue` | `internal/test/peer/open_test.go` | AC-8 | green |
| `TestPeerOptionGracefulRestartParses` | `internal/test/peer/expect_test.go` | the option grammar for 64 | green |
| `TestPeerOptionLLGRParses` | `internal/test/peer/expect_test.go` | the option grammar for 71 | green |
| `TestPeerOptionPathsLimitParses` | `internal/test/peer/expect_test.go` | the option grammar for 76 | green |
| `TestPeerOptionHoldTimeParses` | `internal/test/peer/expect_test.go` | the option grammar for the Hold Time | green |

**RED observed, 2026-09-08.** With the four owned cases removed from
`ownedCapabilities` and `encodeOpen` restored to copying ze's Hold Time octets,
`go test -run TestPeerOpen ./internal/test/peer/` fails these ten, each on the
fact it owns: `GracefulRestartIsTheHarnessOwn`, `GracefulRestartCarriesDeclaredFamilies`,
`GracefulRestartDefaultsToItsOwnFamilies`, `GracefulRestartForwardStateIsDeclarable`,
`LLGRIsTheHarnessOwn`, `SoftwareVersionIsTheHarnessOwn`, `PathsLimitIsTheHarnessOwn`,
`HoldTimeIsDeclared`, `DefaultsInheritNoOctetFromZe`, `RefusesADeclarationZeCannotCarry`.
`StatedCapabilityBeatsOwnedValue` stays green under the revert, which is correct:
a stated capability wins whether or not the builder resolves that code.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Graceful Restart restart time | 0-4095 | 4095 | N/A | 4096 |
| LLGR stale time | 0-16777215 | 16777215 | N/A | 16777216 |
| PATHS-LIMIT limit | 0-65535 | 65535 | N/A | 65536 |
| Hold Time | 0, then 3-65535 | 65535 | 1 and 2, which RFC 4271 Section 4.2 forbids | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-peer-restart-time-drives-timer` | `test/plugin/gr-peer-restart-time-drives-timer.ci` | A peer declaring a restart time ze does not share restarts, and ze times the wait by the PEER's value | green; RED with the `bgp-gr` state dispatch broken |
| `gr-peer-families-drive-mark-stale` | `test/plugin/gr-peer-families-drive-mark-stale.ci` | A peer declaring GR families goes down, and ze dispatches over those families rather than doing nothing | green; RED with the dispatch broken. Its restart time equals ze's, so only the family tuple can produce the row |
| `llgr-peer-stale-time-drives-timer` | `test/plugin/llgr-peer-stale-time-drives-timer.ci` | A peer declaring a graceful-restart and an LLGR time ze does not share goes down, and ze enters LLGR on the peer's numbers | green; RED with the dispatch broken |
| `open-hold-time-peer-lower-wins` | `test/plugin/open-hold-time-peer-lower-wins.ci` | A peer offering a hold time below ze's negotiates the peer's value, which is RFC 4271 Section 4.2's minimum | green; RED before the `zeTestMergePeerFileConfig` fix, where the declared hold time never reached the peer process and the run timed out waiting for a NOTIFICATION 180 seconds away |
| `paths-limit-peer-declared` | not written | A peer declaring it accepts one path per prefix receives one path, not every path ze holds | **not reachable**: `enforcePathsLimit`'s drop has no producer that can offer it two paths for one prefix (A-5). The harness half of AC-5 is proven by unit test and on the wire |
| the 32 `.ci` carrying a mirrored code 64 | `test/plugin/`, `test/bgp/` | unchanged verdicts under the new defaults (AC-7) | green: `plugin` compared against a mirror-restored baseline run, `encode` 61/61, `decode` 39/39, `runner` 10/10 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Ze's wire behavior does not change. The change is confined to `internal/test/peer/`, which no shipped binary links | |

## Files to Modify
- `internal/test/peer/open.go` - `ownedCapabilities` gains codes 64, 71, 75 and 76;
  `openIdentity` gains the resolved sender-facts; `encodeOpen` takes the Hold Time
  from the resolved configuration instead of from ze's body
- `internal/test/peer/peer.go` - `Config` gains the declaration fields
- `internal/test/peer/expect.go` - the `option=open:` switch gains `hold-time`,
  `graceful-restart`, `llgr` and `paths-limit`
- `docs/architecture/testing/ci-format.md` - the "Capability Control" section lists
  which values ze-peer owns, and the Options table gains the four values
- `docs/functional-tests.md` - the `.ci` option surface a test author reads

## Files to Create
- `test/plugin/gr-peer-restart-time-drives-timer.ci`
- `test/plugin/gr-peer-families-drive-mark-stale.ci`
- `test/plugin/llgr-peer-stale-time-drives-timer.ci`
- `test/plugin/open-hold-time-peer-lower-wins.ci`
- `test/plugin/paths-limit-peer-declared.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The declaration surface is a `.ci` option, not operator configuration. Nothing an operator writes changes |
| YANG validation constraints | No | Same reason. The numeric bounds are enforced in the harness's option parser and are covered by the Boundary Tests table |
| YANG custom validators | No | Same reason |
| CLI commands/flags | No | No `ze` command is added or changed |
| CLI grammar (keyword before value) | N-A | The `.ci` `option=` grammar is key/value already, and the four new values follow the existing shape |
| Editor autocomplete | No | No YANG leaf is added |
| Functional test for new RPC/API | Yes | The five `.ci` in Files to Create |
| Pipe completeness | N-A | No command output is produced |
| Env var registration | No | No `ze.*` variable is added |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, binary or certificate. The harness is in-process |
| Prometheus counters/metrics | No | No new observable daemon state. The GR plugin's existing counters begin to move because the path becomes reachable, which is AC-2 rather than a new metric |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability code or attribute. Codes 64, 71, 75 and 76 all exist, and only the harness's side of them changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The harness is not shipped. `docs/features.md` is untouched, and another session holds that file |
| 2 | Config syntax changed? | No | No operator configuration changes |
| 3 | CLI command added/changed? | No | No `ze` command changes |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | No | The `gr` and `rib` plugins are read, not edited |
| 6 | Has a user guide page? | No | The `.ci` format is contributor documentation, covered by rows 10 and 12 |
| 7 | Wire format changed? | No | Ze's encoding is unchanged. Only what the TEST peer writes changes |
| 8 | Plugin SDK/protocol changed? | No | No SDK or process-protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4724.md` and `rfc/short/rfc9494.md` gain no new claim, but the code-64 and code-71 receiving behavior becomes PROVEN for the first time. Any `Support` row change is decided when the tests are green, never before. **Another session holds `rfc/`: coordinate before editing, or leave the row to a follow-up commit that says so** |
| 10 | Test infrastructure changed? | Yes, and the page is deliberately UNCHANGED | `docs/functional-tests.md` states in its own words that it does not list the directives a peer block may carry, that `ClaimLine` is the definition and `ci-format.md` the documentation, and that the list it used to keep drifted and omitted six options. Adding four rows there would rebuild the list that page removed. The four values are documented in `ci-format.md`, "Sender Facts" |
| 11 | Affects daemon comparison? | No | No feature difference against another daemon |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/ci-format.md`, "Capability Control" and the Options table |
| 13 | Route metadata keys added/changed? | No | No metadata key changes |
| 14 | Prometheus counters added/changed? | No | No counter is defined or renamed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, and they resolve | `./le docs-to-code index-check` names seven unresolved anchors and none is in a file this spec touched. The `ci-format.md` anchor on `internal/test/peer/open.go -- buildOpen` still resolves, and the new section carries anchors on `expect.go` and `open_capability.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ci-format.md`'s capability examples use codes 65 and 9. Verify each against the reconciliation rules once the new codes are owned |

### Discovery (`ai/rules/repo-maintenance.md` Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md`, the "CI format" row pointing at `docs/architecture/testing/ci-format.md`. The four new options are documented in the same section that already answers "which values does ze-peer own" |
| What rule prevents the regression? | `ai/rules/principles.md` single-declaration, of which this spec is one instance. The class is recorded in `plan/journal/mirrored-field-asserts-the-wrong-sender.md` |
| What registry or inventory prevents drift? | `ownedCapabilities` is the single place a sender-describing code is resolved. A code absent from it is mirrored by construction, so the function IS the inventory and `ci-format.md` derives its list from it |
| What verification proves it? | The five `.ci`, plus AC-9's recorded break of the `bgp-gr` dispatch. A test that passes with the plugin dead has not proven the plugin |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the option surface and the resolution seam
   - Tests: `TestPeerOptionGracefulRestartParses`, `TestPeerOptionLLGRParses`,
     `TestPeerOptionPathsLimitParses`, `TestPeerOptionHoldTimeParses`
   - Files: `internal/test/peer/expect.go`, `internal/test/peer/peer.go`
   - Verify: a `.ci` can declare each fact and the value reaches `Config`. The
     wiring tests fail first, because nothing resolves the fields yet
2. **Phase: the Hold Time** -- the one fact no `.ci` can reach at all
   - Tests: `TestPeerOpenHoldTimeIsDeclared`, `open-hold-time-peer-lower-wins.ci`
   - Files: `internal/test/peer/open.go` (`openIdentity`, `encodeOpen`)
   - Verify: RFC 4271 Section 4.2's min-selection is exercised for the first time,
     and the suite's verdicts are unchanged under the default (A-3)
3. **Phase: PATHS-LIMIT (76) and software version (75)** -- the two with no test today
   - Tests: `TestPeerOpenPathsLimitIsTheHarnessOwn`,
     `TestPeerOpenSoftwareVersionIsTheHarnessOwn`, `paths-limit-peer-declared.ci`
   - Files: `internal/test/peer/open.go` (`ownedCapabilities`)
   - Verify: `enforcePathsLimit` drops a path because the PEER said so. Prove the
     `.ci` RED with the limit mirrored (A-5)
4. **Phase: Graceful Restart (64), value AND families** -- the largest blast radius
   - Tests: `TestPeerOpenGracefulRestartIsTheHarnessOwn`,
     `TestPeerOpenGracefulRestartCarriesDeclaredFamilies`,
     `gr-peer-restart-time-drives-timer.ci`, `gr-peer-families-drive-mark-stale.ci`
   - Files: `internal/test/peer/open.go`
   - Verify: `onSessionDown` passes its empty-family guard, and the 32 `.ci`
     carrying a mirrored 64 are each read at their verdict. A file that goes red is
     read at its assertion; the defect it exposes is fixed at the product, never by
     weakening the assertion (R-2)
5. **Phase: LLGR (71)** -- stale time and F-bits, on top of a reachable code 64
   - Tests: `TestPeerOpenLLGRIsTheHarnessOwn`, `llgr-peer-stale-time-drives-timer.ci`
   - Files: `internal/test/peer/open.go`
   - Verify: `enterLLGRLocked` arms on the declared value, and the five existing
     `llgr-*.ci` keep their verdicts
6. **Phase: discrimination and documentation**
   - Tests: AC-9's recorded break of the `bgp-gr` dispatch
   - Files: `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`
   - Verify: one `gr-*.ci` and one `llgr-*.ci` go RED with GR unreachable, and the
     before-and-after is recorded. Answer documentation row 16 by running the
     anchors command

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the five is reconciled, with its consumer named. None is left "recorded as inert": code 75 is display-only and is still owned, because the property is about the octets rather than about their reader |
| Correctness | No fact the OPEN asserts is written in one place and inherited in another. Check `ownedCapabilities` against the codes ze can send: a code absent from it is mirrored |
| Discrimination | Every new `.ci` was seen RED with the behavior under test reverted. A GR test that passes with the GR plugin dead is the failure this spec exists to remove |
| Rule: `ai/rules/evidence.md` | Each verdict cites the consuming function, not the capability's presence |
| Rule: `ai/rules/completion.md` | A `.ci` that goes red because the path became reachable is a find to fix or to journal, never an assertion to weaken |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Codes 64, 71, 75 and 76 resolved in `ownedCapabilities` | `gopls references` on `ownedCapabilities`, plus the unit tests for AC-1 to AC-5 |
| The Hold Time no longer read out of ze's body | a grep for the ze-body slice in `internal/test/peer/open.go` returns nothing |
| Four new `option=open:` values | a grep for `hold-time`, `graceful-restart`, `llgr` and `paths-limit` in `internal/test/peer/expect.go` |
| Five new `.ci`, each seen RED then GREEN | the pasted output in the TDD section |
| The GR receiving path is reachable | AC-9's recorded break, with the RED run pasted |
| `ci-format.md` names the owned set | read the "Capability Control" section and compare it against `ownedCapabilities` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: a test harness with no privileged input. The option parser still refuses an out-of-range value rather than truncating it, because a silently narrowed restart time is the zero-value failure in miniature |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A gating test that passed before now fails | It was passing on the mirrored value. Read the test, then decide |
| A `gr-*` or `llgr-*` test fails once code 64 carries families | The path just became reachable. The failure is real: route the defect per `ai/rules/completion.md`, and do not touch the assertion |
| A timing-sensitive test flakes after the Hold Time change | A-3 is broken. Raise the harness default above every advertised value, and re-run |
| An OPEN-hex assertion breaks | The parameter block grew. Check A-6, and update the hex only after confirming the framing is what RFC 9072 requires |

## Design Insights

- A mirror does not only assert the wrong sender. It can make a whole receiving
  path unreachable, and the tests over that path stay green because a DIFFERENT
  subsystem produces the observable they assert. Here `bgp-rib`'s peer-up replay
  produced the re-announcement that five `llgr-*.ci` and 32 GR-carrying `.ci` read
  as proof that graceful restart ran.
- The tell was cheap to get and nobody had taken it: run the test with the
  subsystem deliberately unreachable. Two runs and one control, and the vacuity is
  proven rather than argued.
- Ze's own code-64 value carries no `<AFI, SAFI, Flags>` tuples at all, because
  `parseGRCapValue` returns the 12-bit restart time and nothing else. That is a
  statement about the SHIPPED daemon rather than about the harness, and it is
  outside this spec. See Known Limitations.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Own the code-64 FAMILIES and flags, not only the restart time | swap the five field values and stop | The measurement settles it: with an empty family list `onSessionDown` returns at its guard and the GR receiving path is never entered. A value-only swap changes which number is on the wire and leaves every GR test exactly as vacuous as it is today, which fails the spec's purpose while looking like it succeeded |
| Four typed `option=open:` values, one per fact with a consumer a test must drive | a single generic `add-capability:code=N:hex=...`, which already exists | The generic route exists and is used by nobody for these codes, because a reader cannot see a restart time in a hex string. The typed option is what makes the declaration reviewable, and reviewability is why these facts stayed mirrored for so long |
| No option for software version (75): a fixed harness string | a fifth typed option | 75 has no session consumer at all, since `capability.Parse` has no code 75 arm. Nothing a test can assert varies with the value, so an option would be machinery with no user, which `ai/rules/simplicity.md` cuts |
| A fixed harness default hold time, chosen ABOVE what ze advertises | keep mirroring when the `.ci` says nothing | Mirroring on the default is the defect, in the case that covers most files. A fixed larger default owns the octets and still leaves RFC 4271's minimum at ze's value, so no existing test changes cadence |
| Leave ze's empty code-64 family list alone | fix `parseGRCapValue` here so the mirror carries families | That is a change to the SHIPPED daemon's advertised capability, with an operator-visible blast radius, and it would still leave ze-peer asserting ze's values. Two different problems, and folding one into the other costs this spec its single focus (`ai/rules/rule-precedence.md`) |

## Implementation Outcome (2026-09-08)

| AC | Verdict | Evidence |
|----|---------|----------|
| AC-1 | met | `TestPeerOpenGracefulRestartIsTheHarnessOwn`, and `gr-peer-restart-time-drives-timer.ci`, where `show bgp rib status` answers `gr-state.127.0.0.1.restart-time: 7` against ze's configured 120 |
| AC-2 | met | `TestPeerOpenGracefulRestartCarriesDeclaredFamilies` and `...DefaultsToItsOwnFamilies`, and `gr-peer-families-drive-mark-stale.ci`, whose restart time equals ze's so only the family tuple can produce the row |
| AC-3 | met for the octets and the decode, NOT for the timer's own number | `TestPeerOpenLLGRIsTheHarnessOwn` holds the stale time and the F bit. `llgr-peer-stale-time-drives-timer.ci` shows ze entering LLGR on the peer's restart time, which needs the peer's code-71 tuple to name a family with a non-zero stale time. The stale time's NUMBER has no functional witness: every effect of the LLST timer acts on an Adj-RIB-In `RIBManager.handleState` already released on peer-down |
| AC-4 | met | `TestPeerOpenSoftwareVersionIsTheHarnessOwn` |
| AC-5 | met for the capability, NOT for the enforcement | `TestPeerOpenPathsLimitIsTheHarnessOwn`, and the wire: ze-peer sends `4C05 0001010001` where ze sends `4C05 0001010 00A`. The enforcement half is unreachable and A-5 carries the producer chain |
| AC-6 | met | `TestPeerOpenHoldTimeIsDeclared` and `open-hold-time-peer-lower-wins.ci` |
| AC-7 | met | `TestPeerOpenDefaultsInheritNoOctetFromZe`, plus four suites compared against a mirror-restored baseline |
| AC-8 | met | `TestPeerOpenStatedCapabilityBeatsOwnedValue`, one subtest per newly owned code |
| AC-9 | met | With `handleStructuredState` returning immediately: `gr-peer-restart-time-drives-timer`, `gr-peer-families-drive-mark-stale` and `llgr-peer-stale-time-drives-timer` FAIL; `gr-mark-stale`, `llgr-transition` and `llgr-rib-stale` PASS. The same break in `handleStateEvent` moved no verdict at all, which is how that JSON path was found to be dead |

### Defects this work walked into, and where they are recorded

Both are in `plan/journal/unwired-feature.md` and neither is fixed here.

| Surface | What it is |
|---------|-----------|
| `CommitService.enforcePathsLimit` | Its per-prefix drop has no producer that can reach it. A named commit keys its announcements by prefix with no Path Identifier, so a second path replaces the first before `Commit` ever runs |
| `RIBManager.handleState` and `RIBManager.handleStructuredState` on peer-down | Both carry the same release, and `handleStructuredState` is the live one (AC-9 measured the JSON path dead). Either releases the peer's Adj-RIB-In unless `retainedPeers` already holds the peer, and `retain-routes` is dispatched by another plugin process reacting to the same event. Measured twice: `mark-stale` ran over an empty RIB. RFC 4724's retention therefore does not happen, which is why the two GR files assert the dispatch rather than the marking |

## Known Limitations
- Ze's own Graceful Restart capability carries a restart time and zero
  `<AFI, SAFI, Flags>` tuples (`parseGRCapValue`, `internal/component/bgp/plugins/gr/gr.go`).
  A peer therefore learns that ze supports graceful restart and that ze preserves
  forwarding state for no family. Whether that is what ze should advertise is a
  question about the shipped daemon and about RFC 4724 Section 3. It is NOT this
  spec's work, it is raised to Thomas rather than decided here, and it gets its own
  spec if he wants it changed.
- This spec does not touch the capability SET ze-peer advertises. A code ze never
  offers is still a code ze-peer cannot assert, the four-octet AS excepted.
- Code 70 (Enhanced Route Refresh) describes the session rather than the sender, so
  it stays mirrored, which is correct.

## RFC Documentation (Scope: protocol)

The harness is not protocol-implementing code and adds no `RFC requirement:` tag.

| Fact | Citation the code carries |
|------|---------------------------|
| The restart time | RFC 4724 Section 3, on the Restart Time field of the sender |
| The GR family tuples | RFC 4724 Section 3, on the `<AFI, SAFI, Flags>` list a restarting speaker advertises |
| The LLGR stale time and F-bit | RFC 9494, on the 7-octet per-family tuple and its 24-bit stale time |
| The Hold Time minimum | RFC 4271 Section 4.2, on the negotiated hold time being the smaller of the two |
| The paths limit | draft-abraitis-idr-addpath-paths-limit, on the limit being the sender's declaration of what it accepts |

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
- [ ] Every AC demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Implementation Summary

### What Was Implemented
- `ownedCapabilities` (`internal/test/peer/open_capability.go`) resolves codes 64, 71, 75 and 76 beside the 9, 65, 69 and 73 it already held. `open_capability.go` is a new file: the capability half of `open.go`, moved whole with `complementaryRole` and `invertedAddPath`, plus the four new builders.
- `encodeOpen` (`internal/test/peer/open.go`) writes `id.holdTime` where it copied `zeBody[3:5]`. `newOpenIdentity` is the one place every sender fact takes its harness default, and `resolveFamilies` fills a family list the `.ci` left unstated from ze-peer's own advertised set.
- `Config` (`internal/test/peer/peer.go`) gains `HoldTime`, `GracefulRestart`, `LLGR` and `PathsLimit`, with `GracefulRestartDecl`, `LLGRDecl` and `PathsLimitDecl` beside them.
- The `option=open:` switch (`internal/test/peer/expect.go`) gains `hold-time`, `graceful-restart`, `llgr` and `paths-limit`. Every numeric key is range-checked against its RFC field width and refused rather than truncated.
- Two guards: `validateOwnedCodesStatedOnce` refuses a fact stated by a typed option AND by `add-capability` for the same code; `refuseUnofferedDeclarations` refuses a fact for a capability ze did not offer.
- Four `.ci` under `test/plugin/`, with the compiled observers in `internal/test/fixture/plugin_fixture_gr_sender_facts.go` and their registration in `register_gr_sender_facts.go`.
- `zeTestMergePeerFileConfig` (`internal/test/cli/cmd_peer.go`) carries the four new facts from the peer file into the running config. Without it a declared Hold Time never reached the peer process.

### Bugs Found/Fixed
- `zeTestMergePeerFileConfig` dropped every new fact on the floor. Found by `open-hold-time-peer-lower-wins.ci` timing out on a NOTIFICATION 180 seconds away; covered by that file now.
- Two product defects were found and NOT fixed here. Both are in `plan/journal/unwired-feature.md` and both are named in "Defects this work walked into" above.

### Documentation Updates
- `docs/architecture/testing/ci-format.md`: new "Sender Facts" section (grammar, key table, defaults table, the two refusals), the Options table gains four rows, and "Capability Control" now names all nine resolved facts. Anchors: `<!-- source: internal/test/peer/expect.go -- parseOpenHoldTime, parseGracefulRestartDecl, parseLLGRDecl, parsePathsLimitDecl -->` and `<!-- source: internal/test/peer/open_capability.go -- ownedCapabilities, refuseUnofferedDeclarations -->`.
- `docs/functional-tests.md` is deliberately unchanged. Its line 1886 states that the peer-block directive set is not listed there, that `ClaimLine` is the definition and `ci-format.md` the documentation, and that the list it removed had drifted. Adding four rows would rebuild that list.
- `./le repository check` passed, which is the source-anchor pass. `./le doc check links` was run; its result is in Pre-Commit Verification.

### Deviations from Plan
- `test/plugin/paths-limit-peer-declared.ci` was NOT written. A-5 broke: `CommitService.enforcePathsLimit`'s drop has no producer that can offer it two paths for one prefix. See Work Not Done.
- The spec planned "LLGR (70 and 71)". Code 70 is Enhanced Route Refresh and describes the session, so it stays mirrored. Corrected in the research table at the head of this spec.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 assumed `enforcePathsLimit` is observable from a `.ci` | `Transaction.nlriIndex` (`internal/component/bgp/transaction/commit_manager.go`) keys by AFI, SAFI and `NLRI.WriteTo`, and `INET.WriteTo` (`internal/core/bgp/nlri/inet.go`) writes the prefix length and prefix bytes only. A second path for one prefix replaces the first in `QueueAnnounce` before the commit ends | writing the `.ci` and finding no producer that could reach the drop | journal row in `plan/journal/unwired-feature.md`; AC-5 met for the capability and not for the enforcement |
| assumption | A-1 assumed all five mirrored values have a consumer that acts on them | Three act (64, 71, 76), one is display-only and offline (75: `capability.Parse` holds no code-75 arm), and the Hold Time acts in `session_negotiate` | reading each consumer at its producing function | 75 is still owned, because the property is about the octets rather than about their reader, and it gets no option |
| approach | The functional tests were first written to assert that `mark-stale` MARKED routes | `RIBManager.handleState` and `handleStructuredState` release the peer's Adj-RIB-In on peer-down unless `retainedPeers` is already set, and only the `bgp-gr` plugin's `retain-routes` sets it, from another process reacting to the same event. `mark-stale` ran over an empty RIB | `show bgp rib status` answered a gr-state row beside `routes-in: 0`, twice | the three files assert the DISPATCH instead, which is what this spec owns; the retention defect is journalled |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every fact ze-peer's OPEN asserts about ze-peer is resolved from the test's own configuration | Done | `ownedCapabilities`, `newOpenIdentity`, `encodeOpen` (`internal/test/peer/`) | Nine facts now: 9, 64, 65, 69, 71, 73, 75, 76 and the Hold Time |
| Producer-verify the four facts the spec left unverified | Done | the consumer table in Current Behavior | Each row read at its producing function; A-1 broke and is refined |
| Size and absorb the blast radius on the graceful-restart tests | Done | A-3 and AC-7 | `plugin` compared against a mirror-restored baseline, `encode` 61/61, `decode` 39/39, `runner` 10/10 |
| Make the graceful-restart receiving path reachable | Done | AC-9 | `gr-mark-stale`, `llgr-transition` and `llgr-rib-stale` still PASS with the dispatch broken; the three new files FAIL |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPeerOpenGracefulRestartIsTheHarnessOwn`, `test/plugin/gr-peer-restart-time-drives-timer.ci` | ze configured 120, peer states 7, observer requires 7 |
| AC-2 | Done | `TestPeerOpenGracefulRestartCarriesDeclaredFamilies`, `...DefaultsToItsOwnFamilies`, `test/plugin/gr-peer-families-drive-mark-stale.ci` | that file's restart time equals ze's, so only the family tuple can produce the row |
| AC-3 | Changed | `TestPeerOpenLLGRIsTheHarnessOwn`, `test/plugin/llgr-peer-stale-time-drives-timer.ci` | met for the octets and the decode. The stale time's NUMBER has no functional witness: every LLST effect acts on an Adj-RIB-In already released. See Work Not Done |
| AC-4 | Done | `TestPeerOpenSoftwareVersionIsTheHarnessOwn` | no `.ci` can observe it: `capability.Parse` holds no code-75 arm |
| AC-5 | Changed | `TestPeerOpenPathsLimitIsTheHarnessOwn`, and the wire: ze-peer sends `4C05 0001010001` where ze sends `4C05 000101000A` | met for the capability. The enforcement half is unreachable; A-5 carries the producer chain |
| AC-6 | Done | `TestPeerOpenHoldTimeIsDeclared`, `test/plugin/open-hold-time-peer-lower-wins.ci` | RFC 4271 Section 4.2's min-selection is exercised for the first time |
| AC-7 | Done | `TestPeerOpenDefaultsInheritNoOctetFromZe`, plus four suites against a mirror-restored baseline | |
| AC-8 | Done | `TestPeerOpenStatedCapabilityBeatsOwnedValue`, one subtest per newly owned code | |
| AC-9 | Done | the recorded break in `handleStructuredState` | The same break in `handleStateEvent`, the JSON path, moved NO verdict, which is how that path was found dead |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the twelve unit tests | Done | `internal/test/peer/open_test.go`, `expect_test.go` | all green, plus `GracefulRestartDefaultsToItsOwnFamilies`, `GracefulRestartForwardStateIsDeclarable`, `RefusesADeclarationZeCannotCarry`, `RefusesAFactDeclaredTwice`, `HoldTimeStatedTwiceIsRefused` |
| `gr-peer-restart-time-drives-timer` | Done | `test/plugin/` | PASS, 2.6s |
| `gr-peer-families-drive-mark-stale` | Done | `test/plugin/` | PASS, 2.6s |
| `llgr-peer-stale-time-drives-timer` | Done | `test/plugin/` | PASS, 3.4s |
| `open-hold-time-peer-lower-wins` | Done | `test/plugin/` | PASS, 5.3s |
| `paths-limit-peer-declared` | Skipped | not written | A-5 broke. See Work Not Done |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/peer/open.go` | Done | plus `internal/test/peer/open_capability.go`, the capability half moved out |
| `internal/test/peer/peer.go` | Done | |
| `internal/test/peer/expect.go` | Done | |
| `docs/architecture/testing/ci-format.md` | Done | |
| `docs/functional-tests.md` | Changed | deliberately unchanged; the reason is documentation row 10 |
| the five `.ci` | Changed | four written, the fifth unreachable |
| `internal/test/cli/cmd_peer.go` | Changed | not in the plan; the merge function had to carry the new facts |
| `internal/test/fixture/plugin_fixture_gr_sender_facts.go`, `register_gr_sender_facts.go` | Changed | not in the plan; the three GR observers |

### Audit Summary
- **Total items:** 27
- **Done:** 21
- **Partial:** 0
- **Skipped:** 1 (`paths-limit-peer-declared.ci`, blocked by a product defect, in Work Not Done)
- **Changed:** 5 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No octet of a fact ze-peer's OPEN asserts about itself is inherited from ze's OPEN | functional | `TestPeerOpenDefaultsInheritNoOctetFromZe`, which reads the OPEN ze-peer builds against a ze OPEN carrying different values for all nine facts. RED under the revert, recorded in the TDD section |
| Ze acts on the PEER's restart time, not its own | functional | `test/plugin/gr-peer-restart-time-drives-timer.ci`: ze configured `restart-time 120`, the peer states 7, and `show bgp rib status` answers `restart-time: 7`. The file fails on the value a mirror would produce |
| The graceful-restart receiving path stops being unreachable | discrimination | AC-9's recorded break: with `handleStructuredState` returning immediately the three new files FAIL, while `gr-mark-stale`, `llgr-transition` and `llgr-rib-stale` PASS. That last half is the finding: those three were green over dead code and still are |
| RFC 4271 Section 4.2's hold-time minimum becomes exercisable | functional | `test/plugin/open-hold-time-peer-lower-wins.ci`. Before this spec the two advertised values were always equal, so the min-selection had nothing to choose between |
| Every existing `.ci` keeps its verdict | functional | `plugin` run twice, once resolved and once with the mirror restored: 14 failures common to both, and every one re-ran green in isolation. `encode` 61/61, `decode` 39/39, `runner` 10/10. Re-checked at closure: `gr-mark-stale`, `llgr-transition`, `llgr-rib-stale`, `llgr-readvertise`, `llgr-readvertise-multipeer` and `llgr-egress-state-unloaded` all PASS |

## Work Not Done

<!-- Neither row names a spec, and that is the owner directive of 2026-08-10
     rather than an omission: a DEFECT walked into gets ONE journal row, and
     writing a spec for it is banned (ai/rules/rule-precedence.md, "A DEFECT you
     walked into"). Both rows below are blocked by the same two defects, both
     are recorded in plan/journal/unwired-feature.md, and the class file is what
     earns the fix in a deliberate pass over the journal. -->

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| `test/plugin/paths-limit-peer-declared.ci`, the functional half of AC-5 | `CommitService.enforcePathsLimit`'s drop has no producer that can reach it. `CommitService.Commit` has one caller, `commitToPeer`, reached only from `handleNamedCommitEnd` over `tx.Routes()`, and `Transaction.nlriIndex` keys by AFI, SAFI and `NLRI.WriteTo` with no Path Identifier, so a second path for a prefix replaces the first before the commit ends. `update text` is not a second producer: it reaches `AnnounceNLRIBatch` | `plan/journal/unwired-feature.md`, the `CommitService.enforcePathsLimit` row of 2026-09-08. A defect walked into gets a row, not a spec (owner directive, 2026-08-10) |
| A functional witness for the LLGR stale time's own NUMBER, the second half of AC-3 | Every effect of the LLST timer (`purge-stale`, `release-routes`) acts on an Adj-RIB-In that `RIBManager.handleState` and `handleStructuredState` already released on peer-down, so no command can read a difference. The number is held by `TestPeerOpenLLGRIsTheHarnessOwn` over the octets ze-peer writes | `plan/journal/unwired-feature.md`, the `RIBManager.handleState` row of 2026-09-08. Same directive |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/test-peer-open-mirrors-five-more-sender-facts-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md`, 13 code files, verdict clean |
| `./le spec session review check` | `review_gate: OK (13 code files, clean, hashes match ...)` |
| Rounds | 1 |
| Reviewer lenses used | wiring plus functional-test coverage; logic plus guard audit plus the zero-value trap; RFC conformance plus ze-go-style plus simplicity. Run inline by the closure context, which did not author the diff |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | none: the run reported 0 BLOCKER and 0 ISSUE | - | - |

### Notes recorded, not blocking
| # | Finding | Location | Why it does not block |
|---|---------|----------|----------------------|
| N-1 | `capabilityTLV` panics on a value above 255 octets, and its input length scales with the family list read off ze's OPEN. 37 families would reach it through `llgrTLV`'s 7-octet tuples | `internal/test/peer/open_capability.go` -- `capabilityTLV`, `llgrTLV` | `family.LookupFamily` and ze's own configuration both draw on one registry with 25 `RegisterFamily` call sites, so 175 octets is the ceiling. The code is under `internal/test/peer/`, which no shipped binary imports: a grep for that import path outside `internal/test/` returns nothing |
| N-2 | `awaitLLGREntry` asserts `number13(row["restart-time"]) == 0`, and `number13` returns 0 for an ABSENT key, so a renamed key would pass it | `internal/test/fixture/plugin_fixture_gr_sender_facts.go` -- `awaitLLGREntry`, and `number13` in `plugin_fixture_13.go` | its only caller, `llgrPeerStaleTimeDrivesTimer`, runs `requireRestartTime(row, 1)` first, which fails on an absent key. The coupling is implicit rather than absent |
| N-3 | The Defects table and the journal row named `RIBManager.handleState`; the LIVE handler is `handleStructuredState`, which carries byte-identical release logic | `internal/component/bgp/plugins/rib/rib.go` | a record defect, not a product one. Fixed in ONE edit to the Defects table above; the journal row's mechanism claim is true of both handlers and is left as written |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/gr-peer-families-drive-mark-stale.ci` | yes | `ls -la`: 4249 bytes |
| `test/plugin/gr-peer-restart-time-drives-timer.ci` | yes | `ls -la`: 4351 bytes |
| `test/plugin/llgr-peer-stale-time-drives-timer.ci` | yes | `ls -la`: 4312 bytes |
| `test/plugin/open-hold-time-peer-lower-wins.ci` | yes | `ls -la`: 2962 bytes |
| `test/plugin/paths-limit-peer-declared.ci` | **no** | `ls: cannot access ... No such file or directory`. Named in Work Not Done |
| `internal/test/peer/open_capability.go` | yes | `gopls symbols` lists `ownedCapabilities`, `gracefulRestartTLV`, `llgrTLV`, `softwareVersionTLV`, `pathsLimitTLV`, `refuseUnofferedDeclarations` |
| `internal/test/fixture/plugin_fixture_gr_sender_facts.go`, `register_gr_sender_facts.go` | yes | `gopls symbols` lists the three scenarios, and the register file registers all three by `.ci` name |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the peer's restart time drives ze's timer | the plugin suite, run at closure over the four new files: `PASS 325 gr-peer-restart-time-drives-timer`, 2.6s |
| AC-2 | the peer's family tuple makes ze dispatch | `PASS 324 gr-peer-families-drive-mark-stale`, 2.6s |
| AC-3 | ze enters LLGR on the peer's numbers | `PASS 383 llgr-peer-stale-time-drives-timer`, 3.4s |
| AC-4 | code 75 is ze-peer's own | `--- PASS: TestPeerOpenSoftwareVersionIsTheHarnessOwn (0.00s)` |
| AC-5 | code 76 is ze-peer's own | `--- PASS: TestPeerOpenPathsLimitIsTheHarnessOwn (0.00s)` |
| AC-6 | the peer's lower hold time wins | `PASS 472 open-hold-time-peer-lower-wins`, 5.3s, and `--- PASS: TestPeerOpenHoldTimeIsDeclared` |
| AC-7 | a silent `.ci` inherits no octet | `--- PASS: TestPeerOpenDefaultsInheritNoOctetFromZe`, plus the six pre-existing GR and LLGR files re-run green at closure |
| AC-8 | stated octets beat the resolved value | `--- PASS: TestPeerOpenStatedCapabilityBeatsOwnedValue` |
| AC-9 | the GR receiving path is reachable | the recorded break in `handleStructuredState`: the three new files FAIL, and `gr-mark-stale`, `llgr-transition` and `llgr-rib-stale` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a `.ci` peer block against a ze offering GR | `gr-peer-restart-time-drives-timer.ci` | yes: the file states `option=open:value=graceful-restart:restart-time=7` against `restart-time 120` in the ze block, and the observer requires 7 |
| `option=open:value=graceful-restart:...` | -- | yes: `parseGracefulRestartDecl` fills `Config.GracefulRestart`, read by `newOpenIdentity` and `buildOpen`. `TestPeerOptionGracefulRestartParses` PASS |
| `option=open:value=hold-time:seconds=N` | `open-hold-time-peer-lower-wins.ci` | yes: `encodeOpen` writes `id.holdTime`; the mirror slice `copy(body[3:5], zeBody[3:5])` is gone from `open.go` |
| `option=open:value=llgr:...` | `llgr-peer-stale-time-drives-timer.ci` | yes: `llgrTLV` writes the 7-octet tuples; `TestPeerOptionLLGRParses` PASS |
| `option=open:value=paths-limit:...` | -- | yes to `ownedCapabilities`; the `.ci` half is Work Not Done. `TestPeerOptionPathsLimitParses` PASS |
| a `.ci` peer restart time driving `onSessionDown` | `gr-peer-families-drive-mark-stale.ci` | yes: its restart time EQUALS ze's, so only the family tuple can produce the gr-state row |
| a declared fact reaching the peer PROCESS | all four | yes: `zeTestMergePeerFileConfig` (`internal/test/cli/cmd_peer.go`) carries all four from the file into the running config |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, refined | three act (64, 71, 76), 75 is display-only and offline, and the Hold Time acts in `session_negotiate`. Read at each consumer |
| A-2 | confirmed | the recorded break: the three new files RED, the three old ones GREEN |
| A-3 | confirmed | default 65535, so RFC 4271 Section 4.2's minimum stays ze's. Four suites compared against a mirror-restored baseline |
| A-4 | confirmed | no verdict moved in any suite |
| A-5 | **broken** | `Transaction.nlriIndex` (`internal/component/bgp/transaction/commit_manager.go`) and `INET.WriteTo` (`internal/core/bgp/nlri/inet.go`) read at closure: the key is AFI, SAFI, prefix length and prefix bytes, with no Path Identifier. Mistake Log row written; journal row written |
| A-6 | confirmed | the largest OPEN observed is 0x0038, 56 octets. No framing changed |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ci-format.md` "Sender Facts" default table | every value compared against its constant in `internal/test/peer/open.go`: `peerHoldTime = 65535`, `peerRestartTime = 300`, `peerStaleTime = 600`, `peerPathsLimit = 65535`, `peerSoftwareVersion = "ze-peer"` | yes, all five agree |
| `ci-format.md` "Capability Control" names the owned set | the nine facts it lists compared against `ownedCapabilities` and `newOpenIdentity` | yes |
| `ci-format.md` refusal sentences | `validateOwnedCodesStatedOnce` and `refuseUnofferedDeclarations` read at their producing functions, and both are reached from `LoadExpectFile` and from `New` | yes |
| row 10: `docs/functional-tests.md` correctly left alone | its line 1886 states in its own words that the directive set is not listed there, and names `ClaimLine` as the definition | yes |
| row 16: no doc source anchor on a changed file is stale | `./le repository check` answered "all checks passed", which includes the source-anchor pass | yes |
| doc links | `./le doc check links` reported 27 broken references, all pre-existing and none in a file this spec touched. The three citers of this spec's own path were repointed to the bare stem in commit A | yes |
| rows 1-9, 11, 13-15 answered No | no operator-visible surface changed: no `ze` command, no YANG leaf, no RPC, no metric, no registration. A grep for the `internal/test/peer` import path outside `internal/test/` returns nothing, so no shipped binary links the changed package | yes |

## Core Insight

A mirror can make a whole receiving path UNREACHABLE, and the tests over that
path stay green because a different subsystem produces the observable they
assert. Ze's own code-64 capability carries no `<AFI, SAFI, Flags>` tuples, so
`onSessionDown` returned at its empty-family guard and nothing was dispatched;
the re-announcement that 32 `.ci` read as proof of graceful restart came from
`bgp-rib`'s peer-up replay instead. The tell was two runs and one control: run
the test with the subsystem deliberately unreachable and see whether the verdict
moves. It cost minutes, and it was available to every session that ever touched
those files.
