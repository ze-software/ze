# 552 -- cmd-4 Phase 2: Per-Prefix NLRI Rewriting

## Context

cmd-4 phase 1 (`plan/learned/548-cmd-4-prefix-filter.md`) shipped the
`bgp-filter-prefix` plugin in STRICT whole-update mode: if any prefix in a
multi-prefix UPDATE was denied by the prefix-list, the entire UPDATE was
rejected. For single-prefix UPDATEs (the common production case) strict and
per-prefix modes are identical, but multi-prefix UPDATEs -- common for EBGP
customer sessions, MRT replay, and route collectors -- lose the accepted
prefixes as collateral damage. The handover to dest-3 described this as "a
medium-size spec (~500 lines of code + tests)", flagged that per-prefix
splitting needs the modify action plus a way to express "keep X, drop Y" in
the text protocol, and listed three open questions:

1. Text-protocol delta shape for "withdraw prefix X but keep Y" (none of the
   existing filter_chain.applyFilterDelta paths handle NLRI-level rewrites).
2. Wire-level conversion from the modified text back to a new UPDATE
   payload (textDeltaToModOps handles attributes via AttrOp but skips NLRI).
3. How to express "I rejected N of M prefixes; here's the remaining text
   form" in `FilterUpdateOutput` (new field vs. restructured delta).

## Decisions

- **Text-protocol delta carries the full new nlri block, not a diff.** The
  filter plugin returns `action=modify` with `Update = "nlri ipv4/unicast
  add <accepted-prefix-1> <accepted-prefix-2> ..."`. applyFilterDelta's
  existing logic (parseFilterAttrs + maps.Copy) already replaces the nlri
  key wholesale when the delta contains one, so no new delta format is
  needed. Chose this over a "rejected-prefixes" list field on
  FilterUpdateOutput because the rebuilt nlri block is the single source of
  truth for what survives -- having two representations invites drift.
- **Engine re-encodes the accepted subset, not the plugin.** The filter
  plugin does NOT compute wire bytes. Instead, the engine's
  `extractLegacyNLRIOverride` in `internal/component/bgp/reactor/filter_delta.go`
  walks the modified filter text, pulls out the `ipv4/unicast add` block,
  and encodes each prefix token as `<len><addr-bytes>`. This keeps the
  plugin-side work to pure text manipulation (trivial to add in other filter
  plugins) while putting the wire-level complexity in one place the engine
  already owns (next to textDeltaToModOps for attribute rewrites).
- **IPv4 legacy NLRI only, v1.** extractLegacyNLRIOverride returns nil for
  any NLRI block that is not `ipv4/unicast add`. MP_REACH rewriting
  (IPv6/unicast, mpls-label, etc.) is out of scope in v1: per dest-2, filter
  plugins that need per-NLRI decisions on non-CIDR or non-legacy-IPv4
  families must declare `raw=true` and rewrite the MP_REACH attribute
  themselves. The prefix-list filter plugin is a text-mode consumer and
  does not cover those families in v1.
- **buildModifiedPayload gets a new nlriOverride parameter.** Rather than
  chaining an NLRI rewrite step after buildModifiedPayload, the override is
  plumbed directly into step 8 of the progressive build. When nlriOverride
  is non-nil, step 8 writes the override bytes instead of copying
  `payload[attrEnd:]` verbatim. A zero-length non-nil override means "drop
  every legacy NLRI prefix" (all-reject case). A nil override preserves the
  original copy path. Both callers
  (`reactor_notify.go` ingress + `reactor_api_forward.go` egress) and every
  test caller were updated to pass nil by default; only the ingress path
  computes a real override today.
- **filter_prefix.partitionUpdate sits alongside the existing evaluateUpdate.**
  The old strict evaluator is retained and still used by the accept/reject
  branches; the new partition walker is called unconditionally, and the
  output is used to decide among accept / reject / modify. Keeping both
  keeps the hot-path cost identical for single-prefix UPDATEs (still a
  single walk) while unlocking the modify action for multi-prefix ones.
- **Rejected: NLRI-level delta with explicit `withdraw` verb in filter text.**
  Would have required a new tokeniser path, a new delta merge rule in
  applyFilterDelta, and new wire encoding that converts "withdraw" to
  Withdrawn Routes section bytes. Returning the full new nlri block reuses
  every existing mechanism: parseFilterAttrs already captures nlri as one
  glob, maps.Copy already replaces it, extractLegacyNLRIOverride is the
  only new translation step.
- **Rejected: moving MP_REACH NLRI rewriting into this spec.** Requires
  editing the MP_REACH_NLRI attribute value (attr code 14) mid-stream, which
  means walking attrs again inside buildModifiedPayload, re-serialising the
  attribute header, and fixing attr_len. Much larger change, and the
  handover itself deferred it to "whatever consumes it first".

## Consequences

- `bgp-filter-prefix` now runs per-prefix on multi-prefix UPDATEs, returning
  one of four outcomes: accept (empty or all-accept), reject (all-reject or
  parse-error), modify (mixed), or the implicit accept for empty nlri.
  Callers need no API change -- action=modify flows through the existing
  PolicyFilterChain path.
