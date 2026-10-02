# Handoff: spec-rfc-verdict-test-fix-pass (2026-10-02, end of session e08980d7)

Goal (owner, unchanged): close plan/pre-release/spec-rfc-verdict-test-fix-pass.md and its
eight children. Done = parent AC-11: every child closed, the derived weak/wrong listing holds
only verdicts named in another spec's acceptance criteria, /ze-review gate clean,
`./le verify worktree` has run.

Every file the next session needs is in this directory, committed. Read in this order:
NEXT-SESSION.md, this file, RULINGS.md (R1..R59, owner rulings 2..8), AUTHOR-BRIEF.md and
JUDGE-BRIEF.md (addendums binding), the last ~20 lines of QUEUE.md, then the per-child author
files named below.

## Child state

| Child | State | Next |
|---|---|---|
| vrrp | CLOSED | post-hoc AC-C2 check only |
| bfd | AC-C2 sweep committed (66249a3391); owner ruling on 6.1-3/6.8.6-15 applied (1248bf9d3d) | 3 Linux-guest records owed (RFC5880-9-2 +, RFC5881-5-1 +, RFC5881-5-3 +); verify; /ze-close |
| routing | AC-C2 committed (41faabbc4b); tag claims fixed (1075b8992f); LDP session-up fix landed (8ca6178596) | re-record the 22 stale rfc5036/rfc5561 records (step 1 below); rfc5036 rows for 2.5.3 items 2.c/2.d and the OPENREC "other message gets a NAK" gap belong here; verify; /ze-close |
| services | AC-C2 committed (a640d04015) | verify; /ze-close |
| access | RFC1332-2.1-1 enforced (b3c573f601); listing = 3 rows blocked by spec-radius-rfc-defects | RFC2866-4.1-1 Linux recording (owner); verify; /ze-close |
| ike-eap | AC-C2 committed (6b968f093b); tags fixed (1075b8992f); RFC9190 re-recorded (4074fd651a) | RFC4555-4.2.5-1 positive record owed on a Linux host; verify; /ze-close |
| ospf | 3 rows left: RFC4302-3.4.3-1 (start from TestIPsecInstallerXFRMReplayWindow, ipsec_integration_linux_test.go: 64-on and 0-off for AH), RFC2328-4.4-1 (Linux-only, kernel discriminate-record), RFC3101-3.2-2 (blocked) | 2 stale rfc2328 records (step 1); then the 2 Linux rows; close |
| bgp | ~55 unblocked rows: below rfc8000 ~30 (queue order in the c34 section of tmp state, copied to bgp/c34-author.md), rfc8000+ ~25 (bgp/c35-author.md) | owner decisions below; author/judge pairs, two authors partitioned by stem number |

## Step 1 for the next session (mechanical, no decision)
`./le rfc check` exits 2 with 27 violations at handover:
- 22 records in rfc/discrimination/rfc5036.json and rfc5561.json: producer-changed/unit-changed by the LDP fix 8ca6178596 (handleInit, processMessages, runSession, DecodeInit). Re-record each on its producer (revert route, observed red) under the per-stem lock; rejudge any verdict the tool marks stale.
- 2 records in rfc/discrimination/rfc2328.json (RFC2328-13-7 +, RFC2328-13-1 -) on lsdb/flooding.go::ReceiveUpdate, changed by c797d66a9b. Re-record.
- 3 owner items (below).

