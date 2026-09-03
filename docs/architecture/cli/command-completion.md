# Plugin Commands in Interactive Completion

The interactive completion tree was built from YANG only. The commands that
plugins register through the plugin `CommandRegistry` worked when typed and were
invisible to Tab. The registry already carried `Hidden` and `Complete()`. The
gap was that the interactive tree never consulted it.

## The producers

| Surface | Producer | How it stays live |
|---------|----------|-------------------|
| entry source | `VisibleCommandEntries()` returns every non-`Hidden` command as a `command.CommandEntry` | RLock over the registry map, mirroring `All()` and `Complete()` |
| injection | `MergeCommandPaths` splits each command name on spaces and inserts tree nodes | non-destructive: an existing node is never mutated. Each of the two help fields is set only on a leaf the call creates, or on one that holds nothing in THAT field |
| SSH | `mergePluginCommands` merges into the per-session tree | the tree is rebuilt per session, so a plugin that exited is absent next session |
| Web | `pluginAwareCommandCompleter` builds a throwaway overlay per `/cli/complete` request and composites it, YANG winning a name collision | the shared YANG tree stays immutable and is never mutated |

<!-- source: internal/component/plugin/server/command_registry.go -- VisibleCommandEntries -->
<!-- source: internal/component/command/node.go -- MergeCommandPaths -->
<!-- source: internal/component/cli/client/inject.go -- injectPluginCommands, the client-side tree -->
<!-- source: internal/component/cli/client/main.go -- applyCommandText, nodeAtPath -->

The `ze cli` client and the attached console of `ze start --cli` run the same
rule. Each fetches the command list from the daemon. `injectPluginCommands`
drops a hidden command and hands every other one to `MergeCommandPaths`. A
command therefore enters the client's tree as it enters the SSH and web trees.

That answer carries both texts of every command. The `help` key holds the
summary, and `long-help` holds the explanation. `applyCommandText` writes the
pair onto the node a row names, in ONE walk over `nodeAtPath`. The node that
takes the summary is therefore the node that takes the explanation. A command
the tree does not hold reaches it through the merge above, with the same
pair.

A plugin command can never overwrite a builtin's `WireMethod`, its summary, or
its long help. `MergeCommandPaths` writes each of the two help fields only on a
leaf it creates, or on one whose own copy of that field is empty. The completer
offers a node on name prefix and `backendAllowed` alone and never reads
`WireMethod`, so a completion-only node surfaces.

A `command.CommandEntry` carries both fields. `Description` is the one-line
summary and `Help` is the long explanation the command's own help page prints.
They are decided one at a time, so a plugin that states a summary and no
explanation fills the summary alone.

## What Tab and `?` reveal

Tab and `?` drive one state machine with three levels. The level is derived from the
fields that own each screen region. It is never stored.

| Level | On the screen | The field that owns it |
|-------|---------------|------------------------|
| nothing | the plain prompt | neither field is set |
| candidates | the completion menu, and the selected candidate's summary on message line 2 | `showDropdown` |
| explanation | the command's long explanation, in its own region | `explanation`, which holds the text |

`revealLevel` reads those two fields, so the level and the screen cannot
disagree. `View` tests the explanation before the menu, in the order
`revealLevel` ranks them. The box an operator asked for is the box they see.
`?` reveals an explanation with the menu open, and the menu waits under it. One
Escape takes off the explanation, and the candidate list is on the screen again.

Tab advances the level. With something left to complete it completes, as it
always did. `updateCompletions` hides the menu whenever one candidate or none is
left, which is the state that exhausts Tab. Tab then calls `revealExplanation`,
which asks the command completer for the long explanation the typed command
declares. A second Tab reveals the same text, so the level stops at two.

`?` explains the CANDIDATE, so an operator reads what a name does before they
take it. With the menu open it explains the selected candidate, and with the
menu closed it explains the one match ghost text offers. With neither it falls
through to Tab. `selectedCandidate` answers which of the three states the
operator is in.

A candidate is not the typed text: the operator has typed a prefix and
highlighted a name. `revealCandidateExplanation` asks `completedInput` for the
text the candidate would produce, which is the same function `applyCompletion`
writes into the prompt. A trailing word with no space after it is REPLACED by
the candidate, and otherwise the candidate is appended. One function declares
that rule, so the explanation describes the command the candidate completes.

