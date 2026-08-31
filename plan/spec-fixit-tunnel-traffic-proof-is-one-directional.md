# Spec: fixit-tunnel-traffic-proof-is-one-directional

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (create `plan/deferrals/fixit-tunnel-traffic-proof-is-one-directional.md` on the first deferral) |
| Handoff | - |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`(*scenarioLab).verifyTunnelTraffic` (`internal/le/interoplab/ipsec/helpers.go`) is
the shared proof that traffic flows through the tunnel. Nine strongSwan interop
scenarios call it and nothing else to prove their dataplane. It reads both peers'
XFRM byte counters, pings from Ze to strongSwan, reads both counters again, and
passes when each peer's map advanced on some SPI.

Two defects sit in those four steps, and both make the check pass whatever the
product does.

**The ping verdict is discarded.** `l.ping(ctx, zePeer, swanIP, 4)` returns the
command output and the call site keeps no part of it. Nothing asserts a reply
arrived, so 100 percent loss passes.

**One direction of traffic advances BOTH peers' counters.** Ze encrypting the
echo request advances Ze's outbound SA; strongSwan decrypting that same request
advances strongSwan's inbound SA. The two assertions read as two independent
observations and are one observation written twice. Nothing in the check requires
strongSwan to have encrypted anything toward Ze.

**Measured, 2026-08-30, scenario `psk-site-to-site`.** charon's `bypass-lan`
plugin installs a PASS shunt for every locally attached subnet. In this lab that
is 172.28.0.0/24, which holds both containers, at priority 175423 against the
Child SA policy's 399999, so the lower number wins and every packet strongSwan
sends to Ze leaves in the clear. Ze drops each one as `XfrmInTmplMismatch`, the
ping reports 100 percent loss, and `verifyTunnelTraffic` passes. So strongSwan
has apparently never encrypted toward Ze in this lab, and a defect confined to
Ze's inbound decapsulation would leave all nine scenarios green. Journal row:
`plan/journal/green-that-could-not-have-been-red.md`, 2026-08-30.

This is the failure `ai/rules/principles.md` names first: "a test whose passing
assertion would also pass against a stub", inside the directive that "a value
that is silently wrong MUST NOT be reachable".

The spec has four goals.

1. **Prove both directions.** Asserting the ping succeeded is necessary and is
   not sufficient, because the shunt is exactly what makes an unprotected ping
   succeed. The discriminating observation is per-direction: the SA carrying
   Ze to strongSwan and the SA carrying strongSwan to Ze are separate rows of
   `ip -s xfrm state` and must be read separately.
2. **Run every affected scenario under the strengthened assertion and report
   which ones were passing on one direction only.** That report is a deliverable
   (AC-9), not a side effect.
3. **Route every red the report produces.** A red that is a Ze defect is fixed in
   this spec. A red that is a lab artifact is fixed in the lab. Neither is routed
   around, and no scenario is weakened or deleted to reach green
   (`ai/rules/completion.md`).
4. **Settle `bypass-lan` for the whole lab, in one place.** Three scenarios carry
   a private `strongswan.conf` that disables it. Under `ai/rules/no-layering.md`
   a lab-wide default REPLACES those files rather than sitting beside them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - "Prove a scenario discriminates",
      the four vacuity traps, and the scenario-input contract
  → Constraint: a scenario is evidence only after it has been watched go RED with
    the behavior broken, and the run must quote the image ID the harness built,
    because a shared tag is rebound by any other session's build.
  → Constraint: "An absent value never proves a negative assertion by itself. The
    operation must also name positive evidence that the query mechanism ran." So
    asserting that `XfrmInTmplMismatch` did NOT rise cannot be the primary proof.
  → Decision: assertions live in typed Go checkers under `internal/le/interoplab/`,
    and a scenario directory carries only declarative inputs. A lab-wide daemon
    setting is therefore a fixture file, never a checker branch.
- [ ] `ai/rules/interop-and-goal-validation.md` - the red-phase obligation and the
      Goal Validation table
  → Constraint: reverting the fix, rebuilding the artifact the test drives, and
    recording the RED is owed before this change may be called validated. Here the
    revert is free: running the strengthened assertion with `bypass-lan` still
    loaded IS the red phase, and it is a red the product cannot cause.
  → Constraint: scenario directories are NAMED. No numeric prefix, and a planned
    scenario is named too. This spec creates no scenario.
- [ ] `ai/rules/principles.md` - the silently-wrong-value directive
  → Constraint: quoted verbatim in the Task section. An assertion that would pass
    against a stub is the largest recorded defect source in this repository, and
    the two peers' counters after one stimulus are that shape.
- [ ] `ai/rules/no-layering.md` - replacing X with Y
  → Constraint: "X MUST be deleted first and Y MUST be implemented after." A
    lab-wide `bypass-lan` default deletes the three per-scenario copies. Keeping a
    per-scenario override "for clarity" is the banned hybrid.
- [ ] `ai/rules/simplicity.md` - the simplest FULLY correct answer
  → Decision: the directed assertion takes the set of directions its caller can
    claim, because `checkESPFormChange` legitimately cannot claim one of the four.
    That is one parameter serving two real callers, not an option nobody asked for.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - the security association is unidirectional
  → Constraint: RFC 4301 Section 4.1 reads, in `rfc/full/rfc4301.txt`: "An SA is a
    simplex "connection" that affords security services to the traffic carried by
    it." A bidirectional protected flow is therefore TWO SAs. The current
    assertion treats a peer's SA set as one aggregate, which is why one direction
    satisfies it. Reading per SA is reading the unit the RFC defines.

