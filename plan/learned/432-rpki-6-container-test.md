# 432 -- rpki-6-container-test

## Context

Ze's RPKI implementation had 17 functional tests using a mock RTR server (`ze-rtr-mock`) with deterministic VRP data. The mock tests validate logic but cannot catch wire-protocol incompatibilities with real RTR cache servers or validation errors that only appear at real-world scale (~300K VRPs). The goal was a live integration test using stayrtr with actual RPKI data from Cloudflare's rpki.json endpoint.

## Decisions

- **Go integration test over Python interop framework** -- the interop framework is designed for multi-daemon BGP peer testing; this only needs a single container (stayrtr) and tests the RTR client + ROA cache directly via exported Go APIs. No BGP peer needed.
- **Build tag `live` over `container`** -- names the distinguishing property (requires live internet data) not the mechanism. Keeps the test out of `make ze-verify` without any risk of accidental inclusion.
- **`require` in AC-1 over `assert`** -- discovered via deep review that `assert` in a subtest allows downstream subtests to run against an empty cache, producing false passes (NotFound == empty cache). Using `require` in the parent function aborts the entire test on sync failure.
- **First-line port parsing over LastIndex** -- `docker port` returns multiple lines on dual-stack hosts; the original `LastIndex(":")` worked by coincidence. Explicit first-line extraction via `strings.Cut` is robust.
- **Cloudflare rpki.json as data source over RIPE or routinator** -- single HTTP fetch, well-known stable endpoint, contains the complete global ROA set.

## Consequences

- `make ze-live-test` is available for developers with Docker + internet to validate RTR wire compatibility against real infrastructure.
- The test exercises the full RTR protocol path (TCP dial, Reset Query, PDU parsing, End of Data, ApplyDelta) and RFC 6811 validation at production scale.
- IPv4 and IPv6 validation paths are both covered.
- Test prefixes (Cloudflare, Google) are among the most stable ROAs on the internet, but if a ROA is withdrawn the test fails -- this is intentional (useful signal, not a flake).
- No production code was changed; this is purely additive test infrastructure.

## Gotchas

- Deep review caught that `assert` vs `require` in subtests is dangerous when subtests share state: a failed precondition (empty cache) can produce false passes in downstream subtests that check for absence (NotFound).
- `docker port` output format varies by Docker version and host networking (single-line vs multi-line). Always parse only the first line.
- The `<-done` channel after `close(stopCh)` can block indefinitely if the RTR session is stuck in a network read. A select with timeout prevents test hangs.
- stayrtr needs 10-30s to fetch rpki.json before serving RTR; the test has generous timeouts (60s TCP, 90s sync, 180s total) to accommodate slow networks.

## Files

- `internal/component/bgp/plugins/rpki/rpki_live_test.go` (created) -- live integration test
- `Makefile` (modified) -- `ze-live-test` umbrella + `ze-live-rpki-test` target + `.PHONY` + help text
- `docs/functional-tests.md` (modified) -- live test documentation section
