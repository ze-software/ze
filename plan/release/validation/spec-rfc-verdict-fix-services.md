# Spec: rfc-verdict-fix-services

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P2 |
| Release reason | Required evidence for shipped behavior or truthful release support claims; historical work and proof debts remain distinct. |
| Scope | tooling |
| Depends | `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the services group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Owner permission for this work, 2026-11-14: the owner said the in-progress cap does not apply to this closure work, so the spec was claimed above the cap and its remaining implementation is done in the same session.

## Task

This spec is child 8 of 8 of `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`, the services group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **78 weak and 7 wrong, 85 in all, over 21 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

Re-measured 2026-11-14 with the same selection over the per-requirement audit layout (Derived listing), counting `partial` beside weak and wrong because `checkAuditFindings` treats it as a finding: **5 weak, 4 partial and 0 wrong, 9 in all, over 4 stems** (rfc2131, rfc7950, rfc9728, sflow-v5), each owned by another spec ("Blocked by"). The tenth row of the first 2026-11-14 run, SFLOW-V5-4.3-2, was this child's and became `enforced` in `12945e76f0`.

### Packages it owns

Test packages: DNS, TFTP, DHCP, flow export, MCP, config and YANG, and the rest: `internal/plugins/tftpserver`, `internal/plugins/dhcpserver`, `internal/plugins/geodns`, `internal/plugins/as112`, `internal/plugins/flowexport/...`, `internal/component/config/...`, `internal/component/mcp`, `internal/component/resolve/dns`, `internal/core/dnsserver`, `internal/core/probe`, `cmd/ze/hub`.

| Package | Weak + wrong (2026-09-28) | Of which wrong | Weak, wrong or partial (2026-11-14) |
|---------|--------------|----------------|----------------|
| `internal/plugins/tftpserver` | 12 | 1 | 0 |
| `internal/component/config` | 9 | 0 | 3 (RFC7950-7.6.5-1, 8.3.3-1, 9.1-1) |
| `internal/plugins/flowexport/sflow` | 9 | 0 | 1 (SFLOW-V5-x-11) |
| `internal/component/mcp` | 8 | 2 | 1 (RFC9728-3.3-1) |
| `internal/component/config/yang` | 6 | 2 | 4 (RFC7950-7.6.5-1, 7.9.4-1, 9.1-1, 9.6.4.2-1) |
| `internal/plugins/dhcpserver` | 6 | 1 | 2 (RFC2131-4.1-7, 4.1-15) |
| `internal/plugins/flowexport` | 6 | 0 | 0 |
| `internal/plugins/geodns` | 6 | 1 | 0 |
| `internal/component/resolve/dns` | 4 | 0 | 0 |
| `internal/core/dnsserver` | 4 | 0 | 0 |
| `internal/plugins/flowexport/ipfix` | 4 | 0 | 0 |
| `internal/plugins/flowexport/netflow9` | 4 | 0 | 0 |
| `internal/core/probe` | 3 | 0 | 0 |
| `internal/plugins/as112` | 2 | 0 | 0 |
| `internal/plugins/vrrp` | 2 | 0 | 0 |
| `cmd/ze/hub` | 1 | 0 | 0 |

None of the 2026-11-14 verdicts is `wrong`.

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is DERIVED. No id list is committed here, because the audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak, wrong or partial verdict in this child's scope (stem, id, verdict) | The audit layout changed after this filter was first written: `rfc/audit/` no longer holds one file per RFC with a `.requirements` object, but one file per requirement, `rfc/audit/<stem>/<id>.json`. The filter of 2026-09-28 read the old layout. Its adapted form: take every JSON file two levels below `rfc/audit/`; the stem is the file's directory name and the id its file name without `.json`; `.verdict`, `.tests` and `.units` are top-level keys of the file. Keep a file whose `.verdict` is `weak`, `wrong` or `partial`, whose id is not one of the five cross-group ids, whose stem matches `own` or one of whose `.tests` keys matches `pk`, and whose stem does not match `fo`; print its stem, id and verdict. The 2026-09-28 filter selected weak and wrong only; `partial` is added because `checkAuditFindings` treats it as a finding the same way |
| What the arguments mean | `pk` is `^(cmd/ze/hub/\|internal/component/config/\|internal/component/mcp/\|internal/component/resolve/\|internal/core/dnsserver/\|internal/core/probe/\|internal/plugins/as112/\|internal/plugins/dhcpserver/\|internal/plugins/flowexport/\|internal/plugins/geodns/\|internal/plugins/tftpserver/)` and matches a tagged test path in this child's packages; `own` is `^(rfc792)$` and adds this child's owned stems tagged from another child's packages; `fo` is `^(rfc1071)$` and drops stems another child owns (`^$` matches none); the five ids, RFC1071-1-4, RFC4303-2.1-1, RFC5882-4.4-1, RFC905-x-3 and RFC905-x-4, are the parent's cross-group verdicts |

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 78 weak and 7 wrong, 85 in all, over 21 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). The "Blocked by" verdicts were then moved into acceptance criteria of their blocking specs: `spec-bmp-sflow-export-rfc-defects` AC-3, `spec-mcp-protected-resource-metadata-path` AC-4, and AC-28 of the DNS spec, which now lives in `plan/release/validation/spec-dns-existing-support-proof.md`.

**Re-run, 2026-11-14,** in the per-requirement layout. The first run returned ten rows; after `12945e76f0` moved SFLOW-V5-4.3-2 to `enforced`, nine remain. Running it with `own` widened to every owned stem returns the same rows, because each of them has a tagged test in a `pk` package. Each owner below was read on 2026-11-14 and names the id, except where the row says otherwise:

| Stem | ID | Verdict | Owner |
|------|----|---------|-------|
| rfc2131 | RFC2131-4.1-7 | partial | `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31, since 2026-11-14: the client half, in `internal/plugins/iface/dhcp` (`nclient4Exchanger.acquire` decodes through the vendored `dhcpv4.FromBytes`, which reads `sname` and `file` as NUL-terminated strings and never consults option 52, so the client code is missing, not only a test); the server half is tagged and recorded. `plan/release/validation/spec-rfc-clause-scoped-proof.md` owns neither fix (closure review R1-1) |
| rfc2131 | RFC2131-4.1-15 | partial | the same: `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31, the client half |
| rfc7950 | RFC7950-7.6.5-1 | partial | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-2 |
| rfc7950 | RFC7950-7.9.4-1 | partial | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-1, through its Task table of rows |
| rfc7950 | RFC7950-8.3.3-1 | weak | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-2 |
| rfc7950 | RFC7950-9.1-1 | weak | split by form, not double-owned: `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-2 owns the accepted forms and `plan/post-release/features/spec-config-yang-reference-types.md` AC-9 the unsupported forms, which that spec states: "For already accepted forms, the release child owns correctness and remaining review; a broad RFC7950-9.1-1 claim must not substitute for its specific evidence." |
| rfc7950 | RFC7950-9.6.4.2-1 | weak | `plan/release/fixes/spec-config-yang-loader-structural-checks.md` AC-2 |
| rfc9728 | RFC9728-3.3-1 | weak | `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md` AC-6. The verdict note describes the trailing-slash defect that `d8e084c3eb` (2026-10-18) fixed, so the row owes a test and a re-judge, not a code fix |
| sflow-v5 | SFLOW-V5-x-11 | weak | `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` AC-3 |

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong | Weak, wrong or partial (2026-11-14) |
|------|--------------|----------------|----------------|
| rfc1350 | 6 | 1 | 0 |
| rfc2131 | 4 | 0 | 2 partial |
| rfc2132 | 1 | 1 | 0 |
| rfc2181 | 5 | 0 | 0 |
| rfc2347 | 2 | 0 | 0 |
| rfc2348 | 3 | 0 | 0 |
| rfc3954 | 4 | 0 | 0 |
| rfc4035 | 4 | 0 | 0 |
| rfc4578 | 1 | 0 | 0 |
| rfc7011 | 5 | 0 | 0 |
| rfc7012 | 1 | 0 | 0 |
| rfc7440 | 1 | 0 | 0 |
| rfc7534 | 2 | 0 | 0 |
| rfc7858 | 3 | 0 | 0 |
| rfc7871 | 1 | 1 | 0 |
| rfc792 | 5 | 0 | 0 |
| rfc7950 | 15 | 2 | 5 (2 partial, 3 weak) |
| rfc8414 | 2 | 0 | 0 |
| rfc8484 | 1 | 0 | 0 |
| rfc9728 | 7 | 2 | 1 weak |
| sflow-v5 | 12 | 0 | 1 weak |
| rfc1035 | 0 in `rfc/audit/` | un-enrolled: its six R-7 rows; no audit file | not in `rfc/audit/`; RFC1035-4.1.4-1 re-judged `enforced` 2026-09-30, the other four R-7 rows are AC-28 of `plan/release/validation/spec-dns-existing-support-proof.md` |
| rfc2473 | 0 in `rfc/audit/` | narrowing audit row RFC2473-4.1.1-2; no audit file | not in `rfc/audit/`; RFC2473-4.1.1-2 re-judged `enforced` 2026-09-30 |
| rfc4213 | 0 in `rfc/audit/` | narrowing audit row RFC4213-3.6-1; no audit file | not in `rfc/audit/`; RFC4213-3.6-1 re-judged `enforced` 2026-09-30 |

Stems tagged in this child's packages that another child owns:

| Stem | Owner | Tags here |
|------|-------|-----------|
| rfc1071 | OSPF | RFC1071-1-5 in `core/probe/icmp_test.go`; RFC1071-1-4 is a parent verdict |

Test files that carry tags of two owners. A child edits one only while the other holds no uncommitted change in it:

