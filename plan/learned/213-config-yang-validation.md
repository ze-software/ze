# 213 — Config YANG Validation

## Objective

Wire the YANG validator into the config reader pipeline so that all config values are validated against their YANG schema types at parse time.

## Decisions

- Used `ConfigValidator` interface (not `*yang.Validator` directly) to avoid creating a package-level import cycle between `internal/component/config` and `internal/component/config/yang`.
- YANG validation of numeric types required special handling: JSON `Unmarshal` produces `float64` for all numbers, but YANG distinguishes `int64`/`uint64` — the validator must accept `float64` and convert.

## Patterns

- Interface at the consumer site (not the provider site) is the standard Ze pattern for breaking import cycles — the consumer defines the interface it needs, the provider satisfies it without knowing.

## Gotchas

- JSON `Unmarshal` always produces `float64` for numeric values, regardless of the target YANG type (`int8`, `uint32`, etc.). The validator must explicitly handle the `float64 → integer` conversion path or it will reject all numeric config values.

## Files

- `internal/component/config/yang/validator.go` — YANG validator implementation
- `internal/component/config/reader.go` — validator wired into parse pipeline
