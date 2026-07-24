# 1268 -- ordered-leaf-list-as-path-labels

## Context

A prior fix (`cae320d33`) made the config brace parser accumulate repeated
leaf-list statements via a new `Tree.AppendSlice`, which **deduplicates** members
as a YANG set (RFC 7950 sec 7.7). That silently collapsed AS_PATH: `as-path
[30740 30740 30740 30740 30740 30740 30740]` became `[30740]`, dropping the
prepends. It broke two functional tests -- `test/encode/l2vpn.ci` (bgp encode 31)
and `test/exabgp-compat/encoding/conf-l2vpn.ci` (exabgp 21) -- because AS_PATH is
an ordered SEQUENCE where duplicate ASNs are load-bearing (RFC 4271 sec 5.1.2
path prepending), not a set. MPLS label stacks (RFC 8277) have the same shape.

## Decisions

- **Model "ordered sequence, duplicates preserved" as a per-node property, not a
  parser-wide behavior change.** Added a `ze:ordered` YANG extension
  (`ze-extensions.yang`) that sets `ValueOrArrayNode.Ordered` /
  `BracketLeafListNode.Ordered`; the parser routes those nodes to a new
  `Tree.AppendSequence` (append, never dedup) while every other leaf-list keeps
  set semantics via `AppendSlice`. Chosen over removing dedup globally (which a
  new `name-server` "duplicate collapses" test correctly forbids) or changing
  as-path to a plain string leaf (breaks the `[ ... ]` config syntax).
- **Marked `as-path` and `labels` `ze:ordered` in every live schema.** Two:
  `ze-bgp-conf.yang` (direct leaf-list) and the exabgp migration schema
  `exabgp.yang` (a `value-or-array` leaf reused via `uses route-attributes`).
  The extension survives `uses` expansion, verified by
  `internal/exabgp/migration/aspath_ordered_test.go`. Also marked the dead
  `ze-types.yang` `route-attributes` grouping to close a latent trap.
- **Fail closed on ambiguous deactivation.** Deactivation (`inactiveMembers`) is
  value-keyed, so an ordered leaf-list that repeats a value cannot deactivate one
  copy without blanking all of them. `DeactivateMultiValue` now rejects
  deactivating a repeated value; the parse path surfaces it as an error for
  ordered nodes. Found by adversarial review, not by the failing tests.

## Consequences

- YANG authors: a leaf-list whose duplicates are meaningful (ordered sequence)
  MUST carry `ze:ordered`; the config parser deduplicates by default. Documented
  in `docs/architecture/config/yang-config-design.md`.
- The exabgp migration round-trips config through the ze parser, so the same
  extension fixes both the native BGP path and the exabgp compat path.
- `Tree.AppendSequence` is the ordered-sequence counterpart to `AppendSlice`;
  both share `appendMembersLocked(dedup bool)`.

## Gotchas

- The bug reached the exabgp compat suite through a SECOND schema (`exabgp.yang`),
  not just `ze-bgp-conf.yang` -- a single-schema fix leaves the migration path red.
- `ze-test`'s incremental build did not always re-embed a changed `//go:embed
  *.yang`; a clean `make ze` was needed to see the exabgp fix. Verify a YANG embed
  change with a clean rebuild, not just an isolated `go test` (which recompiles).
- A hand-built schema node in a parser unit test bypasses `getOrderedExtension`,
  so it does not gate the YANG->schema wiring; the exabgp schema test and the
  `l2vpn.ci` end-to-end test are what actually gate it.

## Files

- `internal/component/config/tree.go` (AppendSequence, appendMembersLocked, DeactivateMultiValue guard)
- `internal/component/config/schema.go`, `yang_schema.go`, `parser_list.go`, `parser_test.go`
- `internal/component/config/yang/modules/ze-extensions.yang`, `ze-types.yang`
- `internal/component/bgp/yang/ze-bgp-conf.yang`
- `internal/exabgp/migration/exabgp.yang`, `aspath_ordered_test.go`
- `docs/architecture/config/yang-config-design.md`
