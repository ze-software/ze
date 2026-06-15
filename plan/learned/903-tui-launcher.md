# 903 -- TUI Launcher

## Context

Running `ze` with no arguments printed static help text and exited 1, even in an interactive terminal. The flat command list was longer than most terminal heights. The goal was a hierarchical BubbleTea menu with drill-down navigation, scrolling, and sections as first-level grouping, while preserving the static help fallback for scripts and pipes.

## Decisions

- Chose a hierarchical navigation stack over a flat list, because the full command set exceeds terminal height. Top level shows YANG verbs (show, set, clear, ...) plus section entries (Operations, Configuration, System). Selecting a non-terminal item pushes a new level; selecting a terminal item dispatches.
- Derived top-level items from existing data sources (yangVerbs map + registry.ListRootBySection) over a hardcoded favorites list, satisfying the derive-not-hardcode rule.
- Used the YANG command tree (cli.BuildCommandTree + command.FindNode) for sub-command drill-down, because the tree already models the full command hierarchy with terminal/non-terminal classification via Children.
- Terminal detection: a command with no Children in the YANG tree is terminal. Registry commands use Meta.ResolveSubs() as the heuristic (non-empty Subs means sub-commands exist).
- Back navigation: Left/Backspace go back one level. Esc goes back or quits at top. q quits from anywhere.
- Scrolling via offset tracking and WindowSizeMsg, not BubbleTea viewport component, because the menu is a simple list not a document.
- Applied color-system.md semantic roles: structure(205) for title, identity(73) for names, muted(241) for descriptions and hints, context(33) for breadcrumb.

## Consequences

- `ze` in a terminal is now interactive by default. Scripts using `ze` with no args are unaffected (non-TTY path unchanged: static help, exit 1).
- New commands registered via RegisterRoot/RegisterRootHandler appear in their section automatically.
- YANG verb sub-commands are browsable (show -> bgp -> peer -> list), dispatching only at leaf nodes.
- Multi-word command paths dispatched via strings.Fields on the joined path.

## Gotchas

- BubbleTea v2 changed View() to return tea.View (struct with Content field), not string.
- textbuf.PadRight(s, width) takes the string to pad as first arg; must pad before styling to avoid ANSI codes affecting column width.
- The derive-not-hardcode hook blocks literal string slices that look like command names. Use existing maps (yangVerbs) or registry queries instead.

## Files

- `cmd/ze/tui_menu.go` -- hierarchical BubbleTea model, level builders, scrolling
- `cmd/ze/tui_menu_test.go` -- 16 unit tests (navigation, drill-down, back, scroll, empty menu)
- `cmd/ze/ze_core_dispatch.go` -- TTY check + TUI branch, strings.Fields for multi-word paths
- `test/ui/tui-noargs-nontty-fallback.ci` -- functional test for non-TTY fallback
- `docs/guide/command-reference.md` -- documented no-arg behavior
- `docs/features.md` -- feature row
- `docs/architecture/cli/color-system.md` -- launcher surface mapping
