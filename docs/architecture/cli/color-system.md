# Ze Color System

Design reference for terminal color across TUI editor, help output, autocomplete,
SSH sessions, and diagnostic monitors. The goal is consistency and reduced cognitive
load: the same color always means the same thing, regardless of surface.

## Design Principles

1. **Color encodes meaning, not decoration.** Every color maps to a semantic role.
   If two things share a color, they share a role.
2. **Hierarchy through weight, not hue.** Primary content is normal weight.
   Secondary content is dim. Emphasis is bold. Hue distinguishes category, not
   importance.
3. **Maximum 7 foreground roles.** More than that and the user starts pattern-matching
   colors instead of reading text. Seven is the practical ceiling.
4. **Dark-terminal-first.** The palette targets dark backgrounds (the norm for
   network operators). ANSI 256-color codes, not true-color, for maximum
   terminal compatibility including SSH sessions over constrained links.
5. **Graceful degradation.** 256-color is the design target. 16-color terminals
   get the base ANSI mapping automatically (lipgloss handles this). Monochrome
   terminals get bold/dim only.
6. **Color is off where nobody sees it.** `NO_COLOR` turns it off, then
   `TERM=dumb`, then `ze.log.color`, and otherwise it is on only when the writer
   is a terminal, so a log written to a file carries none. JSON output never
   carries color codes, and color never carries meaning alone: the words say
   the same thing without it (`ai/rules/user-facing-errors.md`).

<!-- source: internal/core/slogutil/color.go -- UseColor -->

## Semantic Color Roles

Seven foreground roles, each with a single purpose across all surfaces.

| Role | Purpose | ANSI 256 | Base ANSI | Visual |
|------|---------|----------|-----------|--------|
| **structure** | Headers, section titles, prompt path, mode indicators | 205 (magenta) | 95 (bright magenta) | Labels that frame the interface |
| **identity** | Commands, keywords, the thing you typed or can type | 73 (cyan) | 96 (bright cyan) | Actionable names |
| **value** | Data values, counts, addresses, ASNs, route counts | 82 (green) | 92 (bright green) | What the system is reporting |
| **caution** | Warnings, flags, non-default state, degraded | 214 (orange) | 93 (bright yellow) | Needs attention, not broken |
| **danger** | Errors, down state, loss, validation failures | 196 (red) | 91 (bright red) | Broken, act now |
| **muted** | Descriptions, secondary text, timestamps, hints | 241 (gray) | 2 (dim) | Safe to skip on fast scan |
| **context** | Navigation breadcrumb, current path, positional info | 33 (blue) | 34 (blue) | Where you are |

Two additional modifiers (not independent roles):

| Modifier | Purpose | How |
|----------|---------|-----|
| **bold** | Emphasis within a role (selected row, active state) | Bold attribute on any role color |
| **background** | Inline validation, selected-row highlight | Paired dark bg (52 for danger, 58 for caution, 6 for selection) |

## Surface Mapping

How each surface uses the seven roles. Read down a column to see
what that role looks like across the interface.

### CLI Help (`ze --help`, `ze bgp --help`)

| Element | Role | Example |
|---------|------|---------|
| Section titles (`Usage:`, `Commands:`) | structure | `Usage:` |
| Command path | identity | `ze bgp` |
| Subcommand/entry names | value | `decode`, `peer` |
| Flag names (`--verbose`) | caution | `--verbose` |
| Placeholders (`<file>`, `[options]`) | muted | `<hex>` |
| Summary descriptions | muted | `BGP protocol tools` |
| Example lines | muted | `ze bgp decode 0x...` |
| Error messages | danger | `unknown command` |
| Hints/suggestions | caution | `did you mean: ...` |

### Interactive Launcher (`ze` with no args)

| Element | Role | Example |
|---------|------|---------|
| Title (`ze`) | structure | `ze` |
| Section headers | structure | `Operations (interact with...)` |
| Command names (unselected) | identity | `doctor` |
| Command names (selected) | identity+bold | `> doctor` |
| Descriptions | muted | `Run health checks` |
| Footer/help | muted | `↑/↓ navigate...` |

### Autocomplete Dropdown

