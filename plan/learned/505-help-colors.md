# 505 -- Structured colored help output

## Context

Ze had 40+ files with inline `fmt.Fprintf(os.Stderr, ...)` help text. Each subcommand hand-formatted its own output with no shared structure and no color. Help text was hard to scan in a terminal, and changing the format required editing every file individually.

## Decisions

- Created `cmd/ze/internal/helpfmt/` package over using an external library (charm/lipgloss) -- the styling is simple ANSI codes, the value is in the structure not the rendering.
- Two-layer const design: color palette (names like `colorBrightCyan`) and role styles (names like `styleCommand = colorBrightCyan`). Change a role's color in one place, like CSS.
- `Page` struct with `Write()` (auto-detects color via `slogutil.UseColor`) and `WriteTo(w, color)` (testable). Subcommands build a struct literal instead of a format string.
- Dynamic column width per section (adapts to longest entry name, min 16) over fixed `%-16s` which broke alignment on long entry names like config subcommands.
- `HelpSection`/`HelpEntry` type names (not `Section`/`Entry`) to avoid collision with `authz.Section`.
- `Software` field on Page for the top-level "ze - ze Software" header, separate from `Summary` used by subcommands. Software text is rendered plain; Summary gets `styleSummary` (dim).
- Flag auto-detection: entries starting with `-` get `styleFlag` (yellow), others get `styleSubcommand` (green). No manual annotation needed.
- `command.HelpEntries()` added to return YANG tree children as data instead of writing directly, so main.go can embed dynamic verbs in a Page struct.

## Consequences

- All help output flows through one renderer. Changing colors, layout, or adding features (e.g. man page generation) requires editing one file.
- `WriteError` and `WriteHint` helpers available for consistent error/hint formatting across subcommands, though most callers still use raw `fmt.Fprintf` for errors.
- The `help_ai.go` machine-readable help was intentionally not migrated -- different purpose, not user-facing styled output.
- Remaining inline `fmt.Fprintf` for help are single-line usage hints in error paths (3 files) -- not help pages.

## Gotchas

- ANSI codes add invisible bytes. `%-16s` padding counts bytes not visible characters, so colored names get under-padded. Fixed by padding the raw name first, then wrapping with color.
- The iface sub-subcommands (`addr.go`, `unit.go`, `create.go`, `delete.go`) were missed by the initial parallel agent migration -- discovered during critical review. Always grep for the pattern after bulk migration.
- `fs.PrintDefaults()` had to be replaced by manually extracting flag definitions into `HelpEntry` items. The flag names and descriptions come from the `fs.Bool`/`fs.String` calls in the same function.
- Some subcommands have post-help prose (notes, format descriptions) that doesn't fit the Page struct. These remain as `fmt.Fprintf` calls after `p.Write()`.

## Files

- `cmd/ze/internal/helpfmt/helpfmt.go` -- Page, HelpSection, HelpEntry types, role-based rendering
- `cmd/ze/internal/helpfmt/helpfmt_test.go` -- 9 unit tests
- `internal/component/command/help.go` -- added HelpEntries() for YANG tree integration
- 36 files under `cmd/ze/` -- migrated from inline fmt.Fprintf to helpfmt.Page
- `test/parse/help-bgp.ci`, `test/parse/help-no-color.ci` -- functional tests
