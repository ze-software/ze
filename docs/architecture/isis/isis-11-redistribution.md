# IS-IS Redistribution

IS-IS is wired into Ze's protocol-agnostic redistribution framework in both
directions: SPF routes out to other protocols (producer), and connected, static
and BGP routes into IS-IS LSPs as Extended IP Reachability (consumer).

| Concern | File |
|---------|------|
| Redistribute-events producer wiring | `redistribute/events/events.go` |
| Config source and connected helpers | `redistribute/source.go` |
| Consumer, TLV write, owned counters | `redistribute/consumer.go` |
| Default metric and the injector seam | `redistribute/redistribute.go` |
| IPv6 twin | `redistribute/ipv6.go` |
| Engine glue | `redist_wiring.go` |

## Decision: one `isis` identity, not per-level identities

The route-change batch has no level field, the orchestrator derives the source
purely from the protocol name, and the generic loop-prevention evaluator matches
the route origin against the importing protocol. Two protocol IDs would break
self-import auto-rejection.

One name keeps self-import rejected and matches the single admin distance. Both
level-1 and level-2 SPF routes travel in one batch.

<!-- source: internal/plugins/isis/redistribute/source.go -- RegisterISISSources, Source.OnSPFChange, ConnectedPrefixInfos -->

The producer **reuses** the protocol ID that the SPF layer already allocated for
its Loc-RIB install, rather than allocating a second one. The batch protocol, the
orchestrator's source resolution, and the Loc-RIB source are then all one
identity.

<!-- source: internal/plugins/isis/redistribute/events/events.go -- the producer registration and the typed route-change handle -->

## Decision: the producer must register as a producer

Registering only the config route source feeds the YANG validator and completion.
The orchestrator subscribes solely to the **producer** registry. Without that
call no IS-IS route ever reaches BGP, and nothing else fails. A named regression
test asserts the producer list contains the IS-IS ID.

## Decision: source at init, consumer at start

The config source registers from `init()`, because `ze config validate` links in
every component but does **not** start the engine, so `import isis` must validate
without a running engine. The consumer needs the engine handle, so it registers
at start.

## Decision: the consumer writes through a narrow injector seam

The redistribution package owns the per-prefix bookkeeping. The engine implements
the injector: report the origination levels, set the prefix at every level,
re-originate. The re-originated LSP then floods and a peer's SPF picks it up.
There is no direct route push and no side channel.

<!-- source: internal/plugins/isis/redistribute/consumer.go -- InjectRoute, WithdrawRoute, rememberSource, forgetSource -->
<!-- source: internal/plugins/isis/redist_wiring.go -- the injector implementation, refreshConnectedPrefixes -->

## Decision: no external bit on IPv4

RFC 5305 section 4 gives the TLV 135 control octet only the up/down bit and the
sub-TLV-present bit. A redistributed IPv4 route is an ordinary entry with up/down
clear on first injection; the bit is set to 1 only on a down-level leak (RFC
2966), which the SPF leaking path does, not the consumer.

The structural guarantee is that the prefix-info type carries no external field.
The external bit exists only on IPv6 TLV 236. Ze does not fabricate an IPv4
external marking the protocol lacks.

## Decision: a fixed default redistribution metric

The generic route entry carries no metric, so redistributed prefixes take a code
constant. A configurable or per-route metric is not implemented.

## Trap: the generic withdraw carries no source

The orchestrator does not thread the source through the withdraw path. The
consumer therefore remembers the source at inject time and recovers it on
withdraw. Without that, the withdrawn and failure counters would always be
labeled with an unknown source.

## Trap: consumer re-registration on an SDK reconnect

The start callback can re-fire and build a fresh engine. A plain consumer
registration then fails with a conflict and redistribution into IS-IS silently
stops for the new engine. Use the re-register call, which replaces the stale
consumer.

## Trap: two distinct paths, do not conflate them

The FIB install of IS-IS routes is SPF to Loc-RIB to sysrib to the FIB backend
(see [`isis-9-spf-rib.md`](isis-9-spf-rib.md)). Redistribution is a different
path: the producer emits a route-change batch to the orchestrator, and the
consumer writes LSP TLVs. Redistribute events never program the kernel.

## Owned metrics

`ze_isis_redist_injected_total{source,afi}`,
`ze_isis_redist_withdrawn_total{source,afi}`,
`ze_isis_redist_inject_failures_total{source}`, and
`ze_isis_lsp_reoriginations_total{level}`.

## Coverage boundary

The functional test exercises the **config surface** only: validate accepts the
both-directions config and the self-import no-op. The producer emit and the
consumer TLV write are unit-tested with a fake bus and a fake injector. A
redistributed prefix appearing in a peer's RIB, and an IS-IS route reaching a BGP
peer, need raw Layer-2 and live in the interop scenario.
