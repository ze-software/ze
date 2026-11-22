# Handover: fixes from the review of the 2026-11-22 unpushed commits

Thomas asked for a review of the 28 unpushed commits (79c6a40ecb..808ee06cfb),
then ordered the findings fixed. No spec was opened: the fixes were ordered
directly. All eight fixes are committed. Nothing is pushed.

## Landed

| Fix | Commit |
|-----|--------|
| LLGR stale routes across a second restart (RFC 9494 4.2) | 26d70dfa0d |
| Config transaction: rollback ends on shutdown, commit has its own bound | 47dc081099 |
| Website: assertions 770170950a changed are restored | 2c7795a098 |
| MCP: tool scope decided on the command line that runs | 4de9d23137 |
| Rules, skills and gates after the rules split | dc9a814656 |
| MRT dump subtype follows what the route is (RFC 8050) | 381e927417 |
| NTP and DDoS detect answer a removal bye within its grace | c34f34b877 |
| NOTIFICATION size capped by the peer's capability (RFC 8654), three RFC rows | 562bc0139d |
| gNMI exposure rule, IPv6 listener addresses, per-user web sessions | d794af6e2a |

Each commit body names the tests that passed and what it did not do.

## Still owed

- `./le rfc check` is red tree-wide (420 violations, 221 stale audit verdicts
  when it ran), nearly all from other work. The 21 verdicts these commits made
  stale were re-judged by independent audit agents and stamped: RFC9494-4.2-1
  to 4.2-5, RFC4724-4-2, 4.2-3, 4.2-4, 4.2-7, 4.2-11, RFC8050-4.1-1, 4.2-1,
  RFC6396-4.2-2, RFC8955-7.1-2, RFC4271-4.2-1, 4.4-1, 5-3, 5-4, 6.2-9, 6.2-10,
  6.3-8. Three came back weak: RFC9494-4.2-4, RFC9494-4.2-5 and RFC4724-4.2-11
  (see "Found and not fixed").
- `./le site build` and `./le site check` to regenerate `../gh-pages` for
  2c7795a098.
- `./le doc check all` for the moved source anchors.
- `./le test functional plugin --pattern mgmt-guard` for d794af6e2a.
- Lint ran on the changed mcp, transaction, gr, mrt, network, gnmi, ntp and
  ddos packages. The two findings these commits introduced in editable code
  are fixed in a follow-up commit. Three sit in RFC-tagged mrt tests and were
  left, because editing a tagged unit stales its proof record:
  `deprecatedComment` and `appendCombine` in `rfc6396_invariants_test.go`,
  `tparallel` in `rfc8050_dump_addpath_test.go`. The other findings predate
  this work. Lint was not run on reactor, rib, config, web, aaa, authz, ssh,
  `cmd/ze/hub` or `internal/le`.

## Reds met, not from this work

Each already failed in another session's log before this work started.

| Test | Package | Cause as diagnosed |
|------|---------|--------------------|
| `TestSDKEventBusWholeFamilyFrames/redistribution` | bgp/plugins/rib | one 30 s `sdkEventEmitTimeout` over a 170k-entry hand-off under the race detector |
| `TestDisableValidationBurstRecoversActualSelection` | bgp/plugins/rib | its own 60 s context over 180k selections under the race detector |
| `TestConfiguredPluginConformanceRejectedReloadRetainsAcceptedTarget` | cmd/ze/hub | "unknown command"; not investigated |
| `TestDoReloadRadiusRemovalLateBindFailurePreservesAccounting` | cmd/ze/hub | "accounting record did not arrive"; not investigated |
| `TestTheHomepageRFCCardSaysWhatTheOldFigureCounted`, `TestADetailPageStatesWhatAnUndeclaredCommandTakes` | internal/le/site | named as reds by 770170950a |

## Open findings, second round

Thomas ordered the ten open findings closed. Seven are closed in code:

| Finding | Commit |
|---------|--------|
| A reconnect during the LLGR entry stopped it, leaving routes without LLGR_STALE | 21587224da |
| RFC4724-4.2-11 had no tagged test with more than one retained connection | 21587224da |
| Labeled unicast dumped to MRT without its label stack | fe57accc2f |
| A commit-driven reload was not cancelled at daemon stop | e0fa3307fc |
| The break-glass grant was revoked by any user's credential change | e0fa3307fc |
| Every symlinked MCP key file was refused | e0fa3307fc |
| Plaintext remote MCP only warned where gNMI refused to boot | e0fa3307fc |
| RFC4724-3-5's negative missed its condition (new tagged pair) | e5302c6d4b |

Found and fixed on the way: no hook refused `git cherry-pick`
(bf127d164a); `ze config validate` printed findings without their fix
(e0fa3307fc).

Still open, each waiting on a decision of Thomas's:

| Finding | Where it is homed | The decision |
|---------|-------------------|--------------|
| RFC 9494 4.2 synchronization when the Selection_Deferral_Timer expires (helper side) | `plan/post-release/features/spec-gr-advanced.md` Q-1; journal `plan/journal/state-released-only-by-a-message-the-peer-may-never-send.md` | Which local bound: FRR and BIRD differ |
| RFC4360-6-1: "The Extended Community attribute MUST NOT be used to modify the BGP best path selection algorithm in a way that leads to forwarding loops." | the row stays `{partial}` | Whether it binds the operator who writes policy |
| The refusal-advice commit gate matches message wording | its replacement (typed fix lines judged by the guards themselves) needs a spec, likely an amendment to `spec-gates-share-one-semantic-population.md` | Delete the wording gate now or after the spec; whether the retired-names sweep counts as a check of what a thing is |

Another session is building RFC 4724 restarting-speaker selection
deferral, uncommitted and under no spec; its edit of
`rfc/short/rfc4724.md` claims all 26 MUST-level requirements proven.
spec-gr-advanced notes it for review.

The two root-only key-file tests skip without root and were not run.

## For Thomas

| Question | What each answer changes |
|----------|--------------------------|
| Is "owner ruling 2026-11-22" the right citation for RFC9234-6-1 returning to a gap? | The coverage ratchet (`ownerRuledMove`) requires a ruling; another wording changes `rfc/short/rfc9234.md` |
| Did you rule the "document-author" not-applicable form (RFC7606-8-1, RFC5492-4-3)? | It is recorded in the spec and the gates page, not in `ai/META-DECISIONS.md` |
| May a subagent run `gopls check`? | The ze-work definition says yes; META-DECISIONS says subagents run no code |
| Which VPP throughput statement is true? | The performance page says throughput backs the guide's numbers; 770170950a had said this lab does not benchmark it |
| The two update-report summaries exceed the 108-character budget | 2026-10-12 (about 165) and 2026-10-25 (about 152) are in your wording, for you to shorten |
| Nine other statements disagree between website pages | The earlier text is restored in each |