| File | Shared by |
|------|-----------|
| `internal/core/probe/icmp_test.go` | this child (its package, rfc792) and OSPF, which owns rfc1071 |
| `internal/plugins/vrrp/gateway_icmp_integration_linux_test.go` | VRRP (its package) and this child, which owns rfc792 (RFC792-Echo-5, Echo-6) |

### Un-enrolled R-7 rows

Copied verbatim from the parent. A row leaves this table when its re-judged verdict is `enforced`, recorded here with the date, or in the stem's audit file if the stem is enrolled first.

| ID | Verdict | Unit | What the test fails to prove |
|----|---------|------|------------------------------|
| RFC1035-2.3.4-1 | weak | `TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive` (`internal/plugins/geodns/rfc1035_rr_test.go`) | the lower bound of "positive"; the sibling 4.1.3-1 negative serves TTL 0, and the row does not cite RFC 2181 Section 8 |
| RFC1035-4.1.1-1 | weak | `TestRFC1035_ReservedZFieldIsZero` (`internal/core/dnsserver/rfc1035_header_test.go`) | "in all queries": Z is held clear in responses only; the queries Ze sends (`resolve/dns/resolver.go`, `as112/health.go`) are not checked |
| RFC1035-4.1.4-1 | **enforced, re-judged 2026-09-30** by the services judge (not the author) | `TestRFC1035_LabelLengthOctetBeginsWithTwoZeroBits` (+ a 63-octet label is served with length octet 0x3F; - a 64-octet label, whose length octet would open 01, is refused at `parseConfig`), and `TestRFC1035_CompressionPointersInATruncatedDatagram`, whose walker `pointerTargets` now fails on any length octet opening 01 or 10 (`internal/plugins/geodns/rfc1035_compression_test.go`) | nothing: records in `rfc/discrimination/rfc1035.json` for all four tags (new unit: `dnsserver/handler.go::send` +, `geodns/config.go::checkName` -; old unit: `send` revert, both) |
| RFC1035-4.1.4-5 | weak | `TestRFC1035_InboundCompressionPointerUnderstood` (`rfc1035_compression_test.go`) | the answer does not depend on the pointer being expanded, and no assertion reads its expansion; replies Ze reads as a client are not driven |
| RFC1035-4.2.2-1 | weak | `TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix` (`rfc1035_compression_test.go`) | "use server port 53": the listener runs on a free port and no unit asserts the TCP default |

- 2026-11-14: RFC1035-4.1.4-1 is the only row of this table that is this child's, and it was re-judged `enforced` in `dbdde9fb4f` (2026-09-30). The other four rows are AC-28 of `plan/release/validation/spec-dns-existing-support-proof.md` ("Blocked by").

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC7950-9.1-1 | a type with no canonical form: the value MUST match its lexical representation | §9.1 |
| RFC7950-11-1 | changed semantics MUST get a new definition and identifier; data definition substatements MUST NOT be reordered | §11 |
| RFC8414-3.3-1 | confidentiality protection MUST use TLS with a ciphersuite giving confidentiality and integrity | §6.1 |
| SFLOW-V5-x-7, x-9, x-10, x-12, x-15, x-20, x-24 | claims the document does not state ("wrapping", "2^30-1", "since boot", "[1, 2*N-1]", XDR alignment) or states in a sibling row: row correction | §3.1, §4.3, §5 |
| SFLOW-V5-x-11 | a `sample_pool` field description tagged MUST; no sentence states an obligation: row correction (the code defect is D2 of `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`) | §5 |
| RFC2347-x-2 | the client "must not use those options which were not acknowledged by the server"; RFC2347-x-3 does not carry it either (blind sample BS-A) | Negotiation Protocol |

### Missing rows

None of this table's rows belong to this child.

### Row-quality corrections

None of this table's rows belong to this child.

### Mistagged units

None of this table's rows belong to this child.

### Deferred items the parent's tables missed

| Item | What is owed | Group |
|------|--------------|-------|
| `TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount` (`internal/plugins/geodns/rfc1035_rr_test.go`) | the row RFC1035-4.1.3-1 is `enforced`, but the comment quotes §3.2.1, not the §4.1.3 sentence | services |
| R-7 detail lost in condensing | RFC1035-4.1.4-1: the negative's comment claims "two zero bits" but asserts only "no 0xC0 octet". RFC9190-5.4-3: the claim that crypto/tls turns the error into bad_certificate is not asserted. RFC9190-5.7-1: `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` proves the valid-cache positive but carries only the 5.7-2 and 5.7-6 tags. DRAFT-8210BIS-7-2: the guard is `hdr.Type != pduErrorRpt` in `RTRSession.readLoop`. DRAFT-8210BIS-5.12-1: the Error Code 9 mapping is in `readLoop` | services, IKE/EAP, BGP |

- The row starting `R-7 detail lost in condensing`: this child owns RFC1035-4.1.4-1 only; the other ids go to the child that owns their stem.

### Narrowing audit rows (parent, 2026-09-28)

Copied from the parent's "Narrowing audit findings, 2026-09-28", where the process and the call meanings are. AC-C3 includes these rows.

Verdicts for the rows of un-enrolled stems (rfc2473, rfc4213 hold no audit file, so `audit-stamp` refuses them), recorded by the services judge on 2026-09-30:

| ID | Verdict | Units | Note |
|----|---------|-------|------|
| RFC2473-4.1.1-2 | enforced | `TestRFC2473EncapLimitDecrementedIntoTheOuterHeader` (+), `TestRFC2473EncapLimitFromThePacketNotTheConfiguredLimit` (-, R1(b): configured limit 4 pushes toward the wrong value) in `internal/plugins/iface/netlink/tunnel_rfc_integration_linux_test.go` | row widened to the "set to one less" sentence (`rfc/corrections/rfc2473.md`); both records (revert `buildIp6tnl`) observed red in the QEMU guest; the judge's guest run on the runtime kernel (`./le test qemu run ... kernel tmp/kernel/build/vmlinuz`) passed all four tunnel units, 2026-09-30 |
| RFC4213-3.6-1 | enforced | `TestRFC4213SitDecapsulatesFromTheConfiguredRemote` (+), `TestRFC4213SitRefusesAnUnverifiedSource` (-: foreign source not decapsulated, positive control, remote "" and 0.0.0.0 refused with no link left) | row widened to the two following sentences, "transport protocols" and "as configured on the decapsulator" (`rfc/corrections/rfc4213.md`); D-8 fixed: `parseTunnelRemote` refused an unspecified remote in the netlink backend (`dbdde9fb4f`); since `04580da85d` config verify refuses it (`TunnelSpec.CheckRemote`, called by `parseTunnelEntry`) for every tunnel kind and either backend, and `parseTunnelRemote` calls the same check again; records revert `buildSittun` (+) and `parseTunnelRemote` (-), guest, the negative re-taken in `04580da85d` with its red observed |

| ID | Call | Obligation | Section |
|----|------|------------|---------|
| RFC1350-2-3 | moved | stated by RFC 1123 4.2.3.1, not by this document. Sorcerer's Apprentice fix (do not resend on duplicate ACK) is RFC 1123; RFC 1350 only cites it. rfc1123.txt not in rfc/full. | RFC 1123 4.2.3.1 |
| RFC2131-4.1-7 | dropped | "The 'file' field MUST be interpreted next (if the 'option overload' option indicates that the 'file' field contains DHCP options), followed by the 'sname' field." | §4.1 |
| RFC2181-10.3-1 | dropped | "It can also have other RRs, but never a CNAME RR." | §10.3 |
| RFC2473-4.1.1-2 | dropped | "The limit value in the encapsulating option is set to one less than the limit value found in the packet being encapsulated." | §4.1.1 |
| RFC4035-2.2-1 | dropped | "o The RRSIG Algorithm, Signer's Name, and Key Tag fields identify a zone key DNSKEY record at the zone apex." | §2.2 |
| RFC4035-3.1.1-4 | dropped | "If space does not permit inclusion of the DS or NSEC RRset and associated RRSIG RRs, the name server MUST set the TC bit (see Section 3.1.1)." | §3.1.4 |
| RFC4035-4.3-1 | dropped | "More precisely, a security-aware resolver must be able to distinguish between four cases:" | §4.3 |
| RFC4213-3.6-1 | dropped | "This is done by verifying that the source address is the IPv4 address of the encapsulator, as configured on the decapsulator." | §3.6 |
| RFC7011-8-3 | dropped | "Template Withdrawals (Section 8.1) MUST NOT be sent by Exporting Processes exporting via UDP and MUST be ignored by Collecting Processes collecting via UDP." | §8.4 |
| RFC7011-x-1 | retire | RFC never mandates short form for 0-254; Section 7 says length may also use 3 octets. Quote is descriptive, carries no obligation. | none |
| RFC7871-7.1.2-3 | dropped | "If an Intermediate Nameserver receives a query with SOURCE PREFIX-LENGTH set to 0, it MUST NOT include client address information in queries made to resolve that client's request (see Section 7.1.2)." | §7.5 |
| RFC7950-5.1-1 | dropped | "A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module." | §7.2.2 |
| RFC7950-7.21.5-1 | dropped | "If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions." | §7.21.5 |
| RFC7950-7.3-1 | dropped | "The "type" statement, which MUST be present, defines the base type from which this type is derived." | §7.3.2 |
| RFC7950-7.9.2-1 | dropped | "The case identifier MUST be unique within a choice." | §7.9.2 |
| RFC7950-9.2.4-1 | dropped | "If a length restriction is applied to a type that is already length-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit length values or ranges, or splitting ranges into multiple ranges with intermediate gaps." | §9.4.4 |
| SFLOW-V5-x-15 | retire | No spec sentence says counters are cumulative since boot; quote is descriptive text about lost counter samples. | none |

### Blocked by

