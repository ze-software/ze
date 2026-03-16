# 293 — YANG Validation

## Objective

Add systematic YANG data validation replacing ad-hoc per-field checks: fix schema (string→enum), add recursive tree walk, add a `ze:validate` extension registry for runtime-determined constraints (address families), and add startup integrity checks.

## Decisions

- YANG native constraints first (enum/pattern/range/length/mandatory) — `ze:validate` is last resort, only for runtime-determined sets (e.g., plugin-registered address families) or semantic checks YANG cannot express
- `ze:validate` functions provide BOTH validation AND completion (returns `[]string` for CLI autocompletion) — same source of truth for both concerns
- Registry location in `internal/yang/` (leaf package, no deps) — validators in other packages register via explicit `RegisterValidators()` call instead of `init()`, allowing registry to be passed explicitly rather than using global mutable state
- Collect ALL errors, never stop at first — users want to see every problem at once
- Fail startup if any `ze:validate` reference lacks a registered function — matches ze's fail-early philosophy
- `Process.Format()` default was already FormatParsed (switch fall-through compensated for wrong FormatHex default) — the schema work revealed this latent constant mismatch

## Patterns

- Extension reading: iterate `entry.Exts`, match keyword, extract argument — same pattern as existing `ze:syntax`, `ze:key-type` extensions
- Editor validator must filter `ErrTypeMissing` during editing (config is always incomplete while being edited) except for `peer-as` which is always mandatory
- `Tree.ToMap()` returns `map[string]string` — YANG validator needs string→number conversion branches alongside typed value handling
- Custom validators on typedef propagate to all uses via RFC 7950 Section 7.3.4

## Gotchas

- `validators_register.go` uses an explicit `RegisterValidators(reg)` function instead of `init()` — calling `init()` would require global state and prevent passing the registry
- `registered-afi` and `large-community-range` validators were planned but not implemented — no current YANG references them (YAGNI)
- Config reader (`reader.go`) was NOT changed to use `ValidateTree` — reader keeps flat `ValidateContainer` for per-block parse-time checks; recursive walk happens at load time via `YANGValidatorWithPlugins`

## Files

- `internal/yang/validator.go` — `ValidateTree`/`walkTree` recursive walk
- `internal/yang/registry.go` — `ValidatorRegistry` with `CustomValidator{ValidateFn, CompleteFn}`
- `internal/component/config/validators.go` — `AddressFamilyValidator`, `NonzeroIPv4Validator`, `CommunityRangeValidator`
- `internal/component/config/validators_register.go` — `RegisterValidators()`
- `internal/component/config/yang_schema.go` — registry creation + integrity check in `YANGValidatorWithPlugins`
- `internal/component/bgp/schema/ze-bgp-conf.yang` — 4 leaves converted string→enum
- `internal/yang/modules/ze-extensions.yang` — added `validate` extension
