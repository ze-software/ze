# 1345 -- A Node That Never Reads The Network's Claim On Its Own LSP

## Context

On 2026-08-04 `originateFragmentLocked`
(`internal/plugins/isis/lsdb/origination.go`) computed every own-LSP sequence
from `o.lastSeq[id]`, its own private counter, and consulted nothing else. Ze is
the only authority for its own LSP IDs, so the counter looked sufficient. It is
not: the NETWORK also carries a value for those IDs, and it can be higher than
anything Ze has issued -- a peer holding Ze's pre-restart LSP, or a purge of it.

`LSDB.Receive` then stored that copy (it is Newer) and `armSRMExcept` flooded it
onward, so Ze relayed a withdrawal of itself. `refloodPurge` logged
`isis: own LSP purged` and did nothing else. The next `originate()` emitted
`lastSeq+1`, below the purge, which every peer rejected as Older and echoed
back. Ze reached the purged sequence only by incrementing once per refresh
interval, which is hours from a four-digit gap.

ISO/IEC 10589 clause 7.3.16.4 c) has covered this since 1992: on a purge of an
own LSP the IS "shall not overwrite with the received LSP", "shall change the
sequence number of the un-expired LSP from S as described in 7.3.16.1",
"generate a new LSP" and "set SRMflag on all circuits". Clause 7.3.16.1 gives
the same rule for a higher sequence returned by any system, and clause 7.3.16.2
for an equal sequence with a differing checksum. None of the three had a code
path.

## Decisions

- **`rfc/full/` has no ISO/IEC 10589 and never will** (`iso/README.md`: not
  redistributable, clean-room summaries only). **RFC 1142 is the citable
  source**: the IETF republication of the same protocol text, freely available,
  with the SAME clause numbering. Corroborated on the NOTE in clause 7.3.16.4
  ("A check of the checksum of a zero Remaining Lifetime LSP succeeds even
  though the data portion is not present"), which IETF drafts attribute to ISO
  10589 clause 7.3.16.4 and which appears verbatim in RFC 1142 clause 7.3.16.4.
  Quote RFC 1142 and say where it came from. Do not paraphrase from memory.
- **Existing citations of "clause 7.3.3" for sequence numbers are wrong.** In
  RFC 1142, 7.3.3 is "Use of Manual Routeing Information"; sequence numbers and
  the wraparound rule are 7.3.16.1. The citations on the lines this work touched
  were corrected; others survive elsewhere in `internal/plugins/isis/`.
- **The floor stores the CLAIMED value, not claimed+1**
  (`Originator.RaiseSequenceFloor`), so the wraparound path composes with no
  special case: a claim at `MaxSequenceNumber` makes the next origination wrap,
  purge, suspend and restart from 1, which is what clause 7.3.16.1 requires
  anyway. The suspension test comes FIRST in `originateFragmentLocked`, so a
  floor raised during a suspension cannot shorten it, and the elapsed branch
  deletes `lastSeq`, so it cannot survive it either.
- **A claim below our own sequence is refused.** Clause 7.3.16.4 b-3 already
  answers it by arming SRM on the arrival circuit.
- **A repeat of a claim already answered needs its OWN record**
  (`answeredClaim`), not the sequence test. Raising the floor writes the claimed
  value into `lastSeq`, so for a fragment the current state no longer produces --
  where the answer never writes that ID -- `lastSeq > claimed` is false forever
  and every retransmission re-originates and re-floods the whole level. A guard
  whose own action is supposed to make it stop firing must be checked: here the
  action does not always happen.
- **A claim on an LSP ID this node does not originate is refused outright.**
  Clause 7.3.16.4 c) needs an un-expired own LSP to change the sequence OF, and
  a) asks only for an acknowledgement. Refusing also keeps a remote party from
  creating per-LSP-ID state in `lastSeq`: our own System ID's fragment space is
  65280 IDs per level, all of them claimable by a peer.
- **Clause 7.3.16.4 a-1's acknowledgement needed a new carrier.** SSN lives on an
  LSDB entry, so it cannot acknowledge an LSP we refuse to hold. `ackOnly`
  (`lsdb/snp.go`) is a per-circuit queue the next PSNP drains, carrying the
  ARRIVED sequence. That is what makes it an ACK: `pendingFor` emits a REQUEST at
  sequence 0 precisely so the holder reads it as older and supplies the LSP.
- The pseudo-node case is answered by dropping its coalescing record and letting
  the 1s DIS re-election tick regenerate it, rather than adding a second
  origination entry point.

## The interop scenario could not run, and neither could the other six

`ze start` with an IS-IS-only config failed every circuit with
`isis: opening circuits: isis/transport: resolve eth0: iface: no backend
loaded`, so no adjacency ever formed. The iface component loads its backend
from an `interface { ... }` block alone (`iface/register.go` OnConfigure), and a
config that names its interfaces only under `isis { interfaces { ... } }` has
none. OSPF already solved this: `resolveOSPFInterface`
(`ospf/transport/backend_linux.go`) calls `iface.EnsureBackend()` first.
`isis/transport.resolveInterface` did not.

