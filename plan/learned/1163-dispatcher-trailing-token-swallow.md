# dispatcher-trailing-token-swallow

The daemon command dispatcher silently swallowed unmatched trailing tokens, and
the obvious fix would have broken most of the CLI. Both halves are worth
remembering.

## The bug

`matchCommandTokens` (`internal/component/plugin/server/command.go`) walks
only the KEY's tokens and never checks that the input is exhausted. Line 428
returns the unmatched tail as `args` and reports a SUCCESSFUL match. `Dispatch`
then validates args only when the matched command has ArgDefs
(`command.go`), and `extractArgDefs`
(`internal/component/config/yang/command.go`) reads YANG **leaf** children only,
so a node whose children are all containers has none.

Net effect: `ze l2tp show --user alice tunnels` matched `show l2tp` (whose
`ze:command` is `ze-l2tp-api:summary`), the validator was skipped, and
`handleSummary` (`internal/component/l2tp/cmd/l2tp.go`) discarded the tail via
`_ []string`. The operator got the SUMMARY for the DEFAULT user, **exit 0**, with
no hint that `--user alice` and `tunnels` were both ignored. A wrong answer that
reports success is worse than an error.

## The trap: "zero ArgDefs" does NOT mean "takes no arguments"

The natural fix -- reject leftover args when `len(ArgDefs) == 0` -- is wrong and
would have broken a large fraction of the CLI. Zero ArgDefs is a fact about YANG
leaf authoring, not about the handler's contract. Roughly 60-70% of `ze:command`
nodes have no leaves. Measured casualties of that rule:

- **all 57 OSPF commands** (`ze-ospf-cmd.yang`: 57 commands, 0 leaves), ISIS (10/0),
  system (14/0), and most `*-cmd` plugins;
- **`clear l2tp tunnel id 42`** -- the clear tree declares no leaf while the show
  tree does, so teardown would stop working entirely;
- **the whole RIB filter feature.** `foldFilters`
  (`internal/component/command/pipe.go`) rewrites `show bgp rib | peer X |
  family ipv4` into the plain string `show bgp rib peer X family ipv4` BEFORE it
  is sent, so pipe filters arrive as exactly the trailing args the rule would
  reject;
- `TestDispatcherDispatch` (`command_test.go`), which registers a zero-ArgDef
  command and asserts trailing tokens reach the handler -- its comment reads
  "PREVENTS: Command misdirection or lost arguments".

`Dispatcher.Register` never sets ArgDefs at all, so every programmatic
registration is zero-ArgDef by construction and indistinguishable from a
zero-leaf YANG node at dispatch time.

## What shipped, and why it is narrow

Reject only FLAG-SHAPED leftovers (`firstFlagToken`, `command.go`): a leading
dash followed by a letter. Justification, not taste:

- no `ArgKind` is signed (`internal/component/command/node.go`: ArgString,
  ArgEnum, ArgUint, ArgUnion), so no valid value starts with `-`;
- pipe folding only ever emits bare filter names and their values;
- `firstPositionalArg` (`internal/component/l2tp/cmd/l2tp.go`) already
  skips `-`-prefixed args deliberately -- which is part of why `--user` slid
  through.

Bare `-`, `--`, and `-5` are deliberately NOT flag-shaped and still pass through.

## Still open

The TYPO class survives: `show l2tp tunnnels` still returns the summary
silently. Closing it needs an explicit contract, not a heuristic -- a
`RegisterOptions.NoArgs` / `ze:no-arguments` declaration, switched on for the
handlers whose signature is already `_ []string` (mechanically greppable). A
cheaper option (reject a leftover that names a known CHILD command) must first
prove `peer` is not a child of `show bgp rib`, or it breaks folded filters again.

## Rule

Before tightening a shared dispatcher, measure the blast radius against the real
command tree. A predicate that sounds like a contract ("no ArgDefs means no
args") can be an artifact of how someone wrote a YANG file. The check that
settles it is cheap: if the new rejection never fires across a full run, the
change is inert; if it fires, find out what it hit before shipping.

## Files

None recorded.
