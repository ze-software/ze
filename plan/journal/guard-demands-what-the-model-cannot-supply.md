# Guard demands a value the model gives no way to type

A registration flag says a command needs a value before it runs. The model
declares the leaf that carries the value. The model is then reshaped and the
leaf goes. The flag stays, because nothing binds the two. One lives in a Go
registration and the other in a YANG container.

The command then refuses every invocation. The refusal names a token no
operator can type.

A unit test over the handler never sees this. The guard runs before the
handler. Tests that prove the handler correct call it directly. The population
that would go red is the functional test the surface does not have.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-31 | - | `announce unicast`, `announce blackhole`, `announce flowspec` | All three forms refuse every invocation with `announce <form> requires a selector`. `init` (`internal/component/bgp/plugins/cmd/announce/announce.go`) registers each one with `RequiresSelector: true`. The guard in `Dispatch` (`internal/component/plugin/server/command.go`) then fails the command unless the `selector` leaf is bound, or `ctx.Peer` is not empty. Before the split, `container announce` carried `leaf selector { mandatory true; }`. The flag had a leaf to read. `274e8e013` made announce three commands, and none of the three declares a selector leaf. `validateCommandArgs` binds names from `matchedCmd.ArgDefs`, so it can never produce one. The one remaining source is `rpcParams.Selector` (`internal/component/plugin/server/server.go`), which no client in the tree sets. Whether the pre-split form reached a peer is UNVERIFIED. `matchBuiltinTokens` fills interior selector slots only, and the pre-split selector sat terminal, so this can predate the split. Measured on a live daemon through a plugin `dispatch-command`: `announce unicast 198.51.100.0/24 next-hop 10.0.1.254 community 65001:666` answers `status=error ... requires a selector`. `docs/guide/configuration.md` documents that exact form with no selector | FIXED 2026-08-31, both halves, on the owner's directive. Announce is wire only. A bare announce reaches every peer, and `peer <selector> announce` reaches the peers the selector matches. `RequiresSelector` is off the three forms, so the bare path answers. The peer-scoped path is a second instantiation of one `grouping announce-forms`, under a `peer` container that declares the selector. The flowspec components became a grouping that each of two augments uses. `anchoredDef` (`internal/component/plugin/server/command.go`) binds the value at dispatch. Without it the peer-scoped command carries two mandatory pattern-less strings, so `implicitSelectorDef` answers nil. The operator who copies the generated line is then told `unknown command`. Proven by `TestAnchoredSelectorResolvesThePeerScopedPath`, shown red by removing the anchor preference, and by `test/ui/test-announce-forms-are-separate-commands.ci` over both paths. Found during the `.ci` coverage the split's closure commit reported as owed |
