# Derive, Never Hardcode

**BLOCKING:** If enumerated data has a canonical source (registry,
map, typed enum, list function), DERIVE every
display/help/error/usage/doc string from it. No second hardcoded copy.

Detail: `ai/rationale/derive-not-hardcode.md`.

## Rule

| Surface | Pull from |
|---------|----------|
| Error messages ("valid: a,b,c") | registry `List()` / `Keys()` |
| `cmdregistry.Meta.Subs` / `Description` | derived at `init()` |
| CLI `flag.NewFlagSet.Usage` | derived at call time |
| Help / `--help` output | derived |
| `.ci` test expectations listing names | test pulls the list |
| Generated docs | `make ze-inventory` |

If the lookup is awkward, add a `List()` accessor. Do not paste the
list twice.

## Structured Data, Not Pre-Formatted Strings

Handlers return typed values; the display layer owns rendering.

| Do | Don't |
|----|-------|
| Return typed struct (`*CPUInfo`, `[]NICInfo`) | Return `"CPU: Intel N100, 4 cores"` |
| Numeric fields (`*-bytes`, `*-mhz`) | Human string (`"8.0 GiB"`) |
| Kebab-case JSON with typed fields | YAML-ish text blocks |
| Let `| table` / web UI render | Render text in handler |

Principle: **registry/struct is truth; every surface is a view of it**.

## Mechanical Check

- Grep for duplicates before committing (`grep -rn "FOO\|BAR\|BAZ" .claude docs cmd internal test plan`).
- Output shape: "could pipe framework and web UI both render without re-parsing?" No -> emit typed field instead.

## When a Hardcoded List Is OK

- Canonical registry doesn't exist yet; you are creating it (same commit as consumer).
- Test fixture deliberately asserts against drift.
- YANG / JSON Schema where the enum IS the contract.

Otherwise: derive.