Each id below was re-checked on 2026-11-14: a weak or partial verdict in `rfc/audit/<stem>/<id>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix or the owed test (AC-C4), and the verdict sits in its acceptance criteria (P-3).

| ID | Verdict (2026-11-14) | Blocking spec |
|----|---------|---------------|
| SFLOW-V5-x-11 | weak | `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` AC-3 (D2, the code; the row correction in the split table stays here) |
| RFC7950-9.6.4.2-1 | weak | `plan/release/fixes/spec-config-yang-loader-structural-checks.md` AC-2 |
| RFC7950-7.6.5-1 | partial | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-2 (OWNER RULING 4, 2026-09-30; tagged, so no `{gap}`) |
| RFC7950-7.9.4-1 | partial | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-1, through its Task table of rows |
| RFC7950-8.3.3-1 | weak | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-2 (OWNER RULING 4, 2026-09-30; tagged, so no `{gap}`) |
| RFC7950-9.1-1 | weak | split by form: `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-2 (the accepted forms) and `plan/post-release/features/spec-config-yang-reference-types.md` AC-9 (the unsupported forms); the second states the split: "For already accepted forms, the release child owns correctness and remaining review; a broad RFC7950-9.1-1 claim must not substitute for its specific evidence." |
| RFC7950-8.3.3-2 | `unimplemented` (`{gap}` row split from 8.3.3-1) | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-1 Task table (OWNER RULING 4, 2026-09-30) |
| RFC9728-3.3-1 | weak | `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md` AC-6 (a test and a re-judge are owed; the producer was fixed in `d8e084c3eb`) |
| RFC2131-4.1-7 | partial | `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 (the client half; the server half is tagged), since 2026-11-14 |
| RFC2131-4.1-15 | partial | `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 (the client half; the server half is tagged), since 2026-11-14 |
| RFC1035-2.3.4-1 | weak (R-7) | `plan/release/validation/spec-dns-existing-support-proof.md` AC-28 |
| RFC1035-4.1.1-1 | weak (R-7) | `plan/release/validation/spec-dns-existing-support-proof.md` AC-28 |
| RFC1035-4.1.4-5 | weak (R-7) | `plan/release/validation/spec-dns-existing-support-proof.md` AC-28 |
| RFC1035-4.2.2-1 | weak (R-7) | `plan/release/validation/spec-dns-existing-support-proof.md` AC-28 |

- Left the table since 2026-09-28, each read `enforced` in `rfc/audit/` on 2026-11-14: RFC7950-7.19-1 (listed then as `wrong`), RFC7950-9.4.4-1, RFC7950-8.1-2, RFC7950-8.3-1, RFC7950-7.6.5-2, RFC7950-7.6.5-3, RFC9728-3.1-3 and RFC9728-3-2.
- The four RFC1035 rows were listed under `spec-fixit-dns-rfc1035-conformance` (now `plan/proposals/spec-fixit-dns-rfc1035-conformance.md`); their AC-28 lives in `plan/release/validation/spec-dns-existing-support-proof.md`. RFC1035-4.1.4-1 is not among them and is this child's.

## Required Reading

### Architecture Docs
- [ ] `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
  → Constraint: every row stays a verbatim span of its cited section, at least 24 characters, never across two sections (`checkRowQuotes`); a span that must cross a section is split into two rows
  → Constraint: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; a new id takes n above the HEAD^ high-water mark of its section (`checkIDAllocation`)
  → Constraint: a new gated row lands with both polarity tags or an annotation, a discrimination record for each new cover (a moved tag is a new cover), and an extraction site mapped to it or listed in `unsourced-ids`
  → Constraint: an upgrade from weak or wrong to `enforced` with unchanged units needs an `upgrade_reason` (`checkAuditFindings`), so author and judge land in one commit where they can
  → Constraint: editing any function in a test file shifts every verdict tagging that file; `./le rfc reseal` is corpus-wide and rewrites other children's audit files, so a reseal runs only when no other child holds uncommitted audit changes
  → Constraint: rfc1035 is not enrolled, so `audit-stamp` refuses it: its R-7 re-judgements are recorded in this spec's R-7 table with the date
  → Constraint: RFC792-Echo-5 and Echo-6 are this child's (it owns rfc792), and their unit sits in a VRRP `_linux_test.go`: `discriminate-record` needs the QEMU guest and `kernel <vmlinuz>`, and the file is edited only while VRRP holds no uncommitted change there
  → Constraint: RFC1071-1-4 tags `core/probe/icmp_test.go` and an OSPF file; it stays in the parent (AC-9)
  → Constraint: the SFLOW-V5 row corrections: each claim no sentence states is a D-2 retirement or a dated correction, never a new row
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions, the verdict vocabulary, STRICTNESS, and the re-judge route
  → Decision: judge strictly: a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each owned stem above - the row text a verdict is judged against; the RFC text in `rfc/full/<stem>.txt` is the authority for every split and correction
  → Constraint: a claim that an obligation was dropped quotes the RFC sentence with its section (parent R-9)

**Key insights:**
- 85 verdicts over 16 packages and 22 stems on 2026-09-28, plus 13 rows copied from the parent's tables; 9 weak or partial verdicts over 4 stems on 2026-11-14, each owned by another spec
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc7950.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/sflow-v5.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc9728.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `internal/le/rfc/check_audit.go` - `checkAuditFindings` refuses a deleted weak or wrong verdict and an unchanged-units upgrade without `upgrade_reason`

**Behavior to preserve:**
- a commit of this child adds no `./le rfc check` violation of its own; a weak or wrong verdict is never deleted
- product behavior of every package in scope, except a D-8 fix of a verified defect, which carries its own failing-first test

**Behavior to change:**
- the tagged tests, rows, tags and verdicts named in this spec's tables and in the derived listing; D-8 producer fixes where a test exposes a verified defect

## Existing Mechanisms (MANDATORY)

Each row was read at its producer on 2026-11-14; the section records the mechanisms this spec already uses and adds no design decision.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Re-judge route of `./le rfc audit-stamp` | `internal/le/rfc/audit_stamp.go`: `stampRefusal`, `rejudgeRefusal`, `stampFingerprints` | the default mode refuses a row that already has a verdict and `mode rejudge` refuses a row that has none; a re-judged verdict replaces the recorded one with fresh fingerprints, and a weak, wrong or partial verdict raised to `enforced` over byte-identical units is refused unless it carries an `upgrade_reason` | uses it: P-2, "a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`", and Data Flow, Transformation Path step 5 |
| Discrimination records | `internal/le/rfc/discriminate_action.go`: `discriminateRecordAnswer`, `recordDiscrimination`, `mergeDiscrimination`; `rfc/discrimination/<stem>/<id>.json` | `./le rfc record` checks the tagged unit, observes the run under the named break, seals the record and writes it into that requirement's own file; a break that leaves the unit green is refused | uses it: Data Flow, Transformation Path step 3, "`./le rfc record` records the observed red under a break for each added or changed cover", and Method, "a discrimination record for every tag added or changed" |
| Audit findings ratchet | `internal/le/rfc/check_audit.go`: `checkAuditFindings`, `upgradeOverUnchangedUnits`, `upgradeReasonGiven` | against the baseline, refuses a deleted weak, wrong or partial verdict and an upgrade to `enforced` over unchanged units with no `upgrade_reason` | uses it: Current Behavior, "`checkAuditFindings` refuses a deleted weak or wrong verdict and an unchanged-units upgrade without `upgrade_reason`", and the Wiring Test row |
| Dated corrections | `internal/le/rfc/check_ratchets.go`: `checkRetiredRequirements`, `retiredIDs`, `parseCorrections`; `rfc/corrections/<stem>.md` | a row id that leaves `rfc/short/<stem>.md` is refused unless a `Retired <YYYY-MM-DD>:` paragraph of its own stem's corrections file names it; `retiredIDs` counts an id only from its own stem's file | uses it: Owned stems, "this child alone writes ... `rfc/corrections/<stem>.md`"; AC-C3, "a dated correction says why not"; Required Reading, "each claim no sentence states is a D-2 retirement or a dated correction, never a new row" |
| Extraction sign-off | `internal/le/rfc/check_extraction.go`: `checkExtractionRatchet`, `checkExtractionRatchetAgainst`; `rfc/extraction/<stem>.json` | refuses a stem whose sign-off existed at HEAD^ and is gone, and a rise in exclusions with no `resign-reason` | uses it: Required Reading, "an extraction site mapped to it or listed in `unsourced-ids`", and Files to Modify, "`rfc/extraction/<stem>.json` for the owned stems only" |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the `RFC requirement:` tags in test files
- `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, typed by a judging agent

### Transformation Path
1. The author agent records `./le rfc approve unit` (P-1), then changes a tagged test, a row, a tag, or a producer (D-8)
2. `./le rfc reseal` re-stamps sibling verdicts the edit only shifted
3. `./le rfc record` records the observed red under a break for each added or changed cover
4. A judging agent that did not author the test writes the verdict in a pending file under session scratch
5. `./le rfc audit-stamp ... mode rejudge` replaces the verdict in place with fresh fingerprints
6. `./le rfc check` compares verdicts, records and rows against HEAD^

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test file to ledger | the `RFC requirement:` tag, fingerprinted by `stampFingerprints` | `./le rfc check` reads the re-judged verdict fresh |
| pending file to audit file | `audit-stamp` `mode rejudge` | the parent's `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` |

### Integration Points
- the parent's `mode rejudge` of `./le rfc audit-stamp` (P-2)
- the blocking specs' acceptance criteria, which receive the blocked verdicts (P-3)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every verdict enters through `audit-stamp`, every record through `discriminate-record` |
| No duplicated functionality | Yes | the listing is the parent's filter restricted to this scope; no id list is committed |
| Registration over hardcoding | N-A | no feature is added; this child changes tests, rows and D-8 producers |

## Method

The parent's Method table governs every phase and is not copied here. The rules that bite this group are the constraints under Required Reading above, plus: one agent per package and never two on one test file; strict judgement; every clause in both polarities; a discrimination record for every tag added or changed; an independent judge for every re-judged verdict; a D-8 defect fixed failing-test-first, with interop where a peer exists.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | `./le rfc audit-stamp` help names `mode` | confirmed: `mode rejudge` landed in `2d078935e3` (2026-09-28), before this child's first re-judge; `stampRefusal` (`internal/le/rfc/audit_stamp.go`, read 2026-11-14) refuses in the default mode a row that already has a verdict, so the in-place replacements in `519d9f4f2a` (eleven weak verdicts to `enforced`, its message says "mode rejudge") and `04c6bd7371` (RFC7950-7.6.1-1, 7.6.5-1 and 8.3.3-1 replaced in place) went through `mode rejudge` |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | broken, 2026-11-14: the listing holds RFC7950-7.9.4-1, RFC2131-4.1-7 and RFC2131-4.1-15, which the 2026-09-28 list lacked; each is now listed under "Blocked by" with the spec that owns it and named in that spec's acceptance criteria (both RFC2131 ids in `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a test fix exposes a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |
| R-2 | another child or spec commits while this child holds uncommitted hunks in a shared test file, or runs a reseal | a commit diff shows another owner's ids | the shared-file rule above; reseal and interop `revert` runs are serialized between children |
| R-C1 | the SFLOW-V5 row corrections retire more than 15% of a stem of 20 or more rows | the per-stem retirement count | stop and report to the owner (D-7) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | test-only changes break nothing an operator sees; a D-8 fix changes protocol behavior and carries its own tests and interop |
| How is it reverted? | one commit per package phase |
| Who else touches this path? | the parent, the children named in the shared-file table, the blocking specs, `spec-rfc-evidence-strength-1` and `-2` (the same discrimination files) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | `checkAuditFindings` over this child's stems | the derived listing above holds only the "Blocked by" ids, and `TestCheckAuditRatchetSeesTipCommit` (`internal/le/rfc/check_audit_baseline_test.go`) stays green |

