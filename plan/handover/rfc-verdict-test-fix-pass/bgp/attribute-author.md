# BGP child, package internal/core/bgp/attribute -- author handoff (first pass, 2026-09-30)

Scope: 48 rows of scratch/listing-bgp-pkg.tsv whose first package is internal/core/bgp/attribute; minus Blocked-by RFC4271-4.3-3, RFC8092-3-1, RFC9830-2.4.2-6 = 45 ids (list: children/bgp/attr-ids.tsv; audit notes: children/bgp/attr-notes.txt).
No verdict stamped, nothing committed. HEAD tags kept on every row (coverage ratchet, R26) except the retired RFC9012-13-7.
Records: batch 1 (children/bgp/attr-rec1.log) DONE, all OBSERVED. Batch 2 (attr-rec2.sh/.log) and batch 3 (attr-rec3.sh/.log) were started under `flock scratch/children/ledger.lock`; check their logs for `rc=` lines -- a refused record means that id's row below needs a rerun of its line from the script.

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|------------------------------------|---------|------------------|-------|
| RFC7311-3-3 (wrong) | tests | + reactor TestRFC7311OtherAIGPTLVsPassedAlongUnchanged: received [T1 100][T1 7][T9 abcd] to the socket, transparent and next-hop-self; octets after the first TLV equal, first metric 100/130. - same unit: policy replacement dropping the other TLVs does not reach the wire (R1(b)). + attribute TestRFC7311OtherAIGPTLVsSurviveTheCodec | batch 1: +/- reactor (revert applyFactsAIGP), + codec (revert ParseAIGP) | enforced | |
| RFC7311-3-1 | tests | + TestRFC7311AIGPTLVLengthIsEleven (WriteTo 01 000b +8; parse Length 11). - Length 12 and 10 refused | batch 1 (revert AIGPMetricOffset) | enforced | |
| RFC7311-3-2 | tests | + TestRFC7311AIGPValueIsWholeTLVSet (two whole TLVs). - 1/2/3 trailing octets refused | batch 1 (revert AIGPMetricOffset) | enforced | |
| RFC9012-13-7 (wrong) | row retired (R2) | binds the speaker that interprets the 7 single-instance sub-TLVs to build a tunnel; Ze reads none (carrier only). Tags dropped from TestRFC9012DuplicateSingleInstanceSubTLVs (same lines keep RFC9830-2.4-1); approval recorded | none | retired | rfc/short row removed; rfc/corrections/rfc9012.md Retired para; extraction 13:7 excluded binds-another-role, signed-off 2026-09-30 + resign-reason. rfc9012 has no discrimination file. Judge: confirm R2 vs a {gap} |
| RFC9012-13-3, 13-5, 13-8, 13-9, 13-11, 13-18, 13-19, 3.1-3, 3.2.1-2, 3.5-3, 4.3-2 | tests | reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong (rfc9012_tunnel_encap_carry_test.go): + enforceRFC7606 gives ActionNone/no error and keeps attr 23 octet-equal; forwarded raw, attr 23 and the Color ext-community reach the wire octet-equal. - the same through buildModifiedPayload (next-hop-self + MED) still octet-equal (R1(b): re-encode pushed toward violation) | batch 2: + revert enforceRFC7606, - revert buildModifiedPayload | enforced | covers the judge's "not the distribution path" objection; IPv4 unicast route (attr 23 handling is family-independent, said in the test) |
| RFC9012-13-10, 13-12, 13-16; RFC9830-2.3-1, 2.3-3 | tests | same unit: + dirty variant (unrecognized, malformed UDP-port, meaningless VXLAN, Color/Egress sub-TLVs) gets the same verdict and handling as clean. - differential: verdicts of dirty and clean runs equal | batch 2: +/- revert enforceRFC7606 | enforced, judge call | Ze processes no sub-TLV; "process as if absent" observed as verdict + carry |
| RFC9830-2.4.1-5, 2.4.1-7, 2.4.2-8, 2.4.2-10, 2.4.3-5, 2.4.3-7, 2.4.4-5, 2.4.4.1-5, 2.4.4.1-7, 2.4.4.2.1-3, 2.4.4.2.1-5, 2.4.4.2.2-3, 2.4.4.2.3-2, 2.4.4.2.4-3, 2.4.6-6, 2.4.7-7, 2.4.8-7 | tests | transmission: new srpolicy TestRFC9830FieldsToIgnoreAreZeroOnTransmission (+ field zero, - neighbour value field all-ones). receipt: reactor TestRFC9830ReservedFieldsIgnoredOnReceipt (+ all-ones fields: ActionNone, forwarded rebuilt octet-equal; - verdict equals the zero run) | batch 2: receipt +/- revert enforceRFC7606; transmit +/- revert srpolicy config.go::buildTunnelEncap | enforced | |
| RFC9830-2.4.4.2.3-3 | tests | receipt only (no transmission clause): reactor TestRFC9830ReservedFieldsIgnoredOnReceipt +/- | batch 2 | enforced | |
| RFC6793-4.2.3-10 | tests | + TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended: AS_SEQ 64500, CONFED_SEQ 65001, AS_SEQ TRANS TRANS vs 2-hop AS4_PATH -> confed segment prepended after 64500. - same confed segment after a non-prepended segment is dropped | batch 3 (revert ReconcileASPathFamily) | enforced | |
| RFC4360-x-1 | tests | + TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities (8, 16 octets). - 12, 20 octets ErrInvalidLength | batch 3 (revert ParseExtendedCommunities) | enforced | |
| RFC4360-2-1 | tests | rib TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets: + equal copy; - each of 8 octets changed -> unequal (drives extCommunitiesEqual) | batch 3 (revert extCommunitiesEqual) | enforced | other comparators (filter plugins) not driven |
| RFC8092-6-3 | tests | + reactor TestRFC8092DuplicateLargeCommunityIsNotMalformed: duplicate value through enforceRFC7606 -> ActionNone, no error, attr kept. Row carries {single-polarity: positive} | batch 3 (revert enforceRFC7606) | enforced | |
| RFC8092-3-2 | DEFECT (D-8), stopped | untagged RED test reactor TestRFC8092RedundantLargeCommunityRemovedBeforePropagation: received duplicate large community is forwarded with both copies | none | weak until fixed | Recommendation: remove redundant values on the receive path where enforceRFC7606 already rewrites the payload (RFC 8092 Section 3 quote above the statement), so RIB, replay and forward all see the deduped attribute; design choice (payload rewrite cost on the zero-copy path) -> main thread. rib parseLargeCommunityWire also keeps duplicates |
| RFC8950-3-2 | unresolved | -- | -- | weak | not started: needs a zero-RD assertion on the encoder (mpnlri.go VPN next hop) and the 48-octet link-local form; decode skips RD without checking |
| RFC6396-4.3.4-1 | unresolved | -- | -- | weak | not started: needs a 2-byte-session route through the RIB into a written MRT RIB entry (internal/plugins/mrt) |

