# 932 -- isis-6-lsdb

## Context
Spec `isis-6-lsdb` builds the native IS-IS Link-State Database and own-LSP
origination (`internal/component/isis/lsdb/`): the per-level (L1/L2) store of
every LSP, the buffer-first raw-bytes + lazy-metadata entry model, the freshness
compare, the per-circuit SRM/SSN flooding-flag data model, own-LSP origination
from live adjacency + prefix state, 1s aging with zero-age purge, refresh,
sequence wraparound, and fragmentation across LSP numbers 0..255. It is the data
model the sibling specs consume: flooding (isis-7) drives the SRM/SSN flags and
sends the wire, SPF (isis-9) lazily decodes the store, isis-8 originates
pseudo-node LSPs into the same store, isis-12 adds the IPv6 TLVs, and isis-13
renders the `show isis database` snapshot. The implementation is DONE: the whole
tree builds (darwin and linux), all lsdb unit tests pass under `-race`, and
golangci-lint is clean on the package. On-the-wire interop with FRR is written as
scenarios but its execution is pending a Linux/QEMU host (this audit ran on
darwin).

## Decisions
- **Store raw bytes + parsed metadata, parse TLVs lazily.** `Entry` holds one
  OWNED copy of the verbatim PDU plus a small typed header (LSP ID, sequence,
  remaining lifetime, checksum, type block) and parses TLVs only on demand via
  `Entry.Decode()`. This is the Ze buffer-first model (mirrors BGP `WireUpdate`):
  an LSP carrying an unknown TLV re-floods byte-for-byte (ISO/IEC 10589 7.3.14)
  and SPF/CLI parse only what they need. `Receive`/`Insert` always copy `raw`
  into an entry-owned slice so a reused transport receive buffer cannot corrupt a
  stored entry (proven by `TestISISLSDBStoreVerbatim`, which scribbles the
  caller's buffer after store and asserts the stored bytes are intact).
- **SRM/SSN are per-LSP, per-circuit maps owned by the LSDB.** Keyed by a small
  engine-assigned `CircuitID` (not the sparse kernel ifindex, spec A-3). The LSDB
  only stores/queries/clears them; isis-7 drives them. A third map `srmSent`
  distinguishes a first flood from a true retransmission so the isis-7 resend
  counter is accurate. `ClearCircuit` drops a closed circuit from every entry.
- **Freshness compare keys on sequence, then a purge tiebreak, then checksum.**
  `compareFreshness` (entry.go): higher sequence is Newer; on equal sequence a
  remaining-lifetime-0 purge beats a live LSP (7.3.16.1, a withdrawn LSP can't be
  resurrected); same sequence + same purge-state + differing checksum keeps OURS
  (a bit-flipped duplicate cannot displace a good LSP -- the originator bumps the
  sequence to resolve genuine ambiguity).
- **Purge != expiry, retained for a grace period.** A lifetime-0 LSP is marked
  `purged` (not deleted) and kept for `ZeroAgeLifetime` (60s), then garbage
  collected. A purge that arrived on the wire (`receivedPurge`) is re-flooded once
  by the tick within the grace window (guarded by `recvPurgeReflooded` so there is
  no per-second storm) and is observably distinct from a local expiry through the
  `PurgeEvent.ReceivedPurge` flag (AC-9, R-2/R-4).
- **Origination is full regeneration, a pure function of (NodeInfo, LevelState).**
  Fragment 0 carries the non-fragmentable TLV 1/129/132/137 + overload bit; TLV 22
  neighbor entries and TLV 135 prefix entries are packed across fragments 0..255
  with no single entry split across fragments. The codec computes the Fletcher
  checksum on `WriteTo`; each fragment is a distinct LSP with its own sequence and
  checksum. Because origination is pure, the engine compares the freshly built
  input against the last-originated input and skips a redundant regen (the
  flooding-amplification fix): an adjacency flap firing `originate()` from several
  goroutines for the SAME resulting state collapses to one sequence bump / one
  re-flood, while a real topology change always falls through.
- **Sequence wraparound: purge then suspend then re-originate from 1.** At
  0xFFFFFFFF the originator purges the LSP ID at the max sequence, records a
  `suspendUntil` deadline of MaxAge + ZeroAgeLifetime, and refuses to re-originate
  that ID until the window elapses, then restarts from sequence 1 (7.3.3, AC-4).
- **Engine glue lives in a dedicated root-package file `lsdb_wiring.go`,** not in
  `server.go`/`events.go` as the spec's Files-to-Modify predicted (those files
  predate the subsystem split). It owns the engine's LSDB + Originator, the
  adjacency-Up -> `originate()` trigger (called from `circuits.go` transition
  hooks), the 1s aging loop folding in own-LSP and pseudo-node refresh, the
  SRM-arming on eligible circuits, and the snapshot proxy. Owned metrics
  registered HERE (per the umbrella canonical table): `ze_isis_lsps{level}`,
  `ze_isis_lsp_fragments{level}`, `ze_isis_lsp_originations_total{level}`,
  `ze_isis_sequence_wraps_total{level}`, `ze_isis_purges_total{level}`.

## Consequences
- A node advertises itself the moment an adjacency reaches Up: its own LSP carries
  TLV 1/129/22/132/135/137 with a valid sequence and Fletcher checksum, stored and
  SRM-armed for isis-7 to flood (`TestISISEngineOriginateOnAdjacencyUp` proves the
  full engine chain; `TestISISOriginateOnAdjacencyUp` proves the pure originator).
