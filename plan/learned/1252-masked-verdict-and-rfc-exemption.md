# 1252 -- A verdict that passes on any one peer, and an RFC exemption granted to everyone

## Context

Four functional tests (91, 224, 398, 458) had been handed forward twice as
"flaky". Only one turned out to be a product race. The others were held up by two
pieces of scaffolding that could not fail: a multi-peer test verdict that passed
if ANY peer succeeded, and 52 malformed BGP frames whose declared Length did not
match their byte count. Fixing the verdict exposed 7 further tests that had been
green while failing. Chasing one of those into the egress path then surfaced an
RFC 4271 violation sitting behind a comment that made it look deliberate.

## Decisions

- **The `.ci` verdict requires EVERY check-mode peer to report success**, not any
  (`peer_contract.go` `failedCheckPeers`, replacing a `strings.Contains` over the
  concatenation of all peers' output in `runner_exec.go`). Chose per-peer
  evaluation over adding a warning: a warning does not fail a test, and
  `ai/rules/evidence.md` requires a guard to deny or speak. The peer
  label is captured at launch rather than reconstructed at verdict time, because
  per-peer attribution is the whole diagnostic value.
- **Repaired the malformed frames rather than deleting the tests.** The tests
  described real behaviour; only their input bytes were wrong. Declared lengths
  were left untouched so the intended frame is recovered, not redefined.
- **Added a repo-wide frame-length gate** (`TestCIFrameLengthsWellFormed`) rather
  than a parse-time check in the runner: a parse-time check only fires when the
  test runs, while a tree-wide gate catches a bad fixture the moment it is added,
  including in suites nobody executes. It carries a `# malformed-frame:` escape
  hatch so a test whose SUBJECT is a bad Length stays possible.
- **`ReactorForwarded` now means "the fast path delivered", not "the fast path
  ran".** bgp-rs reads it as a delivery claim and drops the UPDATE on its
  `default` arm, so setting it after matching zero peers was a silent drop.
- **Scoped the RFC 7947 route-server AS_PATH exemption to actual RS-clients**
  (`reactor_api_batch.go` `buildBatchASPath`). Chose prepend-when-missing over
  rejecting the announce: RFC 4271 Section 5.1.2 requires our AS to lead, and an
  operator who already spelled out the full path is left alone rather than
  double-prepended.

## Consequences

- Expect more red. The verdict fix took the suite from 29 failures to 31, and the
  7 newly-red tests are pre-existing defects that were always failing. They are
  not regressions and must be triaged rather than re-masked.
- Any `.ci` added from now on must carry well-formed frames or an explicit
  opt-out marker.
- `ai/rules/rfc-compliance.md` is now **blocking** and covers every protocol Ze
  implements, not just BGP. It states that only an explicit instruction from
  Thomas authorises a deviation, and that a test pinning a violation is the
  violation with a green bar on top.

## Gotchas

- **An End-of-RIB expectation declared AFTER a forwarded-route expectation is
  unsatisfiable by construction.** A destination peer's EOR is emitted at ITS
  establishment, before any route is forwarded to it; the early EOR does not match
  the route rule, and `checker.go` silently swallows an unmatched EOR instead of
  re-offering it, so the trailing slot can never be filled. Seven fixtures carried
  this. The same shape is FINE for a peer receiving ze's own config routes, which
  really are sent before the EOR (`peer_initial_sync.go`) -- the pattern alone is
  not the defect, the route's provenance is.
- **RFC 4724 does not order the EOR against routes learned later.** It requires
  only that the EOR follow the speaker's own initial dump. Tests asserting
  EOR-vs-forwarded-route ordering asserted something Ze never owed, and two
  sibling fixtures pinned OPPOSITE orders of the same two frames.
- **An attribute-set match is not proof of provenance.** The un-prepended frame in
  test 91 matched `buildRIBRouteUpdate`'s output exactly (no LOCAL_PREF on eBGP,
  no tombstone marker), and a fix was written and then reverted: that test loads
  only the `bgp-rs` plugin, so the function is not reachable there. Read the
  producer's reachability in the specific configuration before editing.
- **An exemption applied unconditionally is a violation for every case it was not
  written for.** The verbatim-AS_PATH arm was commented "route-server
  transparency" and applied to every peer; RFC 7947 Section 2.2.2 grants it to
  RS-clients only.
- `ASPath.Prepend` mutates its receiver and a route's stored path is shared RIB
  state, so any prepend onto a stored path must deep-copy first.

## Files

- `internal/test/runner/peer_contract.go`, `runner_exec.go`, `runner_exec_util.go`,
  `record.go`, `report.go` -- per-peer verdict + attribution
- `internal/test/runner/ci_fixture_test.go` (new) -- frame-length gate
- `internal/component/bgp/reactor/reactor_notify.go`, `forward_rs.go` -- delivery
  claim only when the rail dispatched
- `internal/component/bgp/reactor/reactor_api_batch.go` -- RFC 4271 Section 5.1.2
  prepend, RFC 7947 Section 2.2.2 exemption scoped to RS-clients
- `ai/rules/rfc-compliance.md` -- blocking, every protocol
- 11 `.ci` fixtures -- 52 frame repairs, 7 unsatisfiable EOR rules removed
