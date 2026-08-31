# Handover: the origin/main divergence replay, 2026-08-31

## What happened

`git push` was refused non-fast-forward. Local main was 66 ahead and 12 behind
`origin/main`.

The 12 origin commits were authored `thomas.mangin@exa.net.uk` on 2026-08-30
between 23:20 and 23:46. Every local commit is authored `thomas@mangin.com` from
`~/.gitconfig`, which carries no conditional include. This checkout FETCHED those
12 at `refs/remotes/origin/main@{2026-08-31 00:56:48}: fetch: fast-forward` and
never integrated them. Every reflog entry after that fetch is `commit:`: no reset,
no rebase, no merge, and nothing dropped.

So local kept building on the fork point `26f112aee` for the whole of 2026-08-31
while the same work already sat on origin. Both sides walked the same RFCs for
`spec-rfcgate-6` phase 3 and produced different `rfc/extraction/*.json`, different
`rfc/short/` text and different test files. Origin's artifacts are signed
2026-08-30 by "ze-implement walk agent"; this tree's are signed 2026-08-31 by
"ze-work agent".

Nothing was lost. Main only ever moved forward.

## What was done

The 12 commits were replayed by hand, one at a time, MERGING each file rather
than applying patches: local content kept, origin's additions folded in. Thirty
commits landed. Origin's SHAs are NOT ancestors of HEAD, by construction.

| Origin commit | Landed as |
|---|---|
| `b504edd79` | `c0d4b3612` |
| `17926c82f` | `d892a6666` |
| `8dfa4b57d` | `093dfc8e5` |
| `f1245bbf4` + `3d6112476` | `592659c70` |
| `0e5491e81`, `0e07e7296`, `091e29ffc`, `9471877e8`, `aac81227f` | `30c94da4d` |
| `4f58d46a1` (BGP half only) | `0d031122e` |
| `20022d817` | NOT REPLAYED |
| `4f58d46a1` (RADIUS, command, l2tp, fixture halves) | NOT REPLAYED |

## BLOCKING: a force push would lose work today

`main...origin/main` reads `ahead 98, behind 12`. Fourteen files tracked on
`origin/main` are not tracked in HEAD. Five do not exist on disk at all:

| State | Path |
|---|---|
| ABSENT | `internal/component/config/cli/cmd_ls.go` |
| ABSENT | `internal/component/l2tp/plugins/authradius/rfc5176_walk_test.go` (660 lines) |
| ABSENT | `internal/component/radius/rfc2865_walk_test.go` (643 lines) |
| ABSENT | `internal/test/fixture/ui_fixture_cli_announce.go` (362 lines) |
| ABSENT | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| ABSENT | `test/parse/cli-data-ls-show.ci` |
| on disk, uncommitted | `plan/spec-announce-grammar-stated-and-enforced.md` |
| on disk, uncommitted | `plan/spec-eap-notification-and-nak.md` |
| on disk, uncommitted | `plan/spec-fixit-peer-pending-sync-settles-too-early.md` |
| on disk, uncommitted | `plan/spec-ipsec-transport-nat-selector-substitution.md` |
| on disk, uncommitted | `plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` |
| on disk, uncommitted | `rfc/extraction/rfc7607.json` |
| on disk, uncommitted | `rfc/requirements/rfc7607.md` |
| on disk, uncommitted | `rfc/short/rfc7607.md` |

`git diff HEAD origin/main` also carries 11231 lines origin holds that HEAD does
not, across 835 files. Most are local's own newer authoring of the same subject
and are not losses, but they were never audited line by line and they are not
proven to be safe.

Recover them before any force push:

```
git ls-tree -r --name-only origin/main | sort > /tmp/orig
git ls-files | sort > /tmp/local
comm -23 /tmp/orig /tmp/local
```

## Owner rulings recorded 2026-08-31

Each is now a directive in
`ai/rules/points/rfc-compliance/directives/count-conformance-on-the-whole-stack.md`
(commits `fc868a86d`, `014a033ef`, `8658100ed`).

1. **Conformance counts the behavior the WHOLE STACK produces.** It does not
   matter who enforces it; Ze is TCP conformant thanks to Linux too. A
   requirement met by configuring the kernel correctly is MET, and delegation to
   XFRM implements an obligation rather than exempting Ze from it. Such a
   requirement still owes a test, asserted at the boundary Ze owns.
