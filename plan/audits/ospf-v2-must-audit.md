# OSPFv2 MUST compliance audit

Scope: OSPFv2 implementation under `internal/plugins/ospf`, checked against the repo RFC summaries for RFC 2328, RFC 3101, RFC 5709, and RFC 7474. RFC 6987 is noted separately because it is informational and the repo uses it for max-metric stub-router behavior.

Method: read-only source audit. No code edits, tests, lint, or verify gates were run. Evidence is file and line based from the current working tree.

## Summary

| Status | Count |
|--------|-------|
| Supported | 36 |
| Partial | 20 |
| Missing | 6 |

Highest-risk gaps found:

1. NSSA default-destination behavior is incomplete: Type-7 default install is not gated by P-bit, and default origination into every attached NSSA is config-gated rather than automatic.
2. Cryptographic auth accepts extra unauthenticated trailer bytes and does not select the receive key strictly by packet Key ID.
3. RFC 7474 AuType 3 lacks IP-source-bound Apad and persistent boot-count sequencing.
4. Flooding has several RFC 2328 edge gaps: MaxAge+MaxSequence older-instance silent discard, exact Table 19 ack behavior, and self-originated stale LSA flush when no local copy exists.
5. Positive-cost and positive-InfTransDelay requirements are not fully enforced for explicit zero configuration.

## RFC 2328, base OSPFv2

