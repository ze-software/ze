# 232 — Editor Tree-Canonical

## Objective
Replace raw text as the config editor's source of truth with `config.Tree`, eliminating brace-counting navigation and text-surgery mutations.

## Decisions
- `treeValid` flag added (not in spec): when a config fails to parse, the editor falls back to text mode gracefully rather than crashing.
- `DeleteByPath()` added as a schema-aware dispatcher — it calls `DeleteValue`, `DeleteContainer`, or `DeleteListEntry` based on the schema node type, simplifying `cmdDelete`.
- `ContentAtPath()` added (partial Part 2 work): tree-based content extraction with text fallback, enabling context-aware display.
- `Tree.Delete(name)` implemented in `setparser.go`, not `parser.go` — the mutation methods live where the mutable setparser data structures are defined.
- Functional `.ci` tests not created: the TUI is not testable via `.ci` format; headless model tests in `testing/headless_test.go` serve this role.

## Patterns
- Editor holds `schema *config.Schema` so `Serialize(tree, schema)` can be called without external context.
- `WorkingContent()` returns `Serialize(tree)` when `treeValid`, enabling zero-friction integration with callers that expect config text.
- All mutations (set, delete) call `completer.SetTree()` afterward to keep tab completion in sync.
- Parts 2 and 3 (subtree serialization, load via tree merge) are cleanly deferrable because Part 1's tree-canonical model is internally consistent.

## Gotchas
- Text-surgery functions (findFullContextPath, filterContentByContextPath, setValueInConfig, etc.) must all be deleted together — leaving any one behind creates split-brain between tree and text state.
- `cmdDelete` was a stub in the original code (marked dirty but did nothing). Tree-canonical implementation made it actually work.

## Files
- `internal/config/setparser.go` — `Tree.Delete(name)` added
- `internal/config/editor/editor.go` — tree-canonical model, WalkPath, SetValue, DeleteValue, DeleteContainer, DeleteListEntry, DeleteByPath, ContentAtPath
- `internal/config/editor/model_commands.go` — all commands rewritten, dead text-surgery code removed
- `internal/config/editor/model_render.go` — `filterContentByContextPath` removed
