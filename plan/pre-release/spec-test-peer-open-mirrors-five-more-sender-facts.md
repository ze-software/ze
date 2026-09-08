# Spec: test-peer-open-mirrors-five-more-sender-facts

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
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
| A-2 | Giving code 64 a non-empty family list makes `decodeGR` yield families and `onSessionDown` pass its empty-`staleFamilies` guard, so `retain-routes` and `mark-stale` are dispatched for the first time | the guard's own text in `gr_state.go`, and the measurement showing the path dead with an empty list | the GR receiving path stays unreachable and the five `llgr-*.ci` stay vacuous after the change, which is this spec failing at its purpose | run `test/plugin/gr-mark-stale.ci` after the change with the GR plugin's dispatch broken, and require RED | unvalidated |
| A-3 | A fixed harness default hold time, chosen above the value ze advertises in every existing `.ci`, leaves the negotiated hold time exactly what it is today | RFC 4271 Section 4.2 takes the minimum, so a larger harness value is never selected | ze's KEEPALIVE cadence changes across the suite and timing-sensitive tests flake | run the full `.ci` suite and compare verdicts | unvalidated |
| A-4 | No `.ci` asserts ze's software version inside ze-peer's OPEN | measured: no test covers code 75 at all | a `.ci` breaks when ze-peer stops reporting ze's build as its own | the same suite run | unvalidated |
| A-5 | `enforcePathsLimit` is observable from a `.ci`: a ze-peer declaring a limit of 1 for a family receives one path where ze holds several | `CommitService.enforcePathsLimit` drops past the peer's limit, and `pathsLimitSend` is the remote's | AC-5 has no functional test, and PATHS-LIMIT ownership is provable only by unit test | write the `.ci` and require it RED with the limit mirrored | unvalidated |
| A-6 | The added capability octets keep every OPEN under the one-octet parameter length, or `encodeOpen`'s RFC 9072 framing absorbs the rest | `encodeOpen` already switches to extended framing above 255 octets and refuses above 65535 | an OPEN silently changes framing in a test that asserts OPEN hex | the suite run, plus the existing `encodeOpen` bound test | unvalidated |

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
| `TestPeerOpenGracefulRestartIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-1 | |
| `TestPeerOpenGracefulRestartCarriesDeclaredFamilies` | `internal/test/peer/open_test.go` | AC-2 | |
| `TestPeerOpenLLGRIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-3 | |
| `TestPeerOpenSoftwareVersionIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-4 | |
| `TestPeerOpenPathsLimitIsTheHarnessOwn` | `internal/test/peer/open_test.go` | AC-5 | |
| `TestPeerOpenHoldTimeIsDeclared` | `internal/test/peer/open_test.go` | AC-6 | |
| `TestPeerOpenDefaultsInheritNoOctetFromZe` | `internal/test/peer/open_test.go` | AC-7 | |
| `TestPeerOpenStatedCapabilityBeatsOwnedValue` | `internal/test/peer/open_test.go` | AC-8 | |
| `TestPeerOptionGracefulRestartParses` | `internal/test/peer/expect_test.go` | the option grammar for 64 | |
| `TestPeerOptionLLGRParses` | `internal/test/peer/expect_test.go` | the option grammar for 71 | |
| `TestPeerOptionPathsLimitParses` | `internal/test/peer/expect_test.go` | the option grammar for 76 | |
| `TestPeerOptionHoldTimeParses` | `internal/test/peer/expect_test.go` | the option grammar for the Hold Time | |

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
| `gr-peer-restart-time-drives-timer` | `test/plugin/gr-peer-restart-time-drives-timer.ci` | A peer declaring a restart time ze does not share restarts, and ze times the wait by the PEER's value | |
| `gr-peer-families-drive-mark-stale` | `test/plugin/gr-peer-families-drive-mark-stale.ci` | A peer declaring GR families goes down, and ze marks those families stale rather than doing nothing | |
| `llgr-peer-stale-time-drives-timer` | `test/plugin/llgr-peer-stale-time-drives-timer.ci` | A peer declaring an LLGR stale time ze does not share goes down, and ze holds its routes for the PEER's stale time | |
| `open-hold-time-peer-lower-wins` | `test/plugin/open-hold-time-peer-lower-wins.ci` | A peer offering a hold time below ze's negotiates the peer's value, which is RFC 4271 Section 4.2's minimum | |
| `paths-limit-peer-declared` | `test/plugin/paths-limit-peer-declared.ci` | A peer declaring it accepts one path per prefix receives one path, not every path ze holds | |
| the 32 `.ci` carrying a mirrored code 64 | `test/plugin/`, `test/bgp/` | unchanged verdicts under the new defaults (AC-7) | |

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
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - the four new `option=open:` values |
| 11 | Affects daemon comparison? | No | No feature difference against another daemon |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/ci-format.md`, "Capability Control" and the Options table |
| 13 | Route metadata keys added/changed? | No | No metadata key changes |
| 14 | Prometheus counters added/changed? | No | No counter is defined or renamed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, not answered from memory: run `./le spec citation anchors spec plan/pre-release/spec-test-peer-open-mirrors-five-more-sender-facts.md` at implementation. Known today: `ci-format.md` carries a source anchor naming `internal/test/peer/open.go` and `buildOpen`, which this spec changes |
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

## Review Gate

<!-- Filled by /ze-close's Review Gate step, which runs /ze-review until the
     tables show 0 BLOCKER and 0 ISSUE. Left empty at design time. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
