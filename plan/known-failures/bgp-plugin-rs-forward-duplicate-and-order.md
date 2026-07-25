### `ze-test bgp plugin` route-server forwarding -- intermittent duplicate/ordering, NOT the module rename

Observed 2026-07-25 during the `codeberg.org/thomas-mangin/ze` ->
`github.com/ze-software/ze` module rename. Two DIFFERENT tests in the `plugin`
suite failed on two different full runs, both in the route-server forwarding
path, neither reproducibly:

| Run | Test | Symptom |
|-----|------|---------|
| loaded machine | 380 `rfc7606-relay-one-field` | receiver got the split ANNOUNCE and never the withdraw that must precede it |
| idle machine | 254 `llgr-readvertise-multipeer` | observer received the same UPDATE TWICE (identical bytes, back to back) |

Never the same test twice, which is why this is one entry rather than two: the
failing test moves, the subsystem does not.

NOT caused by the module rename. The rename rewrote import paths only; no `.ci`
file under `test/plugin/` changed (`git diff --stat -- test/plugin/` was empty at
the time of both failures), and no daemon logic changed. 380 passes 3/3 in
isolation and reproduces on invocation 23 of 60 under
`scripts/dev/stress-repro.py "bgp plugin" --test 380 --any-failure`; captured at
`tmp/stress-repro/bgp-plugin-380-20260725-150311.log` (scratch, not durable).

Pre-existing by the tests' own record: the header of
`test/plugin/rfc7606-relay-one-field.ci` documents the sibling
`bgp-rs-reactor-fastpath.ci` failing 1 in 6 **on an unmodified tree** with the
same EOR-versus-forward signature, and states in terms that what produces the
extra frame "is NOT established".

NO root cause asserted. The wire bytes are evidence; the producing function has
not been read. Two hypotheses worth testing FIRST, neither verified:

1. peer establishment races the source UPDATE, so a late-establishing client is
   served by the RS replay path (current RIB state, hence no withdraw and
   possibly a second copy) rather than by the split-forward path;
2. the split and the replay both deliver, giving the duplicate seen in 254.

Deferred by owner decision (2026-07-25): the rename is mechanical and was
committed separately so a BGP investigation does not ride along inside it.
Triage owner is the route server and forward path
(`internal/component/bgp/plugins/rs`, `internal/component/bgp/reactor/forward*`),
not the rename.
