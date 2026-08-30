# On-Demand Route Origination

Operators announce BGP routes at the CLI for DDoS mitigation and for
maintenance. The runtime announce path was fire-and-forget: nothing recorded
what had been announced, so nothing could withdraw it by reference.

<!-- source: internal/component/bgp/plugins/cmd/announce/registry.go -- Registry, tagEntry -->
<!-- source: internal/component/bgp/plugins/cmd/announce/announce.go -- announceRegistry, getOrInitRegistry -->

## The decisions

**The verb covers every BGP family, not FlowSpec only.** `AnnounceNLRIBatch` is
family-agnostic, so a FlowSpec-only CLI verb would have been an artificial
limit. `blackhole` is sugar for `unicast ... community blackhole`. It is not a
separate mechanism.

**Tracking is a generic `tag <key> <value>` annotation.** `track <name>` and
`label <name>` were rejected. The plugin API already carries `Meta map[string]any`
end to end, so a `tag.` prefix inside `Meta` gives cross-path tracking with no
new RPC field. The `config-static` key is the only other `Meta` consumer, so the
prefix cannot collide with it.

**The registry is in-memory and wraps the announce call. The reactor stays
stateless.** Tracking inside the reactor was rejected. The registry is the only
new state, and it is the shared code path for the CLI, for the plugin API
(`tag.` meta) and for the future flowspec-egress bridge.

**Every withdraw starts with a keyword, and each keyword is its own command.**
The three commands are `withdraw tag <key> [value <value>]`, `withdraw id <id>`
and `withdraw all [selector <selector>]`. Bare positional arguments were
rejected for clarity.

They were one command until 2026-08-29, with the three forms behind a keyword
switch on `args[0]`. That put the grammar in a handler and in a description,
where no completion could offer it and no generated usage line could render it.
Splitting it moved the grammar into the model, and the switch was deleted rather
than kept beside the three (`ai/rules/no-layering.md`). The handler functions did
not change: each already took the tail after its keyword.

**The tag value is OPTIONAL, and `withdraw all` takes a selector filter.** Both
were true in the code and false in the prose that documented it. An absent value
withdraws every value of the key, and an absent selector withdraws every
announcement.

An `UpdateRoute` call that carries no `tag.` meta stays fire-and-forget.

## Constraints the code does not state

**The dispatcher strips the tokens it matched, so `ze:command` decides what
`args[0]` is.** `matchCommandTokens`
(`internal/component/plugin/server/command.go`) returns the tokens after the
command's own path. A `ze:command` on `announce > unicast` means the handler
never receives the family keyword, which is what each form's handler wants: it
read `args[1:]` behind the switch and now reads `args`.

Both verbs carry a command on every sub-container. Put the command where the
grammar divides, and the model states the division rather than the handler.

`announce` kept its command on the parent until 2026-08-30 and switched on
`args[0]` itself. The cost fell on the model rather than on the code. One node
states one grammar and the three forms take three, so the generated usage line
named a first token no operator types and spelled the rest as `<args>`. The
split states unicast and blackhole in full. `announce flowspec` still spells its
own form: its match components are the FlowSpec codec vocabulary, which the
flowspec NLRI plugin owns and this module must not restate, and its action is a
mandatory choice where one branch carries a value, which no modifier states.

**A leaf whose name equals its container arrives as a SELECTOR, not in args.**
`withdraw > id` declares `leaf id`, so `matchCommandTokens` matches the keyword
against the leaf of the same name and lifts the value out of the argument list.
`handleWithdrawID` reads `ctx.Selector("id")`, which is the route
`request l2tp outgoing-call remote <remote> called <called>` already takes.

**`Registry.List` returns pointers to live entries.** Every caller treats them
as read-only.

**`globalRegistry` is guarded by a mutex, not by a lazy nil check.** Handler
goroutines run concurrently and race on first access without it.