2. **A gap is an ISSUE; an exclusion is a DECISION.** A `{gap}` says Ze owes the
   behavior and does not produce it. An `excluded` site says the obligation never
   bound Ze, and the reason names which decision put it out of reach.
3. **Conformance is not owed for an OPTIONAL feature outside Ze's scope.** The
   new `excluded-kind: feature-out-of-scope` states it. The absent FEATURE is
   still recorded, as an implementation gap a later scope decision can revisit,
   and never as a conformance gap.
4. **`binds-another-role` is PRESUMED WRONG.** Ze rarely implements one side of a
   protocol. Writing the kind owes a named role, evidence Ze never acts as it,
   and the producer that would act as it if Ze did.
5. **The gomu operator set is an accepted limitation.** Do not extend gomu; fix
   `discriminate` to refuse instead.

## OPEN: decisions only Thomas can take

| # | Decision |
|---|---|
| D-1 | `RFC8671-5.2-1` (pre-policy Adj-RIB-Out marking) is declared in `rfc/short/rfc8671.md` with no source site and no tags. The gate prescribes retiring the id. That deletes a MUST row from the published ledger. |
| D-2 | Two specs claim the same work: `plan/spec-ike-virtual-ip-assignment.md` (new, narrow) and `plan/spec-ipsec-remote-access.md` (design, 983 lines, phases A-G, AC-7..AC-10 and AC-16..AC-28). One must supersede or fold into the other. |
| D-3 | The 188 `binds-another-role` sites across 18 RFCs need reclassifying under ruling 4. Concentrated in `rfc4364` (39), `rfc2869` (33), `rfc2865` (32), `rfc4303` (29), `rfc4761` (16), `rfc1035` (11), `rfc3032` (10). Belongs to the `rfcgate-6` spec, not to a replay session. |

## OPEN: work identified and not done

| # | Item |
|---|---|
| W-1 | `20022d817` not replayed: 11 diverged files, mostly the RFC 5176 and RFC 7607 ledger, `rfc/enrolled.txt`, and two `plan/journal/` shards. |
| W-2 | `4f58d46a1` non-BGP half not replayed: `internal/component/command/` (completer, help, usage), `internal/component/l2tp/plugins/authradius/`, `internal/component/radius/`, `cmd/ze/hub/main_reload_test.go`, `internal/component/config/yang/modules/ze-extensions.yang`, `docs/guide/l2tp.md`, `docs/guide/radius.md`, `docs/architecture/meta/role.md`. |
| W-3 | The named API-sync barrier from `4f58d46a1` (`SignalPeerAPIReady`, `apiSyncExpected`, `resetAPISync`) spans `internal/component/plugin/` and edits the RFC-tagged `TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins`. Owes an owner row. |
| W-4 | The announce YANG `action` one-of container needs a `one-of` modifier in `ze-extensions.yang` and a renderer in `internal/component/command/{usage,completer,help}.go`. Applying the YANG alone flattens the flowspec action grammar. |
| W-5 | `rfc/short/rfc7607.md` is in neither `rfc/enrolled.txt` nor `rfc/not-enrolled.txt`; `./le rfc check` reports it. |
| W-6 | `./le rfc index-update` and a discrimination record are owed for the 9 new `RFC7607-2-*` tagged tests in `internal/component/bgp/reactor/rfc7607_update_test.go`. |
| W-7 | `TestChildSAInboundPolicyUsesNegotiatedTS` (`internal/component/ike/engine/child_test.go`) over-claims: its tag says an inner packet from outside the negotiated TSr is dropped; the body asserts selector values only and sends no packet. Correcting it owes an owner row. |
| W-8 | `RFC2865-5-1` is anchored to section 5 while its only source sentence is section 5.1. `validateID` refuses `(§5.1)` on that id, and renaming to `RFC2865-5.1-1` trips `checkRetiredRequirements`. |
| W-9 | `plan/journal/unwired-feature.md` rows 76 and 79 are one defect written twice by the same spec a day apart. Deduplicate on a journal pass. |
| W-10 | Eight lint findings sit in origin's own IKE code carried in by `093dfc8e5`: godot on three RFC quotations, `strings.Cut`/`CutPrefix`/`SplitSeq` modernizers, one unparam. |
| W-11 | `internal/component/plugin/server/server.go` (`PluginsWithPerPeerOpenPolicy`, ~52 lines) was swept into another session's commit `e691533a6`. Correct and in the tree, absent from this replay's diff. |

