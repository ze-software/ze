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

## Found and not fixed

Each needs a design of its own; none has a journal row yet.

| Item | Where |
|------|-------|
| A reconnect that arrives while the Restart-Time-expiry LLGR entry is still being applied stops the entry: the routes stay at stale level 1 without LLGR_STALE, and NO_LLGR routes stay retained, until End-of-RIB, LLST expiry or the next session-down (audit: RFC9494-4.2-4 and 4.2-5 weak). `TestGRReconnectRevokesHeldLLGREntryTail` pins the revocation, so finishing the entry on reconnect changes a tested behaviour | `llgrFamilyActive` in gr_state.go |
| No tagged test of RFC4724-4.2-11 covers more than one retained connection; tagging `TestRFC9494ConsecutiveRestartEORDeletesEveryConnectionsStaleRoutes` with a proof record would close it | gr tests |
| RFC 9494 4.2 counts a family as synchronized when the Selection_Deferral_Timer expires; Ze has no such timer | gr plugin; rfc4724 gaps 4.1-5, 4.1-8 |
| A commit-driven reload is not cancelled at daemon stop | hub, `commitReloads.close()` |
| Labeled unicast (SAFI 4) is dumped as RIB_GENERIC with the label stripped | rib storage side-data, mrt |
| The break-glass recovery grant is still revoked by any user's credential change | aaa, `rfc8907_*` tests |
| `checkKeyFilePermissions` refuses every symlinked key file under root | breaks secret mounts made of symlinks |
| Plaintext remote MCP with auth disables MCP with a warning; gNMI refuses to boot | boot guard |
| The refusal-advice commit gate matches message wording | `CheckRefusalAdvice` in `internal/le/commit/refusal_advice.go` |
| Negatives of RFC4724-3-5 and RFC4360-6-1 miss their clause's condition | need selection deferral, a looping-policy refusal |

## For Thomas

| Question | What each answer changes |
|----------|--------------------------|
| Is "owner ruling 2026-11-22" the right citation for RFC9234-6-1 returning to a gap? | The coverage ratchet (`ownerRuledMove`) requires a ruling; another wording changes `rfc/short/rfc9234.md` |
| Did you rule the "document-author" not-applicable form (RFC7606-8-1, RFC5492-4-3)? | It is recorded in the spec and the gates page, not in `ai/META-DECISIONS.md` |
| May a subagent run `gopls check`? | The ze-work definition says yes; META-DECISIONS says subagents run no code |
| Which VPP throughput statement is true? | The performance page says throughput backs the guide's numbers; 770170950a had said this lab does not benchmark it |
| The two update-report summaries exceed the 108-character budget | 2026-10-12 (about 165) and 2026-10-25 (about 152) are in your wording, for you to shorten |
| Nine other statements disagree between website pages | The earlier text is restored in each |
