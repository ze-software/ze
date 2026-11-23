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
Thomas then ordered them fixed:

| Test | Package | Fixed by |
|------|---------|----------|
| `TestSDKEventBusWholeFamilyFrames/redistribution`, `TestDisableValidationBurstRecoversActualSelection` | bgp/plugins/rib | b3345bbd87: a deadline and a 4096-entry cap per event frame (a product defect, not slowness alone) |
| `TestDoReloadRadiusRemovalLateBindFailurePreservesAccounting` | cmd/ze/hub | e155d3eeca: fixture lacked the subscriber identity since 6ee99f1e49 |
| `TestAC26APIOnlyDefaultAIGPValidatesAndCommits` | config/cli | 76e5e17c69: validation now reports every selection-feed refusal |
| four delivery-graph and dynamic-group tests | bgp/config | 76e5e17c69: they relied on a leaked registry setting that ac81c6ad1c fixed |
| `TestLocalCommandDaemonMatchesLocal` | cmd/ze/hub | ea93e7baca: plus a product fix, an unread conformance run string is refused |
| `TestADetailPageStatesWhatAnUndeclaredCommandTakes`, `TestLGReplayBlockListsTheTourAndTheTranscript` | internal/le/site | 5ec4ccadca |

Still red, each waiting on a decision of Thomas's:

| Test | Decision |
|------|----------|
| the 11 `TestConfiguredPluginConformance*` | They drive `request plugin conformance`, a daemon feature deferred on 2026-11-17. Build it now, or keep it deferred (the drafts stay red, or leave the default run with the judge's approval) |


`TestTheHomepageRFCCardSaysWhatTheOldFigureCounted` is fixed (36a009db93):
the rule recorded in bd6ba7a195 requires the public Counting history note.

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
| The refusal-advice commit gate matches message wording | its replacement (typed fix lines judged by the guards themselves) needs a spec, likely an amendment to `spec-gates-share-one-semantic-population.md` | Delete the wording gate now or after the spec; whether the retired-names sweep counts as a check of what a thing is |

Another session is building RFC 4724 restarting-speaker selection
deferral, uncommitted and under no spec; its edit of
`rfc/short/rfc4724.md` claims all 26 MUST-level requirements proven.
spec-gr-advanced notes it for review.

The two root-only key-file tests skip without root and were not run.

## Waiting on another session's tooling

The rules in bd6ba7a195 exclude, as `{not-applicable}`, a requirement that
binds only an operator's configuration or IANA. Three rows need that form, and
HEAD's `le` refuses it: the accepting code (`nonSoftwareRoleAccepted`,
`nonSoftwareOperatorRoles` in `internal/le/rfc/notapplicable_roles.go`, test
`TestNotApplicableNonSoftwareAndHeadingForms`) is another session's
uncommitted work. Once it lands:

- `rfc/short/rfc9234.md` RFC9234-6-1: replace the `{gap: ...}` (and its
  "owner ruling 2026-11-22" citation) with `{not-applicable: "Roles MUST NOT
  be configured on an eBGP session with a Complex peering relationship.";
  binds an operator; the sentence binds the operator's configuration of a
  peering relationship, which software cannot observe: whether the
  relationship between two ASes is Complex is a business arrangement no BGP
  message carries, and Section 6 itself says there are no in-band measures to
  check the correctness of the peering configuration.
  internal/component/bgp/plugins/role/config.go::parseRoleContainer accepts
  the one role the operator configures per peer, and docs/guide/bgp-role.md
  tells the operator to omit the role on a Complex peering}`; add the meta row
  `| Ze roles | BGP speaker, route server |`; the enrolment reason becomes
  `+ 2 gaps (no AS Confederation for 5-7 and 5-8) + 1 not applicable (6-1 binds
  the operator's configuration of a Complex peering relationship, which Ze
  cannot observe)`.
- `rfc/short/rfc4360.md` RFC4360-7-1 and 7-2: `{not-applicable}` binding IANA's
  type-allocation rules under Section 7 (IANA Considerations); ze allocates no
  Type value and reads only IANA-assigned codes. The enrolment reason's "Two
  are gaps" sentence changes to match.

Then `./le rfc update` and `./le rfc check`.

## For Thomas

| Question | What each answer changes |
|----------|--------------------------|
| May a subagent run `gopls check`? | The ze-work definition says yes; META-DECISIONS says subagents run no code |
| Which VPP throughput statement is true? | The performance page says throughput backs the guide's numbers; 770170950a had said this lab does not benchmark it |
| The two update-report summaries exceed the 108-character budget | 2026-10-12 (about 165) and 2026-10-25 (about 152) are in your wording, for you to shorten |
| Nine other statements disagree between website pages | The earlier text is restored in each |
