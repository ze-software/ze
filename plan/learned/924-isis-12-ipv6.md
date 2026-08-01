# 924 -- isis-12-ipv6

## Context
Spec `isis-12-ipv6` makes the native IS-IS engine dual-stack (RFC 5308,
single-topology): originate TLV 232 (IPv6 interface address) + TLV 236 (IPv6
reachability) + NLPID 0x8E (TLV 129), run IPv6 route extraction over the SHARED
SPF tree, install IPv6 routes via the same Loc-RIB path as IPv4, and redistribute
IPv6 both ways. Like isis-8, this was INTEGRATION onto an already-built engine
(isis-1..isis-11 had landed from sibling agents -- the task brief's "from scratch,
no internal/component/isis/ exists" was stale). The TLV 232/236 codec already
existed (isis-2); this spec is pure runtime wiring of origination + SPF leaf
extraction + install + redistribution.

## Decisions
- **Single shared SPF tree, second leaf+install pass.** The IPv6 path reuses the
  SAME `results`/`graphs` the IPv4 Dijkstra produces. `Computer.Run` does one tree
  build per level, then `BuildRoutes` (TLV 135) AND `BuildRoutesV6` (TLV 236) over
  that tree, each feeding its own family Installer. No second Dijkstra. `BuildRoutesV6`
  is a near-clone of `BuildRoutes` reading `node.PrefixesV6` and applying the
  MAX_V6_PATH_METRIC filter; the multi-level arbitration (preferenceRank /
  candidate.better) is SHARED unchanged (RFC 5308 sec 5 order == IPv4).
- **Installer parameterized by family+afi.** `NewInstaller` (IPv4) and
  `NewInstallerV6` (IPv6) both call a shared `newInstaller(loc, fam, afi)`. The only
  per-AF differences are the Loc-RIB `family.Family` and the `afi` label on
  `ze_isis_routes_installed`. Same insert/diff/ECMP/remove logic. The IPv6 install is
  the SAME FIB path (locrib.Path, Source=IS-IS ProtocolID, AdminDistance 115), NOT
  redistevents.
- **IPv6 next-hop = neighbor link-local from TLV 232.** A second resolver interface
  `NextHopResolverV6` + engine adapter `engineNextHopResolverV6` reads `row.IPv6`
  (the adjacency already stored the neighbor's IIH TLV 232 link-local -- sibling specs
  anticipated isis-12) and carries the circuit name as the interface (a link-local
  next-hop is unusable without it, R-2).
- **RFC 5308 address scope enforced at origination, not in the codec.** TLV 232 in the
  IIH carries ONLY link-local (circuit layer, `circuit/hello.go ipv6InterfaceAddrTLV`
  reading `c.ipv6LinkLocal`); TLV 232 in the LSP carries ONLY non-link-local
  (`lsdb` LevelState.InterfaceAddrsV6, filtered by `NonLinkLocalV6Addrs`); TLV 236
  NEVER carries link-local (`NonLinkLocalV6Prefixes`, applied at the engine merge AND
  defensively at the redist consumer/connected helper). The codec round-trips whatever
  it is given.
- **Single redistribution source, AFI distinguishes family.** IS-IS keeps ONE "isis"
  ProtocolID/source for both AFs (umbrella contract); the redistevents batch's AFI
  field (not the ProtocolID) selects ipv4 vs ipv6. `emitDelta` (IPv4) now delegates to
  a generalized `emitDeltaFamily(delta, proto, afi, sink)`; `OnSPFChangeV6` emits at
  AFI=2. The SPF Computer has a separate `onChangeV6`/`SetOnChangeV6` so the IPv6 delta
  carries its own family. The BGP IPv6-unicast consumer already accepted AFI=2
  (TestBGPConsumerInjectRouteIPv6), so no BGP-side change was needed (A-4 resolved).
- **Redistributed IPv6 sets the TLV 236 external (X) bit** (RFC 5308 sec 2); IPv4
  TLV 135 has no external bit. up/down clear on injection (RFC 2966), both AFs.

## Consequences
- One adjacency carries both AFs; enabling `address-family ipv6-unicast` per interface
  gates ALL IPv6 (NLPID 0x8E, IIH TLV 232, LSP TLV 232/236, IPv6 SPF).
- The IPv6 install pass is always wired (initSPF), harmless on an IPv4-only topology
  (no TLV 236 leaves -> empty IPv6 route set; `ze_isis_routes_installed{afi=ipv6}`=0).
- No new metric series (umbrella contract): IPv6 sets `afi=ipv6` on the existing
  `ze_isis_routes_installed{level,afi}` + `ze_isis_redist_*{...,afi}`. Both Installers
  register the same gauge name; the metrics registry caches by name and returns the
  same handle, so two SetMetrics calls share one series (no duplicate-registration
  panic) -- verified in `internal/core/metrics/prometheus.go GaugeVec`.

## Gotchas
- **The umbrella TLV 236 flags bit-mask diverges from the RFC packet diagram.** The
  codec (isis-2 `tlv_ipv6.go`) assigns U=0x80, X=0x20, S=0x40 per the umbrella Shared
  Contracts, NOT the diagram's descending U|X|S. Trust the umbrella/constants, pinned
  by `TestISISTLVIPv6FlagBits`. isis-12 reads the codec's `IPv6ReachEntry.External`/
  `UpDown` bools, never the raw bits.
- **`netip.Prefix("...:ok::")` panics** -- `ok` is not valid hex. Use valid hex labels
  (`bad`, `fee`, `dead`) in test prefixes. Cost me one failing test.
- **MAX_V6_PATH_METRIC filter is `> 0xFE000000` (strictly greater).** A prefix metric
  EXACTLY 0xFE000000 passes the per-entry filter but then the accumulated path cost
  hits the `>= MaxPathMetric` ceiling in clampMetric and is dropped as unreachable.
  Both boundary cases tested (TestISISIPv6MetricAboveMaxIgnored / AtMaxBoundary).
- **`go build ./internal/...` is BLOCKED by a hook** and so is `go build -o tmp/...`
  (must be `bin/`). Use `make bin/ze bin/ze-test`, then `bin/ze-test isis -pattern ipv6`.
- **`bin/ze-test isis --all` flaked on isis-flooding** (parallel daemon contention,
  pre-existing, NOT mine); `-p 1` (serial) passes all 11. Run serially to confirm.
- **`show isis route ipv6` added to register.go** (minimal additive dispatch + command
  decl) so the IPv6 route table is visible; full CLI grammar/rendering + command-
  reference.md doc are owned by isis-13. The `isis-dualstack-frr` interop scenario is
  also isis-13's (this spec only notes it).
- Single-topology assumes IPv4/IPv6 congruent topologies; a non-congruent link
  blackholes IPv6 (documented, A-1). RFC 5120 MT is the real fix, out of scope.

## Files
Created:
- `internal/plugins/isis/lsdb/origination_ipv6.go` (+`origination_ipv6_test.go`):
  `NonLinkLocalV6Prefixes`/`NonLinkLocalV6Addrs` RFC 5308 scope filters.
- `internal/plugins/isis/spf/ipv6.go` (+`ipv6_test.go`): `MaxV6PathMetric`,
  `NextHopResolverV6`, `BuildRoutesV6`, `resolveHopsV6`.
- `internal/plugins/isis/redistribute/ipv6.go` (+`ipv6_test.go`): IPv6
  inject/withdraw (TLV 236, external bit), `ConnectedPrefixInfosV6`, `OnSPFChangeV6`,
  `emitDeltaFamily`.
- `internal/plugins/isis/circuit/hello_ipv6_test.go`: IIH TLV 232 link-local scope.
- `test/isis/isis-ipv6.ci`: dual-stack config + `show isis route ipv6` wiring.

Modified (additive):
- `lsdb/origination.go` (+PrefixInfoV6, LevelState.PrefixesV6/InterfaceAddrsV6,
  fragment IPv6 TLVs), `lsdb/encode.go` (+interfaceAddrV6TLVs/extIPv6ReachEntryBytes).
- `spf/graph.go` (+Node.PrefixesV6/AddrsV6, decode TLV 232/236), `spf/install.go`
  (family+afi param, NewInstallerV6), `spf/computer.go` (IPv6 resolver/installer/
  onChangeV6/lastV6, second pass in Run, SnapshotV6/RoutesV6, Stop).
- `circuit/circuit.go` (+IPv6LinkLocal), `circuit/hello.go` (+ipv6InterfaceAddrTLV).
- `circuits.go` (+interfaceIPv6LinkLocal/Prefixes/NonLinkLocal, thread into Config),
  `lsdb_wiring.go` (IPv6 LevelState merge, setPrefixesV6), `redist_wiring.go`
  (SetRedistPrefixV6/RemoveRedistPrefixV6, IPv6 connected advertise, wire OnChangeV6),
  `spf_wiring.go` (engineNextHopResolverV6, IPv6 installer in initSPF, routeSnapshotV6),
  `server.go` (+prefixesV6/redistPrefixesV6), `register.go` (show isis route ipv6),
  `redistribute/redistribute.go` (LSPInjector +V6 methods), `redistribute/consumer.go`
  (dispatch AFI=2 to ipv6.go), `redistribute/source.go` (emitDelta -> emitDeltaFamily).
- Docs: `docs/guide/isis.md` (dual-stack section + single-topology caveat),
  `docs/architecture/wire/isis.md` (IPv6 origination rows + scope note), `docs/features.md`,
  `docs/comparison.md`, `docs/plugin-development/metrics.md` (afi=ipv6),
  `docs/functional-tests.md` (isis-ipv6.ci row).
