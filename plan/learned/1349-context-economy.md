# 1349 - Context economy: where agent sessions spend tokens, and what changes it

**Date:** 2026-08-05
**Scope:** tooling, agent workflow, rules

## What Was Asked

Read the session logs, say what makes a spec implementation expensive, and fix
what the answer showed.

## What The Measurement Found

`make ze-token-economy` (`scripts/dev/token_economy.py`) reads the machine-local
Claude Code transcript store and reports where the tokens go. On the store at
the time of writing:

| Measure | Value |
|---------|-------|
| API calls | ~40,000, of which 76% are subagent calls |
| Tokens fed to the model | 12.4B, for 231M distinct |
| Re-read factor | every distinct context token is read again ~53 times |
| Subagent share of context | ~65% |

**Cost per API call is the context size at that call.** The bill is round trips
times context, and nothing else moves it much. Two facts follow, and both are
counter-intuitive enough to be worth writing down:

- **Trimming tool OUTPUT buys nothing.** Bash results average 436 tokens. The
  round trip is the cost, not the payload.
- **Long agents are expensive because context grows with turns.** An
  implementation agent averaged 147 API calls at 294k mean context; the largest
  single agent ran 724 calls. Cost inside one agent grows super-linearly.

Review is the cheapest phase measured (15.4% of subagent context) and the
fix/debug phase it prevents is the second most expensive (24.5%). Cost pressure
must never be applied there. `ai/rules/context-economy.md` says so explicitly,
because "review is expensive, one lens will do" is the reasoning this
measurement most obviously invites and most clearly refutes.

## Three Traps In Reading The Transcripts

Anyone who re-derives these numbers will hit all three. Each was found by an
independent reviewer AFTER the first analysis had been reported as fact.

| Trap | What goes wrong |
|------|-----------------|
| **Usage is repeated on every split record of one API call** | One API call is written as several assistant records. Counting records doubles every figure. Dedupe by `message.id` |
| **`output_tokens` GROWS across those records; the context fields do not** | Taking the first record undercounts output 2.49x. Measured: across 25,106 multi-record ids the three context fields never vary, only `output_tokens` does, always monotonically. `_merge` takes the maximum per field, which is exact rather than approximate |
| **A resumed session copies the earlier session's records into the new transcript** | Per-FILE dedup is not enough: 487 ids appeared in two files, double-counting 208M context tokens. `assign_owners` picks one owning transcript per id, using the record's own `session_id` (a resume rewrites `sessionId` but leaves `session_id`), then a non-fork preference, then documented-arbitrary order |

## The Second Trap Is The Lesson

The first analysis of this store was reported to the operator with a cost split
of 78/18/4. It was wrong: output was undercounted 2.49x, and the real split is
73/17/9. The conclusion survived, which is exactly why the error was easy to
miss -- cache-read dominates either way.

**A number that supports the conclusion you already reached gets less scrutiny
than one that contradicts it.** Both corrections here came from an adversarial
reader, not from the author re-reading their own work. This is the independence
requirement in `ai/rules/planning.md` paying for itself on an analysis rather
than on code.

## The Provenance Rule This Produced

The rule first said "every figure here comes from `make ze-token-economy`" while
the tool parsed only `message.usage` and could not produce most of them. The
figures were real, measured by scratch scripts that no longer existed; the
citation was not.

**A figure in a rule needs a command that reproduces it, named next to it.** The
fix was to teach the tool to print what the rule cites, not to soften the claim.
Where a figure comes from elsewhere (the `gopls` ratios are properties of files,
not of the store) the rule names that command instead.

Corollary: **do not pin absolute totals in a rule.** The store grows every
session, so a quoted call count is stale on arrival. Quote ratios and per-agent
means; where an absolute earns its place, mark it as a reading at a date.

## LSP: Present Tool, Absent Server

`.claude/rules/session-start.md` made loading LSP a BLOCKING first action and
`.claude/hooks/block-until-lsp.sh` enforced it. `gopls` was not installed on this
machine, so every LSP call would have returned `ENOENT`. Across the whole
measured store there was not one LSP call -- the gate was satisfied by *asking
for the tool*, every session, for weeks.

**A gate that checks you asked for a capability does not check that the
capability answers.** `gopls_status` in `scripts/dev/dev-setup.py` now probes the
server functionally in `make ze-setup`, and reports "present but not answering"
distinctly from "not installed": different problems, different fixes.

Measured saving once it worked, `gopls symbols` against reading the whole file:
9.0x, 34.0x and 10.3x on three real files; one `definition` answered in 208
bytes.

## Subagents And LSP: State The Capability, Not The Registry

Probed on Claude Code 2.1.222: a subagent's deferred registry does not carry the
LSP tool, and a custom agent definition with `background: false` does not change
it. But `gopls` is on PATH and subagents have Bash, so the CAPABILITY is
available to them.

Every skill said "you have no LSP tool" and stopped there, which sent agents to
read whole files. Two corrections, and the second is the durable one:

1. Tell them the fallback: `gopls symbols|definition|references` from Bash.
2. **Write it as a check-and-fall-back, never as a statement of absence.** Which
   contexts carry the tool is a property of the harness build and the machine --
   there are two dev machines here and they differ. "Try the tool, else use
   `gopls`" is correct on both, and survives a harness change. A rule asserting
   absence is falsified by moving machines.

`.mcp.json` registers `gopls mcp` as a stdio MCP server, which agents can reach.
It needs a one-time interactive approval and a fresh session, so it was NOT
proven working when this was written.

## Files

- `scripts/dev/token_economy.py` - the measurement tool; `assign_owners`, `_merge`, `capped_counterfactual`, `render`
- `scripts/dev/token_economy_test.py` - its tests, each mutation-proved to discriminate
- `ai/rules/context-economy.md` - the rule, trigger-routed rather than always-on
- `ai/rules/planning.md` - supervisor thinness, one agent per implementation phase
- `ai/skills/ze-implement.md` - phase decomposition, handoff through the per-spec state file
- `.claude/hooks/subagent-context.sh` - the directives every spawn receives
- `.claude/rules/session-start.md` - a loaded schema is not a working server
- `scripts/dev/dev-setup.py` - `gopls` in `REQUIRED_TOOLS`, plus `gopls_status`
- `mk/inventory.mk` - `ze-token-economy`, `ZE_CONTEXT_CAP`
- `.mcp.json` - `gopls mcp` registration

## Known Limitations

- The store is machine-local, so the figures describe one developer's machine.
  A fresh checkout reports nothing and says so rather than printing zeros.
- The phase split is a keyword heuristic over each spawn's description. Nothing
  in the store records the phase an agent ran, and the tool's own output says so.
- The capped-context counterfactual is arithmetic over calls that already
  happened. A session run under a smaller context would have made different
  calls; it is never a prediction.
- No gate can prove an agent batched independent calls or read a range instead
  of a file. The tool measures the aggregate after the fact.
- The 600k main-thread handoff threshold is unobservable from inside the thread
  it governs, so it can only ever be self-asserted.
