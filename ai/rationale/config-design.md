# Config Design Rationale

Why: `ai/rules/config-design.md`

## Why No Version Numbers
- All config changes must be machine-transformable
- Migration framework handles old -> new conversion
- Detect old syntax -> transform to new
- Version numbers encourage "check version, branch logic" which accumulates forever

## Why Fail on Unknown
- Unknown key at any level -> fail with clear error
- No silent ignore of typos or deprecated fields
- Forces explicit migration, prevents subtle misconfiguration
- Error message should suggest closest valid key if possible
