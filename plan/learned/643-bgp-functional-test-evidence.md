# 643 -- bgp-functional-test-evidence

## Context

The spec started with two honesty gaps in BGP test evidence:

- `internal/component/bgp/config/loader_test.go` skipped `TestParseAllConfigFiles`
  entirely even though `etc/ze/bgp/` still ships many example configs.
- Several functional tests claimed AC coverage for egress or wire behavior that
  their actual `.ci` assertions never exercised.

The job was to make the evidence honest, not to make every blocked path pass.

## Decisions

- **Convert a curated parser subset instead of pretending the whole tree is covered.**
  The parser-focused `etc/ze/bgp/parse-*.conf` fixtures were the smallest useful
  subset to convert to current native syntax. `parse-multisession.conf` stayed
  excluded because it documents the removed ExaBGP `multi-session` capability.
- **Classify every shipped BGP fixture explicitly.**
  `TestParseAllConfigFiles` now walks `etc/ze/bgp/`, parses the curated native
  subset, and fails on any unclassified new fixture. Legacy `api-*`, `conf-*`,
  `extended-nexthop.conf`, `example-healthcheck.conf`, `unknown-message.conf`,
  and `parse-multisession.conf` are excluded with a named reason.
- **Downgrade overclaiming functional tests instead of leaving TODO optimism.**
  `community-strip.ci` and `role-otc-egress-filter.ci` now say plainly that they
  are blocked harness-wiring tests, not AC proof.
- **Make partial tests say what they actually prove.**
  `bgp-rs-fastpath-ebgp-shared.ci`, `llgr-readvertise.ci`, and
  `nexthop-self-ipv6-forward.ci` now distinguish wiring/config proof from the
  still-deferred wire assertions.
- **Document the real proof rules once.**
  `docs/functional-tests.md` now says that egress claims require an actual
  forward path (`ForwardUpdate()` / `ForwardUpdatesDirect()`) plus a deterministic
  destination-side assertion, and it records the known single-`ze-peer`
  multi-IP timing limitation.

## Consequences

- `TestParseAllConfigFiles` is now a real signal. It exercises eight native
  example fixtures and no longer hides the rest of `etc/ze/bgp/` behind an
  unconditional skip.
- Reviewers reading the touched `.ci` files no longer get false confidence from
  `VALIDATES:` lines that outrun the assertions.
- The remaining blockers are explicit:
  `community-strip.ci` and `role-otc-egress-filter.ci` still need a forwarding
  plugin plus destination-wire assertions to become full evidence.
- The repo now has one documented place that explains why “two peers exist” is
  not enough to claim egress-filter coverage.

## Verification

- `env GOCACHE=/tmp/ze-gocache go test ./internal/component/bgp/config -run TestParseAllConfigFiles -count=1` -- pass
- `bin/ze-test bgp plugin --port 20000 88 247 w 148 169` -- pass
  This required running outside the sandbox because the functional runner binds
  localhost sockets.
- `make ze-lint` -- blocked by another concurrent `golangci-lint` already
  running in the shared worktree
- `make ze-unit-test` -- not attributable to this spec in the current dirty
  worktree: it surfaced unrelated existing failures in `cmd/ze`
  (`TestAvailablePlugins`, `TestInvokePluginForkPath`) plus sandbox-blocked
  localhost-listener tests in `cmd/ze/config` and `cmd/ze/hub`

## Files

- `internal/component/bgp/config/loader_test.go`
- `etc/ze/bgp/parse-community.conf`
- `etc/ze/bgp/parse-dual-neighbor.conf`
- `etc/ze/bgp/parse-md5.conf`
- `etc/ze/bgp/parse-multiple-process.conf`
- `etc/ze/bgp/parse-process.conf`
- `etc/ze/bgp/parse-simple-v4.conf`
- `etc/ze/bgp/parse-simple-v6.conf`
- `etc/ze/bgp/parse-ttl.conf`
- `test/plugin/community-strip.ci`
- `test/plugin/role-otc-egress-filter.ci`
- `test/plugin/bgp-rs-fastpath-ebgp-shared.ci`
- `test/plugin/llgr-readvertise.ci`
- `test/plugin/nexthop-self-ipv6-forward.ci`
- `docs/functional-tests.md`
