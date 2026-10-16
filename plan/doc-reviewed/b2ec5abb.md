# Reviewed claims: session b2ec5abb

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/exabgp/exabgp-differences.md` | `internal/exabgp/bridge/bridge_wire_json.go` | `WireUpdateToExabgpJSON` | `69b67933a2` | the hunk renames the callee ZebgpToExabgpJSON to the package-private zebgpToExabgpJSON and updates a comment; the overlay of AS_PATH, extended communities and unknown attributes from wire bytes is untouched |
| `docs/exabgp/exabgp-differences.md` | `internal/exabgp/bridge/bridge.go` | `translateAndForward` | `69b67933a2` | the hunk renames the callee ZebgpToExabgpJSON to the package-private zebgpToExabgpJSON; translateAndForward still receives Ze's JSON event as text with no wire to read |