- The `show isis database` functional surface is live: `test/isis/isis-show.ci`
  boots a real engine with a NET and a passive interface, asserts the database
  carries the node's own originated LSP (with `lsp-id`) and that `database detail`
  expands the TLVs -- the snapshot path end to end on darwin (no raw socket).
- A 256-fragment cap and a per-level `MaxLSPsPerLevel` (16384) bound memory against
  a hostile flood of distinct LSP IDs (an existing LSP ID is always updatable so a
  refresh/purge is never dropped).

## Gotchas
- **The aging tick is the ONLY refresh driver in a quiescent network.** `ageOnce`
  folds in `refreshOwnLSPs` and `refreshPseudonodes`: without that, in a settled
  network nothing else calls `originate()` and the node's own LSPs age to MaxAge
  and purge -- the node vanishes from peers' LSDBs. Refresh is cheap when not due
  (a timestamp compare, no level-state rebuild).
- **A wrapped fragment is purged+suspended on the SAME Originate pass and skipped;**
  it is re-originated from sequence 1 only by a LATER `Originate` call after the
  suspension deadline (the engine re-triggers). Do not expect the wrap pass itself
  to produce the sequence-1 LSP.
- **`purgeStaleFragmentsLocked` walks contiguous fragment numbers and stops at the
  first absent one** -- fragments are always contiguous from 0, so a gap means
  "no more own fragments", not "skip and continue".
- **Equal-sequence freshness updates only the lifetime, never re-stores bytes;**
  Older stores nothing. Only Newer (or a first sighting) copies raw and bumps the
  size gauge. The sorted-ID cache (`idsSorted`) is rebuilt only when the KEY SET
  changes (new ID or delete), never on a freshness overwrite -- the hot
  flood/CSNP path copies an already-sorted slice under the read lock.
- **A purge MUST strip the body and re-authenticate (RFC 5304 sec 2).** Every
  purge routes through `packet.StripPurgeBody` then the signer, so a purge can
  never carry a stray body and always carries valid TLV 10 when signing is on.
- **Metric handles are read under `d.mu` even though Inc is concurrency-safe:**
  `SetMetrics` rebinds the interface value under the write lock while the
  Originator calls `incOriginations`/`incWraps` holding its OWN mutex, so the
  handle read (not the Inc) is the race that the lock guards. The LSDB never
  acquires the Originator's mutex, so no lock-ordering cycle.

## Interop validation pending Linux execution
The on-the-wire ACs cannot run on a darwin host (raw L2 / AF_PACKET + FRR isisd).
The scenario FILES exist and are wired, but were NOT executed this session:
- `test/interop/scenarios/isis-p2p-frr/` (check.py + frr.conf + ze.conf):
  asserts FRR forms a P2P adjacency with Ze, accepts Ze's originated LSP into its
  database (proving valid sequence/checksum/TLVs), and converges routes. Owns the
  LSDB-sync and SPF-convergence interop proof for this spec. `check.py` itself
  declares "CANNOT run on darwin".
- `internal/component/isis/adjacency_integration_linux_test.go` (build-tagged
  linux): drives a real veth adjacency through the engine, exercising origination
  on adjacency Up; execution pending a Linux/QEMU host.
- The spec's user-story #1 references `test/isis/isis-lsdb-sync.ci` as an isis-7
  artifact; the in-tree flooding functional test is `test/isis/isis-flooding.ci`
  (sibling isis-7). The LSDB's OWN functional surface, the show snapshot, is
  covered on darwin by `test/isis/isis-show.ci`.

## Files
- `internal/plugins/isis/lsdb/lsdb.go`: the two-level store, Receive/Insert/
  Lookup/Delete, freshness compare dispatch, SRM/SSN flag API, LSPIDs/LSPEntries,
  Snapshot, metric registration + size gauges. Single RWMutex (single writer).
- `internal/plugins/isis/lsdb/entry.go`: the per-LSP Entry (raw + metadata +
  purge/received-purge markers + SRM/SSN maps), lazy `Decode()`, `compareFreshness`.
- `internal/plugins/isis/lsdb/origination.go`: NodeInfo/AdjacencyInfo/PrefixInfo/
  LevelState inputs, the Originator (sequence/suspend state + signer), full
  regeneration, fragment packer, wraparound, stale-fragment purge.
- `internal/plugins/isis/lsdb/aging.go`: Tick (1s decrement), zero-age purge
  with grace, received-purge one-shot re-flood, garbage collection, markPurged.
- `internal/plugins/isis/lsdb/origination_ipv6.go`: TLV 232/236 entry encoders
  (consumed by isis-12; the origination path already packs them).
- `internal/component/isis/lsdb/{lsdb_test,entry-via-lsdb_test,origination_test,
  aging_test,boundary_test,metrics_test}.go`: 14 spec-named unit tests + boundary
  + metrics + purge-strip + received-purge-reflood tests, all passing under -race.
- `internal/plugins/isis/lsdb_wiring.go` (+lsdb_wiring_test.go): engine glue
  (origination trigger, aging loop, refresh, SRM arming, snapshot, metrics).
- Docs: `docs/architecture/wire/isis.md` (LSP origination contents, aging/purge,
  fragmentation, overload), `docs/plugin-development/metrics.md` (5 LSDB rows),
  `docs/guide/isis.md`.
