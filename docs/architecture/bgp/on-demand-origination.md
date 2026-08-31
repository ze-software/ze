# On-Demand Route Origination

Operators announce BGP routes at the CLI for DDoS mitigation and for
maintenance. The runtime announce path was fire-and-forget: nothing recorded
what had been announced, so nothing could withdraw it by reference.

<!-- source: internal/component/bgp/plugins/cmd/announce/registry.go -- Registry, tagEntry -->
<!-- source: internal/component/bgp/plugins/cmd/announce/announce.go -- announceRegistry, getOrInitRegistry -->

## The decisions

**Announce is wire only. It reaches peers and it touches no RIB (owner
directive, 2026-08-31).** The route is not inserted in the Loc-RIB, so it does
not compete in cross-source best path and it does not reach the kernel FIB. That
is deliberate: the verb exists for DDoS mitigation and maintenance, where the
operator states what a peer must hear.

Every other rail that reaches a peer works the same way.
`(*BGPConsumer).InjectRoute` formats `update text ... nlri <family> add <prefix>`
and dispatches it, so a redistributed static or OSPF route re-enters through this
same announce rail. Export policy still applies: `exportFilterForBody` runs the
destination peer's export chain at the session write gate for every originated
route.

**A bare announce reaches every peer, so no form states
`RequiresSelector`.** `selector.ParseDefault` reads an empty selector as
`All()`. That is what an operator asks for when they name no peer. The flag
belongs to a command whose model carries a selector leaf to satisfy it.

`announce` carried such a leaf until 2026-08-30. Each form then became its own
command, and the leaf went with the split. The flag stayed. For that day every
form refused every invocation with `announce <form> requires a selector`. The
error named a token the model gave the operator no way to type.

**A peer selector is typed before the verb, in the ExaBGP order.**
`peer <selector> announce unicast <prefix>` reaches the peers the selector
matches. The forms are one `grouping`, instantiated at `announce` and at
`peer > announce`, so the second path restates no grammar.

The selector is declared on the `peer` container. `appendAnchored` then anchors
it to that keyword, which is the word the operator types it after. `anchoredDef`
binds it at dispatch. The peer-scoped command carries two mandatory pattern-less
strings, selector and prefix, so `implicitSelectorDef` sees two candidates and
answers nil.

<!-- source: internal/component/bgp/plugins/cmd/announce/announce.go -- the RPCRegistration block -->
<!-- source: internal/core/selector/selector.go -- ParseDefault -->
<!-- source: internal/component/bgp/reactor/egress_inject_filter.go -- exportFilterForBody -->
<!-- source: internal/component/bgp/redistribute/consumer.go -- formatAnnounce, InjectRoute -->

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
and `withdraw all`. Bare positional arguments were rejected for clarity.

They were one command until 2026-08-29, with the three forms behind a keyword
switch on `args[0]`. That put the grammar in a handler and in a description,
where no completion could offer it and no generated usage line could render it.
Splitting it moved the grammar into the model, and the switch was deleted rather
than kept beside the three (`ai/rules/no-layering.md`). The handler functions did
not change: each already took the tail after its keyword.

**The tag value is OPTIONAL.** It was true in the code and false in the prose
that documented it. An absent value withdraws every value of the key.

**One peer prefix scopes every withdraw form, and no form states a scope of its
own (owner directive, 2026-08-31).** `withdraw all` carried a
`selector <pattern>` leaf until then. One form of three took a scope and the
other two took none. `peer <selector> withdraw all` replaces it, and the same
prefix narrows `withdraw tag` and `withdraw id`.

The value is compared against the selector each announcement was MADE with,
rather than resolved against the peer table. An entry records the fan-out it went
to, so naming a peer asks for the announcements sent to that fan-out. An operator
who names a peer that received nothing withdraws nothing.

`withdraw tag *` and `withdraw all` walk one set rather than two. Only a tagged
announcement enters the registry, so every tracked announcement is a tagged one,
and `withdrawAll` answers both.

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
named a first token no operator types and spelled the rest as `<args>`.

**The vocabulary a command borrows is declared by the plugin that owns it.**
`announce flowspec` takes nineteen match components, and they belong to the
FlowSpec codec: `isComponentKeyword`
(`internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go`) is their
single producer, and `handleAnnounceFlowspec` never reads them. So the flowspec
plugin declares them itself, through an `augment` onto the announce module's
`flowspec` container, and the announce module never restates another plugin's
words. Two things follow that a reader will otherwise meet as a silent
behaviour. An augmented container states `config false` itself, because
`mergeYANGEntry` (`internal/component/config/yang/command.go`) drops a child
whose own `Config` is not false and goyang never propagates it from a parent.
And `declaredContainerOrder` counts the PARENT's own container statements, so
an augmented container's `ModifierOrder` is 0: augmented groups sort by name and
land ahead of every locally declared one, which is why the components print
before the action and the options.

**The action is an extended community, not an alternation.** RFC 8955 Section 7
says so, and `handleAnnounceFlowspec` already agreed before it was modeled: it
synthesizes `traffic-rate` arguments for `rate-limit` and a rate of zero for
`discard`, then hands both to `route.ParseExtendedCommunities`. Those two are
spellings of one thing rather than two branches of a choice, so the model states
`community <value>` as the general case and keeps them as sugar. Reading them as
a mandatory choice where one branch carries a value is what made this command
look inexpressible, and it is why it kept an authored sentence until 2026-08-30.

The generated line brackets all three, so it says an action is optional and the
handler says it is not: `splitFlowspecArgs` answers `errFlowspecRequiresAction`
for a tail that names none, and `handleAnnounceFlowspec` answers
`errMissingFlowspecComponents` for a tail with no component. A group states
`once` or `repeat`, and `required` is the wrong word for one member of a set
where any single member satisfies the rule. The operator reads the obligation
from the error rather than from the line, and closing that gap needs a modifier
that states "one of these", which no command declares today.

<!-- source: internal/component/bgp/plugins/cmd/announce/announce.go -- splitFlowspecArgs -->
<!-- source: internal/component/command/usage.go -- modifierChildren -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/yang/ze-flowspec-cmd.yang -- augment -->
<!-- source: internal/component/config/yang/command.go -- declaredContainerOrder -->
<!-- source: internal/component/config/yang/command.go -- mergeYANGEntry -->

**A leaf whose name equals its container arrives as a SELECTOR, not in args.**
`withdraw > id` declares `leaf id`, so `matchCommandTokens` matches the keyword
against the leaf of the same name and lifts the value out of the argument list.
`handleWithdrawID` reads `ctx.Selector("id")`, which is the route
`request l2tp outgoing-call remote <remote> called <called>` already takes.

**`Registry.List` returns pointers to live entries.** Every caller treats them
as read-only.

**`globalRegistry` is guarded by a mutex, not by a lazy nil check.** Handler
goroutines run concurrently and race on first access without it.