| Element | Role | Example |
|---------|------|---------|
| Box border (`+-- Completions --+`) | muted | `+--...--+` |
| Selected-row prefix | identity+bold | `> ` |
| Completion keyword (command/path) | identity | `neighbor` |
| Completion description | muted | `BGP neighbor configuration` |
| Selected row background | background | cyan bg (6) |
| "... N more" indicator | muted | `... 12 more` |
| Ghost text (inline suggestion) | muted | `neighbor 192.168...` |

### TUI Config Editor

**The prompt is the one place where color carries STATE rather than category**
(owner directive, 2026-08-24). Everywhere else on this page a color names what a
thing IS. On the prompt it names what the session is doing. The operator reads
the mode on the line they are typing. Three states, and failure outranks the
mode:

| State | Color | Why |
|-------|-------|-----|
| Operational mode | context (33), blue | The next command reaches the daemon |
| Configuration mode | value (82), green | The next edit stages, and reaches nothing until commit |
| The last command failed | structure (205), magenta | Clears on the next command that succeeds |

This reuses three hues rather than adding any, so the seven-role ceiling holds.
The producer is `promptColor` (`internal/component/cli/model_render.go`), and
the failure state reads `Model.err`. The breadcrumb keeps the context color in
every state, because it says WHERE the operator is and a failure does not change
that.

| Element | Role | Example |
|---------|------|---------|
| Prompt, operational mode | context (33) | `ze> ` |
| Prompt, configuration mode | value (82) | `ze# `, `ze[...]# ` |
| Prompt, last command failed | structure (205) | `ze> `, `ze# ` |
| Welcome/banner text | caution | `Welcome to ze` |
| Context path (breadcrumb) | context | `[neighbor 192.168.1.1]` |
| Success messages | value | `commit successful` |
| Warning messages | caution | `missing optional field` |
| Error messages | danger | `invalid address` |
| Hint/help text | muted | `press ? for help` |
| Validation error line | danger+bg | red text on dark red bg |
| Validation warning line | caution+bg | yellow text on dark yellow bg |
| Help overlay border | muted | rounded border |

### BGP Dashboard

| Element | Role | Example |
|---------|------|---------|
| Header bar (AS, router-id, uptime) | structure+bold | `AS 65000  rid 1.2.3.4` |
| Column headers | structure+bold | `Peer  ASN  State  Rx` |
| Selected row | bold, and a `> ` marker | no background, and no color of its own, so the state cell keeps its own role color |
| Established state | value | `Established` |
| Degraded state (OpenSent, Active) | caution | `Active` |
| Down/error state (Idle, error) | danger | `Idle` |
| Numeric values (Rx, Tx, Rate) | value | `142856` |
| Footer/status line | muted | `q quit  s sort  / filter` |

### Ping / Traceroute Monitors

| Element | Role | Example |
|---------|------|---------|
| Header (target, interval) | structure+bold | `ping 10.0.0.1 interval 1s` |
| Labels (Sent, Recv, Loss, Min) | muted | `Sent` |
| Values (counts, latency) | value | `42`, `1.23ms` |
| Loss OK (0-5%) | value | `0.0%` |
| Loss warning (5-20%) | caution | `12.5%` |
| Loss bad (>20%) | danger | `35.0%` |
| Footer/help | muted | `q quit` |
| Waiting state | muted | `waiting for data...` |

### Structured Logging

Color spans preserve the structured record byte for byte. Removing ANSI
styling leaves the same keys, values, quotes, separators, and newline as the
plain text record. The RFC 3339 timestamp keeps its original format.

| Span | Meaning | Dark foreground | Light foreground | ANSI fallback |
|------|---------|-----------------|------------------|---------------|
| Keys, including the equals sign | Field boundaries | Blue `#91B8FA` | Blue `#2456A6` | Bright blue |
| Calendar date | Date | Lavender `#C5B5F4` | Violet `#59428C` | Bright magenta |
| Clock and fractional seconds | Time | Cyan `#62D9F4` | Deep cyan `#006174` | Bright cyan |
| `subsystem` and `plugin` values | Component identity | Teal `#83D6C1` | Deep teal `#006A55` | Cyan |
| `WARN` and `degraded` | Needs attention | Bold amber `#FFCB6B` | Bold amber `#805000` | Bold bright yellow |
| `ERROR`, `failed`, and `down` | Failure | Bold coral `#FF8D9A` | Bold crimson `#A42139` | Bold bright red |
| `up`, `healthy`, and `success` | Positive state | Green `#9BDB8B` | Deep green `#326622` | Bright green |
| Timestamp separator and timezone, routine severity, messages, and ordinary values | Record content | Terminal foreground | Terminal foreground | Terminal foreground |