## Owner items (decide before the next BGP authors start)
| Item | Recommendation |
|---|---|
| rfc/audit/rfc8955.json still has the RFC8955-6-2 entry (row retired under RFC 9117 in 2046bf766a). Auto mode refuses agents deleting it | Owner deletes the key by hand (as for RFC4577, 5cdad9cd96), or orders the main thread to |
| rfc/extraction/rfc8955.json: exclusions 12 -> 13 with the same signed-off date; auto mode refused updating the date | Owner updates `signed-off` or orders it |
| RFC5575-6-1 [MUST] has no annotation: RFC 9117 section 7 made the AS_PATH check optional and Ze does not enforce it | Record it as a disclosed deviation ({gap} citing RFC 9117 section 7) |
| ROUTE-REFRESH does not re-send config-static routes (RFC2918-4-3; red test test/draft/plugin/refresh-config-static.ci) | Fix A: the RIB keeps config-static routes in ribOut with a flag; peer-up replay skips them, a refresh includes them |
| VPP SRv6 service routes never install (RFC9252-5-3; red test fib/vpp/rfc9252_srv6_encap_red_test.go): no sr_policy_add | New spec; local BSID allocated from the local SRv6 locator |
| Duplicate Prefix-SID TLVs relayed (RFC8669-6-3; red test reactor/rfc8669_duplicate_tlv_red_test.go) | P-3 into spec-bgp-prefix-sid-rfc-defects; types 5 and 6 single-occurrence (RFC 9252 section 7) |
| Label-Index Reserved/Flags on relay (RFC8669 3.1-3/3.1-5) | Relay unchanged (RFC 8669 section 3.2 forbids changing the attribute in propagation) |
| spec-bgp-addpath-best-path-per-prefix: when it runs | After this pass, before release |
| spec-bgp-llgr-per-family-config: existing per-peer configs | One-shot migration, byte-identical OPEN |
| RFC9514-7.2-5/6: {gap} is refused beside tags (annotationBarsATest, check_core.go) | Options a/b/c in bgp/c35-author.md |
| RFC9086-7-1: PeerAdj/PeerSet absent; R3 split leaves a fragment | Ruling in bgp/c35-author.md |
| RFC5082-3-3: Linux-only, likely no genuine negative (R1) | OWNER-GATE candidate, write-up in bgp/c34-author.md |
| RFC2866-4.1-1: owner sudo recording on Linux (NEXT-SESSION.md step 2) | still not in this checkout |

## Main-thread items without a decision
- MRT add-path parsing D-8 (RFC8050-x-4; red test internal/mrt/rfc8050_addpath_nlri_red_test.go): ParseBGPMessage/ParseMPReach/ParseMPUnreach parse NLRI with add-path off; fix the three signatures (~27 call sites); the writer picks the subtype from the negotiated add-path state of the UPDATE's family, not config.
- RFC8955-6-1 and RFC9117-4.1-1/4.2-1 weak: the best-match (longest covering prefix) selection in flowSpecAuthorized is untested; add two covering routes of different lengths with different originators.
- RFC9086-5-5 duplicates 5-3: retire 5-5 under D-3, move its 3 tags to 5-3.
- RFC9494-4.2-5 item 2 (NO_LLGR set by import policy) needs a .ci test.
- Orphan records left in place (owner ruling 6 approves deleting them; auto mode blocks agents): RFC4302-3.4.3-5 negative on TestIPsecReplayWindowRange. Collect any new ones and hand the owner one script (the orphans.py pattern in the session scratch).

## Session e08980d7, 2026-10-02: what landed (58 commits, none pushed)
Defects fixed: OSPF LS Update per-LSA discard (7488f60bc2); BGP-LS node descriptor TLV 518 (04b95f61d5); GR capability cleared on re-establish (a1a782247b); LLGR zero restart time (ba11f0e317); LLGR only for families Ze declared (3817eab289); GR capability lists only RIB-storable families (d34eecef09); RFC 8277 label Rsrv cleared on relay (c822fc2a81); SR-Algorithm TLV length (55581e8304); mapping-server PHP (30b572216a); LDP session up only when Operational (8ca6178596, interop ospf-ldp-sync-frr passes); static routes only for negotiated families (db87202229); opaque LSA 4-octet alignment (c797d66a9b). Gate: coverage ratchet owner-ruled move (03036dc006) and owner-ruled gap (c70b05c203). Specs: spec-bgp-addpath-best-path-per-prefix, spec-bgp-llgr-per-family-config (b4a4455529). RFC 9117 enrolled (2046bf766a).

## Known reds that are not this work
- Interop on this Mac: ospf-opaque-frr, ospf-te-frr, ospf-ext-prefix-link-frr fail at baseline (journal assertion-reads-state-the-scheduler-owns.md).
- Host-only (darwin): reactor 127.0.0.2 binds, link-local tests needing fd00::2 on loopback, TestRFC5187InterfaceIDPreservedAcrossRestart (needs lo).
- Full native verification never ran this session: rows in plan/verification-debt/6912077e.md. `./le verify worktree` on a quiet machine before any push.

## Process notes
- Commit session 6912077e has used every automatic tag: pass `tag <name>` to `./le commit create`.
- Agents hit the 100-call cap often: the main thread appends their final handoff lines (see "c29 close", "c30 close" in ospf-author.md).
- Unused D-15 rows accumulate in tmp/commit-rfc-approved-6912077e.md (the gate does not consume them for comment-only edits or new units); harmless.
- Auto mode blocks agents from deleting ledger records/audit keys and sometimes from `./le rfc reseal`: route those to the owner, do not work around it.
