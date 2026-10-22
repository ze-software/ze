# Handover: verify the RPKI revalidation relay tests on Linux (2026-10-22)

Thomas, 2026-10-22: "i want a commit if you need some testing done, give me an
handover for it to be done on linux". The Mac session committed the work without
running any gate; this file is what the Linux session owes.

## What was committed unverified

| Commit subject | What it carries |
|----------------|-----------------|
| `test: rpki revalidation reaches bgp-rr and bgp-rs destinations` | `test/plugin/bgp-rr-rpki-revalidate-relay.ci`, `test/plugin/bgp-rs-rpki-revalidate-relay.ci`, the observer fixture `plugin/rpki-revalidate-relay` in `internal/test/fixture/plugin_fixture_15{,_rpki}.go` |
| `rfc: reseal SHIFTED audit verdicts for rfc3948, 4456, 5575, 7705, 8955` | mechanical `./le rfc reseal` re-stamps |
| `docs: README states what each area does` | README prose and the area table |
| `verification-debt: record sessions 9391d1fe, cb4c8c08, ff5c264c, c50c93a8` | debt ledger rows written by earlier commit scripts |

The test commit was made with `structural-red-ok`: `go staticcheck/check/part/2/of/6`
was red at `ee108f4731` in a verify run of 2026-10-21 and no later run reached it.
The debt rows of sessions ff5c264c and cb4c8c08 attribute that red to the
environment cache fault (`cache/stdpatch`), not to code. Unproven on this tree.

## What to run on Linux, in order

| Step | Command | Green means |
|------|---------|-------------|
| 1 | `./le go staticcheck check part 2 of 6` | the carried red was the cache fault; if still red, read the finding: a hit in `plugin_fixture_15_rpki.go` is this work's to fix |
| 2 | `./le test plugin` scoped to `bgp-rr-rpki-revalidate-relay` and `bgp-rs-rpki-revalidate-relay` | both relays deliver the path whose verdict flips NotFound to Valid after peer-up replay |
| 3 | `./le verify worktree` once | clears the debt rows this commit added (`./le commit debt-clear`) |

The tests' headers claim "Proven red with the relay call disabled". That red was
observed on the Mac by the session that wrote them and no record of it is in the
repository. If step 2 is green, repeat the red walk once per relay (disable the
relay call in `rr/validation.go` `replayValidation` and in
`rs/server_validation.go`, rebuild, expect the destination's conn=2 seq=2 UPDATE
to be missing), restore, and note the result here.

## Closure text committed ahead of closure

The IKE-EAP closure text (`plan/learned/032-ike-eap-rfc-defects.md`, the
"Status: built" line in `docs/contributing/patching-the-standard-library.md`,
the repoints in `plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` and
`spec-rfc-verdict-test-fix-pass.md`) and the four update-propagation repoints in
`plan/immediate/` say their specs closed on 2026-10-21. Both specs are open.
Thomas ordered it committed anyway ("the plans and docs are not commited"): it
landed in `db31d2bdb9`, which removed handover 18's two patches. At each spec's
close there is nothing to apply; the closing commit only has to make the text true.

## Linux session result (2026-10-22, step 2)

Both tests were red on first run, from test defects, not product defects:

| Failure | Cause, at the producer | Fix |
|---------|------------------------|-----|
| `ZE-OBSERVER-FAIL: the destination's second session came up before the route was judged ineligible` (both) | The observer used `request peer 127.0.0.2 teardown 4` and assumed ze waits out `connect-retry`. `Peer.run` (`internal/component/bgp/reactor/peer_run.go`) answers `ErrTeardown` by dialing again with no wait, so conn=2 raced the source UPDATE | `plugin_fixture_15_rpki.go` holds the destination with `request peer 127.0.0.2 disable ...` and releases it with `request peer 127.0.0.2 enable` only after the route is judged NotFound/ineligible |
| rr destination conn=2 seq=2 got `UPDATE WITHDRAWN 10.0.0.0/24` | Consequence of the race: conn=2 was up when the source UPDATE was live-forwarded, and `forwardUpdateValidated` / `forwardValidationWire` (`internal/component/bgp/reactor/forward_validation.go`) send an ineligible announcement as a withdrawal, as `docs/architecture/bgp/structural-forwarding.md` "Receive validation and cache changes" documents. Expected behavior, not a defect | gone with the ordering fix |
| rr `expect=bgp:conn=2:seq=2` was a 17-byte stub | never filled in | full 68-byte UPDATE: received attributes, ORIGINATOR_ID 127.0.0.1, CLUSTER_LIST 10.0.0.1; matched byte for byte by the run |

Green: `./le --name h19 test bgp plugin --pattern rpki-revalidate-relay -v`, PASS 140 bgp-rr-rpki-revalidate-relay, PASS 159 bgp-rs-rpki-revalidate-relay.

Red walk, once per relay, each relay call replaced by `continue`, then restored byte for byte (no diff left in either file):

| Relay disabled | Result |
|----------------|--------|
| `rr/validation.go` `replayValidation`, the `ribevents.RelayReplayPath` call | FAIL 140: `ZE-OBSERVER-FAIL: the destination never received the revalidated path 10.0.0.0/24: 0 UPDATEs sent before the release, 0 after`; the destination's conn=2 second message was the shutdown Cease 6/2, the expected UPDATE (ORIGINATOR_ID, CLUSTER_LIST, NLRI 10.0.0.0/24) missing |
| `rs/server_validation.go` `replayValidation`, the `ribevents.RelayReplayPath` call | FAIL 159: same observer line; the expected UPDATE (AS_PATH [65000 65001], NEXT_HOP 1.1.1.1, NLRI 10.0.0.0/24) missing, Cease 6/2 in its place |

Committed as `926c6f9fa6`. Rerun on a fresh build after the commit: PASS 140, PASS 159.

## Steps 1 and 3: selective proof only (owner, 2026-10-22)

Thomas stopped the full run: "no full run the machine is too busy you can do
selective testing to allow us to finish". So neither gate ran in full:

| Gate | What ran instead | Result |
|------|------------------|--------|
| 1. `./le go staticcheck check part 2 of 6` | One attempt hit the 10m30s deadline at load average ~130, no verdict. Staticcheck has no package scope | still owed |
| 1, substitute | `./le go lint run scope ./internal/test/fixture/` (host and Linux integration builds) | no finding in `plugin_fixture_15_rpki.go`. Four findings in files other sessions committed: `plugin_fixture_14.go` errorlint, `plugin_fixture_02_events.go` / `register_config_apply_ordering.go` goconst `del`, `plugin_fixture_16.go` unparam `plugin16UpdateAndQuiesce` |
| 3. `./le verify worktree` | `./le arch tier check` and `./le arch iface-resolution`: both OK | full verify and `./le commit debt-clear` still owed; debt rows in `plan/verification-debt/1840d698.md` stay open |

The next quiet-machine `./le verify worktree` clears the rows; nothing else is owed by this handover.
