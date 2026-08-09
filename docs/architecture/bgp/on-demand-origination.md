# On-Demand Route Origination

Operators announce BGP routes at the CLI for DDoS mitigation and for
maintenance. The runtime announce path was fire-and-forget: nothing recorded
what had been announced, so nothing could withdraw it by reference.

<!-- source: internal/component/bgp/plugins/cmd/announce/registry.go -- Registry, tagEntry -->
<!-- source: internal/component/bgp/plugins/cmd/announce/announce.go -- handleAnnounce, getOrInitRegistry -->

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

**Every withdraw starts with a keyword.** The grammar is `withdraw tag <key>
<value|*>`, `withdraw tag *`, `withdraw id <N>` and `withdraw all`. Bare
positional arguments were rejected for clarity.

An `UpdateRoute` call that carries no `tag.` meta stays fire-and-forget.

## Constraints the code does not state

**`ze:command` goes on the parent container, never on a sub-container.** The
dispatcher strips the tokens it matched. A `ze:command` on `announce > unicast`
means the handler never receives the family keyword as `args[0]`. Put it on
`announce`.

**`Registry.List` returns pointers to live entries.** Every caller treats them
as read-only.

**`globalRegistry` is guarded by a mutex, not by a lazy nil check.** Handler
goroutines run concurrently and race on first access without it.
