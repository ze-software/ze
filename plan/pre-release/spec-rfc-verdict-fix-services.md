# Spec: rfc-verdict-fix-services

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the services group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 8 of 8 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, the services group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **78 weak and 7 wrong, 85 in all, over 21 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

### Packages it owns

Test packages: DNS, TFTP, DHCP, flow export, MCP, config and YANG, and the rest: `internal/plugins/tftpserver`, `internal/plugins/dhcpserver`, `internal/plugins/geodns`, `internal/plugins/as112`, `internal/plugins/flowexport/...`, `internal/component/config/...`, `internal/component/mcp`, `internal/component/resolve/dns`, `internal/core/dnsserver`, `internal/core/probe`, `cmd/ze/hub`.

| Package | Weak + wrong (2026-09-28) | Of which wrong |
|---------|--------------|----------------|
| `internal/plugins/tftpserver` | 12 | 1 |
| `internal/component/config` | 9 | 0 |
| `internal/plugins/flowexport/sflow` | 9 | 0 |
| `internal/component/mcp` | 8 | 2 |
| `internal/component/config/yang` | 6 | 2 |
| `internal/plugins/dhcpserver` | 6 | 1 |
| `internal/plugins/flowexport` | 6 | 0 |
| `internal/plugins/geodns` | 6 | 1 |
| `internal/component/resolve/dns` | 4 | 0 |
| `internal/core/dnsserver` | 4 | 0 |
| `internal/plugins/flowexport/ipfix` | 4 | 0 |
| `internal/plugins/flowexport/netflow9` | 4 | 0 |
| `internal/core/probe` | 3 | 0 |
| `internal/plugins/as112` | 2 | 0 |
| `internal/plugins/vrrp` | 2 | 0 |
| `cmd/ze/hub` | 1 | 0 |

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is DERIVED. No id list is committed here, because the audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^(cmd/ze/hub/\|internal/component/config/\|internal/component/mcp/\|internal/component/resolve/\|internal/core/dnsserver/\|internal/core/probe/\|internal/plugins/as112/\|internal/plugins/dhcpserver/\|internal/plugins/flowexport/\|internal/plugins/geodns/\|internal/plugins/tftpserver/)' --arg own '^(rfc792)$' --arg fo '^(rfc1071)$' 'input_filename as $f \| ($f\|ltrimstr("rfc/audit/")\|rtrimstr(".json")) as $s \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| select(.key\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select((($s\|test($own)) or ([(.value.tests // {})\|keys[]\|test($pk)]\|any)) and ($s\|test($fo)\|not)) \| "\($s)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` adds this child's owned stems tagged from another child's packages; `fo` drops stems another child owns (`^$` matches none); the five ids are the parent's cross-group verdicts |

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 78 weak and 7 wrong, 85 in all, over 21 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). The "Blocked by" verdicts are now acceptance criteria of their blocking specs: `spec-bmp-sflow-export-rfc-defects` AC-3, `spec-mcp-protected-resource-metadata-path` AC-4, `spec-fixit-dns-rfc1035-conformance` AC-28 (paused with that spec).

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong |
|------|--------------|----------------|
| rfc1350 | 6 | 1 |
| rfc2131 | 4 | 0 |
| rfc2132 | 1 | 1 |
| rfc2181 | 5 | 0 |
| rfc2347 | 2 | 0 |
| rfc2348 | 3 | 0 |
| rfc3954 | 4 | 0 |
| rfc4035 | 4 | 0 |
| rfc4578 | 1 | 0 |
| rfc7011 | 5 | 0 |
| rfc7012 | 1 | 0 |
| rfc7440 | 1 | 0 |
| rfc7534 | 2 | 0 |
| rfc7858 | 3 | 0 |
| rfc7871 | 1 | 1 |
| rfc792 | 5 | 0 |
| rfc7950 | 15 | 2 |
| rfc8414 | 2 | 0 |
| rfc8484 | 1 | 0 |
| rfc9728 | 7 | 2 |
| sflow-v5 | 12 | 0 |
| rfc1035 | 0 in `rfc/audit/` | un-enrolled: its six R-7 rows; no audit file |
| rfc2473 | 0 in `rfc/audit/` | narrowing audit row RFC2473-4.1.1-2; no audit file |
| rfc4213 | 0 in `rfc/audit/` | narrowing audit row RFC4213-3.6-1; no audit file |

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
| RFC1035-4.1.4-1 | weak | `TestRFC1035_CompressionPointersInATruncatedDatagram` (`internal/plugins/geodns/rfc1035_compression_test.go`) | "the label must begin with two zero bits": a label length octet of 0x40 to 0xBF passes both polarities |
| RFC1035-4.1.4-5 | weak | `TestRFC1035_InboundCompressionPointerUnderstood` (`rfc1035_compression_test.go`) | the answer does not depend on the pointer being expanded, and no assertion reads its expansion; replies Ze reads as a client are not driven |
| RFC1035-4.2.2-1 | weak | `TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix` (`rfc1035_compression_test.go`) | "use server port 53": the listener runs on a free port and no unit asserts the TCP default |

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC7950-9.1-1 | a type with no canonical form: the value MUST match its lexical representation | §9.1 |
| RFC7950-11-1 | changed semantics MUST get a new definition and identifier; data definition substatements MUST NOT be reordered | §11 |
| RFC8414-3.3-1 | confidentiality protection MUST use TLS with a ciphersuite giving confidentiality and integrity | §6.1 |
| SFLOW-V5-x-7, x-9, x-10, x-12, x-15, x-20, x-24 | claims the document does not state ("wrapping", "2^30-1", "since boot", "[1, 2*N-1]", XDR alignment) or states in a sibling row: row correction | §3.1, §4.3, §5 |
| SFLOW-V5-x-11 | a `sample_pool` field description tagged MUST; no sentence states an obligation: row correction (the code defect is D2 of `plan/immediate/spec-bmp-sflow-export-rfc-defects.md`) | §5 |
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

Each id below was checked on 2026-09-28: a weak or wrong verdict in `rfc/audit/<stem>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix (AC-C4), and the verdict moves into its acceptance criteria (P-3).

| ID | Verdict | Blocking spec |
|----|---------|---------------|
| SFLOW-V5-x-11 | weak | `spec-bmp-sflow-export-rfc-defects` (D2, the code; the row correction in the split table stays here) |
| RFC7950-7.19-1 | weak | `spec-config-yang-loader-structural-checks` (AC-2, 2026-09-30) |
| RFC7950-9.4.4-1 | weak | `spec-config-yang-loader-structural-checks` (AC-2, 2026-09-30) |
| RFC7950-9.6.4.2-1 | weak | `spec-config-yang-loader-structural-checks` (AC-2, 2026-09-30) |
| RFC9728-3.1-3 | wrong | `spec-mcp-protected-resource-metadata-path` |
| RFC9728-3-2 | weak | `spec-mcp-protected-resource-metadata-path` (AC-5; trailing-slash identifier, same defect as 3.1-3; the query clause is proven) |
| RFC9728-3.3-1 | weak | `spec-mcp-protected-resource-metadata-path` (AC-6; trailing-slash identifier, same defect as 3.1-3) |
| RFC1035-2.3.4-1 | weak (R-7) | `spec-fixit-dns-rfc1035-conformance` |
| RFC1035-4.1.1-1 | weak (R-7) | `spec-fixit-dns-rfc1035-conformance` |
| RFC1035-4.1.4-5 | weak (R-7) | `spec-fixit-dns-rfc1035-conformance` |
| RFC1035-4.2.2-1 | weak (R-7) | `spec-fixit-dns-rfc1035-conformance` |

- `spec-fixit-dns-rfc1035-conformance` is `blocked`: its four rows stay with it unless the owner unblocks it (parent overlap table). RFC1035-4.1.4-1 is not among them and is this child's.

## Required Reading

### Architecture Docs
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
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
- 85 verdicts over 16 packages and 22 stems, plus 13 rows copied from the parent's tables
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc7950.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/sflow-v5.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc9728.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `internal/le/rfc/check_audit.go` - `checkAuditFindings` refuses a deleted weak or wrong verdict and an unchanged-units upgrade without `upgrade_reason`

**Behavior to preserve:**
- a commit of this child adds no `./le rfc check` violation of its own; a weak or wrong verdict is never deleted
- product behavior of every package in scope, except a D-8 fix of a verified defect, which carries its own failing-first test

**Behavior to change:**
- the tagged tests, rows, tags and verdicts named in this spec's tables and in the derived listing; D-8 producer fixes where a test exposes a verified defect

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the `RFC requirement:` tags in test files
- `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, typed by a judging agent

### Transformation Path
1. The author agent records `./le rfc approve unit` (P-1), then changes a tagged test, a row, a tag, or a producer (D-8)
2. `./le rfc reseal` re-stamps sibling verdicts the edit only shifted
3. `./le rfc discriminate-record` records the observed red under a break for each added or changed cover
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
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | `./le rfc audit-stamp` help names `mode` | unvalidated |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | unvalidated |

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

## Files to Modify

- the `_test.go` and `.ci` files of the packages above that carry the listed tags
- `rfc/short/<stem>.md`, `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/corrections/<stem>.md`, `rfc/extraction/<stem>.json` for the owned stems only
- a producer under the packages above when a test exposes a verified defect (D-8), named in the phase that finds it
- `docs/features/rfc-status.md`, the row of each owned stem whose support claim changes
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
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` for each owned stem a phase changes, and its `docs/features/rfc-status.md` row, with source anchors; a new `wrong` verdict discloses non-support there first |
| 10 | Test infrastructure changed? | No | this child changes tests, rows and verdicts, not this surface |
| 11 | Affects daemon comparison? | No | this child changes tests, rows and verdicts, not this surface |
| 12 | Internal architecture changed? | No | this child changes tests, rows and verdicts, not this surface |
| 13 | Route metadata keys added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 14 | Prometheus counters added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | this child changes tests, rows and verdicts, not this surface |
| 16 | Any changed source file referenced by existing doc source anchors? | No | only test and ledger files change; a D-8 producer fix runs `./le spec citation anchors` in its phase |
| 17 | Existing docs show config/CLI/API examples for this area? | No | this child changes tests, rows and verdicts, not this surface |

## Implementation Steps

**Procedure P, for every package phase.** One author agent per package, never two on one test file. It reads the listing restricted to the package, records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before each tagged-unit edit (P-1), makes each clause of the quoted sentence asserted in both polarities or corrects the row, and fixes a verified defect under D-8. It runs the scoped package test under `./le job run`, then `./le rfc discriminate-record` for each added or changed cover. An independent judge that wrote none of the tests writes the verdicts in a pending file under session scratch and stamps them with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. `./le rfc reseal` runs when the edit shifted sibling verdicts, serialized between children. Each package lands in one commit through `./le commit create` with the `RFC-approved:` trailer, author and judge together where they can. Verify: the listing filtered to the package holds only blocked ids, and `./le rfc check` adds no violation.

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
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for the D-8 protocol fixes (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** tests + rows + verdicts + records + D-8 code + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-verdict-fix-services.md` only, in the same `./le commit create` script
