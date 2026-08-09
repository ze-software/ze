# IS-IS Reliable Flooding and SNP Synchronisation

The flooding layer is the dynamic counterpart of the LSDB: the receive-side
freshness-to-flag mapping, the periodic SRM-driven flood timer, CSNP and PSNP
build, send and receive, the per-circuit pending-request set, and the
point-to-point initial CSNP at adjacency up (ISO/IEC 10589 clauses 7.3.14 to
7.3.17).

The **protocol** behavior is documented in
[`../wire/isis.md`](../wire/isis.md), under "Reliable flooding and SNP
synchronisation". This page carries the structural decisions.

| Concern | File |
|---------|------|
| Flooder, freshness-to-flag mapping, flood tick | `lsdb/flooding.go` |
| CSNP and PSNP build and receive, pending-request set | `lsdb/snp.go` |
| Engine glue | `flooding_wiring.go` |

## Decision: the Flooder holds no storage and no imports

The Flooder carries an LSDB reference plus two **injected function fields**: a
transmit function that the engine wires to the transport, and a circuits function
that the engine derives from the running interface config.

This keeps all flooding and SNP logic inside the `lsdb` package with no import of
the transport or circuit packages, so there is no import cycle and the plugin
stays self-contained. The Flooder is a flag-to-wire pump, never a second source
of truth.

<!-- source: internal/plugins/isis/lsdb/flooding.go -- the Flooder, ReceiveLSP, armSRMExcept, FloodTick -->

## Decision: freshness is decided elsewhere, this layer maps the outcome

The compare lives in the LSDB. The flooding layer only maps its result to flags:

| Outcome | Flags |
|---------|-------|
| Newer | SRM on every other eligible circuit, SSN on the incoming circuit, clear any matching pending entry |
| Equal (duplicate) | no SRM; SSN on the incoming circuit only on a LAN, since a point-to-point duplicate needs no acknowledgement |
| Older, equal sequence | SSN, to request the authoritative copy |
| Older, strictly lower sequence | SRM on the incoming circuit, to send ours back |

## Decision: SRM is cleared only by an acknowledgement

SRM is left set after a successful send, so an unacknowledged LSP resends on the
next tick. It clears on a PSNP at our sequence or an equal CSNP entry. That is
the reliable-flooding obligation of clause 7.3.15.1.

Only an unacknowledged **resend**, a transmit after a prior transmit since SRM
was armed, increments the resend counter, so the first flood is not miscounted as
a storm.

## Decision: a separate pending-request set for LSPs we do not hold

SSN keeps its acknowledge semantics and can only sit on an LSP already in the
database. A CSNP-driven request for an LSP we do **not** hold therefore goes into
a per-circuit pending-request set owned by this layer, bounded and deduplicated
by LSP ID. An entry clears when the requested LSP arrives and is stored.

<!-- source: internal/plugins/isis/lsdb/snp.go -- buildCSNPs, buildPSNP, ReceiveCSNP, ReceivePSNP, the pending set -->

## Decision: a request is encoded at sequence 0

A PSNP request for an LSP we do not hold carries sequence 0, lifetime 0 and
checksum 0 (clause 7.3.15.3). It never echoes the sequence learned from the
neighbor's CSNP.

This is the subtlest correctness point in the layer. An entry at the holder's
current sequence is indistinguishable from an acknowledgement, so the holder
would clear SRM and never supply the LSP, and the requester would wait forever.
The SNP-layer compare orders by sequence with the purge tiebreak, so a sequence-0
request always reads as older and sets SRM to supply.

## Decision: re-flood the stored bytes verbatim

A re-flood re-transmits the stored raw LSP bytes, preserving unknown TLVs byte
for byte (clause 7.3.14) and making no per-LSP allocation on the periodic timer.

## Decision: registration through the shared dispatcher

LSP, CSNP and PSNP arrive through the component's PDU dispatcher; this layer
registers all six level-1 and level-2 handlers there and never re-derives the PDU
type or keeps its own switch. Transmission goes out through the transport only.

Engine glue lives in `flooding_wiring.go`, a sibling of `lsdb_wiring.go` and
`dis_wiring.go`: it constructs the Flooder, installs the handlers, runs the
flood, PSNP and point-to-point CSNP timers, and fires the initial CSNP from the
circuit up hook.

<!-- source: internal/plugins/isis/flooding_wiring.go -- Flooder construction, handler install, the timers -->

## Decision: metrics behind one atomic pointer

The eight owned counters are held in one immutable struct behind an atomic
pointer. `SetMetrics` publishes the whole handle set with one store, and the hot
path loads it with one load. That is cheaper than a per-counter lock and a reader
never observes a torn rebind.

Owned series: `ze_isis_lsps_received_total{level}`,
`ze_isis_lsps_transmitted_total{level}`, `ze_isis_csnp_sent_total{level}`,
`ze_isis_csnp_received_total{level}`, `ze_isis_psnp_sent_total{level}`,
`ze_isis_psnp_received_total{level}`, `ze_isis_srm_resends_total{level}`,
`ze_isis_lsps_dropped_total{level,reason}`.

## Constraint: never re-flood on the incoming circuit

Clause 7.3.14. The arrival circuit is skipped when SRM is armed. Tests assert SRM
is not set on the incoming circuit for both the newer and the purge cases.

## Constraint: LAN CSNP cadence is DIS policy

This layer provides the CSNP and PSNP build, send and receive mechanism. The
obligation to source periodic CSNPs on a broadcast circuit belongs to the elected
designated IS; see [`isis-8-dis-broadcast.md`](isis-8-dis-broadcast.md). The
periodic CSNP timer here fires only on point-to-point circuits.

## Trap: the shared in-memory test wire drops frames

The shared test wire sends without blocking and drops on a full buffer. That is
harmless for the self-repeating hello path but loses flooded LSPs, which are then
recovered only slowly through CSNP and PSNP, making flood-timing assertions
flaky.

A lossless harness that blocks while respecting the receiver's stop lives in the
flooding wiring test. Reuse it for any multi-engine flood test on a host with no
raw socket. A real link does not randomly drop frames, so no production code
changed for the harness.
