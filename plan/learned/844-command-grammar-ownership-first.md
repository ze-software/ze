# 844 -- command-grammar-ownership-first

## Context

A command-grammar audit drifted into code changes before the ownership boundary was re-read from source. That produced two bad classes of edits:

- config-tree mutations were treated like runtime RPC verbs;
- BGP-specific peer grammar leaked into shared dispatch and generic tests.

The immediate symptoms were nonsense grammar such as `unit del` / `addr del`, public examples that invented unagreed peer mutations, and generic `plugin/server` logic that knew too much about BGP command spelling.

## Decisions

- Classify every command change first: config-tree mutation, runtime operation, or read/query.
- Keep config-tree mutation under engine `set` / `delete`; do not invent RPC verbs just to regularize grammar.
- Treat owner package and owner YANG module as blocking design inputs before changing syntax.
- Keep BGP public grammar in BGP-owned code and docs. Shared dispatch may carry selector scope, but it must not own BGP command spelling.
- Do not write speculative command examples in agent rules or patterns. Unagreed examples become false source material for later agents.

## Consequences

- Grammar cleanup must start with ownership evidence, not with mechanical normalization.
- Generic command infrastructure tests stay domain-agnostic; owner syntax tests live with the owner package.
- Handover notes are the right place to capture unfinished refactor direction without committing broken code.

## Gotchas

- `peer` is overloaded: it can be public BGP syntax and also an internal selector scope field. Do not confuse transport/state plumbing with user-facing grammar ownership.
- Full verification catches owner-boundary mistakes quickly because many plugin functional tests dispatch commands through generic plumbing.
- If a commit changes agent rules, patterns, or workflow guardrails, add a learned summary even when the product code refactor is not yet ready to commit.

## Files

- `ai/rules/cli.md`
- `ai/patterns/cli-command.md`
- `ai/rules/cli.md`
- `ai/INSTRUCTIONS.md`
- `handover/16-bgp-peer-dispatch-ownership-refactor.md`
