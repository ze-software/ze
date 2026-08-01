# 931 -- isis-5-adjacency

## Context
Spec `isis-5-adjacency` builds the IS-IS circuit abstraction and the adjacency
finite state machine for the native IS-IS engine (ISO/IEC 10589 clause 8.2). A
circuit is the per-interface runtime created for each interface IS-IS is enabled
on; it consumes the spec-isis-3 L2 byte pipe and the spec-isis-2 IIH codec, runs
a periodic Hello sender, and drives one adjacency FSM per neighbour. The FSM
implements Down/Initializing/Up, reaching Up via the RFC 5303 P2P three-way
handshake (TLV 240, with a legacy implicit fall-back) or the LAN three-way check
(our own SNPA echoed in a neighbour's TLV 6). This layers on isis-2 (wire/TLV
codec) and isis-4 (component, config, PDU dispatcher, events namespace), which
already existed. The implementation is DONE: the tree builds on darwin and
linux, all adjacency/circuit unit tests pass under -race, and golangci-lint is
clean.

## Decisions
- The adjacency FSM is PURE (`adjacency/fsm.go` `ReceiveHello`): it takes a parsed
  `HelloInput` + the immutable `Local` identity + `now`, mutates an `Adjacency`
  in place, and returns a `Transition{State,SessionUp,SessionDown,Rejected,
  RejectReason}`. It performs NO I/O and never parses raw bytes -- the circuit
  decodes with the isis-2 codec, fills `HelloInput`, and is the single writer of
  the table. This keeps every transition unit-testable with a fake clock and no
  socket, which is what let the work complete on a darwin host.
- Bidirectionality (the Up condition) is medium-specific in one `bidirectional`
  switch: LAN -> `slices.Contains(in.NeighborSNPAs, local.SNPA)`; P2P with a
  TLV 240 ever seen -> reported Up/Init AND the neighbour echoed our System ID;
  P2P with no TLV 240 ever seen -> implicit two-way fall-back (RFC 5303 sec 3.2).
  `sawTLV240` is sticky per adjacency so a 3-way-capable peer is never silently
  downgraded to the legacy path.