**Key insights:** (minimal context to resume after compaction)
- `xfrmCounters` returns `map[SPI]uint64` and drops the `src X dst Y` header line
  that identifies the direction. `parseXFRMCounters` already detects that line as
  a record boundary, and `srcDstPattern` already exists in the same file for
  `espPolicyPairs`. The direction is available and thrown away.
- An SPI is chosen by the RECEIVER, so Ze's outbound SA and strongSwan's inbound
  SA carry the SAME SPI. Direction can only come from src and dst, never from the
  SPI, and this is precisely why the aggregate maps conflate.
- `assertESPAdvanced` compares only SPIs present in BOTH snapshots, so a rekey
  that retires an SA does not fail the check. That property must survive.
- The per-scenario `strongswan.conf` is mounted at
  `/etc/strongswan.d/99-interop.conf` and only when the scenario also has a
  `swanctl.conf` (`prepareScenario`, `internal/le/interoplab/ipsec/ipsec.go`). It
  is a drop-in, not a replacement, so a shared drop-in composes with it.
- `test/interop-ipsec/daemons` and `test/interop-ipsec/vtysh.conf` are already
  shared fixtures mounted from the suite root for the FRR peer. A shared
  strongSwan drop-in follows an existing pattern rather than inventing one.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/interoplab/ipsec/helpers.go` - `verifyTunnelTraffic` reads both
      peers' counters, calls `ping` and discards its result, then calls
      `assertESPAdvanced` on each peer's aggregate map. `xfrmCounters` runs
      `ip -s xfrm state` and returns `parseXFRMCounters` output, keyed by SPI
      alone. `ping` returns the command output. `assertESPAdvanced` returns nil on
      the first common SPI whose byte count rose.
- [ ] `internal/le/interoplab/ipsec/checkers.go` - nine `verifyTunnelTraffic` call
      sites, each the whole dataplane proof of its scenario.
      `checkESPFormChange` is the tenth ESP-counter site: it pings BOTH ways,
      parses `% packet loss` inline with its own regexp, asserts
      `XfrmInStateMismatch` rose, and then makes the same two aggregate
      `assertESPAdvanced` calls.
- [ ] `internal/le/interoplab/ipsec/ipsec.go` - `prepareScenario` builds the
      strongSwan peer's mounts. `swanctl.conf` is required, `strongswan.conf` is
      optional and lands at `/etc/strongswan.d/99-interop.conf`, PKI material is
      mounted when the scenario has a PKI directory.
- [ ] `internal/le/interoplab/ipsec/ipsec_test.go` - `fakeCheckerLab` scripts one
      response list per (peer, argv) key. `xfrmCounter` builds a counter fixture
      whose header is `src 172.28.0.2 dst 172.28.0.3` for BOTH peers, so the
      strongSwan fixture claims a direction the strongSwan container never emits
      for its outbound SA. `TestPSKCheckerRejectsPeerThatAcceptedNoESP` proves the
      checker discriminates against a stalled peer counter, which the real lab
      cannot produce.
- [ ] `test/interop-ipsec/parity_test.go` - pins the exact 24-scenario population
      and, per scenario, the extra input files that must exist. `natt-transport-
      inner-checksum`, `natt-tunnel-inner-checksum`, `eap-tls13`,
      `responder-eap-mschapv2` and `responder-eap-tls13` name `strongswan.conf`.
- [ ] `test/interop-ipsec/scenarios/esp-form-change/strongswan.conf` - carries
      only the `bypass-lan` disable, with the measured reading in its comment.
- [ ] `test/interop-ipsec/scenarios/natt-tunnel-inner-checksum/strongswan.conf` -
      same content, same single setting.
- [ ] `test/interop-ipsec/scenarios/natt-transport-inner-checksum/strongswan.conf` -
      same content, same single setting.
- [ ] `test/interop-ipsec/Dockerfile.strongswan` - Alpine 3.21 charon, run in the
      foreground under tini, with no drop-in of its own.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Only SPIs present in both snapshots are compared, so a rekey between snapshots
  does not fail the check.
- Only `lifetime current` bytes are counted; `lifetime config` and `stats` blocks
  stay excluded (`TestParseXFRMCountersBySPI`).
- The failure text "strongSwan accepted no ESP" stays reachable for the direction
  it names, because `TestPSKCheckerRejectsPeerThatAcceptedNoESP` asserts on it.
- Every existing scenario name, its checker registration, and the lexical
  selection order pinned by `TestNativeScenarioRegistryIsExact`.
- `checkESPFormChange` keeps its `XfrmInStateMismatch` rise assertion and its
  raw-ESP-drop assertion.

**Behavior to change:**
- `verifyTunnelTraffic` proves four directed observations plus a lossless ping,
  instead of two aggregate observations and a discarded ping.
- `xfrmCounters` reports each SA's direction alongside its SPI.
- `charon.plugins.bypass-lan.load = no` applies to every strongSwan peer in the
  lab, from one shared file, and the three per-scenario copies are deleted.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator or a gate runs the native action `integration/interop-ipsec`,
  optionally selecting scenarios with `IPSEC_INTEROP_SCENARIO`. `RunAt` discovers
  the scenario directories, joins each to its typed checker, and starts the
  containers.
- Format at entry: a scenario directory of declarative files (`ze.conf`,
  `swanctl.conf`, optional `strongswan.conf`, optional PKI and FRR inputs).

### Transformation Path
1. `prepareScenario` (`ipsec.go`) renders `ze.conf` into the run scratch directory
   and builds the peer mount list, including the strongSwan daemon drop-ins.
2. The harness starts the Ze and strongSwan containers and waits for the readiness
   probe (`swanctl --stats` plus `swanctl --load-all`).
3. The scenario's checker runs `establish`, then its own protocol assertions, then
   `verifyTunnelTraffic`.
4. `verifyTunnelTraffic` runs `ip -s xfrm state` on each peer over `docker exec`,
   parses the text into directed byte counters, drives a ping from Ze, re-reads
   both peers, and compares.
5. The checker's error, or nil, becomes the scenario verdict in `Report`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ Ze container | `docker exec` running `ip -s xfrm state` and `ping` | No |
| Runner ↔ strongSwan container | `docker exec` running `ip -s xfrm state` | No |
| charon ↔ Linux XFRM | charon installs policies and SAs; `bypass-lan` installs PASS shunts whose priority outranks the Child SA policy | No |
| Ze IKE ↔ Linux XFRM | Ze installs its own SAs and policies over netlink | No |

### Integration Points
- `scenarioCheckers` (`checkers.go`) - the registry the runner reads; the nine
  call sites live in the functions it names.
- `interoplab.PeerConfig.Mounts` - where the shared strongSwan drop-in is added.
- `test/interop-ipsec/parity_test.go` - the inventory that keeps scenario inputs
  and the registry from drifting.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ip -s xfrm state` prints a `src <addr> dst <addr>` header for every SA on both images | `parseXFRMCounters` already treats that unindented line as a record boundary; `espPolicyPairs` parses the same shape from `ip xfrm policy` | The direction key cannot be built and the mechanism fails | Run A prints the raw dump for both peers into the run log before any parsing change is trusted | unvalidated |
