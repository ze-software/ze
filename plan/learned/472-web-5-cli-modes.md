# 472 -- CLI Modes (Bar and Terminal)

## Context
Not all users prefer GUI forms. The web interface needed a CLI escape hatch -- a persistent input bar accepting the same command grammar as the SSH CLI. Two modes: integrated (commands drive the GUI) and terminal (full text CLI over HTTPS replacing the content area).

## Decisions
- Persistent CLI bar at bottom over CLI as a separate page -- always available without navigation
- Integrated mode as default over terminal mode -- GUI users get CLI shortcuts, CLI users toggle to terminal
- URL as source of truth for context over Editor.contextPath -- HTMX's `HX-Push-Url` keeps browser URL, CLI prompt, and breadcrumb in sync
- Command tokenizer with quote support over naive space split -- handles values containing spaces

## Consequences
- CLI bar commands produce HTMX multi-target responses (content + breadcrumb OOB + notification OOB) -- more complex responses than simple page loads
- Terminal mode reuses Editor methods for command execution -- same underlying infrastructure, different rendering
- Autocomplete endpoint returns JSON (not HTML) -- consumed by client-side JS for dropdown rendering

## Gotchas
- web-5 depends on web-3 (not just web-2) -- CLI bar's set/delete/commit commands need the EditorManager from web-3
- Completer thread safety: `Complete()` is read-only but `SetTree()` is not -- web component must ensure no concurrent `SetTree()` during autocomplete
- Terminal mode scrollback is server-side per request (not accumulated) -- each command returns its output, client accumulates

## Files
- `internal/component/web/cli.go`, `cli_test.go`
- `internal/component/web/templates/terminal.html`, `cli_bar.html`
