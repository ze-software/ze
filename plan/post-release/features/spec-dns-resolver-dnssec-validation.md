# Spec: dns-resolver-dnssec-validation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Optional DNSSEC work: the fttx-edge profile lists RFC 4033, 4034 and 4035 under `Optional use cases` only, `rfc/short/rfc4034.md` records the owner ruling that DNSSEC is not urgent and optional only, and `ai/META-DECISIONS.md` counts an optional RFC only once Thomas brings it into scope. This spec runs only once he brings DNSSEC into scope. |
| RFC gaps | RFC2181-5.4.1-8, RFC4035-3-5, RFC4035-3.1.6-6, RFC4035-3.2.1-1, RFC4035-3.2.1-2, RFC4035-3.2.1-3, RFC4035-3.2.2-1, RFC4035-3.2.2-4, RFC4035-3.2.3-2, RFC4035-4.2-1, RFC4035-4.2-3, RFC4035-4.2-5, RFC4035-4.2-6, RFC4035-4.3-1, RFC4035-4.4-1, RFC4035-4.7-2, RFC4035-4.7-3, RFC4035-4.7-7, RFC4035-4.8-1, RFC4035-5-1, RFC4035-5-2, RFC4035-5-3, RFC4035-5.2-1, RFC4035-5.2-3, RFC4035-5.3.1-1, RFC4035-5.3.1-2, RFC4035-5.3.1-3, RFC4035-5.3.1-4, RFC4035-5.3.1-5, RFC4035-5.3.1-6, RFC4035-5.3.1-7, RFC4035-5.3.1-8, RFC4035-5.3.1-9, RFC4035-5.3.1-10, RFC4035-5.3.2-1, RFC4035-5.3.2-2, RFC4035-5.3.2-3, RFC4035-5.3.3-1, RFC4035-5.3.3-2, RFC4035-5.4-1, RFC4035-5.4-2, RFC4035-5.4-3, RFC4035-5.4-4, RFC4035-5.5-2, RFC4035-5.5-3 |
| Depends | spec-lan-dns-resolver.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make the LAN resolver of `plan/post-release/features/spec-lan-dns-resolver.md`
a security-aware, validating recursive server (RFC 4033, 4034, 4035), with Ze's
own Go validator in `internal/core/dnssec`, which
`plan/post-release/features/spec-firewall-dnssec-membership-validation.md` also
consumes.

This spec was split out of `spec-lan-dns-resolver` on 2026-10-27, when its
D-1 was answered (b) by existing owner rulings: the LAN resolver ships
not security-aware, and this spec holds what the previous draft designed under
D-1 (a), moved verbatim so nothing is lost. It is a skeleton: the design below
is the previous draft's, not yet re-verified at the producers, and the spec is
not designed until it is.

Open design items carried from the split, to settle when DNSSEC is brought into
scope:

