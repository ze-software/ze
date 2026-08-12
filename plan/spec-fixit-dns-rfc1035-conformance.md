# Spec: fixit-dns-rfc1035-conformance

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | WP-1, WP-2, WP-3, WP-5, WP-6, WP-7 landed; WP-4 and one escalation open |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the spec already says the shard is created only if something is deferred. Create `plan/deferrals/fixit-dns-rfc1035-conformance.md` on the first deferral) |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`plan/spec-rfcgate-4-ledger.md` re-authored the extraction for RFC 1035 on 2026-07-30.
The new summary declares **27 gated MUST-level obligations**. `docs/features/rfc-status.md`
publishes RFC 1035 as `Supported`. The stem is declared `backlog` in
`rfc/not-enrolled.txt` and cannot enrol until its obligations are met and proven.

Two problems block enrolment.

1. **Six obligations have no code path in Ze at all.** The 512-octet UDP bound is
   never enforced. The TC bit is never set. A response TTL is never raised to the
   zone SOA MINIMUM. Ze performs no zone transfer. An unsupported inverse query
   draws no Not Implemented reply.
2. **About six more admit only a positive polarity.** `github.com/miekg/dns` owns
   the wire codec. A negative test for those rows looks unreachable at first read.

**Owner ruling (Thomas, 2026-07-30): FULL compliance, including zone transfer.**
He was offered a narrower option. That option would have scoped out the roles Ze
does not play. It would also have recorded the positive-only rows under his
authorisation. He rejected it and chose full compliance.

**This spec therefore ADDS a capability rather than only fixing one.** Full
compliance with §4.2 requires AXFR and IXFR. Ze has no zone transfer of any kind
today. That scope expansion is a decision the owner made knowingly, and it is
recorded here so no later reader mistakes it for scope creep.

**Goal.** Every one of the 27 obligations has a real Ze code path. Every one
carries a positive and a negative test tagged `RFC requirement: <id> <polarity>`.
The stem moves to `rfc/enrolled.txt`. `make ze-rfc-check` stays exit 0.

### Normative register: RFC 1035 predates RFC 2119

RFC 1035 was published in November 1987. RFC 2119 came ten years later. The
document contains **zero capitalised RFC 2119 keywords** and no
requirements-terminology section. Every obligation is lowercase indicative prose.

`rfc/extraction/rfc1035.json` records that reading. Its `register` field is
`prose`. Its `register-reason` field states why. The walk covers all 73 sections
and 31 sites. It is signed off at `signed-off = 2026-07-30`.

A capitalised-keyword scan of `rfc/full/rfc1035.txt` returns nothing. Extraction
under that scan would declare the DNS wire format free of obligations. The prose
register is the only reading under which this document constrains an
implementation. `rfc/short/rfc1035.md` records the same reading.

→ Constraint: all 27 rows are `prose`-register obligations. Do not re-derive them
under a keyword scan, because that scan yields zero rows.

### `RFC1035-3.3.13-1` is superseded by RFC 2308. Do not implement it (2026-08-12)

**The SOA MINIMUM TTL floor MUST NOT be implemented.** RFC 2308 (Standards
Track, March 1998) carries the header `Updates: 1034, 1035`, and its Section 4
withdraws the RFC 1035 Section 3.3.13 meaning of the field:

> "Despite being the original defined meaning, the first of these, the minimum
> TTL value of all RRs in a zone, has never in practice been used and is hereby
> deprecated."

RFC 2308 leaves MINIMUM one meaning, the negative-caching TTL, and geodns
already serves that: `buildSOA` (`internal/plugins/geodns/server.go`) puts the
SOA in the Authority section of a negative answer with `SOA.Minimum` as both its
TTL and its `Minttl`.

The floor WAS implemented on 2026-08-12 and reverted the same day. What it cost
while it stood is worth recording, because it is what the deprecation is about:
the default MINIMUM of 300 raised every record configured below 300 seconds, and
`TestRFC2181_RRSetEqualTTL` (`internal/plugins/geodns/server_test.go`) went red
with `RRSet TTL = 300, want the configured 120`. Ze's own YANG leaf is already
described as `"SOA minimum / negative-cache TTL seconds"`, the RFC 2308 reading.

→ Decision: `hdr` stamps the configured TTL and applies no floor. The reason
lives in its doc comment, so the next reader who finds Section 3.3.13 meets the
update that withdrew it.

→ Constraint: **enrolment status is not a reason to prefer an older RFC.** The
instruction that started this work said to implement Section 3.3.13 "since RFC
1035 is what is enrolled here". That was wrong. `ai/rules/rfc-compliance.md`
puts the RFC TEXT first, and the governing text is the later one.

→ Constraint: no annotation kind can express this, so none was applied. The
reason is a gap in the ledger itself, recorded in its own section below.

### Position on 2026-08-12: 25 of 27 rows proven, 2 open

`rfc/requirements/rfc1035.md` is the measurement. WP-1, WP-3 and the
`RFC1035-2.3.4-1` half of WP-2 landed earlier that day; WP-5, WP-6, WP-7 and
`RFC1035-4.1.3-1` landed in three commits after it. Every one of the 21 rows in
those packages carries a positive and a negative tagged test, and every test was
proven to discriminate by breaking the behaviour it names and watching that
named test go red.

Two obligations had no code path at all and now do. `shapeAuthoritative` holds
the Z field (and the AD bit RFC 4035 took from it) to zero, which no answer func
could be stopped from setting before. `parseConfig` bounds every configured name
to the two size limits of section 3.1, including the `ns<N>.<zone>` glue name it
synthesizes and no config leaf holds; `parseSOA` and the YANG `minimum` leaf
bound the one SOA field that reaches the wire as a TTL.

Two rows remain, and each is a different kind of open.

| Row | Kind | State |
|-----|------|-------|
| `RFC1035-3.3.13-1` | settled, unrecordable | RFC 2308 section 4 withdrew it. Held by `TestRFC2308_NoZoneWideTTLFloor` and by the section above. The ledger has no word for "superseded" |
| `RFC1035-4.2-1` | not implemented | Zone transfer. WP-4, untouched, and the partition above splits cleanly at WP-4a |

### `RFC1035-4.1.1-3` is answered and proven (2026-08-12)

The row was escalated because neither responder emitted RCODE 3 for a name in a
zone it serves, and its only RCODE 3 carried AA=1 for a name it serves no zone
for. Thomas was shown both halves and ruled "fix any issues - the code must be
RFC compliant", so both were implemented.

| Query | Answer now |
|-------|-----------|
| A name a served zone owns, with no record of the type asked for | NOERROR, empty Answer, the zone SOA in the Authority section |
| A name inside a served zone that the zone does not own | RCODE 3, the zone SOA in the Authority section |
| A name under no served zone | RCODE 5, AA clear, nothing in any section |

The existence test is a property of the ZONE, not of the client. Host sets are
chosen by source prefix, so a name configured in one host set and not another has
data for one client and none for another; both clients get NOERROR, because the
name exists either way. `resolverState.names` (`internal/plugins/geodns/state.go`)
holds that set, built once per config generation from each configured host plus
every interior node above one, and `nameExists`
(`internal/plugins/geodns/server.go`) reads it. as112 needs no such set: every
zone it serves holds its records at the apex and nothing below it, so the apex
comparison IS the existence test.

RFC 7534 does not conflict, which is what made the as112 half safe. Its section
3.5 zone files put every record at `@`, and it asks for a "standards-compliant"
authoritative server, which answers RCODE 3 below the apex of such a zone. All
three RFC 7534 rows keep the polarities they had.

The AA decision moved to the RCODE, in `shapeAuthoritative`
(`internal/core/dnsserver/handler.go`), so neither responder special-cases it.

→ Decision: RFC 2308 section 3 is cited in prose rather than tagged. It is not
enrolled and has no summary, so it carries no requirement id.

→ Constraint: `matchZone` in geodns compared characters, so `evilexample.com.`
matched the zone `example.com.` and would have drawn an authoritative answer
where REFUSED is owed. It now uses `dns.IsSubDomain`, as as112 already did, and
`hasZoneSuffix` in the config parser uses the same test so a host the parser
accepts is a host the answer path can place.

### Structural gap: the ledger cannot say "a later RFC withdrew this"

**This is a property of the requirement ledger, not a fact about RFC 1035.** It
recurs for every summary carrying an obligation a later Standards-Track RFC
withdrew, and RFC 1035 is unlikely to be the only one: `Updates:` and
`Obsoletes:` headers are ordinary in the corpus.

`scripts/dev/rfc_requirements.py` holds two closed vocabularies, both validated
at parse time, and neither carries the RFC-to-RFC relation:

| Vocabulary | Values | Why none fits |
|------------|--------|---------------|
| `ANNOTATION_KINDS` (a requirement row in `rfc/short/<stem>.md`) | `not-applicable`, `gap`, `single-polarity` | `{not-applicable}` claims the obligation misses Ze's role. False: Ze is the authoritative server the section addresses. `{gap}` publishes a debt that does not exist, and `check_status_agreement` carries it onto the public Remaining count as an unmet MUST |
| `EXCLUSION_KINDS` (an extraction site in `rfc/extraction/<stem>.json`) | `not-a-requirement`, `binds-another-role`, `duplicate-of`, `cross-document`, `advisory-in-context`, `relocated-to-spec` | `advisory-in-context` says the sentence is advisory in ITS OWN document, which is the wrong axis: the text binds in RFC 1035 and a different document withdrew it. `cross-document` is closest and still says the obligation belongs elsewhere, not that it was retired |

