# Open items from the origin/main divergence replay, 2026-08-31

The replay itself is DONE and its narrative is in `git log`, from `c0d4b3612` to
`fe51839da`. This page carries only what outlives it.

One sentence of context, because several rows below cite it: `origin/main` held 12
commits made under a different git identity on 2026-08-30, fetched here at
`refs/remotes/origin/main@{2026-08-31 00:56:48}` and never integrated, so both
machines walked the same RFCs and wrote different artifacts. The 12 were replayed
by hand, merging each file. Origin's SHAs are not ancestors of HEAD, by
construction.

## Force push readiness

Three files are tracked on `origin/main` and not in HEAD. All three were deleted
DELIBERATELY by local commits, so a force push removing them is correct:

| Path | Deleted by |
|---|---|
| `internal/component/config/cli/cmd_ls.go` | `79d199a31` |
| `test/parse/cli-data-ls-show.ci` | `03d5e261c` |
| `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` | `cc6a79d2c` |

Re-run the gate before pushing, because other sessions share this checkout:

```
comm -23 <(git ls-tree -r --name-only origin/main|sort) <(git ls-files|sort)
```

`git diff HEAD origin/main` still shows 6498 lines origin holds that HEAD does
not. 4180 of them are in files the 12 commits NEVER touched, where origin's copy
is simply the older one and local superseded it after the fork. The remaining
~2300 are in files the 12 did touch; each was read by a merge agent that kept
local as the newer authoring. That set was not re-audited line by line.

## Decisions only Thomas can take

| # | Decision |
|---|---|
| D-1 | `RFC8671-5.2-1`, the rule for marking a pre-policy Adj-RIB-Out view, is declared in `rfc/short/rfc8671.md` with no source site and no tags. Ze exports the post-policy view only, which RFC 7854 Section 5 permits. The gate prescribes retiring the id, which deletes a MUST row from the published ledger. |
| D-2 | Two specs claim one job: `plan/spec-ike-virtual-ip-assignment.md` (new, assignment only) and `plan/spec-ipsec-remote-access.md` (design, phases A-G, AC-7..AC-10 and AC-16..AC-28). One supersedes or folds into the other. Either way the survivor must say that the work reverses the recorded no-IRAC-role decision on the RFC 7296 status row. |
| D-3 | 188 `binds-another-role` sites across 18 RFCs need reclassifying under the 2026-08-31 ruling that the kind is presumed wrong. Concentrated in `rfc4364` (39), `rfc2869` (33), `rfc2865` (32), `rfc4303` (29), `rfc4761` (16), `rfc1035` (11), `rfc3032` (10). This belongs to the `rfcgate-6` spec, not to a replay session. |
| D-4 | Eleven RFC 2865 ids are now tagged twice, once on the admin path in `internal/component/radius` and once on the subscriber path in `internal/component/l2tp/plugins/authradius`. Two producers, two claims. Keep both, or cut one side. |

## Work identified and not done

