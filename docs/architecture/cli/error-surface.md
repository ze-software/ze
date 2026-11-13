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
| 1 | the completion hint | Tab or `?` asked for an explanation the command does not declare, or the input is invalid |
| 2 | the selected candidate's summary | the completion menu is open |
| 3 | the validation hint | the config holds an error or a warning |
| 4 | the idle banner | nothing above it applies |

`?` puts a long explanation in a box above the prompt, and writes nothing on
this row. The row is one row, so it holds a summary and never an explanation.

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

`./le doc yang-contract help-shape` refuses a newline in a summary declared in this
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
| Lowercase start, no trailing punctuation | Go convention; errors get wrapped, joined, and grepped |
| An operator refusal is an `opererr.Error`, with the fix on its last line | The operator reads the command to run on a line of its own; see "The operator error" below |
| One stable leading phrase per failure kind (e.g. `reject=syslog pattern found:`) | Agents and log scanners match on it, so it is not reworded per call site |
| Wrap the cause and add context: `fmt.Errorf("parse %s: %w", path, err)` | Preserves the `errors.Is`/`errors.As` chain; a layer adds only a fact the layer below did not hold, so no prefix appears twice |
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
| A user-facing failure with no diagnostic code or remediation | Register a `doctor-*` code and make it `ze show diagnostic`-able |

A machine-facing failure carries a registered code from
`internal/core/diagnostic/codes.go`, holding a title, a description, examples
and remediation, and `ze show diagnostic <code>` prints it. The code is declared once,
as an exported constant beside the table, and every surface that emits it
references that constant rather than the string. The handler returns the code
and structured fields rather than a finished sentence, which is what makes the
corrective action machine-readable.

## The operator error

A refusal an operator reads is an `opererr.Error`
(`internal/core/opererr`). The producer that holds the fact builds it, once,
from three parts, all required: what went wrong, why it matters, and the
command or config change that fixes it. An optional cause follows the explanation as evidence.
The fix is the command alone, the exact text the operator types, never a
repair id: a diagnostic's `Repair` and `FixSafety` name an automated repair.

| Surface | What it renders |
|---------|-----------------|
| Go error chain, API answer, plugin RPC answer | `Error()`: the problem on line 1, the why on line 2 followed by `: <cause>` when there is one, `fix: <command>` on the last line |
| CLI | the same text, printed unchanged by the client, in the error zone above |
| Log line | the problem as the message, with `why`, `fix` and `error` (the cause) attributes, then a `code` attribute when the caller passes one; the type never invents a code |
| Web notice | the problem, the why and the fix, read through `Problem`, `Why` and `Fix` |

The fix is always the last line, so a layer that prepends context to line 1 never moves it.
A layer MUST NOT append to the text either. A refused reload whose rollback or
candidate cleanup also failed reports those failures on a line of their own
above the fix (`rollback failed: ...; candidate cleanup failed: ...`), so the
fix line stays the exact command the operator copies. Behind an error that is
not an operator error, the same note trails the text in parentheses.

<!-- source: internal/core/opererr/opererr.go -- Error, LogAttrs, Problem, Why, Fix -->
<!-- source: cmd/ze/hub/reload_unwind.go -- withUnwind, unwoundError -->

A command handler can also refuse by answering: a `plugin.Response` whose
`Status` is `plugin.StatusError`, returned with a nil Go error. `Response.Error`
is a plain string on the wire, and it is what the operator reads, so it owes the
same three parts. The one route an operator error takes into it is
`plugin.OperatorErrorResponse(e *opererr.Error)`, which writes `Error()` whole
into `Response.Error`. Its parameter type is the proof: no other field and no
second constructor carries a typed refusal into the answer.

<!-- source: internal/component/plugin/types.go -- OperatorErrorResponse -->

`./le arch operator-error check` judges both forms on the lines a change adds or
changes, at `./le commit create` and in the verify stage. `./le arch
operator-error report` lists every instance in the tree and refuses nothing.
The registered handlers come from their registration sites. The check reads
each returned error, and each error answer the handler body builds outside a
nested function literal, and classifies it:

| Status | A returned error | An error answer |
|--------|------------------|-----------------|
| typed | a call of `opererr.New` or `opererr.Wrap`, or a method chain rooted in one | a call of `plugin.OperatorErrorResponse` |
| plain, refused when a change adds it | `errors.New` or `fmt.Errorf` at the return, or an `opererr.Error` composite literal, which is the zero operator error and names no fix | an `Error` the site wrote: a literal, a constant, a built string, no `Error` at all, or the `Error()` of an error the site built, `opererr`'s included, because that text bypasses the route |
| pass-through, reported only | any other error value | the `Error()` of an error value the syntax does not type (a callee's error, a sentinel), or another answer's `Error` field forwarded |

The scan does not read an answer a helper function builds, an answer whose
`Status` is computed at run time, or an `Error` assigned after the literal.

The package exports its site list for `./le arch error-inventory`
(`docs/contributing/error-inventory.md`): `Sites` over the working tree, and
`SitesOnSide` over one side of a commit, so that check's commit judge leaves
a writer call on a handler's return line to this one.

<!-- source: internal/le/arch/operatorerror/response.go -- responseSites, classifyResponse, classifyMessage -->
<!-- source: internal/le/arch/operatorerror/report.go -- addedSites, checkLines, SitesOnSide -->
<!-- source: internal/le/arch/operatorerror/operatorerror.go -- Sites -->

## A refusal of several lines in the editor

Row 1 holds one line, and a value refusal holds several: the problem, the
accepted values with their help, and the fix (its shape is "Value refusals" in
`docs/architecture/config/yang-config-design.md`). `Model.setError` writes an
error whose text holds a newline whole into the viewport, and
`Model.feedbackLine` puts only its first line on row 1, after `Error: `. The
operator reads the problem where every error appears and the fix in full
above it, and the prompt row stays where the view counted it.

A refused commit splits the same way: row 1 carries the header (how many
errors and warnings block the commit), and the viewport carries the header
and then each issue as its own block of lines, each ending with its `fix:`
line (`CommitRefusal`).

<!-- source: internal/component/cli/model_render.go -- setError, feedbackLine -->
<!-- source: internal/component/cli/model_commands_commit.go -- CommitRefusal, commitValidationRefusal -->

## A fix is framed for where the operator types it

A fix is the exact text the operator types, so it depends on where they type
it. The editor's refusals carry the command they propose as a
`cli.FixCommand` (the verb, the position its words are relative to, the
words), and `cli.FrameFix` writes it for the operator's position, once, for
every editor surface (the SSH editor, `ze config edit`, the web terminal and
the web CLI bar):

| Operator's position | Fix line |
|---------------------|----------|
| the position the command's words are relative to | the command as typed, corrected: `fix: set session capability route-refresh require` inside `edit bgp peer peer1` |
| the root | the command from the root: `fix: set bgp peer peer1 session capability route-refresh require` |
| any other context | `top`, then the command from the root: `fix: top set bgp peer peer1 session capability route-refresh require` |

`top <command>` runs one command as if at the root and leaves the operator
where they were, so a `top set ...` line is exact from any position. A
refused `set`, `insert`, `rename` or `copy` keeps the form the operator typed
(`FramedSetRefusal`, `FramedKeyRefusal`, `FramedWordRefusal`); a commit
refusal names the leaf from the root, behind `top` inside a context. A web
form and the commit modal hold no position, so their fix is the command from
the root. The position a fix is framed for is the one the operator stands at
(`Model.operatorPosition`), never the root a `top` line ran at.

Fixes retain each literal key and value as one token. The shared config quoter
quotes and escapes concrete words, including an actual enum name containing
spaces or quotes. The refusal producer separately marks correction
placeholders such as `<1..4096 characters>`, so those remain readable syntax
to replace. A literal `<two words>` is quoted as data. A literal `"|"` is data
too; only a bare pipe is an operator. The parsed command keeps that lexical
distinction through `top` dispatch and every filter in a pipeline. Its words
and pipe positions are private, and callers receive copies of decoded words.
File-form corrections use the same literal-versus-placeholder decision as
command-form corrections.
<!-- source: internal/core/configtext/token.go -- QuoteToken, QuoteCommandWord -->
<!-- source: internal/component/config/yang/refusal.go -- correctionToken, CorrectionText, CorrectionIsPlaceholder -->
<!-- source: internal/component/config/edit_path.go -- ParseEditWords -->
<!-- source: internal/component/cli/command_words.go -- CommandWords, ParseCommandWords, Tail, SplitPipe -->
<!-- source: internal/component/cli/model_load.go -- ParsePipeFilters -->

<!-- source: internal/component/cli/fix_frame.go -- FixCommand, FrameFix, FramedSetRefusal -->
<!-- source: internal/component/cli/key_refusal.go -- FramedKeyRefusal, FramedWordRefusal -->
<!-- source: internal/component/cli/model_commands.go -- cmdTop -->
<!-- source: internal/component/cli/model.go -- operatorPosition -->

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