| A-2 | Ze's outbound SA is `src 172.28.0.2 dst 172.28.0.3` and strongSwan's outbound SA is `src 172.28.0.3 dst 172.28.0.2`, in tunnel mode as well as transport | `zeIP` and `swanIP` constants, and the tunnel endpoints are the container addresses in every scenario `ze.conf` | Direction keys never match and every scenario reds for the wrong reason | The same Run A dump, read per peer | unvalidated |
| A-3 | Disabling `bypass-lan` lab-wide leaves container control traffic working, because every query and every readiness probe runs over `docker exec` rather than the lab network | `prepareScenario` readiness probe and `(*scenarioLab).exec` both use the lab's exec path | A scenario hangs at readiness or a query times out | Run B over all 24 scenarios | unvalidated |
| A-4 | No scenario config sets `charon.plugins.bypass-lan` for any purpose other than disabling it | The three files read in Current Behavior carry only that setting | A shared drop-in and a scenario drop-in disagree and the winner is undefined | `TestNoScenarioCarriesItsOwnBypassLanOverride` | unvalidated |
| A-5 | `checkESPFormChange` cannot claim Ze's INBOUND kernel SA, because the scenario exists on the forms disagreeing and Ze receives that ESP in userspace | The checker asserts `XfrmInStateMismatch` rose, which is the kernel refusing the inbound state | Forcing four directions there reds a scenario that is behaving correctly | The scenario's own Run B verdict, and `TestESPFormChangeClaimsThreeDirections` | unvalidated |
| A-6 | The nine scenarios' reds in Run B are lab artifacts of the shunt, not Ze defects | The journal row measures the shunt as the cause in `psk-site-to-site` only | A real Ze inbound-path defect is in scope and this spec grows | Run B, per scenario, with the routing table of AC-9 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | With `bypass-lan` off and a 0.0.0.0/0 Child SA selector, strongSwan's unrelated traffic is matched by the tunnel policy and the container misbehaves | A scenario that passed in Run A times out at readiness in Run B | charon's `bypass-lan` accepts `interfaces_ignore`, so the shunt can be suppressed on the lab interface alone rather than removing the plugin. The per-scenario override is NOT restored |
| R-2 | A rekey lands between the two snapshots and one direction has no surviving SA, so a correct scenario reds | `child-rekey` or `responder-raises-child-rekey` reds with an empty direction | The message distinguishes "no surviving SA in this direction" from "the SA did not advance", and the checker waits for the rekey to settle before it measures |
| R-3 | The unit fixtures are corrected to satisfy the new assertion rather than to describe what the containers emit, and the tests become self-agreeing again | A fixture whose src and dst were chosen to make a test pass | Every fixture header is taken from the Run A raw dump of the real containers, and the dump is quoted in the spec at closure |
| R-4 | Run A is measured against an image another session rebuilt under the shared tag | The image ID differs between the Run A and Run B logs with no build in between | Let the harness build, quote the `docker build -q` image ID beside each run, per `docs/architecture/testing/interop.md` |
| R-5 | A scenario stays red in Run B for a reason nobody roots, and the temptation is to exempt it | A routing row with no producer named | `ai/rules/completion.md`: the red is either a Ze defect fixed here or a lab artifact fixed here. A third answer needs the owner |
| R-6 | The ping loss clause passes on output the fixture never produced, because the regexp found no match and the code read that as success | A ping fixture with no summary line still passes | Absence of a `% packet loss` match is a FAILURE, never a pass, and `TestVerifyTunnelTrafficRejectsUnparseablePing` pins it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. This is interop test infrastructure. A wrong landing costs the nine scenarios their verdict, in either direction: a false red blocks the gate, a false green restores the hole this spec closes |
| How is it reverted? | Single commit revert. No config migration, no wire change, no daemon behavior |
| Who else touches this path? | `plan/spec-ipsec-dataplane-inspection.md` plans a `dataplane-readback` scenario in the same suite; `plan/future/spec-fixit-strongswan-tls13-certreq-authorities.md` edits `eap-tls13/strongswan.conf`. Neither touches `verifyTunnelTraffic` or the shared drop-in |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le integration` selects `psk-site-to-site`, the runner calls the registered checker | → | `checkPSKSiteToSite` calls `verifyTunnelTraffic` with four directed claims | `TestPSKCheckerRejectsTrafficThatStrongSwanNeverEncrypted` |
| `./le integration` selects `esp-form-change` | → | `checkESPFormChange` calls the directed assertion with three claims | `TestESPFormChangeClaimsThreeDirections` |
| `prepareScenario` builds the strongSwan peer for every scenario | → | the shared `strongswan-lab.conf` mount is present in each plan | `TestEveryStrongSwanPeerMountsTheSharedLabDropIn` |
| The whole suite run | → | nine checkers assert both directions against a live charon | Run B recorded in AC-9 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ip -s xfrm state` output holding two SAs with the SAME SPI in opposite directions | The parser reports two distinct entries, each carrying its own source, destination and byte count |
| AC-2 | A snapshot pair where Ze's outbound SA advanced and strongSwan's inbound SA advanced, and neither reverse SA moved | `verifyTunnelTraffic` returns an error naming strongSwan as the peer that encrypted nothing toward Ze |
| AC-3 | A snapshot pair where all four directed SAs advanced, and the ping reports 100 percent loss | `verifyTunnelTraffic` returns an error naming the loss |
| AC-4 | A ping reporting 0 percent loss with no directed SA advancing at all | `verifyTunnelTraffic` returns an error; a lossless ping alone never passes the check |
| AC-5 | Ping output carrying no `% packet loss` summary at all | `verifyTunnelTraffic` returns an error naming the unreadable ping output, and does not treat the absent match as success |
| AC-6 | A direction whose SPI is present before and absent after, the rest advancing | The error names that direction as having no surviving SA, with wording distinct from a stalled counter |
| AC-7 | `checkESPFormChange` running against a form disagreement | It asserts Ze outbound, strongSwan inbound and strongSwan outbound, and does NOT require Ze's inbound kernel SA, because that ESP is received in userspace |
| AC-8 | Every prepared scenario plan that carries a strongSwan peer | Its mount list carries the shared lab drop-in, read-only, at a path under `/etc/strongswan.d/` |
| AC-9 | The full 24-scenario suite run twice: Run A with the strengthened assertion and the shunt still installed, Run B with the shunt removed lab-wide | Both verdict sets are recorded per scenario in the Goal Validation section, each naming the image ID the harness built. Every scenario red in Run A and green in Run B is reported as having been passing on one direction only |
| AC-10 | Any scenario still red in Run B | It is rooted to a producer and fixed in this spec: a Ze defect at the Ze source, a lab artifact in the lab fixture. No scenario is weakened, skipped or deleted |
| AC-11 | A grep for `bypass-lan` over `test/interop-ipsec/scenarios/` after the change | Returns nothing. The setting exists once, in the shared file, and the three per-scenario copies are gone |
| AC-12 | A reader consulting `docs/architecture/testing/interop.md` about vacuity | Finds a fifth trap row describing an assertion whose clauses are satisfied by one stimulus, and a note naming the shared strongSwan drop-in and why `bypass-lan` is off |

