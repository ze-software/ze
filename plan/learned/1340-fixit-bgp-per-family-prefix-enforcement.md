# 1340 -- fixit-bgp-per-family-prefix-enforcement

## Context

Three BGP prefix-limit YANG leaves (`teardown`, `idle-timeout`, `updated`) sat
inside the per-family `prefix` container but were stored as per-peer scalars on
`PeerSettings`. The parser wrote them once per family inside a sorted loop, so
the last family in key order governed every family: a peer carrying
`ipv4/unicast` and `ipv6/unicast` always took `ipv6/unicast`'s values, because
the byte `4` sorts before `6`. YANG defaults are materialized into every family
entry, so a family that stated NO opinion still arrived carrying `teardown true`
and overwrote a neighbor that had explicitly asked for warn-only. Ze stopped
sessions the operator asked to keep. The fix makes all three per family and
plumbs the offending family through the teardown error to the reconnect
decision.

## Decisions

- Made the three fields `map[string]uint16` / `bool` / `string` over rejecting a
  config where two families disagree: the YANG already advertises per-family
  control and two sibling leaves in the same container were already maps, so
  rejecting would narrow a config surface the schema promises.
- Added `PrefixTeardownFor` over reading `map[string]bool` at the call site: a
  bare read returns `false`, which means warn-only, which is the direction that
  silently disables the defense against a peer flooding the RIB.
- Carried the offending family BESIDE `ErrPrefixLimitExceeded` in a
  `prefixLimitError` wrapper, over replacing the sentinel with a new type: every
  existing `errors.Is` on the reconnect path had to keep matching.
- Stored `updated` per family and aggregated to the OLDEST date at the boundary,
  over changing the `prefix-updated` JSON key to a per-family object: the
  external shape survives, and oldest keeps the staleness alarm conservative.
- Thomas ruled that `idle-timeout 0` (the YANG default) HOLDS THE PEER DOWN,
  over reconnecting on the normal backoff, and a `prefix { reconnect never |
  backoff | timer; }` leaf was added so both intents stay expressible. Cisco and
  Juniper hold the peer down for the same event.

## Consequences

- Every peer whose config never mentioned `idle-timeout` changes behavior: a
  prefix-limit teardown now keeps it down until an operator recreates it. The
  state reads `idle-hold`, `ze show warnings` carries `prefix-hold`, and
  `reconnect backoff` restores the old behavior.
- `PeerStateIdleHold` was appended to `reactor.PeerState`, which is mirrored by
  value in `plugin.PeerState`. A new state must go at the END of BOTH, and
  `peerStateNames` (`peer_stats.go`) needs the string or metric labels leak.
- Enforcement is now decided per family, but `ze_bgp_prefix_teardown_total` is
  still labeled `{peer}` only. Adding a `family` label changes an existing
  metric's label set, so the family reaches the operator on the log line and in
  the RFC 4486 NOTIFICATION instead.
- The warn-only drop is per UPDATE, not per NLRI: `processMessage`
  (`session_read.go`) returns before plugin delivery, so an UPDATE carrying the
  prefix that crossed the maximum is consumed whole, other families' routes in
  that same message included.

## Gotchas

- **A `.ci` under `test/parse/` can be vacuous for a storage claim.** `ze config
  validate` and `ze config dump --json` both read the config TREE. This defect
  lived downstream of the tree, in the tree-to-`PeerSettings` parser, so the
  parse test stayed green with the fix reverted while its header claimed to
  prove the fix. Ask what LAYER a command observes before believing what a test
  proves. `ze config validate` does reach `parsePeerFromTree`, so a test that
  asserts a REJECTION does discriminate; one that asserts a value round-trips
  does not.
- **`audit-test-relaxation.py` is FILE-scoped where the pre-write hook is
  FUNCTION-scoped.** Both call the same `_rfc_tagged_change_err`, but the hook
  passes `tag_scope=_enclosing_tagged_scope(fp, hunks)`. Editing an untagged
  function in a file that carries an `RFC requirement:` tag elsewhere passes the
  hook and is reported `[WEAKENED]` by the audit. That is a candidate finding,
  not a verdict, and the answer is evidence in the review, never a self-written
  `rfc-test-change-approved:` token.
- **A doc comment claiming a guard fails closed is not a guard.**
  `prefixReconnectDecision` said an unnamed family "reads as never" while the
  code returned `ok=false` and fell through to the normal backoff. The comment
  survived review twice because it described the intended design. Two reviewers
  found it by reading the producer; nothing else could have.
- **`map[string]bool` for a safety flag whose safe value is `true` points the
  wrong way.** The Go zero value is the unsafe answer. This is the zero-value
  trap in `ai/rules/evidence.md` in its exact documented form.
- **A field type change reaches every test that SETS the field, not only the
  readers.** One of those tests was RFC-tagged, which turned a mechanical
  refactor into an edit only Thomas could authorize and stalled the spec for a
  day. Route such a change through a shared, untagged helper wherever one exists.
