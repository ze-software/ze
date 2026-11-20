# Spec: shell-reaches-every-daemon-command

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P2 |
| Release reason | An operator at a shell cannot complete or run 33 daemon commands that every other surface offers, and tab completion offers no verb but `show` as a first word. The owner's rule is that every operator command reaches every command surface with the same output (`ai/META-DECISIONS.md`, "Goal and scope"). |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

## Task

Make the shell surface offer and run every daemon command `command.AnswerSurface`
says it owes, and make the generated completion scripts offer every command
verb as a first word.

Two gaps were found on 2026-11-20, when `ze completion words run` was removed
because `ze run` no longer exists (commit 3eb576533c).

1. A word that a root command owns is closed to the daemon commands under it.
   `plugin`, `resolve` and `help` are root commands, and the command tree also
   declares daemon commands below those words (`plugin command list`, `resolve
   dns a`). Dispatch gives the word to the root handler, and completion walks the
   root's own second words only. `system` is in the same state from the shell:
   `ze system version software` answers "unknown command".
2. The scripts offer `show` alone as a first word, appended by hand beside the
   registered roots, and the fish script has a completion rule for `show` only.
   Bash, zsh and nushell complete a verb once it is typed; no shell proposes it.

## Source and Evidence

- `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`) names
  the 33 paths on the shell arm. Before 3eb576533c its shell probe asked
  `ze completion words run <prefix>`, a query no generated script sends, so the
  absolute command tree answered for every path and the gap was not visible.
- Read in source, not run against a fresh binary: `yangVerbs`
  (`cmd/ze/ze_core_dispatch.go`) removes every root's name from the words the
  verb dispatch serves, and `completionTree`
  (`internal/plugins/completion/words.go`) answers a root's tree before a verb's.
- Run on 2026-11-20 against a binary built that morning: `ze plugin command
  list`, `ze plugin session ping`, `ze resolve dns a`, `ze system command list`
  and `ze system version software` each answer "unknown command".
- The first-word list is `shellRootCommands`
  (`internal/plugins/completion/root_commands.go`).

The design is not chosen. It is put to Thomas before this spec moves to ready.

## Existing Mechanisms

| Mechanism | What it already does | How this work uses it |
|-----------|----------------------|-----------------------|
| `command.AnswerSurface` (`internal/component/command/surface.go`) | Decides, from a registration alone, what the shell surface owes each command | The definition of done. No second list of shell commands is written |
| `yangVerbs` and the root registry (`cmd/ze/ze_core_dispatch.go`) | Derive the words `ze` dispatches from the command tree and the root registrations, a root winning its name | The rule that a root wins its word is kept or changed on purpose; either way the population stays derived |
| `declaredCommandHelp` (`cmd/ze/ze_core_dispatch.go`) | Gives the help word a second route to a command node below a root-owned word, narrowed to a path of two words or more whose node carries a `ze:command` | The precedent for reaching a daemon command under a root-owned word. Extending the same narrowing from help to execution and completion is one candidate design |
| `completionTree`, `rootCommandTree`, `rootSubVerbs` (`internal/plugins/completion/words.go`) | Answer what the shell offers after a root or a verb, from registrations | Extended so the answer under a root-owned word also holds the daemon commands the surface owes |
| `command.Verbs` | The one vocabulary of command verbs | The source of the first-word list, replacing the hand-appended `show` |
| `registry.Meta.DaemonPath` | Lets a local command at an offline root name the daemon path that answers it | Unchanged; it covers a local command, not a daemon command under a root's word |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Each daemon command `command.AnswerSurface` owes on `SurfaceShell`, including those under `plugin`, `resolve`, `help` and `system` | `ze completion words` offers its last word after its prefix, by the query the generated scripts send |
| AC-2 | The same commands typed as `ze <words>` with a daemon running | The command runs and prints what `ze cli -c "<words>"` prints |
| AC-3 | A root command's own second words (`ze plugin test`, `ze resolve help`) | Unchanged: the root handler answers them and completion offers them |
| AC-4 | First word, in bash, zsh, fish and nushell | Every verb of `command.Verbs` that no root owns is offered, each with a description read from the command tree, and fish completes under each one |
| AC-5 | `TestEveryCommandOnEverySurface` | Green on the shell arm with a probe that sends only what a shell sends |
| AC-6 | The four script goldens under `internal/plugins/completion/testdata/` | Regenerated from the generators and judged by `ze-test-judge`; on 2026-11-20 `TestZeCompletionScriptsUnchanged` is red for a cause this session did not find |

## Files to Modify

- `cmd/ze/ze_core_dispatch.go` - how a word a root owns reaches the daemon commands declared below it.
- `internal/plugins/completion/words.go` - what completion answers under a root-owned word.
- `internal/plugins/completion/root_commands.go` - the first-word list, derived from the verb vocabulary.
- `cmd/ze/hub/surface_parity_test.go` - the shell probe, if the design changes what a shell sends.
- `docs/architecture/cli/command-completion.md` - the page that states today's limits.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-20 | shell: 33 daemon commands under `plugin`, `system`, `resolve`, `help` neither completed nor run; only `show` offered as a first word |