A withdrawn requirement is neither inapplicable, nor owed-and-missing, nor
someone else's. It is settled, and the ledger has no word for settled.

→ Decision: nothing was annotated, and `RFC1035-3.3.13-1` stays the unproven row
it was before this work. Nothing false is published, because rfc1035 is not
enrolled. The absence of the floor is held by a test instead
(`TestRFC2308_NoZoneWideTTLFloor`, `internal/plugins/geodns/server_rfc1035_test.go`).

→ Constraint: adding a `superseded` kind is its own work with its own owner
decision. It would need the superseding RFC's stem and section as required
fields, so the annotation carries its grounds rather than an assertion, and it
would have to be refused for a stem whose superseding document is not in the
repo.

## Work Package Partition

Seven work packages cover all 27 gated rows. The count check is the final row.

| WP | Theme | Requirement IDs | Count |
|----|-------|-----------------|-------|
| WP-1 | Message size and truncation (UDP) | `2.3.4-2`, `4.2.1-1`, `4.2.1-2` | 3 |
| WP-2 | TTL derivation and bounds | `2.3.4-1`, `3.3.13-1`, `4.1.3-1` | 3 |
| WP-3 | Opcode handling and the NOTIMP path | `6.4-1` | 1 |
| WP-4 | Zone transfer over virtual circuits | `4.2-1` | 1 |
| WP-5 | Name encoding and case comparison | `2.3.3-1`, `2.3.3-2`, `3.1-1`, `3.1-2`, `3.1-3`, `3.1-4`, `3.1-5`, `3.1-6` | 8 |
| WP-6 | Header fields and RCODEs | `4.1.1-1`, `4.1.1-2`, `4.1.1-3` | 3 |
| WP-7 | RR fields, compression, transport framing | `4.1.3-2`, `4.1.4-1`, `4.1.4-2`, `4.1.4-3`, `4.1.4-4`, `4.1.4-5`, `4.2.1-3`, `4.2.2-1` | 8 |
| **Total** | | | **27** |

All ids carry the `RFC1035-` prefix. The table drops it for width.

### Which packages carry the two problems

| Problem | Work packages |
|---------|---------------|
| No code path at all | WP-1 (all 3), WP-2 (`3.3.13-1`), WP-3 (`6.4-1`), WP-4 (`4.2-1`) |
| Positive-only candidates | WP-5 (`3.1-1`, `3.1-2`, `3.1-4`), WP-7 (`4.1.3-2`, `4.1.4-2`, `4.1.4-3`) |
| Already conformant, needs tags only | WP-6 (all 3), WP-7 (`4.2.1-3`, `4.2.2-1`) |

### WP-4 is plausibly spec-sized on its own

AXFR and IXFR are a new protocol capability. They need a zone-data source, a
serial comparison, a multi-message response stream, and an access-control
surface. That is larger than the other six packages combined.

**Recommendation: keep WP-4 in this spec but land it in four commits.** Each
commit leaves `make ze-verify` green. WP-4 is the last package implemented, so a
green gate exists at every earlier boundary.

| Commit | Lands | Gate state |
|--------|-------|------------|
| WP-4a | RFC 5936 and RFC 1995 summaries, plus the YANG access-control leaves | green, no wire change |
| WP-4b | AXFR request routing over the existing TCP listener, refusal by default | green, AXFR returns REFUSED |
| WP-4c | AXFR full-zone response stream, both polarity tags | green, AXFR serves |
| WP-4d | IXFR with SOA-only fallback to AXFR, both polarity tags | green, `4.2-1` proven |

If the owner prefers WP-4 as a sibling spec, this partition splits cleanly at
WP-4a. Raise that with him rather than deciding it here.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - where a shared listener harness sits relative to its consumers
  → Constraint: the small-core plus registration pattern means the harness must not gain per-plugin knowledge.
- [ ] `docs/architecture/dns/server-harness.md` - why the authoritative shape lives in one place
  → Decision: `shapeAuthoritative` is a single invariant defined in exactly one place. Truncation and the NOTIMP reply belong at the same altitude, not in each plugin.
