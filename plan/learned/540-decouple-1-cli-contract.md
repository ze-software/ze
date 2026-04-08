# 540 -- decouple-1-cli-contract

## Context

SSH and web components imported cli directly for editor, model, completer, and data types. The dependency direction was correct (presentation depends on domain) but the coupling was deep and concrete -- ssh created cli.Model instances, web held *cli.Editor fields. Removing or replacing cli would require changing ssh and web.

## Decisions

- Introduced cli/contract/ with plain types and interfaces (over putting interfaces in cli itself) because the contract must have zero component imports to be a true leaf dependency.
- Made cli types (LoginWarning, MonitorSession, MonitorFactory, DashboardFactory, CommitResult, Conflict, Completion) type aliases of contract types (over keeping separate types and converting) to eliminate all conversion code.
- Used SessionModelFactory func type for SSH (over an Editor interface) because SSH's model creation involved 90 lines of bubbletea wiring that doesn't reduce to a clean interface.
- Used Editor interface with any-typed Tree() for web (over importing config in contract) to keep contract free of component imports. Type assertions happen at the adapter boundary.
- Made cli.Completer.SetTree accept any (over a wrapper) so *cli.Completer satisfies contract.Completer directly without an adapter.

## Consequences

- SSH production code has zero cli imports. Model creation logic lives in hub/session_factory.go.
- Web production code has zero cli imports. EditorManager accepts contract.Editor via factory injection.
- Hub provides adapters (editorAdapter, editSessionFactory) that bridge cli concrete types to contract interfaces.
- cli.Completer.SetTree signature changed from *config.Tree to any -- callers passing *config.Tree are unaffected, but the compiler no longer catches wrong types at the call site.
- Test files in ssh and web still import cli directly (test code is excluded from the AC checks).

## Gotchas

- Type aliases for CommitResult/Conflict triggered the exhaustive linter on existing switch statements over ConflictType, because the type moved to a new package. Required nolint annotations.
- EditorManager.NewEditorManager gained two factory parameters, breaking all test callers. A test_factories_test.go helper centralizes the test adapter.
- The editorAdapter duplicates between hub and web test code. Acceptable because hub is production and web tests are test-only.

## Files

- `internal/component/cli/contract/contract.go` -- interfaces and plain types
- `internal/component/cli/warnings.go` -- LoginWarning alias
- `internal/component/cli/model_monitor.go` -- MonitorSession/MonitorFactory aliases
- `internal/component/cli/model_dashboard.go` -- DashboardFactory alias
- `internal/component/cli/completer.go` -- Completion alias, SetTree(any)
- `internal/component/cli/editor_draft.go` -- CommitResult/Conflict/ConflictType aliases
- `internal/component/ssh/session.go` -- replaced with factory delegation
- `internal/component/ssh/ssh.go` -- contract types, getter methods
- `internal/component/web/editor.go` -- contract.Editor via factory
- `internal/component/web/cli.go` -- contract.Completer
- `cmd/ze/hub/session_factory.go` -- SSH model creation logic
- `cmd/ze/hub/editor_adapter.go` -- Editor/EditSession adapters
