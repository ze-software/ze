# 096 — ExaBGP Migration Tool

## Objective
Build a CLI tool (`ze bgp config migrate`) to convert ExaBGP config files to ZeBGP format, with a companion `check` command to detect migration need.

## Decisions
- SAFI detection priority: `rd` present → `mpls-vpn` (RFC 4364, SAFI 128); `label` only → `nlri-mpls` (RFC 8277, SAFI 4); multicast range → `multicast`; else `unicast`.
- `--in-place` creates a backup before overwriting; `--dry-run` prints to stdout only.
- Python API scripts explicitly out of scope — they require manual migration.

## Patterns
- RFC constraint comments inline at detection logic: `// RFC 8277: label-only = SAFI 4`, `// RFC 4364: RD present = SAFI 128`.
- Migration functions are pure transformers (input text → output text), making them easy to test with table-driven tests before implementing.

## Gotchas
- RFC 8277 (labeled unicast, SAFI 4) and RFC 4364 (L3VPN, SAFI 128) share label syntax. Detection must check `rd` presence first.
- `migrate.go` transforms peer/neighbor names; `static.go` extracts route blocks; `api.go` handles API syntax — keep concerns separated.

## Files
- `internal/component/config/migration/helpers.go`, `static.go`, `api.go`, `migrate.go`, `detect.go` — migration logic
- `cmd/ze/bgp/config_check.go`, `cmd/ze/bgp/config_migrate.go` — CLI entry points
