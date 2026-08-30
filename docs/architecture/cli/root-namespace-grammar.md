# CLI Root Namespace Grammar

> **Placement note.** This page describes a CLI-wide rule, not an IS-IS rule. It
> lives here because the IS-IS and OSPF offline decode CLIs are its two
> registered examples. The shape rule it extends is
> [`../cli/command-namespacing.md`](../cli/command-namespacing.md), "Token
> naming: hyphen for one name, space for a namespace".

The CLI grammar rules are enforced over the YANG command tree. The root
namespace, where handlers register directly rather than through YANG, was not
covered, so it drifted: four roots hyphenated two ideas that YANG already models
as two tokens, and one root was named after a single operator kind while
carrying a whole operator language.

The load-bearing deliverable was the **gate**, not the renames.

## The command shape, and what each feeder checks

```
<verb> <noun> <action> [<args>]
<verb> <noun> <selector-kind> <selector-value> <action> [<args>]
```

| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show interface <name>` | `show interface name <name> detail` | `<name>` is untyped and could collide with a keyword (`brief`, `errors`) |
| `show interface <name> counters` | `show interface name <name> counters` | The selector value appears before its kind |
| `show l2tp session <id>` | `show l2tp session id <id> detail` | The id is typed before use |
| `show vpn ipsec peer <name>` | `show vpn ipsec peer name <name> detail` | A named lookup needs an explicit selector kind |
| `cache <id> retain` | `cache retain <id>` | Id before action |
| `commit <name> start` | `commit start <name>` | Name before action |

Seven feeders enforce this, and `./le cli-grammar` runs them.

| Feeder | What it checks | Run |
|--------|----------------|-----|
| Static gate | Every built-in command (YANG command tree) against R1-R9, plus no `--flag` in any `.yang`. R9 sibling-collision is static-gate-only because it needs sibling context | `./le cli-grammar`. It is not a `./le verify worktree` stage; it reaches CI through `TestTheRealCheckoutPassesAndWasRead` in `internal/le/cligrammar/cligrammar_test.go`, which runs the same checker over the real checkout under the unit stage |
| Registration | Every plugin `CommandDecl` at registration (`validateCommandName`) | plugin startup in the functional and exabgp suites |
| Runtime guard | The runtime built-in assembly (`AllBuiltinRPCs` by `WireMethodToPaths`) re-checked with `ExemptCategory` by wire method, and the `CommandRegistry.Register` boundary rejecting a bad name | `TestRuntimeBuiltinSurfaceGrammar`, `TestRegistrationRejectsBadGrammar` (unit) |
| Root namespace | Every registered root command against R9 across surfaces (`grammar.CheckRootNamespace`): a hyphenated root whose left segment names a YANG verb or container is a namespace member masquerading as a compound root. Root handlers never pass through the YANG-tree static gate, so this is the only feeder that governs them | `./le cli-grammar`; `TestRootNamespaceGrammar` (unit) |
| Demo call sites | Every `ze <token>` invocation under `demos/terminal/`: the position-1 token must be a YANG verb, a registered root, or the `-` stdin sentinel | `./le cli-grammar` reads the demo sources; `./le terminal-demo check-all` validates the published artifacts |
| `le` surface | `le`'s own command tree, which the first five feeders never reach because it registers outside the YANG tree and outside `registry.RegisterRoot` | `./le cli-grammar` |
| Offline flags | The `cmd/ze/` flag surface: a root spelled as a flag, a flag a client sends to the daemon, a flag repeating a pipe operator, and a flag the parser and `registry.RegisterCommandFlags` disagree about. What a static scan cannot place is counted and printed, never dropped | `./le cli-grammar`; the shapes are pinned by `TestTheFlagFeederDrawsARowForEachShape` (unit) |

A verb is added by editing `command.Verbs`, which both the plugin gate and the
static gate derive from. Category exemptions — the text bridge, the
`ze-plugin:`/`ze-system:` wire-protocol directives, and `ze-editor:` modes —
live in `grammar.ExemptCategory`, keyed on the handler wire-method namespace,
never a per-command allowlist.

The runtime guard is an in-process check, not a daemon-boot audit. Built-ins are
fully YANG-derived (a handler with no YANG path is skipped,
`LoadBuiltinsWithAliases`), so they are a strict subset of the static gate's tree,
and plugin commands are rejected at registration by the registration feeder. The
merged `system command list` surface therefore holds only conforming commands by
construction. A boot-and-dump audit would add no catch value, and it would depend on
an all-plugins config that cannot exist, because startup is config-path-gated. The
guard instead locks the two runtime sources against regression cheaply and
deterministically.

## Decision: a fourth feeder for the grammar gate

`grammar.CheckRootNamespace(roots, namespaces)` flags a root whose left
hyphen segment names a YANG verb or container. It is a **cross-surface** check,
not a reuse of the sibling-collision check.

The sibling check fires only when the colliding namespace is a sibling at the
same tree level. No `traffic` root existed to be `traffic-control`'s sibling,
which is exactly why the gate stayed green over four violations.

This root-namespace check joined the static YANG check, plugin registration and
the runtime guard as the fourth feeder. Three more followed: the demo call
sites, le's own root surface, and the flag register below. `./le cli-grammar`
prints the size of every population it read, because a run that checked nothing
and a run that found nothing report the same zero findings.

## Decision: a seventh feeder for which register a token belongs to

The six feeders before it read the command MODEL, where a `--flag` is banned
outright and the check is a string test. The offline surface is the one where a
flag is LEGAL, so nothing checked it: 57 `flag.NewFlagSet` call sites, 121 flag
declarations and 78 distinct flag names, and no gate over any of them.

Feeder 7 asks the four questions `ai/rules/cli.md` asks of a flag, from source.
The checks are pure functions beside `CheckName` and `CheckRootNamespace`; the
gate collects the populations they judge by parsing every Go source under
`cmd/ze` and `internal` once.

<!-- source: internal/component/command/grammar/flags.go -- the four flag-register checks -->
<!-- source: internal/le/cligrammar/flags.go -- the populations they judge, and the tracked debt -->

| Rule | What fails | Why it matters |
|------|-----------|----------------|
| F1 | a registered root whose name is a flag | a flag that dispatches enters no tree, so completion, `ze help command` and every grammar feeder are blind to it. `--version`, `-V`, `--help` and `-h` are the stated exception |
| F2 | a string literal that names a daemon command path and carries a flag | `(*Dispatcher).Dispatch` refuses a flag-shaped token before any handler runs, so the command fails on every invocation while its client half and its daemon-side parser both read as finished code |
| F3 | a rendering flag on any command of the `ze` surface: `--json`, `--ndjson`, `--table`, `--text`, `--yaml`, `--raw`, `--format` and `--no-header` | rendering is the pipe layer's job. Where the answer is served by `registry.MustRegisterLocalData` the flag is a second spelling of `\| json` and only the operator composes; where it is not, the missing registration is the defect the finding names |
| F4 | a flag the parser reads that `registry.RegisterCommandFlags` never declared, or a declared flag no parser reads | completion offers what the registry holds, and prose drifts from the parser in both directions |

`FlagShaped` is the same predicate the daemon refuses by, called from
`firstFlagToken`. A gate that judged flag shape differently from the daemon
would pass a command string the daemon rejects.

## Decision: F3 asks about the flag, never about the registration

F3 first fired only where `registry.MustRegisterLocalData` served the path, so
the commands that reach no pipe layer at all were the ones it passed. That
condition read the accident as the rule: whether a command reaches the pipe
layer is a fact about how it was registered, and `command.ServeLocal` renders
the whole operator set over any path registered that way. So "this command
reaches no pipe layer" is the defect, and the finding names the registration as
the fix rather than skipping the command.

The banned spellings are DERIVED. `command.PipeOperatorCatalog` is the one
statement of the operator language and `PipeOperator.Renders` says which of its
operators decide the form of an answer, so `--json`, `--ndjson`, `--table`,
`--text`, `--yaml` and `--raw` are read from there and an operator added to the
catalog is a banned flag spelling on the same commit. Two spellings are named in
`internal/component/command/grammar/flags.go` because no operator carries them:
`--format` selects among the rendering operators, and `--no-header` drops the
header the table renderer writes. `ai/rules/cli.md` bans both by name.

Widening it drew 11 more violations, all tracked as debt: eight commands parse
`--json` or `--yaml` with no local-data registration behind them, `config
migrate` parses `--format`, and `ze cli --format` sets a session default that
`commandWithFormat` lowers into the pipe operator.

## Decision: the debt is a ledger, never an allowlist

The tree carried 50 violations when the feeder landed, and 11 more when F3
widened, so a feeder that failed on all of them would block every commit.
`flagRegisterDebt` and `flagDeclarationDebt` list each one with the reason it is
still there, and the
gate prints the whole ledger and its count on every run. A violation that is not
listed fails. The F4 entries name the exact flags they forgive, so a listed path
that starts parsing a NEW flag still fails.

An entry whose violation is gone reports `FIXED, delete this entry` rather than
failing the gate. Several sessions share this checkout, and a gate that went red
on somebody else's landed fix is a gate nobody can keep green.

## Decision: the fixture test lives with the pure function

The gate itself is a `//go:build ignore package main` program, so a sibling test
can only run it as a subprocess and cannot unit-test its functions. A red-green
fixture test needs a pure function, so it lives in the `grammar` package.

