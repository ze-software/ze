# Spec: cli-retired-command-hint

| Field | Value |
|-------|-------|
| Status | future |
| Scope | cli |
| Depends | - |
| Phase | 0/1 |
| Deferral shard | `plan/deferrals/cli-show-bgp-is-the-command.md` |
| Handoff | - |
| Updated | 2026-08-21 |

## Task

`spec-cli-show-bgp-is-the-command` retired `show bgp summary`, the first
command this repository has removed. An operator who types it gets the
unknown-command error and nothing else: the string they typed "names no
subcommand and no address family: unknown command" (`handleBgpOverview`,
`internal/component/bgp/plugins/cmd/peer/summary.go`).

The work deferred here is the hint: tell that operator what replaced the name
they typed. It is recorded in `plan/deferrals/cli-show-bgp-is-the-command.md`
and it is not started.

## What is deferred

A retired command answers as though it never existed. The operator learns that
the string is unknown, not that `show bgp` now answers what they were asking
for. Muscle memory is what a removal costs, and a hint is what pays it back.

## Why it was not built with the removal

Building it for one command would either hard-code that one case or invent a
general facility inside a spec whose subject was a different thing. The
facility is general and the removal was not.

## Open questions a design phase owes an answer to

- **Where the retired names live.** No registry holds a record of what a name
  became. `commandRegistry` (`internal/component/command/column_order.go`) and
  the dispatch registries in `internal/component/plugin/server/command.go` hold
  only what exists today, by design: a removed command is removed.
- **How long a retired name stays known.** A record that never expires becomes
  a second command tree nobody prunes. A record that expires needs a rule for
  when, and a release the rule is measured from.
- **Where the hint is produced.** `handleBgpOverview`
  (`internal/component/bgp/plugins/cmd/peer/summary.go`) answers the
  unknown-command error for this one case, but a general hint belongs at the
  dispatcher, beside `matchBuiltinTokens`, where every unknown command arrives.
- **Whether a hint is owed for a RENAME as well as a removal.** The two look the
  same to the operator and differ in whether anything answers the old path.

## Related

- `docs/guide/command-reference.md` carries the release note for this one
  removal. A hint would not replace that note; it would reach the operator who
  never reads it.
