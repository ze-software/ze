# Spec: cli-pipe-column-modifiers

| Field | Value |
|-------|-------|
| Status | future |
| Scope | cli |
| Depends | - |
| Phase | 0/1 |
| Deferral shard | `plan/deferrals/cli-order-pipe.md` |
| Handoff | - |
| Updated | 2026-08-19 |

## Task

`spec-cli-order-pipe` shipped two column operators, `| display <field>...` and
`| fill [alpha] [reverse]`. It named two modifiers it deliberately left
out, and this spec is where both live until somebody decides they run. Neither
is started. Both rows are recorded in `plan/deferrals/cli-order-pipe.md`.

## The two deferred modifiers

### 1. Exclusion, so an operator names what to DROP

Today "every column except these two" means naming the other seventeen. This is
the modifier `spec-cli-order-pipe` judged worth adding next, ahead of any
positional wildcard variant.

Open questions a design phase owes an answer to:

- Which operator carries it. A third operator (`| hide <field>...`) keeps the
  one-argument-kind rule. A prefix on a field name inside `| display` breaks
  that rule, because a token would then be a field name in one spelling and a
  modifier in another.
- What it means beside `| display`. Naming a field in both is a contradiction
  the grammar has to answer rather than resolve by accident.
- Whether it selects in the programmatic formats. `| display` does, because
  which fields to answer with is a data question the operator asked out loud
  (`docs/features/formatting.md`). The same argument reaches an exclusion.

### 2. Row ordering, which is a different axis entirely

Sorting the ROWS of an answer by a field's value is neither `| display` nor
`| fill`: both order and select COLUMNS. `knownPipeOps`
(`internal/component/command/pipe.go`) holds no operator that sorts rows at all.

Open questions:

- Its own name, because folding it into `display` would make one operator carry
  two axes.
- Where it applies. `renderList` has rows in an array, `renderMapOfMaps` has
  rows keyed by a parent value that already sorts alphabetically, and
  `renderRecord` has no rows at all.
- Whether it is client-side over the dispatcher's JSON like every other generic
  operator, or a server-side fold for the commands that own pipe filters.

## Why this is parked rather than open

`spec-cli-order-pipe` closed with the syntax the owner specified and nothing
more (`ai/rules/simplicity.md`). Neither modifier blocks anything the shipped
operators do. This file exists so the two rows have a destination that names
the work rather than the word "later" (`ai/rules/planning.md`).
