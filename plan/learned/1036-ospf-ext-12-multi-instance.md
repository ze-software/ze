# 1036 - OSPFv2 Multi-Instance (RFC 6549)

## Context

RFC 6549 splits the former 2-byte OSPFv2 AuType field into a 1-byte Instance ID
(offset 14) + 1-byte AuType (offset 15) in the common header, so multiple OSPF
instances can share an interface, demultiplexed by Instance ID. Delivered as one
full OSPFv2 engine per configured Instance ID. First OSPF ext spec built in an
isolated git worktree and integrated into main by diff+apply.

## Decisions

- One `instanceManager` owning a `map[uint8]*engine`; base instance 0 always present and additionally owns redistribution + default-origination; non-zero instances run the core link-state protocol on their OWN raw socket (per-instance transport), demuxed by Instance ID in the dispatcher.
- Per-instance transport (a raw proto-89 socket per engine with SO_BINDTODEVICE + IP_MULTICAST_LOOP=0) over a shared-transport-with-fan-out model, because the transport signer + iface-up/down hooks are per-transport; shared transport would reintroduce shared mutable state. (Spec assumption A-7 marked broken -> R-4 fallback.)
- Config: per-interface `instance-id` LEAF-LIST (uint8), not a single leaf and not a top-level instance list; absent means base instance 0. Only shape that expresses "two instances on one interface" (RFC 6549 §3.1).
- The header split is byte-for-byte identical for instance 0 (offset 14 = 0 = the old AuType high byte, which was always 0 since AuType < 256), proven by a golden re-encode test.

## Consequences

- Every OSPFv2 packet's common header now carries InstanceID at offset 14; encoders (`neighbor.NewV4Encoder(id)`, `lsdb.NewV4PacketEncoder(id)`, the iface Hello encoder) stamp it; the dispatcher drops + counts a mismatched Instance ID before any handler.
- Opaque consumers (TE/RI) + redistribution bind to the base engine (instance 0) only: `registerOpaqueConsumer` is a package-global map with dup-rejection, so per-instance builds beyond the base log a harmless "already registered" warn. A documented single-engine limit, out of scope for RFC 6549 core.
- `signPacket` must NOT write offset 14 (it now only writes offset 15 = AuType); writing offset 14 clobbered the Instance ID. The auth digest covers offset 14, so the Instance ID is authenticated.

## Gotchas

- Integration into main (which already had ext-1/2/3) was 19 clean `git apply`s + 6 hand-reconciled shared files (config.go, instance.go, register.go, cmd_show.go, docs/guide/ospf.md, umbrella). The register.go reconcile had to wrap ALL existing eng-wiring (ext-2 TE consumer, ext-3 RI consumer, metrics, redistribution) into the `wireV4Engine` closure so every consumer still registers when the base engine is built.
- Stacking specs into the same files pushed config.go/instance.go past the 1000-line soft threshold (non-blocking warning) and made a duplicated string literal ("unknown") trip goconst at min-3 across ext-1/ext-2/ext-12; resolved with a package const.
- Injected LSP/harness diagnostics reported stale undefined-symbol errors mid-integration (packet.Header.InstanceID etc.); go vet + go test on the final tree were authoritative and clean.

## Files

- `internal/plugins/ospf/multi_instance.go` (+ test), `neighbor/neighbor_test.go`
- `internal/plugins/ospf/packet/header.go` (InstanceID split), `packet/json.go`, `auth_wiring.go`, `codec.go`, `dispatcher.go` (onInstanceMismatch), `iface/iface.go`, `neighbor/neighbor.go` (NewV4Encoder), `lsdb/flooding.go` (NewV4PacketEncoder)
- `internal/plugins/ospf/{config,instance,register,cmd_show}.go`, `yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-instance-*.ci`, `test/interop/scenarios/ospf-multiinstance-frr/`
- `rfc/short/rfc6549.md`, `docs/guide/ospf.md`