## Measurement Record (AC-9, AC-10)

<!-- LIVE: the population is pinned NOW so no scenario can be dropped from the
     report later. Run A is the strengthened assertion with the bypass-lan shunt
     still installed. Run B is the same assertion with the shunt gone lab-wide.
     Routing is owed for every Run B red, and for nothing else. -->

Run A image ID: pending. Run B image ID: pending.

| Scenario | Calls `verifyTunnelTraffic` | Run A | Run B | Routing |
|----------|------------------------------|-------|-------|---------|
| `child-rekey` | Yes | pending | pending | pending |
| `child-rekey-narrowing` | No | pending | pending | pending |
| `clear-reestablish` | No | pending | pending | pending |
| `cookie-challenge` | Yes | pending | pending | pending |
| `delete-while-window-held` | No | pending | pending | pending |
| `eap-mschapv2` | Yes | pending | pending | pending |
| `eap-tls` | No | pending | pending | pending |
| `eap-tls13` | Yes | pending | pending | pending |
| `esn-both-offered` | Yes | pending | pending | pending |
| `esn-extended-only-refused` | No | pending | pending | pending |
| `esp-form-change` | No, uses the directed assertion with three claims | pending | pending | pending |
| `initiator-rekey-answer-narrows` | No | pending | pending | pending |
| `invalid-ke-retry` | Yes | pending | pending | pending |
| `ipsec-bgp-redistribute-frr` | No | pending | pending | pending |
| `natt-transport-inner-checksum` | No | pending | pending | pending |
| `natt-tunnel-inner-checksum` | No | pending | pending | pending |
| `peer-reload-narrowing` | No | pending | pending | pending |
| `psk-site-to-site` | Yes | pending | pending | pending |
| `responder-accepts-reinit` | No | pending | pending | pending |
| `responder-eap-mschapv2` | No | pending | pending | pending |
| `responder-eap-tls13` | No | pending | pending | pending |
| `responder-ike-rekey` | No | pending | pending | pending |
| `responder-psk` | Yes | pending | pending | pending |
| `responder-raises-child-rekey` | Yes | pending | pending | pending |

