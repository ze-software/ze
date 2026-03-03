# 036 — Extended Community Hex Format

## Objective

Add ExaBGP-compatible `0x...` hex format parsing for extended communities in config, fixing test Z (vpn).

## Decisions

- Hex format is identified by `0x` prefix with no colons — unambiguous from the existing `target:ASN:NN` format.
- Strict length validation: hex format must be exactly 16 hex chars (8 bytes) — the fixed wire size for an extended community.
- Implementation placed as a prefix check before the existing parser in `parseOneExtCommunity()`.

## Patterns

None beyond standard hex parsing using the already-imported `encoding/hex` package.

## Gotchas

None.

## Files

- `internal/config/routeattr.go` — `parseExtCommunityHex()` + prefix check in `parseOneExtCommunity()`
- `internal/config/routeattr_test.go` — `TestParseExtendedCommunityHex`
