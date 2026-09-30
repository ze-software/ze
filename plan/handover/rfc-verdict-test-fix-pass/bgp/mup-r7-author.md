# Author handoff: spec-rfc-verdict-fix-bgp, MUP 3.3-1 and 3.3.7-2 under ruling R7 (2026-09-29)

Continues `mup-srpolicy-author.md`, which left both rows as "judge's call". R7 + R2 applied.

| id | resolution | what now proves each clause | records written | expected verdict | notes |
|----|-----------|-----------------------------|-----------------|------------------|-------|
| DRAFT-IETF-BESS-MUP-SAFI-3.3-1 | row (retired) | n/a: binds the PE and MUP Controller roles (§3.3), which Ze does not implement | 2 removed from rfc/discrimination/draft-ietf-bess-mup-safi.json | none (row gone, audit entry removed) | Capability negotiation tags moved to RFC4760-8-2 (see next row). Tags on `TestRFCMUPFamiliesCoverBothAFIs` and `TestRFCMUPRejectsNonMUPFamily` dropped (family constants, codec scope: no row states them). Extraction site 3.3:1 is excluded binds-another-role |
| DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2 | row (retired) | n/a: binds the MUP Controller role (§3.3.7) | 1 removed | none | Tags on `TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI` and the second tag of `TestMUPSessionNegotiatesBothAFIs` dropped (EncodeRoute AFI + SAFI 85: no RFC 4760 row states it). The EncodeRoute assertion stays in the test body, untagged. Extraction site 3.3.7:2 is excluded binds-another-role |
| RFC4760-8-2 (receives the moved tags) | tests | + `TestMUPSessionNegotiatesBothAFIs`: when both speakers advertise MP(1,85)+MP(2,85), capability.Negotiate makes both families usable. - `TestMUPSessionMissingAFIIsNotNegotiated`: a family the peer did not advertise is not usable (peer v4-only leaves ipv6/mup out; peer with none leaves both out) | +/- revert of `capability/negotiated.go::Negotiate`, both OBSERVED red, in rfc/discrimination/rfc4760.json | judge's call (SHOULD row, had no verdict and no tags before). Chosen over RFC4760-8-1 because the tests vary the PEER's advertisement ("determine whether the speaker could use MP with a particular peer"); 8-1 is about ze's own advertisement, which neither test varies | Approvals recorded (D-15) for all five units |

Checks run once each: `go test -race` of nlri/mup under `./le job run`: ok. `./le rfc check`: first run refused the extraction exclusion rise (18 to 20) with no resign-reason; fixed (`resign-reason`, `signed-off` 2026-09-29). Final run: no finding names draft-ietf-bess-mup-safi or rfc4760; the remaining 215 findings belong to other stems (other sessions).

Needs the main thread:
- Consistency flag, not acted on: sibling controller/PE rows (3.3.7-1, 3.3.8-1, 3.3.10-1/-2, 3.3.11-1, 3.3.1-x, 3.3.4-x) are still `{gap}` rows that bind the same roles, while R7 retires 3.3-1 and 3.3.7-2 as binds-another-role. rfc-compliance.md presumes binds-another-role wrong. If R7's reading holds, those rows need the same treatment; if the gaps are right, R7 needs to be reconsidered. This is an owner-level question about the MUP role scope.
- Judge: re-judge RFC4760-8-2 (new tags). Do not stamp from here.
- Gates owed: `./le go lint run` over nlri/mup (only test files changed, only comments).
- `git diff rfc/audit/draft-ietf-bess-mup-safi.json` also shows a concurrent `./le rfc reseal` edit (reaudit_history, one tests hash) that is not mine.

## Files changed
- internal/component/bgp/plugins/nlri/mup/rfc_mup_session_test.go (2 tags retargeted to RFC4760-8-2, 1 tag removed, doc comment)
- internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go (3 tag lines removed)
- rfc/short/draft-ietf-bess-mup-safi.md (rows 3.3-1 and 3.3.7-2 deleted)
- rfc/corrections/draft-ietf-bess-mup-safi.md (two `Retired 2026-09-29:` paragraphs)
- rfc/extraction/draft-ietf-bess-mup-safi.json (sites 3.3:1, 3.3.7:2 excluded binds-another-role; signed-off 2026-09-29; resign-reason)
- rfc/audit/draft-ietf-bess-mup-safi.json (entries 3.3-1, 3.3.7-2 removed)
- rfc/discrimination/draft-ietf-bess-mup-safi.json (3 records removed)
- rfc/discrimination/rfc4760.json (2 records added by discriminate-record)
- approvals recorded by `./le rfc approve unit` for mup.TestMUPSessionNegotiatesBothAFIs, mup.TestMUPSessionMissingAFIIsNotNegotiated, mup.TestRFCMUPFamiliesCoverBothAFIs, mup.TestRFCMUPRejectsNonMUPFamily, mup.TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI (wherever the tool stores them)