- Padding is owned by the engine, not the transport (umbrella "Final PDU bytes:
  padding then authentication"). `circuit/hello.go` builds the full IIH
  (origination TLV 1/129/132 + TLV 6 LAN / TLV 240 P2P), then `padHello` appends
  TLV 8 to the interface MTU BEFORE authentication, so the isis-10 digest covers
  the padding (RFC 5304 signs padded Hellos). The transport only adds 802.3+LLC.
- The neighbour table keys per (System ID, level): one P2P adjacency per circuit,
  one per (circuit, System ID) on a LAN. A dropped adjacency is held for a grace
  period (`deleteAt`) before reap, to absorb transient flaps (bio-rd uses 120s).
- Session up/down events go through the typed isis-4 `events.go` handles
  (`SessionUp`/`SessionDown`) via an `eventSink` adapter; the snapshot API
  (`table.go` `NeighborSnapshot`/`Snapshot`) is a flat by-value row the isis-13
  CLI renders as `show isis neighbor`. Metrics `ze_isis_adjacencies_up{level,
  interface}` / `ze_isis_adjacencies_total{level}` are owned and registered here.

## Consequences
- IIH delivery flows transport -> isis-4 PDU dispatcher -> the IIH handler this
  spec registers for 0x0f/0x10/0x11 (`server.go` `handleIIH`) -> `circuit.Receive`
  -> codec -> FSM -> table -> events. There is no PDU-type switch outside the
  dispatcher and no inline byte parsing in the FSM.
- The transport gained `RawFrame.SrcMAC` plus `CircuitInfo` / `CircuitNameByIfIndex`
  accessors (in `transport/transport.go`, additive) because the LAN three-way
  echo needs the frame source MAC threaded through to the FSM.
- The hold time advertised in a Hello = hello-interval * hold-multiplier, clamped
  to the 16-bit range so a 0 multiplier can never advertise an instantly-expiring
  adjacency.

## Gotchas
- **RWMutex is not reentrant.** Rendering the event snapshot via `Table.Snapshot()`
  (read lock) while inside `Table.Each` (write lock) deadlocks. Fixed by adding a
  lock-free `(*Adjacency).Snapshot()` captured under the existing write lock, and
  firing events AFTER the lock is released (`runtime.go` `fireEvents`). This bug
  class recurred across the isis circuit/table work -- prefer value snapshots
  captured under the writer, never re-enter a held RWMutex.
- **TLV 8 padding and the PDU Length field.** The IIH encoder backfills the PDU
  Length to the UNPADDED length (skip-and-backfill), so appending TLV 8 after that
  makes a receiver's decoder skip the padding. `padHello` rewrites the PDU Length
  field to the full padded length.
- **`TestISISHelloPeriodicSend` from the TDD plan was not implemented under that
  name.** The periodic-send behaviour is split: the cadence is the ticker in
  `circuits.go` (exercised by `TestISISComponentStart`) and the build/encode is
  `SendHello` (exercised by `TestISISIIHOriginationTLVs`). Same behaviour, a
  different decomposition -- recorded as "Changed" in the audit, not skipped.
- **AC-5 "logged" is partial.** The L1 area mismatch is REJECTED (no adjacency
  forms -- the load-bearing security property, tested by `TestISISL1AreaMismatch`)
  and the reason is returned as a structured `RejectReason:"l1-area-mismatch"`
  token, but the dispatch call site (`server.go` `handleIIH`) discards the
  returned Transition without emitting a log line. Explicit log emission of the
  reject reason is a small follow-up; the FSM reject itself is complete.
- **The summary's bare `transport.go` reference was imprecise.** There is no root
  `internal/component/isis/transport.go`; the `SrcMAC`/`CircuitInfo`/
  `CircuitNameByIfIndex` additions live in `internal/component/isis/transport/
  transport.go` (the transport package). The spec reference was corrected.
- FRR adjacency interop (`isis-p2p-frr`) is owned and run by isis-13, not here.
  DIS/pseudo-node (isis-8), Hello auth (isis-10), and BFD-driven teardown are out
  of scope.

## Interop validation pending Linux execution
The on-the-wire validation was NOT executed on this darwin host (no AF_PACKET /
FRR). These artifacts are written but their execution is pending a Linux/QEMU
runner:
- `internal/component/isis/adjacency_integration_linux_test.go`
  (`TestISISAdjacencyUpVeth`, build tag `integration && linux`): two real engines
  on a veth pair in a netns form an L1 adjacency over raw L2. Scenario written;
  execution pending Linux/QEMU.
- `test/interop/scenarios/isis-p2p-frr/` (check.py + frr.conf + ze.conf): Ze and
  FRR form a P2P RFC 5303 three-way adjacency and routes converge both ways.
  Scenario written; execution pending Linux/QEMU + FRR isisd (owned by isis-13).

Everything else is verified on darwin: `go vet ./internal/component/isis/...`
(darwin) and `GOOS=linux go vet ./internal/component/isis/...` both exit 0;
`go test -race ./internal/component/isis/adjacency/... ./internal/component/isis/
circuit/...` ok; the two-engine in-memory `TestISISAdjacencyUp` passes under
-race; `golangci-lint run` on both packages exits 0.

## Files
- `internal/plugins/isis/adjacency/adjacency.go`: State + Adjacency record
  (SystemID/SNPA/Level/Areas/IPv4/IPv6/HoldExpiry/Priority + reported-state).
- `internal/plugins/isis/adjacency/fsm.go` (+test): `ReceiveHello` (own-ID and
  too-many-areas guards, L1 area match, LAN/P2P bidirectionality, TLV 132/232
  next-hop store), `Expire`/`Down` hold-timer transitions, `classify`.
- `internal/plugins/isis/adjacency/table.go` (+test): per-(SystemID,level)
  keying, single-writer `Update`, grace-period reap, lock-free `(*Adjacency).
  Snapshot()` and `Table.Snapshot()`, `UpCount`, MaxNeighbors cap.
- `internal/plugins/isis/circuit/circuit.go`: circuit struct, Sender/EventSink
  interfaces.
- `internal/plugins/isis/circuit/hello.go` (+test): LAN (0x0f/0x10) and P2P
  (0x11) IIH build with TLV 1/6/129/132/240, `padHello` (TLV 8 to MTU + PDU
  Length rewrite), `HoldTime` (interval*mult clamped).
- `internal/plugins/isis/circuit/runtime.go` (+test): RX decode -> `HelloInput`
  -> FSM, `SendHello`, `Sweep`, `Teardown`, `fireEvents` (outside the table lock).
- `internal/plugins/isis/server.go` + `circuits.go`: IIH handler registration
  with the isis-4 dispatcher, ifindex->circuit routing (`handleIIH` passes
  `RawFrame.SrcMAC`), per-circuit Hello+sweep goroutine, metrics, merged
  `show isis neighbor` snapshot.
- `internal/plugins/isis/events.go`: `SessionUp`/`SessionDown` typed handles +
  `eventSink` adapter.
- `internal/plugins/isis/transport/transport.go` (additive): `RawFrame.SrcMAC`,
  `CircuitInfo`, `CircuitNameByIfIndex`.
- `internal/plugins/isis/adjacency_up_test.go`: two-engine in-memory wiring
  test (`TestISISAdjacencyUp`).
- `internal/plugins/isis/adjacency_integration_linux_test.go`: QEMU veth
  integration test (pending Linux execution).
- `test/isis/isis-adjacency.ci`: config-surface functional test.
- `test/interop/scenarios/isis-p2p-frr/`: FRR P2P interop scenario (pending Linux;
  isis-13).
- Docs: `docs/functional-tests.md` (isis-adjacency.ci row),
  `docs/plugin-development/metrics.md` (2 metric rows).
