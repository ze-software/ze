# 654 -- config-3-deactivate

## Context

Ze already had Junos-style `inactive:` deactivation working end-to-end
for containers, list entries, and leaf-list values (from spec-policy-3
/ 541-policy-framework). Three gaps remained: leaves were explicitly
rejected at the parser and TUI command level; there was no one-shot
CLI verb (only the TUI exposed the feature); and the set / single-line
format had no equivalent at all -- a deactivated config round-tripped
through `ze config dump --format set` would silently lose the marker.

The user's design constraint was that deactivation must be engine-level
and transparent to schema authors: no per-schema `ze:deactivable`
annotation, every YANG node deactivatable by default.

## Decisions

- **Tree-level state for leaf inactivity**, in `inactiveValues map[string]bool` on `*Tree`. The block-format leaf-inactive marker lives on the parent Tree, not in the leaf value -- chosen over encoding `"inactive:" + value` in the value string to avoid collision risk and keep the value verbatim. The Tree-level state for leaves is structurally different from the auto-injected `inactive` schema leaf used by containers/list-entries, but both schemes round-trip cleanly so the asymmetry is acceptable. A future refactor could unify both onto the parent-side scheme; deferred (see plan/deferrals.md row 2026-04-25).
- **Universal applyInactive helper** in parser.go, dispatching by node type to either `Tree.SetLeafInactive` (for any leaf-like node) or the existing schema-injected `inactive` leaf (for containers and list entries). Used by parseRoot, parseContainer, and parseListFieldBlock -- one rule across the three structural-statement parse sites.
- **One-shot CLI verbs** `ze config deactivate <file> <path>` and `ze config activate <file> <path>` mirror `cmd_set.go` (flags, NewEditorWithStorage, dry-run, save, daemon notify). The path-resolution dispatch is in cmd_deactivate.go::dispatchDeactivate, using new `Editor.LookupSchemaNode` (terminal-leaf lookup, distinct from `WalkPathWithSchema` which halts at structural nodes) and `Editor.ResolveLeafListValue` (extracted from Model so the CLI verb can route to `DeactivateLeafListValue` without going through the TUI).
- **AC-8 idempotency on the CLI verb**: a re-deactivate on an already-inactive leaf, or a re-activate on an already-active leaf, exits 0 with a "no change" status. The Editor primitive errors so callers can distinguish; the CLI verb maps the sentinel to success for script-friendliness.
- **AC-12 positional-list rejection**: lists with all-leaf children (`nlri`, `nexthop`, `add-path`) skip schema-injected inactive in `yang_schema.go:561`. The CLI verb detects this case via `listNode.Has(InactiveLeafName)` and rejects with a message pointing the user at the parent container. Without the explicit reject, `SetValue(path, "inactive", "true")` would silently store an unknown leaf.
- **Single-keyword set-format**: the set / single-line config format gets one new top-level keyword `inactive <path>` (no `activate` symmetric verb, no `set inactive: <path>` prefix form). The presence of the line declares the path inactive; absence means active. To re-activate via the file you drop the line; via the CLI you use `ze config activate` which clears the marker and re-serializes without the line. This replaces the legacy `set <path> inactive true` round-trip form that leaked the engine-injected leaf into the user-visible file.
- **Container-level inline serialization disabled when any leaf is inactive**: `canInlineContainer` returns false when `inactiveValues` is non-empty, since the `inactive: ` prefix only renders correctly on multi-line statements.

## Consequences

- Every YANG node is deactivatable through both formats: block format uses `inactive: <node>` prefix (existing for structural nodes, new for leaves); set format uses `inactive <path>` keyword on its own line.
- Schema authors do not opt in. No new YANG extension. No grep target for `ze:deactivable` -- it does not exist.
- Round-trip identity: parse->serialize->parse on configs with deactivated leaves yields equal Trees (verified by extending TreeEqual to compare `inactiveValues` and by `TestRoundTripInactiveLeaf` and `TestSetRoundTripInactiveLeaf`).
- `PruneInactive` runs at every documented apply site (`loader.go:111`, `editor.go:365`, `bgp/config/peers.go:48`, `cmd_validate.go:205`); the new leaf branch is invoked from inside `pruneNode` via `tree.pruneInactiveLeaves()` so callers do not change.
- TUI `deactivate <leaf-path>` now succeeds where it previously errored "cannot deactivate a leaf value, use delete instead".
- Existing `set <path> inactive true` lines in old set-format configs still parse for backward compatibility, but freshly serialized configs use the cleaner `inactive <path>` keyword.

