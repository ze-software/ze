# Config Design

## No Version Numbers

Config should NOT contain version numbers. Design for migration.

- No `version N;` fields in config syntax
- All config changes must be machine-transformable
- Migration framework handles old→new conversion
- Detect old syntax → transform to new

## Fail on Unknown

ZeBGP MUST reject configs with unknown variables/blocks.

- Unknown key at any level → fail with clear error
- No silent ignore of typos or deprecated fields
- Forces explicit migration, prevents subtle misconfiguration
- Error message should suggest closest valid key if possible
