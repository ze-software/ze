# Spec: fixit-ike-test-discrimination

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | `plan/deferrals/fixit-ike-dpd-cleartext.md` |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**~~Four~~ Five places in the IKE and IPsec test surface report green without testing the
thing they name.**

Found on 2026-08-02 while closing the rfcgate-1b RFC 7296 pilot spec. Two were
measured by reverting the production guard and re-running. Two were read.

`ai/rules/interop-and-goal-validation.md` states the standard this spec applies: a
passing test is evidence only if it would FAIL when the behavior under test is
broken. Each item below fails that standard, so the coverage it appears to give is
not there.

### Item 1: the RFC 2759 one-octet guard survives its own revert

`mschapv2Method.Process` (`internal/component/ike/eap/eap_mschapv2.go`) refuses
an empty response at `:89`, guarding the `response.TypeData[0]` read at `:93`. Its
comment explains the floor carefully, including why a blanket four-octet floor was
wrong.

Reverting that guard leaves the WHOLE `eap` package green. No test drives an
MSCHAPv2 response with an empty `TypeData`, so nothing reaches the panic the guard
prevents. MEASURED on 2026-08-02.

### Item 2: the pendingRekey clear defer survives its own revert

`runEstablished` (`internal/component/ike/engine/established.go`) clears the
session rekey slot in a deferred call at `:71`, with a comment stating it is safe
there and only there.

Reverting that defer leaves the whole `engine` package green. MEASURED on
2026-08-02.

This one is UNPROVEN HARDENING rather than a false claim, and the distinction
matters. `RFC7296-2.12-1` is tagged over `forgetKeys`, and those tags describe what
they actually prove. No public compliance claim outruns its evidence here. What is
missing is a test for the clear itself.

### Item 3: two EAP-TLS interop scenarios never assert ESP is accepted

`04-eap-tls/check.py` and `06-eap-tls13/check.py` both stop at `wait_xfrm_sa` for
each container and then call `log_pass`. Presence of an XFRM SA is necessary and
not sufficient: it proves the SAs were installed, never that ESP traffic is
accepted across them.

Scenario `07-responder-psk/check.py` is the in-tree control and shows the fix
already exists: it pings across the tunnel and then reads the ESP counters, with a
comment recording which counter advances and which does not. Scenario `02` follows
the same pattern by carrying BGP routes through the tunnel.

So this is a back-fill of a pattern the tree already has, not a new technique. READ,
not measured.

### Item 4: a loopback probe is a blind instrument

A loopback egress moves NO xfrm counter. Any future QEMU probe built on `lo` will
therefore report zero whether the dataplane works or not, which is the
absence-assertion vacuity trap named in `ai/rules/interop-and-goal-validation.md`.

This item is a CONSTRAINT rather than a defect: nothing in the tree is wrong today.
It is recorded here so the next author of an xfrm probe reads it before choosing an
interface, and so that author pairs any counter assertion with a positive control
that is known to move the counter.

~~`ai/rules/platform-linux.md` is arguably the better long-term home for this
constraint. Moving it there is a rule change and belongs to the owner, so this spec
records it and does not edit the rule.~~

→ Decision 2026-08-15, R-3 RESOLVED: the constraint has three homes, each for a different
reader, and the rule is one of them.

| Home | Reader | Form |
|------|--------|------|
| `ai/rules/points/platform-linux/how-to-write-a-qemu-integration-test/a-dataplane-counter-owes-an-egress-and-a-positive-control.md` | an agent, through the platform-linux trigger | a MUST-level directive under its own heading, "4. Dataplane counters need a real remote peer", rendered into `ai/rules/platform-linux.md` |
| `docs/architecture/testing/qemu-integration.md`, "Dataplane Counters Need a Real Remote Peer" | a person writing a QEMU integration test | prose with a source anchor at the rule section |
| `scripts/evidence/qemu-run.py` docstring | a probe author starting from the driver | a six-line pointer at the rule section, not a third copy of the text |

→ Constraint: the rule edit is an OWNER-VISIBLE decision. This spec's Task section said the
move belonged to the owner. The move was made on instruction during closure, so the record
above replaces the earlier "stays an owner question", and the closure report carries the
change to Thomas rather than burying it here.

→ Constraint: the directive is about the SELECTOR, not the interface. A counter reads zero
for a self-addressed packet when it sits behind state written for a remote peer, which is
what an `ip xfrm` counter does. A plain nftables rule counter in an input or output chain
still advances, so the rule names that as an explicit non-instance. The first draft
generalised to "moves no counter" and was wrong for nftables; the closure review caught it.

### Item 5: no `.ci` holds a DPD interval open, so the liveness probe is unproven at daemon level

Added 2026-08-03, homed here from the DPD cleartext fixit, which closed the same day. The row lives in
`plan/deferrals/fixit-ike-dpd-cleartext.md`, which survives that closure because it still
holds this live row.

That work fixed a probe Ze sent in cleartext, and proved the fix with four unit tests in
`internal/component/ike/engine/rfc7296_dpd_test.go` plus the existing ipsec suite. It did
not add the daemon-level proof it names as the strongest. That proof is a `.ci` that
configures a DPD `interval` and asserts the tunnel is STILL up after more than one interval
has passed. Its own words: "Add one when the harness can drive a peer for longer than one
DPD interval."

So `test/ipsec/*.ci` proves the SA establishes and re-establishes with the changed probe
path, and nothing proves a configured DPD does not tear a healthy tunnel down. That is the
exact failure the cleartext defect produced in the field, and it is the shape item 3 above
warns about: a necessary assertion standing in for a sufficient one.

~~The blocker is harness duration, not signal.~~ **CLOSED 2026-08-07. There was no
blocker.** `newDPDState` (`internal/component/ike/engine/dpd.go`) returns nil at
`Interval == 0`, so the test configures a non-zero interval and outlives it.

