# 443 -- command-inventory

## Context

Ze had no way to list all registered commands programmatically. The command tree in `docs/architecture/api/commands.md` was hand-maintained and drifted from reality. Users had no discovery mechanism beyond `help`. The goal was a build-time tool (like `make ze-inventory`) that queries real registrations and outputs a complete, verb-classified command list.

## Decisions

- Build-time Go script (`scripts/command_inventory.go`) over runtime RPC, matching the `inventory.go` pattern. No daemon needed.
- Blank imports of individual cmd packages over `plugin/all`, because `plugin/all` pulls in the full SDK which may have compilation issues from in-progress work in other sessions.
- Added `StreamingPrefixes()` accessor to `handler.go` over exposing the private map, keeping the registry encapsulated.
- Verb classification uses first path component (`show`, `set`, `del`, `update`, `monitor`) with `-` for unclassified. Simple string match, no registry of verbs.
- TUI-only commands (like `monitor bgp`) are deduplicated against RPC registrations -- if YANG already maps the wire method to the CLI path, no manual entry needed.

## Consequences

- `make ze-command-list` is the authoritative source for the command tree. Documentation can be regenerated from it.
- Adding a new RPC via `RegisterRPCs()` + YANG automatically appears in the output. No manual doc update needed for the command list.
- The script requires blank imports for every cmd package that registers RPCs. New cmd packages must be added to the import list.
- `make ze-command-list-json` enables tooling (CI checks, doc generation, completion scripts).

## Gotchas

- `plugin/all` import chain can break compilation if any plugin has in-progress uncommitted changes (e.g., filter_community adding `handleFilterUpdate` to SDK). Using individual cmd package imports avoids this.
- System commands (`ze-system:*`) need explicit `internal/core/ipc/schema` import for YANG path resolution. Without it, wire methods appear as paths.
- Hook `block-os-exit.sh` blocks `os.Exit()` even in build-time scripts. Used implicit exit code 0 instead.

## Files

- `scripts/command_inventory.go` -- command inventory tool (new)
- `internal/component/plugin/server/handler.go` -- `StreamingPrefixes()` accessor (new)
- `Makefile` -- `ze-command-list` and `ze-command-list-json` targets (new)
