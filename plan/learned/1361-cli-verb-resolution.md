# 1361 - A relative namespace owes its inverse, and the caller that ends in a side effect cannot be tested

**Date:** 2026-08-08
**Scope:** cli, testing, architecture

## What Changed

`ze show ...`, `ze clear ...`, `ze monitor ...`, `ze request ...`, `ze set ...`
and `ze delete ...` stopped answering `unknown command` for their subcommands.
The resolution step moved out of `RunCommand` into an exported pure function
`ResolveCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`), and the verb-relative
command tree got an owning file, `internal/component/cli/client/verb_tree.go`,
which carries `verbContextPath` and its inverse `AbsoluteVerbPath`.

## The Failure

**Two trees describe the same commands, and only one direction of the mapping
was written.**

`ze show bgp rib status` and the interactive `show bgp rib status` walk
different trees. `BuildVerbCommandTree` builds the first by stripping the verb,
so the command above sits at `bgp rib status`. `RunCommand` then validated argv
against that tree with argv still carrying `show`. The first word never matched
a child, so the walk failed at depth zero, for every declared subcommand of
every verb.

## The Mechanism, and the thing that hid it

**A function that ends in a process call has no return value to assert on.**
The resolution lived inside `RunCommand`, whose result is an exit code produced
after dispatch. No unit test could hand it an argv and read back what that argv
resolved to, so the only way to observe the defect was to run each command
against a live daemon. The functional suites reached the daemon by other entry
points and never spelled `ze show <subcommand>` as a shell argv.

The fix is structural rather than a repaired condition: `ResolveCommand` returns
a `Resolution`, `RunCommand` becomes its caller, and the exported pure function
is what a test drives.

## What To Do Next Time

| Situation | Do |
|-----------|-----|
| You map a path INTO a relative namespace | Write the inverse in the same file, deriving both directions from the SAME registrations. `AbsoluteVerbPath` reads `AllCLIRPCs()` exactly as `verbContextPath` does, so the pair cannot drift. Two hand-written directions drift on the first new command |
| A decision is welded into a function ending in `os.Exit`, a dispatch, or a write | Extract the decision as a pure function and export it. Testability is the reason, and it is sufficient on its own |
| You test a registry-driven surface | Enumerate the registry, never a sample. `TestDeclaredCommandsResolveFromArgv` walks every declared command and resolves each from a real argv. The defect hit almost every subcommand of every verb at once, so a small sample would have been as red as the whole set; what a sample could not do is prove the set is covered as commands are added |

## A trap worth naming

**A node's description cannot tell a grouping container from a declared
command.** `show bgp` is a container a caller must render as a subcommand list;
`show bgp rib status` is a command to dispatch. `MergeYANGNodes` gives the
container a YANG description too, so "has no description" marks nothing.
`AbsoluteVerbPath` answers it with a second return value, `declared`, taken from
whether a registration declares that exact path.

## Files

- `cmd/ze/internal/cmdutil/cmdutil.go` -- `ResolveCommand` and its `Resolution`
  are new, extracted from `RunCommand`, which becomes their caller. The verb is
  stripped from argv before the walk, and every absolute form is rebuilt with
  `cli.AbsoluteVerbPath`
- `internal/component/cli/client/verb_tree.go` -- new: `BuildVerbCommandTree`,
  `verbContextPath`, its inverse `AbsoluteVerbPath`, and
  `recordContextDescriptions`
- `internal/component/cli/client/main.go` -- the tree construction this file
  carried moved to `verb_tree.go`
- `cmd/ze/internal/cmdutil/cmdutil_test.go` -- `TestDeclaredCommandsResolveFromArgv`
  enumerates the registry; `TestInteropHarnessCommandsResolve`,
  `TestReadOnlyCommandUnderShowKeepsItsRoot` and
  `TestOfflineOnlyCommandKeepsVerbPrefix` cover the three shapes that invert
  differently
- `test/interop/interop.py` -- the harness commands the fix unblocked

## Related

- `ai/rules/cli.md` - the CLI contract this surface implements
- `ai/rules/testing.md` - enumerate rather than sample
- `plan/learned/1355-wire-edit-4-api-origin-deferred-bird-interop.md` - the
  sibling failure: a test that runs but asserts on an anchor the broken path
  also satisfies
