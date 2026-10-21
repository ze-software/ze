# Handover: spec-bgp-update-propagation-rfc-defects, AC-13 interop and closure

Written 2026-10-21 on the Mac by a subagent of session 0b4a8e87, which stopped at
budget. Spec: `plan/immediate/spec-bgp-update-propagation-rfc-defects.md`
(status in-progress, phase 4/4). Claim released at stop; re-claim with
`./le spec claim spec plan/immediate/spec-bgp-update-propagation-rfc-defects.md`.

## Done this session

| Item | Evidence |
|------|----------|
| AC-5: RFC7999-3.1-2 weak to enforced | commit b65e2c5a21: `TestAnnounceUnicastWithBlackholeCommunityMeetsTheSameGate` tagged, new `TestAnnounceUnicastWithBlackholeCommunityNarrowsToAgreedPeers`; a go test overlay changing `sel, err = agreedSelector(...)` to `_, err =` in `handleAnnounceUnicast` turns only the new unit red; independent auditor re-judged enforced with `audit-stamp mode rejudge`. Every AC-5 ID is now `enforced` in `rfc/audit/*.json` |
| Spec gained the Deliverables and Security Review checklists | ze-close requires both; the design phase left them out |
| D1 interop green | `bgp-attr-flags-low-nibble-gobgp` PASS (A-2 confirmed) |
| D2 checker assertion corrected | `internal/le/interoplab/bgp/checkers.go`: GoBGP 3.31.0 prints `{LargeCommunity: [ 65000:1:1, 65000:3:3, 65000:2:2]}`. Ze's wire was right (each value once). Not yet re-run |
| Review of the landed D1 to D5 producers (read at HEAD, not recorded as a gate) | no BLOCKER or ISSUE found in `normalizeTransmittedUpdate`, `compactLargeCommunities`, `OTCEgressFilter`, `blackholeReadvertiseRefused`, `configTx`, `nlritype.RetainUpdate`, `typedNLRIEdit`, the Q-2 refusals. NOTE: the D5 gate reads the received COMMUNITIES only, so an egress policy that adds 65535:666 towards an unagreed eBGP peer is not refused. NOTE: the `PeerFilterInfo` source fills do not set `IsIBGP` (nothing reads it there today) |

## How the interop was run

