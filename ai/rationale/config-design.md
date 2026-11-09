# Config Design Rationale

Why: `ai/rules/config.md`

## Why No Version Numbers
- Ze is pre-release: no shipped config exists, so a syntax change converts the repository's configs in the same change and keeps no rewrite path for the old form (`ai/META-DECISIONS.md`)
- Version numbers encourage "check version, branch logic" which accumulates forever

## Why Fail on Unknown
- Unknown key at any level -> fail with clear error
- No silent ignore of typos or deprecated fields
- Forces an explicit edit, prevents subtle misconfiguration
- Error message should suggest closest valid key if possible
