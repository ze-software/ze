# Reviewed claims: session b2ec5abb

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| `docs/exabgp/exabgp-differences.md` | `internal/exabgp/bridge/bridge_wire_json.go` | `WireUpdateToExabgpJSON` | `69b67933a2` | the hunk renames the callee ZebgpToExabgpJSON to the package-private zebgpToExabgpJSON and updates a comment; the overlay of AS_PATH, extended communities and unknown attributes from wire bytes is untouched |
| `docs/architecture/api/wire-format.md` | `internal/component/config/yang/register.go` | `RegisterModule` | `b5ca30fee1` | the hunk adds advanceModulesGeneration after the append, so DefaultLoader can key its cached resolution on the module set; each module is still stamped with its calling package through callsite.Package(2), which is the owner OwnerPrefix reads, as the page says |
| `docs/architecture/firewall/firewall-irr.md` | `internal/component/resolve/irr/store/store.go` | `Refresh` | `5bafb8e4d0` | the hunk moves the body into RefreshLive and makes Refresh call RefreshLive then Persist; the never-replace-with-none guard and the StaleSince marking stay in the resolve and commit path both entry points share, so the guard still lives in the store for every consumer, as the page says |
| `docs/architecture/firewall/firewall-irr.md` | `internal/component/resolve/irr/store/store.go` | `markStale` | `5bafb8e4d0` | the hunk replaces the inline durable write with a mark for the next Persist; the entry still keeps its cached prefixes and gains StaleSince, as the page says |
| `docs/architecture/firewall/firewall-irr.md` | `internal/component/resolve/irr/store/store.go` | `Purge` | `5bafb8e4d0` | the hunk also clears the entry's pending-persist mark; Purge still removes the entry from memory and its durable copy, which is the operator removal the page describes |
| `docs/guide/logging.md` | `internal/component/plugin/process/process.go` | `monitorCmd` | `3e8a71b905` | the hunk records the exit on the plugin's group before the reap and closes a reaped channel for the exit supervisor; the stderr pipe is still parent-owned and drainStderrRelay still reads it after the reap under stderrDrainGrace, so the relay keeps the last line, as the page says |
| `docs/functional-tests.md` | `internal/test/fixture/ui_fixture_le_placement_answers.go` | `lePlacementAnswers` | `f6efaf67dc` | the hunk derives the expected check count from the gate's JSON instead of hardcoding five; the fixture still runs the placement gates against the real checkout and rejects repository violations, as the page says |
| `docs/exabgp/exabgp-differences.md` | `internal/exabgp/bridge/bridge.go` | `translateAndForward` | `69b67933a2` | the hunk renames the callee ZebgpToExabgpJSON to the package-private zebgpToExabgpJSON; translateAndForward still receives Ze's JSON event as text with no wire to read |