| # | Requirement | Status | Evidence | Finding |
|---|-------------|--------|----------|---------|
| 1 | Packets use the 24-byte common header and Version 2. | Supported | `internal/plugins/ospf/packet/header.go,126-135,172-181` | Version is fixed at 2, decoded packets with any other version are rejected, and encoding writes the standard header. |
| 2 | Packet bodies match the five OSPFv2 packet formats. | Supported | `internal/plugins/ospf/packet/hello.go`; `dbdesc.go`; `lsreq.go`; `lsupdate.go`; `lsack.go` | Hello, DD, LSReq, LSUpdate, and LSAck length and field layouts are decoded and encoded. |
| 3 | Packet checksum is the IP checksum over the OSPF packet excluding the 64-bit authentication field. | Supported | `internal/plugins/ospf/packet/checksum.go`; `internal/plugins/ospf/types/checksum.go` | Checksum logic skips bytes 16..23 and accepts checksum zero only for cryptographic auth. |
| 4 | LS checksum is Fletcher over the complete LSA excluding LS age and must not be zero. | Supported | `internal/plugins/ospf/types/checksum.go,86-94`; `internal/plugins/ospf/packet/checksum.go`; `internal/plugins/ospf/packet/lsa.go` | Encoding backfills the Fletcher checksum and zero checksum verifies false. |
| 5 | Router, Network, Summary, and AS-external LSA body formats, including RFC 2328 TOS blocks. | Partial | `internal/plugins/ospf/packet/lsa_router.go`; `lsa_network.go`; `lsa_summary.go`; `lsa_external.go` | Core fields are implemented. Router-LSA decode skips obsolete TOS metrics, Summary-LSA rejects extra TOS blocks, and encoders do not emit TOS metrics. |
| 6 | Flooding receive discards LSAs with invalid LS checksum. | Supported | `internal/plugins/ospf/lsdb/flooding.go`; `internal/plugins/ospf/lsdb/lsdb.go` | Invalid LSA checksum returns `bad-lsa-checksum` before install or flood. |
| 7 | Flooding receive discards LSAs of unknown LS type. | Partial | `internal/plugins/ospf/packet/lsa.go`; `internal/plugins/ospf/packet/lsa.go`; `internal/plugins/ospf/lsdb/lsdb.go` | Wire LSUpdate decoding rejects unknown types through `DecodeLSAHeader`. The LSDB direct install path has no independent LS type guard for constructed LSAs. |
| 8 | Type 5 AS-external LSAs are not flooded into or exposed through stub areas. | Supported | `internal/plugins/ospf/lsdb/flooding.go,238-252`; `internal/plugins/ospf/lsdb/lsdb.go` | Receive, send eligibility, lookup, and summary filtering block Type 5 in stub-like areas. |
| 9 | Link State Update and Link State Acknowledgment from neighbors below Exchange are dropped. | Supported | `internal/plugins/ospf/instance.go,187-211`; `internal/plugins/ospf/neighbor/table.go` | Instance handlers call `AcceptsFlooding` before LSDB processing; neighbors below Exchange are rejected. |
| 10 | Newest LSA comparison uses sequence, then checksum, then MaxAge, then age difference greater than MaxAgeDiff. | Supported | `internal/plugins/ospf/lsdb/entry.go`; `internal/plugins/ospf/lsdb/lsdb.go` | `CompareHeaders` implements the RFC order and install uses it before replacement. |
| 11 | If DB copy is MaxAge with MaxSequenceNumber, discard a received older instance silently. | Missing | `internal/plugins/ospf/lsdb/flooding.go`; `internal/plugins/ospf/lsdb/entry.go` | The code sends the database copy back for an older received LSA and has no special MaxAge+MaxSequence silent-discard branch. |
| 12 | Add flooded LSAs to the adjacency retransmission list and retransmit every RxmtInterval until acknowledged. | Partial | `internal/plugins/ospf/lsdb/flooding.go,269-303,344-360` | Queuing, retransmission, and ack clearing exist. New entries have `last` unset, while `RetransmitTick` waits only when `last` is non-zero, so a tick can retransmit before RxmtInterval. |
| 13 | Increment outgoing LS age by positive InfTransDelay and cap at MaxAge. | Partial | `internal/plugins/ospf/lsdb/flooding.go,255-260,349-353`; `internal/plugins/ospf/lsdb/entry.go`; `internal/plugins/ospf/types/lsage.go`; `internal/plugins/ospf/config.go` | Age addition caps at MaxAge and retransmit defaults to 1 second. Explicit configured `transmit-delay` zero is accepted, so the positive-delay MUST is not fully enforced. |
| 14 | Never age past MaxAge and exclude MaxAge LSAs from route calculation. | Supported | `internal/plugins/ospf/types/lsage.go`; `internal/plugins/ospf/spf/graph.go`; `internal/plugins/ospf/spf/interarea.go`; `internal/plugins/ospf/spf/external.go` | LS age saturates, and SPF graph, summary, and external route paths skip MaxAge LSAs. |
| 15 | Remove a MaxAge LSA only after it is on no retransmission list and no neighbor is in Exchange or Loading. | Supported | `internal/plugins/ospf/lsdb/aging.go`; `internal/plugins/ospf/lsdb/flooding.go,508-537` | Purged entries are retained until retransmit and Exchange/Loading blockers clear. |
| 16 | Acknowledge every newly received LSA according to RFC 2328 Table 19. | Partial | `internal/plugins/ospf/lsdb/flooding.go,408-421,438-457` | Direct and delayed ack paths exist. The code always delays non-duplicate, non-flooded acks and never passes `floodedBack=true`, so Backup/DR-specific Table 19 cases are incomplete. |
| 17 | Detect self-originated LSAs and re-originate or flush by premature aging. | Partial | `internal/plugins/ospf/lsdb/origination.go,476-523` | Detection and re-origination exist. A received self-originated LSA with no local copy is dropped instead of flushed by premature aging. |
| 18 | Intra-area SPF includes transit links only when the reverse link exists and the target LSA is not MaxAge. | Supported | `internal/plugins/ospf/spf/graph.go`; `internal/plugins/ospf/spf/spf.go,224-254` | Graph build skips MaxAge, and SPF checks reciprocal router and network links before using an edge. |
| 19 | Route preference: intra/inter-area over external, Type 1 external over Type 2, then Type 2 metric. | Supported | `internal/plugins/ospf/spf/route.go,194-198`; `internal/plugins/ospf/spf/external.go` | Route ranks and external comparison implement the RFC preference order. |
| 20 | ABRs compute inter-area routes from backbone summary-LSAs only. | Supported | `internal/plugins/ospf/spf/interarea.go` | ABR mode filters candidate summary-LSAs to the backbone. |
| 21 | Skip summary and AS-external LSAs with LSInfinity cost, MaxAge, or self-originated. | Supported | `internal/plugins/ospf/spf/interarea.go`; `internal/plugins/ospf/spf/external.go` | Summary and external route calculations skip unreachable, MaxAge, and self-originated LSAs. |
| 22 | Simple password AuType 1 packets must match the configured 64-bit password. | Supported | `internal/plugins/ospf/packet/auth_verify.go,197-200`; `internal/plugins/ospf/auth_wiring.go`; `internal/plugins/ospf/auth_keystore.go` | Signing fills the 8-byte auth field and receive uses constant-time comparison. |
| 23 | AuType 2 cryptographic auth sets checksum zero, appends digest outside OSPF length, and covers only the OSPF packet length. | Partial | `internal/plugins/ospf/packet/header.go`; `internal/plugins/ospf/packet/auth_verify.go,201-213` | Encoding uses checksum zero and a digest trailer. Receive checks only that the trailer is long enough, not that `Packet Length + Auth Data Len == wire length`, so extra trailing bytes are accepted. |
| 24 | Cryptographic sequence number is non-decreasing, reset when neighbor goes Down, and updated on accepted packets. | Partial | `internal/plugins/ospf/auth_keystore.go,76-77,152-161` | Accepted sequence high-water marks are stored and updated. No neighbor-down reset hook or delete path was found. |
| 25 | DD Interface MTU is zero on virtual links. | Missing | `internal/plugins/ospf/neighbor/dd.go`; `internal/plugins/ospf/iface/ism.go`; `internal/plugins/ospf/spf/interarea.go` | Physical MTU mismatch is handled, but virtual links are not implemented, so the virtual-link DD MTU rule is absent for full RFC 2328 compliance. |
| 26 | Only one DD packet may be outstanding per adjacency. | Supported | `internal/plugins/ospf/neighbor/dd.go,166-183,197-205`; `internal/plugins/ospf/neighbor/table.go` | The neighbor table stores the last DD and retransmits only that packet. |
| 27 | BadLSReq is generated and exchange restarts when an LSReq names an absent LSA. | Supported | `internal/plugins/ospf/neighbor/lsreq.go` | Missing requested LSA restarts exchange, sends initial DD, records `bad-lsreq`, and moves to ExStart. |
| 28 | Interface output cost is positive. | Partial | `internal/plugins/ospf/instance.go,881-899`; `internal/plugins/ospf/config.go`; `internal/plugins/ospf/yang/ze-ospf-conf.yang` | Runtime defaults missing cost to 1, but explicit configured zero is accepted by `parseInterface`; the YANG range says 1..65535. |

