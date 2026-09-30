# Routing child, package internal/plugins/isis/packet (+ isis root leftovers), author handoff

Author: isis packet author agent, 2026-09-29. No verdict stamped, nothing committed. All records route revert, each OBSERVED red.
Package runs (./le job run): packet, spf, lsdb, root all green (logs scratch/isispkg-final.log, isisroot-final.log; the root's first run hit a concurrent go-cache eviction, rerun green). gofmt clean. Lint and ./le rfc check not run (owed to the main thread).
Budget ran out before every verdict was handled: see "unresolved" rows; a continuation starts there.

| id | resolution | what now proves each clause (+ / -) | records written | expected verdict | notes |
|----|-----------|--------------------------------------|-----------------|------------------|-------|
| RFC1195-5.2-4 | tests (tag removed) | root units unchanged (+ TestRFC1195OriginatedExternalEntriesCarryDefaultMetric, - TestRFC1195ExternalEntryWithoutDefaultMetricRefused) | none (tag removal) | enforced | positive tag removed from packet/tlv_ipv4_test.go::TestISISTLVIPv4RoundTrip (TLV 135 is RFC 5305), approval D-15 |
| RFC5305-4.1-1 | tests | + spf TestISISLeakOriginationL1L2: l2Derived enters L2 with UpDown false, leakInto SETS the bit on the L2->L1 leak (assertion 2); - same unit: alreadyDown (UpDown true in L1) absent from IntoL2 (assertion 3), the "only down the hierarchy" clause | +/- on spf/leak.go::leakInto | enforced | approval D-15; lsdb/redistribute tags unchanged |
| RFC1195-3.9-1 | tests | + NEW root TestRFC1195InvalidAuthLSPDiscardedAtReceive: a valid signed L1 LSP goes through dispatcher.dispatch to the L1 LSP handler once; - same unit: one digest octet changed, handler never called (the whole PDU discarded at the receive path) | +/- on auth_wiring.go::verifyFrame | enforced | packet tags (VerifyPDU level) kept |
| RFC5310-3.5-1 (row had no tags) | tests (tags moved in) | + / - NEW TestRFC1195InvalidAuthLSPDiscardedAtReceive (matching data delivered, mismatch discarded); - packet TestISISAuthWrongKeyRejected, TestISISAuthKeyIDMismatchRejected (moved from RFC5310-4-2) | +/- verifyFrame; - x2 on packet/auth_verify.go::VerifyPDU | first verdict; logging clause ("SHOULD be logged") has no assertion, judge's call | approval D-15 on both packet units |
| RFC5310-4-2 | R1 fallback -> OWNER-GATE | + TestISISAuthRotation, TestISISAuthRotationOverlapAccepts, TestRFC5310NonSendingKeyStillUsed; negative: only root TestRFC5310NonSendingKeyStillUsed (judge: a further positive) | none | weak | packet negatives moved to 3.5-1 (they prove the discard rule). No (a)/(b) negative exists: "able to store and use more than one key" has no refusal path and no input drives the store toward one key (resolveChain keeps every decodable key; acceptKeys caps at 16). Add to OWNER-GATE |
| RFC1195-4.4-2 | R1 fallback -> OWNER-GATE | unchanged (root author's row) | none | weak | no receiving-side or producer-side negative for "required to transmit and receive ISH on p2p links"; add to OWNER-GATE |
| RFC1195-5.3.4-3 (wrong) | row correction | row now quotes the §5.3.4 bit-7 sentence of the DELAY/EXPENSE/ERROR octets, which the existing +/- units (TestRFC1195NarrowMetricReservedTransmit/Receive) prove | none (tags unchanged) | enforced (stale-requirement, re-judge) | Correction paragraph in NEW rfc/corrections/rfc1195.md; extraction 5.3.4:6 mapped to the row, 5.3.4:3 (bit 8) excluded cross-document to RFC 2966 §2. NOTE: rfc/short and rfc/extraction edits made with a python replace, not the Edit tool; the extraction sign-off may need a reseal (judge) |
| RFC905-x-1 | tests | + TestISISChecksumFixedVector, TestISISChecksumModulus (unchanged); + NEW TestRFC905MinusZeroReadAsZero (clause b: 0xFF region and a stored 255 octet verify); - NEW TestRFC905FaultySenderChecksumRefused/modulo-256 (receiver refuses a mod-256 sender's checksum, which differs from Checksum's) | +/- on checksum.go::VerifyChecksum | enforced | x-1 negative dropped from TestISISChecksumDetectsCorruption (approval D-15) |
| RFC905-x-2 | tests | + TestISISChecksumVectors (unchanged); - NEW TestRFC905FaultySenderChecksumRefused/c1-before-c0 (receiver refuses a checksum built with the B.3.3 steps swapped) | - on VerifyChecksum | enforced | x-2 negative dropped from TestISISChecksumDetectsCorruption |
| RFC5310-4-1 | tests | + NEW TestRFC5310LifetimeNotAuthenticated: HMAC-SHA-256 (type 3) LSP verifies after lifetime 1/600/0xFFFF (checksum recomputed), a changed sequence number is refused. Row keeps its {single-polarity: positive} marker | + on auth_sign.go::lspAuthLayout | enforced | 4-1 positive dropped from TestISISAuthRotationOverlapAccepts (body never touches the lifetime), approval D-15 |
| RFC3787-x-1 | tests | + NEW lsdb TestRFC1195UnknownTLVsFloodedUnchanged: an LSP carrying TLV 131 is stored by Flooder.ReceiveLSP and flooded byte-identical, 131 included; + / - packet TestISISIgnoreObsoleteTLVs131And133 (prose narrowed to TLV 131, approval D-15) | + on lsdb/flooding.go::ReceiveLSP; +/- re-recorded on packet/tlv_opaque.go::DecodeTLVs | enforced, or weak if the judge also wants an IIH carrying 131 through the adjacency handler (not written) | |
| RFC1195-3.1-1 | tests | + NEW lsdb TestRFC1195UnknownTLVsFloodedUnchanged: LSP with TLVs 199/250 stored, flooded on the other circuit byte-identical, not flooded back; + packet TestISISUnknownTLVPassthrough (prose cites sec 5.2 now, approval D-15). Row keeps its {single-polarity: positive} marker | + on ReceiveLSP; + re-recorded on DecodeTLVs | enforced | |
| RFC1195-4.2-1 | unresolved | - | - | weak | needs an interaction unit: neighbor with 63 addresses in TLV 132 used for adjacency / next hop |
| RFC1195-5.2-2 | unresolved | - | - | weak | needs the engine path (lsdb_wiring.go) collecting interface addresses into TLV 132, asserting the addresses |
| RFC3787-x-2, RFC3787-4-1 (narrowing rows) | unresolved | - | - | - | split/correction rows not started |
| RFC905-x-2/x-3/x-4 split rows (B.3.1/B.3.2, B.3.5, B.4.2) | unresolved | - | - | - | R-C1: an rfc905 row edit stales the parent's x-3/x-4 verdicts; coordinate with the parent |
| RFC5301-3-8 row-quality | unresolved | - | - | - | D-3 review of MUST NOT over "The string is not null-terminated." not started |

## Files changed
- internal/plugins/isis/packet/tlv_ipv4_test.go (tag removed)
- internal/plugins/isis/packet/auth_verify_test.go (4-2 negatives -> 3.5-1; 4-1 tag dropped from TestISISAuthRotationOverlapAccepts)
- internal/plugins/isis/packet/checksum_test.go (x-1/x-2 negatives dropped from TestISISChecksumDetectsCorruption)
- internal/plugins/isis/packet/rfc905_arithmetic_test.go (NEW)
- internal/plugins/isis/packet/auth_lifetime_rfc5310_test.go (NEW)
- internal/plugins/isis/auth_discard_rfc1195_test.go (NEW)
- internal/plugins/isis/spf/leak_test.go (RFC5305-4.1-1 tags)
- internal/plugins/isis/lsdb/unknown_tlv_flood_rfc1195_test.go (NEW)
- internal/plugins/isis/packet/tlv_opaque_test.go (tag prose of RFC1195-3.1-1 and RFC3787-x-1)
- rfc/discrimination/rfc3787.json (records)
- rfc/short/rfc1195.md (RFC1195-5.3.4-3 text)
- rfc/extraction/rfc1195.json (sites 5.3.4:3, 5.3.4:6)
- rfc/corrections/rfc1195.md (NEW)
- rfc/discrimination/rfc1195.json, rfc5305.json, rfc5310.json, rfc905.json (records)
- tmp approvals via ./le rfc approve (6 units)

## OWNER-GATE additions
- RFC5310-4-2, RFC1195-4.4-2 (R1 fallback: no genuine negative exists)