- [ ] `docs/architecture/dns/geodns.md` - geodns listener, EDNS0, answer synthesis
  → Constraint: geodns owns answer policy only. The harness owns the wire write.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1035.md` - the 27 gated obligations and the prose-register reading
  → Constraint: §4.2.1 binds **UDP only**. It does not bind TCP, DoT, or DoH.
  → Constraint: §3.3.13 raises a response TTL to `max(RR TTL, SOA MINIMUM)`.
  → Constraint: §6.4 binds **all** name servers, not only ones implementing inverse queries.
- [ ] `rfc/short/rfc2181.md` - TTL bounds that narrow §2.3.4
  → Constraint: the TTL is an unsigned 32-bit value bounded to 0..2147483647.
- [ ] `rfc/short/rfc7871.md` - EDNS0 client subnet, which Ze already reads
  → Decision: Ze reads an inbound OPT but never emits one. So Ze cannot advertise a UDP payload of more than 512 octets.
- [ ] `rfc/short/rfc7534.md` - the AS112 obligations already enrolled
  → Constraint: AS112 zones are SOA-only and NS-only. Their TTLs are fixed constants.
- [ ] `rfc/short/rfc8484.md` - DoH, whose tagged test forbids truncation
  → Constraint: an existing tagged test asserts the DoH path must not set TC. Truncation must be scoped to UDP.
- [ ] `rfc/short/rfc7858.md` - DoT, the other stream transport
  → Constraint: DoT is a stream. The 512-octet bound does not apply to it.

**MUST CREATE before WP-1 and WP-4 design work** (`ai/rules/protocol.md`):

| Stem | Topic | Needed by | State |
|------|-------|-----------|-------|
| `rfc6891.md` under `rfc/short/` | EDNS0, which redefines the UDP payload bound | WP-1 | summary and full text both absent |
| `rfc5936.md` under `rfc/short/` | AXFR, the full-zone transfer | WP-4 | summary and full text both absent |
| `rfc1995.md` under `rfc/short/` | IXFR, the incremental transfer | WP-4 | summary and full text both absent |

→ Constraint: three summaries do not exist yet, and neither do their full texts.
Fetch each into `rfc/full/` and summarise it. Do this before you design WP-1 or WP-4.
`ai/rules/protocol.md` forbids a citation from memory when a summary belongs in
the repo.

**Key insights:** (minimal context to resume after compaction)
- Two DNS responders share one harness. Fix truncation and NOTIMP once, in the harness.
- The TCP listener already exists. AXFR needs routing, not a new listener.
- `Compress = false` makes every response larger than it needs to be. Truncation will fire more often than an operator expects.
- The wire write discards its error. A message that fails to pack vanishes silently.

## Current Behavior (MANDATORY)

**Source files read:** (you must read these BEFORE you write this spec)
- [ ] `internal/core/dnsserver/handler.go` - `Authoritative` wraps an answer func, shapes the reply, and owns the single wire write
- [ ] `internal/core/dnsserver/handler.go` - `_ = w.WriteMsg(msg)`. The only write path. No size accounting, no `Truncate` call, and the returned error is discarded
- [ ] `internal/core/dnsserver/handler.go` - `shapeAuthoritative` sets `Authoritative = true`, `RecursionAvailable = false`, `Compress = false`
- [ ] `internal/core/dnsserver/manager.go` - `bind` opens a UDP and a TCP listener per endpoint
- [ ] `internal/core/dnsserver/manager.go` - the UDP `dns.Server`
- [ ] `internal/core/dnsserver/manager.go` - the TCP `dns.Server`, already bound and serving
- [ ] `internal/core/dnsserver/client.go` - `r.IsEdns0()` reads an inbound OPT for client-subnet only
- [ ] `internal/plugins/geodns/server.go` - `recordRR` emits `rec.TTL` verbatim
- [ ] `internal/plugins/geodns/server.go` - `buildSOA` uses `SOA.Minimum` as the SOA record's own TTL
- [ ] `internal/plugins/geodns/server.go` - `Minttl` carries `SOA.Minimum` into the wire SOA
- [ ] `internal/plugins/geodns/server.go` - `appendNS` synthesizes `ns<N>.<zone>` with no length guard
- [ ] `internal/plugins/geodns/server.go` - geodns builds its handler through `dnsserver.Authoritative`
- [ ] `internal/plugins/geodns/config.go` - `fqdn(z)` appends the trailing dot to each configured zone
- [ ] `internal/plugins/as112/server.go` - as112 builds its handler through the same `dnsserver.Authoritative`
- [ ] `internal/plugins/as112/zones.go` - `soaMinTTL uint32 = 604800`
- [ ] `internal/plugins/as112/zones.go` - `zoneTTL uint32 = 604800`
- [ ] `internal/core/dnsserver/secure_test.go` - `TestDoHIgnoresEDNSUDPSize`, tagged `RFC8484-6-1 positive`, asserts the DoH path must not set TC

**Verified absent** (grep over `internal/`, `pkg/`, `cmd/`):

| Behavior | Evidence |
|----------|----------|
| AXFR, IXFR, or `dns.Transfer` | no match anywhere in production or test code |
| A production `Msg.Truncate` call or a `Truncated` assignment | no match in `internal/core/dnsserver/` or `internal/plugins/geodns/`. The one `Truncated` reference is `secure_test.go`, which asserts the bit is CLEAR |
| `RcodeNotImplemented` or any `Opcode` inspection | no match in either DNS package. An inverse query is not even detected |
| An emitted OPT record, a `SetUDPSize` call, or a 512-octet constant | no match in production code in either DNS package |
| A name-length guard in geodns | no match. The only `len()` test is the `ns<N>` prefix shape at `server.go` |
| Any `RFC1035-` test tag | zero matches across `internal/` |

**Behavior to preserve:** (unless the user explicitly said to change it)
- The authoritative shape re-asserted after the answer func (`handler.go`). It is a security invariant, not a convention.
- `RecursionAvailable = false` on every reply. Ze must never advertise recursion.
- The panic guard that drops a reply rather than crashing the listener (`handler.go`).
- `TestDoHIgnoresEDNSUDPSize` stays green. DoH must not truncate on an advertised UDP size.
- ~~NXDOMAIN for a name outside every served zone. NODATA plus the zone SOA for an in-zone name with no matching record.~~ VOID 2026-08-12: both were the defect. See "`RFC1035-4.1.1-3` is answered and proven".
- AS112 SOA and NS parameters. `TestSOA_RFCMandatedParameters` pins them for RFC 7534.
- The answer func never receives the `ResponseWriter`. It cannot bypass the shaping.

**Behavior to change:** (only what the user asked for)
- A UDP reply of more than 512 octets is truncated and carries TC. Today it is written whole.
- A response RR TTL is raised to `max(RR TTL, zone SOA MINIMUM)`. Today geodns emits the record TTL verbatim.
- An inverse query draws a Not Implemented reply. Today it draws a normal answer attempt.
- AXFR and IXFR are served over TCP, subject to access control. Today neither exists.
- A failed wire write is logged and counted. Today the error is discarded.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A DNS query arrives as a UDP datagram or a TCP stream on a configured listener, port 53 by default. `manager.go` binds both per endpoint.
- A DoT or DoH query arrives on a separate secure listener built by `secure.go`.
- Format at entry: raw DNS wire bytes. `github.com/miekg/dns` parses them into a `dns.Msg` before Ze sees them.

### Transformation Path
1. `miekg/dns` accepts the connection and unpacks the wire bytes into a `dns.Msg`. Malformed input never reaches Ze.
2. `dnsserver.Authoritative` (`handler.go`) installs the panic guard, allocates the reply, and calls `SetReply`.
3. `shapeAuthoritative` (`handler.go`) sets the AA bit, clears recursion, and disables compression.
4. The plugin answer func runs. geodns uses `answerQuery` (`server.go`). as112 uses its own `answerQuery` (`server.go`).
5. The answer func resolves the client through `dnsserver.ClientIP` (`client.go`), which CAN read an inbound EDNS0 client-subnet option.
6. The answer func appends records to `Answer`, `Ns`, and `Extra`, or sets an RCODE.
7. `shapeAuthoritative` runs a second time (`handler.go`), so no answer func can leave the reply non-authoritative or compressed.
8. `handler.go` packs and writes the reply once. The pack error is discarded today.

**New stages this spec inserts.** Stage 3.5 rejects an unsupported opcode with
NOTIMP before any answer func runs. Stage 7.5 applies the transport-aware size
bound and sets TC. Stage 8 gains error handling. WP-4 adds a parallel path from
stage 2 for a zone-transfer request, which streams several messages rather than one.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ `miekg/dns` | wire bytes unpacked to `dns.Msg`, packed back on write | Yes - `manager.go` and `:174` |
| `miekg/dns` ↔ harness | `dns.HandlerFunc(w dns.ResponseWriter, r *dns.Msg)` | Yes - `handler.go` |
| Harness ↔ plugin answer func | `AnswerFunc(msg, r *dns.Msg, p Peer) bool`. `Peer` exposes `RemoteAddr` only | Yes - `handler.go` |
| Harness ↔ transport identity | **not currently crossed.** The answer func and the write path cannot tell UDP from TCP | No - this is the WP-1 gap |
| Plugin ↔ zone data | geodns reads a config snapshot through `loadState()`. as112 uses compile-time constants | Yes - `server.go` and `zones.go` |
| Plugin ↔ metrics | per-plugin counters. The harness never owns metrics | Yes - `server.go` |

→ Constraint: the harness cannot see the transport today. WP-1 must surface it. The
answer func must still not receive the `ResponseWriter`. That would break the
shaping invariant recorded in `docs/architecture/dns/server-harness.md`.

### Integration Points
- `dnsserver.Authoritative` - the single choke point for truncation, the NOTIMP reply, and write-error handling. Both responders funnel through it.
- `dnsserver.Manager.bind` - already owns the TCP listener AXFR needs.
- `dnsserver.Options` - the existing extension seam. A transport hint and a transfer-authoriser hook belong here, registered by the consumer.
- `geodns.recordRR` and `geodns.appendNS` - where the SOA MINIMUM clamp applies.
- `geodns.parseConfig` - where a name that cannot pack must be rejected.
- `internal/core/diagnostic/codes.go` - new doctor codes for the transfer surface.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To fill during design. Truncation must sit at the single write in `handler.go`, not in either plugin. |
| No unintended coupling (components stay isolated) | No | To fill during design. The harness must not learn a plugin name. A transfer authoriser is registered, not switched on. |
| No duplicated functionality (extends existing, does not recreate) | No | To fill during design. WP-4 reuses the bound TCP listener at `manager.go`. It must not open a second one. |
| Zero-copy preserved where applicable (refs, not copies) | No | To fill during design. `Msg.Truncate` mutates in place. A transfer stream must not buffer a whole zone. |
| Registration over hardcoding. New commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core package (`ai/rules/plugins.md`) | No | To fill during design. The transfer authoriser and the zone-data source register through `dnsserver.Options`. No `if plugin == "geodns"` in the harness. |

## Positive-Only Rows: The Polarity Problem

Six rows describe wire-format invariants that `miekg/dns` enforces. The claim on
record is that no Ze-side change can break them, so a negative test is
unreachable by construction. That claim needs verifying per row, and the result
is not uniform.

### What the rules actually require

| Rule | Requirement |
|------|-------------|
| `ai/rules/testing.md` | every gated MUST needs a positive and a negative tagged test |
| `rfc/enrolled.txt` | enrolment needs both polarities, or an annotation of `{not-applicable}`, `{gap}`, or `{single-polarity}` |
| `scripts/dev/rfc_requirements.py` | `{single-polarity}` is a first-class annotation kind |
| `scripts/dev/rfc_requirements.py` | `{single-polarity}` needs an explicit polarity AND a reason |
| `ai/rules/rfc-compliance.md` | every earlier answer pointing away from full compliance or full proof is VOID |

**A correction to the framing that commissioned this spec.**
`ai/rules/rfc-compliance.md` does **not** void `{single-polarity}`. The string
does not appear in that file at all. Its void table names `{gap}`,
`{not-applicable}`, and `partial` only.

`{single-polarity}` is documented at `rfc/enrolled.txt`, validated by the gate,
and already used by roughly twenty enrolled RFCs.
`scripts/dev/testing_health.py` treats a change from one into a test pair as
an improvement. The gate therefore reads it as a weaker but legal state, not as a
void one.

That correction narrows the problem. It does not dissolve it. A
`{single-polarity}` row still proves less than a pair. The owner chose full
compliance. So adopting it for six rows is a decision that lowers what Ze proves,
and it belongs to him.

### The three routes

| Route | What it does | Cost | Honest? |
|-------|--------------|------|---------|
| A. Test at the library boundary | assert Ze output round-trips, and that an input Ze can actually produce is rejected | medium. Needs a reachable malformed input per row | Yes. The negative lands at Ze's seam rather than inside the library |
| B. Narrow the requirement text | keep the id, restrict the wording to what Ze owns, making both polarities expressible | low | Partly. `check_retired_requirements` permits it because the id survives, but a reviewer must see the text change |
| C. Escalate the residue | put the specific ids to Thomas | low | Yes, and required for whatever A and B cannot reach |

### Recommendation: A first, then C for the residue. Not B.

**Route A is viable for more rows than the original claim assumed, and
investigating it already found a real Ze-owned defect.**

`vendor/github.com/miekg/dns/msg.go` defines
`maxDomainNameWireOctets = 255 // See RFC 1035 section 2.3.4`. The library
returns `ErrLongDomain` (`msg.go`) rather than silently truncating. So an
over-long name is a **detectable error at Ze's seam**, not an invariant the
library hides.

Ze can reach that error today:

1. `internal/plugins/geodns/yang/ze-geodns-conf.yang` bounds a zone name with `length "1..255"`. That counts presentation characters, not wire octets.
2. `internal/plugins/geodns/config.go` appends a trailing dot with `fqdn(z)`.
3. `internal/plugins/geodns/server.go` synthesizes `ns<N>.<zone>` with no length guard at all.

A 255-character zone therefore yields a synthesized glue name of more than 255 wire
octets. `miekg/dns` refuses to pack it. Ze then **discards the error**, because
`internal/core/dnsserver/handler.go` reads `_ = w.WriteMsg(msg)`.

