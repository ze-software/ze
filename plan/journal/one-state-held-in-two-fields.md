# One piece of state is held in two fields, and each writer updates one of them

Two objects on one path each keep a field for the same fact, seeded from the
same source at startup. They agree for as long as nobody writes, so every test
that reads either one passes. Then one writer updates one copy and another
writer updates the other, and the two answer different questions about the same
running system. Nothing reports it: both fields hold a value of the right type,
and the reader that gets the stale one cannot tell.

The tell is a facade that delegates every method of an interface except the
accessors for one field, which it answers from itself.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-13 | yang-rpc-declarations-with-no-handler | `Coordinator.GetConfigTree` and `Coordinator.SetConfigTree` (`internal/component/plugin/coordinator.go`) against `reactorAPIAdapter.GetConfigTree` and `SetConfigTree` (`internal/component/bgp/reactor/reactor_api.go`) | The daemon held the running configuration TWICE. `NewCoordinator` was seeded with `tree.ToPluginMap()` and so was `reactor.Config.ConfigTree`: two equal maps with two identities. Every other method of the coordinator delegates to the registered reactor; these two answered its own field. The plugin server holds the COORDINATOR (`pluginserver.NewServer(serverConfig, coordinator)`, `cmd/ze/hub/main.go`), so `reloadConfig` read and replaced the coordinator's copy while the reactor's stayed at the tree it was built with, forever. A command handler holds the ADAPTER (`Server.Reactor` answers `Coordinator.FullReactor`), so `agreedSelector` (`internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go`) read a tree no commit had ever updated, and its comment states the opposite: the running config is read on each invocation rather than cached, so nothing has to be invalidated when the operator commits a new agreement. `recordASNotation`, whose comment says the reload coordinator calls it last on every reload, was reached by no reload at all. Found while building `update bgp config`: the command recorded a runtime peer through the adapter, the reload decomposed against the coordinator's stale copy, and a peer deleted at runtime made the next reload emit `remove-peer peer1` for a peer nothing was running, which failed the whole transaction | fixed at the source: the two accessors delegate to the reactor where one is attached, `SetReactor` hands the coordinator's tree over as the reactor attaches, and the local field answers only while no reactor is registered. `test/plugin/api-peer-save.ci` holds it end to end. It was RED on the split, with `reload error: config verify failed: operation bgp-remove-peer-peer1 verify failed`, and is green on the fix |
