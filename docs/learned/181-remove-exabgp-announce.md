# 181 — Remove ExaBGP Announce Syntax

## Objective

Remove ExaBGP `announce {}` / `static {}` blocks from YANG and Go schema, replacing them
with the native `update { attribute {} nlri {} }` syntax implemented in spec 180.

## Decisions

- ExaBGP parsing code stays in `internal/exabgp/` — never in the main config package.
- `ze bgp config migrate` tries native YANG first; falls back to `exabgp.ParseExaBGPConfig()`.
- Template `inherit` expansion: collect templates first, merge into neighbors, drop template block from output.
- Link-local-nexthop capability (code 77) deferred — draft not yet RFC.

## Patterns

- Migration isolation: all ExaBGP-aware code lives in one package (`internal/exabgp/`), named with `ExaBGP`/`exabgp` prefix.
- Complex NLRI families (FlowSpec, MVPN, MUP) removed from config syntax; announced via API commands instead.
- ExaBGP test suite used as compatibility oracle — drove discovery of bugs (ADD-PATH path-id, LOCAL_PREF EBGP, route-refresh cap).

## Gotchas

- ~600 lines of Go schema and ~400 lines of YANG removed — large but mechanical.
- `inherit` field was silently dropped by `copySimpleFields()` before fix; templates were copied but never expanded.
- Migration output included `announce {}` which Ze YANG then rejected — ordering issue: remove must precede migrate.
- 11 functional tests timeout because complex-family config syntax was not implemented; resolved by switching those tests to API command delivery.

## Files

- `internal/component/bgp/schema/ze-bgp.yang` — ExaBGP blocks removed
- `internal/component/config/bgp.go` — `LegacyBGPSchema()` and helper functions deleted
- `internal/exabgp/migrate.go` — template expansion added
- `cmd/ze/bgp/config_migrate.go` — ExaBGP fallback wired in
- `test/exabgp-compat/` — ExaBGP QA harness (38 CI tests)