Prove the gate **wiring** end to end separately, by temporarily reintroducing a
hyphenated root, watching it flag, and restoring.

## Decision: `pipe` names the operator language

`pipe` is the word the codebase already uses for the operator set, and it reads
correctly at a literal shell pipe. `format` is a **kind within** that set, so it
could never name the set: a command asking a "format" command for a filter is
incoherent.

## Decision: object roots with a closed-keyword sub-dispatch

`traffic`, `isis` and `ospf` are root handlers whose members (`control`,
`decode`) are dispatched by a closed keyword set inside the owning package. Two
alternatives were rejected: a YANG container, because these are offline tools
with no config surface; and promoting the operators to roots, because a bare
operator name would collide with a YANG verb.

<!-- source: internal/plugins/isis/cli/register.go -- the isis root namespace and the decode member -->
<!-- source: internal/plugins/ospf/cli/register.go -- the ospf root namespace and the decode member -->

Bare `ze isis` now enumerates its members. Completion, usage and the TUI needed
no change: they derive from the root listing.

## Decision: a server verb goes through the local-handler registry

A root named after a YANG verb is dead code, because the verb check returns
before root dispatch. Command execution consults local handlers before the YANG
tree, which is the mechanism `show version` already uses, so a verb-named command
registers there instead.

## Trap: a rename that frees a token can collide it in another binary