A scenario red in Run A and green in Run B was passing on one direction only.
A scenario red in Run B is a hidden gap, and its Routing cell names the producer
and the fix (AC-10). A scenario whose Run A and Run B verdicts differ while it
does not call the assertion is a `bypass-lan` side effect and belongs to R-1.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the IPsec interop gate and gets a verdict on whether Ze's ESP works in both directions against strongSwan | `./le integration` → `RunAt` → `scenarioCheckers` → `verifyTunnelTraffic` → directed XFRM counters on both peers | Run B of AC-9, `psk-site-to-site` |
| 2 | Breaks Ze's inbound decapsulation and expects the suite to say so | Same path; Ze's SA from strongSwan stops advancing | The Run A red set of AC-9, which is that failure produced by the shunt |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseXFRMCountersKeepsDirection` | `internal/le/interoplab/ipsec/ipsec_test.go` | AC-1: two SAs sharing one SPI in opposite directions stay distinct, and `lifetime config` bytes stay excluded | |
| `TestVerifyTunnelTrafficRejectsOneWayTraffic` | same | AC-2: the exact 2026-08-30 measurement, as a fixture. Ze out and strongSwan in advance, both reverse SAs stall, the check fails | |
| `TestVerifyTunnelTrafficRejectsLossyPing` | same | AC-3 | |
| `TestVerifyTunnelTrafficRejectsPingWithNoESP` | same | AC-4: a lossless ping over a clear path fails | |
| `TestVerifyTunnelTrafficRejectsUnparseablePing` | same | AC-5 and R-6: no summary line is a failure, not a pass | |
| `TestVerifyTunnelTrafficNamesTheDirectionThatStalled` | same | AC-2 and AC-6: four subtests, one per stalled direction, each asserting its own message; plus the retired-SA wording | |
| `TestPSKCheckerRejectsTrafficThatStrongSwanNeverEncrypted` | same | Wiring: the assertion is reached through the registered checker, not called directly | |
| `TestPSKCheckerRequiresSuccessfulHandshakeAndPeerESP` (existing, fixtures corrected) | same | The positive fixture carries four direction-correct SAs and a 0 percent loss summary | |
| `TestPSKCheckerRejectsPeerThatAcceptedNoESP` (existing) | same | The "strongSwan accepted no ESP" wording survives for the direction it names | |
| `TestAssertESPAdvancedUsesSurvivingSPIs` (existing, extended) | same | Surviving-key semantics preserved under the directed key | |
| `TestESPFormChangeClaimsThreeDirections` | same | AC-7 and A-5 | |
| `TestEveryStrongSwanPeerMountsTheSharedLabDropIn` | same | AC-8, over the plans `prepareScenario` builds | |
| `TestNoScenarioCarriesItsOwnBypassLanOverride` | `test/interop-ipsec/parity_test.go` | AC-11 and A-4: no-layering, enforced by inventory | |
| `TestEveryNativeScenarioHasCompleteInputs` (existing, edited) | same | The two `natt-*` rows lose `strongswan.conf` when the files are deleted | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ping packet loss percent | 0-100 | 0 | N/A (no negative loss is emitted) | 1 |
| directed SA byte delta | 0 upward | 1 (any rise) | 0 (no rise) | N/A |
| directions claimed by a caller | 1-4 | 4 (`verifyTunnelTraffic`) | 0 (a caller claiming nothing must be refused) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestNativeScenarioRegistryIsExact` | `test/interop-ipsec/parity_test.go` | The 24-scenario population and lexical order survive the fixture deletions | |
| `TestScenarioPlansPreserveTopologyAndInputs` | `internal/le/interoplab/ipsec/ipsec_test.go` | The prepared plans still carry every mount each scenario needs, plus the new shared one | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `psk-site-to-site` | `test/interop-ipsec/scenarios/` | strongSwan 5.9.14 | ESP flows in BOTH directions over a PSK site-to-site Child SA | |
| `eap-mschapv2` | same | strongSwan 5.9.14 | Both directions after EAP-MSCHAPv2 authentication | |
| `eap-tls13` | same | strongSwan 5.9.14 | Both directions after EAP-TLS over TLS 1.3 | |
| `responder-psk` | same | strongSwan 5.9.14 | Both directions with Ze as responder | |
| `child-rekey` | same | strongSwan 5.9.14 | Both directions survive a Child SA rekey | |
| `responder-raises-child-rekey` | same | strongSwan 5.9.14 | Both directions after Ze raises the rekey | |
| `cookie-challenge` | same | strongSwan 5.9.14 | Both directions on the SA built after a COOKIE challenge | |
| `invalid-ke-retry` | same | strongSwan 5.9.14 | Both directions on the SA built after INVALID_KE_PAYLOAD | |
| `esn-both-offered` | same | strongSwan 5.9.14 | Both directions after Ze selects non-extended sequence numbers | |
| `esp-form-change` | same | strongSwan 5.9.14 | Three directions plus the userspace receive path, under a form disagreement | |
| Remaining 14 scenarios | same | strongSwan 5.9.14, FRR 10.3.1 | Unchanged verdicts under a lab with no `bypass-lan` shunt (R-1) | |

