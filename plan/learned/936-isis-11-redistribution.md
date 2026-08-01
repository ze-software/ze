# 936 -- isis-11-redistribution

## Context
Spec `isis-11-redistribution` wires the native IS-IS engine into Ze's
protocol-agnostic redistribution framework in BOTH directions (umbrella AC-7/AC-8):
IS-IS SPF routes out to BGP (producer), and connected/static/BGP routes into IS-IS
LSPs as Extended IP Reachability (consumer). It layers on isis-9 (SPF + LSP
origination + the separate Loc-RIB FIB install) and isis-6 (LSDB origination). The
framework (source registry, consumer registry + `RedistConsumer`, the
redistribute-orchestrator, loop prevention) already shipped; the IS-IS-specific work
was registration plus a TLV 135 write, exactly as the spec's Core Insight predicted.
The implementation is DONE and verified on darwin: whole tree builds (darwin + linux),
all 29 unit tests in `internal/component/isis/redistribute/...` pass under `-race`,
golangci-lint clean. Interop validation (FRR isisd over raw L2) is pending Linux
execution.

## Decisions
- **Single `isis` source/consumer, not per-level `isis-l1`/`isis-l2`.**
  `redistevents.RouteChangeBatch` has no level field, the orchestrator derives the
  source purely from `ProtocolName(b.Protocol)`, and the generic loop-prevention
  evaluator matches `route.Origin == importingProtocol`. Two protocol IDs would break
  self-import auto-rejection (AC-10). A single name keeps self-import rejected and
  matches the single admin distance. Both L1 and L2 SPF routes go in one batch.
- **Reuse the single `spf.ProtocolID()` identity** for the redistevents producer
  rather than allocating a second `RegisterProtocol("isis")`. isis-9 already allocated
  it for the Loc-RIB install Source; `redistribute/events` reuses it so the batch's
  `Protocol`, the orchestrator's source resolution, and the Loc-RIB Source are all one
  `isis` identity. The producer wiring is the four mandatory parts (RegisterProtocol
  reuse + `RegisterProducer` + typed `events.Register` handle + EMIT), mirroring
  `internal/plugins/connected/events`.
- **Producer-side registration MUST be `RegisterProducer`, not just `RegisterSource`.**
  Registering only the config `RouteSource` feeds the YANG validator/completion but the
  orchestrator subscribes solely to `redistevents.Producers()`. Without
  `RegisterProducer` no IS-IS route ever reaches BGP. `TestISISProducerRegistered`
  (AC-11) is the regression guard that asserts `Producers()` contains the isis ID.
- **Source registered at init, consumer at OnStarted.** The single config source is
  registered in `registerISIS` (init) so `ze config validate` -- which links all
  components but does NOT start the engine -- accepts `import isis` (AC-2/AC-9). The
  consumer needs the engine handle, so it registers at `OnStarted`.
- **Consumer writes TLV 135 via a narrow `LSPInjector` seam.** The redistribution
  package owns the per-prefix bookkeeping; the engine (root package, `redist_wiring.go`)
  implements `OriginationLevels`/`SetRedistPrefix`/`RemoveRedistPrefix`/`Originate`
  (+ V6 twins). Inject -> set prefix at every origination level -> re-originate ->
  flooding -> peer SPF. No direct route push, no side channel.
- **No external bit on IPv4 TLV 135.** RFC 5305 sec 4: the TLV 135 control octet has
  only up/down + sub-TLV-present. Redistributed IPv4 routes are ordinary entries with
  up/down CLEAR on first injection (set to 1 only on a down-level leak, RFC 2966, done
  by SPF leaking, not the consumer). The structural guarantee is that `lsdb.PrefixInfo`
  has no external field; the external (X) bit exists only on IPv6 TLV 236.
- **Fixed default redistribution metric** (`DefaultRedistMetric = 100`, a code
  constant, no YANG leaf): the generic `RouteEntry` carries no metric. A
  configurable/per-route metric is future work.

## Consequences
- `redistribute { destination bgp { import isis } }` exports IS-IS SPF routes (both
  levels) to BGP; `redistribute { destination isis { import connected|static|bgp } }`
  injects those into IS-IS LSPs as TLV 135; `destination isis { import isis }` validates
  at the schema level but is a runtime no-op (self-import rejected).
- Own enabled/passive interface prefixes are advertised as internal TLV 135 reachability
  (passive forms no adjacency, RFC 1195) via `refreshConnectedPrefixes`.
- This spec OWNS and registers four counters:
  `ze_isis_redist_injected_total{source,afi}`, `ze_isis_redist_withdrawn_total{source,afi}`,
  `ze_isis_redist_inject_failures_total{source}`, `ze_isis_lsp_reoriginations_total{level}`.
