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