The consequence is a silent drop. No log. No metric. No SERVFAIL. That is a
`ai/rules/evidence.md` failure and an `ai/rules/cli.md`
failure in its own right, independent of RFC 1035.

So route A gives `RFC1035-3.1-4` a genuine negative. A config that produces an
unpackable name is rejected at validate time. The write path then reports a pack
failure rather than swallows it. `RFC1035-3.1-1` and `RFC1035-3.1-2` reach the same
seam through the same packer.

**Route B is not recommended.** It lowers the obligation but keeps the id. That is
exactly the shape `ai/rules/rfc-compliance.md` voids. It also hides the change
from a reader who sees only the id.

**Route C carries the residue.** Some rows plausibly have no Ze-reachable
negative even after route A. `RFC1035-4.1.4-2` and `RFC1035-4.1.4-3` govern
compression-pointer emission. `handler.go` sets `Compress = false`, so Ze
never emits a pointer at all. `RFC1035-4.1.3-2` governs RDLENGTH, which the
packer computes with no Ze input.

→ Decision required from Thomas, not from the implementer. Put the surviving ids
to him with the RFC text, the producing `file:line`, and the cost of a pair. Ask
which way he wants each fixed. Never offer to skip one
(`ai/rules/completion.md`).

**A design-time task, before WP-5 and WP-7 code.** Attempt route A per row.
Record the outcome in the table below. Escalate only what survives.