## Files to Modify
- `internal/le/interoplab/ipsec/helpers.go` - directed counters, the directed
  assertion, the ping loss reader, `verifyTunnelTraffic`
- `internal/le/interoplab/ipsec/checkers.go` - `checkESPFormChange` moves onto the
  shared loss reader and the directed assertion; the nine call sites keep their
  scenario wording and gain the direction the failure names
- `internal/le/interoplab/ipsec/ipsec.go` - mount the shared strongSwan drop-in
  for every strongSwan peer
- `internal/le/interoplab/ipsec/ipsec_test.go` - corrected and new fixtures
- `test/interop-ipsec/parity_test.go` - the two `natt-*` extra rows, and the new
  no-layering inventory assertion
- `docs/architecture/testing/interop.md` - the fifth vacuity trap row, and the
  strongSwan daemon drop-in note. Declared by the `// Design:` header of
  `helpers.go`, `checkers.go` and `ipsec.go`
- `docs/guide/ipsec.md` - checked against the scenario-input changes
  (`ai/CODE-TO-DOCS.md` maps `test/interop-ipsec/scenarios` here)
- `plan/journal/green-that-could-not-have-been-red.md` - the 2026-08-30 row's Fix
  cell records the lab-wide outcome instead of "the other 22 keep the one-way
  proof"

## Files to Create
- `test/interop-ipsec/strongswan-lab.conf` - the lab-wide charon drop-in that
  disables `bypass-lan`, carrying the measured reading the three deleted files
  held

## Files to Delete
- `test/interop-ipsec/scenarios/esp-form-change/strongswan.conf`
- `test/interop-ipsec/scenarios/natt-transport-inner-checksum/strongswan.conf`
- `test/interop-ipsec/scenarios/natt-tunnel-inner-checksum/strongswan.conf`

Each carries only the `bypass-lan` disable, so the whole file goes. Delete with
plain `rm` and name the paths to `./le commit create` (`ai/rules/git-safety.md`).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface. This is interop test infrastructure |
| YANG validation constraints | N-A | Same |
| YANG custom validators | N-A | Same |
| CLI commands/flags | N-A | The native action `integration/interop-ipsec` already exists and gains no flag |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC. The suite itself is the functional surface, listed under Interop Tests |
| Pipe completeness | N-A | No command output added |
| Env var registration | N-A | `IPSEC_INTEROP_SCENARIO` and `ZE_IPSEC_INTEROP_SUFFIX` already exist and are unchanged |
| Doctor check for runtime dependencies | N-A | No new runtime dependency in the product. The new file is a test fixture mounted into a test container |
| Prometheus counters/metrics | N-A | No daemon state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Nothing a Ze operator can reach changes |
| 2 | Config syntax changed? | No | No `ze.conf` grammar change; the scenario `ze.conf` files are untouched |
| 3 | CLI command added/changed? | No | The native action keeps its name and arguments |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | `bypass-lan` is a charon plugin in the peer image, not a Ze plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, checked against the deleted scenario inputs and edited if it names them |
| 7 | Wire format changed? | No | No Ze encoder or decoder changes |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 4301's simplex-SA reading is what the directed assertion follows. No requirement level changes, so `rfc/short/` and `docs/features/rfc-status.md` gain no row; the proof STRENGTH of the existing ESP rows changes and is recorded in Goal Validation |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (fifth vacuity trap, strongSwan drop-in note). `docs/functional-tests.md` checked: it covers `.ci`, not the interop suites |
| 11 | Affects daemon comparison? | No | No feature claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/interop.md`, the declared owner of all three edited Go files |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | The scenario registry and its population are unchanged; only three optional input files are deleted |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time: `./le spec citation anchors spec plan/spec-fixit-tunnel-traffic-proof-is-one-directional.md`. `docs/architecture/testing/interop.md` is DECLARED by `helpers.go`, `checkers.go` and `ipsec.go` and is named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` and `docs/architecture/testing/interop.md` are read for stale scenario-input examples naming a deleted `strongswan.conf` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the defect reproducible in a unit
   test before touching the assertion
   - Tests: `TestVerifyTunnelTrafficRejectsOneWayTraffic`,
     `TestPSKCheckerRejectsTrafficThatStrongSwanNeverEncrypted`
   - Files: `internal/le/interoplab/ipsec/ipsec_test.go`
   - Verify: both fail against today's code, because today's aggregate assertion
     passes the one-way fixture. That failure is the defect, written down.
