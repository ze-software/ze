# 1041 - BFD for OSPF (RFC 5880/5881)

## Context

Integrate Ze's existing BFD engine with OSPF for sub-second failure detection,
both address families (single-hop). OSPF registers a Full adjacency as a BFD
client and drives the shared NSM down on a BFD session failure. Built base-only
in a worktree; consumes ONLY `internal/component/bfd/api` (does not modify the
BFD component). Integrated into main by diff+apply.

## Decisions

- Register a BFD client only when the neighbour reaches Full (a new `neighborEventSink` onFull/onLost seam), release it on leaving Full / interface down (`bfdReleaseInterface`) / shutdown (`bfdStopAll`). Distinct BFD keys per (interface, neighbour) and per AF (v4 uses the neighbour IP, v6 the link-local), so v4 and v6 sessions on one link never collide.
- A BFD session-down (or AdminDown) event drives the OSPF neighbour Down via the existing `Table.NeighborDown` seam; the subscriber detaches itself first to avoid a self-join deadlock; idempotent with the OSPF dead-timer.
- Per-interface config `bfd {enabled, min-tx, min-rx, multiplier}` in MILLISECONDS (stored x1000 as microseconds for the api), default 50 ms; ceiling capped at 10 s (a sub-second liveness protocol; the initial 255 s ceiling was meaningless).
- A `doctor-ospf-bfd-plugin-absent` informational check fires only when BFD is enabled and the BFD service is absent.

## Consequences

- The `ze_ospf_bfd_*` series are get-or-create by name, so both AF engines share one series (per the af-unify metrics pattern); wire `setBFDMetrics` per-v6-engine inside ext-15's `v6EngineSet.spawn` (there is no single eng6 anymore).
- Adding a value-typed `BFD bfdInterfaceConfig` field to `interfaceConfig` pushed it to 160 bytes and tripped gocritic rangeValCopy on unrelated range loops; reorder `interfaceConfig` fields by alignment (8/4/2/1) to shrink it rather than converting other specs' loops or making the field a pointer (a repo size-guard test `TestInterfaceConfigCopyBudget` enforces the cap).
- OSPF works with the BFD plugin removed (a bfd-enabled interface simply never registers a session; the doctor check warns).

## Gotchas

- Register the BFD client only at Full, not earlier NSM states, or a bring-up handshake Down would tear down an otherwise-healthy adjacency. AdminDown counts as down; Init/Up during bring-up is inert.
- The lock discipline matters: `e.mu` and `e.bfdMu` are never nested; the onFull/onLost callbacks (which re-enter neighbour accessors) run OUTSIDE the neighbor table's lock; `stopBFDSession` releases bfdMu before waiting on the session goroutine.

## Files

- `internal/plugins/ospf/{bfd_client,bfd_client_v6}.go` (+ tests, config_bfd_test.go)
- `internal/plugins/ospf/{config,instance,register,doctor,spf_wiring}.go`, `register_multiaf.go` (per-v6-engine setBFDMetrics), `instance_snapshots.go` (BFD snapshot annotation), `iface/iface.go`, `neighbor/{neighbor,table,lsreq}.go` (NeighborAddress accessor, LinkScopeLSDB->linkScopeLSDB)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` (bfd container, both AFs)
- `test/ospf/ospf-bfd-config.ci`, `test/ospfv3/ospfv3-bfd-config.ci`, `test/interop/scenarios/{ospf,ospfv3}-bfd-frr/`
- `docs/guide/ospf.md`, `docs/features.md`