→ Decision: the deferral's stated reason, "the ipsec `.ci` harness cannot drive a peer
for longer than one DPD interval", named a limit the harness never had. The default
per-test budget is 15s (`runCISubcommandInner`, `internal/test/cli/ci_runner.go`) and
`resolveOrchestratedTimeout` (`internal/test/runner/runner_exec_util.go`) lets a `.ci`
override it with no cap, which `ipsec-clear-reestablish.ci` had already done at 60s. The
smallest legal DPD interval is 1 second (`parseDPD`,
`internal/component/ike/ipsec/config.go`, and the YANG `range "1..3600"`), and
`maintainSA` (`established.go`) ticks once a second. So one second of DPD fits inside a
budget the suite was already declaring, and the test needed no harness change. It stays
in the `.ci` tier rather than moving beside interop scenario `07`.

→ Constraint: the assertion is `uptime-seconds` from the SA's own `EstablishedAt`
(`saToMap`, `internal/component/ike/cmd/show_ipsec.go`), polled until it passes 20. A DPD
teardown destroys that SA, so the value resets and no host load can carry it past the
threshold. Nothing in the test reads the test's own clock.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/interop-and-goal-validation.md` - the discrimination standard and the vacuity traps
  → Constraint: revert the behavior under test and confirm the test FAILS, before claiming it validates anything
- [ ] `ai/rules/testing.md` - test sensitivity ratchets and the assert-nothing detector
  → Constraint: a test whose oracle is implicit needs the `// test-asserts-nothing:` annotation, so silence is never the answer
- [ ] `ai/rules/platform-linux.md` - what the Alpine VM provides and the virtual substitutes
  → Constraint: a dataplane assertion needs an interface that actually carries the traffic, which is why item 4 exists

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2759.md` - MSCHAPv2 packet shapes, which set the one-octet floor
  → Constraint: the Success Response carries only an OpCode, so the floor is one octet and not four
- [ ] `rfc/short/rfc7296.md` - IKE rekey and the SA lifecycle the pending slot tracks
  → Constraint: `RFC7296-2.12-1` is already tagged over `forgetKeys` and its tags are honest, so item 2 adds coverage without changing a claim

**Key insights:** (minimal context to resume after compaction)
- Items 1 and 2 are measured. Items 3 and 4 are read.
- Item 2 is unproven hardening, NOT a false compliance claim. Say so, and do not narrow any RFC row for it.
- Scenario `07-responder-psk` is the in-tree control for item 3. Copy it.
- Item 4 changes no code. It is a constraint for the next probe author.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/eap/eap_mschapv2.go` - `Process` at `:82`, the empty-response guard at `:89`, the `TypeData[0]` read at `:93`
- [ ] `internal/component/ike/engine/established.go` - `runEstablished` at `:56`, the `pendingRekey.clear()` defer at `:71`
- [ ] `test/ipsec-interop/scenarios/04-eap-tls/check.py` - stops at `wait_xfrm_sa` then `log_pass`
- [ ] `test/ipsec-interop/scenarios/06-eap-tls13/check.py` - same shape
- [ ] `test/ipsec-interop/scenarios/07-responder-psk/check.py` - the control: pings, then reads ESP counters

**Behavior to preserve:** (unless the user explicitly said to change it)
- Both production guards stay exactly as they are. This spec adds tests, it does not change the guards.
- The `RFC7296-2.12-1` tags stay as they are. They describe `forgetKeys` honestly and no claim is being corrected.
- Scenarios `02` and `07` already discriminate and must not be disturbed.
- Every existing `RFC requirement:` tag keeps its id and polarity. `ai/rules/testing.md` makes a tagged test the requirement itself.

**Behavior to change:** (only what the user asked for)
- Add a test that reddens when the RFC 2759 one-octet guard is reverted.
- Add a test that reddens when the `pendingRekey` clear defer is reverted.
- Extend scenarios `04` and `06` to assert ESP is accepted, following scenario `07`.
- Record the loopback constraint where a probe author will read it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An EAP-MSCHAPv2 response arrives from a peer, carrying an attacker-controlled `TypeData` length.
- An established IKE session enters and leaves `runEstablished`, carrying a session-owned rekey slot.
- An interop lab brings up a tunnel between ze and strongSwan.

### Transformation Path
1. EAP response parsed by `Process`, which reads `TypeData[0]` after the length guard.
2. Session rekey slot set by the timer paths, cleared by the deferred call on loop exit.
3. Interop scenario waits for the XFRM SA, then asserts on the dataplane.
4. ESP counters read from both containers to prove traffic crossed the tunnel.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer ↔ EAP method | EAP packet with peer-controlled length | No |
| Session ↔ rekey slot | in-process struct field, cleared on loop exit | No |
| Ze ↔ strongSwan | ESP over the interop lab network | No |
| Container ↔ kernel | `ip xfrm state` counters read per container | No |

### Integration Points
- `internal/component/ike/eap` - the method registry the MSCHAPv2 method is reached through.
- `internal/component/ike/engine` - the owner loop that holds the rekey slot.
- `test/ipsec-interop/` - the shared scenario helpers, including `wait_xfrm_sa`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `TestMSCHAPv2ProcessRefusesEmptyTypeData` drives `Session.Process`, the authenticator entry point, not `mschapv2Method.Process`. `TestRunEstablishedClearsPendingRekeyOnExit` drives `runEstablished` through `establishPSK`, not the deferred call. Both scenarios read the kernel's own `ip xfrm` counters through `xfrm_sa_bytes_by_spi` rather than trusting Ze's report |
| No unintended coupling (components stay isolated) | Yes | The EAP test stays inside `internal/component/ike/eap`. It cannot import `internal/component/ike/engine`, which is the production producer's package, because `engine` imports `eap`. It builds the identical packet shape with `DecodePacket` instead, and its comment names the production path it stands in for |
| No duplicated functionality (extends existing, does not recreate) | Yes | No helper was added. Both scenarios call `assert_esp_accepted` and `xfrm_sa_bytes_by_spi`, which already existed in `test/ipsec-interop/lab.py` for scenarios 01 and 07. The design question in Files to Create, whether a shared helper belongs in the library, is answered: it is already there |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test-only spec, no encoding path touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An empty-`TypeData` MSCHAPv2 response is reachable from the wire, not only from a constructed unit input | read, the EAP receive path | Item 1 is a defensive guard with no reachable trigger, which lowers severity but still leaves it unproven | trace the EAP packet path from the transport to `Process` | confirmed 2026-08-15 |
| A-2 | Reverting each guard really does leave the package green, and the run was not scoped too narrowly | measured 2026-08-02, both packages | The coverage exists and one of the two items closes immediately | re-run the revert with the full feature tags per `ai/rules/commands.md` | confirmed 2026-08-15 |
| A-3 | ESP counters in scenarios `04` and `06` behave as they do in `07`, so the control transfers | read, `07-responder-psk/check.py` | The assertion needs a different shape for the EAP-TLS labs | run the extended scenarios once | confirmed 2026-08-15 |
| A-4 | No other test already covers either guard from a different package | measured for the two packages, not tree-wide | An existing test covers it and the new one duplicates | grep the tree for the guard's behavior before writing the test | confirmed 2026-08-15 |

