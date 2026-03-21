# 367 — Editor Set Path Handling

## Objective
Fix `set` command to correctly handle YANG list paths, reject invalid paths, and enforce `config false`.

## Decisions
- Added `validateTokenPath` to walk full token paths (with list keys) and enforce list key presence
- `ValidateValueAtPath` now rejects unknown paths (entry==nil) and non-leaf paths
- `isConfigFalse` walks all ancestors checking `Entry.Config == TSFalse` (RFC 7950 §7.21.1 inheritance)
- Both `validateTokenPath` and `ValidateValueAtPath` are called — intentional safety-in-depth

## Patterns
- `getEntry` skips list keys silently (schema-only navigation) vs `validateTokenPath` enforces key presence (token-level validation)
- Detecting missing list key: if next token after list IS in `entry.Dir`, it's a schema child (user forgot key), not a key value
- goyang `Entry.Config` is a `TriState` — `TSFalse` means `config false`, inherited by children

## Gotchas
- `cmdSet` positional path splitting (last=value, second-to-last=key) actually works correctly because `walkOrCreate` in `editor.go` already handles list keys. The fix was adding validation, not changing the splitting.
- `getEntry(["bgp", "peer", "hold-time"])` succeeds because it treats "hold-time" as a direct schema child of "peer" (not as a key value) — this is correct for schema navigation but wrong for config paths without a key
- `isConfigFalse` does O(n²) schema walks for each prefix — acceptable for short config paths

## Files
- `internal/component/config/editor/completer.go` — ValidateValueAtPath, isConfigFalse, validateTokenPath
- `internal/component/config/editor/model_commands.go` — cmdSet calls validateTokenPath
- `test/editor/commands/set-through-list.et`, `set-config-false-rejected.et`