| # | Item |
|---|---|
| W-1 | The named API-sync barrier from `4f58d46a1` is NOT landed. `plugin.ReactorStartupCoordinator` is embedded in `types_bgp.go`, so the signature change breaks seven packages at once. One change must carry: `reactor/peer.go` (`apiSyncExpected []string` and `apiSyncSignalled` replacing the int32 counter, `resetAPISync([]string)`, `SignalAPIReady(plugin.Sender)`), `reactor/peer_run.go`, `reactor/api_sync.go`, `reactor/reactor_api.go`, `plugins/cmd/peer/session.go`, six `mock_reactor_test.go`, `update_text_test.go`, and `plugin/types.go` plus `plugin/coordinator.go`. It also edits the RFC-tagged `TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins`, so it owes an owner row. |
| W-2 | `internal/test/fixture/ui_fixture_cli_announce.go` is UNREACHABLE. It registers `ui/cli-announce-reaches-the-wire` and `ui/cli-announce-tag-round-trip`; no `.ci` names either, here or on origin, and the `peer-script` it reads exists in neither tree. The driver half was never written. |
| W-3 | The two walk tests landed in `fe51839da` owe discrimination records. A run before they landed reported 68 violations of the form "new against git HEAD and carries no discrimination proof" against their tags. |
| W-4 | `TestChildSAInboundPolicyUsesNegotiatedTS` (`internal/component/ike/engine/child_test.go`) over-claims: the tag says an inner packet from outside the negotiated TSr is dropped; the body asserts selector values and sends no packet. Correcting it owes an owner row. |
| W-5 | `RFC2865-5-1` is anchored to section 5 while its only source sentence is section 5.1. `validateID` refuses `(§5.1)` on that id, and renaming to `RFC2865-5.1-1` trips `checkRetiredRequirements`. |
| W-6 | `RFC2865-5-5` carries a positive tag and no negative anywhere. |
| W-7 | `plan/journal/unwired-feature.md` rows 76 and 79 are one defect written twice by one spec a day apart. Deduplicate on a journal pass. |
| W-8 | Eight lint findings sit in origin's own IKE code carried in by `093dfc8e5`: godot on three RFC quotations, `strings.Cut` / `CutPrefix` / `SplitSeq` modernizers, one unparam. |
| W-9 | `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json` are stale for `ze-radius-conf.yang`, both before and after the `profile-attribute` change. |
| W-10 | `TestDraftReadmeNamesEveryCheck` (`internal/test/runner/draft_dir_test.go`) expects the substring `docwiring/checks.go`; `test/draft/README.md` says `internal/le/doc/wiring/checks.go`. Committed drift from the package move: the test string is stale, the README is right. |
| W-11 | Fifteen rows were removed from `test/weakened.md` so an unrelated commit could pass, and saved verbatim under this session's scratch as `weakened-other-session-rows.txt`. `weekly` has confirmed they are not its. They name `TestSummary`, `TestActionTableDeclaresFourNativeVerbs`, the `TestAdminTreeFromYANG_*` trio and the `TestHelp*` group, so `short-and-long` or `rfc-tag` is the likely owner. Whichever session owns them cannot commit until they are restored. |

## Permanent reds, both correct, both closed only by product work

`checkCoverageRatchet` reports each as no longer proven. Both were ordered by
Thomas on 2026-08-31 and neither is cleared by an annotation.

- `RFC8671-6.2-1`. `writeStatisticsReport` encodes a Statistics Report correctly
  and clears the O flag; nothing calls it, so none reaches the wire. The missing
  piece is a timer reading `statistics-timeout`, which `behaviorOf` already
  carries into `senderBehavior.statistics`.
- `RFC3948-5.1-1`. `Pool.Allocate` leases uniquely and refuses on exhaustion;
  `registerIKE` discards the pool at `_ = ipPool`, and no engine code builds a
  Configuration payload. `plan/spec-ike-virtual-ip-assignment.md` carries it.

## Owner rulings, 2026-08-31

All five are directives in
`ai/rules/points/rfc-compliance/directives/count-conformance-on-the-whole-stack.md`
(`fc868a86d`, `014a033ef`, `8658100ed`) and in
`plan/journal/green-that-could-not-have-been-red.md`. They are recorded here only
so a reader knows where to look.

1. Conformance counts what the WHOLE STACK produces; who enforces it does not
   matter. A requirement met by configuring the kernel is MET, and still owes a
   test at the boundary Ze owns.
2. A gap is an ISSUE; an exclusion is a DECISION.
3. Conformance is not owed for an OPTIONAL feature outside Ze's scope. The new
   `excluded-kind: feature-out-of-scope` states it; the absent feature is recorded
   as an implementation gap, never a conformance gap.
4. `binds-another-role` is PRESUMED WRONG and owes a justification.
5. The gomu operator set is an accepted limitation. Fix `discriminate` to refuse
   when no candidate falls inside the producing statement; do not extend gomu.

## Two process faults worth not repeating

- `./le commit create ... append` against a script that has ALREADY run re-executes
  its earlier blocks. That produced `aedfaa70f` and `85dedea88`, duplicate
  subjects, and `85dedea88` swallowed regenerated `rfc/requirements/*` shards under
  the wrong message. Use `replace` for every new commit.
- `test/rfc-changed.md` and `test/weakened.md` are rewritten PER COMMIT. Rows for
  an already-committed change must be removed before the next `./le commit create`,
  and in a shared checkout those rows may belong to another session: save them
  before deleting. The gate names an approval row by FUNCTION, not by file stem.