→ Decision (A-1): `DecodePacket` (`internal/component/ike/eap/eap.go`) copies `TypeData` only
when the EAP Length is more than 5, so a Length of exactly 5 decodes to an MS-CHAPv2
response whose `TypeData` is empty. The length is peer-controlled and the peer is
unauthenticated, so the guard's trigger is reachable from the network. The test drives
`Session.Process`, the authenticator's own entry point, not the method helper.

→ Decision (A-2, A-4): each revert failed ONLY the new test. Every other test in each
package stayed green, which is the same measurement as 2026-08-02 and shows no other test
covers either guard.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new EAP test asserts the error string rather than the behavior, so a reword breaks it | the test fails on a message change that keeps behavior | assert that no panic occurs and a method failure is returned, never the exact text |
| R-2 | Extending scenarios `04` and `06` makes them flaky, since interop is nightly and advisory | intermittent reds in the nightly tier | wait on the counter condition, never on elapsed time (`ai/rules/completion.md`) |
| R-3 | Item 4's constraint stays in this spec and is deleted at closure, so the next probe author never sees it | a future probe built on `lo` | RESOLVED 2026-08-15. Three durable homes, listed in the Item 4 decision table: the rule point (agent), `docs/architecture/testing/qemu-integration.md` (person), the `qemu-run.py` docstring (pointer at the point of use). Nothing about item 4 dies with this spec |
| R-4 | Adding assertions to a scenario carrying an `RFC requirement:` tag trips the rfc-tagged-test hook | the edit is blocked at write time | check each scenario for tags first; a tagged change needs the owner's approval, never a self-issued one |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. This spec adds tests and changes no production behavior. The risk is a flaky nightly interop scenario, which costs signal rather than function. |
| How is it reverted? | Single commit revert. Test-only. |
| Who else touches this path? | `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` (evidence carriers and tiers), `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` (which interop trees have an automated caller) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| EAP response with empty TypeData | → | `mschapv2Method.Process` length guard | `TestMSCHAPv2ProcessRefusesEmptyTypeData` |
| Established loop exit | → | `runEstablished` pendingRekey clear | `TestRunEstablishedClearsPendingRekeyOnExit` |
| EAP-TLS tunnel up | → | ESP counters on both peers | `test/ipsec-interop/scenarios/04-eap-tls/check.py` |
| EAP-TLS 1.3 tunnel up | → | ESP counters on both peers | `test/ipsec-interop/scenarios/06-eap-tls13/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An MSCHAPv2 response whose `TypeData` is empty | `Process` returns a method failure and does not panic |
| AC-2 | The one-octet guard is reverted | AC-1's test FAILS, proving it discriminates |
| AC-3 | `runEstablished` returns with a rekey pending | The session rekey slot is clear afterwards |
| AC-4 | The clear defer is reverted | AC-3's test FAILS, proving it discriminates |
| AC-5 | Scenario `04-eap-tls` runs against strongSwan | ESP traffic is proven accepted, not only that an XFRM SA exists |
| AC-6 | Scenario `06-eap-tls13` runs against strongSwan | Same as AC-5 |
| AC-7 | A reader plans a QEMU xfrm probe | The loopback constraint and the positive-control requirement are recorded where that reader looks |
| AC-8 | A peer is configured with a non-zero DPD `interval` and stays healthy | The tunnel is still established after more than one interval, proven by a test that outlives the interval rather than by a unit test on the probe builder (item 5) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Authenticates with EAP-TLS and sends traffic | IKE_AUTH → Child SA → ESP across the tunnel | `04-eap-tls/check.py` |
| 2 | Authenticates with EAP-TLS 1.3 and sends traffic | same, over TLS 1.3 | `06-eap-tls13/check.py` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMSCHAPv2ProcessRefusesEmptyTypeData` | `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` | AC-1, AC-2 | done 2026-08-15. Discriminates: deleting the length guard panics it with `index out of range [0] with length 0`, restoring turns it green |
| `TestRunEstablishedClearsPendingRekeyOnExit` | `internal/component/ike/engine/established_test.go` | AC-3, AC-4 | done 2026-08-15. Discriminates twice: deleting the whole defer fails both assertions, deleting only `clear()` fails the `HasPrivate` one |