| ID | Original claim | Route A reachable? | Status |
|----|----------------|--------------------|--------|
| `RFC1035-3.1-1` | library owns label encoding | likely, via the packer seam | to verify |
| `RFC1035-3.1-2` | library owns root terminator | likely, via the packer seam | to verify |
| `RFC1035-3.1-4` | library owns the 255-octet bound | **yes, verified.** `nsN.<zone>` synthesis has no guard | confirmed reachable |
| `RFC1035-4.1.3-2` | packer computes RDLENGTH | doubtful | to verify, then escalate |
| `RFC1035-4.1.4-2` | packer computes OFFSET | doubtful. `Compress = false` means no pointer is emitted | to verify, then escalate |
| `RFC1035-4.1.4-3` | packer chooses pointer sites | doubtful. Same reason | to verify, then escalate |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | §4.2.1 binds UDP only, so truncation must not apply to TCP, DoT, or DoH | `rfc/full/rfc1035.txt:1752-1758` is headed "4.2.1. UDP usage" | a naive all-transport bound breaks `TestDoHIgnoresEDNSUDPSize` and cripples DoH | re-read §4.2.1 and §4.2.2, then run `internal/core/dnsserver` tests | unvalidated |
| A-2 | An existing tagged test forbids DoH truncation | `internal/core/dnsserver/secure_test.go`, tagged `RFC8484-6-1 positive`, asserts TC is clear on a reply of more than 512 bytes | WP-1 lands red against a protected RFC-tagged test, which the `rfc-tagged-test` hook guards against any edit | read the test, then confirm it stays green after WP-1 | confirmed by read, re-confirm by run |
| A-3 | Ze never emits an OPT record, so it cannot advertise a payload of more than 512 octets | grep found no `SetUDPSize` and no emitted `dns.OPT` in production code in either DNS package. `client.go` only reads an inbound OPT | if Ze did advertise a larger size, RFC 6891 would move the bound and a flat 512 would be wrong | grep production code again, then read the `rfc6891.md` summary once it exists | confirmed by grep |
| A-4 | The TCP listener AXFR needs already exists | `internal/core/dnsserver/manager.go` binds and serves a TCP `dns.Server` per endpoint | WP-4 grows by a whole listener lifecycle, and WP-4b stops being a small commit | read `bind`, then assert an AXFR request reaches the handler over TCP | confirmed by read |
| A-5 | No existing `.ci` asserts an untruncated large UDP response | the four geodns `.ci` files are `test/ui/doctor-geodns.ci`, `test/parse/geodns-config.ci`, `test/parse/geodns-invalid-record.ci`, `test/plugin/geodns-show.ci`. None issues a live query | WP-1 breaks a functional test, and the fix looks like a regression | read all four, then run the parse, ui, and plugin suites after WP-1 | confirmed by read, re-confirm by run |
| A-6 | as112 already satisfies `RFC1035-3.3.13-1` by constant equality | `internal/plugins/as112/zones.go` set `soaMinTTL` and `zoneTTL` both to 604800, so `max(TTL, MINIMUM)` equals the TTL | the WP-2 clamp must also cover as112, and its RFC 7534 pinned parameters constrain the fix | read the constants, then add a regression test asserting the equality holds | confirmed by read |
| A-7 | `miekg/dns` reports an over-long name rather than truncating it | `vendor/github.com/miekg/dns/msg.go` defines the 255-octet limit. `msg.go` defines `ErrLongDomain`. `msg.go` returns it | route A loses its negative for `RFC1035-3.1-4`, and the residue escalated to Thomas grows | read the vendored packer, then drive a pack failure from a geodns config | confirmed by read |
| A-8 | `Compress = false` makes truncation fire more often than an operator expects | `internal/core/dnsserver/handler.go` disables compression on every reply | the operational impact of WP-1 is smaller than estimated, which is a benign miss | measure a realistic multi-record reply size with and without compression | unvalidated |
| A-9 | The YANG `length "1..255"` bound counts presentation characters, not wire octets | `internal/plugins/geodns/yang/ze-geodns-conf.yang`. RFC 1035 §2.3.4 bounds wire octets, which include a length octet per label | the name-length negative is unreachable and `RFC1035-3.1-4` joins the escalation residue | compute the wire length of a maximal configured zone plus the `ns1.` prefix | unvalidated |
| A-10 | Three RFC summaries this spec needs do not exist yet | `rfc/short/` has no entry for RFC 6891, RFC 5936, or RFC 1995, and `rfc/full/` has no text for any of the three | WP-1 and WP-4 design proceeds on memory, which `ai/rules/protocol.md` forbids | list `rfc/short/` and `rfc/full/` again before you start WP-1 | confirmed by check |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A flat 512-octet bound applied to every transport breaks DoH and DoT | `TestDoHIgnoresEDNSUDPSize` goes red | make the bound transport-aware at the single write. Bound UDP only. A-1 and A-2 gate this |
| R-2 | Surfacing the transport to the write path tempts a design that hands the answer func the `ResponseWriter` | a review notes the answer func can now write the wire itself | keep the write in the harness. Pass a transport enum, never the writer. `docs/architecture/dns/server-harness.md` records why |
| R-3 | The SOA MINIMUM clamp changes cached TTLs for existing geodns operators | a functional test asserting a specific TTL changes value | the clamp only ever raises a TTL, so it cannot shorten caching. Document the change in the geodns guide |
| R-4 | AXFR exposes a whole zone to any client that asks | an AXFR from an unlisted source succeeds in a test | refuse by default. WP-4b lands refusal before WP-4c lands service. Access control is a YANG leaf, not a flag |
| R-5 | WP-4 grows past the spec and strands the other six packages half-proven | WP-4a slips while WP-1 through WP-3 sit unlanded | order WP-4 last. Every earlier commit leaves the gate green, so WP-4 CAN split into a sibling spec and no revert is needed |
| R-6 | Truncation drops the SOA from a negative answer, breaking RFC 2308 caching | a NODATA reply arrives with TC set and an empty Authority section | truncate whole records from the least significant section first. Never emit a partial record. Assert the SOA survives in a negative answer |
| R-7 | Fixing the discarded write error turns silent drops into a flood of logs | log volume rises sharply after the write-error fix | count the failure in a metric and rate-limit the log. A pack failure is a config defect, so also reject it at validate time |
| R-8 | Adding the NOTIMP path changes the reply to a query Ze previously answered | a test asserting an answer to a non-standard opcode changes | gate NOTIMP on the opcode alone. A standard QUERY is unaffected. Assert QUERY still answers |
| R-9 | 27 new tag pairs make `ai/RFC-REQUIREMENTS.md` stale at every commit | `make ze-rfc-check` fails on a stale ledger | run `make ze-rfc-index` and commit the ledger in the SAME commit as each tag change, per `ai/rules/testing.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every DNS answer Ze serves. A wrong truncation bound silently drops records from replies, so resolvers cache partial answers. A wrong TTL clamp changes caching for every zone. A wrong AXFR access decision leaks a whole zone to anyone. |
| How is it reverted? | WP-1 through WP-3 and WP-5 through WP-7 are single-commit reverts with no persisted state. WP-4 adds YANG leaves, so a revert after an operator has configured transfer access needs a config migration. |
| Who else touches this path? | Both DNS responders share the harness, so any change reaches geodns and as112 together. as112 carries enrolled RFC 7534 obligations with pinned SOA parameters. The DoH and DoT paths carry enrolled RFC 8484 and RFC 7858 tagged tests. Concurrent sessions own `internal/component/mcp/`, `internal/test/`, `test/plugin/mcp-*`, `test/plugin/task-*`, and `test/chaos-web/`, none of which this spec touches. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UDP query on port 53 whose reply exceeds 512 octets | → | the transport-aware size bound at the single write in `dnsserver.Authoritative` | `test/plugin/dns-udp-truncation.ci` |
| TCP query on port 53 whose reply exceeds 512 octets | → | the same bound, which must not fire on a stream transport | `test/plugin/dns-tcp-no-truncation.ci` |
| geodns A query for a host whose record TTL is below the zone SOA MINIMUM | → | the clamp in `geodns.recordRR` | `test/plugin/dns-ttl-soa-minimum.ci` |
| A query carrying an unsupported opcode | → | the opcode check in `dnsserver.Authoritative` before any answer func | `test/plugin/dns-inverse-query-notimp.ci` |
| An AXFR request over TCP from an authorised source | → | the transfer handler routed from the existing TCP listener | `test/plugin/dns-axfr-authorised.ci` |
| An AXFR request over TCP from an unauthorised source | → | the transfer authoriser registered through `dnsserver.Options` | `test/plugin/dns-axfr-refused.ci` |
| A geodns config whose synthesized glue name exceeds 255 wire octets | → | the name-length rejection in `geodns.parseConfig` | `test/parse/dns-name-too-long.ci` |
| `ze doctor --json` with a transfer-enabled config | → | the new transfer doctor check | `test/ui/doctor-dns-transfer.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A UDP query whose complete reply would exceed 512 octets | The reply is at most 512 octets and the TC bit is set. Positive and negative tags for `RFC1035-4.2.1-1`, `RFC1035-4.2.1-2`, `RFC1035-2.3.4-2`. |
| AC-2 | A UDP query whose complete reply is at most 512 octets | The reply is complete and TC is clear. This is the negative polarity for AC-1. |
| AC-3 | A TCP, DoT, or DoH query whose reply exceeds 512 octets | The reply is complete and TC is clear. `TestDoHIgnoresEDNSUDPSize` stays green. |
| AC-4 | A UDP reply truncated mid-answer for a negative answer | No partial record is emitted and the zone SOA survives in the Authority section. |
| ~~AC-5~~ | ~~A geodns A record whose configured TTL is below the zone SOA MINIMUM~~ | ~~The emitted TTL equals the SOA MINIMUM. Positive tag for `RFC1035-3.3.13-1`.~~ VOID 2026-08-12: RFC 2308 Section 4 deprecates the floor. See "`RFC1035-3.3.13-1` is superseded by RFC 2308". |
| AC-6 | A geodns A record whose configured TTL is below or above the zone SOA MINIMUM | The emitted TTL equals the record TTL, unchanged, in both cases. No floor is applied. |
| ~~AC-7~~ | ~~An as112 reply~~ | ~~Every emitted TTL is at least the zone SOA MINIMUM.~~ VOID 2026-08-12, same reason. The `soaMinTTL` and `zoneTTL` equality is a coincidence with no obligation behind it. |
| AC-8 | A configured TTL outside 0..2147483647 | The config is rejected at validate time with the offending value named. Positive and negative tags for `RFC1035-2.3.4-1`, `RFC1035-4.1.3-1`. |
| AC-9 | A query carrying an opcode Ze does not support, including an inverse query | The reply carries RCODE 4 Not Implemented. Positive tag for `RFC1035-6.4-1`. |
| AC-10 | A standard QUERY opcode | The query is answered as usual and never draws Not Implemented. This is the negative polarity for AC-9. |
| AC-11 | An AXFR request over TCP from an authorised source | The full zone is streamed, opening and closing with the SOA. Positive tag for `RFC1035-4.2-1`. |
| AC-12 | An AXFR request over UDP | The request is refused, because §4.2 requires a virtual circuit. This is the negative polarity for AC-11. |
| AC-13 | An AXFR or IXFR request from an unauthorised source | The reply is REFUSED and no zone data is emitted. |
| AC-14 | An IXFR request whose client serial is current | An SOA-only reply is returned. An out-of-range serial falls back to a full AXFR. |
| AC-15 | A query name differing only in letter case from a configured zone or host | The answer is identical to the exact-case query. Positive and negative tags for `RFC1035-2.3.3-1`, `RFC1035-3.1-5`, `RFC1035-3.1-6`. |
| AC-16 | A configured name whose wire form exceeds 255 octets, including a synthesized `ns<N>.<zone>` glue name | The config is rejected at validate time and names the offending value. Positive and negative tags for `RFC1035-3.1-4`. |
| AC-17 | A configured label exceeding 63 octets | The config is rejected at validate time. Positive and negative tags for `RFC1035-3.1-3`. |
| AC-18 | A reply that fails to pack for any reason | The failure is logged, counted in a metric, and never a silent drop. `handler.go` no longer discards the error. |
| AC-19 | Any reply Ze emits | The Z field is zero, the AA bit is set, and a name outside every served zone draws RCODE 3. Positive and negative tags for `RFC1035-4.1.1-1`, `RFC1035-4.1.1-2`, `RFC1035-4.1.1-3`. |
| AC-20 | Any reply Ze emits, round-tripped through the packer and unpacker | Every RR survives unchanged, proving label, terminator, RDLENGTH, and pointer handling at Ze's seam. Tags for `RFC1035-3.1-1`, `RFC1035-3.1-2`, `RFC1035-4.1.3-2`, `RFC1035-4.1.4-4`, `RFC1035-4.1.4-5`. |
| AC-21 | An inbound query containing a compression pointer | The query is understood and answered. Positive and negative tags for `RFC1035-4.1.4-5`. |
| AC-22 | The configured listener set | UDP and TCP both bind port 53 by default, and the TCP reply carries the two-octet length prefix. Positive and negative tags for `RFC1035-4.2.1-3`, `RFC1035-4.2.2-1`. |
| AC-23 | Every one of the 27 gated rows | Each carries a positive and a negative tagged test, or a `{single-polarity}` annotation Thomas explicitly authorised, with his answer recorded in the summary. |
| AC-24 | Enrolment | `rfc1035` is removed from `rfc/not-enrolled.txt` and added to `rfc/enrolled.txt` with a reason naming each row's proof. `rfc/extraction/rfc1035.json` stays valid, its `source-sha` still matching `rfc/full/rfc1035.txt`. |
| AC-25 | `make ze-rfc-check` and `make ze-verify` | Both exit 0. `ai/RFC-REQUIREMENTS.md` is regenerated and committed alongside the tag changes. |
| AC-26 | The published status row | `docs/features/rfc-status.md` no longer claims obligations with no code path, and its coverage text carries source anchors to the producing lines. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sends a UDP query for a name with many records | UDP listener → harness → geodns answer → size bound → TC set → 512-octet reply | `test/plugin/dns-udp-truncation.ci` |
| 2 | Retries the same query over TCP because the UDP reply carried TC | TCP listener → harness → geodns answer → no bound → complete reply | `test/plugin/dns-tcp-no-truncation.ci` |
| 3 | Configures a host TTL below the zone SOA MINIMUM and queries it | config → parse → `recordRR` clamp → reply carrying the MINIMUM | `test/plugin/dns-ttl-soa-minimum.ci` |
| 4 | Sends an inverse query to Ze | UDP listener → harness opcode check → Not Implemented reply | `test/plugin/dns-inverse-query-notimp.ci` |
| 5 | Runs a zone transfer from an authorised secondary | TCP listener → transfer handler → authoriser → streamed zone | `test/plugin/dns-axfr-authorised.ci` |
| 6 | Runs a zone transfer from an unlisted host | TCP listener → transfer handler → authoriser → REFUSED | `test/plugin/dns-axfr-refused.ci` |
| 7 | Configures a zone name so long its glue name cannot pack | config → validate → rejection naming the value | `test/parse/dns-name-too-long.ci` |
| 8 | Runs `ze doctor` against a transfer-enabled config | doctor registry → transfer check → JSON verdict | `test/ui/doctor-dns-transfer.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUDPReplyTruncatedAt512` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-1. A UDP reply of more than 512 octets is bounded and carries TC | |
| `TestUDPReplyUnder512NotTruncated` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-2, the negative polarity | |
| `TestStreamTransportNotTruncated` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-3. TCP, DoT, and DoH stay complete | |
| `TestTruncationKeepsSOAInNegativeAnswer` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-4, and R-6 | |
| `TestTruncationEmitsNoPartialRecord` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-4 | |
| `TestUnsupportedOpcodeReturnsNotImplemented` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-9 | |
| `TestQueryOpcodeAnsweredNormally` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-10, the negative polarity | |
| `TestWriteFailureLoggedAndCounted` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-18. The discarded error at `handler.go` | |
| `TestReplyZBitZeroAndAASet` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-19 | |
| `TestReplyRoundTripsThroughPacker` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-20. The route-A library-boundary assertion | |
| `TestInboundCompressionPointerUnderstood` | `internal/core/dnsserver/handler_rfc1035_test.go` | AC-21 | |
| `TestListenersBindPort53WithTCPLengthPrefix` | `internal/core/dnsserver/manager_rfc1035_test.go` | AC-22 | |
| `TestRecordTTLRaisedToSOAMinimum` | `internal/plugins/geodns/server_rfc1035_test.go` | AC-5 | |
| `TestRecordTTLAboveMinimumUnchanged` | `internal/plugins/geodns/server_rfc1035_test.go` | AC-6, the negative polarity | |
| `TestGlueAndNSTTLRaisedToSOAMinimum` | `internal/plugins/geodns/server_rfc1035_test.go` | AC-5 for the `appendNS` path | |
| `TestZoneAndHostMatchFoldsCase` | `internal/plugins/geodns/server_rfc1035_test.go` | AC-15 | |
| `TestNonAlphabeticOctetsMatchExactly` | `internal/plugins/geodns/server_rfc1035_test.go` | AC-15, the exact-match half | |
| `TestConfigRejectsNameOverWireLimit` | `internal/plugins/geodns/config_rfc1035_test.go` | AC-16. Includes the synthesized glue name | |
| `TestConfigAcceptsNameAtWireLimit` | `internal/plugins/geodns/config_rfc1035_test.go` | AC-16, the boundary negative | |
| `TestConfigRejectsLabelOver63` | `internal/plugins/geodns/config_rfc1035_test.go` | AC-17 | |
| `TestConfigRejectsTTLOutOfRange` | `internal/plugins/geodns/config_rfc1035_test.go` | AC-8 | |
| `TestAS112SOAMinimumEqualsZoneTTL` | `internal/plugins/as112/zones_rfc1035_test.go` | AC-7, pinning the A-6 coincidence | |
| `TestAXFROverTCPStreamsFullZone` | `internal/core/dnsserver/transfer_test.go` | AC-11 | |
| `TestAXFROverUDPRefused` | `internal/core/dnsserver/transfer_test.go` | AC-12, the negative polarity | |
| `TestTransferRefusedForUnauthorisedSource` | `internal/core/dnsserver/transfer_test.go` | AC-13 | |
| `TestIXFRCurrentSerialReturnsSOAOnly` | `internal/core/dnsserver/transfer_test.go` | AC-14 | |
| `TestIXFROutOfRangeSerialFallsBackToAXFR` | `internal/core/dnsserver/transfer_test.go` | AC-14 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| UDP reply size | 1..512 octets | 512 | N/A | 513 |
| Label length | 1..63 octets | 63 | 0 | 64 |
| Domain name wire length | 1..255 octets | 255 | 0 | 256 |
| TTL | 0..2147483647 seconds | 2147483647 | N/A | 2147483648 |
| SOA MINIMUM | 0..2147483647 seconds | 2147483647 | N/A | 2147483648 |
| Listener port | 1..65535 | 65535 | 0 | 65536 |
| TCP length prefix | 0..65535 octets | 65535 | N/A | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-udp-truncation` | `test/plugin/dns-udp-truncation.ci` | A UDP query with a large reply returns TC and at most 512 octets | |
| `dns-tcp-no-truncation` | `test/plugin/dns-tcp-no-truncation.ci` | The same query over TCP returns the complete reply | |
| `dns-ttl-soa-minimum` | `test/plugin/dns-ttl-soa-minimum.ci` | A short record TTL is served at the zone SOA MINIMUM | |
| `dns-inverse-query-notimp` | `test/plugin/dns-inverse-query-notimp.ci` | An inverse query returns Not Implemented | |
| `dns-axfr-authorised` | `test/plugin/dns-axfr-authorised.ci` | An authorised secondary transfers the zone | |
| `dns-axfr-refused` | `test/plugin/dns-axfr-refused.ci` | An unlisted host is refused and receives no zone data | |
| `dns-name-too-long` | `test/parse/dns-name-too-long.ci` | A zone whose glue name cannot pack is rejected at validate time | |
| `doctor-dns-transfer` | `test/ui/doctor-dns-transfer.ci` | `ze doctor --json` reports the transfer surface | |

**Mutation-verify each functional test** (`ai/rules/testing.md`).
Disable the producing function, confirm the test flips red, then revert. A
truncation test that passes with truncation disabled guards nothing.

**Vacuity warning** (`ai/rules/interop-and-goal-validation.md`). AC-3, AC-12, and
AC-13 assert an ABSENCE. Deleting the mechanism leaves the same absence. Each
needs a paired presence assertion in the same scenario.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-dns-axfr-bind` | `test/interop/scenarios/` | BIND as secondary | A real secondary completes a transfer from Ze and loads the zone | |
| `NN-dns-truncation-dig` | `test/interop/scenarios/` | `dig` as resolver client | A real client sees TC over UDP and retries successfully over TCP | |
| `NN-dns-ixfr-bind` | `test/interop/scenarios/` | BIND as secondary | A real secondary performs an incremental transfer and falls back to AXFR | |