2. **Phase: Directed counters** -- carry the direction the parser already sees
   - Tests: `TestParseXFRMCountersKeepsDirection`,
     `TestAssertESPAdvancedUsesSurvivingSPIs`
   - Files: `helpers.go`
   - Verify: same-SPI opposite-direction SAs stay distinct; surviving-key
     semantics unchanged
3. **Phase: The strengthened assertion** -- four directed claims plus a lossless
   ping in `verifyTunnelTraffic`, three claims in `checkESPFormChange`, one shared
   loss reader replacing the inline regexp
   - Tests: every remaining unit row of the TDD plan
   - Files: `helpers.go`, `checkers.go`, `ipsec_test.go`
   - Verify: phase 1's tests go green, and each stalled direction names itself
4. **Phase: Run A, the recorded RED** -- the full 24-scenario suite with the shunt
   still installed
   - Files: none. A measurement
   - Verify: the red set is recorded per scenario with the harness image ID, and
     the raw `ip -s xfrm state` dumps of both peers are captured to validate A-1
     and A-2 before the fixtures are trusted
5. **Phase: Remove the shunt, once** -- create the shared drop-in, mount it for
   every strongSwan peer, delete the three per-scenario files, edit the parity
   inventory, add the no-layering assertion
   - Tests: `TestEveryStrongSwanPeerMountsTheSharedLabDropIn`,
     `TestNoScenarioCarriesItsOwnBypassLanOverride`,
     `TestEveryNativeScenarioHasCompleteInputs`
   - Files: `ipsec.go`, `test/interop-ipsec/strongswan-lab.conf`, the three
     deletions, `parity_test.go`
   - Verify: no `bypass-lan` spelling survives under `scenarios/`
6. **Phase: Run B and routing** -- the full suite again
   - Files: whichever the routing demands
   - Verify: AC-9's table is complete, and every AC-10 red is rooted and fixed
7. **Phase: Documentation** -- the fifth vacuity trap, the drop-in note, the
   `docs/guide/ipsec.md` check, the journal Fix cell
   - Files: `docs/architecture/testing/interop.md`, `docs/guide/ipsec.md`,
     `plan/journal/green-that-could-not-have-been-red.md`
   - Verify: `./le spec citation anchors spec plan/spec-fixit-tunnel-traffic-proof-is-one-directional.md`
     names no unaddressed owner

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test; AC-9 and AC-10 have a recorded run, not an intention |
| Feature completeness | All ten ESP-counter call sites carry the directed assertion, not only the nine that call `verifyTunnelTraffic` |
| Correctness | The direction key comes from src and dst, never from the SPI, because both peers see the same SPI for one direction |
| Correctness | A no-match on the ping summary FAILS. Absence is never read as success (`ai/rules/principles.md`) |
| Naming | Each failure message names the peer that failed to act and the direction, so a reader can tell "strongSwan sent nothing" from "Ze received nothing" |
| Data flow | Nothing about the direction is inferred from a scenario name or a checker; it is read from the peer's own dump |
| Rule: `ai/rules/no-layering.md` | The three per-scenario overrides are DELETED. Grep proves it |
| Rule: `ai/rules/interop-and-goal-validation.md` | Run A is a real red, produced by the shunt rather than by editing the fix out, and both runs quote their image ID |
| Rule: `ai/rules/completion.md` | No scenario is weakened, skipped or deleted to reach green in Run B |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Directed counters | `go test ./internal/le/interoplab/ipsec/ -run TestParseXFRMCountersKeepsDirection` |
| The one-way fixture is refused | `go test ./internal/le/interoplab/ipsec/ -run TestVerifyTunnelTrafficRejectsOneWayTraffic` |
| No per-scenario `bypass-lan` survives | `grep -r bypass-lan test/interop-ipsec/scenarios/` returns nothing |
| The shared drop-in is mounted everywhere | `go test ./internal/le/interoplab/ipsec/ -run TestEveryStrongSwanPeerMountsTheSharedLabDropIn` |
| Run A and Run B verdicts | The Goal Validation table, one row per run, each naming the image ID |
| The fifth vacuity trap | `grep -c "Vacuity trap" docs/architecture/testing/interop.md` finds the table, and the table holds five rows |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The parser reads untrusted-shaped text from a container. A malformed or absent `src`/`dst` header must produce a refusal, never a zero-valued direction that compares equal to a real one |
| Fail closed | Every new comparison fails on absent data. An empty counter map, an empty ping output and an unreadable dump are each errors |
| Resource exhaustion | None. The change adds no command and no loop over unbounded input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A scenario reds in Run B | Root it to a producer, then AC-10: Ze defect fixed at the Ze source, lab artifact fixed in the lab |
| A scenario reds in Run B and the producer is outside this suite | It BLOCKS this spec's goal, so it is fixed here (`ai/rules/completion.md`); it is not a journal row |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| The tunnel proof covers BOTH directions | interop scenario with a red phase | Run B of `psk-site-to-site` green with four directed claims, against Run A of the same scenario red on "strongSwan encrypted nothing toward Ze". Both runs quote the `docker build -q` image ID |
| The ping verdict is no longer discarded | unit test at the entry point | `TestVerifyTunnelTrafficRejectsLossyPing` and `TestVerifyTunnelTrafficRejectsUnparseablePing`, both driven through `verifyTunnelTraffic` |
| A lossless ping alone cannot pass | unit test | `TestVerifyTunnelTrafficRejectsPingWithNoESP`: 0 percent loss, no SA advance, refused. This is the shunt's own signature |
| Which scenarios were passing on one direction only | recorded measurement | The AC-9 table: 24 scenarios, Run A verdict and Run B verdict, one row each |
| Every hidden gap is routed, none dropped | per-scenario routing | The AC-10 column of the same table: each Run B red names its producer and the commit that fixed it |
| `bypass-lan` is settled in one place | inventory test plus grep | `TestNoScenarioCarriesItsOwnBypassLanOverride`, and `TestEveryStrongSwanPeerMountsTheSharedLabDropIn` over the prepared plans |
| The class is written where the next reader meets it | documentation | The fifth row of the vacuity-trap table in `docs/architecture/testing/interop.md`, and the updated Fix cell of the 2026-08-30 row in `plan/journal/green-that-could-not-have-been-red.md` |