## Gotchas

- `Editor.WalkPathWithSchema` halts at leaves (returns nil because `walkSchemaNode` returns `(false, nil, 0)` for leaf-like types); using it for terminal-leaf classification fails. New `Editor.LookupSchemaNode` walks schema-only and is the right helper for "what kind of node is at this path".
- `capability` is nested inside `session`, not directly under `peer`. AC-12 test originally failed with "no such path" because the path was wrong; the actual path is `bgp peer <name> session capability nexthop <key>`.
- TreeEqual in serialize_test.go did not initially compare `inactiveValues`. Round-trip tests would have given false positives; extended TreeEqual to include the new map.
- Set-format `extra values` walk (`serializeSetExtraValues`) needed to suppress the engine's `inactive` marker explicitly: when the test schema does not include the auto-injected leaf (manual `List()` constructions in tests), the marker would have leaked through as an extra value.
- The set-format `inactive` keyword has no `activate` counterpart in the file syntax (per user direction). Re-activation through the file is "remove the line"; through the CLI it is `ze config activate`.

## Files

- `internal/component/config/tree.go` -- new `inactiveValues map[string]bool` field, `SetLeafInactive`/`IsLeafInactive`/`ClearLeafInactive`, `pruneInactiveLeaves` (lock-respecting, called from `pruneNode`), Clone copies inactiveValues
- `internal/component/config/parser.go` -- `parseRoot` accepts `inactive: <name>` sugar at root; new `applyInactive` helper dispatches by node type for leaves vs containers vs list entries
- `internal/component/config/parser_list.go` -- `parseListFieldBlock` replaces leaf-rejection warning with `applyInactive` call
- `internal/component/config/serialize.go` -- LeafNode / MultiLeafNode / BracketLeafListNode / ValueOrArrayNode emit `inactive: ` prefix when `tree.inactiveValues[name]`; `canInlineContainer` refuses inline when any leaf is inactive
- `internal/component/config/serialize_annotated.go`, `serialize_blame.go` -- mirror leaf-prefix emission for the annotated and blame views
- `internal/component/config/prune.go` -- `pruneNode` calls `tree.pruneInactiveLeaves()` before walking structural children
- `internal/component/config/setparser.go` -- new `cmdInactive` keyword; `parseInactive` walks path through schema and tree, dispatching by node type; rejects `activate` keyword (single-keyword design)
- `internal/component/config/serialize_set.go` -- `emitSetInactive` for leaves and `emitSetInactiveStructural` for containers / list entries; suppresses legacy `set <path> inactive true` round-trip lines and `extra values` leakage of the injected marker
- `internal/component/config/serialize_test.go` -- `TreeEqual` extended to compare `inactiveValues`
- `internal/component/cli/editor.go` -- new `WalkPathWithSchema`, `LookupSchemaNode` (terminal-leaf), `ResolveLeafListValue` (extracted from Model)
- `internal/component/cli/editor_commands.go` -- new `DeactivateLeaf` / `ActivateLeaf` wrappers around `Tree.SetLeafInactive`/`ClearLeafInactive`
- `internal/component/cli/model_commands.go` -- TUI `cmdDeactivate` / `cmdActivate` route leaf paths to the new Editor methods; positional-list constraint message refined; `resolveLeafListValue` simplified to delegate to `Editor.ResolveLeafListValue`
- `cmd/ze/config/cmd_deactivate.go` -- new file, one-shot CLI verbs sharing the same `runDeactivateLike` flow; AC-8 idempotency via `errAlreadyInState` sentinel; AC-12 positional-list reject
- `cmd/ze/config/main.go`, `cmd/ze/config/register.go` -- register the new verbs in `storageHandlers` and the help / `Subs` listings
- `docs/architecture/config/syntax.md` -- new "Inactive prefix" section formalizes the previously-undocumented grammar; set-format `inactive <path>` line documented
- `docs/guide/config-deactivate.md` -- new user guide
- `docs/features.md` -- new entry for the feature
- `docs/guide/command-reference.md` -- new CLI verbs listed
- `test/parse/cli-config-deactivate-leaf.ci`, `cli-config-deactivate-container.ci`, `cli-config-activate.ci`, `parse-inactive-leaf.ci` -- functional tests