**All six FRR IS-IS interop scenarios were red because of it, and nothing said
so.** `make ze-interop-test` is not in `ze-verify`, so the suite had gone
unrun. The fix is one call, and it turned `isis-p2p-frr` green in the same run
that turned the new scenario green.

Two things to carry forward. A component that reads another component's data
must ENSURE that component is loaded, never assume the operator's config
happened to load it; grep `EnsureBackend` for the list of consumers that
already know this. And a test suite outside `ze-verify` decays silently: the
only reason this surfaced is that a new scenario forced someone to run it.

## Consequences

- **A coalescing guard eats a fix that changes no INPUT.** `originationUnchanged`
  compares the origination input and the refresh deadline. A sequence-floor raise
  changes neither, so the regeneration was skipped until `forceOriginate` dropped
  the recorded input. Any future "re-originate now" reason needs the same
  treatment: the guard is not a cache, it is a filter that must be told.
- **`f.db.SetSSN` is a NO-OP when the LSP ID is absent from the database**, and
  it fails silently. That is why clause 7.3.16.4 a-1 needed the `ackOnly` queue
  rather than a flag: the SSN/SRM model can only mark an LSP the database
  already holds. A flag API that silently does nothing for a missing key is easy
  to call and hard to notice; the first version of this work called it and
  believed the acknowledgement was sent.
- **A SIGNED own LSP was stored with a checksum that did not match its own
  bytes, and that was already a live interop defect.** `packet.LSP.WriteTo` fills
  the struct's `Checksum` with the PRE-signature value; `packet.SignPDU` inserts
  TLV 10 and recomputes the checksum inside the BYTE SLICE
  (`finalizeLSPChecksum`) and never writes back to the struct. `LSDB.Insert`
  takes both and stores `lsp.Checksum` as the entry metadata beside `raw` as the
  flooded bytes. So with a key chain configured, the CSNP this node sources
  advertised a checksum no receiver can reproduce from the LSP it holds, and
  clause 7.3.16.2 tells that receiver to treat the LSP as confused and PURGE it.
  Fixed in `Originator.encodeAndSign` by reading the checksum back out of the
  signed bytes (`packet.LSPChecksumOf`). **Rule: anything that stores or
  advertises an encoded PDU's checksum reads it from the bytes it stored, never
  from a struct built before a later pass mutated them.** This work only found
  it because the new own-LSP rule turned a silent metadata error into a
  self-sustaining sequence-bump loop against Ze's own echo.
- **Ze's AF_PACKET socket sees Ze's own transmissions** (`ETH_P_ALL`, no
  `PACKET_OUTGOING` filter in `transport/backend_linux.go`). So every own-LSP
  rule must be inert on an exact echo of what this node just sent, or the node
  answers itself in a loop. The equal-sequence-equal-checksum exclusion carries
  that, and `TestISISEngineOwnLSPEchoIsNotAConflict` pins it.
- **`test/interop/scenarios/isis-purge-reorig-frr` is the first IS-IS scenario to
  inject a raw PDU.** Every non-`check.py` `.py` in a scenario directory is
  auto-mounted into the ze container at `/etc/ze/<name>`, which has `python3`
  and `CAP_NET_RAW`, so a stdlib `AF_PACKET` injector needs no new harness
  support and no scapy (which exists in no image here). `bin/ze isis decode
  --pretty` validates a hand-built PDU against Ze's own codec, `checksum-valid`
  included, before it ever reaches a container.

## Files

- `internal/plugins/isis/lsdb/origination.go`: `RaiseSequenceFloor` (new), the
  `answeredClaim` map, the checksum refresh in `encodeAndSign`
- `internal/plugins/isis/lsdb/lsdb.go`: `ownConflictResult` (new), `Receive`
  refusing to store an own LSP, `ReceiveResult.OwnConflict`
- `internal/plugins/isis/lsdb/flooding.go`: the OwnConflict branch of
  `ReceiveLSP`, the `ackOnly` set
- `internal/plugins/isis/lsdb/snp.go`: `recordAckOnly`, `drainAckOnly`, and the
  ack-only list in `buildPSNP`
- `internal/plugins/isis/lsdb/pseudonode.go`, `internal/plugins/isis/lsdb/aging.go`:
  claim state cleared with the sequence state; corrected clause citation
- `internal/plugins/isis/own_lsp_conflict.go` (new): `reoriginateAboveClaim`,
  `forceOriginate`, `clearPseudonodeCoalescing`
- `internal/plugins/isis/flooding_wiring.go`: the OwnConflict branch of
  `handleLSP`, `lspLevel`
- `internal/plugins/isis/transport/backend_linux.go`: `ensureIfaceBackend` in
  `resolveInterface`, the fix that let any IS-IS interop scenario run at all
- `internal/plugins/isis/packet/lsp.go`: `LSPChecksumOf` (new)
- `internal/plugins/isis/lsdb/own_conflict_test.go` (new),
  `internal/plugins/isis/own_lsp_conflict_test.go` (new),
  `internal/plugins/isis/transport/backend_linux_test.go`
- `test/interop/scenarios/isis-purge-reorig-frr/`: `check.py`, `purge.py`,
  `ze.conf`, `frr.conf`
- `docs/architecture/wire/isis.md`, `docs/functional-tests.md`