## RFC 3101, NSSA

| # | Requirement | Status | Evidence | Finding |
|---|-------------|--------|----------|---------|
| 29 | Verify N-bit and E-bit in received Hellos match area type before adjacency. | Supported | `internal/plugins/ospf/iface/iface.go,625-634`; `internal/plugins/ospf/instance.go` | Hello receive validates the required options and rejects E/N mismatches. |
| 30 | Keep E-bit clear whenever N-bit is set. | Supported | `internal/plugins/ospf/instance.go`; `internal/plugins/ospf/lsdb/nssa.go` | NSSA area options clear E and set NP; Type 7 P-bit uses the NSSA option bit only when requested. |
| 31 | Originate NSSA LSAs with LS Type 7. | Supported | `internal/plugins/ospf/lsdb/nssa.go`; `internal/plugins/ospf/types/lstype.go` | `OriginateNSSA` uses `LSTypeNSSA` and installs the LSA in the area store. |
| 32 | Flood Type 7 LSAs only within the originating NSSA. | Supported | `internal/plugins/ospf/lsdb/flooding.go,238-252`; `internal/plugins/ospf/lsdb/nssa.go` | Receive accepts Type 7 only in NSSA areas and send eligibility requires same-area NSSA. |
| 33 | Set P-bit on Type 7 LSAs an NSSA internal ASBR wants translated to Type 5. | Partial | `internal/plugins/ospf/redist_wiring.go`; `internal/plugins/ospf/lsdb/nssa.go` | Redistributed routes set P only when Type 5 cannot be injected directly and a non-zero forwarding address exists. The generic LSDB originator accepts any caller-provided P-bit without policy checks. |
| 34 | If P-bit is set, forwarding address must be non-zero, otherwise do not originate Type 7. | Partial | `internal/plugins/ospf/redist_wiring.go`; `internal/plugins/ospf/lsdb/nssa.go` | Redistribute path guards P=1 on a non-zero forwarding address. `OriginateNSSA` itself does not reject P=1 with zero forwarding address. |
| 35 | Clear P-bit when the same network is also originated as Type 5. | Partial | `internal/plugins/ospf/redist_wiring.go`; `internal/plugins/ospf/lsdb/nssa.go` | Redistribute avoids P when Type 5 can be advertised. The generic Type 7 originator does not inspect same-network Type 5 state. |
| 36 | Clear P-bit on Type 7 default from an NSSA border router and install Type 7 default only if P-bit is set. | Missing | `internal/plugins/ospf/nssa.go`; `internal/plugins/ospf/spf/external.go` | ABR default origination uses P=0, but external route calculation treats Type 7 candidates without a default-route P-bit install guard. |
| 37 | NSSA border routers originate a default-destination LSA into every directly attached NSSA. | Partial | `internal/plugins/ospf/nssa.go` | The machinery can originate Type 7 or Type 3 defaults per NSSA, but only when that area's `DefaultOriginate` config is enabled. RFC 3101 says NSSA border routers must originate the default. |
| 38 | NSSA border routers set E-bit in Type 1 router-LSAs of directly attached non-stub areas. | Partial | `internal/plugins/ospf/lsdb/origination.go,92-100` | Type 1 E-bit is tied to existing self-originated Type 5 external LSAs, not explicitly to NSSA border-router status. |
| 39 | Support optional import of summary routes into NSSAs as Type 3 Summary-LSAs. | Supported | `internal/plugins/ospf/spf/area_type.go`; `internal/plugins/ospf/spf/summary.go` | NSSA areas accept Type 3 summaries unless `no-summary` is configured, and ABR summary origination includes NSSA policy. |
| 40 | Elect translator as the reachable NSSA border router with Nt set or highest Router ID. | Supported | `internal/plugins/ospf/nssa.go` | Translator role `never`, `always`, and candidate highest-Router-ID logic are implemented. |
| 41 | Translation sets advertising router to translator and preserves mask, path type, metric, forwarding address, and tag. | Supported | `internal/plugins/ospf/nssa.go`; `internal/plugins/ospf/lsdb/nssa.go` | Translation builds desired Type 5 LSAs from Type 7 body fields and originates them with `self` as advertising router. |
| 42 | Suppress duplicate translation when another equivalent Type 5 translator with higher Router ID exists. | Supported | `internal/plugins/ospf/nssa.go,207-244` | Translation skips zero forwarding address and duplicate Type 5 cases, then reconciles desired Type 5 state. |

