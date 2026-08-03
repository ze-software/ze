# 1278 -- fixit-sleeps-cli-harness (closed as stale)

## Context

This spec proposed building CLI test-harness infrastructure so five `.ci` tests could
replace blind `time.sleep` calls with deterministic waits. It was carried as open work for
weeks and cited as one of the blocked items in the spec-fixit backlog.

Every premise it rested on was false by the time anyone came back to it. The work it
described had already been done piecemeal by other changes, and nobody told the spec.

## Decisions

- **Closed as STALE rather than implemented.** Checking the spec against the tree cost one
  session and a handful of greps; implementing it would have built harness infrastructure
  for a problem that no longer existed.
- **The staleness verification is recorded in the spec itself** (commit `a95f0b7f8`,
  preserved in history) rather than only in this summary, so the evidence for each falsified
  premise survives alongside the claim it refutes.

## Consequences

- The five files the spec targeted need no work. Their remaining `time.sleep` calls are
  bounded polls that `ai/rules/testing.md` classifies as already-correct
  waits, each carrying the comment that rule requires.
- The `.ci` sleep ratchet is unaffected: the count sits exactly on its ceiling of 101
  (`test/.ci-sleep-baseline`, the sum of `125 -11 -12 -1`), with zero headroom. Any NEW
  `.ci` sleep fails `check_ci_sleep_ratchet` immediately.

## The residual item -- "stale" did NOT mean "nothing left"

The first staleness verdict on this spec said it had no remaining work. That was wrong,
and an independent check caught it before the spec was deleted.

The spec's A-4 obligation was live: *"wire the linux-gated static suite into an actually-run
linux path, do NOT drop the gate."* The tree had moved AGAINST it. `test/static/004-show.ci`
and `005-table-interface.ci` both carry `option=needs-linux`, and
`internal/test/runner/record_parse.go` skips such a record when `GOOS != linux`, so
they never run on the darwin dev host. `static` is absent from `all_suites` in
`mk/test-functional.mk`, so the default verify gate never runs it, and absent from the
`fsuite` list in `scripts/evidence/qemu-all-tests.sh`, so the QEMU Linux-only target -- the
only automated Linux functional path -- never runs it either. The suite's only two
invocation sites are a manual target and `ze-release-evidence`, which no workflow invokes.

**A rewrite fixed a real defect in those tests and left them behind a gate no runner
honors.** They are not skipped honestly; they are never reached. The two tests the
staleness argument cited as PROOF the spec was dead are themselves tests that execute
nowhere.

Rehomed to `plan/spec-release-evidence-gate.md` (which already names `static` among its
non-gated suites) with rows in `plan/deferrals/ad-hoc-2026-07-27-2c83641a.md`, in the same
commit that removed this spec. Two smaller items travel with it: both static tests run
`ip link add` while declaring no `caps=net-admin`, which `record_parse.go`
documents as a fail-open shape; and `test/vpp/007-fib-route-lookup.ci` cites a
`// Design:` spec that does not exist.

## Gotchas

- **A spec is a claim about the code, not a fact about it, and it rots silently.** The
  headline premise was that `test/static/004-show.ci` calls `ze cli -c "show static"`
  against a config declaring no SSH server, so the CLI could never connect. That test no
  longer uses `ze cli` at all: it dispatches through an external plugin
  (`static-show.run`) and waits on `api.wait_for_post_startup`. Its own header explains
  why. The spec had simply not been re-read against the tree.
- **"Stale" is a claim about every premise, and it is easy to over-apply.** The first pass
  here declared the spec had no remaining work; one of its obligations was live, and the
  evidence cited FOR staleness (two rewritten tests) was itself the evidence for the item
  that survived. Verify a staleness verdict the way you would verify a fix: independently,
  by someone trying to refute it. A spec deleted with live work in it loses that work
  silently, which is strictly worse than leaving it open.
- **Check before working, not after.** This is the second spec in one session found
  already-settled (see `plan/learned/1277-fixit-rfc6286-bgp-identifier.md`, which was
  finished at Phase 5/5 but unclosed). Two of the specs probed were not live work. The
  cheap check is `ai/rules/planning.md` "Verify Specs Against Code"; run it before
  budgeting a session, because the real backlog can be materially smaller than the file
  count suggests.
- **Neither of these two was visible to `spec-closure-check.py`.** That detector keys on a
  committed `plan/learned/NNN-<slug>.md` matching the spec stem. A stale spec has no such
  summary and never will until someone closes it, so the tool is silent on exactly the
  specs most worth retiring. Its silence is not evidence that a spec is live.

## Files

- `spec-fixit-sleeps-cli-harness` -- the spec itself, removed by this closure; its `## STALE`
  section (commit `a95f0b7f8`) holds the per-premise verification
- `test/.ci-sleep-baseline` -- the ratchet, unchanged
