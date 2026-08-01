# 1117 — test-sync-quiesce

Layer 1 of eliminating `sleep` from tests: a **quiesce barrier**. `request quiesce`
(`ze-system:quiesce`) blocks until every registered subsystem has drained its
pending asynchronous work, then replies, so a test does `send(change);
request quiesce; assert` with no fixed sleep. It is the general form of
`ze-bgp:peer-flush`.

Producers (verified first-hand):
- **Barrier core:** `internal/component/plugin/server/quiesce.go` — `Quiescer`,
  `QuiescerRegistry` (runtime `Register`/`All`, lock-guarded), `quiesceAll`
  (drains all quiescers concurrently, each bounded by a per-quiescer
  `context.WithTimeout`, aggregates errors, replies `StatusError` naming any
  stuck/failed subsystem), and `handleQuiesce`. Registered as an RPC via
  `RegisterRPCs(RPCRegistration{WireMethod: "ze-system:quiesce", Handler:
  handleQuiesce})` in `init()`.
- **Registry on the server:** `server.go` gains a `quiescers QuiescerRegistry`
  field; `NewServer` calls `registerReactorQuiescer(s, reactor)` which registers
  the reactor's `FlushForwardPool` (method value == `QuiesceFunc`) as
  `bgp-forward-pool`. `handleQuiesce` reads `ctx.Server.Quiescers()`.
- **YANG:** `ze-system-cmd.yang` maps `request quiesce` -> `ze-system:quiesce`;
  `ze-system-api.yang` declares `rpc quiesce` (output leaf-lists `quiesced`,
  `failed`).
- **Test SDK:** `test/scripts/ze_api.py` `quiesce()` sends the RPC.
- **Functional proof:** `test/plugin/quiesce-barrier.ci` (send two routes,
  `quiesce()`, peer asserts both on-wire, no sleep) — passes end-to-end.

Insight: the control plane is ALREADY synchronous (a dispatch reply lands after
its handler; `request bgp rib inject` recomputes best-path before replying).
Sleeps survive only on the async downstream planes (forward pool to peer sockets,
and later FIB/tc/listeners) that emit no completion signal. quiesce is the
completion signal, generalized over a registry so Layer 3 subsystems opt in with
zero handler change.

## GOTCHAS
- **`wait_for_ack` was NOT migrated (spec AC-6 deferred to follow-on).** Its
  trailing `time.sleep` also covers the PEER SIMULATOR's own `cmd=api`
  interleaving (EOR etc.), which the forward-pool quiesce does not drain. Removing
  it made `nexthop`-style tests race. Fully migrating needs a peer-side quiescer
  (Layer 2/3). `quiesce()` ships as the sleepless barrier for tests that don't
  depend on ze-peer timing.
- **Registering an RPC requires a YANG command mapping.** `TestEveryRPCHasYANGPath`
  fails ("RPC ze-system:quiesce has no YANG path mapping") until the `ze:command`
  is added to `ze-system-cmd.yang`; and `TestRPCRegistrationPerModule` asserts a
  stable ze-system RPC count (bump 13 -> 14). Both are wiring tests that catch a
  half-wired RPC.
- **Quiescer registration is runtime, not `init()`** — the drain closes over a
  live reference (the reactor), available only when the reactor attaches to the
  server (`NewServer`).
- Landed on a shared working tree alongside a concurrent netlink/copp session
  whose uncommitted WIP holds the ci-sleep + cli-grammar STRUCTURAL gates red, so
  the closure commit is blocked (commit_helper won't bypass a structural red)
  until that session's tree clears. Pre-existing/other-session functional reds
  (`nexthop` #278, `ddos-flowspec-announce` #154) are NOT this change — they fail
  with `wait_for_ack` reverted.

## Files

None recorded.
