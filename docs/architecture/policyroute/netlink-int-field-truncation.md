# Netlink `int` Fields Truncate uint32 Config on a 32-bit Build

The vendored netlink bindings type several u32 WIRE fields as a Go `int`:
`netlink.Rule.Table`, `netlink.Route.Table`, `netlink.Route.Priority`. Their
encoders emit the attribute only for a non-negative value.

So on a 32-bit build, a value above `MaxInt32` turns negative and the attribute
is DROPPED WITH NO ERROR. The `ip rule` is installed with `RT_TABLE_UNSPEC` and
the route lands in `RT_TABLE_MAIN` at the kernel default metric.

Policy routing and VRF-style table isolation are security controls, so that
failure direction is fail-open. CodeQL alert 170
(`go/incorrect-integer-conversion`, high) flagged the conversion in
`rules_linux.go`.

## The bound is `math.MaxInt`, not `math.MaxInt32`

<!-- source: internal/plugins/policyroute/config.go -- validateActionTable, maxEncodableTable -->
<!-- source: internal/plugins/policyroute/rules_linux.go -- newIPRule error return -->

Ze ships `linux/amd64` and `linux/arm64` only (`mk/appliance.mk`), where `int` is
64-bit and every uint32 is exact. A fixed 32-bit bound would reject table IDs the
kernel accepts and that work today, which narrows a documented YANG range.
Exact-or-reject says reject what ze cannot deliver exactly, not what it can.

## The bound is a function parameter

<!-- source: internal/plugins/policyroute/netlinkint_linux_generic.go -- maxEncodable constant -->

`validateTableID(id, maxEncodable)`, `validateActionTable(tbl, maxEncodable)` and
`validateRouteMetric(metric, maxEncodable)` take the bound as a parameter, and
the constant is supplied at the call site.

On a 64-bit host the guard is UNREACHABLE, so a test driven from the exported
entry point cannot gate it: deleting the guard leaves every test green. The
parameter lets a unit test pass `MaxInt32` and exercise the 32-bit rejection
deterministically on any host. All three guards were mutation-verified: remove
the guard and the test goes red.

## The guard sits at both layers

Config validation tells the operator at commit time. The netlink call site stops
a future producer from bypassing the validation. `newIPRule` gained an error
return for this. `buildRoute` and `buildRichRoute` already had one.

No shared helper package was added. The duplicated expression is `math.MaxInt`, a
stdlib constant. That is not duplicated knowledge, and a new `internal/core`
package for three lines is not worth the import edge.

## Consequences

- Any future netlink field typed `int` in the bindings and u32 on the wire needs
  the same treatment. Grep for
  `(Table|Priority|Realm|TunID|LinkIndex):\s+int\(`.
- `policyroute` auto-route tables are deliberately NOT guarded at the call site.
  `allocateTable` bounds them to 2000 to 2999, and the allocator is where that
  bound belongs.
- The rejection path can have no functional (`.ci`) coverage, because on a 64-bit
  build no config value reaches it. `test/parse/netlink-int-field-range.ci` pins
  the opposite invariant, that the full uint32 range stays accepted, so the bound
  cannot be quietly narrowed to a 32-bit constant.

## Traps

- **CodeQL only recognises a bounds check against a CONSTANT.** A check against a
  variable does not clear the alert.
- **`uint32(int(v))` round-trips on 64-bit**, so a test asserting "a large value
  is rejected" fails on the dev host. Assert the invariant, "accepted if and only
  if it fits in `int`", not a fixed outcome.
- A `maxNetlinkInt` constant consumed only by a `_linux.go` file must itself live
  in a `//go:build linux` file, or it lints as unused on darwin.
