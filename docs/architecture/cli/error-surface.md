---
title: One Error Surface
description: Where a fault appears, for every command and on every surface.
category: architecture
---
# One Error Surface

A command states its own subject. It does not state whether Ze is well.

This is one rule, and it holds for every command, every live view, and every
surface Ze renders to.

## The rule

**A command's output area holds facts about the thing the command asked
about. A fault goes to the error zone.**

The error zone is one place per surface:

| Surface | Where a fault appears |
|---------|-----------------------|
| CLI, ordinary command | line 1 of the message area, above the prompt (`Model.feedbackLine`) |
| CLI, live view (`monitor bgp`, `ping`, `traceroute`) | the same zone, rendered under the view |
| Web | the page's own error region, from the same value |

An operator learns one place. What they learned holds for the next command.

## Why the rule exists

`monitor bgp` used to print a second header line. It read `connected` when the
CLI's last poll of the daemon succeeded, and the poll's error text when it did
not.

Three things were wrong with it.

The word said nothing in its healthy state. It sat directly under
`peers 3/3`, where a reader takes it for a statement about the BGP sessions.
The peer rows already said `established` three times.

The word was not about BGP. It reported the CLI's own transport to the daemon,
inside a command whose subject is the routing protocol.

The fault had nowhere else to go, so the header grew a place for it. Every view
that did the same grew a different place, and an operator had to learn each one.

## What a view owes

A live view answers `problem(m *Model) string` on the `activeView` interface
(`internal/component/cli/view_registry.go`). It returns the fault, or `""`.

The view does NOT render it. `Model.View`
(`internal/component/cli/model_render.go`) renders what `problem` answers, in
the shared zone, in the error style. A view that paints its own error line is
the defect this rule names.

## Every command resolves

**A command MUST tell the operator that it finished, and what it did.** Silence
after a command is a question the operator cannot answer: did it work, is it
still running, did it do nothing?

The confirmation goes to line 1 of the message area (`Model.feedbackLine`). It
is the same zone the fault would have used.

**The healthy state is written in the muted role. A fault is written in the
danger role.** Both roles are defined once, in
`docs/architecture/cli/color-system.md`, which is the authority for every color
this document names. Salience comes from the ROLE, not from the line being
empty.

| State | Role | Why |
|-------|------|-----|
| Healthy | muted (gray 241) | Safe to skip on a fast scan. The text is there for a reader who looks, and for nobody else |
| Fault | danger (red 196) | Broken, act now. It is the only color in a quiet zone, so it is what the eye lands on |

Lack of color IS the message. A reader who wants to know that a command worked
reads the gray line. A reader who does not care never notices it, and the same
zone turning red is unmissable because nothing around it competes.

This is why `connected` failed, and the failure was in the color rather than in
the word. `dashConnStyle` painted it `Color("2")`, the **value** role, which
`docs/architecture/cli/color-system.md` reserves for "what the system is
reporting". A word in the data color, sitting under `peers 3/3`, is read as
data about the peers. In the muted role it is a line the curious CAN read and
everyone else skips.

**Prefer an outcome to a fixed word where there is one to state.** `committed 3
changes` answers more than `OK`, and it costs the same line. But a fixed word
in the muted role is not the defect a fixed word in a loud color is.

**A live view confirms with freshness, not with a word.** `monitor bgp` and
`traceroute` are continuous, so `Last update: 0s ago` is the confirmation: it
changes every second, and it goes stale visibly when the data stops. A view
that shows fresh data is a view that is working.

## The message area is two rows

The message area above the prompt is two lines, and each row has one owner. An
operator learns which row answers which question, and learns it once.

| Row | Owner | What it carries |
|-----|-------|-----------------|
| 1 | `Model.feedbackLine` | the error, the command result, the welcome banner |
| 2 | `Model.warningLine` | help about the command being typed |

Row 1 is the error zone. Row 2 is help. The two are different rows, so nothing
on row 2 can hide a fault. The completion menu and an error are on screen
together, and each stays where the operator expects it.

Row 2 has four occupants and one order. The first occupant that has text wins
the row.

| Order | Occupant | When it has text |
|-------|----------|------------------|
| 1 | the `?` hint | `?` was pressed on a candidate, or the input is invalid |
| 2 | the selected candidate's summary | the completion menu is open |
| 3 | the validation hint | the config holds an error or a warning |
| 4 | the idle banner | nothing above it applies |