→ Constraint: WP-4 is a wire-visible protocol feature, so
`ai/rules/interop-and-goal-validation.md` makes an interop test mandatory. A
transfer Ze believes it served but BIND rejects has failed at its only job.

## Files to Modify
- `internal/core/dnsserver/handler.go` - the transport-aware size bound, the opcode check, and write-error handling at the single write
- `internal/core/dnsserver/manager.go` - surface the transport identity to the handler and route a transfer request from the TCP listener
- `internal/plugins/geodns/server.go` - clamp `recordRR` and `appendNS` TTLs to the zone SOA MINIMUM
- `internal/plugins/geodns/config.go` - reject a name whose wire form exceeds 255 octets, including a synthesized glue name
- `internal/plugins/geodns/yang/ze-geodns-conf.yang` - correct the name bound to wire octets and add the transfer access leaves
- `internal/plugins/as112/zones.go` - assert the SOA MINIMUM relationship rather than relying on a constant coincidence
- `internal/core/diagnostic/codes.go` - register the new doctor codes for the transfer surface
- `rfc/short/rfc1035.md` - record each row's proof, and any annotation Thomas authorises
- `rfc/not-enrolled.txt` - remove the `rfc1035` row
- `rfc/enrolled.txt` - add the `rfc1035` row naming each row's proof
- `docs/features/rfc-status.md` - correct the coverage and remaining columns for RFC 1035, with source anchors
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index`

## Files to Create
- `internal/core/dnsserver/transfer.go` - the AXFR and IXFR handler, the authoriser seam, and the response stream
- `internal/core/dnsserver/handler_rfc1035_test.go` - harness-level tagged tests
- `internal/core/dnsserver/manager_rfc1035_test.go` - listener and framing tagged tests
- `internal/core/dnsserver/transfer_test.go` - transfer tagged tests
- `internal/plugins/geodns/server_rfc1035_test.go` - answer-policy tagged tests
- `internal/plugins/geodns/config_rfc1035_test.go` - config-validation tagged tests
- `internal/plugins/as112/zones_rfc1035_test.go` - the AS112 SOA MINIMUM regression test
- `rfc/full/rfc6891.txt` and its `rfc6891.md` summary under `rfc/short/` - EDNS0, needed by WP-1
- `rfc/full/rfc5936.txt` and its `rfc5936.md` summary under `rfc/short/` - AXFR, needed by WP-4
- `rfc/full/rfc1995.txt` and its `rfc1995.md` summary under `rfc/short/` - IXFR, needed by WP-4
- `test/plugin/dns-udp-truncation.ci`
- `test/plugin/dns-tcp-no-truncation.ci`
- `test/plugin/dns-ttl-soa-minimum.ci`
- `test/plugin/dns-inverse-query-notimp.ci`
- `test/plugin/dns-axfr-authorised.ci`
- `test/plugin/dns-axfr-refused.ci`
- `test/parse/dns-name-too-long.ci`
- `test/ui/doctor-dns-transfer.ci`
- `plan/deferrals/fixit-dns-rfc1035-conformance.md` - only if anything is deferred

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/geodns/yang/ze-geodns-conf.yang` for the transfer access leaves. Read `ai/rules/config.md` and `ai/rules/config.md` |
| YANG validation constraints | Yes | correct the name `length` bound to wire octets, and add a `range` to every TTL leaf |
| YANG custom validators | Yes | a `ze:validate` for a packable domain name, since a wire-octet bound is not expressible as a `length` on a presentation string |
| CLI commands/flags | Yes | a `show` view for transfer state, owned by the geodns plugin, never a central verb package |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`. Any new selector uses a typed keyword |
| Editor autocomplete | Yes | automatic for the new enum and range leaves. A zone list needs a `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/plugin/dns-axfr-authorised.ci` and the seven sibling `.ci` files |
| Pipe completeness | Yes | route any new `show` output through `ApplyPipes` per `ai/rules/cli.md` |
| Env var registration | N-A | no `environment/` leaf is added. The transfer surface is operator config, so `ai/rules/config.md` puts it in YANG |
| Doctor check for runtime dependencies | Yes | the transfer surface adds an access-control decision and a listener role. Owning-package check plus `internal/core/diagnostic/codes.go` plus unit and functional tests, per `ai/rules/repo-maintenance.md` |
| Prometheus counters/metrics | Yes | a transfer request counter labelled by outcome, and a pack-failure counter for AC-18 |
| BGP family surface (new SAFI / capability / attribute) | N-A | this spec touches no BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`. Zone transfer is a new capability |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the transfer access leaves |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the transfer `show` view |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` for the new handler |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`. geodns gains a transfer surface |
| 6 | Has a user guide page? | Yes | the geodns guide page gains truncation and transfer sections |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/` gains the transfer message stream and the truncation rule |
| 8 | Plugin SDK/protocol changed? | No | the `AnswerFunc` signature is unchanged. The transport hint rides in `dnsserver.Options`, which is internal, not SDK |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1035.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | No | the eight `.ci` files use existing suites and directives. No new runner or format. Confirm before closure that no `.ci` directive was added |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`. Zone transfer is a feature other daemons list |
| 12 | Internal architecture changed? | Yes | the harness gains transport awareness and a transfer path. Update the dnsserver subsystem doc |
| 13 | Route metadata keys added/changed? | N-A | no route metadata is involved |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the transfer and pack-failure counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | a new command and doctor check change the inventory. Update `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/rfc-status.md` anchors `internal/core/dnsserver/handler.go`. Every anchor to a changed file must be re-verified |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify every geodns config example still validates after the name and TTL bounds tighten |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- surface the transport, register the transfer seam, write failing wiring tests
   - Tests: every row of the Wiring Test table, each failing because the feature is a stub
   - Files: `internal/core/dnsserver/handler.go`, `internal/core/dnsserver/manager.go`, `internal/core/dnsserver/transfer.go`
   - Verify: the transport identity reaches the write path, the transfer authoriser is registered through `dnsserver.Options`, and every wiring test fails for the right reason
