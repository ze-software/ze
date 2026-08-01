# 933 -- isis-7-flooding

## Context
Spec `isis-7-flooding` adds reliable LSP flooding and SNP (sequence-number PDU)
synchronisation to the native IS-IS engine (ISO/IEC 10589 clause 7.3.14-17): the
receive-side freshness->flag algorithm, the periodic SRM-driven flood timer, CSNP
(full-database digest) and PSNP (partial ack/request) build/send/receive, the
per-circuit pending-request set for LSPs not yet held, and the P2P initial CSNP at
adjacency Up. It is the dynamic counterpart to isis-6: isis-6 owns the LSDB store,
the freshness compare (`LSDB.Receive`), the per-circuit SRM/SSN flag storage,
origination, aging and purge; isis-7 owns the algorithms that consume and produce
those flags over the wire. It layers on isis-6 (LSDB + flags), isis-4 (PDU
dispatcher), isis-3 (transport TX) and isis-2 (LSP/CSNP/PSNP + TLV 9 codecs), which
already existed as sibling work -- so this was integration over an established
contract, not a from-scratch build.

The implementation is DONE: the whole tree builds darwin+linux, all isis unit and
engine wiring tests pass under `-race`, golangci-lint reports 0 issues. Interop
validation against FRR over raw L2 is pending a Linux execution host (see below).

## Decisions
- The `Flooder` (`lsdb/flooding.go`) holds NO LSDB storage. It carries a `*LSDB`
  reference plus two injected function fields -- `tx TxFunc` (engine wires to
  `transport.SendPDU`) and `circuits CircuitsFunc` (engine derives the live circuit
  set from the running InterfaceConfig). This keeps all flooding/SNP logic in the
  `lsdb` package (rule plugin-self-containment) with no import of the transport or
  circuit packages and no import cycle. `flooding.go` is a flag-to-wire pump, never
  a second source of truth.
- Freshness is decided by isis-6 `LSDB.Receive`; `ReceiveLSP` only MAPS the outcome
  to flags: Newer -> SRM on every other eligible circuit + SSN on incoming + clear
  any matching pending entry; Equal (duplicate) -> no SRM, SSN on incoming only on a
  LAN (a P2P duplicate needs no ack); Older split by sequence -> equal seq sets SSN
  to request the authoritative copy, strictly-lower sets SRM to send ours back.
- SRM is LEFT SET after a successful send on BOTH P2P and LAN; it is cleared ONLY on
  acknowledgement (PSNP at our sequence, or an equal CSNP entry). This is the
  reliable-flooding obligation (clause 7.3.15.1). The periodic `FloodTick` (5 s)
  re-transmits while SRM stays set; only an unacknowledged RESEND (a transmit after a
  prior transmit since SRM was armed) bumps `ze_isis_srm_resends_total`, so the first
  flood is not miscounted as a storm.
- A per-circuit PENDING-REQUEST set (`snp.go`), owned by isis-7 and independent of
  the LSDB, represents CSNP-driven requests for LSPs we do NOT yet hold. SSN keeps
  its acknowledge semantics on HELD LSPs only. The set is bounded
  (`maxPendingPerCircuit`=4096) and deduped by LSP ID; a pending entry clears when
  the requested LSP arrives and is stored (`ReceiveLSP` Newer -> `clearPending`).
- A PSNP request for a not-held LSP is encoded at SEQUENCE 0 / lifetime 0 / checksum
  0 (the "send me this LSP" form, clause 7.3.15.3), NEVER echoing the sequence learned
  from the neighbour's CSNP. An entry at the holder's current sequence is
  indistinguishable from an ACK, so the holder would clear SRM and never supply the
  LSP -- the requester would wait forever.
- LSP/CSNP/PSNP arrive via the isis-4 PDU dispatcher; `installFloodHandlers`
  registers all six L1/L2 types. isis-7 never re-derives the PDU type or maintains its
  own switch. TX goes out via isis-3 only.
