# Reviewed claims: session b2ec5abb

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/exabgp/exabgp-differences.md` | `internal/exabgp/bridge/bridge_wire_json.go` | `WireUpdateToExabgpJSON` | `69b67933a2` | the hunk renames the callee ZebgpToExabgpJSON to the package-private zebgpToExabgpJSON and updates a comment; the overlay of AS_PATH, extended communities and unknown attributes from wire bytes is untouched |
| `docs/functional-tests.md` | `internal/test/fixture/ui_fixture_le_placement_answers.go` | `lePlacementAnswers` | `f6efaf67dc` | the hunk derives the expected check count from the gate's JSON instead of hardcoding five; the fixture still runs the placement gates against the real checkout and rejects repository violations, as the page says |
| `docs/exabgp/exabgp-differences.md` | `internal/exabgp/bridge/bridge.go` | `translateAndForward` | `69b67933a2` | the hunk renames the callee ZebgpToExabgpJSON to the package-private zebgpToExabgpJSON; translateAndForward still receives Ze's JSON event as text with no wire to read |