## Acceptance Criteria

Inherited from the parent (AC-C1 to AC-C7), restated for the services group.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-C1 | the derived listing above | holds only the "Blocked by" ids, each named in its blocking spec's acceptance criteria |
| AC-C2 | every verdict this child moves to `enforced` | its tagged units carry a discrimination record, and an agent other than the test's author judged it |
| AC-C3 | every row of this child in the split-needed and missing-rows tables, including the rows the parent's narrowing audit adds for this group (the "Narrowing audit rows (parent, 2026-09-28)" table) | the dropped obligation is a row of its own, or a dated correction says why not, and the new row carries a verdict |
| AC-C4 | every spec in the parent's code-defect list | this child declares none of their defects and does not fix them; a test whose producer one of them changes waits for, or lands with, that spec |
| AC-C5 | `./le rfc check` after each commit of this child | no violation the commit added, and no stale verdict in this child's stems |
| AC-C6 | every row and unit of this child in the row-quality and mistagged-unit tables | corrected, merged or re-tagged, or a dated correction says why it stands; a changed row or tag is re-judged |
| AC-C7 | every row of this child in "Un-enrolled R-7 rows" that no spec blocks | resolved as the parent's Goal states and re-judged `enforced` by an agent that did not write the test, recorded in that table with the date or in the stem's audit file |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| every unit tagged by a listed verdict | `internal/plugins/tftpserver/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/config/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/flowexport/sflow/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/mcp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/config/yang/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/dhcpserver/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/flowexport/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/geodns/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/resolve/dns/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/dnsserver/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/flowexport/ipfix/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/flowexport/netflow9/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/probe/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/as112/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/vrrp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `cmd/ze/hub/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| each numeric field a quoted sentence bounds | the RFC's range | the RFC's last valid value | tested where the sentence states a lower bound | tested where the sentence states an upper bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| an existing tagged `.ci` test in scope, changed only where its verdict demands | `test/` | the operator-visible behavior the quoted sentence names | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| named when a D-8 fix is found | `test/interop/scenarios/` | interop for any D-8 fix where a peer exists (a DNS resolver, a TFTP client, a DHCP client, a flow collector); none is planned before a defect is found | the fixed behavior against another implementation | |
| `TestTFTPCurlDuplicateACKInterop` | `internal/plugins/tftpserver/tftp_curl_docker_test.go` (build tags `docker` and `linux`, added in `93c8f3d9c7`) | curl, as an independent TFTP client in a container | the D-8 fix of `519d9f4f2a`: a relay duplicates curl's ACK 1 while DATA 2 is outstanding, and Ze answers with silence, then timeout recovery, then curl's exact 1500-byte download | observed 2026-11-14 (Goal Validation): green on the real build, red under the mutation its comment names, green again; recorded on `docs/architecture/testing/interop.md` in `62f22dc4a5` |
| none | none | none: the unspecified-remote refusal (`dbdde9fb4f`, moved to config verify in `04580da85d`) refuses a config value and has no wire peer | | N-A |

## Files to Modify

- the `_test.go` and `.ci` files of the packages above that carry the listed tags
- `rfc/short/<stem>.md`, `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/corrections/<stem>.md`, `rfc/extraction/<stem>.json` for the owned stems only
- a producer under the packages above when a test exposes a verified defect (D-8), named in the phase that finds it
- not `docs/features/rfc-status.md`: it is derived by `./le rfc update` from `rfc/short/` and the verified records, git-ignored (`.gitignore`), never hand-edited and never committed; a changed support claim is made in `rfc/short/<stem>.md`
- the acceptance criteria of each blocking spec, which receive its blocked verdicts (P-3)