Ids left (unresolved): RFC8950-3-2, RFC6396-4.3.4-1; RFC8092-3-2 waits on the defect fix.

## Files changed
- internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go (new)
- internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go (new)
- internal/component/bgp/reactor/rfc8092_duplicate_not_malformed_test.go (new; holds the intentionally RED untagged test)
- internal/component/bgp/plugins/nlri/srpolicy/rfc9830_transmit_zero_test.go (new)
- internal/component/bgp/plugins/rib/rfc4360_equality_test.go (new)
- internal/core/bgp/attribute/rfc7311_tlv_test.go (new)
- internal/core/bgp/attribute/rfc6793_confed_adjacent_test.go (new)
- internal/core/bgp/attribute/rfc4360_length_test.go (new)
- internal/core/bgp/attribute/rfc9012_test.go (two RFC9012-13-7 tag lines removed; approval recorded)
- rfc/short/rfc9012.md (RFC9012-13-7 row removed)
- rfc/extraction/rfc9012.json (site 13:7 excluded; signed-off, resign-reason)
- rfc/corrections/rfc9012.md (new, Retired paragraph)
- rfc/discrimination/rfc7311.json, rfc9012.json?, rfc9830.json, rfc6793.json, rfc4360.json, rfc8092.json (records from batches 1-3)

## Results (final)
- Records: batch 1 7/7, batch 2 102/102, batch 3 7/7, all OBSERVED red, no refusal.
- go test -race (attribute, rib, srpolicy, reactor): all ok except the deliberate red TestRFC8092RedundantLargeCommunityRemovedBeforePropagation.
- ./le rfc check on my stems: only STALE-verdict lines for the re-judged ids (expected, judges re-stamp), plus `rfc/audit/rfc9012.json: RFC9012-13-7 is not a requirement` -- the retired row's old verdict; per P-2 the audit entry is not hand-deleted, so the judge replaces it through audit-stamp.
- golangci-lint not run (it would outlast the 5-minute window); owed by the main thread.

## Gates owed (main thread)
- go test -race and golangci-lint on internal/core/bgp/attribute, internal/component/bgp/reactor, .../plugins/rib, .../plugins/nlri/srpolicy (reactor has the one deliberate red).
- ./le rfc check lines for rfc7311, rfc9012, rfc9830, rfc6793, rfc4360, rfc8092.
- New test files rfc4360_length_test.go, rfc4360_equality_test.go, rfc7311_tlv_test.go, rfc6793_confed_adjacent_test.go, rfc7311_aigp_other_tlvs_test.go lack the VALIDATES/PREVENTS file header the edit hook warns about (warning only).