- IPv6 redistribution (TLV 236, `ipv6.go`) landed alongside, formally owned by isis-12;
  the shared `emitDeltaFamily` generalizes the producer over AFI so the IPv4 path is
  complete and IPv6 reuses the same seams.

## Gotchas
- **The generic `WithdrawRoute` carries NO source.** The orchestrator does not thread
  the source through the withdraw path, so the consumer remembers the source at inject
  time (`rememberSource`) and recovers it on withdraw (`forgetSource`). Without this the
  withdrawn/failure metrics would always be labeled `source="unknown"`. Regression
  guards: `TestISISRedistConsumerWithdrawMetricSource{,V6,Unknown}`.
- **Consumer re-registration on SDK reconnect.** `OnStarted` can re-fire and create a
  fresh engine instance. A plain `RegisterConsumer` then fails with
  `ErrConsumerConflict` and redistribution into IS-IS silently stops for the new engine.
  Use `configredist.ReregisterConsumer`, which replaces the stale consumer.
- **Two distinct paths, do not conflate.** The FIB/kernel install of IS-IS routes is
  isis-9 (SPF -> Loc-RIB `locrib.Path` -> sysrib -> fibkernel). THIS spec is purely
  redistribution: producer emits `redistevents.RouteChangeBatch` to the orchestrator,
  consumer writes LSP TLVs. `redistevents` NEVER programs the kernel.
- **The live route flow needs Linux raw L2.** A redistributed prefix appearing in a
  peer's RIB, and an IS-IS route appearing in BGP, require AF_PACKET veth + FRR isisd.
  The darwin functional test `test/isis/isis-redist-bgp.ci` exercises the CONFIG SURFACE
  only (validate accepts the both-directions config + the self-import no-op); the producer
  emit-on-bus and consumer TLV-inject halves are unit-tested with a fake bus / fake
  injector. The end-to-end peer-observable assertions live in the interop scenario.

## Interop validation pending Linux execution
- `test/interop/scenarios/isis-redist-frr/` is written (check.py, frr.conf, ze.conf,
  README.md present) but was NOT executed: this session ran on a darwin host. The
  scenario proves FRR isisd installs the reachability Ze redistributed into IS-IS
  (connected/static/BGP -> TLV 135, up/down bit honoured) and that an IS-IS route Ze
  learns reaches a BGP peer (and is withdrawn on removal). It runs only under the Linux
  Docker/QEMU interop harness and depends on the IS-IS-aware peer class delivered by
  isis-13 (`wait_adjacency`/`has_isis_route`); the config + assertions here are the
  contract that harness must satisfy. The interop-affected ACs (AC-1 live-route-in-BGP,
  AC-3 peer-RIB, AC-6 peer-kernel-withdraw) each have full unit + config coverage on
  darwin; only their peer-observable legs await Linux execution.

## Files
- `internal/plugins/isis/redistribute/events/events.go` (+test): redistevents PRODUCER
  wiring -- reuses `spf.ProtocolID()`, `RegisterProducer`, typed `RouteChange` handle.
- `internal/plugins/isis/redistribute/source.go` (+test): single config source `isis`
  (`RegisterISISSources`/`sync.Once mustRegister`), `Source.OnSPFChange` emit, connected
  helpers (`ConnectedPrefixInfos`).
- `internal/plugins/isis/redistribute/consumer.go` (+test): `RedistConsumer` impl
  (`InjectRoute`/`WithdrawRoute` -> TLV 135), source-remember/recover, owned counters.
- `internal/plugins/isis/redistribute/redistribute.go`: `DefaultRedistMetric` +
  `LSPInjector` seam (added).
- `internal/plugins/isis/redistribute/ipv6.go` (+test): IPv6 TLV 236 twin (formally
  isis-12) + shared `emitDeltaFamily`.
- `internal/plugins/isis/redist_wiring.go`: engine `LSPInjector` impl,
  `refreshConnectedPrefixes` (enabled+passive enumeration), SPF OnChange -> producer.
- `internal/plugins/isis/register.go` (modified): source at init, consumer
  (`ReregisterConsumer`) + producer at OnStarted, metrics.
- `test/isis/isis-redist-bgp.ci`, `test/isis/isis-redist-arbitration.ci`: config-surface
  functional tests.
- `test/interop/scenarios/isis-redist-frr/`: FRR isisd interop (Linux-pending).
- Docs: `docs/features.md`, `docs/guide/configuration.md`, `docs/guide/isis.md`,
  `docs/architecture/wire/isis.md`, `docs/plugin-development/metrics.md` (4 counters),
  `docs/plugin-overview.md`, `docs/comparison.md`, `docs/functional-tests.md`.