→ Constraint: the item 1 test lives in a NEW file, not in `eap_mschapv2_test.go` as the
Files to Modify list said. That file carries `RFC requirement:` tags, and
`.claude/hooks/pretool-writeedit.py` refuses every edit to a tagged test file, an addition
included (R-4 predicted this). The new file changes no tagged assertion.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MSCHAPv2 `TypeData` length | 1..N octets | 1 (OpCode only, the Success Response) | 0 (empty, the guard's case) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing IPsec suite | `test/ipsec/*.ci` | ~~No new `.ci` is expected.~~ Confirm at design time whether the EAP guard is reachable from a `.ci`, and add one if it is | |
| DPD hold-open (item 5) | `test/ipsec/ipsec-dpd-holds-tunnel.ci` | An operator configures a DPD `interval` and the tunnel stays up past more than one interval | done 2026-08-07. `make ze-ipsec-test` 14/14, three consecutive runs. Discriminates: blocking the sends in `sendDPD` and `retransmitDPD` turns it RED at the uptime step, restoring them turns it GREEN. RE-MEASURED 2026-08-15 against the current tree: GREEN 15/15, RED 14/15 with only this test failing (`engine step 3: output never matched regexp "uptime-seconds":[2-9][0-9]`, last output `{"peers":[]}`, so the DPD verdict had destroyed the SA), GREEN 15/15 again after a byte-identical restore |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `04-eap-tls` | `test/ipsec-interop/scenarios/` | strongSwan | ESP accepted across an EAP-TLS tunnel, not only SA presence | done 2026-08-15, PASS |
| `06-eap-tls13` | `test/ipsec-interop/scenarios/` | strongSwan | Same, over TLS 1.3 | done 2026-08-15, PASS |

→ Constraint: both scenarios were measured against a broken dataplane, by flipping one
octet of the outbound ESP key in `installChildSA` (`internal/component/ike/engine/child.go`)
so the SAs still install with keys that do not agree. Under that mutation BOTH old
assertions still passed (XFRM SA present on both ends) and so did Ze's own counter. Only
the peer's inbound counter caught it. That is the whole of item 3 in one measurement: the
SA-presence check cannot see a tunnel whose bytes no peer can decrypt.

## Files to Modify
- `scripts/evidence/qemu-run.py` - a pointer at item 4's rule point, in the docstring of the driver every QEMU probe starts from. LANDED 2026-08-15
- `ai/rules/points/platform-linux/how-to-write-a-qemu-integration-test/a-dataplane-counter-owes-an-egress-and-a-positive-control.md` and its manifest - item 4's constraint as a MUST-level directive. LANDED 2026-08-15 at closure, see the Item 4 decision table
- `ai/rules/platform-linux.md` - rendered from the point above by `rules_points.py render`, never hand-edited
- `docs/architecture/testing/qemu-integration.md` - item 4's constraint for a person, under "Writing Integration Tests". LANDED 2026-08-15 at closure
- `docs/functional-tests.md` - the IPsec interop row's fail-closed claim, corrected at closure because it describes the two scenarios this spec changed
- `test/ipsec-interop/scenarios/04-eap-tls/check.py` - assert ESP is accepted, following scenario `07`
- `test/ipsec-interop/scenarios/06-eap-tls13/check.py` - same
- `internal/component/ike/eap/eap_mschapv2_test.go` - the discriminating test for the one-octet guard
- `internal/component/ike/engine/established_test.go` - the discriminating test for the clear defer

## Files to Create
- `test/ipsec/ipsec-dpd-holds-tunnel.ci` - item 5 / AC-8. LANDED 2026-08-07
- Confirm at design time whether a shared ESP-counter assertion helper belongs in the interop library rather than in each scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test-only spec |
| YANG validation constraints | N-A | Test-only spec |
| YANG custom validators | N-A | Test-only spec |
| CLI commands/flags | N-A | Test-only spec |
| CLI grammar (keyword before value) | N-A | Test-only spec |
| Editor autocomplete | N-A | Test-only spec |
| Functional test for new RPC/API | N-A | No new RPC or API |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test discrimination only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | | Item 2 adds coverage without changing a claim. Confirm at design time that no `docs/features/rfc-status.md` row moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if a shared interop assertion helper lands |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for the two scenario paths at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

0. **Phase: DPD hold-open (item 5, AC-8)** - DONE 2026-08-07
   - Tests: `test/ipsec/ipsec-dpd-holds-tunnel.ci`
   - Files: that one file. No harness change was needed; see item 5 above
   - Verify: `make ze-ipsec-test` green, and the RED/GREEN mutation recorded in the
     test's own header comment
1. **Phase: Wiring (MANDATORY FIRST)** - prove each item discriminates before fixing it
   - Tests: the two unit tests, written to FAIL only when the guard is present, then reverted to confirm
   - Files: the two `_test.go` files
   - Verify: each new test reddens when its guard is reverted, which is AC-2 and AC-4
2. **Phase: Interop assertions** - extend scenarios `04` and `06`
   - Tests: `04-eap-tls/check.py`, `06-eap-tls13/check.py`
   - Files: both `check.py` files, and any shared helper
   - Verify: check each scenario for an `RFC requirement:` tag FIRST (R-4). Wait on the counter, never on elapsed time
3. **Phase: Record the constraint** - item 4
   - Tests: none. This phase produces text, not behavior
   - Files: `scripts/evidence/`
   - Verify: a probe author reading the evidence helpers meets the loopback constraint and the positive-control requirement

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | All four items addressed, and item 4 has a durable home rather than only this spec |
| Correctness | Each new test was actually run against the reverted guard, and the revert was undone afterwards |
| Naming | Test names state the behavior asserted, not the mechanism |
| Data flow | The interop assertion reads a counter that the traffic really moves, per item 4 |
| Rule: `ai/rules/interop-and-goal-validation.md` | No new assertion is an absence assertion |
| Rule: `ai/rules/testing.md` | No tagged test changed behavior without the owner's approval |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| EAP guard discriminates | Revert the guard, run `go test ./internal/component/ike/eap/`, confirm RED, restore |
| Rekey clear discriminates | Revert the defer, run `go test ./internal/component/ike/engine/`, confirm RED, restore |
| Scenarios assert ESP | `make ze-ipsec-interop-test` with scenarios 04 and 06 |
| Constraint recorded | `grep -rn "loopback" scripts/evidence/` returns the note |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Item 1 IS an input-validation guard on peer-controlled length. The test must drive it from the peer-facing entry point, not the helper alone (`ai/rules/evidence.md`) |
| Resource exhaustion | None introduced |
| Error leakage | The new EAP test must not pin an error string that reveals internal state |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Item 4 changes no code. If it stays only in this spec it dies at closure, which R-3 tracks.
- Interop scenarios run in the nightly tier and are advisory, so items 3's proof does not gate a merge.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

Five places that reported green without testing what they name are now measured.
No production file changed: `git diff` is empty for `eap_mschapv2.go`,
`established.go`, `child.go` and `dpd.go`.

- `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` (NEW).
  `TestMSCHAPv2ProcessRefusesEmptyTypeData` drives an EAP-Response/MSCHAPv2 with an
  empty `TypeData` through `Session.Process`, the authenticator's own entry point.
- `internal/component/ike/engine/established_test.go`.
  `TestRunEstablishedClearsPendingRekeyOnExit` fills the session rekey slot with a
  real `crypto.DHExchange`, stops the session, and asserts the slot is nil and the
  DH private value is gone.
- `test/ipsec-interop/scenarios/04-eap-tls/check.py` and `06-eap-tls13/check.py`.
  Each pings across the tunnel and calls `assert_esp_accepted` twice, once for Ze
  and once for strongSwan. The peer's counter is the interop assertion.
- `scripts/evidence/qemu-run.py`, `ai/rules/platform-linux.md` and
  `docs/architecture/testing/qemu-integration.md` carry item 4's constraint.
- `test/ipsec/ipsec-dpd-holds-tunnel.ci` (item 5, landed 2026-08-07) was re-measured
  against the current tree.

### Bugs Found/Fixed

- No product bug. This spec exists because two guards had no test, and finding a
  guard untested is not finding it broken. Both guards were correct and stay
  byte-identical.
- The strongest measurement is item 3's. Flipping ONE octet of the outbound ESP key
  in `installChildSA` (`internal/component/ike/engine/child.go`) left BOTH
  pre-existing assertions passing on both ends and Ze's own counter advancing. Only
  the new peer-side assertion failed. The scenarios were proving an SA had been
  created, never that ESP was accepted.
- `docs/functional-tests.md` claimed "Dataplane checks gate on XFRM availability".
  `wait_xfrm_sa` and `assert_esp_accepted` (`test/ipsec-interop/lab.py`) both raise
  `AssertionError`, so nothing gates and nothing skips. Corrected here because the
  row describes the two scenarios this spec changed.

### Documentation Updates

- `docs/architecture/testing/qemu-integration.md`: new subsection "Dataplane
  Counters Need a Real Egress", anchored at the rule point.
- `docs/functional-tests.md`: the IPsec interop row's fail-closed claim corrected,
  and its scenario enumeration dropped for a rule a reader can check.
- `make ze-doc-test`: green. One earlier run reported `ai/DOCS-TO-CODE.md` stale;
  another session regenerated it mid-run and `docs_to_code.py --check` is green.

### Deviations from Plan

- The item 1 test lives in a NEW file, not in `eap_mschapv2_test.go` as Files to
  Modify said. That file carries `RFC requirement:` tags and
  `.claude/hooks/pretool-writeedit.py` refuses every edit to a tagged test file, an
  addition included. R-4 predicted this. No tagged assertion was touched.
- Item 4's constraint landed in three homes rather than one, and one of them is
  `ai/rules/platform-linux.md`, which the Task section had reserved to the owner.
  See the Item 4 decision table and the Mistake Log.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first draft of item 4's constraint said a self-addressed packet "is handled by no dataplane state", so the counter cannot move | That is true for an `ip xfrm` counter, whose SA is selected by a policy naming a remote peer. It is false for a plain nftables rule counter in an input or output chain, which a self-addressed packet still advances | closure review, security lens | The rule point, the doc subsection and the docstring now scope the claim to a counter behind state written for a remote peer, and name the nftables case as an explicit non-instance |
| approach | The new EAP test's comment justified reachability with `DecodePacket` | `eap.DecodePacket` has NO non-test caller. Production decodes with `PayloadEAP.ReadFrom` (`internal/component/ike/wire/payload_eap.go`) and `wireEAPToPacket` (`internal/component/ike/engine/fsm.go`), reached from `handleResponderEAP`. The conclusion held; the named producer did not | closure review, two lenses independently | The comment names the production path and says why the test cannot call it: `engine` imports `eap`, so the fixture is built with `DecodePacket`, which yields the identical shape |
| approach | Both scenarios' comments said the counter is "waited on rather than timed" | Nothing polls. The counters are read once before and once after a blocking ping. R-2's actual ban, an elapsed-time wait, is honoured, so the test is sound and the sentence was not | closure review, three lenses | Reworded to what the code does, and each scenario now records the key-flip measurement that proves it discriminates |
| escalation | Fixing a stale ENUMERATION in `docs/functional-tests.md` by replacing it with a GENERALISATION ("the `responder-` scenarios are the ones where strongSwan starts the exchange") produced a fresh false claim in the same edit | Two scenarios with strongSwan as initiator carry no `responder-` prefix: `18-cookie-challenge` and `23-esp-form-change`, both stating it in their own `swanctl.conf` | checked the scenario directories before recording the fix as done | Pointed at the file that HOLDS the fact instead, having verified all 16 scenarios carry a `swanctl.conf`. The lesson generalises: a doc claim about a set is checked against the set, and swapping a list for a rule swaps one drift for another |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Item 1: a test that reddens when the RFC 2759 one-octet guard is reverted | Done | `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` | Revert panics `index out of range [0] with length 0` |
| Item 2: a test that reddens when the `pendingRekey` clear defer is reverted | Done | `internal/component/ike/engine/established_test.go`, `TestRunEstablishedClearsPendingRekeyOnExit` | Two oracles, each with its own revert |
| Item 3: scenarios `04` and `06` assert ESP is accepted | Done | `test/ipsec-interop/scenarios/04-eap-tls/check.py`, `06-eap-tls13/check.py` | Peer-side `assert_esp_accepted` is the interop assertion |
| Item 4: the loopback constraint recorded where a probe author reads it | Done | rule point, `docs/architecture/testing/qemu-integration.md`, `scripts/evidence/qemu-run.py` | Three homes, three readers. See the Item 4 decision table |
| Item 5: a `.ci` that holds a DPD interval open | Done | `test/ipsec/ipsec-dpd-holds-tunnel.ci` | Landed 2026-08-07, re-measured 2026-08-15 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestMSCHAPv2ProcessRefusesEmptyTypeData` | Asserts `CodeFailure` and `!Succeeded()`, no panic, no error text pinned |
| AC-2 | Done | same test, guard deleted | RED with a panic in `mschapv2Method.Process` |
| AC-3 | Done | `TestRunEstablishedClearsPendingRekeyOnExit` | Slot nil and `dh.HasPrivate()` false after return |
| AC-4 | Done | same test, defer deleted | RED on both assertions; `clear()`-only revert reds one |
| AC-5 | Done | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls` | PASS 2026-08-15; FAIL under the one-octet key flip |
| AC-6 | Done | same for `06-eap-tls13` | PASS 2026-08-15 |
| AC-7 | Done | rule point, qemu-integration doc, qemu-run docstring | Three homes, listed in the Item 4 decision table |
| AC-8 | Done | `test/ipsec/ipsec-dpd-holds-tunnel.ci` | `make ze-ipsec-test` 15/15; RED 14/15 with both `sendRaw` calls blocked |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestMSCHAPv2ProcessRefusesEmptyTypeData` | Done | `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` | New file, not the planned one. See Deviations |
| `TestRunEstablishedClearsPendingRekeyOnExit` | Done | `internal/component/ike/engine/established_test.go` | Appended, no existing assertion changed |
| `04-eap-tls` ESP assertion | Done | `test/ipsec-interop/scenarios/04-eap-tls/check.py` | |
| `06-eap-tls13` ESP assertion | Done | `test/ipsec-interop/scenarios/06-eap-tls13/check.py` | |
| DPD hold-open `.ci` | Done | `test/ipsec/ipsec-dpd-holds-tunnel.ci` | Landed 2026-08-07 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/evidence/qemu-run.py` | Changed | Reduced to a pointer at the rule point during closure review, rather than a third full copy |
| `test/ipsec-interop/scenarios/04-eap-tls/check.py` | Done | |
| `test/ipsec-interop/scenarios/06-eap-tls13/check.py` | Done | |
| `internal/component/ike/eap/eap_mschapv2_test.go` | Changed | Untouched. The test went in a new sibling file, blocked by the RFC-tagged-test hook |
| `internal/component/ike/engine/established_test.go` | Done | |
| `test/ipsec/ipsec-dpd-holds-tunnel.ci` | Done | Created 2026-08-07 |
| shared ESP-counter helper in the interop library | Done | Question answered: `assert_esp_accepted` and `xfrm_sa_bytes_by_spi` already exist in `test/ipsec-interop/lab.py`. Nothing new was added |

### Audit Summary
- **Total items:** 22
- **Done:** 19
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations: the EAP test's file, the qemu-run docstring's form, the untouched tagged test file)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The RFC 2759 one-octet guard no longer survives its own revert | unit, mutation-verified | `TestMSCHAPv2ProcessRefusesEmptyTypeData`. Deleting `len(response.TypeData) == 0` from `mschapv2Method.Process` panics the test with `index out of range [0] with length 0`; restoring it returns the package to green. `make ze-test-pkg PKG=./internal/component/ike/eap` ok 4.060s |
| The `pendingRekey` clear defer no longer survives its own revert | unit, mutation-verified twice | `TestRunEstablishedClearsPendingRekeyOnExit`. Deleting the whole defer from `runEstablished` fails both assertions; deleting only `clear()` fails the `HasPrivate` one, so each half of the defer has its own oracle. `make ze-test-pkg PKG=./internal/component/ike/engine` ok 1.528s |
| Scenarios `04` and `06` prove ESP is ACCEPTED, not that an SA exists | interop, mutation-verified | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls` → `ESP counters advanced on 0xc2c97276` on ze-ipsec-ze and on ze-ipsec-swan, `PASS 1 scenario(s)`. Same for `06-eap-tls13` → `0xc7dd5b9f` on both. THE DISCRIMINATION: with one octet of the outbound ESP key flipped in `installChildSA` (`internal/component/ike/engine/child.go`), both `wait_xfrm_sa` calls still passed and Ze's own counter still advanced. Only strongSwan's inbound counter caught it. That is the whole of item 3 in one measurement |
| The next author of an xfrm probe meets the loopback constraint before choosing an interface | durable record, three readers | The rule point `a-dataplane-counter-owes-an-egress-and-a-positive-control` fires on the platform-linux trigger; `docs/architecture/testing/qemu-integration.md` carries it for a person under "Writing Integration Tests"; the `scripts/evidence/qemu-run.py` docstring points at the point id. `make ze-rules-lint` and `make ze-rules-condensed` green, `TRIGGERS.md` and `CORE.md` unchanged because platform-linux is a triggered rule |
| A configured DPD does not tear a healthy tunnel down, proven at daemon level | functional `.ci`, mutation-verified | `test/ipsec/ipsec-dpd-holds-tunnel.ci`. `make ze-ipsec-test` pass 15/15 100.0% on 2026-08-15. RED 14/15 with both `sendRaw` calls blocked in `dpd.go`, the failing step being `uptime-seconds` never reaching 20 because the DPD verdict destroyed the SA |

## Deferrals Resolved

Shard: `plan/deferrals/fixit-ike-dpd-cleartext.md`. It is NOT removed: row 3 is live.

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Row 1 (2026-08-03): a `.ci` that configures a DPD `interval` and asserts the tunnel is still established after more than one interval | done | `test/ipsec/ipsec-dpd-holds-tunnel.ci`, this spec's item 5 and AC-8. `make ze-ipsec-test` 15/15 on 2026-08-15, RED 14/15 under the blocked-send mutation |
| Row 2 (2026-08-03): `sendDPD` with a nil transport strands the request window | done | Fixed 2026-08-07 under `plan/spec-fixit-ike-resource-lifetime-leaks.md`. Not this spec's work; the row was already terminal |
| Row 3 (2026-08-03): `RFC7296-2.4-2` [SHOULD], liveness checks are demand-driven rather than periodic | deferred | LIVE. Homed at `plan/spec-ike-dpd-demand-driven.md` (Status `skeleton`) on Thomas's answer of 2026-08-07. The shard therefore outlives this closure and is NOT `git rm`ed |

No FOREIGN shard was emptied by these resolutions.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ike-test-discrimination-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md` (5 code files, verdict clean) |
| `review_gate.py check` | clean. `review_gate: OK (5 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 3 |
| Reviewer lenses used | Round 1, three parallel independent agents over the full diff: logic+wiring+test-discrimination, security+RFC+edge-cases, doc-drift+rule-format+simplicity. Round 2, one agent scoped to the round-1 fixes. Round 3, one agent scoped to the round-2 fixes. Every round returned 0 BLOCKER |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The diff edits a rule the spec still recorded as an owner question, so the record and the tree disagreed | `plan/spec-fixit-ike-test-discrimination.md` item 4, R-3 | Item 4 now carries the three-home decision table and names the rule edit as owner-visible; R-3 is marked RESOLVED; the closure report carries the change to Thomas |
| 2 | ISSUE | A false kernel claim in a MUST-level directive: a self-addressed packet "is handled by no dataplane state", generalised to any dataplane counter. False for an nftables rule counter in an input or output chain | the rule point, `ai/rules/platform-linux.md`, `docs/architecture/testing/qemu-integration.md` | Scoped to a counter behind state written for a remote peer, with the selector named as the reason and nftables named as an explicit non-instance |
| 3 | ISSUE | The new EAP test's reachability argument named `eap.DecodePacket`, which has no non-test caller | `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` header | The comment names `PayloadEAP.ReadFrom`, `wireEAPToPacket` and `handleResponderEAP`, and says the test builds the identical shape with `DecodePacket` because `engine` imports `eap` |
| 4 | ISSUE | The same constraint stood in three full copies, and the longest was the unanchored `qemu-run.py` docstring, mis-sited on a script that never runs the Docker lab it cited | `scripts/evidence/qemu-run.py` | Reduced to a six-line pointer at the rule point id. The rule and the doc keep the text |
| 5 | ISSUE | The two scenario edits carried no discrimination evidence, which is the standard the spec exists to apply | `04-eap-tls/check.py`, `06-eap-tls13/check.py` | Each now records the 2026-08-15 key-flip measurement and names `installChildSA` as the mutation site |
| 6 | ISSUE | `docs/functional-tests.md` claimed the IPsec dataplane checks "gate on XFRM availability" | `docs/functional-tests.md`, IPsec interop row | Corrected: `wait_xfrm_sa` and `assert_esp_accepted` raise `AssertionError`. The brittle scenario enumeration was dropped |
| 8 | ISSUE | The directive's opening sentence was unconditional over "a dataplane counter" while its own second paragraph named an nftables counter the directive does not reach, so the MUST forbade a probe the same point declared valid | the rule point, rendered into `ai/rules/platform-linux.md` | The opening sentence is scoped to "a counter sitting behind state written for a remote peer", and the carve-out is stated as MAY rather than as a contradiction. Round 2 |
| 9 | NOTE-acted | The directive rendered under `### 3. Graceful skip when capabilities are missing`, a heading about `t.Skip`, so four paragraphs of counter doctrine were misfiled and unfindable | `ai/rules/points/platform-linux/how-to-write-a-qemu-integration-test/manifest.md` | New heading point `### 4. Dataplane counters need a real remote peer`; the Makefile section renumbered to `### 5`. The docs heading and the `qemu-run.py` pointer now name that section, so all three homes agree on one searchable string. Round 2 |
| 7 | ISSUE | The replacement for finding 6 introduced its OWN false claim: "the `responder-` scenarios are the ones where strongSwan starts the exchange". `18-cookie-challenge` and `23-esp-form-change` both say "strongSwan is the INITIATOR" in their `swanctl.conf` and carry no such prefix | `docs/functional-tests.md`, IPsec interop row | Replaced, then replaced again. See finding 10 |
| 10 | ISSUE | The SECOND replacement was false too: "each scenario's `swanctl.conf` states which side starts the exchange". Measured over all 16 directories, seven state no role at all (`01`, `02`, `03`, `04`, `06`, `10`, `24`); only nine carry an INITIATOR/RESPONDER comment, a `start_action` or an `auto=` | `docs/functional-tests.md`, IPsec interop row | Dropped rather than replaced a third time, then given a checkable form. See finding 11 |
| 11 | NOTE-acted | "the scenario directory is where that is recorded" named no file and no key, so a reader still had to hunt | `docs/functional-tests.md`, IPsec interop row | The row names `ze.conf`'s `connection-type` leaf. Measured over all 16 scenarios: every one carries `connection-type initiate` or `connection-type respond`, eight each way, and the split agrees with the strongSwan side. The claim is now one grep from being checked. Round 3 |
| 12 | NOTE-acted | The `### 4` → `### 5` renumber staled two citations of `ai/rules/platform-linux.md` "step 4" in `plan/spec-bgp-netns.md`, one of which no longer quoted the heading and so read as a claim about the new section | `plan/spec-bgp-netns.md`, lines 230 and 606 | Both repointed to step 5. The renumber created them, so they close with it. Round 3 |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` | Yes | `git status --porcelain` lists it as `??`; `make ze-test-pkg PKG=./internal/component/ike/eap` compiles and runs it |
| `test/ipsec/ipsec-dpd-holds-tunnel.ci` | Yes | `make ze-ipsec-test` runs it by name: `25.6s 7/15 PASS 7 ipsec-dpd-holds-tunnel` |
| `ai/rules/points/platform-linux/how-to-write-a-qemu-integration-test/a-dataplane-counter-owes-an-egress-and-a-positive-control.md` | Yes | `python3 scripts/dev/rules_points.py render` renders it into `ai/rules/platform-linux.md`; `make ze-rules-lint` green |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The empty-`TypeData` response is refused, and the test discriminates | `GOFLAGS=-count=1 make ze-test-pkg PKG=./internal/component/ike/eap` → `ok github.com/ze-software/ze/internal/component/ike/eap 4.060s`, uncached |
| AC-3, AC-4 | The rekey slot and its DH private value are gone after the loop exits | `GOFLAGS=-count=1 make ze-test-pkg PKG=./internal/component/ike/engine -run TestRunEstablishedClearsPendingRekeyOnExit` → `ok ... 1.528s`, uncached |
| AC-5 | Scenario 04 proves ESP accepted | `make ze-ipsec-interop-test IPSEC_INTEROP_SCENARIO=04-eap-tls` → `ESP counters advanced on 0xc2c97276` on both containers, `PASS 1 scenario(s)` |
| AC-6 | Scenario 06 proves ESP accepted | same target for `06-eap-tls13` → `ESP counters advanced on 0xc7dd5b9f` on both containers, `PASS 1 scenario(s)` |
| AC-7 | The constraint is recorded where a probe author looks | `python3 scripts/dev/rules_points.py render` → `28 rules rendered`; `make ze-rules-lint` exit 0; the rendered paragraphs appear in `ai/rules/platform-linux.md` |
| AC-8 | A healthy tunnel survives more than one DPD interval | `make ze-ipsec-test` → `pass 15/15 100.0% 25.6s`, `ipsec-dpd-holds-tunnel` among them |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| EAP response with empty TypeData | `internal/component/ike/eap/eap_mschapv2_empty_response_test.go` | Yes. Read: the test calls `session.Process(response)`, not `mschapv2Method.Process`. The production path that produces the same packet shape is `PayloadEAP.ReadFrom` → `wireEAPToPacket` → `handleResponderEAP` → `sess.Process`, read at `internal/component/ike/engine/fsm.go` and `responder_eap.go` |
| Established loop exit | `internal/component/ike/engine/established_test.go` | Yes. Read: the test calls `ps.runEstablished(...)` in a goroutine and takes the return value as the happens-before edge, so the deferred clear has run before either assertion |
| EAP-TLS tunnel up | `test/ipsec-interop/scenarios/04-eap-tls/check.py` | Yes. Read: snapshots taken after `wait_xfrm_sa`, ping, then `assert_esp_accepted` for ze and for strongSwan |
| EAP-TLS 1.3 tunnel up | `test/ipsec-interop/scenarios/06-eap-tls13/check.py` | Yes. Same shape, and the run above proves it executes |
| A configured DPD holds a healthy tunnel | `test/ipsec/ipsec-dpd-holds-tunnel.ci` | Yes. Ran by name in `make ze-ipsec-test`, PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | An empty-`TypeData` MSCHAPv2 response is reachable from the wire. `PayloadEAP.ReadFrom` allocates `EAPData` only when the payload length is more than 4, and `wireEAPToPacket` fills `TypeData` only when `EAPData` is longer than one octet, so a length of exactly 5 reaches `Session.Process` as Type 26 with an empty `TypeData`. The length is peer-controlled and the peer is unauthenticated |
| A-2 | confirmed | Each revert failed ONLY the new test; every other test in each package stayed green. Same measurement as 2026-08-02 |
| A-3 | confirmed | The scenario 07 control transferred unchanged. Both new scenarios pass with the same helpers and the same shape |
| A-4 | confirmed | No other test covers either guard. The reverts prove it: had another test covered the behavior, the revert would have reddened it too |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #10 Test infrastructure changed → `docs/functional-tests.md` | The IPsec interop row's fail-closed claim was checked against `wait_xfrm_sa` and `assert_esp_accepted` in `test/ipsec-interop/lab.py`: both call `log_fail` and then `raise AssertionError` | Yes, corrected |
| #10 Test infrastructure changed → `docs/architecture/testing/qemu-integration.md` | New subsection under "Writing Integration Tests", anchored `<!-- source: ai/rules/platform-linux.md -- a-dataplane-counter-owes-an-egress-and-a-positive-control -->`. `make ze-doc-test` source-anchor stage: `checked 2082 code paths, 501 packages, all references valid` | Yes, added |
| #9 RFC behavior implemented, changed, or newly proven → no `docs/features/rfc-status.md` row moves | `grep -rn "RFC requirement:"` over every changed file returns only prose in the new test's header. `git diff` on `established_test.go` is additive plus one import, so no tagged assertion moved. `RFC7296-2.12-1` keeps its existing tags over `forgetKeys` | Yes, no row moves |
| #16 Existing doc source anchors naming a changed file | `grep -rn` over `docs/` for the changed paths returns `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` (anchored at `eap_mschapv2.go`, unchanged), `docs/architecture/ike/ipsec-11-interop-eap.md` (says scenarios 03 and 04 exist, still true), and `docs/architecture/testing/qemu-integration.md` (updated here) | Yes |
| No shared interop helper landed, so the spec's conditional Yes on #10 does not fire for a helper | `assert_esp_accepted`, `xfrm_sa_bytes_by_spi` and `docker_exec_quiet` already exist at HEAD in `test/ipsec-interop/lab.py`; the scenarios import them | Yes |

## Core Insight

A necessary assertion standing in for a sufficient one is invisible from inside
the test. `wait_xfrm_sa` on both ends is a real check that really passes, and it
keeps passing over a tunnel whose bytes no peer can decrypt. What separates the
two is not more assertions on the same side: Ze's own ESP counter also advanced
under a wrong key, because sending is not proof of acceptance. Only the PEER's
inbound counter moved differently. The general form is the question
`ai/rules/interop-and-goal-validation.md` already asks, applied to the observer
rather than to the mechanism: not "what would still be absent if the code were
removed", but "who is in a position to notice".
