# 918 -- exabgp-compat-sync

## Context

Ze's ExaBGP compatibility test data (`test/exabgp-compat/encoding/`) had diverged from ExaBGP's `qa/encoding/` over several months. A systematic comparison (2026-06-17) found missing test cases, missing JSON expectations, one real code bug (IPv6 extended community rendered as opaque attribute), and a wire data difference in EVPN labels. The goal was to synchronize both sides and document what each repo should port from the other.

## Decisions

- Chose to match ExaBGP's `router-id` JSON placement (flat key under `neighbor`) over a sub-dict, because compatibility with ExaBGP's format is the purpose of these tests.
- Chose to implement IPv6 extended community JSON via a `RegisterJSONFormatter` registry rather than adding another case to the hardcoded switch, because the existing attribute JSON rendering was already overdue for plugin-driven extensibility.
- Chose not to port `unknown-message` encoding test (BGP message type 255) because it is niche and low value.
- Determined ze is correct on EVPN label S-bit (`01` = bottom-of-stack) over ExaBGP's `00`, per RFC 3032 + RFC 7432.

## Consequences

- `:json:` lines in exabgp-compat `.ci` files are documentation only (not validated by the test runner). Updates keep them accurate for reference but do not affect test outcomes.
- ExaBGP needs fixes ported back: `conf-flow-bytes`, `conf-flow-packets`, `conf-paths-limit` tests, 18 decode test binaries, and the EVPN S-bit bug.
- The `RegisterJSONFormatter` registry pattern enables BGP plugins to own their attribute JSON rendering without touching the central format package.

## Gotchas

- A-4 assumption was wrong: the exabgp-compat test runner (`bin/bgp`) skips `:json:` lines entirely (they are reference documentation, not assertions). This means JSON correctness is not actually tested by the encoding suite.
- SR-Policy encoding test was added but the ExaBGP config -> ze migration for SR-Policy routes is incomplete (route builder support missing). Filed as `spec-sr-policy-migration.md`.

## Files

- `internal/core/bgp/attribute/json.go` -- RegisterJSONFormatter registry
- `internal/component/bgp/plugins/filter_community/json.go` -- community JSON formatters
- `internal/exabgp/bridge/bridge_event.go` -- router-id in bridge JSON
- `test/exabgp-compat/encoding/*.ci` -- all 29 files with `:json:` lines updated
- `test/exabgp-compat/encoding/conf-sr-policy.ci` -- new SR-Policy test
- `test/exabgp-compat/encoding/extended-nexthop.ci` -- new extended-nexthop test