Interop: the peer daemon is strongSwan 5.9.14 on Alpine 3.21, already the lab's
peer. No new scenario is created and no scenario is renamed.

## Design Insights

- The two peers' ESP counters are not two observations. An SPI is chosen by the
  receiver, so one packet advances the sender's outbound SA and the receiver's
  inbound SA under the SAME SPI value. Any assertion that reads a peer's SA set as
  an aggregate has one observation wearing two names.
- RFC 4301 already says this: an SA is simplex. Reading the aggregate was reading
  a unit the protocol does not define.
- `TestPSKCheckerRejectsPeerThatAcceptedNoESP` is a discrimination test that
  passes today while the lab carries the defect it is named for. The fake can
  express a peer counter that does not advance; the real lab cannot, because Ze's
  own traffic advances it. A double that can express a situation the real
  collaborator never produces certifies a discrimination that does not exist.
  Same shape as the `fakePlatform.parentIfindex` rows in
  `plan/journal/green-that-could-not-have-been-red.md`.
- The existing `xfrmCounter` test helper writes `src 172.28.0.2 dst 172.28.0.3`
  for both peers. A fixture in a shape the producer never emits is the mechanism
  of the 2026-08-23 config-list rows in the same journal file, met again here.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Prove both directions with per-direction SA counters PLUS a lossless ping | (a) assert the ping succeeded and nothing else; (b) ping from strongSwan as well and keep the aggregate counters; (c) assert `XfrmInTmplMismatch` did not rise | (a) is necessary and not sufficient: the PASS shunt is precisely what makes a clear-text ping succeed, so it proves reachability and says nothing about protection. (b) adds a stimulus without adding an observation: two pings move the same aggregate maps, so an assertion that already passed on one direction still passes. It is the trap repeated, not fixed. (c) is an absence assertion, which the interop page names as vacuity trap 2, and it proves nothing carried. Direction-keyed counters are the only reading that distinguishes, and the ping clause ties the protected bytes to a completed round trip |
| Key the counter map by source, destination and SPI | Key by SPI and infer the direction from which peer was queried | The peer alone cannot say which of its own SAs moved, and both of a peer's SAs live in one dump. The producer already prints the direction on the record header, and `parseXFRMCounters` already detects that header as a boundary. Inferring it would be a second declaration of a fact the dump states |
| `bypass-lan` off lab-wide, from `test/interop-ipsec/strongswan-lab.conf` mounted into every strongSwan peer | (a) leave it per-scenario; (b) bake the setting into `Dockerfile.strongswan` | (a) is what the journal row deferred and what Thomas replaced with this spec: 22 scenarios keeping a one-way proof is the hole, not a boundary. (b) hides a lab fixture in an image layer, invisible to a reader of `test/interop-ipsec/`, and needs an image rebuild to change. The mount follows the existing shared-fixture pattern of `daemons` and `vtysh.conf`, which are mounted from the same suite root for FRR |
| Delete the three per-scenario `strongswan.conf` files | Keep them as documentation of the reason | `ai/rules/no-layering.md`: X is deleted, Y is implemented. Two files setting one value is a future disagreement with nothing to arbitrate it. The measured readings in their comments move into the shared file, so no reading is lost |
| The directed assertion takes the set of directions the caller claims | Always demand four | `checkESPFormChange` exists on the two peers disagreeing about ESP form, so Ze receives that ESP in userspace and its inbound KERNEL SA correctly does not advance. Forcing four there would red a scenario that is behaving as designed (A-5) |

## Known Limitations

- The assertion proves that ESP bytes moved in each direction inside the
  measurement window and that the ping completed. It does not prove those bytes
  WERE the ping. The lab carries no other traffic between the two peers, and
  narrowing further would need a per-packet capture that no scenario needs today.
- The four directed claims assume a two-peer topology using `zeIP` and `swanIP`.
  `ipsec-bgp-redistribute-frr` adds an FRR peer, and it does not call
  `verifyTunnelTraffic`. A future three-peer dataplane proof would need the
  directions passed in rather than taken from the constants.

## RFC Documentation (Scope: protocol)

RFC 4301 Section 4.1 defines a security association as a simplex connection, so a
protected bidirectional flow is two SAs. The directed assertion's comment cites
that section and quotes it, because the reason for reading per SA rather than per
peer IS that sentence. No Ze protocol code changes, so no new
`// RFC NNNN Section X.Y:` comment lands in a producer, and no `rfc/short/`
requirement level moves.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