## RFC 5709, HMAC-SHA OSPFv2 cryptographic authentication

| # | Requirement | Status | Evidence | Finding |
|---|-------------|--------|----------|---------|
| 43 | Implement HMAC-SHA-256 for OSPFv2 cryptographic authentication. | Supported | `internal/plugins/ospf/packet/auth_verify.go,47-63,65-77,103-109` | SHA-256 is implemented, with SHA-1, SHA-384, and SHA-512 also present. |
| 44 | Operators can configure any supported algorithm for any Key ID value. | Partial | `internal/plugins/ospf/yang/ze-ospf-conf.yang`; `internal/plugins/ospf/auth_keystore.go,144-145`; `internal/plugins/ospf/packet/auth_verify.go,206-213` | Config stores algorithm per key. Receive verification loops through configured keys and does not require the received Key ID to match the candidate key before accepting a digest. |
| 45 | HMAC packets use AuType 2, Auth Data Len equals hash length, and digest is appended after the OSPF packet. | Supported | `internal/plugins/ospf/packet/auth_verify.go,103-109,151-162`; `internal/plugins/ospf/packet/header.go` | HMAC algorithms map to AuType 2 unless extended sequence is selected, write the digest length, and append the digest outside the OSPF length. |
| 46 | 32-bit cryptographic sequence number follows RFC 2328 Appendix D. | Partial | `internal/plugins/ospf/auth_keystore.go,110-125,152-161` | Send increments sequence and receive enforces non-decreasing state. The neighbor-down reset required by RFC 2328 is not wired. |
| 47 | Authentication trailer length matches Auth Data Len exactly. | Partial | `internal/plugins/ospf/packet/auth_verify.go` | Too-short packets are rejected, but extra bytes after the digest are accepted. |
| 48 | Fill trailer with Apad, derive Ko to length L, and compute HMAC First-Hash and Second-Hash. | Supported | `internal/plugins/ospf/packet/auth_verify.go,79-100,103-109,123-132,201-213` | HMAC uses Apad, derives Ko by hash or zero padding, computes HMAC, and compares in constant time. |
| 49 | On receive, save wire digest, replace trailer with Apad, recompute, and compare. | Supported | `internal/plugins/ospf/packet/auth_verify.go` | Verification recomputes expected digest and compares it against the wire digest. |
| 50 | Select algorithm and key on receive implicitly from packet Key ID. | Partial | `internal/plugins/ospf/packet/auth_verify.go,206-213`; `internal/plugins/ospf/auth_keystore.go` | The packet Key ID is parsed, but candidate key KeyID is not compared in `packet.Verify`; any configured key with a matching digest can pass. |
| 51 | Ensure new key KeyStartGenerate is before or equal to old key KeyStopGenerate on rollover. | Missing | `internal/plugins/ospf/config.go`; `internal/plugins/ospf/auth_keystore.go` | Send and accept lifetimes are parsed, but no rollover overlap validation or most-recent-send-key selection was found. |
| 52 | Do not revert to unauthenticated operation when the last key expires. | Partial | `internal/plugins/ospf/config.go`; `internal/plugins/ospf/auth_keystore.go` | No expiry enforcement exists, so the code does not actively revert to unauthenticated state. The required key-expiry behavior is not modeled. |

