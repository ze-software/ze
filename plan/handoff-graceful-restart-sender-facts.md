# Handoff: graceful restart, the sender facts, and three findings

**Spec:** `spec-test-peer-open-mirrors-five-more-sender-facts`, closed on 2026-09-08, so it is no longer on disk. Its implementation is in commits `1e9951ab4`, `d2ce72db0` and `693eb7553`, and its closure record is the commit that removed it
**Branch:** main
**Goal at the September handoff:** The test peer owns the sender facts it used to mirror from Ze. The investigation recorded GR advertisement/retention defects and tests that stayed green with GR dispatch broken. Those are dated observations, not a current runtime verdict.

## Historical evidence and current ownership

The findings and measurements below record 2026-09-08 and the PATHS-LIMIT
working-tree snapshot of 2026-09-09. The sender-fact spec is closed, as the header
states; its closure is no longer in progress. The recorded uncommitted state
does not describe the current checkout. Current PATHS-LIMIT requirements belong
to `plan/immediate/spec-add-path-limit-send-receive.md`.

Source reconciliation on 2026-09-19 found that `parseGRCapValue` in
`internal/component/bgp/plugins/gr/gr.go` still emits the two-octet restart
value without family tuples. RFC 4724 permits that form for a receiving-only
helper (`rfc/short/rfc4724.md`, Encoding Rules), so the two-octet value alone
does not establish a defect. The required disposition is whether Ze promises
only helper support or preserved forwarding as a restarting speaker, with proof
of the chosen behaviour before adding any tuple. The original unconditional retention-race
explanation is superseded: `onPeerStateChange` in
`internal/component/bgp/server/events.go` sorts reverse dependency tiers and
waits for each delivery result; `grPlugin.dispatchCommand` synchronously calls
`DispatchCommandArgs`. The GR callback can therefore establish retention before
the RIB's down handler on successful ordered delivery. This source evidence
does not prove retention on every delivery path or discharge the recorded
discrimination gap. `plan/pre-release/spec-release-audit-2-bgp-protocol.md`
owns the base GR semantics and end-to-end evidence disposition. Any product
defect it confirms needs a separate fix owner; the optional
`plan/spec-gr-advanced.md` extensions supply no baseline readiness evidence.

### Findings as recorded on 2026-09-08 and 2026-09-09

| # | Finding | Producer | Row |
|---|---|---|---|
| 1 | Ze advertises Graceful Restart for **no address family** | `parseGRCapValue` (`internal/component/bgp/plugins/gr/gr.go`) returns `fmt.Sprintf("%04x", restartTime&0x0FFF)` and nothing else: four flag bits, twelve-bit restart time, two octets. RFC 4724 Section 3 puts zero or more `<AFI, SAFI, Flags>` tuples after that pair. Wire confirms: `40 02 00 78`. The sibling is correct — `parseLLGRCapValue` (`gr_llgr.go`) takes a `families` argument and emits tuples with the F-bit | `plan/journal/unwired-feature.md`, 2026-09-08 |
| 2 | **RFC 4724 retention does not happen** | `RIBManager.handleState` (`internal/component/bgp/plugins/rib/rib.go`) releases the peer's Adj-RIB-In on peer-down unless `retainedPeers` is ALREADY set, and the only writer is `rib_commands.go` acting on the `bgp-gr` plugin's `retain-routes`, which reacts to the same event from another process. The retention loses the race by construction. Measured twice: a gr-state row naming the peer's restart time beside `routes-in: 0` | `plan/journal/unwired-feature.md` |
| 3 | **PATHS-LIMIT had no effective enforcement. Addressed in the working tree on 2026-09-09** | Historical evidence: `CommitService.enforcePathsLimit` (`internal/component/bgp/rib/commit.go`) could never drop a path. `Transaction.nlriIndex` (`internal/component/bgp/transaction/commit_manager.go`) keyed by AFI+SAFI+`NLRI.WriteTo`, which excluded the Path ID, so a second path replaced the first. `update text` reached `AnnounceNLRIBatch`, not `CommitService`. The working tree moves enforcement to session writers and retains transaction path IDs. Package tests and live wire checks passed; changes are deliberately uncommitted for operator review | `plan/journal/unwired-feature.md` |

Finding 1 explains why 2 was invisible: with no families on the wire, nothing downstream ever asked for retention, so the dead path had no witness.

## Why the tests did not catch it in the recorded run

Measured in the September investigation with a control:

| Run | GR reachable | Result |
|---|---|---|
| `gr-mark-stale.ci` verbatim | yes | PASS 7.2s |
| same, GR dispatch broken | no | **PASS 9.3s** |
| `llgr-transition.ci` verbatim | yes | PASS 7.1s |
| same, GR dispatch broken | no | **PASS 7.0s** |
| control, one NLRI octet changed | no | FAIL on that assertion |

The control proves the harness discriminates, so the two passes are real. Connection 2's re-announcement, which those tests read as evidence of Graceful Restart, comes from `RIBManager.handleState` replaying `ribOut`; `ribOut` is deleted only in the withdraw path, never on peer-down. 32 `.ci` put capability 64 on the wire and 5 exercise LLGR; on this evidence none of them tests Graceful Restart.

Breaking `handleStateEvent`, the JSON dispatch, moved no verdict at all. **That path is dead code.**

## Status recorded at handoff

**Done, committed:**
- `1e9951ab4` — the test peer owns the Hold Time and capabilities 64, 71, 75, 76; unit tests; `docs/architecture/testing/ci-format.md` updated, since it named only 9, 65, 69 and 73 as owned facts.
- `d2ce72db0` — four `.ci` that fail when a sender fact is mirrored, the observer fixture, and the `cmd_peer.go` merge fix.
- `693eb7553` — the spec record and its journal rows.
- `1e3ca0f89` — finding 1's journal row.
- `1d347f171` — the spec taken from `skeleton` to `design` with the measurement in it.

**Closure:** completed on 2026-09-08, as recorded in the header.

**Remaining at handoff, outside the sender-fact spec:**
- Findings 1 and 2 each need a fix. They are journal rows, and a row is a step toward a fix, never a substitute (`ai/rules/principles.md`).
- Finding 3 is addressed in the working tree. Session writers enforce the negotiated remote limit across batches, including route-server fast-path forwarding. Named transactions retain path IDs and order them deterministically. `test/plugin/paths-limit-live.ci` passed in three load repetitions. Changes remain uncommitted at the operator's request.
- The 32 GR `.ci` and 5 LLGR `.ci` still assert nothing about Graceful Restart. The four new `.ci` are fenced by a recorded break; the old ones are not.

**Deliberately not done, and why:**
- Finding 1 is the shipped daemon's own OPEN, and the spec that found it is about the test peer. Absorbing it would have cost that spec its focus and made a harness change wire-visible. Recorded as a Known Limitation and a rejected alternative inside the spec as well as in the journal.
- Findings 2 and 3 were met while implementing two acceptance criteria; both criteria were STOPPED ON rather than weakened, which is why the spec's AC table says "enforcement unreachable" rather than claiming the criterion.

## Design questions retained from the handoff

Not "which families does Ze configure". A tuple in the Graceful Restart capability asserts forwarding state **actually preserved across a restart**, so what Ze's FIB does on restart decides which families may honestly appear. Answer that before writing the encoder, or finding 1 gets repaired into a different false claim.

The retained requirement is that the RIB's peer-down release observes the GR
retention decision before discarding routes. The September proposal required a
pre-dispatch change in `bgp-rib`; current ordered delivery provides another
mechanism, so that implementation prescription is withdrawn. Discriminating
end-to-end retention evidence is still required before a readiness claim.

## Files handled in the original session

- `internal/component/bgp/plugins/gr/gr.go` — `parseGRCapValue` builds the code-64 payload; `handleStructuredState` is the live dispatch and `handleStateEvent` is dead.
- `internal/component/bgp/plugins/gr/gr_llgr.go` — `parseLLGRCapValue`, the correct sibling that emits family tuples.
- `internal/component/bgp/plugins/rib/rib.go` — `handleState` does the peer-up replay from `ribOut` and the peer-down Adj-RIB-In release; `retainedPeers` guards only the latter.
- `internal/test/peer/open.go` — `ownedCapabilities` and `reconcileParams`: the mirror, and the four typed `option=open:` values that now replace it.
- `docs/architecture/testing/ci-format.md`, "Capability Control" — already updated for the new owned facts.

## Original verification procedure

The handoff prescribed the following command and forced-break experiment.
They are historical instructions, not a current result; a resuming investigation
must select the current entry point and preserve the same discrimination:

    ./le job run label grcheck command ./bin/ze-test bgp plugin --pattern gr-peer -v

and confirm the recorded break still discriminates by making `handleStructuredState` return immediately: the four new `.ci` must go red while `gr-mark-stale` stays green. That contrast is the whole evidence, and it is the thing to re-establish before trusting any later change to this area.