- The wire UPDATE forwarded downstream (and into adj-rib-in) for a mixed
  UPDATE now carries only the accepted prefixes. Routes that were denied
  never reach downstream filter stages or plugin consumers. adj-rib-in stats
  and bgp-rib storage see the filtered set, not the original.
- `buildModifiedPayload` signature changed. Every call site was updated.
  Tests that exercise the pre-phase-2 behavior (nlriOverride == nil) are
  unchanged in semantics. The test count rose by one (prefix-filter-modify-
  partial) and the six existing cmd-4 filter tests still pass.
- Filter plugins that need per-prefix rewriting for non-IPv4-unicast
  families need to declare `raw=true` and rewrite the wire themselves, per
  the dest-2 contract. v1 does not expose a text-mode modify path for those
  families.
- Parse errors in the filter text prefix list still fail-closed: if any
  prefix token fails ParsePrefix, partitionUpdate sets hadParseError and
  handleFilterUpdate returns action=reject for the whole update (same
  behaviour as the original strict evaluator).

## Gotchas

- **extractLegacyNLRIOverride must return a non-nil empty slice (not nil)
  when every prefix is denied but the call still wants a rewrite.** The
  caller distinguishes "no override needed" (nil) from "replace the NLRI
  section with nothing" (zero-length non-nil) when deciding whether to
  invoke buildModifiedPayload. Forgetting this causes the original NLRI to
  be copied through unchanged, silently defeating the all-deny case on
  multi-prefix UPDATEs.
- **splitNLRIBlocks splits on " nlri " (with leading and trailing spaces)**
  because the concatenated nlri field can contain multiple blocks (e.g.,
  legacy IPv4 + MP_REACH IPv6). A naive split on "nlri" would also match
  the leading tokens of the field itself. The helper is intentionally
  local to filter_delta.go because the reactor and filter_prefix packages
  cannot import each other, and duplicating a 15-line parser is cheaper
  than extracting a new shared package.
- **The 3-prefix reproducer encodes all three prefixes as /24** so each
  prefix consumes exactly 4 bytes (1 length + 3 data). Under 1-byte
  alignment, the three-prefix hex is
  `180A0001 180A0002 18C0A801` (10.0.1.0/24, 10.0.2.0/24, 192.168.1.0/24).
  When writing similar tests, remember that `(bits+7)/8` rounds up: a /20
  encodes as length + 3 bytes, a /5 encodes as length + 1 byte.
- **Test observer uses dest-1 approved pattern.** The new
  `prefix-filter-modify-partial.ci` deliberately does NOT do observer-side
  assertion introspection (that is the dest-1 antipattern). Instead it uses
  a minimal observer whose only job is to dispatch `daemon shutdown` after
  3 seconds, and every assertion is an `expect=stderr:pattern=` on the
  production `prefix-list modify` log line (accepted=N, rejected=M, the
  filter name). This keeps the test resistant to silent observer failures.
- **The `filter_prefix` package cannot log at WARN without flag tricks.**
  `logger().Info("prefix-list modify", ...)` intentionally lands at INFO,
  not WARN, so the test harness needs `option=env:var=ze.log.bgp.filter.
  prefix:value=info` to see it. Any test copying this pattern must include
  that env-var line or its assertions silently never fire.

## Files

- `internal/component/bgp/plugins/filter_prefix/match.go` -- new
  `partitionResult` type and `partitionUpdate` walker; file-level godoc
  updated to document both evaluation modes
- `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` -- new
  `filterActionModify` const, rewritten `handleFilterUpdate` that picks
  among accept / reject / modify based on the partition, new
  `buildModifyDelta` and local `joinWords` helpers
- `internal/component/bgp/plugins/filter_prefix/match_test.go` --
  `TestPartitionUpdate` covering all-accept, all-reject, mixed, empty,
  header-only, and malformed-prefix cases
- `internal/component/bgp/reactor/filter_delta.go` -- new
  `extractLegacyNLRIOverride`, `extractIPv4UnicastAddBlock`,
  `extractNLRIField`, `findNLRIBlock`, and `splitNLRIBlocks` helpers
- `internal/component/bgp/reactor/filter_delta_test.go` -- new
  `TestExtractLegacyNLRIOverride` covering unchanged / subset / all-denied /
  zero-length / sub-byte / ipv6 / mixed-families / no-nlri / malformed paths
- `internal/component/bgp/reactor/forward_build.go` --
  `buildModifiedPayload` gains a new `nlriOverride []byte` parameter;
  step 8 writes the override when non-nil
- `internal/component/bgp/reactor/reactor_notify.go` -- ingress path
  computes `nlriOverride` from the filter text delta and passes it into
  `buildModifiedPayload`
- `internal/component/bgp/reactor/reactor_api_forward.go`,
  `filter_delta_handlers_test.go`, `filter_delta_test.go`,
  `forward_build_test.go` -- updated to the new `buildModifiedPayload`
  signature (passing nil for nlriOverride; none of the existing paths use
  it yet)
- `test/plugin/prefix-filter-modify-partial.ci` -- new functional test
  covering the mixed-prefix partition end-to-end via the production
  `prefix-list modify` log line