- Engine cross-package glue lives in a dedicated root-package file
  `flooding_wiring.go` (matching the existing `lsdb_wiring.go`/`dis_wiring.go`
  split), NOT threaded through circuit/lsdb. It constructs the Flooder, installs the
  dispatcher handlers, runs the flood/PSNP/P2P-CSNP timers, and fires the P2P initial
  CSNP from the circuit onUp hook (`circuits.go:155`).
- Flooder metrics are held in one immutable struct (`flooderMetrics`) behind an
  atomic pointer. `SetMetrics` publishes the whole handle set with one atomic store;
  the hot path loads it with one atomic load -- cheaper than a per-counter lock and
  race-free (a reader never observes a torn rebind).
- Re-flood re-transmits the stored RAW LSP bytes verbatim (`entry.Raw()`),
  preserving unknown TLVs byte-for-byte (clause 7.3.14) and making no per-LSP
  allocation on the periodic timer (buffer-first).

## Consequences
- A level converges: every router ends with the same LSDB via flood + CSNP/PSNP
  reconciliation, proven on darwin by the in-memory three-node engine wiring test.
- Owned metrics (8, per-owner registration HERE, isis-13 only scrapes):
  `ze_isis_lsps_received_total{level}`, `ze_isis_lsps_transmitted_total{level}`,
  `ze_isis_csnp_sent_total{level}`, `ze_isis_csnp_received_total{level}`,
  `ze_isis_psnp_sent_total{level}`, `ze_isis_psnp_received_total{level}`,
  `ze_isis_srm_resends_total{level}`, `ze_isis_lsps_dropped_total{level,reason}`.
- Minimal additive edits to siblings: `packet/tlv_core.go` exported
  `LSPEntriesTLV.EncodedLen()` + `WriteLSPEntriesTLV` (reuse the isis-2 TLV 9 encoder,
  do not re-implement the entry layout); `lsdb/lsdb.go` added read-only `LSPIDs(level)`
  + `LSPEntries(level)` enumerators and `noteSRMTransmit`; `server.go` added the
  `flooder` field + init/handlers/loops + metrics/SystemID wiring; `circuits.go` added
  the P2P-up initial-CSNP call to the existing onUp hook.

## Gotchas
- **The shared in-memory test wire drops frames on a full buffer (non-blocking
  send).** That is fine for the self-repeating Hello path but loses flooded LSPs
  (recovered only slowly via CSNP/PSNP), making flood-timing assertions flaky. A
  lossless `relWire`/`multiBackend` harness (blocking send respecting the receiver
  stop) lives in `flooding_wiring_test.go`; reuse it for any N-engine flood test on
  darwin. A real P2P/LAN link does not randomly drop. No production code changed for
  the test harness.
- **The freshness comparison has THREE tiers, in order: sequence, then remaining
  lifetime, then checksum.** The remaining-lifetime tier must not be skipped: a
  same-sequence LSP with remaining lifetime 0 (a purge) is MORE recent than a held
  non-zero copy, so it is accepted and re-flooded -- NOT discarded as a duplicate
  (AC-16). Only at equal seq AND equal lifetime-class does a differing checksum
  trigger a request and an equal checksum count as a duplicate. A missing
  lifetime tier loses purges that arrive at the same sequence number. The tier lives
  in isis-6 `compareFreshness`; isis-7 relies on it and adds the purge-tier tests.
- **A request must be encoded at sequence 0, not the CSNP-learned sequence.** This is
  the single subtlest correctness point: echoing the holder's sequence reads as an ACK
  and the LSP is never supplied. The SNP-layer `compareSNPEntry` deliberately compares
  by sequence (with the purge tiebreak) so a seq-0 request always reads as older (cmp
  < 0) and sets SRM to supply.
- **"Never re-flood on the incoming circuit"** (clause 7.3.14) is enforced by skipping
  the arrival circuit in `armSRMExcept`; tests assert SRM is NOT set on the incoming
  circuit for both the Newer and the purge cases.
- LAN CSNP cadence and the DIS-only obligation to source periodic CSNPs on broadcast
  circuits are POLICY supplied by isis-8 (DIS election); isis-7 provides only the
  CSNP/PSNP build/send/receive mechanism. The P2P periodic CSNP timer here fires only
  on P2P circuits.

