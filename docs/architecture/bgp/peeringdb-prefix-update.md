# update bgp peer prefix: PeeringDB refresh through the editor

`update bgp peer <selector> prefix` fetches PeeringDB max-prefix data and
proposes the config change. It was removed once, as collateral damage in a
command-ownership refactor that grouped it with config-mutation commands that
bypassed the editor. The grouping was wrong: this is a data-refresh command.
Without it, an operator sees a login warning about stale prefix data and has no
action to take.

<!-- source: internal/component/bgp/plugins/cmd/peer/prefix_update.go -- the handler -->

## The decisions

**The handler writes through the draft flow, never directly.** It opens an
`EditSession("peeringdb", "api")`, calls `SetSession()`, then `SetValue()`, then
`SaveDraft()`. The operator commits with `config commit`. The original code
called `Save()`, which wrote the config file directly, and that is the behavior
whose removal was correct.

**The wire method stays `ze-update:bgp-peer-prefix`**, unchanged from the
original, for API stability.

**The `rpc bgp-peer-prefix` declaration moved into the BGP peer command
package's own YANG.** It was orphaned in the generic CLI update API module.
Removing the BGP peer command package now removes its RPC declaration too.

**`container update` is merged onto the `update` verb root** from the peer
command YANG, the same container-merge pattern `bgp-filter-irr` uses.

The `prefix-updated` timestamp and the `prefix-stale` display in
`show bgp peer detail` are populated by this command.

## Constraint

**`SaveDraft()` requires a session and `Save()` refuses one.** `SaveDraft()`
returns an error when no session is set. `Save()` returns an error when a
session IS set. Any future RPC handler that writes config through the editor
creates an `EditSession` and calls `SetSession()` first.
