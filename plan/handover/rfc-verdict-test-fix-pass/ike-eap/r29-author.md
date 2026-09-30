# r29 author (ike-eap child): R29 defects, R28, 2.10-3, 5-2

| id | resolution | what now proves it | records | expected verdict | notes |
|----|-----------|--------------------|---------|------------------|-------|
| R29-a catch-all fails open | defect FIXED | engine/unmatched_failclosed_test.go (untagged): TestUnmatchedDiscardInstallFailureFailsTheApply (applyConfig both phases, failing backend loaded via dataplane.Load), TestUnmatchedDiscardWithNoEnforcingDataplaneIsAnError, TestUnmatchedDiscardRefusedAtVerifyWhereItCannotBeEnforced. Red observed (3 FAIL) before the fix, green after | none (untagged) | n/a | installUnmatched returns error; discard not installed (nil dp, unsupported, install error) -> error; bypass tolerated (kernel passes anyway). applyConfig installs catch-all FIRST (before SPD entries, cookie threshold, peers) and returns the error. OnConfigVerify refuses `unmatched discard` via verifyUnmatchedEnforceable -> new optional dataplane.CatchAllInstaller (xfrm linux: opens NETLINK_XFRM; xfrm_other and VPP: ErrNotSupported; noop: nil; undeclared backend = unable). RFC 4301 4.4.1 + 5 quotes above the enforcing return. Existing call sites now check the error (D-15 approvals: engine.TestRFC4301UnmatchedDiscardIsTheLastEntryOfEveryDatabase, ...NeverDiscardsWhatTheOperatorDidNotAskToDiscard (protect case now also asserts the error), ...InboundClearPacketMatchingNoEntryFollowsTheUnmatchedLeaf; helpers finalEntriesFor, outStepSPD, inStepSPDI). Docs: docs/guide/ipsec.md "When the entry is not there", docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md |