- **An UPPER-bound assertion over a helper that answers 0 on failure cannot
  fail.** `Ze.rib_count` (`test/interop/interop.py`) returns 0 for a failed
  exec, a timeout and a regex miss alike. Every other caller asserts a LOWER
  bound, where that 0 fails loudly; the one upper-bound caller read it as
  success. Making the read raise proved it had NEVER worked: the `ze` client
  inside the interop image resolves no BGP show verbs at all, and scenario 05
  is red on the same cause. Before asserting a bound, ask which direction the
  helper's failure sentinel points.
- **Mutate with the RUNNER's build tags, or the measurement lies.** A mutant `ze`
  built from a hand-written tag list PASSED the functional test it was supposed
  to redden, and the conclusion drawn from that (the test does not discriminate,
  withdraw it) was wrong. Rebuilt with `runner.TestBuildTags` (`zetest ze_core
  ze_distro ze_setup` plus `feature-gates.txt`), the same mutation FAILS the same
  test in 20.0s. A mutation that fails to redden is a claim about the BUILD
  before it is a claim about the test. Run it through
  `ZE_TEST_NO_BUILD=1 ZE_BIN=<mutant> ZE_TEST_BIN=bin/ze-test`: the runner honours
  those variables together and only together.
- **A recorded gap that does not exist costs what a false claim costs.** One bad
  mutation produced a deferral row, a split-verdict acceptance criterion and a
  withdrawn test, all describing a hole in the coverage that was never there. The
  reviewer who caught it re-ran the experiment rather than reading the record.
- **ze's own read path merges UPDATEs, so a count of survivors is not a stable
  assertion.** ze-peer is deterministic: `Peer.runMessageLoop`
  (`internal/test/peer/peer.go`) writes ONE UPDATE per `send-route`. The merging
  is `readAndProcessCoalesced` (`session_coalesce.go`), which appends consecutive
  ipv4/unicast NLRIs with byte-identical attributes and flushes when the read
  buffer drains, so how many messages `applyPrefixCheck` sees depends on read
  timing. It is on by default and switchable with `ze.bgp.reactor.coalesce=false`. A test asserting "exactly
  one route survives an over-limit burst" passed, then failed with 2, and both
  answers were correct behavior. Assert the invariant the feature promises, the
  count never goes PAST the maximum, above a floor a separate in-limit route
  establishes first. A bound alone would be satisfiable by zero; a bound above a
  proven floor is not.
- **One green run is not a green test.** The flake above appeared on the second
  run of a test that had just been promoted on its first. Run a new functional
  test three times before it goes into the suite somebody else has to keep green.
- **A test can look like coverage it does not have.**
  `TestPrefixPerFamilyIsolation` proves per-family COUNTERS and its name invites
  the reader to assume per-family ENFORCEMENT. Read what a test asserts, never
  what it is called.

## Files

- `internal/component/bgp/reactor/peersettings.go` -- three fields became maps;
  `PrefixTeardownFor`, `PrefixIdleTimeoutFor`, `PrefixReconnectFor`,
  `OldestPrefixUpdated`, `PrefixReconnectMode`
- `internal/component/bgp/reactor/config_prefix.go` (new) --
  `parsePrefixLimitFromFamily` and `parsePrefixReconnect`, moved out of
  `config.go`, which the 1000-line gate forced
- `internal/component/bgp/reactor/session_prefix.go`, `session.go`,
  `session_read.go` -- `prefixLimitError`, `prefixTeardownCause`, and the
  offending family carried to the peer
- `internal/component/bgp/reactor/peer_run.go` -- `prefixReconnectDecision`,
  `holdDownAfterPrefixTeardown`
- `internal/component/bgp/reactor/peer.go`, `peer_stats.go`, `session_health.go`
  -- `PeerStateIdleHold`
- `internal/component/bgp/reactor/reactor_dynamic.go`, `reactor_api.go`,
  `reactor_peers.go` -- map cloning and the aggregated date
- `internal/component/bgp/yang/ze-bgp-conf.yang` -- the `reconnect` leaf
- `internal/component/bgp/reactor/config_prefix_test.go`,
  `session_prefix_family_test.go`, `peer_prefix_hold_test.go` (new)
- `test/plugin/prefix-warn-only-drops-nlri.ci` -- the RIB read that proves the
  excess NLRI is not installed
- `test/parse/prefix-per-family-parse.ci`,
  `test/parse/prefix-reconnect-invalid.ci`,
  `test/plugin/prefix-per-family-teardown.ci`,
  `test/plugin/prefix-teardown-holds-peer-down.ci`,
  `test/plugin/prefix-teardown-reconnect-backoff.ci`
- `test/interop/scenarios/46-max-prefix-per-family-frr/`
- `docs/features/configuration.md`, `docs/guide/configuration.md`
