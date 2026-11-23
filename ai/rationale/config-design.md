# Config Design Rationale

Why: `ai/rules/config.md`

## Why No Version Numbers
- The infrastructure that converts a config written in an older format of Ze's own syntax stays (`ze config migrate`, the start-time evolution, the relocated-block and retired-leaf registries); the conversion of one format change is written only once Ze has a stable release, and a YANG change converts the repository's configs in the same work (`ai/META-DECISIONS.md`)
- Version numbers encourage "check version, branch logic" which accumulates forever

## Why Fail on Unknown
- Unknown key at any level -> fail with clear error
- No silent ignore of typos or deprecated fields
- Forces an explicit edit, prevents subtle misconfiguration
- Error message should suggest closest valid key if possible
