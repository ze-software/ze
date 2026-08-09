# The no-argument TUI launcher

`ze` with no arguments in a terminal opens a hierarchical menu. Outside a
terminal it prints the static help and exits 1, unchanged, so scripts and pipes
see what they always saw.

<!-- source: cmd/ze/tui_menu.go -- the menu model, level builders, and scrolling -->
<!-- source: cmd/ze/ze_core_dispatch.go -- the TTY check and the branch into the menu -->

## Decisions

- **A navigation stack, not a flat list.** The command set is longer than a
  terminal is tall. The top level shows the YANG verbs plus the section
  entries. Selecting a non-terminal item pushes a level; selecting a terminal
  item dispatches.
- **Every level is derived, never hardcoded.** The top level comes from the verb
  map and the registry's section listing. A new root command appears in its
  section with no menu edit. A hardcoded favourites list was rejected, and the
  derive-not-hardcode check blocks literal command-name slices anyway.
- **Drill-down walks the YANG command tree**, which already models the hierarchy
  and already classifies terminal against non-terminal by whether a node has
  children. Registry commands use the presence of sub-commands as the same test.
- **Scrolling is offset tracking against the window size**, not the framework's
  viewport component. The menu is a list, not a document.
- Navigation keys: Left and Backspace go back one level, Escape goes back or
  quits at the top, and q quits from anywhere.
- Colors use the semantic roles in `color-system.md`: structure for the title,
  identity for names, muted for descriptions and hints, context for the
  breadcrumb.

## Traps

- The framework's view method returns a struct with a content field, not a
  string. A version bump changed that signature.
- Pad a string to a column width BEFORE styling it. Styling first puts ANSI
  escape codes inside the measured width, and the columns drift.

## Related

- `color-system.md` for the semantic roles this surface uses
