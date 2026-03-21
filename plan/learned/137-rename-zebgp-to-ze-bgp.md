# 137 — Rename zebgp to ze bgp

## Objective

Rename the binary from `zebgp` to `ze` with `bgp` as a subcommand, update the Go module path from `codeberg.org/thomas-mangin/zebgp` to `codeberg.org/thomas-mangin/ze`, and update env var prefixes from `zebgp.*` to `ze.bgp.*`.

## Decisions

Mechanical rename, no design decisions.

## Patterns

- CLI structure: `cmd/ze/main.go` (top-level dispatcher) + `cmd/ze/bgp/` (BGP subcommand handlers). ExaBGP subcommand lives at `cmd/ze/exabgp/`.
- Helpers named with dashes: `ze-peer`, `ze-test`.

## Gotchas

- Bulk sed replacements on `.md` files and `.ci` files risk corrupting spec files that show "before" state — the old format in examples gets replaced too.

## Files

- `go.mod` — module path updated
- `cmd/ze/main.go` — new top-level entry point
- `cmd/ze/exabgp/main.go` — ExaBGP subcommand
- `internal/component/config/env/env.go` — prefix `zebgp.*` → `ze.bgp.*`
- 511 files modified total