## RFC 7474, extended sequence number authentication

| # | Requirement | Status | Evidence | Finding |
|---|-------------|--------|----------|---------|
| 53 | Carry the 64-bit sequence number in the 8 octets after the OSPFv2 packet and include it in digest computation. | Supported | `internal/plugins/ospf/packet/auth_verify.go,214-227` | AuType 3 appends and verifies the 64-bit sequence after Packet Length. |
| 54 | Compose 64-bit sequence as high-order boot count plus low-order strictly increasing counter. | Partial | `internal/plugins/ospf/auth_keystore.go,110-125` | Composition exists, but boot count persistence is explicitly not implemented. |
| 55 | Increment the lower-order 32-bit sequence for every OSPF packet sent. | Supported | `internal/plugins/ospf/auth_keystore.go` | Signing consumes the next send sequence per packet. |
| 56 | Preserve aggregate sequence strict increase for router lifetime, including cold restarts. | Missing | `internal/plugins/ospf/auth_keystore.go,110-125` | Boot count is process-local, so cold restart can reuse aggregate sequences. |
| 57 | On receive, accept only a sequence greater than the last accepted packet of that type from that neighbor. | Supported | `internal/plugins/ospf/auth_keystore.go,152-163` | Replay key includes interface, neighbor Router ID, key ID, and packet type; `seq <= last` is rejected. |
| 58 | Track accepted sequence high-water per neighbor and OSPF packet type. | Supported | `internal/plugins/ospf/auth_keystore.go,152-163` | The replay map key includes neighbor and packet type. |
| 59 | AuType 3 authentication field uses 24-bit reserved zero, Auth Data Len, and 32-bit Key ID. | Supported | `internal/plugins/ospf/packet/header.go`; `internal/plugins/ospf/packet/auth_verify.go` | AuType 3 writes the extended auth field and digest length. |
| 60 | Include the 64-bit sequence in the First-Hash input. | Supported | `internal/plugins/ospf/packet/auth_verify.go,224-227` | Digest computation covers the packet and the extended sequence material. |
| 61 | Initialize first 4 octets of Apad to the packet IP source address on send and receive. | Missing | `internal/plugins/ospf/packet/auth_verify.go,174-177,224-227`; `internal/plugins/ospf/auth_wiring.go,50-62` | Apad is always the fixed `0x878FE1F3` pattern, and sign/verify have no IP source address input. |
| 62 | Append OSPFv2 Cryptographic Protocol ID to the authentication key before AuType 3 digest use. | Supported | `internal/plugins/ospf/packet/auth_verify.go,174-177,224-227` | Protocol ID `0x0001` is appended to the key for signing and verification. |

