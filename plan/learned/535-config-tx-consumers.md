# 535 -- Config Transaction Consumers

## Context

The config transaction protocol infrastructure (journal, orchestrator types, txLock,
SDK callbacks) was built in spec-config-tx-protocol. The next step was wiring the 5
system plugins (BGP, interface, sysrib, fib-kernel, fib-p4) as transaction consumers
with OnConfigVerify, OnConfigApply, and OnConfigRollback callbacks using the SDK journal.

## Decisions

- Chose journal wrapping at the callback level (register.go OnConfigApply) over
  modifying the core apply functions (config.go, sysrib.go, fibkernel.go). The journal
  wraps the entire applyConfig call, not individual operations inside it. This avoids
  touching stable apply logic that is shared with the startup path.
- Chose to remove the direct reactor VerifyConfig/ApplyConfigDiff calls from reload.go
  over keeping dual paths. BGP now participates via SDK callbacks like the other 4 plugins.
  No-layering rule: old path replaced, not kept alongside new.
- Chose BGP-last apply ordering (explicit in reload.go) to preserve the semantic that
  other plugins (sysrib, interface) have config applied before BGP peers reconcile.
- Chose ConfigJournal interface in registry package over importing sdk in the reactor.
  Avoids adding a dependency from reactor to the plugin SDK. sdk.Journal satisfies
  the interface structurally.
- Chose single `interface/rollback` bus event over per-operation undo events
  (`interface/deleted`, `addr/removed`). applyConfig is monolithic; per-operation
  events would require refactoring it. Downstream consumers re-query state on rollback.

## Consequences

- All 5 config-owning plugins now declare WantsConfig and VerifyBudget/ApplyBudget
  at Stage 1 registration. This makes them visible to the reload coordinator.
- reconcilePeers always uses a journal internally. On partial failure, all successful
  peer operations are rolled back. This is strictly better than the old behavior
  (partial state on failure).
- The implementation uses sequential RPC dispatch, not the bus-native concurrent
  protocol described in transaction-protocol.md. A follow-up spec
  (spec-config-tx-bus-native.md) covers the migration from RPCs to bus events.
- fib-kernel and fib-p4 journals are protocol compliance no-ops. These plugins react
  to bus events from sysrib, not to config directly.

## Gotchas

- WantsConfig must be set in sdk.Registration (Stage 1 runtime) for a plugin to
  receive config-verify/config-apply RPCs during reload. ConfigRoots in
  registry.Registration (init-time) is for auto-loading only. Missing WantsConfig
  means the plugin's OnConfigVerify/Apply handlers are dead code.
- Rollback state (previousDist for sysrib, activeCfg for interface) must be initialized
  from OnConfigure, not left nil. Otherwise the first reload rollback restores to empty
  state instead of to the startup config.
- The design doc (transaction-protocol.md) describes a bus-native protocol with
  concurrent delivery, transaction IDs, deadlines, failure codes, and recovery.
  The implementation is sequential RPCs with journals. The doc is the target; the
  code is an intermediate step.

## Files

- `internal/component/plugin/registry/interfaces.go` -- ConfigJournal interface, BGPReactorHandle methods
- `internal/component/bgp/reactor/reactor_api.go` -- reconcilePeersJournaled, peerDiffCount, internalJournal
- `internal/component/bgp/reactor/reactor.go` -- PeerDiffCount, ReconcilePeersWithJournal public methods
- `internal/component/bgp/plugin/register.go` -- OnConfigVerify/Apply/Rollback, WantsConfig, budgets
- `internal/component/iface/register.go` -- journal-wrapped OnConfigApply, OnConfigRollback, WantsConfig
- `internal/plugins/sysrib/register.go` -- journal-wrapped admin distance apply, OnConfigRollback
- `internal/plugins/fibkernel/register.go` -- protocol compliance callbacks, WantsConfig
- `internal/plugins/fibp4/register.go` -- protocol compliance callbacks, WantsConfig
- `internal/component/plugin/server/reload.go` -- removed direct reactor calls, BGP-last ordering
- `docs/architecture/plugin/plugin-relationships.md` -- transaction participation rows
- `internal/component/bgp/reactor/reactor_api_test.go` -- 5 journal tests
- `internal/component/iface/config_test.go` -- 4 journal tests
- `internal/plugins/sysrib/sysrib_test.go` -- 1 journal test
- `internal/plugins/fibkernel/fibkernel_test.go` -- 1 journal test
- `internal/plugins/fibp4/fibp4_test.go` -- 1 journal test
- `test/reload/test-tx-bgp-rollback.ci` -- functional test
- `test/reload/test-tx-iface-apply.ci` -- functional test
- `test/reload/test-tx-iface-bgp-chain.ci` -- functional test