2. **Phase: Prerequisite RFC summaries** -- fetch and summarise RFC 6891, RFC 5936, RFC 1995
   - Tests: none. This phase produces reference documents
   - Files: `rfc/full/` texts plus their `rfc/short/` summaries
   - Verify: each summary is a standalone protocol reference with no Ze specifics, per `ai/rules/rfc-compliance.md`
3. **Phase: WP-1 message size and truncation** -- the transport-aware bound and TC
   - Tests: `TestUDPReplyTruncatedAt512`, `TestUDPReplyUnder512NotTruncated`, `TestStreamTransportNotTruncated`, `TestTruncationKeepsSOAInNegativeAnswer`, `TestTruncationEmitsNoPartialRecord`, `dns-udp-truncation`, `dns-tcp-no-truncation`
   - Files: `internal/core/dnsserver/handler.go`
   - Verify: `TestDoHIgnoresEDNSUDPSize` stays green, and R-1 and R-6 are closed
4. **Phase: WP-1 continued, the discarded write error** -- AC-18
   - Tests: `TestWriteFailureLoggedAndCounted`
   - Files: `internal/core/dnsserver/handler.go`
   - Verify: a pack failure is logged and counted, never silently dropped
5. **Phase: WP-3 the NOTIMP path** -- the opcode check before any answer func
   - Tests: `TestUnsupportedOpcodeReturnsNotImplemented`, `TestQueryOpcodeAnsweredNormally`, `dns-inverse-query-notimp`
   - Files: `internal/core/dnsserver/handler.go`
   - Verify: a standard QUERY is unaffected, closing R-8
6. **Phase: WP-2 TTL derivation and bounds** -- the SOA MINIMUM clamp and the range checks
   - Tests: `TestRecordTTLRaisedToSOAMinimum`, `TestRecordTTLAboveMinimumUnchanged`, `TestGlueAndNSTTLRaisedToSOAMinimum`, `TestAS112SOAMinimumEqualsZoneTTL`, `TestConfigRejectsTTLOutOfRange`, `dns-ttl-soa-minimum`
   - Files: `internal/plugins/geodns/server.go`, `internal/plugins/as112/zones.go`, `internal/plugins/geodns/yang/ze-geodns-conf.yang`
   - Verify: the clamp only ever raises a TTL, closing R-3
7. **Phase: WP-5 and WP-7 route-A investigation** -- attempt a reachable negative per positive-only row
   - Tests: `TestReplyRoundTripsThroughPacker`, `TestInboundCompressionPointerUnderstood`, `TestConfigRejectsNameOverWireLimit`, `TestConfigAcceptsNameAtWireLimit`, `TestConfigRejectsLabelOver63`, `TestZoneAndHostMatchFoldsCase`, `TestNonAlphabeticOctetsMatchExactly`, `dns-name-too-long`
   - Files: `internal/plugins/geodns/config.go`, `internal/plugins/geodns/yang/ze-geodns-conf.yang`
   - Verify: fill the route-A table. Escalate the surviving ids to Thomas first, per `ai/rules/rfc-compliance.md`
8. **Phase: WP-6 and WP-7 remaining tags** -- header, RCODE, and transport framing
   - Tests: `TestReplyZBitZeroAndAASet`, `TestListenersBindPort53WithTCPLengthPrefix`
   - Files: test files only. These rows are already conformant
   - Verify: each tag pair fails when the producing behavior is mutated
9. **Phase: WP-4a transfer prerequisites** -- YANG access-control leaves and doctor checks
   - Tests: `doctor-dns-transfer`
   - Files: `internal/plugins/geodns/yang/ze-geodns-conf.yang`, `internal/core/diagnostic/codes.go`
   - Verify: the gate stays green and no wire behavior changes
10. **Phase: WP-4b transfer routing, refused by default** -- AC-13
    - Tests: `TestTransferRefusedForUnauthorisedSource`, `TestAXFROverUDPRefused`, `dns-axfr-refused`
    - Files: `internal/core/dnsserver/transfer.go`, `internal/core/dnsserver/manager.go`
    - Verify: AXFR reaches the handler over TCP and is refused, closing R-4
11. **Phase: WP-4c AXFR full-zone response** -- AC-11
    - Tests: `TestAXFROverTCPStreamsFullZone`, `dns-axfr-authorised`, `NN-dns-axfr-bind`
    - Files: `internal/core/dnsserver/transfer.go`
    - Verify: BIND loads the transferred zone
12. **Phase: WP-4d IXFR** -- AC-14
    - Tests: `TestIXFRCurrentSerialReturnsSOAOnly`, `TestIXFROutOfRangeSerialFallsBackToAXFR`, `NN-dns-ixfr-bind`
    - Files: `internal/core/dnsserver/transfer.go`
    - Verify: `RFC1035-4.2-1` carries both polarities