The status words apply to `state` and `status` fields. Ze uses the background
index from `COLORFGBG` as a hint and interprets it using standard ANSI palette
values. On RGB and 256-color terminals, it checks for 4.5:1 contrast against
that estimate after converting the foreground to the supported palette.
When a swatch falls below that threshold, Ze uses the terminal foreground and
keeps the bold weight for warnings and errors. If the hint is unavailable
or the terminal only supports basic colors, Ze uses the terminal's ANSI palette.

Ze selects color per stdout or stderr destination. Use `--color` to force
color for those streams and `--no-color` for plain text. Syslog and kernel
logs use plain text.

<!-- source: internal/core/slogutil/palette.go -- logPaletteFor -->
<!-- source: internal/core/slogutil/color.go -- colorizeLine, valueStyle -->

### SSH Sessions

SSH sessions render the same TUI as local sessions. The Wish middleware
detects terminal capabilities per connection. No separate palette needed.
The 256-color codes degrade automatically via lipgloss/colorprofile.

## Current Code vs. This Spec

Where existing code already matches, and where it diverges.

| Surface | Status | Notes |
|---------|--------|-------|
| Help (`helpfmt`) | Mostly aligned | `styleSubcommand` uses green (value) for entry names. `styleCommand` uses bold cyan (identity). `styleFlag` uses yellow (caution). All match. |
| Interactive launcher | Aligned | structure(205) for title/headers, identity(73) for names, identity+bold(73) for selected, muted(241) for descriptions/hints. |
| Editor styles | Mostly aligned | `promptStyle` 205 = structure. `hintStyle` 73 = identity (should be muted). `contextStyle` 33 = context. `successStyle` 82 = value. Match. |
| Dashboard | Aligned | Green/yellow/red for state = value/caution/danger. Header white+bold = structure. The selected row carries bold and a `> ` marker and no background, so a state cell keeps its own role color when the row is selected. |
| Ping/traceroute | Aligned | Same traffic-light pattern as dashboard. |
| Autocomplete | **Needs work** | No color on dropdown items. No selected-row highlight. Only prefix `> ` distinguishes selection. |
| Logging | Aligned | Severity colors match the role mapping. |

### Migration Path

The autocomplete dropdown is the main gap. Changes:

1. **Selected row**: add cyan background (ANSI 6) to the selected completion.
2. **Completion keyword**: render in identity color (73).
3. **Border**: render in muted color (241).
4. **"... N more"**: render in muted color (241).

The dropdown holds no description column. A menu row is the command name alone,
and the selected candidate's summary is on message line 2, in the muted role
(`docs/architecture/cli/error-surface.md`).

The `hintStyle` in the editor (currently 73/cyan = identity) is used for
"press ? for help" type text, which is semantically muted. Consider
changing to 241, or leave it as-is since hint text is also actionable
guidance.

## Web Interface Mapping

The web UI uses CSS variables, not ANSI codes. The semantic roles should
map to the same visual meaning:

| Terminal Role | CSS Variable | Hex (dark) |
|---------------|-------------|------------|
| structure | `--accent` | #58a6ff |
| identity | `--text-primary` | #c9d1d9 |
| value | `--success` | #3fb950 |
| caution | `--warning` | #d29922 |
| danger | `--error` | #ff6b6b |
| muted | `--text-secondary` | #8b949e |
| context | `--accent` | #58a6ff |

The web palette predates this spec and has its own design language.
This mapping is for reference, not a mandate to change the web CSS.

## Quick Reference Card

For anyone adding a new surface or extending an existing one:

```
structure (205/magenta) -- "what kind of thing am I looking at?"
identity  (73/cyan)     -- "what can I type or act on?"
value     (82/green)    -- "what is the system telling me?"
caution   (214/orange)  -- "something is off, not broken"
danger    (196/red)     -- "something is broken, act now"
muted     (241/gray)    -- "background info, skip on fast scan"
context   (33/blue)     -- "where am I?"
```

When in doubt: if the user needs it to make a decision, it gets a role
color. If they can skip it when scanning, it is muted. If it demands
action, it is danger or caution depending on urgency.