The test binary registers its own suite roots named `traffic`, `isis` and `ospf`
**and** imports the generated composition root. While the tools were
`traffic-control`, `isis-decode` and `ospf-decode` there was no clash. After the
rename the test binary panicked on a duplicate root command.

The fix marks the three tool CLIs `// codegen:skip`, so they reach the shipping
binaries only through their direct dispatch imports, consistent with the firewall,
iface and L2TP CLIs.

## Trap: a renamed token is not the same as a renamed identity

`isis-decode` and `ospf-decode` are unambiguously CLI command names and were
renamed everywhere, including about ten functional-test invocation lines that no
file list had recorded. `traffic-control` is **also** the config and component
identity, appearing in error messages, the YANG module and functional tests, and
that spelling stays. "No stragglers" therefore means no CLI-command references,
not zero matches.

## Trap: functional-test output matching is cumulative

Standard output and standard error assertions accumulate across all commands in
one file, so a negative assertion fails if any earlier step in the same file
emitted the string. Completion negatives live in their own isolated file.

## Coverage boundary

The feeder governs the **root** surface only. The local-handler surface is not
covered: local registration validates an empty path and a nil handler and never
checks command-name grammar. A hyphenated local command would not be flagged.
Extending the gate there is separate work.

F3 reads the flags a `flag.NewFlagSet` declares, so a rendering flag a command
parses BY HAND is invisible to it. `ze pipe help --json` is read by
`slices.Contains(args, "--json")` in `runPipe` (`cmd/ze/ze_core_pipe.go`), and
`explain` switches on the same token in its own argument loop
(`internal/plugins/explain/main.go`). Neither reaches the scan. Covering them
needs a second population, because a hand-read token carries no flag set to name
the command it belongs to.

Three further items are deliberately untouched. A root-level flag is a
different rule's question, and the feeder skips leading-hyphen roots. No root
is flag-shaped today: `--plugins` was the last one and is now `show plugins`.
The two `format` value vocabularies, the CLI output format and the editor pipe
operator, are both preserved and unreconciled. `ze pipe` takes one shell-quoted
pipe expression rather than repeated operators.