13. **Phase: Enrolment and documentation** -- AC-24, AC-25, AC-26
    - Tests: `make ze-rfc-check`, `make ze-rfc-index`, `make ze-doc-test`, `make ze-verify`
    - Files: `rfc/short/rfc1035.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`, `ai/RFC-REQUIREMENTS.md`
    - Verify: the stem is enrolled and every gate exits 0

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line`, and all 27 rows appear in the enrolment reason |
| Feature completeness | Every user story has a working path. A client that sees TC can retry over TCP and get the whole answer |
| Correctness | Truncation removes whole records only, never a partial RR. The SOA survives a truncated negative answer |
| Correctness | The TTL clamp raises, never lowers. `max(RR TTL, MINIMUM)` is not `MINIMUM` |
| Correctness | The transport bound fires on UDP only. DoT, DoH, and TCP are untouched |
| Correctness | An over-long name is rejected at config validate time, not discovered at pack time |
| Naming | New YANG leaves are kebab-case with unit-free names plus a `units` statement, per `ai/rules/config.md` |
| Naming | New JSON keys are kebab-case, and each metric name matches the documented one |
| Data flow | Truncation lives at the single write in the harness. Neither plugin implements it |
| Data flow | The answer func still never receives the `ResponseWriter`. R-2 stays closed |
| Registration over hardcoding | The transfer authoriser and zone-data source register through `dnsserver.Options`. The harness contains no plugin name and no per-plugin switch case |
| Rule: `ai/rules/rfc-compliance.md` | No row is annotated `{gap}`, `{not-applicable}`, `partial`, or `{single-polarity}` without Thomas's explicit answer recorded in the summary |
| Rule: `ai/rules/evidence.md` | The transfer authoriser denies on a miss, an empty allow-list, or an error. A zero value is never a valid-looking allow |
| Rule: `ai/rules/testing.md` | `ai/RFC-REQUIREMENTS.md` is regenerated in the same commit as every tag change |
| Rule: `ai/rules/testing.md` | Each `.ci` was mutation-verified. Disabling the producing function flips it red |
| Rule: `ai/rules/plugins.md` | Deleting geodns removes its transfer surface, its command, and its doctor check, leaving as112 and the harness working |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| All 27 rows carry tags or an authorised annotation | `grep -c 'RFC1035-' internal/ -r` against the 27-row partition table |
| The stem is enrolled | `grep '^rfc1035' rfc/enrolled.txt` returns a row, and the same grep on `rfc/not-enrolled.txt` returns nothing |
| The extraction sign-off is still valid | `make ze-rfc-check` exits 0, confirming `source-sha` still matches `rfc/full/rfc1035.txt` |
| The requirement ledger is fresh | `make ze-rfc-index` produces no diff |
| Truncation is reachable from a user entry point | `make ze-plugin-test` runs `dns-udp-truncation` green, and mutation-verify flips it red |
| Zone transfer interoperates | the BIND interop scenario loads the transferred zone |
| No obligation is left unproven | `make ze-rfc-check` exits 0 with `rfc1035` in scope |
| Documentation matches the code | `make ze-doc-test` exits 0 |
| The pre-commit gate is green | `make ze-verify` exits 0 |
| Prose passes the style gate | `make ze-ste-review-changed` reports nothing on the files this spec adds |

### Security Review Checklist

A DNS server is an amplification surface and, once it serves transfers, a
disclosure surface. Both are in scope here.

| Check | What to look for |
|-------|-----------------|
| Amplification: response size | Truncation is a mitigation, not just a conformance fix. An unbounded UDP reply lets a spoofed query generate a large response to a victim. Confirm the bound applies to every UDP reply, including a refusal and an error reply |
| Amplification: compression off | `handler.go` sets `Compress = false`, so every reply is larger than it needs to be. Quantify the amplification factor. Decide whether enabling compression for UDP is a separate, owner-visible decision rather than a silent change |
| Amplification: the TC path | A truncated reply must not invite a larger retry over UDP. Confirm a client is steered to TCP |
| Zone disclosure: default posture | AXFR must be refused unless explicitly authorised. Confirm an absent, empty, or malformed allow-list denies. `ai/rules/evidence.md` forbids a zero value that reads as allow |
| Zone disclosure: scope of a grant | An authorisation for one zone must not transfer another. Confirm the authoriser receives the zone, not only the source address |
| Zone disclosure: what a zone reveals | A transferred zone lists every host name. Document that enabling transfer publishes the whole namespace to the authorised peer |
| Resource exhaustion: transfer streams | A transfer holds a TCP connection and streams many messages. Bound the concurrent transfer count and the per-transfer duration. Confirm a slow reader cannot pin memory by forcing a whole zone to buffer |
| Resource exhaustion: the opcode path | The NOTIMP reply must be cheap. Confirm it is produced before any zone lookup or client resolution |
| Input validation: names from config | A name that cannot pack must be rejected at validate time with the offending value named. Confirm the error names the value, per `ai/rules/cli.md` |
| Input validation: names from the wire | `miekg/dns` rejects a malformed query before Ze sees it. Confirm no new path bypasses that parse |
| Error leakage | A refusal must not reveal whether a zone exists, whether an allow-list is configured, or how many entries it has. Confirm REFUSED carries no zone data and no diagnostic detail |
| Failure visibility | AC-18 turns a silent drop into a logged and counted failure. Confirm the log is rate-limited so it cannot itself become a denial-of-service vector, per R-7 |
| Authorisation cannot fail open | Read the authoriser's miss, error, and empty-set branches. Each must deny. Drive the test from the query entry point, never the helper alone |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, go back to RESEARCH |
| `TestDoHIgnoresEDNSUDPSize` goes red | R-1 has fired. The bound is not transport-aware. Return to phase 3 |
| A tagged test for another RFC goes red | STOP. The `rfc-tagged-test` hook governs it, and only the user CAN authorise a change |
| A positive-only row has no reachable negative after route A | Escalate that id to Thomas with the RFC text and the producing `file:line`. Do not annotate it unilaterally |
| Lint failure | Fix inline. If architectural, go back to DESIGN |
| Functional test fails | Check the AC. A wrong AC goes to DESIGN, a correct AC goes to IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| WP-4 exceeds its four commits | Raise the WP-4a split with the owner. Do not silently drop IXFR |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- **Two responders, one harness.** `internal/plugins/geodns/server.go` and `internal/plugins/as112/server.go` both build their handler through `dnsserver.Authoritative`. Truncation, the opcode check, and write-error handling therefore belong in the harness. One fix serves both surfaces, which matches the existing `shapeAuthoritative` design.
- **The TCP listener already exists.** `internal/core/dnsserver/manager.go` binds and serves a TCP `dns.Server` per endpoint. AXFR needs request routing, not a new listener. This shrinks WP-4 materially.
- **`Compress = false` enlarges every reply.** `internal/core/dnsserver/handler.go` disables compression. Truncation will therefore fire on smaller record sets than an operator would predict from a compressed baseline. This is a conformance interaction and an amplification consideration at once.
- **The write path discards its error.** `internal/core/dnsserver/handler.go` reads `_ = w.WriteMsg(msg)`. A reply that fails to pack vanishes with no log, no metric, and no SERVFAIL. This is an independent defect. It surfaced when this spec checked the positive-only claim.
- **The YANG name bound measures the wrong thing.** `internal/plugins/geodns/yang/ze-geodns-conf.yang` uses `length "1..255"` on a presentation string. RFC 1035 §2.3.4 bounds wire octets, which include one length octet per label. `internal/plugins/geodns/server.go` then synthesizes `ns<N>.<zone>` with no guard at all, so a maximal zone yields an unpackable glue name.
- **AS112 conformance for the MINIMUM rule is a coincidence.** `internal/plugins/as112/zones.go` set `soaMinTTL` and `zoneTTL` both to 604800. `max(TTL, MINIMUM)` equals the TTL only because the two constants happen to be equal. A future edit to either breaks conformance silently, so the equality needs a regression test.
- **`{single-polarity}` is legal, not void.** It is defined at `rfc/enrolled.txt`, validated at `scripts/dev/rfc_requirements.py`, and used by roughly twenty enrolled RFCs. `ai/rules/rfc-compliance.md` never mentions it. The framing that it is void does not hold. It still proves less than a pair, so the owner decides whether to accept it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Truncate at the single write in the harness | Truncate in each plugin's answer func | The harness already owns the wire write and the authoritative shape. Two copies of a size bound would drift, and one plugin CAN omit it. Matches `docs/architecture/dns/server-harness.md` |
| Make the bound transport-aware | Apply 512 octets to every transport | §4.2.1 is headed "UDP usage" and binds UDP only. A flat bound would break the enrolled RFC 8484 tagged test and cripple DoT and DoH |
| Pass a transport enum, never the `ResponseWriter` | Give the answer func the writer so it can decide | Withholding the writer is the invariant that stops an answer func bypassing the shaping. Handing it over to solve a size problem would trade a security property for convenience |
| Reject an unpackable name at config validate time | Discover it at pack time and answer SERVFAIL | `ai/rules/protocol.md` requires a config Ze cannot serve exactly to fail at verify. An operator learns at commit, not from a resolver timeout |
| Refuse transfers by default, authorise explicitly | Serve transfers to any client, as some daemons once did | A zone transfer publishes the whole namespace. `ai/rules/evidence.md` requires the deny path on a miss or an empty set |
| Route A before route C for positive-only rows | Annotate all six `{single-polarity}` immediately | Investigating route A already found a reachable negative and a real defect. Annotating first would have hidden both |
| Reject route B outright | Narrow each requirement's text to what Ze owns | It keeps the id but lowers the obligation, which is the shape `ai/rules/rfc-compliance.md` voids. It also hides the change from a reader who sees only the id |
| Keep WP-4 in this spec, landed in four commits | Split WP-4 into a sibling spec now | The owner asked for full compliance including transfer. Splitting it invites the transfer half to stall. The four-commit shape keeps the gate green throughout and still splits cleanly at WP-4a if he prefers |

## Known Limitations
- The three prerequisite RFC summaries do not exist yet. Design work on WP-1 and WP-4 is blocked until they are written, per `ai/rules/protocol.md`.
- This spec raises compression for UDP replies as a security and conformance observation. It does not decide it. Compression changes wire output on every query, so it needs an owner decision.
- Any positive-only row that survives route A is escalated to Thomas rather than annotated. Nothing in this spec authorises a `{single-polarity}` row.
- The 6 SHOULD-level and RECOMMENDED rows in `rfc/short/rfc1035.md` are not gated and are out of scope. Enrolment does not require them.
- Anything genuinely deferred lands in `plan/deferrals/fixit-dns-rfc1035-conformance.md` with a destination spec. No RFC obligation is deferred until Thomas answers first.

## RFC Documentation (Scope: protocol)

Add `// RFC 1035 Section X.Y: "<quoted requirement>"` above every enforcing site.
Quote the indicative prose verbatim, because this document has no capitalised
keywords to quote instead.

| Site | Comment must quote |
|------|--------------------|
| the UDP size bound | §4.2.1 "Messages carried by UDP are restricted to 512 bytes (not counting the IP or UDP headers)." |
| the TC bit | §4.2.1 "Longer messages are truncated and the TC bit is set in the header." |
| the TTL clamp | §3.3.13 "Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA." |
| the NOTIMP reply | §6.4 "While inverse query support is optional, all name servers must be at least able to return the error response." |
| the transfer path | §4.2 "Zone refresh activities must use virtual circuits because of the need for reliable transfer." |
<!-- ste: ignore -->
| the name-length guard | §3.1 "the total length of a domain name (i.e., label octets and label length octets) is restricted to 255 octets or less" |
| the label-length guard | §3.1 "The high order two bits of every length octet must be zero, and the remaining six bits of the length field limit the label to 63 octets or less." |

Every MUST-level site also carries its `RFC requirement: <id> <polarity>` tag on
the enforcing test, not on the production code.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-26 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] All 27 gated rows carry both polarities, or an annotation Thomas explicitly authorised
- [ ] `rfc1035` moved from `rfc/not-enrolled.txt` to `rfc/enrolled.txt`
- [ ] `make ze-rfc-check` exits 0 with `rfc1035` in scope
- [ ] `make ze-rfc-index` produces no diff
- [ ] `make ze-verify` passes, which is the pre-commit gate in `ai/rules/git-safety.md`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### Quality Gates
- [ ] Security Review Checklist answered, including the amplification and zone-disclosure rows
- [ ] Every `.ci` mutation-verified: disabling the producing function flips it red
- [ ] Every absence-asserting AC paired with a presence assertion
- [ ] `make ze-lint-changed` clean
- [ ] `make ze-doc-test` exits 0
- [ ] `make ze-ste-review-changed` reports nothing on new prose
- [ ] No `.ci` sleep added without a justifying comment

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