## Interop validation pending Linux execution
Live reliable flooding over raw L2 (a three-node line A->B->C converging, the SRM
flood timer, CSNP/PSNP request/ack, the P2P initial CSNP at adjacency Up, and purge
re-flood + network-wide age-out) and correctness against FRR require AF_PACKET on a
Linux host and cannot run on this darwin host. The scenario files are written and in
place:
- `test/interop/scenarios/isis-p2p-frr/` (check.py, frr.conf, ze.conf): P2P
  adjacency + LSDB sync via flooding and P2P CSNP/PSNP against FRR isisd. Scenario
  written; execution pending Linux/QEMU.
- `test/interop/scenarios/isis-lan-dis-frr/` (check.py, frr.conf, ze.conf): LAN CSNP
  cadence + PSNP ack against a real DIS (cadence policy from isis-8). Scenario
  written; execution pending Linux/QEMU.
- `test/isis/isis-flooding.ci`: the live 3-node flood / purge age-out portion is
  Linux-pending; the offline CSNP/PSNP wire-format decode through `ze isis-decode`
  runs on any host and is pinned to the codec by
  `internal/component/isis/packet/flood_ci_test.go` (`TestISISFloodCIFixtures`).

On darwin the live behaviour is fully exercised by the in-memory engine wiring tests
over the lossless `relWire` harness: `TestISISLSDBSync` (three-node line),
`TestISISFloodSRMTimer`, `TestISISCSNPGapRequest`, `TestISISPSNPAck`, all passing
under `-race`. The two FRR interop scenarios and the live QEMU flooding run are the
remaining validation, gated on a Linux/QEMU host (the FRR scenarios are owned/driven
by the isis-13 interop harness).

## Files
- `internal/plugins/isis/lsdb/flooding.go` (+`flooding_test.go`,
  `flooding_metrics_test.go`): the `Flooder` (receive-side freshness->flag mapping
  `ReceiveLSP`/`handleOlderLSP`/`armSRMExcept`; periodic SRM-draining `FloodTick`;
  the 8 owned metrics behind an atomic pointer).
- `internal/plugins/isis/lsdb/snp.go` (+`snp_test.go`): CSNP/PSNP build
  (`buildCSNPs` whole-range + multi-PDU split, `buildPSNP` from SSN-acks +
  pending-requests), receive (`ReceiveCSNP` gap detection, `ReceivePSNP`
  ack-clears-SRM / request-sets-SRM), the bounded per-circuit pending-request set,
  the P2P `InitialCSNP`, and `compareSNPEntry`.
- `internal/plugins/isis/flooding_wiring.go` (+`flooding_wiring_test.go`): engine
  glue -- constructs the Flooder, installs the LSP/CSNP/PSNP dispatcher handlers,
  runs the flood/PSNP/P2P-CSNP timers, fires the P2P initial CSNP on adjacency Up.
- `internal/plugins/isis/packet/flood_ci_test.go`: pins `test/isis/isis-flooding.ci`.
- `test/isis/isis-flooding.ci`: CSNP/PSNP wire-format decode (offline).
- `test/interop/scenarios/isis-p2p-frr/`, `.../isis-lan-dis-frr/`: FRR interop
  scenarios (written; Linux/QEMU-pending).
- Modified (additive): `internal/plugins/isis/lsdb/lsdb.go`
  (`LSPIDs`/`LSPEntries`/`noteSRMTransmit`), `internal/plugins/isis/server.go`
  (flooder field + init/handlers/loops + metrics/SystemID), `internal/plugins/isis/circuits.go`
  (P2P-up initial-CSNP hook), `internal/plugins/isis/packet/tlv_core.go`
  (`LSPEntriesTLV.EncodedLen()` + `WriteLSPEntriesTLV`).
- Docs: `docs/architecture/wire/isis.md` (Reliable flooding + SNP section),
  `docs/plugin-development/metrics.md` (8 metric rows),
  `docs/functional-tests.md` (isis-flooding.ci row).
