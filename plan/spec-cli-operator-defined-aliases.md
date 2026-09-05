# Spec: cli-operator-defined-aliases

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | 0/1 |
| Handoff | - |
| Updated | 2026-08-19 |

## Task

`spec-cli-pipe-aliases` shipped the pipe alias mechanism. An alias is a name an
operator types in place of an operator chain, and every alias that exists is
registered in Go at init: `RegisterAliases`
(`internal/component/command/alias.go`) is called from `registerAliases`
(`internal/component/bgp/plugins/cmd/peer/peer.go`) with two literal `Alias`
values.

That spec named operator-defined aliases as out of scope, and this spec is
where the work lives until somebody decides it runs. It is not started. The row
is recorded in the retired deferral shard "cli-pipe-aliases".

## What is deferred

An operator who wants a third alias cannot have one. They edit no
configuration, they write Go and rebuild. The two that ship are the two the
`show bgp` payload happens to split into, and nothing lets a site name
the fields it looks at every morning.

## The blocker is plumbing, not schema

`container cli` (`internal/component/hub/yang/ze-hub-conf.yang`) can express a
keyed list, and `list ... key "name"` is standard in this tree. Expressing the
alias table in YANG is the easy half.

The hard half is that **no list-valued config reaches
`internal/component/command` today**. The one existing `environment cli` leaf
arrives there as a scalar env string, through `env.MustRegister` and
`configuredDefault` (`internal/component/command/pipe.go`). Building a path
that carries a keyed list into that package is a larger change than the alias
mechanism itself was.

## Open questions a design phase owes an answer to

- **Which registry the operator's aliases join.** `RegisterAliases` writes two
  tables, `aliasRegistry` for a command path and `globalAliases` for every
  command. An operator's alias needs to say which it wants, or the spec has to
  choose one for them.
- **What replaces the registration-time refusals.** All four refusals in
  `checkedAlias` are `panic("BUG:")`, which is correct while only this
  repository can register an alias. An operator's typo must become a config
  validation error instead, and the collision checks (`filterShadowing`,
  `aliasShadowing`) have to run at config-apply time rather than at init.
- **What happens at reload.** The Go tables are written once at init and never
  cleared outside `ResetAliasesForTest`. A reload that changes the alias list
  needs a replace that is safe while a session is mid-chain.
- **Whether an operator's alias may shadow a shipped one.** The longest-prefix
  rule already answers a command-specific alias beating a global one. It does
  not answer an operator's alias meeting a Ze one on the same path.

## Known Limitations carried forward

`spec-cli-pipe-aliases` also left these, and they are not this spec's subject:

- An alias takes no argument.
- Alias completion offers names but not their expansions.

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `cli-pipe-aliases.md`, 2026-08-19

Deferred by spec-cli-pipe-aliases.

Operator-defined aliases in configuration, so an operator can name their own pipe expressions rather than only using the ones registered in Go at init