Files (R29-a): internal/component/ike/engine/{unmatched.go,apply.go,register.go,unmatched_failclosed_test.go,rfc4301_unmatched_test.go,rfc4301_final_entry_test.go,rfc4301_outbound_steps_linux_test.go,rfc4301_inbound_steps_linux_test.go}, internal/component/ike/dataplane/{dataplane.go,xfrm_linux.go,xfrm_other.go,vpp_policy.go,noop.go}, docs/guide/ipsec.md, docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md
| R29-b zero-value transforms | defect CONFIRMED in engine; failing test written, fix STOPPED (brief rule c: large cascade + design choice) | engine/transform_lookup_zero_test.go TestEngineProposalBuilderNeverOffersAZeroTransform (untagged): RED observed ("proposal 1 offers ENCR 0 for 3des") | none | n/a | CONFIG HALF IS ALREADY DONE, the R29 premise "config accepts 3des/chacha20poly1305" is stale: ipsec/config.go:264 (ESP), :437 (IKE enc), :455 hash, :474 DH refuse via EncryptionImplemented/HashImplemented/DHGroupImplemented, all derived from the crypto registry (ipsec/algorithm_support.go); proven by TestParseRejectsUnimplementedEncryption, TestAcceptedAlgorithmsResolveToTransforms; docs/guide/ipsec.md:125 already says refused at commit. So the engine zero is reachable only by an IPsecConfig built outside the parser. Error propagation touches 17 prod sites incl. functions with no error return: buildSAInitRequest ([]byte), handleSAInitRequest (void, responder), PeerSession.Info (display), espConfigForAccepted (bool), buildESPProposals, buildWireESPProposals, respondChildRekey, applyChildRekeyResponse, createFirstChildSA, initiateIKERekey, respondIKERekey, verifyAcceptedOffer, buildChildSAResponsePayloads, selectResponderESP; plus ~50 test call sites (many RFC-tagged units -> D-15 approvals). RECOMMEND: either (1) the full (T,error) cascade in a dedicated continuation, or (2) panic("BUG: ...") in the three lookups, which the style guide names for a state only a Ze defect can produce (config parse is the first check, no peer input reaches these). Owner/main thread picks; R29 says (1). |
| RFC4301-5.2-1, RFC4301-5.2-9 (R28) | gap KEPT, reason rewritten on the fixed code | no tag change; netns probe not needed: the paths without an installed catch-all are found at the producer | none | gap | Paths with NO SPD-I catch-all: (a) VPP: vppPolicyInterface refuses IfIndex 0, so no catch-all under default bypass and config not refused (only discard is refused at verify); (b) before first apply / after exit; (c) bypass install failure or no-XFRM platform tolerated. rfc/short/rfc4301.md: both {gap} texts and the Support-remaining sentence updated. 4.4.1-3 deviation unchanged. Owner option to close: refuse `vpn ipsec` on VPP unless the catch-all can be expressed per interface, and fail the apply on a bypass install failure. |
| RFC7296-5-2 (retag) | tests | + TestIKENeverNegotiatesNullAlgorithms (lookup refusal; its trailing block relabelled CONTROL, "negative" tag removed); + TestRFC7296NegotiationRefusesNullIntegrityAndNullCipher (positive: real proposal accepted); - same unit (negative): NegotiateIKE and VerifyAcceptedIKE refuse AUTH_NONE+AES-CBC with ErrProposalIncomplete and ENCR_NULL with ErrTransformUnspecified (errors.Is, which error asserted) | 3 records, observed red: 5-2 negative (revert ikeProposalComplete), 5-2 positive null test (revert NegotiateIKE), 5-2 positive TestIKENeverNegotiatesNullAlgorithms (revert LookupIntegrity) | enforced | D-15 approvals for both units. Files: internal/component/ike/crypto/rfc7296_test.go, internal/component/ike/crypto/rfc7296_null_negotiation_test.go, rfc/discrimination/rfc7296.json |
| RFC7296-2.10-3 (D-10) | row | + TestRFC7296EmittedNoncesMeetHalfOfEveryPRFKey, - TestRFC7296EmittedNoncesMeetTheLargestPRFBound (unchanged tests; the row's gated clause is the half-PRF floor) | 2 records re-recorded, observed red (revert engine/sa.go::GenerateNonce) | enforced | Row text widened to the whole verbatim §2.10 sentence; Correction 2026-09-30 paragraph in rfc/corrections/rfc7296.md states the gated clause and names 2.10-4 / 2.10-2 for the leading clauses. Files: rfc/short/rfc7296.md, rfc/corrections/rfc7296.md, rfc/discrimination/rfc7296.json |

## Gates run (this author)
- go test -race ike/{engine,dataplane,crypto,ipsec}: all ok except the deliberate untagged red TestEngineProposalBuilderNeverOffersAZeroTransform (R29-b, awaiting the fix decision).
- golangci-lint ike/{engine,dataplane,crypto}: 0 issues.
- ./le rfc check: rfc4301/rfc7296 lines are only STALE verdicts on rows this pass changed (RFC4301-5.2-1, 5.2-9, 4.4.1-3 via installUnmatched; RFC7296-2.10-3 row text; RFC7296-5-2 units) -> judges re-judge; and SHIFTED verdicts (RFC4301-5.1-4/5/6, 5.2-6/7/8/10; RFC7296-1.7-2, 2.17-1/2) -> judges reseal. No record, quote, coverage or allocation refusal on either stem.
- Owed by main thread / fresh agent: ./le verify worktree; the Linux netns units (rfc4301_inbound/outbound_steps_linux_test.go) ran inside the -race package run above.

## All files changed
internal/component/ike/engine/unmatched.go
internal/component/ike/engine/apply.go
internal/component/ike/engine/register.go
internal/component/ike/engine/unmatched_failclosed_test.go (new)
internal/component/ike/engine/transform_lookup_zero_test.go (new, deliberately red)
internal/component/ike/engine/rfc4301_unmatched_test.go
internal/component/ike/engine/rfc4301_final_entry_test.go
internal/component/ike/engine/rfc4301_outbound_steps_linux_test.go
internal/component/ike/engine/rfc4301_inbound_steps_linux_test.go
internal/component/ike/dataplane/dataplane.go
internal/component/ike/dataplane/xfrm_linux.go
internal/component/ike/dataplane/xfrm_other.go
internal/component/ike/dataplane/vpp_policy.go
internal/component/ike/dataplane/noop.go
internal/component/ike/crypto/rfc7296_test.go
internal/component/ike/crypto/rfc7296_null_negotiation_test.go
docs/guide/ipsec.md
docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md
rfc/short/rfc4301.md
rfc/short/rfc7296.md
rfc/corrections/rfc7296.md
rfc/discrimination/rfc7296.json
(plus ./le rfc approve unit writes: tmp/commit-rfc-approved-*.md for 5 units)