## RFC 6987 stub-router behavior, informational but implemented here

| Requirement | Status | Evidence | Finding |
|-------------|--------|----------|---------|
| Advertise stub-router status by setting all non-stub Router-LSA links to MaxLinkMetric 0xFFFF. | Supported | `internal/plugins/ospf/lsdb/origination.go`; `internal/plugins/ospf/lsdb/origination_test.go` | Max-metric Router-LSA changes transit links and keeps stub links at their real cost. |
| Keep the router in the LSDB while advertising stub-router status. | Supported | `internal/plugins/ospf/instance.go`; `internal/plugins/ospf/lsdb/origination.go` | `originateSelfLSAs` still originates a Type 1 Router-LSA with modified link costs. |
| Support on-startup and on-shutdown max-metric operation. | Missing | `internal/plugins/ospf/yang/ze-ospf-conf.yang`; `internal/plugins/ospf/config.go`; `internal/plugins/ospf/instance.go` | Config fields are parsed, but origination only uses `router-lsa always`; startup and shutdown timers are not applied. |

## Out-of-scope or intentionally incomplete areas affecting full RFC coverage

| Area | Finding | Evidence |
|------|---------|----------|
| Virtual links | Not implemented. This blocks full RFC 2328 virtual-link compliance, including DD MTU zero and backbone repair through transit areas. | `internal/plugins/ospf/iface/ism.go`; `internal/plugins/ospf/spf/interarea.go` |
| NBMA | Not implemented in the audited interface state machine and config. NBMA-only MUSTs are therefore absent from the current feature set. | `internal/plugins/ospf/iface/ism.go`; `internal/plugins/ospf/config.go` |
| Obsolete TOS routing metrics | Parsed partially or rejected, not propagated through route calculation or origination. This is visible in LSA body codec behavior. | `internal/plugins/ospf/packet/lsa_router.go`; `internal/plugins/ospf/packet/lsa_summary.go` |

## Recommended fix order

1. Fix auth verification first: exact trailer length, Key ID selection, AuType 3 source-address Apad. These are narrow and security-sensitive.
2. Fix explicit zero validation for interface cost and transmit delay. These are low-risk config validation issues.
3. Fix flooding edge behavior: MaxAge+MaxSequence silent discard, Table 19 ack cases, self-originated no-local-copy flush, retransmit initial timestamp.
4. Fix NSSA defaults and P-bit policy at one boundary, preferably the LSDB origination API plus engine policy, so callers cannot bypass RFC 3101 constraints.
5. Decide whether full RFC 2328 virtual links and NBMA are in scope. If not, document them as unsupported protocol modes rather than partial compliance.