Both keys read `commandCompleterInput`, which is the same text
`updateCompletions` gives `Complete`. In config mode that is the text after the
`run ` prefix, and in operational mode it is the whole input. One function
answers for all of them, so an explanation always describes the command the menu
was completing. `revealExplanationOf` takes the subject text from its caller and
puts the explanation on the screen.

Nothing is invented. `Explain` answers false for an input that names no command,
and for a command that declares no long explanation. The level then stays where
it is, and message line 2 reads `<command>: no explanation is declared`. Silence
would leave the operator unable to tell an undeclared explanation from a dead
key.

Every key that ends a reveal calls `dismissReveal`, which clears the hint and the
explanation in one place. The table below says what each key does at each level.

| Key | At the menu | At the explanation |
|-----|-------------|--------------------|
| Escape | closes the menu, and the summary goes with it | takes off the explanation alone, and a menu under it stays open |
| a text rune | dismisses the reveal, and the rune reaches the input | the same |
| Backspace | dismisses the reveal, and the deletion reaches the input | the same |
| Enter | accepts the highlighted candidate | dismisses the reveal, then runs the command as typed |
| Ctrl-C | dismisses the reveal, then asks to quit | the same |
| an arrow key | moves the selection, and the summary follows it | recalls a command, and the reveal goes with the input it replaced |

Escape descends one level for each press. `handleKeyMsg` tests the level before
the arm that clears the whole input. So the words the operator typed to reach an
explanation stay in the prompt. A second Escape then clears the prompt, as it
always did.

At the menu, Enter is the only key that accepts the highlighted candidate,
because Tab cycles the selection. Enter runs the command on the press after
that one, and what it runs is the input the menu completed.

`?` is bound to help, as Tab is. It reveals rather than dismisses, and it does
not reach the input. It writes nothing on message line 2 while an explanation is
declared. `warningText` already reads the selected candidate's summary there, so
a hint of its own would be a second copy of that sentence.

The explanation region is bounded at the render boundary, not at the
declaration. `renderExplanationBox` wraps the text at the box width, reads no
more rows than the box can draw, and says `... more` when text remains. It also
passes the text through `sanitizeForDisplay`, which strips C0, DEL, C1 and ANSI
escape sequences. A plugin declares this text, so both bounds hold whatever the
declaration says.

<!-- source: internal/component/cli/model_help_level.go -- revealLevel, dismissReveal -->
<!-- source: internal/component/cli/model_keys.go -- handleKeyMsg, handleTab, revealExplanation, selectedCandidate, revealCandidateExplanation, revealExplanationOf -->
<!-- source: internal/component/cli/model.go -- commandCompleterInput, updateCompletions, applyCompletion, completedInput -->
<!-- source: internal/component/cli/model_render.go -- View, renderExplanationBox, wrapForBox -->
<!-- source: internal/component/command/completer.go -- TreeCompleter.Explain -->

## Why web sources at request time

`runYANGConfig` waits for plugin startup before `buildServices` constructs and
binds the optional looking-glass, web, and MCP management services.
`signalStartupComplete` freezes the dispatcher command registry before
`WaitForStartupComplete` returns. The MCP `tools/list` endpoint therefore cannot <!-- doc-links: ignore (JSON-RPC method name, not a repository path) -->
answer before the initial registry freeze.

Web still builds a live per-request overlay so plugins that a reload adds or
removes are visible. A one-time build snapshot would become stale after a reload.
SSH has the same liveness because it rebuilds its whole tree per session.

The attached console of `ze start --cli` builds its tree once, when `runYANGConfig`
attaches it. The operator detaches and attaches again to see the commands of a
plugin a later reload added.

<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: internal/component/cli/client/main.go -- newAttachedModel -->
<!-- source: internal/component/plugin/server/startup.go -- signalStartupComplete, WaitForStartupComplete -->

## The three completion paths are distinct

1. The interactive SSH and web tree. This is the path plugin commands were
   missing from.
2. `ze completion words`. A standalone CLI process with no daemon. It stays
   YANG-only and cannot see live plugin commands.
3. The daemon's `system command complete` RPC. It already completed plugin
   commands through `Registry().Complete()`.
