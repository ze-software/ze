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