The menu row shows the command name alone, so row 2 says what the selected name
does. `Model.warningText` READS the summary from the selection, so an arrow key
moves it and no key handler writes it. The summary renders whole. A declared
summary is a sentence its author wrote, and this row cuts none of it.

The summary is written in the muted role, which is the role a description
wears.

Row 2 is ONE row, and `oneRow` holds it to one. It strips the escape bytes. It
then folds every whitespace run into a single space. No word is lost, and no
newline survives. A second line would draw a row the view did not count. The
cursor would then sit one row above the prompt.

`./le docvalid help-shape` refuses a newline in a summary declared in this
tree. It reads the source. A plugin declares its own summary over the wire, and
no gate reads that one. The bound is applied where the text is drawn.

<!-- source: internal/component/cli/model_render.go -- messageLines, feedbackLine, warningText, oneRow -->

## What this rule does not say

It does not say a command hides a state it is asked about. `waiting for data`
is a fact about the BGP dashboard's own subject, so it stays in the header. A
peer in `idle` is a BGP fact, and it stays in the peer table, in its own color.

The test is the subject, not the severity: ask what the operator typed, and
whether the line answers it.

## How an error reads

An error answers what failed, why, and what to do next. `ai/rules/cli.md` states
the obligation; this is the shape it takes.

| Rule | Why |
|------|-----|
| Lowercase start, no trailing punctuation, single line | Go convention; errors get wrapped, joined, and grepped |
| One stable leading phrase per failure kind (e.g. `reject=syslog pattern found:`) | Agents and log scanners match on it, so it is not reworded per call site |
| Wrap the cause and add context: `fmt.Errorf("parse %s: %w", path, err)` | Preserves the `errors.Is`/`errors.As` chain; each layer adds what it knows |
| Name the subject and the value, not just the type | "invalid value" with no value is unactionable |
| Truncate a large blob (body, dump, hex) before embedding it | A 10 MB error is unreadable for a human and for an agent |
| No `fmt.Sprintf`/`fmt.Errorf` on a hot path | A boundary or one-shot error may use `fmt.Errorf`; a hot path uses an append builder (`ai/rules/performance.md`) |

These are the forms that fail, and what to write instead.

| Pattern | Fix |
|---------|-----|
| `errors.New("failed")`, `"invalid input"`, `"unexpected error"` | Name what, the value, and the expected |
| Dropping the cause inside `if err != nil` (`return errors.New("parse failed")`) | Wrap: `fmt.Errorf("parse %s: %w", name, err)` |
| Reporting a value as invalid without printing it | Include `%q` of the offending value |
| Rewording a stable error phrase per call site | Keep one phrase so it stays greppable |
| Returning `nil` or skipping when a check cannot run | Return an error; fail closed |
| A user-facing failure with no diagnostic code or remediation | Register a `doctor-*` code and make it `ze explain`-able |

A machine-facing failure carries a registered code from
`internal/core/diagnostic/codes.go`, holding a title, a description, examples
and remediation, and `ze explain <code>` prints it. The handler returns the code
and structured fields rather than a finished sentence, which is what makes the
corrective action machine-readable.

## A value carries no marker

State is a field or a column. It is never a character glued to a value: no `*`,
`>`, `+`, or leading dot on an identifier.

| What breaks | Why |
|-------------|-----|
| The value | `\| grep <lsp-id>` stops matching, and a parsed field carries a character that is not part of the identifier. The text form and the JSON form then disagree about what the identifier IS |
| The token | `*` is already an INPUT token in Ze: the selector wildcard for "all" (`peer *`, `clear bgp rib in *`, `192.168.*.*`), documented in `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md` and `docs/guide/route-injection.md`. One character pointing in two directions |

A marker that exists only in the text rendering is decoration rather than
information: `| json` has nowhere to put it, so the two renderings carry
different facts. Every command here composes with the pipe operators, which is
why a sigil is a defect in Ze rather than a matter of style.

Other implementations do decorate. FRR and Extreme both print the local system's
IS-IS LSP as `rtr.00-00 *`. Copying that breaks both of the things above.

## Related

- `docs/architecture/cli/color-system.md` -- the seven semantic roles, and the
  ANSI values behind them. This document names roles (muted, danger, value) and
  never names a color. The color system is where a role becomes a number, for
  the CLI, the web UI, and every other surface.
- `ai/rules/cli.md` -- command surface rules
