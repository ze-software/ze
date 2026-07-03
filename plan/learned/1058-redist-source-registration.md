# 1058 -- redist-source-registration

## Context

`redistribute { destination bgp { import static } }` was a config that passed
`ze config validate` but did nothing at runtime. Three coupled defects, surfaced while
trying to make the quickstart's first example non-"magical" (drop `process rib`, lead with
declarative redistribution):

- **Bug A:** the static plugin emitted redistribute route events (a producer) but never
  called `configredist.RegisterSource`, so `import static` was rejected at runtime with
  `unknown source "static"`. Every other producer registered a source; static was the lone
  gap.
- **Bug B1:** the YANG walker (`walkTree`) validated a list entry's *children* but never the
  list *key*, so `ze:validate "redistribute-source"` on the `import` key was dead code —
  even `import totalgarbage123` validated clean.
- **Bug B2 (coupled):** `ze config validate` imports plugins (init runs) but never starts
  their engines, and connected/kernel/l2tp registered their source only at engine-run. So a
  B1 fix alone would have falsely rejected every non-BGP `import` at validate time.

## Decisions

- **Register redistribute sources at `init()`, not at engine-run.** Source registration is
  pure metadata and must be visible to `ze config validate`, which imports plugins but does
  not run them. BGP/ospf/isis/ike already did this; connected/kernel/l2tp did not. This is
  the general rule: *anything the config validator checks against a registry must be
  populated at import time, not engine start.*
- **Fix B1 in the walker, not with a redistribute-specific check.** Validating the list key
  against its key-leaf schema (type + `ze:validate`) closes the whole class — it now covers
  `family`, static `hop`/`route`, ospf `nbma-neighbor`, and redistribute `import`. A
  bespoke redistribute validator would have left every other keyed list unguarded.
- **`validateListKey` guards.** Skips composite keys, missing key leaves, and keys also
  stored as a child (avoids a duplicate error). Runs both `validateEntry` (type) and
  `applyCustomValidators` (ze:validate), mirroring how leaf values are validated. Numeric
  keys are safe: `validateUnsigned`/`validateSigned` already parse string-encoded values.
- **Parity test enumerates the registry** (`redistevents.Producers()` vs registered
  sources) rather than a hardcoded protocol list, so a future producer added without a
  source fails a test, not an operator in production.

## Consequences

- `redistribute { destination bgp { import static } }` now works end to end; the
  isis-redist-frr and l2tp-02 interop configs (migrated to current syntax) validate.
- `ze config validate` now rejects unregistered/misspelled redistribute sources (and any
  bad key in the 6 validated keyed-lists) instead of deferring to a silent runtime no-op.
- A 366-config sweep found **zero** B1-attributable false-rejections: the newly-validated
  keys (IPs, prefixes, families) were already valid in every shipped config.

## Gotchas

- **B1 without B2 is a regression trap.** They must ship together; otherwise validate falsely
  rejects run-time-registered sources.
- **Loading a new YANG module into a test binary can break `TestCheckAllValidatorsRegistered_AllPresent`.**
  It hardcoded a validator subset; importing `config/redistribute/yang` in a config test
  exposed the missing `redistribute-source`. Fixed by pointing the test at production
  `RegisterValidators`. Lesson: that test must use the real registry, not a hand-maintained list.
- **`destination <protocol>` is still unvalidated at validate time** — consumers (bgp/ospf/isis)
  register at engine-run, so the consumer set is empty during validate. Catching a bad
  destination needs the same init-move for consumers (future work).
- The macOS dev host cannot start the BGP plugin tier (fails at plugin declare-registration
  for any config), so wire-level advertisement is proven via config-resolution + dispatch
  unit tests locally and the Linux interop harness in CI.

## Files

- `internal/plugins/static/register.go` -- `registerStaticSources()` at init (Bug A, prior commit)
- `internal/component/config/yang/validator.go` -- `validateListKey` + walkTree call (Bug B1)
- `internal/plugins/connected/register.go`, `internal/plugins/kernel/register.go`,
  `internal/component/l2tp/register.go` -- source registration moved to init (Bug B2)
- `internal/component/config/redistribute_source_validate_test.go` -- B1 test
- `internal/component/plugin/all/redistribute_parity_test.go` -- parity + init-registration tests
- `internal/component/config/validator_yang_test.go` -- use production `RegisterValidators`
- `test/interop/scenarios/isis-redist-frr/ze.conf`, `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/ze.conf` -- migrated to current syntax