## Permanent reds, both correct, both closed only by product work

- `RFC8671-6.2-1`: `checkCoverageRatchet` reports it is no longer proven. Ordered
  by Thomas. `writeStatisticsReport` encodes correctly and nothing calls it; the
  missing piece is a timer reading `statistics-timeout`.
- `RFC3948-5.1-1`: the same shape. `Pool.Allocate` leases correctly, `registerIKE`
  discards the pool at `_ = ipPool`, and no engine code builds a Configuration
  payload. `plan/spec-ike-virtual-ip-assignment.md` carries the work.

## Defects the replay found in origin's own commits

- `f1245bbf4` does not compile as written. `openPolicyPlugins`, `ErrBadPeerAS` and
  a caller for `validateOpenPeerAS` are referenced and never shipped, so the RFC
  7607 OPEN check was dead code. Written and wired in `592659c70`.
- The same commit identified an OPEN-policy holder by a per-peer capability
  declaration alone. `bgp-gr` and `bgp-softver` declare per-peer capabilities and
  register no validate-open callback, so as written every peer configured with
  `graceful-restart` or `software-version` would have had its OPEN refused with
  NOTIFICATION 2 subcode 11.
- `0e5491e81` references `deliverMarker`, `snmpCounters`, `parseSNMPCounters` and
  `counterDelta` from `checkers.go` and ships none of them.
- `4f58d46a1` ships `TestSendOpenRefusesLocalASZero` knowingly RED and works
  around it with a second function.
- Origin's `TestRFC8671SenderConfigChangeBouncesTheSession` would pin
  NON-CONFORMANT behavior against a deleted producer. RFC 8671 Section 7.2 names
  a Peer Down/Peer Up sequence, which exists only inside a live session, so
  "bounced" cannot mean the session is torn down. Not carried forward.

## Defects the replay found in local code

- `internal/component/bgp/reactor/session_validation.go` quoted a sentence
  attributed to RFC 4760 that does not appear in RFC 4760. Replaced with the real
  Section 6 and Section 7 text in `0d031122e`.
- `writeMessageWithin` dereferenced a nil `bufWriter` and crashed. Guarded.
- `sendOpen` accepted a local AS of zero. The YANG range on `typedef asn`
  prevented it from config and nothing prevented it from a caller. Refused now.
- `parseTrailingOpts` (`internal/component/bgp/plugins/cmd/announce/announce.go`)
  drops an unclaimed trailing token silently, so `announce flowspec
  destination-ipv4 1.1.1.1/32 discard rate-limit 500` announces a plain discard.
  Thomas ordered the fix 2026-08-31.
- `./le rfc discriminate` can certify a tag from a mutant that cannot touch the
  requirement: gomu does not mutate `&^=`, so `RFC8671-6.2-1` had zero candidates
  on its own producing statement. Row in
  `plan/journal/green-that-could-not-have-been-red.md`.

## Environment, confirmed by the owner

- This Linux VM keeps the checkout on a network drive that freezes for several
  seconds. It produces waves of socket tests failing together at their nominal
  deadline. One re-run separates it from a defect.
- `cache/go-cache/...: no such file or directory` on files that exist is a
  CONCURRENT `./le scratch cache-clean` by another session, not the drive and not
  a full disk.

Both are written up in `docs/contributing/running-commands.md`.

## Process faults in this session, for the next one to avoid

- `./le commit create ... append` was run against a script that had ALREADY been
  executed, so it re-ran its earlier blocks. That produced `aedfaa70f` and
  `85dedea88`, duplicate subjects, and `85dedea88` swallowed regenerated
  `rfc/requirements/*` shards and `ai/RFC-REQUIREMENTS.md` under the wrong
  message. Use `replace` for every new commit.
- `test/rfc-changed.md` and `test/weakened.md` are rewritten PER COMMIT, and
  several agents wrote into them concurrently. Rows for an already-committed
  change must be removed before the next `./le commit create`, or the gate
  refuses with "names a test this commit does not change".
- The gate names an approval row by FUNCTION, not by file stem, wherever the tag
  sits inside a top-level `func`.
