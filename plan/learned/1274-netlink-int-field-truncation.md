# 1274 -- netlink int fields truncate uint32 config on a 32-bit build

## Context

CodeQL alert 170 (`go/incorrect-integer-conversion`, high) flagged
`rule.Table = int(r.Table)` in `internal/plugins/policyroute/rules_linux.go`:
a routing-table ID parsed with `strconv.ParseUint(v, 10, 32)` reaching a Go
`int` with no upper bound check. The vendored netlink bindings type several
u32 wire fields as `int` (`netlink.Rule.Table`, `netlink.Route.Table`,
`netlink.Route.Priority`) and their encoders emit the attribute only for
non-negative values, so a value above `MaxInt32` on a 32-bit build turns
negative and the attribute is **dropped without an error**: the ip rule is
installed with `RT_TABLE_UNSPEC`, the route lands in `RT_TABLE_MAIN` at the
kernel default metric. Policy routing and VRF-style table isolation are
security controls, so the failure direction is fail-open.

## Decisions

- Bound against `math.MaxInt`, not a hardcoded `math.MaxInt32`. Ze ships only
  `linux/amd64` and `linux/arm64` (`mk/appliance.mk`), where `int` is 64-bit
  and every uint32 is exact. A fixed 32-bit bound would reject table IDs the
  kernel accepts and that work today, narrowing a documented YANG range.
  `exact-or-reject` says reject what we cannot deliver exactly, not what we can.
- Pass the bound as a **function parameter** (`validateTableID(id, maxEncodable)`,
  `validateActionTable(tbl, maxEncodable)`, `validateRouteMetric(metric, maxEncodable)`)
  with the constant supplied only at the call site. On a 64-bit host the guard
  is unreachable, so a test against the exported entry point cannot gate it:
  deleting the guard leaves every test green. The seam lets a unit test pass
  `MaxInt32` and exercise the 32-bit rejection deterministically on any host.
  All three were mutation-verified (guard removed -> red).
- Guard at both layers: config validation (the operator hears about it at
  commit) and the netlink call site (a future producer cannot bypass it).
  `newIPRule` gained an error return for this; `buildRoute` and
  `buildRichRoute` already had one.
- No shared helper package. The duplicated expression is `math.MaxInt`, a
  stdlib constant, not duplicated knowledge, and a new `internal/core` package
  for three lines was not worth the import edge.

## Consequences

- Any future netlink field typed `int` in the bindings but u32 on the wire
  needs the same treatment. Grep pattern:
  `(Table|Priority|Realm|TunID|LinkIndex):\s+int\(`.
- `policyroute` auto-route tables (`rules_linux.go` autoRoutes) are deliberately
  **not** guarded: `allocateTable` bounds them to 2000-2999 (`marks.go`), and
  `marks_test.go` asserts it. The bound lives at the allocator, not the call site.
- The rejection path has no functional (`.ci`) coverage and cannot have any:
  on a 64-bit build no config value can reach it. `test/parse/netlink-int-field-range.ci`
  pins the opposite invariant, that the full uint32 range stays accepted, so the
  bound cannot be quietly narrowed to a 32-bit constant.

## Gotchas

- CodeQL only recognises bounds checks against a **constant**. A check against
  a variable will not clear the alert.
- `uint32(int(v))` round-trips on 64-bit, so a test asserting "large value
  rejected" fails on the dev host. Assert the invariant ("accepted iff it fits
  in int"), not a fixed outcome.
- `maxNetlinkInt` in `fib/kernel` must live in a `//go:build linux` file. Placed
  in the untagged `richroute.go` it lints as unused on darwin, where the only
  consumer (`nexthop_linux.go`) is excluded.
- Adding `<!-- source: -->` anchors to `docs/` makes `make ze-doc-test` fail
  until `make ze-doc-index` regenerates `ai/CODE-TO-DOCS.md`.

## Files

- `internal/core/routingtable/registry.go` -- `validateTableID` + `maxEncodableTableID`
- `internal/plugins/policyroute/config.go` -- `validateActionTable` + `maxEncodableTable`
- `internal/plugins/policyroute/rules_linux.go` -- `newIPRule` returns an error
- `internal/plugins/static/config.go` -- `validateRouteMetric` + `maxNetlinkInt`
- `internal/plugins/static/backend_linux.go` -- `buildRoute` bounds Table and Metric
- `internal/plugins/fib/kernel/nexthop_linux.go` -- `buildRichRoute` bounds Metric and TableID
- `test/parse/netlink-int-field-range.ci` -- the range-not-narrowed pin
- `docs/guide/policy-routing.md`, `docs/guide/static-routes.md` -- table/metric range
