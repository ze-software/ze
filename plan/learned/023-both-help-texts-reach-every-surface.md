# 023 - Both help texts reach every surface

**Spec:** spec-both-help-texts-reach-every-surface, closed 2026-09-15
**Class:** `plan/journal/command-takes-an-untyped-positional-value.md`,
`plan/journal/guard-demands-what-the-model-cannot-supply.md`

## What the work built

Every YANG node an operator reads declares `ze:help` (the summary) and
`description` (the explanation), and five carriers used to drop one or both
before a reader saw them. `command.ArgDef` now holds the pair (`argDefFor`,
`internal/component/config/yang/command.go`) and the CLI JSON help, the site
and wiki catalogs and the web admin form render it. `yang.LeafMeta` holds the
pair (`extractEntryLeaves`, `rpc.go`) and `ParamMeta`, the JSON Schema
(`title`, `description`), the gRPC `ParamInfo`, the MCP `inputSchema` and
`ze help ai --json` carry it. The web config editor renders a leaf, container
or list explanation as a block under the tooltip that keeps the summary.
`cli.AnalysisNode` carries both texts and the enum values with their
summaries, so the published configuration reference prints them. One reader,
`yang.EnumValueSummaries` and `yang.EnumValueNames` (`enum.go`), serves the
completer, the tree and the help-shape gate, and the `enum value` literal is
gone. `./le docvalid help-shape` judges 813 enum values, 241 arguments, 265
rpc leaves and 20 notification leaves, all with a summary.

## Decisions

**JSON Schema `title` is the summary and `description` the explanation.** The
MCP specification defines `title` for the tool only and shows properties with
`description` alone, so a `description`-only client loses the summary (A-3
broken). Joining the two into `description` would derive one text from the
other, which the spec forbids, so the pair stays two keys.

**An absent text omits the key.** Every envelope writes `short-help` and
`description` under `omitempty`; nothing prints an empty string and nothing
fills one text from the other.

**A typedef-borne enumeration resolves through goyang's own resolver.**
`collectEnumSummaries` follows `YangType.Base` from the leaf's statement to
the builtin and reads the `ze:help` off each statement's `enum` list on the
way; nothing is looked up by name.

**The completer caches the summary map per leaf entry.** Completion runs per
keystroke; `Completer.enumValueSummaries` walks the AST once per entry, and
the test asserts the map identity, not equality.

## Trap for the next session

A form input with no reader. The admin command form rendered an `<input>` per
`ArgDef` and `HandleAdminExecute` built the command from the URL path alone,
so a typed value vanished. The fix taught a second trap: a value whose
`ArgDef.Anchor` names a path keyword binds bare after that keyword, inside the
path (`anchoredDef`), and `Dispatch` refuses the keyword form. The MCP call
builder still emits the keyword form (journal row, out of this spec's goal).

A `-cmd` module's `ze:command` container can declare no leaf while the rpc
INPUT does (`system command help`): `extractArgDefs` reads the container's
own leaves, so the admin page for that command renders no input. Pick a
command whose argument leaf lives on the `-cmd` container before you write a
`.wb` against the form.
