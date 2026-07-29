# 1290 -- Every IPv6 next-hop was invisible in `show bgp rib`, behind a test that could not fail

## Context

`test/plugin/rib-inject-rfc5549.ci` (CI test 396) was the last unexplained red in
the plugin suite, failing 2 in 5 standalone on an idle host and reported as a
load flake. It was neither a flake nor a test-only problem.

The test exists to prove that `request bgp rib inject ... nexthop 2001:db8::1`
survives a round trip through the RIB and is rendered back to the operator; its
`PREVENTS:` header names exactly that. Its next-hop assertion failed on EVERY
run, passing and failing alike, and nothing noticed: the check was
`print('FAIL: ...'); sys.exit(1)`, the observer-exit antipattern that
`ai/rules/testing.md` bans outright. ze itself still exits 0, so
`expect=exit:code=0` passed, and the file's only `reject` matches
`ZE-OBSERVER-FAIL`, which `sys.exit` never emits.

The defect it was hiding is user-visible and total.
`enrichRouteMapFromEntry` (`rib_attr_format.go`) rendered the next-hop from
`Bundle.NextHop` alone, and `storage/attrparse.go` interns that handle only from
`attribute.AttrNextHop` -- the IPv4-only type 3. RFC 4760 puts a multiprotocol
next-hop in MP_REACH_NLRI, which `attrparse` routes to `otherAttrs` instead, so
`show bgp rib best` and `show bgp rib received` printed `"attributes":{"origin":
...}` and no next-hop at all for **every** IPv6 unicast route and **every** RFC
5549 extended next-hop.

## Decisions

- **Fall back to MP_REACH in the renderer, mirroring the forward path.** The
  recovery already existed twice: `bestCandidateNextHopAddr`
  (`rib_bestchange.go`) tries type 3 then `extractMPNextHopAddr`, and
  `ribout_entry.go` handles `AttrMPReachNLRI` in its attribute switch. Only the
  show renderer was never taught the second step, so this is one missing branch,
  not a new mechanism. Chosen over interning the MP next-hop into `Bundle.NextHop`
  at parse time, which would have made the bundle lie about which attribute the
  route actually carries and changed the dedup pool's contents on a hot path to
  fix a cold-path rendering bug.
- **Both callers, one fix.** `enrichRouteMapFromEntry` feeds the best-path
  terminal (`rib_pipeline_best.go`) and the general show pipeline
  (`rib_pipeline.go`), so `received` was fixed by the same branch. The docs had
  the split written down as intended behavior and are corrected.
- **The barrier waits for the peer, not for ze's own counter.** `eor-sent >= 1`
  proves ze flushed the marker to its own socket (`IncrEORSent` runs after
  `Session.SendUpdate` -> `flushWrites` returns) and nothing about whether the
  peer read it, so the observer's `request shutdown` could still close first.
  That was observed once in 258 stressed runs. The ze-peer runs in check mode
  without linger and closes as soon as its expectations are met, so the test now
  waits for ze to leave `established` before shutting down: a condition, not a
  delay. It also made the test faster (2.5s, down from 5.5s).

## Consequences

- Any operator or tool that read a next-hop out of `show bgp rib best|received`
  was reading nothing for IPv6. Dashboards and scripts built on that field were
  silently empty, exactly as with the plugin metrics in `plan/learned/1286`.
- `docs/guide/route-injection.md` claimed `show bgp rib best` "displays the
  recovered next hop" (false until now) and that `received` "omits MP next hops"
  (true until now, and recorded as if it were a design choice). A documented
  limitation is not evidence that anyone decided it.
- The three siblings of this test have now all been found to assert nothing:
  1283, 1286 and this one. The common shape is an observer whose failure path
  cannot reach the runner. `runtime_fail` is the only spelling that can.

## Gotchas

- **A test can fail its own assertion on every run and still report PASS.** The
  identical `FAIL: IPv6 extended next-hop not in best show` line appears in the
  passing and the failing captures; the only difference between them was
  `end-of-rib send failed ... invalid FSM state`. `sys.exit` raises through the
  `finally`, which fired `request shutdown` early and skipped the EOR barrier, so
  a permanent defect wore a 40% flake as a disguise.
- **`stress-repro.py` sets `ZE_TEST_NO_BUILD=1` and defaults to the bare
  `bin/ze`.** Under an AI session the binary you just built is
  `bin/ze-<session-id>`, so the first stress run tested a five-hour-old binary and
  "reproduced" a bug that was already fixed. Export `ZE_BIN`/`ZE_TEST_BIN` before
  invoking it. Documented in `ai/rules/flaky-under-load.md`; still caught me.
- **Do not rebuild `bin/ze-<session>` while a test run is launching it.** One
  draft run failed with zero daemon output because `make ze` was rewriting the
  binary underneath it. It looks exactly like a startup hang.
- **The `-p` flag of `ze-test web` is `--pattern`, not parallelism.** The web
  suite has no parallelism flag at all -- it is hardcoded 4-way -- so the
  diagnostic `plan/known-failures/ze-functional-test-web-commit-flow.md` suggests
  (`ze-test web -a -p 1`) silently runs a 19-test subset 4-way and proves nothing.

## Files

- `internal/component/bgp/plugins/rib/rib_attr_format.go` -- MP_REACH next-hop fallback in `enrichRouteMapFromEntry`
- `internal/component/bgp/plugins/rib/rib_commands_test.go` -- best-path and received renderer tests, plus the legacy-NEXT_HOP guard
- `test/plugin/rib-inject-rfc5549.ci` -- `runtime_fail` throughout, and the peer-consumed barrier
- `docs/guide/route-injection.md` -- the corrected claim about both show views