| # | Item | Why open |
|---|------|----------|
| O-1 | Default of `dns-resolver dnssec validation` (`on` in the previous draft) | The firewall owner decision of 2026-10-26 sets `off` as the default of `system dns dnssec-validation` for the stub; whether the LAN resolver follows it is not decided |
| O-2 | V-13 and RFC4035-4.2-5, 4.2-6, 5.2-1 (DS asked of the parent's servers) | They need full recursion, which is `plan/post-release/features/spec-dns-resolver-recursion.md` and waits on D-2 |
| O-3 | Trust anchors: static (built-in root DS for key tags 20326 and 38696, replaceable by config) or RFC 5011 rollover | The previous draft chose static: RFC 4035 Section 4.4 asks for a robust boot-time mechanism with a SHOULD, RFC 5011 has no summary and no profile lists it, and `ai/META-DECISIONS.md` reports optional requirements as a second figure |

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-lan-dns-resolver.md` - the resolver this spec makes security-aware; its harness wrapper `Recursive`, cache and limits
  → Constraint: the plugin reaches nothing but its own state, `internal/core/dnsserver` and `internal/core/dnssec`; the validator package holds no plugin or config vocabulary
- [ ] `docs/architecture/dns/server-harness.md` - the shared listener and the enforced answer shape
- [ ] `rfc/profiles/fttx-edge.md` - RFC 4033, 4034, 4035 and 8106 are optional

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4035.md` - DNSSEC protocol modifications
  → Constraint: every recursive-server and validator row is `{not-applicable: ze's DNS servers never recurse ...}` or `{not-applicable: ze runs no local validator ...}` today; each row in `RFC gaps` becomes a tagged, discriminated test and its annotation is rewritten
- [ ] `rfc/short/rfc4034.md` - record formats the validator parses (exists, not enrolled)
- [ ] `rfc/short/rfc4033.md` - terms only (foundation)
- [ ] `rfc/short/rfc2181.md` - RFC2181-5.4.1-8 ("When DNS security ... is in use"), rewritten `excluded-kind: feature-out-of-scope` by the LAN resolver spec, closed here

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/resolve/dns/resolver.go` - the stub's `dnssecDecision` treats upstream SERVFAIL as the DNSSEC failure signal; no local validation
- [ ] `vendor/github.com/miekg/dns/dnssec.go` - `RRSIG.Verify` covers RSASHA1, RSASHA1NSEC3SHA1, RSASHA256, RSASHA512, ECDSAP256SHA256, ECDSAP384SHA384, ED25519; miekg has no validator and no chain building
- [ ] `internal/test/mock/dns/server.go`, `zone.go` - the functional-test upstream; it signs nothing

Carried from the previous draft and to be re-verified at the producers when this spec is designed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `service { dns-resolver { dnssec { ... } } }`.
- Wire: a recursive query to the LAN resolver whose answer comes from a signed or unsigned zone.

### Transformation Path
1. `OnConfigVerify` also refuses a malformed trust anchor.
2. The cache key also carries the CD bit.
3. Every upstream exchange carries EDNS0 with DO=1 and CD=1.
4. Validation through `internal/core/dnssec`: chain from the configured anchor through DS and DNSKEY to the signer; outcome secure, insecure, bogus or indeterminate; NSEC or NSEC3 denial for negative answers.
5. Bogus results go to a BAD cache with a fixed 60 s TTL.
6. Reply: bogus with CD=0 draws SERVFAIL; AD set iff secure and the query had DO or AD; DNSSEC RRs stripped when DO=0 unless asked for.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to validator | `internal/core/dnssec` Go API (core package, no plugin import) | No |
| Firewall domain plugin to validator | the consumer `spec-firewall-dnssec-membership-validation` designs | No |

### Integration Points
- `internal/core/dnssec` - new package shared by the resolver and the firewall consumer
- `internal/plugins/dnsresolver` - security-aware upstream queries, BAD cache, AD rule, stripping

## Config and CLI surface (from the previous draft)

| Path | Type | Default | Constraint | Purpose |
|------|------|---------|------------|---------|
| `dns-resolver dnssec validation` | enum `on`, `off` | `on` (O-1) | - | validate or pass through |
| `dns-resolver dnssec trust-anchor` | leaf-list of DS presentation strings | built-in root DS for key tags 20326 and 38696 | 1..16 | replace the built-in anchors |

Fixed: BAD-cache TTL 60 s (Unbound `val-bogus-ttl` default 60). `show dns-resolver`
gains the BAD cache entry count, anchor key tags and validation outcome counts.
`le mock dns` gains `--signed` (generate a key, sign the zone, print the DS),
`--tamper-rrsig` and `--strip-rrsig`.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | miekg/dns `RRSIG.Verify` covers the algorithms the root and TLDs use | `vendor/github.com/miekg/dns/dnssec.go` `Verify` switch lists RSASHA1, RSASHA1NSEC3SHA1, RSASHA256, RSASHA512, ECDSAP256SHA256, ECDSAP384SHA384, ED25519 | a missing algorithm must be treated as insecure (RFC 4035 Section 5.2), never bogus | unit test per algorithm with keys generated by miekg | partly read at source |
| A-5 | The host clock is close enough for RRSIG validity windows at boot | NTP client plugin (`internal/plugins/ntp`) | every signed answer bogus until sync | R-3 mitigation test | unvalidated |
| A-6 | NSEC3 denial proofs are needed for correct operation (`.com` and most TLDs use NSEC3) and need no summary | real zones; the summary freeze applies to summaries, not code | every unsigned delegation under an NSEC3 parent bogus, a logic defect | interop zone signed with NSEC3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-3 | Clock unsynced at boot makes signed answers bogus | SERVFAIL burst after boot, `validation_total{outcome="bogus"}` with reason `signature-expired` | log names the clock; doctor check `doctor-dns-resolver-clock-unsynced`; never downgrade bogus to insecure |
| R-4 | Work amplification from NSEC3 iterations | `upstream_queries_total` per client query | NSEC3 iteration cap treated as insecure above the cap |
| R-5 | BAD cache memory growth | `ze_dns_resolver_cache_entries{cache="bad"}` at limit | entry-count bound with LRU eviction |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| signed zone from `le mock dns --signed` | → | `internal/core/dnssec` chain validation | `test/plugin/dns-resolver-dnssec-secure.ci` |
| tampered signature | → | bogus path | `test/plugin/dns-resolver-dnssec-bogus.ci` |

## Acceptance Criteria

Moved verbatim from `spec-lan-dns-resolver` (its Validation ACs).

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| V-1 | Signed zone with a valid chain from the configured anchor; query DO=1 | NOERROR, AD=1, RRSIGs in the answer section |
| V-2 | Same zone; query DO=0 AD=0 | AD=0, no RRSIG, NSEC or NSEC3 in any section |
| V-3 | Same zone; query DO=0 AD=1 | AD=1, DNSSEC records stripped |
| V-4 | Query DO=0 for type RRSIG or DNSKEY explicitly | Those records returned, not stripped (RFC4035-3.2.1-3) |
| V-5 | Signed zone with a tampered RRSIG; query CD=0 | SERVFAIL (RFC4035-5.5-2), `validation_total{outcome="bogus"}` +1 |
| V-6 | Same; query CD=1 | Full response, AD=0 (RFC4035-5.5-3) |
| V-7 | Signed zone whose upstream strips RRSIGs | SERVFAIL; absence of signatures is not taken as insecure (RFC4035-5-3) |
| V-8 | Unsigned child of a signed parent, proven by an authenticated NSEC and, separately, NSEC3 record that no DS exists | NOERROR, AD=0 (insecure, not bogus) |
| V-9 | Name absent from a signed zone, NSEC and NSEC3 variants | NXDOMAIN with AD=1 when DO=1, denial validated; a forged NXDOMAIN without valid proof draws SERVFAIL |
| V-10 | Bogus answer queried again within 60 s | SERVFAIL from the BAD cache with no upstream query; after 60 s refetched; a BAD entry never returned to a CD=0 query as data (RFC4035-4.7-3, 4.7-7) |
| V-11 | `dnssec trust-anchor` set to a lab DS record | The lab hierarchy validates; the built-in root anchors are not used; a malformed DS refuses the commit naming the value |
| V-12 | `dnssec validation off` | No validation, AD never set, no SERVFAIL for a bogus zone; the resolver side still sends DO=1 (RFC4035-3.2.1-1) |
| V-13 | (with recursion, O-2) DS for a child is asked of the parent's servers | RFC4035-4.2-6, 5.2-1 |
| V-14 | Upstream reply in forward mode carries AD=1 for an unsigned zone | Ze's reply AD=0 (validated locally; upstream queried with CD=1) |
| V-15 | Wildcard-expanded signed answer; DNAME-synthesized CNAME | Validated (RFC4035-4.2-3, 4.8-1) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Attacker zone serves a bad signature | upstream → validator bogus → SERVFAIL to the customer | `test/plugin/dns-resolver-dnssec-bogus.ci`; interop bogus zone (scenario to name at design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChainSecure`, `TestChainBogusSignature`, `TestChainExpired`, `TestChainMissingSignatures`, `TestUnsupportedAlgorithmInsecure` | `internal/core/dnssec/chain_test.go` | RFC4035-5-1/5-2/5-3, 5.2, 5.3.1-1..10 | |
| `TestNSECDenial`, `TestNSEC3Denial`, `TestNSEC3IterationCap` | `internal/core/dnssec/denial_test.go` | RFC4035-5.4-1..4, A-6, R-4 | |
| `TestWildcardExpansion`, `TestDNAMESynthesizedCNAME`, `TestTTLClampToRRSIG`, `TestTrustAnchorParse` | `internal/core/dnssec/*_test.go` | RFC4035-4.2-3, 5.3.2-1, 5.3.3-1, 4.8-1, 5.3.3-2, 4.4-1 | |
| `TestBadCacheTTL`, `TestBadCacheNotServedCD0`, `TestStripDNSSECWhenDOClear`, `TestKeepRequestedDNSSECType`, `TestUpstreamDOAndCD`, `TestDSFromParent` | `internal/plugins/dnsresolver/*_test.go` | RFC4035-4.7-2/3/7, 3.2.2-4, 3.2.1-1/2/3, 3-5, 3.2.2-1, 4.2-5, 4.2-6, 5.2-1, 5.2-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `dnssec trust-anchor` count | 1-16 | 16 | N/A | 17 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-resolver-dnssec-secure` | `test/plugin/dns-resolver-dnssec-secure.ci` | signed mock zone, DO and AD variants | |
| `dns-resolver-dnssec-bogus` | `test/plugin/dns-resolver-dnssec-bogus.ci` | tampered RRSIG: SERVFAIL; CD=1 passthrough; BAD cache | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| to name at design | `test/interop/scenarios/` | Knot DNS hierarchy: signed NSEC, signed NSEC3, unsigned child, bogus zone; Unbound validating with the same anchors as the cross-check | validation outcomes equal Unbound's | |

## Files to Modify
- `internal/plugins/dnsresolver/` - security-aware upstream queries (DO=1 CD=1), strip on DO=0, BAD cache, AD rule, `dnssec` config leaves, metrics `ze_dns_resolver_validation_total{outcome}` and `ze_dns_resolver_cache_entries{cache="bad"}`, doctor `doctor-dns-resolver-clock-unsynced`
- `internal/test/mock/dns/server.go`, `zone.go`, `dns.go` - `--signed`, `--tamper-rrsig`, `--strip-rrsig`
- `rfc/short/rfc4035.md` - every `RFC gaps` row annotated to its tagged test
- `rfc/short/rfc2181.md` - RFC2181-5.4.1-8 closed

## Files to Create
- `internal/core/dnssec/` - `anchor.go` (built-in root DS for key tags 20326 and 38696, parse), `chain.go`, `rrsig.go`, `denial.go` (NSEC, NSEC3), `wildcard.go`, tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `dnssec` container in `ze-dns-resolver-conf.yang` |
| YANG custom validators | Yes | DS record syntax (`ze:validate`) |
| Doctor check for runtime dependencies | Yes | `doctor-dns-resolver-clock-unsynced` |
| Prometheus counters/metrics | Yes | `ze_dns_resolver_validation_total{outcome}` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/lan-dns-resolver.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4035.md`, `rfc/short/rfc2181.md` |

## Implementation Steps

1. **Phase: Validator** -- `internal/core/dnssec`, anchors, chain, NSEC, NSEC3, wildcard, DNAME; `le mock dns --signed`
2. **Phase: Security-aware resolver** -- DO=1 CD=1 upstream, strip on DO=0, BAD cache, AD rule
3. **Phase: Interop and RFC rows** -- the signed-zone scenario with a recorded red; every `RFC gaps` row tagged and discriminated

## Design Insights

RFC sentences the previous draft rested on, read from `rfc/full/`:

| RFC section | Sentence | Design consequence |
|-------------|----------|-------------------|
| RFC 4035 Section 3.2.1 | "The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests, regardless of the state of the DO bit in the initiating request received by the name server side." | DO=1 upstream always, also with `validation off` |
| RFC 4035 Section 3.2.1 | "If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response but MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested." | strip on DO=0 unless the qtype is a DNSSEC type |
| RFC 4035 Section 3.2.3 | "The name server side of a security-aware recursive name server MUST NOT set the AD bit in a response unless the name server considers all RRsets in the Answer and Authority sections of the response to be authentic." | AD only on secure; mixed sections clear it |
| RFC 4035 Section 4.4 | "A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR and SHOULD be capable of being configured with multiple trusted public keys or DS RRs." | `dnssec trust-anchor` list, built-in root DS defaults |
| RFC 4035 Section 4.7 | "Since RRsets that fail to validate do not have trustworthy TTLs, the implementation MUST assign a TTL." | fixed 60 s BAD-cache TTL |
| RFC 4035 Section 5 | "The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists." | stripped signatures from a signed zone are bogus |
| RFC 4035 Section 5.5 | "If the validation was being done to service a recursive query, the name server MUST return RCODE 2 to the originating client.  However, it MUST return the full response if and only if the original query had the CD bit set." | SERVFAIL on bogus with CD=0; full response with CD=1 |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ze's own Go validator in `internal/core/dnssec` | ship Unbound in the appliance and proxy to it; a third-party Go validator | RFC 4035's summary declares Implementation `ze`; the firewall spec needs a validator at the Go resolver boundary; core placement lets both consumers share it without a plugin import |
| Validate locally in forward mode, querying upstream with CD=1 | trust the upstream AD bit | RFC4035-4.6-3 makes a resolver disregard AD from an insecure channel; Unbound validator.c: "User queries are validated with BIT_CD to make our cache clean" |
| AD on a reply iff secure and the query had DO or AD | AD only with DO | RFC 4035 is silent on DO=0 queries. Unbound `reply_info_answer_encode` (util/data/msgencode.c, read 2026-10-27 at line 1039) sets AD when the answer is secure and either the DO bit or the query's AD bit is set. BIND `ns_query_start` agrees ("This allows AD to be returned on queries without DO set.") |
| BAD-cache TTL fixed at 60 s | a config leaf | unbound.conf `val-bogus-ttl` "Default: 60"; RFC asks only that a TTL be assigned |
| NSEC3 validation included | NSEC only | A-6: without it most real TLDs fail; RFC 5155 has no summary and none is written, so its tests carry no RFC tag |

## Known Limitations
- Static trust anchors only (O-3): a root KSK roll needs a Ze release or a config change, until RFC 5011 is admitted.
- No summary is written for RFC 5155 or RFC 5011 (no profile lists them).

## Work Not Done
| Item | Spec |
|------|------|
| DS from the parent's servers in recursive mode (V-13, O-2) | `plan/post-release/features/spec-dns-resolver-recursion.md` must land first |

## RFC Documentation (Scope: protocol)

Every row in `RFC gaps` becomes a tagged unit (`RFC requirement:`) whose
discrimination record is written by `./le rfc record` in the commit of the code
it proves.

## Checklist

### Goal Gates (MUST pass)
- [ ] V-1..V-15 all demonstrated
- [ ] The targeted tests for what changed pass: the scoped package tests and `./le verify changed` over the closing commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only