## Files to Create
- none planned; a split or missing row is a new row in an existing `rfc/short/<stem>.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| YANG validation constraints | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| YANG custom validators | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| CLI commands/flags | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| CLI grammar (keyword before value) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Editor autocomplete | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Functional test for new RPC/API | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Pipe completeness | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Env var registration | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Doctor check for runtime dependencies | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Prometheus counters/metrics | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| BGP family surface (new SAFI / capability / attribute) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | this child changes tests, rows and verdicts, not this surface |
| 2 | Config syntax changed? | No | this child changes tests, rows and verdicts, not this surface |
| 3 | CLI command added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 4 | API/RPC added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 5 | Plugin added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 6 | Has a user guide page? | No | this child changes tests, rows and verdicts, not this surface |
| 7 | Wire format changed? | No | tests and rows only; a D-8 fix that changes the wire names `docs/architecture/wire/*.md` in its phase |
| 8 | Plugin SDK/protocol changed? | No | this child changes tests, rows and verdicts, not this surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` for each owned stem a phase changes, with source anchors; a new `wrong` verdict discloses non-support there first. `docs/features/rfc-status.md` is derived from those files by `./le rfc update` and git-ignored, so it is regenerated, never edited or committed |
| 10 | Test infrastructure changed? | No | this child changes tests, rows and verdicts, not this surface |
| 11 | Affects daemon comparison? | No | this child changes tests, rows and verdicts, not this surface |
| 12 | Internal architecture changed? | No | this child changes tests, rows and verdicts, not this surface |
| 13 | Route metadata keys added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 14 | Prometheus counters added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | this child changes tests, rows and verdicts, not this surface |
| 16 | Any changed source file referenced by existing doc source anchors? | No | only test and ledger files change; a D-8 producer fix runs `./le spec citation anchors` in its phase |
| 17 | Existing docs show config/CLI/API examples for this area? | No | this child changes tests, rows and verdicts, not this surface |

## Implementation Steps

**Procedure P, for every package phase.** One author agent per package, never two on one test file. It reads the listing restricted to the package, records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before each tagged-unit edit (P-1), makes each clause of the quoted sentence asserted in both polarities or corrects the row, and fixes a verified defect under D-8. It runs the scoped package test under `./le job run`, then `./le rfc record` for each added or changed cover. An independent judge that wrote none of the tests writes the verdicts in a pending file under session scratch and stamps them with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. `./le rfc reseal` runs when the edit shifted sibling verdicts, serialized between children. Each package lands in one commit through `./le commit create` with the `RFC-approved:` trailer, author and judge together where they can. Verify: the listing filtered to the package holds only blocked ids, and `./le rfc check` adds no violation.

1. **Phase: Wiring** - confirm the parent's `mode rejudge` has landed (A-1), reproduce the derived listing and its counts, and move each "Blocked by" verdict into its blocking spec's acceptance criteria (P-3)
   - Tests: the Wiring Test row
   - Files: the blocking specs' Acceptance Criteria tables
   - Verify: the listing count equals the Task figure, or the difference is explained by commits since 2026-09-28
2. **Phase: `internal/plugins/tftpserver`** - 12 verdicts, 1 wrong; procedure P
3. **Phase: `internal/component/config`** - 9 verdicts, 0 wrong; procedure P
4. **Phase: `internal/plugins/flowexport/sflow`** - 9 verdicts, 0 wrong; procedure P
5. **Phase: `internal/component/mcp`** - 8 verdicts, 2 wrong; procedure P
6. **Phase: `internal/component/config/yang`** - 6 verdicts, 2 wrong; procedure P
7. **Phase: `internal/plugins/dhcpserver`** - 6 verdicts, 1 wrong; procedure P
8. **Phase: `internal/plugins/flowexport`** - 6 verdicts, 0 wrong; procedure P
9. **Phase: `internal/plugins/geodns`** - 6 verdicts, 1 wrong; procedure P
10. **Phase: `internal/component/resolve/dns`** - 4 verdicts, 0 wrong; procedure P
11. **Phase: `internal/core/dnsserver`** - 4 verdicts, 0 wrong; procedure P
12. **Phase: `internal/plugins/flowexport/ipfix`** - 4 verdicts, 0 wrong; procedure P
13. **Phase: `internal/plugins/flowexport/netflow9`** - 4 verdicts, 0 wrong; procedure P
14. **Phase: `internal/core/probe`** - 3 verdicts, 0 wrong; procedure P
15. **Phase: `internal/plugins/as112`** - 2 verdicts, 0 wrong; procedure P
16. **Phase: `internal/plugins/vrrp`** - 2 verdicts, 0 wrong; procedure P
17. **Phase: `cmd/ze/hub`** - 1 verdicts, 0 wrong; procedure P
18. **Phase: split and missing rows** - after the parent commits its narrowing output for this group (parent R-11): each row of the split-needed and missing-rows tables becomes a row of its own with both polarity tags, a record and a verdict, or a dated correction says why not (AC-C3)
   - Verify: `./le rfc check` accepts the new ids, extraction sites and records
19. **Phase: row quality and mistagged units** - each row of those tables corrected, merged or re-tagged, or a dated correction says why it stands; each changed row or tag re-judged (AC-C6)
   - Verify: `./le rfc check` reads no stale verdict in the owned stems
20. **Phase: un-enrolled R-7 rows** - each row not under "Blocked by" resolved and re-judged by an agent that did not write the test, recorded in the R-7 table above with the date (AC-C7)
21. **Phase: closure check** - the derived listing holds only "Blocked by" ids (AC-C1), each named in its blocking spec's acceptance criteria

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-C row demonstrated; the listing holds only blocked ids |
| Correctness | every `enforced` verdict names an identifier in its unit, has both polarities or `{single-polarity}`, and a judge other than the author |
| Rule: no weakened test | no assertion removed or loosened to reach `enforced` (`ai/rules/completion.md`) |
| Rule: one writer per ledger file | no commit touches a stem another child owns |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the listing reduced to blocked ids | the filter under "Derived listing" |
| a record for every changed cover | `./le rfc check` adds no missing-record violation |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a negative test that feeds malformed input asserts the refusal, not only the absence of a crash |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | re-read the RFC sentence; a verified defect is a D-8 fix in this child unless a blocking spec owns it |
| A verdict cannot move without another spec's producer fix | add it to "Blocked by" and to that spec's acceptance criteria (P-3) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the owner |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Scope by package, with each stem's ledger at one owner | scope by stem | the parent's split: test files are per package, ledger files per stem |
| A verdict of an owned stem whose units sit only in another child's package belongs to the stem owner | give it to the package owner | the ledger file has one writer; the shared test file is edited one child at a time |

## Known Limitations

- verdicts under "Blocked by" leave this child through P-3 and close with their blocking spec
- the parent's cross-group verdicts and RFC7296-2.4-2 are not in this child's scope

## RFC Documentation (Scope: protocol)

A D-8 fix adds `// RFC NNNN Section X.Y: "<quoted requirement>"` above the enforcing code, per `ai/rules/rfc-compliance.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-C1..AC-C7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped evidence passes: the unit packages and the interop scenarios this spec reaches, each named in Goal Validation; whole-tree verification is GitHub CI's on the pushed commit (`ai/META-DECISIONS.md`, Ze is pre-release)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for the D-8 protocol fixes (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** tests + rows + verdicts + records + D-8 code + edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-rfc-verdict-fix-services.md` only, in the same `./le commit create` script

## Progress, 2026-11-14

Written from git history and the `rfc/` ledger, each claim read at HEAD on 2026-11-14. The ten commits made in this session, `12945e76f0`, `335e80c658`, `62f22dc4a5`, `8b6e37aaaa`, `2587658ad3`, `2cbf113ecc`, `39205d383c`, `cba4bb0810`, `04580da85d` and `8f64f14196`, each carry the trailer `Spec-work: rfc-verdict-fix-services` (read with `git log -1 --format=%b`); the five earlier commits, `519d9f4f2a`, `9e8443e287`, `dbdde9fb4f`, `04c6bd7371` and `a640d04015`, carry none.

## Implementation Summary

### What Was Implemented
| Commit | Date | What it carries |
|--------|------|-----------------|
| `519d9f4f2a` | 2026-09-28 | TFTP D-8 fix (below); two transfer-level test files (`rfc1350_transfer_test.go`, `rfc2347_2348_transfer_test.go`) with records; eleven verdicts re-judged weak to `enforced` in `mode rejudge` (RFC1350-2-2, 4-3, 5-3, 6-1, 7-1; RFC2347-x-1, x-3; RFC2348-x-3, x-4, x-5; RFC7440-3-1); first verdict RFC1350-2-3 `enforced`; `docs/architecture/provisioning/tftp-server.md` |
| `9e8443e287` | 2026-09-30 | dns, icmp and tls verdicts: rfc792 Echo-1, 2, 3, 5, 6; rfc2181 4.1-1, 5.4-2, 8-1, 10.3-1, 11-1; rfc4035 3-6, 4.1-1, 4.1-4, 4.6-3; rfc7534 3.5-1, 3.5-2; rfc7858 3.1-1, 3.4-5, 8-1; RFC8484-6-2 and RFC9728-7.1-2, each weak to `enforced`; RFC7871-7.2.2-1, 7.2.2-2 and RFC1350-5-2 retired; geodns D-8 fix (below) |
| `dbdde9fb4f` | 2026-09-30 | RFC2131-2-3 weak to `enforced`; RFC7011-8-3 new and `enforced`; `not-applicable` then for RFC2347-x-4, RFC4035-2.2-1, 3.1.1-4, 4.3-1 and RFC7871-7.1.2-3 (read in `rfc/audit/` on 2026-11-14: RFC4035-2.2-1, 3.1.1-4, 4.3-1 and RFC7871-7.1.2-3 now read `unimplemented`, and RFC2347-x-4 reads `enforced`); `{gap}` rows RFC7950-11-3, 9.1-2, 7.2.2-1, 7.21.5-2 stamped `unimplemented`; RFC1035-4.1.4-1, RFC2473-4.1.1-2 and RFC4213-3.6-1 re-judged `enforced` (recorded in this spec); RFC2347-x-2 merged into x-4 and RFC7011-x-1 retired into the new row RFC7011-7-2, with dated corrections; sit tunnel D-8 fix (below) |
| `04c6bd7371` | 2026-09-30 | rfc7950 under owner ruling 4: Meta Implementation `mixed`, naming openconfig/goyang; 7.6.5-1 split into -1, -2, -3 and 8.3.3-1 into -1, -2, the new rows `{gap}` and `unimplemented`; RFC7950-7.6.1-1 weak to `enforced`; 7.6.5-1 and 8.3.3-1 re-judged weak and moved to `spec-config-yang-when-unique-choice` AC-2 |
| `a640d04015` | 2026-10-02 | AC-C2 record sweep: 59 observed-red records, all on the revert route, for the tagged units of the 77 ids this child moved to `enforced`; the judge re-took RFC8414-2-1 negative on `as_metadata.go::fetchASMetadata`; no verdict changed (`plan/handover/rfc-verdict-test-fix-pass/services/ac-c2-sweep.md`) |
| `12945e76f0` | 2026-11-14 | SFLOW-V5-4.3-2 weak to `enforced`: `TestSFlowV5AgentAddressMultiHomedEngineWarns` and `TestSFlowV5AgentAddressOnLoopbackEngineSilent` (`internal/plugins/flowexport/sflow_v5_agent_advice_test.go`) drive `runEngine` through the SDK configure callback, with an interface backend registered through the production registry. With the agent address on routed eth0 of a multi-homed router the config applies and exactly one WARN names the address, the interface and the dummy-interface fix; with it on lo or on dummy agent0 nothing is logged at WARN or above. Both records by `./le rfc record` on hand-written breaks, both reds observed: positive, the Warn loop's log call in `runEngine`'s configure closure replaced by a discard; negative, `adviseSFlowCollectors` classifying against an empty interface list. An independent judge approved both units and re-judged the verdict |
| `335e80c658` | 2026-11-14 | AC-C3 ledger items: RFC 2181 Section 10.3 "never a CNAME RR" gets no new row, since it restates RFC2181-10.3-1, which `TestRFC2181NSTargetIsNotAnAlias` enforces (dated correction in the new `rfc/corrections/rfc2181.md`, reason in `rfc/extraction/rfc2181.json`); SFLOW-V5-x-15 retired (its sentence obliges no agent; the cumulative-counter meaning is RFC 2233's, which is not enrolled), its row, verdict and record removed and its tag taken off `TestInterfaceCountersFromCumulative`, which keeps every assertion, with an independent test judge's approval and the `RFC-approved:` trailer; SFLOW-V5-x-11 left verbatim on purpose, a correction saying why (demoting a format definition needs the owner's call); RFC1350-2-3 gains its dated correction (the old claim was RFC 1123 Section 4.2.3.1's; the row quotes RFC 1350 Section 5); the TFTP upload obligation returns as the gap RFC1350-5-5, a fresh ordinal because the retired id RFC1350-5-2 stays reserved, and extraction site 5:2 maps to it |
| `62f22dc4a5` | 2026-11-14 | `docs/architecture/testing/interop.md`: the green and red runs of `TestTFTPCurlDuplicateACKInterop`, and the route that runs a docker-tagged Linux package test from a Mac through the docker-lab guest |
| `8b6e37aaaa` | 2026-11-14 | two tests of `internal/plugins/flowexport/sflow_v5_register_test.go` tagged SFLOW-V5-x-16 (`TestInterfaceCountersFromUnknownSpeed` positive, `TestInterfaceCountersFromExtended` negative), each with a record on a hand-written break whose red was observed, approved by an independent test judge (two `RFC-approved:` lines); x-16 re-judged, still `enforced`, over five tests |
| `2587658ad3` | 2026-11-14 | the four RFC9728-7.1-2 records in `rfc/discrimination/rfc9728/RFC9728-7.1-2.json` re-taken after `537b3a87f9` changed `loadMCPTLSConfig`, each red observed |
| `2cbf113ecc` | 2026-11-14 | a lint annotation on the test log handler's by-value record in `sflow_v5_agent_advice_test.go`; no assertion changed |
| `39205d383c` | 2026-11-14 | reseal of the SFLOW-V5-4.3-2 verdict, whose file fingerprint `2cbf113ecc` had shifted while its units stayed byte-identical |
| `cba4bb0810` | 2026-11-14 | ledger prose: the RFC 1350 enrolment reason and the extraction reasons for RFC1350-2-2, 6-1 and 7-1 name `serveFile` and `windowSender.awaitACK` in place of `sendAndWaitACK`, and two reasons no longer claim a single-polarity annotation the rows do not carry; the SFLOW-V5-x-11 correction states decision D-3 rather than leaving a question open (closure review R1-6, R1-N3) |
| `04580da85d` | 2026-11-14 | the unspecified tunnel remote refused at config verify (D-8, below; closure review R1-3); the RFC4213-3.6-1 negative record re-taken |
| `8f64f14196` | 2026-11-14 | test maintenance, not a product fix: a stale DUID refusal assertion in `internal/component/iface/config_test.go`, the package `04580da85d` changed, replaced by an assertion on the typed refusal instead of a word in its text |

### Bugs Found/Fixed (D-8)
| Commit | Fix |
|--------|-----|
| `519d9f4f2a` | RFC 1350 Section 5: `sendAndWaitACK` retransmitted the outstanding DATA on any non-matching packet, so a duplicate ACK drew a resend (the Sorcerer's Apprentice bug); it now retransmits only on the ACK timeout. `TestRFC1350DuplicateACKTriggersNoDATA` fails against the previous `handler.go`. Since `c120b09f02` (2026-10-31, RFC 7440 windowsize as the sender) the DATA path waits in `windowSender.awaitACK` (`internal/plugins/tftpserver/window_send.go`), which keeps the same rule |
| `9e8443e287` | RFC 2181 Section 11: geodns `checkName` accepted an empty label; it now refuses one |
| `dbdde9fb4f` | `parseTunnelRemote` (`internal/plugins/iface/netlink/tunnel_linux.go`) accepted an unspecified remote (0.0.0.0, ::), which installs a wildcard sit tunnel that decapsulates from any source; it then refused one, but only when the netlink backend applied it; `docs/features/interfaces.md` updated |
| `04580da85d` | the refusal of `dbdde9fb4f` sat in one backend: `ze config validate`, a commit and the VPP backend accepted an unspecified remote, and the message named no fix (closure review R1-3). `TunnelSpec.CheckRemote` (`internal/component/iface/tunnel.go`), called by `parseTunnelEntry` (`internal/component/iface/config.go`), now refuses it at config verify for every tunnel kind and either backend, with an operator message naming the fix; `parseTunnelRemote` and the VPP backend's tunnel path call the same check again; `docs/features/interfaces.md` updated |
| none, `12945e76f0` | SFLOW-V5-4.3-2 found no product defect: warning without refusing is the faithful reading of the Section 4.3 SHOULD; the commit adds tests and records only |

### Documentation Updates
- `docs/architecture/provisioning/tftp-server.md` (`519d9f4f2a`), `docs/architecture/dns/geodns.md` and `docs/architecture/dns/secure-transports.md` (`9e8443e287`), `docs/features/interfaces.md` (`dbdde9fb4f`, and `04580da85d` for the refusal at config verify), `docs/architecture/testing/interop.md` (`62f22dc4a5`).
- Not updated: `docs/architecture/flowexport/flow-export-1-counter-export.md`, section "Daemon agent-address warning tests", still says the classifier unit tests are the only coverage of this warning below the daemon, which `12945e76f0` makes false (Work Not Done).
- `rfc/short/<stem>.md` rows and `rfc/corrections/` for the owned stems; `docs/features/rfc-status.md` is derived from them by `./le rfc update` and git-ignored.
- At closure, 2026-11-14: `plan/learned/050-rfc-verdict-fix-services.md` and the news fragment `website/changes/fragments/rfc-verdict-fix-services.md` (the TFTP duplicate-ACK fix and the unspecified tunnel remote refusal, the two changes an operator sees); the citers of this spec's path repointed to the learned entry where their spec-edit hook allowed it (Pre-Commit Verification).

### Deviations from Plan
- A-2 was broken: three verdicts the 2026-09-28 "Blocked by" list lacked (RFC7950-7.9.4-1, RFC2131-4.1-7, RFC2131-4.1-15) were found by the 2026-11-14 re-run and routed to their owners.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| wrong assumption | A-2 held the 2026-09-28 "Blocked by" list complete for this scope | the 2026-11-14 re-run of the derived listing held RFC7950-7.9.4-1, RFC2131-4.1-7 and RFC2131-4.1-15 too | re-running the "Derived listing" filter in the per-requirement audit layout, 2026-11-14 | each routed under "Blocked by"; both RFC2131 ids are owned by `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 since 2026-11-14 |
| fix placed in one backend | the unspecified tunnel remote was refused in the netlink backend's `parseTunnelRemote` (`dbdde9fb4f`) and recorded as fixed | `ze config validate`, a commit and the VPP backend accepted it, so the operator met the refusal only after commit and never on VPP, and the message named no fix | the closure review read the producer of the recorded fix (R1-3) | `04580da85d` refuses it at config verify for every kind and either backend |
| claim without its producer read | AC-C1 was marked done while RFC2131-4.1-7 and 4.1-15 sat in no spec's acceptance criteria | the client's option overload code is missing, not only a test | the closure review (R1-1) | owned by `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 |
| wrong ledger call | RFC1350-5-2 was retired in `9e8443e287` on the ground that Ze refuses WRQ | an owner rule makes the TFTP receiver requirements owed work (`ai/META-DECISIONS.md`, "Goal and scope") | the AC-C3 audit, 2026-11-14 | restored as the gap RFC1350-5-5 in `335e80c658` |
| near miss | the naming finding on `sflow_v5_register_test.go` was first to be cleared with an `RFC naming: untagged` marker | two tests in the file enforce SFLOW-V5-x-16, so the marker would have been false | reading the file's tests against the x-16 row before writing the marker | tagged, recorded and judged in `8b6e37aaaa` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Goal (AC-C1): the derived listing holds only the "Blocked by" verdicts, each in its blocking spec's acceptance criteria | Done | the "Derived listing" re-run of 2026-11-14 | nine rows, each under "Blocked by" and named in its owner's acceptance criteria; the two RFC2131 ids since 2026-11-14 in `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 (closure review R1-1) |
| P-1: an approval before each tagged-unit edit, `RFC-approved:` trailers | Done | `335e80c658` (x-15 tag removal), `8b6e37aaaa` (two `RFC-approved:` lines), and the earlier per-package commits | |
| P-2: verdicts replaced only through `audit-stamp mode rejudge` | Done | A-1 | |
| P-3: each blocked verdict moved into its blocking spec | Done | "Blocked by" table, each owner read 2026-11-14 | |
| D-8 defects fixed failing-test-first, with interop where a peer exists | Done | Bugs Found/Fixed; Goal Validation | the TFTP fix has its curl interop red and green; the geodns and tunnel fixes have no wire peer for a refused config value |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-C1 | Done 2026-11-14 | the "Derived listing" re-run after `12945e76f0` returns nine rows, each named under "Blocked by" with the spec that owns it | SFLOW-V5-4.3-2, the one row that was this child's, re-judged `enforced` by an independent judge in `12945e76f0`. The closure review (R1-1) found RFC2131-4.1-7 and 4.1-15 in no spec's acceptance criteria when this row was first marked done; since 2026-11-14 both are AC-31 of `plan/release/fixes/spec-network-services-one-off-defects.md` (the client half: the vendored `FromBytes` truncates `sname` and `file` at the first NUL, so the code is missing), and `plan/release/validation/spec-rfc-clause-scoped-proof.md` owns neither fix |
| AC-C2 | Done | the 77 ids this child moved to `enforced`, derived by diffing the audit ledger between `519d9f4f2a~1` and `a640d04015` (67 from weak, 10 with no earlier verdict), are all still `enforced` on 2026-11-14, each with at least as many records under `rfc/discrimination/<stem>/<id>.json` as tagged tests in its audit file; every re-judge was by an agent that did not write the test (commit messages of `519d9f4f2a`, `9e8443e287`, `dbdde9fb4f`, `a640d04015`) | |
| AC-C3 | Done 2026-11-14 | the 2026-11-14 audit found the split-needed and narrowing rows resolved except five, and `335e80c658` resolves those five: RFC 2181 Section 10.3 "never a CNAME RR" by a dated correction (it restates RFC2181-10.3-1); SFLOW-V5-x-15 retired; SFLOW-V5-x-11 kept verbatim with a correction saying why; RFC1350-2-3 by a dated correction; and the TFTP upload obligation, retired as RFC1350-5-2 in `9e8443e287` on the ground that Ze refuses WRQ, against the owner rule that the TFTP receiver requirements are owed work (`ai/META-DECISIONS.md`, "Goal and scope"), restored as the gap RFC1350-5-5 | the closure review spot-checked all 17 narrowing rows: each now has a row of its own or a dated correction. RFC1350-5-5 is listed in the RFC gaps of `plan/post-release/features/spec-tftp-server-uploads.md` and owned there by AC-32 (a stored upload read back is byte-identical), and that spec's site 5:2 constraint is corrected (closure review R1-4) |
| AC-C4 | Done | every verdict another spec's producer fix owns sits in that spec's acceptance criteria ("Blocked by"), and this child fixed none of their defects | |
| AC-C5 | Done 2026-11-14 | `./le rfc check` after `8f64f14196` reports 176 violations across the corpus and none naming this child's stems, packages or ids. The closure review (R1-5) found that the check had not run after `2cbf113ecc`: that lint annotation had shifted the SFLOW-V5-4.3-2 verdict's file fingerprint with its units byte-identical, and `39205d383c` resealed it. The run after `2587658ad3` had reported 177 violations, none of this child's. The two findings of the earlier run, below, were resolved: (1) the naming finding on `internal/plugins/flowexport/sflow_v5_register_test.go` in `8b6e37aaaa`, not with an `RFC naming: untagged` marker, which would have been false, but by tagging the two tests in it that enforce SFLOW-V5-x-16 for the if_counters fields ifSpeed and ifDirection (`TestInterfaceCountersFromUnknownSpeed` positive, `TestInterfaceCountersFromExtended` negative), each with a record on a hand-written break whose red was observed; an independent judge approved both tags and re-judged x-16, which stays `enforced` over five tests; (2) the four RFC9728-7.1-2 records, re-taken in `2587658ad3`, each red observed. Two record runs first failed for reasons outside this work (another session's cache clean removed build-cache entries mid-build; another session's half-applied reactor edit did not compile) and passed once those ended. The earlier run, after `335e80c658`, reported two findings in this child's owned stems and no stale or shifted audit verdict, both since resolved: the test-file-name finding on `sflow_v5_register_test.go` that `335e80c658` caused by taking off its SFLOW-V5-x-15 tag (resolved in `8b6e37aaaa`), and four revert records of RFC9728-7.1-2 in `cmd/ze/hub` that `537b3a87f9` (2026-11-10) staled by changing `loadMCPTLSConfig` (re-taken in `2587658ad3`). The RFC2131, RFC7950-7.6.5-1, 7.9.4-1 and 9.10.2-1 lines after the violation list are the report of `partial` rows, not violations | neither earlier finding was added by a verdict this child changed; the second was another spec's code change staling this child's records |
| AC-C6 | Done, vacuously | the row-quality and mistagged-unit tables hold no row of this child | |
| AC-C7 | Done | RFC1035-4.1.4-1 re-judged `enforced` 2026-09-30 in `dbdde9fb4f`, recorded in the R-7 table; the other four R-7 rows are AC-28 of `plan/release/validation/spec-dns-existing-support-proof.md` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| tagged units of the sixteen packages in scope | Done | the per-package commits listed under What Was Implemented; records under `rfc/discrimination/<stem>/` | 77 ids `enforced` (AC-C2); `12945e76f0` and `8b6e37aaaa` add the 2026-11-14 units |
| boundary tests where a quoted sentence bounds a field | Done | judged per verdict by the non-author judges | |
| functional `.ci` tests in scope | Done | none needed a change; no verdict demanded one | the two daemon-level sFlow `.ci` cases are owned elsewhere (Work Not Done) |
| interop for the D-8 fixes | Done | `TestTFTPCurlDuplicateACKInterop` (Goal Validation) | green, red, green, 2026-11-14; the geodns and tunnel fixes refuse a config value and have no wire peer |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `_test.go` files of the packages in scope | Done | per-package commits |
| `rfc/short/`, `rfc/audit/`, `rfc/discrimination/`, `rfc/corrections/`, `rfc/extraction/` of the owned stems | Done | one writer per ledger file held |
| D-8 producers | Done | `internal/plugins/tftpserver/handler.go`, geodns `checkName`, `internal/plugins/iface/netlink/tunnel_linux.go`; in `04580da85d` also `internal/component/iface/tunnel.go`, `internal/component/iface/config.go` and `internal/plugins/iface/vpp/tunnel.go` |
| `docs/features/rfc-status.md` | Changed | derived and git-ignored, never edited |
| the blocking specs' acceptance criteria | Done | "Blocked by" |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | Skipped | owed by the documentation rule, refused by the permission system for an agent (Work Not Done) |

### Audit Summary
- **Total items:** 22 (5 requirements, 7 ACs, 4 test rows, 6 file rows)
- **Done:** 20
- **Partial:** 0
- **Skipped:** 1 (the flow-export page, owed before closure: closure review R1-2, open)
- **Changed:** 1 (`docs/features/rfc-status.md`, derived rather than edited)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| AC-C1: the derived listing holds only "Blocked by" ids | ledger query | 2026-11-14, after `12945e76f0`: nine rows, each under "Blocked by" with its owning spec; `rfc/audit/sflow-v5/SFLOW-V5-4.3-2.json` reads `enforced` |
| the sFlow agent-address advice reaches the operator | unit tests through the engine's configure callback | `TestSFlowV5AgentAddressMultiHomedEngineWarns` and `TestSFlowV5AgentAddressOnLoopbackEngineSilent`, each red under its recorded break; unit packages `internal/plugins/flowexport` and `internal/plugins/flowexport/sflow` green, 2026-11-14 (`12945e76f0`) |
| every resolved verdict proven and judged by a non-author | discrimination records and audit | 2026-11-14: the 77 moved ids are `enforced` with records covering every tagged test (AC-C2) |
| the TFTP duplicate-ACK fix works against another implementation | interop | `TestTFTPCurlDuplicateACKInterop`, observed 2026-11-14 on arm64, Ze's kernel 7.3.0-ze in the QEMU docker-lab guest, curl 8.23.0 pinned by digest, the test binary cross-built for linux/arm64 and run in a docker:cli container on the guest's network. Real build PASS: "replayed native ACK 1 once; dropped first DATA 2; kernel arrival interval 5.007927169s; forwarded final ACK 3; curl matched all 1500 bytes (8 relay datagrams)". With `windowSender.awaitACK` returning on an ACK that is not for the outstanding block (the behaviour before `519d9f4f2a`): FAIL, "duplicate ACK relay: DATA in answer to duplicate ACK after 36.333µs, before five-second timeout", curl exit 28 with 512 bytes received. Real build PASS again (interval 5.004289378s). Recorded on `docs/architecture/testing/interop.md` in `62f22dc4a5` |
| the sit remote fix | unit, kernel integration and functional | `TestRFC4213SitRefusesAnUnverifiedSource` refuses remote "" and 0.0.0.0 with no link left, recorded on `parseTunnelRemote` (R-7 narrowing table, 2026-09-30). After `04580da85d`, observed 2026-11-14: unit packages `internal/component/iface` and `internal/plugins/iface/vpp` green; in the netlink package `TestBuildTunnelLinkRefusesAnUnspecifiedRemote` and both RFC 4213 sit tests green on Ze's kernel 7.3.0-ze in the QEMU guest; `test/parse/iface-tunnel-unspecified-remote-refused.ci` green through `ze config validate`, stable over 80 runs under load before promotion; the RFC4213-3.6-1 negative record re-taken in the guest, red observed. No wire peer exists for a refused config value |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| the nine "Blocked by" verdicts and the four R-7 rows | producer fix or owed test owned elsewhere | each spec named in "Blocked by" |
| the DHCPv4 client's option overload, RFC2131-4.1-7 and RFC2131-4.1-15 (closure review R1-1) | the client code is missing, not only a test: the vendored `dhcpv4.FromBytes` reads `sname` and `file` as NUL-terminated strings and never consults option 52; the server half is tagged and recorded. Owned since 2026-11-14 by AC-31 of the spec on the right (that edit is uncommitted at the time of writing and lands with this closure's record); `plan/release/validation/spec-rfc-clause-scoped-proof.md` owns neither fix. No journal row of 2026-11-14 for RFC2131-4.1-7 exists in `plan/journal/field-carries-two-meanings.md`, which holds only the 2026-10-30 row for 4.1-15 | `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 |
| RFC1350-5-5, the restored TFTP upload gap (closure review R1-4) | `335e80c658` restored the obligation as RFC1350-5-5 in `rfc/short/rfc1350.md`. The owner spec now lists it in its RFC gaps, owns it in AC-32 (a stored upload read back is byte-identical) and corrects its site 5:2 constraint (uncommitted at the time of writing; lands with this closure's record). Its first edit was refused because a file the spec will create was named under Files to Modify, not because of a hook defect, so no hook journal row is owed | `plan/post-release/features/spec-tftp-server-uploads.md` AC-32 |
| the flow-export architecture page (closure review R1-2, OPEN) | `docs/architecture/flowexport/flow-export-1-counter-export.md`, section "Daemon agent-address warning tests", says the absent-address case and the classifier unit tests are the only coverage of this warning, which `12945e76f0` made false. `ai/rules/documentation.md` owes that correction in this same work, before closure; it is not done because the permission system refused the agent's edit of the page, and the owner has been asked. The uncommitted 2026-11-14 row in `plan/journal/comment-describes-superseded-behaviour.md` is listed in no spec's Owned journal rows table yet, so a commit carrying it is refused until it is owned or the page edit lands | this spec, before closure |
| the two daemon-level `.ci` callers of the Linux fixture (closure review R1-N1) | `internal/test/fixture/register_sflow_agent_address_linux.go` (`69aaf63f58`) registers two fixtures that no committed `.ci` calls, and its comment says the two callers must hold an exclusive group. It is not unowned: the same occurrence is the 2026-10-30 row of `plan/journal/claim-outlives-the-evidence-it-cites.md`, listed in the owner spec's Owned journal rows table, whose entry says its sFlow export work writes them (both read 2026-11-14) | `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` |
| a `./le` action that runs one docker-tagged package test on the docker-lab guest (closure review R1-N6) | `TestTFTPCurlDuplicateACKInterop` was run by hand: cross-built, then run in a container on the guest's network. `docs/architecture/testing/interop.md` says no action does it (`62f22dc4a5`). Open, with no owner named yet: it still needs a journal row and an owning spec | none named yet |
| RFC7950-9.1-1, named by two specs | both name the verdict, and they split it by form: `spec-config-yang-reference-types` says "For already accepted forms, the release child owns correctness and remaining review; a broad RFC7950-9.1-1 claim must not substitute for its specific evidence." The verdict reaches `enforced` when both halves land | `plan/release/fixes/spec-config-yang-when-unique-choice.md` (accepted forms, AC-2); `plan/post-release/features/spec-config-yang-reference-types.md` (unsupported forms, AC-9) |
| `./le verify worktree` | obsolete: the owner's standing rule (`ai/META-DECISIONS.md`, "Ze is pre-release") replaces it; closing a spec never waits on a whole-tree check, no agent runs any `./le verify` command, and GitHub CI verifies a pushed commit | none: GitHub CI verifies the pushed commit |

## Review Gate

**The gate is not clean.** Round 1 ended with findings, R1-2 is still open, and no round has yet verified the fixes made after it.

| Field | Value |
|-------|-------|
| Artifact | none recorded yet: `./le spec review record` has not run for this spec |
| `./le spec review check` | not clean: round 1 ended with 6 ISSUEs, one of them open (R1-2) |
| Rounds | 1 |
| Reviewer lenses used | round 1 (independent reviewer, 2026-11-14, authored none of the change; scope: the five earlier commits `519d9f4f2a`, `9e8443e287`, `dbdde9fb4f`, `04c6bd7371`, `a640d04015`, and `12945e76f0`, `62f22dc4a5`, `335e80c658`, `8b6e37aaaa`, `2587658ad3`): 0 BLOCKER, 6 ISSUE, 6 NOTE |

### Findings (round 1, 2026-11-14, reviewer authored none of the change)

The fixes below were made after the round. No round has verified them at source yet.

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| R1-1 | ISSUE | AC-C1 was marked done, but RFC2131-4.1-7 and RFC2131-4.1-15 sat in no acceptance criterion of any spec. The DHCPv4 client never honours option overload: the vendored library's `FromBytes` truncates `sname` and `file` at the first NUL, so the code is missing, not only a test | owned since 2026-11-14 by `plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 (uncommitted at the time of writing; lands with this closure's record). `plan/release/validation/spec-rfc-clause-scoped-proof.md` owns neither fix |
| R1-2 | ISSUE | `docs/architecture/flowexport/flow-export-1-counter-export.md`, section "Daemon agent-address warning tests", says the classifier unit tests are the only coverage below the daemon; false since `12945e76f0` | OPEN. An agent's edit to that page was refused by the permission system and the owner has been asked. The correction is owed before closure: a page made wrong by a change is corrected in the same work |
| R1-3 | ISSUE | the unspecified tunnel remote was refused only when the netlink backend applied it; `ze config validate`, a commit and the VPP backend accepted it, and the message named no fix | fixed by `04580da85d`: `TunnelSpec.CheckRemote`, called by `parseTunnelEntry`, refuses it at config verify for every tunnel kind and either backend with an operator message naming the fix; both backends repeat the same check. Evidence: unit packages `internal/component/iface` and `internal/plugins/iface/vpp` green; the netlink package's `TestBuildTunnelLinkRefusesAnUnspecifiedRemote` and both RFC 4213 sit tests green on Ze's kernel 7.3.0-ze in the QEMU guest; `test/parse/iface-tunnel-unspecified-remote-refused.ci` green through `ze config validate`, stable over 80 runs under load before promotion; the negative record of RFC4213-3.6-1 re-taken in the guest, red observed; scoped lint reports nothing in the files changed |
| R1-4 | ISSUE | `plan/post-release/features/spec-tftp-server-uploads.md` did not list RFC1350-5-5 and still said extraction site 5:2 was excluded | that spec now lists RFC1350-5-5 in its RFC gaps, owns it in AC-32 (a stored upload read back is byte-identical), and its site 5:2 constraint is corrected (uncommitted at the time of writing; lands with this closure's record). Its edit had been refused because a file to be created was named under Files to Modify, not because of a hook defect |
| R1-5 | ISSUE | `./le rfc check` had not run after `2cbf113ecc` | run: the lint annotation of `2cbf113ecc` had shifted the SFLOW-V5-4.3-2 verdict's file fingerprint (units byte-identical), resealed in `39205d383c`; the check run after `8f64f14196` reports 176 violations across the corpus and none naming this child's stems, packages or ids |
| R1-6 | ISSUE | the RFC 1350 ledger named `sendAndWaitACK` for the duplicate-ACK fix | fixed by `cba4bb0810`: the enrolment reason and the extraction reasons for RFC1350-2-2, 6-1 and 7-1 name `serveFile` and `windowSender.awaitACK`; two reasons no longer claim a single-polarity annotation the rows do not carry |
| R1-N1 | NOTE | the Linux fixture `internal/test/fixture/register_sflow_agent_address_linux.go` looked unowned | not unowned: the 2026-10-30 row of `plan/journal/claim-outlives-the-evidence-it-cites.md`, owned by `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`, both read 2026-11-14; the Work Not Done row says so |
| R1-N2 | NOTE | the sFlow test harness's host goroutine is not joined in cleanup, but exits on cancel or close; restore order and the configured-record check are sound | recorded, no change |
| R1-N3 | NOTE | the SFLOW-V5-x-11 correction left a question open | reworded in `cba4bb0810` to state decision D-3 |
| R1-N4 | NOTE | RFC1350-5-5 has no audit verdict, as rfc1350's other gaps have none | recorded |
| R1-N5 | NOTE | the re-taken RFC9728-7.1-2 records are whole-body breaks that prove reach | recorded; the verdict is unchanged |
| R1-N6 | NOTE | no `./le` action runs one docker-tagged package test on the docker-lab guest; `docs/architecture/testing/interop.md` says so (`62f22dc4a5`) | recorded under Work Not Done as open, with no owner named yet: it still needs a journal row and an owner |

The round also recorded as sound: the `windowSender.awaitACK` logic; RFC1350-5-5 as a verbatim span at the right level, with site 5:2 mapped to it; the RFC 2181 and RFC1350-2-3 corrections; the SFLOW-V5-x-15 retirement; and the SFLOW-V5-x-16 tag claims.

## Pre-Commit Verification

Observed results only, 2026-11-14.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `plan/learned/050-rfc-verdict-fix-services.md` | yes | written at closure, 2026-11-14 |
| `website/changes/fragments/rfc-verdict-fix-services.md` | yes | written at closure, 2026-11-14 |
| `internal/plugins/flowexport/sflow_v5_agent_advice_test.go` | yes | added in `12945e76f0` (`git show --stat`) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-C1 | the listing holds only "Blocked by" ids | the "Derived listing" re-run after `12945e76f0`: nine rows, each under "Blocked by" |
| AC-C5 | no violation in the owned stems | `./le rfc check` after `8f64f14196`: 176 violations across the corpus, none naming this child's stems, packages or ids |
| scoped unit tests | the packages the 2026-11-14 Go changes reach stay green | unit packages `internal/plugins/flowexport` and `internal/plugins/flowexport/sflow` green, and the new sFlow engine tests pass with `-race -count=2`; after `04580da85d`, `internal/component/iface` and `internal/plugins/iface/vpp` green, the netlink tunnel tests green in the QEMU guest, and `test/parse/iface-tunnel-unspecified-remote-refused.ci` green (Goal Validation) |
| interop | the TFTP duplicate-ACK fix holds against curl | `TestTFTPCurlDuplicateACKInterop`: green on the real build, red under the recorded mutation, green again (Goal Validation, `62f22dc4a5`) |
| scoped lint | the changed Go passes lint | `internal/plugins/flowexport`: zero issues after `2cbf113ecc`. `internal/component/iface`, `internal/plugins/iface/vpp` and `internal/plugins/iface/netlink`: 19 findings, none in the files `04580da85d` or `8f64f14196` changed |
| derived listing owners | each of the nine rows is owned elsewhere | each owner read 2026-11-14 ("Derived listing" table), each naming its id in its acceptance criteria |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc check` → `checkAuditFindings` | none: a Go unit, `TestCheckAuditRatchetSeesTipCommit` (`internal/le/rfc/check_audit_baseline_test.go`) | `checkAuditFindings` (`internal/le/rfc/check_audit.go`) read at closure: it refuses a deleted weak, wrong or partial verdict and an upgrade to `enforced` over byte-identical units with no `upgrade_reason`; the 2026-11-14 `./le rfc check` run named no finding in the owned stems |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `stampRefusal` (`internal/le/rfc/audit_stamp.go`): every re-judge here used `mode rejudge` |
| A-2 | broken | three verdicts the 2026-09-28 list lacked, each routed (Mistake Log, Deviations) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/testing/interop.md`, the TFTP curl runs | `62f22dc4a5` | yes |
| `docs/features/interfaces.md`, the unspecified-remote refusal | `TunnelSpec.CheckRemote` (`internal/component/iface/tunnel.go`) read 2026-11-14: called by `parseTunnelEntry` (`internal/component/iface/config.go`) at config verify, it refuses an unparsable or unspecified remote for every tunnel kind with a message naming the fix (`set interface tunnel <name> encapsulation <kind> remote ip <address>`); `parseTunnelRemote` and the VPP backend's tunnel path call it again; the page was updated in `04580da85d` | yes |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | the page's coverage sentence is false since `12945e76f0` | no: owed, refused for an agent (Work Not Done) |
| `docs/features/rfc-status.md` is not edited | derived by `./le rfc update`, git-ignored | yes |

## Core Insight

Reading the RFC's own text decided every ledger item: a retired row whose sentence obliged nothing (SFLOW-V5-x-15), a retirement an owner rule reversed (RFC1350-5-2, back as the gap RFC1350-5-5), and a sentence that restated an existing row (RFC 2181 Section 10.3, already RFC2181-10.3-1). A test that reached the classifier but not the engine left SFLOW-V5-4.3-2 weak until tests drove `runEngine`'s configure callback. And the naming check surfaced real untagged coverage that an `RFC naming: untagged` marker would have hidden: two tests of SFLOW-V5-x-16. Routed to `plan/learned/050-rfc-verdict-fix-services.md`.
