# 809 -- pol-3-validation: Unique Filter Names and Plain References

## Context

Ze's policy filter chain required operators to use plugin-prefixed references like `remove-private-as:STRIP` or `bgp-filter-prefix:CUSTOMERS` even when the filter instance name was globally unique. The machinery to resolve plain names already existed (BuildFilterRegistry, canonicalizeFilterRefs), but docs, examples, and `show policy chain` output all taught the prefixed form as primary. This spec made plain names the default operator-facing form.

## Decisions

- Updated docs and examples to use plain names as primary over prefixed forms, because filter instance names are already enforced unique under `bgp policy` and operators should not need to learn plugin internals to write a filter chain.
- Added `policyFilterRef` struct with both `name` (plain) and `canonical` (plugin ref) fields to `show policy chain` output over keeping bare canonical strings, because operators need the friendly name while debuggers still need the plugin dispatch target.
- Used `strconv.Quote` over `fmt.Sprintf` for the direction error message, eliminating the only `fmt.Sprintf` call in the file (project rule: no-sprintf-alloc).
- Kept prefixed forms as accepted escape hatches over removing them, because compatibility and explicit disambiguation have legitimate use cases.

## Consequences

- Operators can now write `filter { export [ STRIP ] }` as the normal form. All filter types (prefix-list, as-path-list, community-match, modify, remove-private-as, aspath-length) support plain names.
- `show policy chain` JSON output changed shape: `import`/`export` arrays now contain `{"name": "STRIP", "canonical": "bgp-filter-remove-private-as:STRIP"}` objects instead of bare strings. Consumers parsing the old shape need updating.
- The `policyFilterRef` type is package-level in `show_policy.go`, available if future `show policy test` (pol-4) needs the same display structure.

## Gotchas

- The `show policy chain` output shape change is backwards-incompatible. Any `.ci` tests or external tools parsing the old `[]string` form will break. The trade-off was judged acceptable because the old output was not yet widely consumed and the new shape is strictly more informative.
- The `toFilterRefs` function strips the first colon-delimited prefix to derive the display name. For `inactive:bgp-filter-prefix:DENY`, it correctly produces `inactive:DENY`. If a filter name itself contains a colon (e.g. from an external plugin), the extraction still works because inactive is stripped first.

## Files

- `internal/component/cmd/show/show_policy.go` -- added `policyFilterRef`, `toFilterRefs`, replaced `fmt.Sprintf` with `strconv.Quote`
- `internal/component/cmd/show/show_policy_test.go` -- added `TestToFilterRefs`
- `internal/component/bgp/config/redistribution_test.go` -- added `TestCanonicalizePlainPrefixRef`, `TestCanonicalizeInactivePlainRef`
- `test/parse/remove-private-as-plain-ref.ci` -- new parse test for plain name remove-private-as
- `docs/guide/plugins.md` -- updated remove-private-as examples to use plain names
- `docs/guide/configuration.md` -- updated prefix-list example and reference forms table
