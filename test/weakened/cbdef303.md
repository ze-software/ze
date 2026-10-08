# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| TestRegisterJSONFormatter | Retire the literal callback registration/getter echo, not real formatter behavior. Prefix-SID payload-to-JSON assertions and IPv6 community rendering/wrong-shape refusals remain. Independent AttributeIsolationReview accepted removal; the full attribute package and real filter-community formatter roots passed twenty race iterations with a deletion overlay matching the native removal. |

## Reviewed fixture corrections

These semantic explanations remain recorded even though the native commit
detector reports no structural weakening for these carriers.

- `test/plugin/llgr-transition.ci`: Replaced configured-static reconnect observations with received-route GR-to-LLGR state transitions, NO_LLGR removal, and an observer-fenced real TCP loss. The old static replay did not prove retention. The draft completed 80 physical runs before promotion.
- `test/plugin/llgr-rib-stale.ci`: Replaced static replay after reconnect with received-route retention, LLGR_STALE wire output, and exact EOR-triggered removal of only the unrefreshed route. Receiver acknowledgements fence each phase; refreshed-route withdrawal remains forbidden. The draft completed 80 physical runs.
- `test/plugin/llgr-readvertise.ci`: Replaced static replay with actual destination delivery of retained routes carrying LLGR_STALE and withdrawals of NO_LLGR and the nonretained family. Receiver acknowledgements fence TCP loss and stale delivery; retained-route withdrawal remains forbidden. The draft completed 80 physical runs.
- `test/plugin/initial-sync-barrier-raw.ci`: Raw injection is outside the initial-route completion fence. The promoted fixture requires both the exact injected route and EOR without imposing their relative order; it does not make raw injection delay EOR. Captured-session completion and 80 physical runs cover the revised fixture.
- `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci`: Corrected the recognized EVPN input records to valid native framing instead of treating truncated recognized payloads as valid controls. The unknown-record rejection and adjacent recognized-record delivery assertions remain. The draft completed 80 physical runs.
- `test/plugin/rfc7606-54-discard-unrecognized-mup-nlri.ci`: Corrected the recognized MUP input records to valid native framing. The unknown-record rejection remains active through an added actual-recipient fence and linger; adjacent recognized records must arrive. The draft completed 80 physical runs.
- `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci`: Replaced the retired RS command builder's synthesized attributes and sorted-order assumption with actual mandatory-RIB source-owned recovery. Both complete MP_UNREACH-only frames must arrive, byte-exact, one message per check, in either order. No NLRI may disappear or duplicate its sibling's proof. The draft completed 80 physical runs.
- `test/plugin/vpn-withdrawal-inventory-rs-ipv4.ci`, `test/plugin/vpn-withdrawal-inventory-rs-ipv6.ci`: Mandatory-RIB recovery uses RFC8277 Section2.4's recommended Compatibility value `0x800000`, not the announcement's label. Both distinct survivor identities still require complete MP_UNREACH attributes, in either order, and the explicit `0x123450` receive control remains. Each draft completed 80 physical runs.
- `test/plugin/vpn-withdrawal-inventory-rr-ipv4.ci`, `test/plugin/vpn-withdrawal-inventory-rr-ipv6.ci`: Removed only the relative-order assumption between two complete, distinct survivor withdrawals. Each check still consumes one message, and the explicit changed-Compatibility withdrawal remains. Each draft completed 80 physical runs.
- `test/plugin/plugin-check.ci`: Changed only its IPv6 next-hop input and corresponding exact MP_REACH bytes from loopback `::1` to `2001:db8::1`. The `::1/128` NLRI, IPv4 frames and EOR assertions remain. The full live run exposed the newly enforced refusal of the old input; corrected `plugin-check` passed the real peer exchange in `74ad911d`.
- `test/encode/cap-refuse-asn4.ci`, `test/encode/cap-require-asn4.ci`: Corrected the obsolete zero-length capability tuple `4100` to the complete causing OPEN tuple `41040000FDE9`, as RFC5492 Section5 requires. Both refusal/requirement policies and exact NOTIFICATION code/subcode remain. Native encode run `c5051c0d` exposed both mismatches; no producer change was made to satisfy an invalid oracle.
- `test/encode/extended-nexthop-encode.ci`: Preserved both prefixes and next-hop endpoints, with the reverse IPv4 endpoint explicitly encoded as `::ffff:170.170.170.170` in the defined sixteen-octet AFI2 field. Removed only its misleading reverse capability5 tuple and the malformed four-octet expectation; all EOR obligations remain. Independent review rejected replacing the endpoint or dropping the reverse case. Native `c5051c0d` exposed the invalid old field.

Pending, outside this commit: `test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci`
now uses distinct speaker, recipient and next-hop-entity namespaces. Complete
live and replay UPDATEs remain byte-exact. The replacement carrier passed once
on kernel7.2 with unchanged guest-root stateless nft rules and tables
(`job-joint-subnet-native-command-scoped-oracles-cccf06c2.log`); the older
eighty-run evidence predates that replacement and does not qualify it. Its
RFC2545-3-1 native record is present; the RFC2545-3-2 record and fresh eighty-run
qualification remain pending.
