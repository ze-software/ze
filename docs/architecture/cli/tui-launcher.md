# The no-argument TUI launcher

`ze` with no arguments in a terminal opens a hierarchical menu. Outside a
terminal it prints the static help and exits 1, unchanged, so scripts and pipes
see what they always saw. `le` opens the same menu over its derived command
tree when stdin and stdout are both terminals, and prints its root page
otherwise (`docs/contributing/running-commands.md`, "At a terminal").

<!-- source: internal/component/cli/menu/menu.go -- the menu model, LevelOf, NewDeclared, DeclaredLevelOf, scrolling, filtering and styles -->
<!-- source: internal/component/cli/menu/prompt.go -- Ask, the one-value prompt with Tab completion -->
<!-- source: cmd/ze/tui_menu.go -- buildTopLevel, ze's first screen, and runTUILauncher -->
<!-- source: internal/le/le/root/prompt.go -- openMenu, le's first screen from Tree() -->
<!-- source: cmd/ze/ze_core_dispatch.go -- the TTY check and the branch into the menu -->

## Decisions

- **One menu, two programs.** The model lives in
  `internal/component/cli/menu`, outside `internal/le`, because a shipped `ze`
  never links le (`TestNormalZeLinksNoInternalLe`). It knows nothing of either
  program: the caller hands it the first screen, the tree every deeper level is
  built from, and the program name the first breadcrumb shows, and runs the
  chosen path its own way. ze builds its first screen from its verbs and root
  sections; le builds it from the first words of its derived tree.
- **The prompt reuses the menu's terminal model** rather than adding a line
  editor. It asks for one required value, Tab completes from a function the
  caller supplies, Enter on an empty line keeps asking, and Ctrl-C or Ctrl-D
  answers a cancel the caller turns into "nothing ran". It draws inline, not on
  the alternate screen, so the question and the answer stay in the scrollback.
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
- **A tree that declares its text opens with `NewDeclared`.** le's derived
  tree gives every node a one-line ShortHelp and a long Description, so its
  menu shows each item's ShortHelp whole in the right column, and prints the
  entered node's Description, wrapped to the window, under the breadcrumb
  (`DeclaredLevelOf`). ze's YANG help is not declared that way, so ze opens
  `New` and `LevelOf`, whose right column is the derived line, and a level with
  no Description prints no header: ze's screen is byte-identical.
- **Scrolling is offset tracking against the window size**, not the framework's
  viewport component. The menu is a list, not a document.
- Navigation keys: Left and Backspace go back one level, Escape goes back or
  quits at the top, and q quits from any level while no filter is typed. Once a
  filter is typed, q is a letter of it.
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