On macOS the BGP lab runs in the Ze-kernel QEMU guest. From a `git archive HEAD`
export (never the shared tree, which carries other sessions' edits):

```
./le test qemu docker-lab lab "test integration interop" env "INTEROP_SCENARIO=<name>" timeout 3600s
```

About 13 minutes a run, mostly guest setup. Only one QEMU guest fits at a time
(host port 2222): check `pgrep -fl 'qemu-system|bin/le verify'` first. The red
tree is an export with all five fixes reverted, each marked `RED-TREE`:
`attrs[pos] &^= attrFlagsUnusedMask` removed (D1), the LARGE_COMMUNITY branch
behind `if false &&` (D2), `ValidateNLRI` failure answering `Keep` in
`mup/rfc7606.go::recognizerFor` (D3), `|| srcRole == roleRS` removed in
`OTCEgressFilter` (D4), `blackholeReadvertiseRefused` returning false first
(D5). The local scratch copies are not portable; rebuild them from this list.

## Done 2026-10-21, second session (subagent of 0b4a8e87)

| Item | Evidence |
|------|----------|
| D3 wire defect fixed | 27b0796cea: a malformed MUP NLRI is withdrawn by the session's held well-formed generation of its key (`reactor/session_validation_nlritype_held.go`, `nlritype.RegisterBodyRecognizer`), nothing relayed when none is held; covers received MP_UNREACH too. Decision with RFC 7606 Section 2, RFC 4271 Section 9.2, FRR and BIRD quotes in the spec's Design Insights. Units and the `.ci` red with the substitution off, green on |
| D4 and D5 cause | bba92170c4: scenario config. Neither `ze.conf` attached `bgp-adj-rib-in`, so bgp-rs had nothing to replay to FRR (D5 log). D4's `routes-in 0` is not explained by its log (the Ze log tail held only SSH lines); the next green run decides it |
| Review note: IsIBGP | 88ef88edda: every source `PeerFilterInfo` fill carries `IsIBGP`; the RFC 8195 relation tag reads it (red against HEAD for an RFC 7705 migration session) |
| Review note: RFC 7999 policy-added BLACKHOLE | a25881cc5e: journal row only (`guard-added-to-one-half-of-a-pair.md`); the fix needs the destination at the COMMUNITY AttrModHandler |
| Audit | 399ee383e1: independent rejudge of MUP 3.1.3.1-6, -7, -10 (enforced); a second rejudge is owed after 897ac54217 renamed a field the tagged unit reads |
| Closure review round 1 | 1 BLOCKER, 2 ISSUE: held generations could grow an UPDATE past 65535; 2 allocs per refresh; hold lost on a GR reconnect. All three fixed in 897ac54217 (withdraw bodies sized to the limit, visitors bound once, hold on the Peer with RFC 4724 lifetime). Round 2 runs next |
| Review rounds 2 to 3 | round 2: 2 ISSUE (same-UPDATE announce and withdraw, RFC 4271 Section 4.3; no wiring test), fixed in 2b6976ddd2; round 3: 0 BLOCKER, 0 ISSUE. Artifact recorded, `./le spec review check` OK. Round 4 runs over 0bd7a50667 and 05ea9ac55d |
| D4 cause | 0bd7a50667: the injector's KEEPALIVE expects closed its session when Ze sent the RS-Client its static route (`le test peer` mismatch); `option=linger`. Ze has no defect |
| D2 cannot go red at GoBGP | GoBGP removes LARGE_COMMUNITY repeats on receipt (RFC 8092 Section 3), so the red run passed. 05ea9ac55d adds the native speaker with oracle `no-repeated-large-community` |
| Red run at 897ac54217 trees | D1 red (assertion 3, route never at GoBGP), D2 pass (the reason above), D3 red (assertion 4), D4 red (assertion 4, its own), D5 red (assertion 5, its own) |
| Review rounds 4 and 5 | round 4: 1 ISSUE (the D2 speaker oracle passed with no LARGE_COMMUNITY); fixed in 5af84be2ea (counts `large-community-updates` and `large-community-distinct`, checker requires 1 and 3). Round 5: 0 BLOCKER, 0 ISSUE. The review artifact must be re-recorded with `rounds 5` over the reviewed files plus `internal/le/interoplab/bgp/{speaker.go,check_rfc_predicate.go,checkers.go,speaker_test.go,register_speaker.go,names.go}` and the D2 scenario files |
| Final runs NOT DONE | the green and red runs at 5af84be2ea died before any scenario: the Mac root disk was full (391M free; journal `full-disk-false-red.md`, 2026-10-21). Trees `scratch/iopg4` (green) and `scratch/iopr4` (red, five `RED-TREE` marks) are still on disk with their caches removed; rebuild both from HEAD if in doubt |

## Resume here (next machine or session)

1. Reclaim disk (owner), then run green and red over the five scenarios, one boot each: build a `git archive HEAD` export, move every other directory out of `test/interop/scenarios/`, run `./le test qemu docker-lab lab "test integration interop" timeout 3600s` from it with no selector. Wait on `pgrep -f "[q]emu-system-aarch64"` and port 2222 (`lsof -nP -iTCP:2222 -sTCP:LISTEN`); a bare `qemu-system` pattern matches the waiting shell itself.
2. Expect green 5/5 (the last green, at 897ac54217 trees, passed D1, D2, D3, D5; D4 failed for the injector cause fixed in 0bd7a50667). Expect each red at its own assertion; D2's red is now the speaker oracle.
3. Record green and red lines in the spec's Goal Validation (rows say INTEROP-Dn as placeholders), re-record the review artifact (`rounds 5`), run `./le verify worktree` once, then ze-close commit A (spec, the five citing specs repointed to the bare stem, `plan/journal/` rows) and commit B (`remove` the spec), `./le spec release`.
| Interop trees (older) | `scratch/iopg2` (HEAD 897ac54217, only the five scenarios) and `scratch/iopr2` (five `RED-TREE` mutations as listed below) under this session's scratch; one boot runs all five with no selector. Logs `scratch/interop-green2.log`, `scratch/interop-red2.log` |
| R-1 benchmark (indicative, loaded machine) | `BenchmarkWriteUpdateBodyEgressWalk` 9 allocs/op in every case both before D1 (020457d0f7^) and after: the walk adds none, the 9 are the writer's own. ns/op noisy (a foreign verify ran); the owner's quiet run is still owed |

## Outstanding, in order

| # | Item | What is known |
|---|------|---------------|
| 1 | D3 scenario: Ze relays the malformed NLRI bytes in its withdrawal | GoBGP 3.31.0 panics in `MUPType1SessionTransformedRoute.DecodeFromBytes` (index out of range, NLRI length 22) on Ze's relayed MP_UNREACH_NLRI. `typedNLRIEdit` moves the received K' bytes into the synthesized MP_UNREACH_NLRI and the forward rails relay them as received. Ze sends a NLRI its own `ValidateNLRI` calls malformed. Fix direction to decide at the producer: withdraw with the bytes of the generation Ze advertised (the Adj-RIB-In entry of K's key), or relay only a key-reconstructed form. Ze also logs "owner withdrawal sent without election: the RIB recovery owner does not receive the source's UPDATEs" in this lab. The GoBGP crash itself is GoBGP's defect; Ze emitting malformed bytes is Ze's (M4) |
| 2 | D4 scenario: `routes-in 0` | The injector (Ze's role `rs` session) sends its 47-octet UPDATE, Ze sends its own control 10.77.4.0/24 back, and Ze's `show bgp rib status` never counts it. Ze logs that bgp-rib and bgp-adj-rib-in are attached to no peer. Either the scenario config lacks the `attach process` the daemon now requires, or the wait reads the wrong counter. Read `ze.conf` against the current attach rules first |
| 3 | D5 scenario: FRR never gets the control route | Ze logs "forward matched no target" when BIRD's routes arrive before FRR is up, then "peer-up replay gave up waiting for adj-rib-in to reach the cut ... ingested=0" when FRR comes up. Same attach warning as D4. Likely the same config cause as item 2; if the config is right, it is a bgp-rs replay defect (sibling spec `spec-bgp-rs-replacement-on-withdrawal` works the same rail) |
| 4 | D2 green re-run with the corrected checker | expected PASS |
| 5 | Red runs, all five, from the red tree | each must FAIL at its own assertion (AC-13). Record red and green log lines in the spec |
| 6 | Closure | `/ze-close`: append `plan/TEMPLATE-CLOSURE.md` sections, Goal Validation, Assumptions A-1..A-7 (A-2, A-3 confirmed above; A-4 confirmed: GoBGP negotiated ipv4-mup and held the T1ST), `/ze-review` inline with `./le spec review record`, `./le verify worktree` once, commit A then commit B |
| 7 | R-1 benchmark | `BenchmarkWriteUpdateBodyEgressWalk` exists; the owner runs it on the Mac per the spec |

## Logs (local to the Mac that ran them, summarised above)

`tmp/session/2026-10-20-0b4a8e87-9c06-4f85-90a2-0445cb843906/scratch/interop-green-<scenario>.log`.
