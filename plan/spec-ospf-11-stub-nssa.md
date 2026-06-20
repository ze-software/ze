# Spec: ospf-11-stub-nssa

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-9-inter-area-abr.md, spec-ospf-10-as-external-asbr.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - umbrella `## Shared Contracts (canonical)`: "LSA inventory" (Type 5 AS-External not flooded into stub/NSSA; Type 7 NSSA-LSA scope = NSSA area only, originated here), "LSA header + body layout" (Type 7 shares the Type 5 body layout; the NSSA P-bit rides in the LSA-header Options field; Forwarding Address + External Route Tag fields), "Route preference / path types" (intra > inter > E1 > E2, resolved INSIDE OSPF SPF, one winning `locrib.Path` per prefix at AdminDistance 110), "Area + interface config model" (`area-type` enum `normal`/`stub`/`nssa`, `no-summary`, `default-cost`, `ranges`), this spec OWNS exactly `ze_ospf_nssa_translations_total{area}`, and the ospf-11 Child Specs / Dependency Graph rows (depends on ospf-9 and ospf-10)
4. `docs/research/ospf-implementation-guide.md` §6f "Stub, Totally Stubby, and NSSA Areas" (lines 439-455), trap #9 "NSSA Translator Election" (lines 1480-1482), trap #11 "Hello E-bit Mismatch in Stub Areas" (lines 1488-1490)
5. `rfc/short/rfc3101.md` (to create via `/ze-rfc` at implementation time) - NSSA: §2 Type 7 LSA + P-bit + N-bit, §2.5 route-selection preference (Type 7 P=1 > Type 5 > Type 7 P=0), §3.5 translator election (highest Router ID), §3.6 Type 7→5 translation rules
6. `rfc/short/rfc2328.md` (to create) - §3.6 stub areas (E-bit clear, no Type 4/5, ABR default), §12.4.3 Summary-LSA default origination, §A.4.5 AS-External-LSA body (shared by Type 7)
7. `plan/spec-ospf-9-inter-area-abr.md` - the ABR + Type 3/4 Summary-LSA + area-range machinery this spec extends for stub default injection and totally-stubby/totally-NSSA Type 3 suppression
8. `plan/spec-ospf-10-as-external-asbr.md` - the Type 5 AS-External origination + external route computation (§16.4, E1/E2, forwarding address) + redistribution consumer this spec reuses for Type 7 origination and Type 7→5 translation
9. `internal/component/ospf/spf/` and `internal/component/ospf/lsdb/` - the SPF route computation and LSDB origination packages this spec adds NSSA logic to (created by ospf-8/ospf-9/ospf-10)

## Task

Add OSPFv2 stub-area and Not-So-Stubby-Area (NSSA) support to Ze, completing the
area-type hierarchy declared in the umbrella (`plan/spec-ospf-0-umbrella.md`,
"Area types in v1? Normal + stub + NSSA"). This is the last route-computation
child before authentication (ospf-12) and CLI/interop (ospf-13). It depends on
the ABR machinery from ospf-9 (Type 3/4 Summary-LSA origination, area ranges) and
the ASBR machinery from ospf-10 (Type 5 AS-External origination, §16.4 external
route computation with E1/E2 + forwarding address, redistribution consumer),
because NSSA is "external redistribution at an ABR/ASBR inside an area that
forbids transit Type 5", which is exactly those two features composed under
RFC 3101 constraints.

**Stub areas (RFC 2328 §3.6).** A stub area carries no AS-External information.
Concretely: (1) the E-bit (external-routing-capability) in the Options field of
every Hello and Router-LSA originated within the area is CLEAR; (2) Type 5
AS-External-LSAs (from ospf-10) and Type 4 ASBR-Summary-LSAs (from ospf-9) are
NOT flooded into the area and NOT accepted from it; (3) each ABR attached to the
stub area originates a single Type 3 Summary-LSA for the default destination
`0.0.0.0/0` with metric = the configured `default-cost`, so stub routers reach
the rest of the world via the nearest ABR. A Hello whose E-bit does not match the
receiving interface's area E-capability MUST be discarded (adjacency never forms;
guide trap #11). **Totally-stubby** (`no-summary` on a stub area, the FRR
"no-summary" / RFC-implied vendor extension): the ABR additionally SUPPRESSES all
Type 3 Summary-LSAs into the area EXCEPT the injected default, so a spoke site
holds only intra-area routes plus one default.

**NSSA (RFC 3101).** An NSSA is a stub-like area that additionally PERMITS local
external redistribution: an ASBR inside the NSSA may originate Type 7 NSSA-LSAs
for routes it redistributes, but transit Type 5 AS-External-LSAs from the rest of
the AS are still blocked (as in a stub). Concretely: (1) the N-bit (NSSA
capability) in the Hello Options MUST match between neighbours or the adjacency
does not form (guide trap #11; the E-bit is clear in an NSSA exactly as in a stub
because Type 5 is still absent); (2) an NSSA ASBR originates Type 7 NSSA-LSAs
(LS Type 7), which share the Type 5 AS-External body layout (Network Mask, E-bit
+ metric, Forwarding Address, External Route Tag) and additionally carry the
P (Propagate) bit in the LSA-header Options field; the Forwarding Address MUST be
set to a non-zero, intra-NSSA-reachable address when translation is desired
(§2.3); (3) Type 7 LSAs are flooded ONLY within the NSSA; (4) the elected
translator ABR re-originates each P=1 Type 7 as a Type 5 onto the backbone
(below). NSSA route computation extends ospf-10's §16.4 with the RFC 3101 §2.5
preference order when the same external prefix is known both ways: a Type 7 with
P=1 is preferred over a Type 5, which is preferred over a Type 7 with P=0.

**NSSA translator election (RFC 3101 §3.5; guide trap #9).** Among the ABRs
attached to a given NSSA, exactly ONE is elected the Type 7→Type 5 translator:
the candidate ABR with the HIGHEST Router ID that is configured/able to translate
(the per-area translator-role: `candidate`/`always`/`never`, default
`candidate`). The election is sticky (re-elect only on candidate-set change),
analogous to DR election, with hysteresis (a `stability-interval`, default 40 s
per RFC 3101 §3.5) so a transient flap of the current translator does not cause
churn. If the elected translator loses candidacy (config change, ABR down, or
loss of backbone attachment) a new translator is elected and re-floods the
Type 5s. Failing to implement the election (every NSSA ABR translating) injects
DUPLICATE Type 5 LSAs into the backbone (guide trap #9).

**Type 7→Type 5 translation (RFC 3101 §3.6).** At the elected translator only,
each NSSA Type 7 LSA with P=1 and a non-zero Forwarding Address is re-originated
as a Type 5 AS-External-LSA onto the backbone (AS-wide scope): the P-bit is
CLEARED, the Advertising Router is set to the translator's own Router ID, the
Forwarding Address is PRESERVED (so the backbone routes traffic back to the real
NSSA ASBR), the metric / E-bit / External Route Tag are preserved, and the LS ID
is preserved (the external network address) unless an LS-ID collision forces the
RFC 3101 §3.2 range-selection. A Type 7 with P=0 (or a zero Forwarding Address)
is NOT translated and stays local to the NSSA. Translation increments
`ze_ospf_nssa_translations_total{area}` (the one metric this spec owns).

**NSSA default handling.** The NSSA ABR MAY originate a Type 7 default
(`0.0.0.0/0`) into the NSSA (so internal routers reach external destinations via
the ABR), governed by per-area config; with `no-summary` (totally-NSSA) the ABR
additionally suppresses Type 3 summaries except the default, exactly as for
totally-stubby. The NSSA default is a Type 7 (not Type 3) because it carries the
external semantics; it is NOT translated to Type 5 (a default originated by an
ABR has P=0 / is filtered from translation per RFC 3101 §2.3).

All of this is route-computation + LSA-origination/flooding-policy logic layered
on the existing per-area LSDB (ospf-7), SPF route table (ospf-8), ABR summaries
(ospf-9), and ASBR externals (ospf-10). No new wire format is introduced (the
Type 7 body and the N/P/E option bits are codec-owned by ospf-2). Packages
touched: `internal/component/ospf/lsdb/` (Type 7 origination, E/N-bit Hello
policy, default injection, totally-stubby/NSSA Type 3 suppression, translator
election + Type 7→5 translation) and `internal/component/ospf/spf/` (RFC 3101
§2.5 preference in external route computation; stub/NSSA flood-acceptance
filters).

## Required Reading

### Architecture Docs
- [ ] `docs/research/ospf-implementation-guide.md` §6f "Stub, Totally Stubby, and NSSA Areas" (lines 439-455) - stub area E-bit clear + ABR default; totally-stubby Type 3 suppression; NSSA Type 7 + translator; P-bit and the §2.5 preference tiers
  → Decision: model the area-type as a policy that gates LSA flooding-acceptance and origination at the area/interface, not as a special LSDB; the LSDB stays the ospf-7 store and this spec adds accept/originate filters
  → Constraint: in a stub/NSSA, the E-bit in Hellos and self Router-LSAs is CLEAR and Type 4/5 are neither flooded in nor accepted; a mismatched E-bit (stub) or N-bit (NSSA) Hello is discarded (no adjacency)
  → Constraint: external route selection MUST honour RFC 3101 §2.5: Type 7 with P=1 > Type 5 > Type 7 with P=0; a naive equal treatment picks the wrong next hop
- [ ] `docs/research/ospf-implementation-guide.md` trap #9 "NSSA Translator Election" (lines 1480-1482) - one ABR (highest Router ID) translates; forgetting it duplicates Type 5
  → Constraint: elect exactly one translator per NSSA; if every ABR translates, the backbone gets duplicate Type 5 LSAs
- [ ] `docs/research/ospf-implementation-guide.md` trap #11 "Hello E-bit Mismatch in Stub Areas" (lines 1488-1490) - E-bit clear in stub, N-bit set in NSSA; mismatch silently blocks adjacency
  → Constraint: validate the Options E-bit (and the N-bit for NSSA) on every received Hello against the receiving interface's area type before the adjacency forms
- [ ] `plan/spec-ospf-9-inter-area-abr.md` - ABR detection, Type 3/4 Summary-LSA origination, area ranges, backbone-attachment rule
  → Constraint: stub default injection and totally-stubby/NSSA Type 3 suppression are policy layered on the ospf-9 Summary-LSA originator; this spec does not re-implement summary origination, it filters/extends it
- [ ] `plan/spec-ospf-10-as-external-asbr.md` - Type 5 origination, §16.4 external route computation (E1/E2 + forwarding address), redistribution consumer
  → Constraint: Type 7 origination reuses the ospf-10 redistribution-consumer route source and the Type 5 body builder (the bodies are identical); §16.4 external computation is extended with the §2.5 Type 7/Type 5 preference; the translator re-uses the Type 5 originator to flood the translated LSA
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md` - self-contained component
  → Constraint: all stub/NSSA policy, election, and translation live under `internal/component/ospf/` (lsdb/ and spf/); no NSSA spelling leaks into generic packages
- [ ] `ai/rules/buffer-first.md` - zero-copy LSDB
  → Constraint: Type 7→5 translation rewrites only the changed header/option fields (P-bit, Advertising Router, LS Age reset, recompute Fletcher checksum) and re-emits via the buffer-first LSA writer; the body bytes (mask, metric, forwarding address, tag) are copied verbatim

### RFC Summaries (MUST for protocol work; created via `/ze-rfc` at implementation time)
- [ ] RFC 3101 short summary (`rfc/short/rfc3101.md`, to create) - The OSPF NSSA Option
  → Constraint: §2 Type 7 LSA format (LS Type 7, body = Type 5 body, P-bit in the Options field, N-bit in Hello/DD Options); §2.3 Forwarding Address rules (non-zero, intra-NSSA reachable, for translation); §2.5 route-selection preference Type 7 P=1 > Type 5 > Type 7 P=0; §3.5 translator election (highest Router ID candidate ABR, sticky, stability interval); §3.6 Type 7→5 translation (clear P-bit, set Advertising Router to translator, preserve Forwarding Address / metric / tag, AS-wide flood); §2.3 a P=0 or zero-FA Type 7 is not translated
- [ ] RFC 2328 short summary (`rfc/short/rfc2328.md`, to create) - OSPF Version 2 base
  → Constraint: §3.6 stub areas (E-bit clear, Type 4/5 absent, single ABR default at `default-cost`); §12.4.3 the ABR default Summary-LSA; §A.4.5 AS-External-LSA body shared by Type 7; the Options E-bit semantics

**Key insights:** (minimal context to resume after compaction)
- Area type is a flooding/origination POLICY over the existing ospf-7 LSDB and ospf-8 route table, not a new datastore. Stub = clear E-bit + drop Type 4/5 in/out + ABR injects Type 3 default at `default-cost`. Totally-stubby (`no-summary`) = also suppress Type 3 except the default.
- NSSA = stub that ALSO allows local Type 7 origination (external bodies identical to Type 5) flooded only within the NSSA, with the P-bit in the LSA-header Options and the N-bit (not E-bit) negotiated in Hellos.
- Exactly ONE ABR per NSSA translates Type 7→5 onto the backbone: highest Router ID among translate-capable candidates, sticky with a stability interval; forgetting this duplicates Type 5 (trap #9).
- Translation clears the P-bit, sets Advertising Router = translator, PRESERVES the Forwarding Address (and metric/tag/LS-ID), resets LS Age, recomputes the Fletcher checksum, and floods AS-wide via the ospf-10 Type 5 originator. P=0 or zero-FA Type 7 is not translated.
- External route selection adds RFC 3101 §2.5 preference (Type 7 P=1 > Type 5 > Type 7 P=0) ON TOP of the §16.4 E1/E2 + forwarding-address resolution from ospf-10; the umbrella preference contract (intra > inter > external, one winning `locrib.Path` at AdminDistance 110) is unchanged.
- NSSA ABR may originate a Type 7 default (not Type 3) into the NSSA; totally-NSSA suppresses Type 3 except a default. This NSSA-default Type 7 is not translated.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec; packages created by ospf-7/8/9/10)
- [ ] `internal/component/ospf/lsdb/lsdb.go` - the per-area LSDB store + self-LSA origination (Router-LSA, Network-LSA) + the §13 flooding procedure (ospf-7), and the ABR Type 3/4 Summary-LSA originator + area ranges (ospf-9), and the ASBR Type 5 AS-External originator + redistribution consumer (ospf-10) live under `internal/component/ospf/lsdb/`
  → Constraint: this spec adds, in the same package, the Type 7 originator (reusing the Type 5 body builder), the E/N-bit Hello/Router-LSA option policy, the stub default + totally-stubby/NSSA Type 3 suppression (filtering the ospf-9 summary originator), and the translator election + Type 7→5 translation (reusing the ospf-10 Type 5 originator); it does NOT rewrite the LSDB store or the flooding procedure
- [ ] `internal/component/ospf/spf/` - intra-area SPF + route table with path types (ospf-8), inter-area route computation from summaries (ospf-9), external route computation §16.4 with E1/E2 + forwarding address (ospf-10)
  → Constraint: this spec adds the RFC 3101 §2.5 Type 7/Type 5 preference into the external route-computation step and the stub/NSSA accept-filter (do not compute Type 5 routes inside a stub/NSSA); the intra > inter > external preference and the single-winning-`locrib.Path` install model are unchanged
- [ ] `internal/component/ospf/iface/` - the ISM + Hello send/receive + header validation (ospf-5)
  → Constraint: this spec adds the E-bit/N-bit set-on-send and match-on-receive into the existing Hello validation; the ISM and DR/BDR election are unchanged
- [ ] `internal/component/ospf/yang/ze-ospf-conf.yang` + config resolve (ospf-4) - the `areas/area` container already carries `area-type` (`normal`/`stub`/`nssa`), `no-summary`, `default-cost`, `ranges` per the umbrella "Area + interface config model"
  → Constraint: the area-type schema EXISTS (umbrella schema owner ospf-4); this spec adds only the NSSA translator-role leaf (`translate-role`: `candidate`/`always`/`never`, default `candidate`) and the NSSA default-originate leaf if not already present, and gives the existing leaves runtime meaning. Confirm during the audit whether ospf-4 already shipped these leaves; if so this spec adds none

**Behavior to preserve:**
- The per-area LSDB store and the §13 flooding procedure (ospf-7) are unchanged; area type only gates which LSA types are accepted/flooded/originated.
- The ABR Type 3/4 Summary-LSA originator and area ranges (ospf-9) are unchanged for normal areas; stub/NSSA add a default and a suppression filter on top.
- The Type 5 AS-External originator, the §16.4 external route computation, and the redistribution consumer (ospf-10) are unchanged for normal areas; Type 7 origination reuses the Type 5 body builder; the §2.5 preference is additive to §16.4.
- The umbrella route-preference contract (intra > inter > E1 > E2, resolved inside OSPF SPF, one winning `locrib.Path` per prefix at AdminDistance 110) is unchanged: NSSA only changes WHICH external LSA wins, not the install model.
- DR/BDR election, the ISM/NSM, and adjacency formation (ospf-5/6) are unchanged except that a stub/NSSA Hello E/N-bit mismatch now blocks the adjacency (a new discard reason, not a new state).
- The LSA wire codec (Type 7 body, N/P/E option bits) owned by ospf-2 is unchanged; this spec consumes it.

**Behavior to change:**
- New stub/NSSA flooding-acceptance policy in `lsdb/`: drop Type 4/5 into/out of stub and NSSA areas; permit Type 7 only within an NSSA.
- New Type 7 NSSA-LSA origination at an NSSA ASBR (reusing the Type 5 body builder, setting the P-bit per config + a non-zero Forwarding Address).
- New ABR stub/NSSA default injection (Type 3 default for stub at `default-cost`; Type 7 default for NSSA) and `no-summary` Type 3 suppression-except-default.
- New NSSA translator election (RFC 3101 §3.5) and Type 7→Type 5 translation at the elected translator, incrementing `ze_ospf_nssa_translations_total{area}`.
- New RFC 3101 §2.5 Type 7/Type 5 preference in the SPF external route computation.
- E-bit (stub + NSSA) and N-bit (NSSA) set-on-send / match-on-receive in the Hello path.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: an `areas/area` entry with `area-type` = `stub` or `nssa` (optionally `no-summary`, `default-cost`, `translate-role`, NSSA `default-information-originate`), resolved by ospf-4 into the area runtime struct and applied on `OnConfigApply`.
- Received Hello on a stub/NSSA interface (E-bit / N-bit fields in the Options) from ospf-5.
- LSDB change inside the area (a neighbour's Router-LSA, a redistributed route from the ospf-10 consumer, a new/changed Type 7, an ABR set change) from ospf-7.
- Redistribution event (connected/static/BGP route) arriving at the ospf-10 `RedistConsumer` when the local router is an NSSA ASBR.

### Transformation Path
1. **Area-type resolution:** ospf-4 resolves `area-type`/`no-summary`/`default-cost`/`translate-role` into the area runtime; the area exposes `IsStub()`, `IsNSSA()`, `NoSummary()`, and the E/N option bits for its interfaces.
2. **Hello option policy (ospf-5 path, extended here):** outgoing Hellos on a stub/NSSA interface clear the E-bit; NSSA interfaces set the N-bit. On receive, a Hello whose E-bit (stub+NSSA) or N-bit (NSSA) does not match the interface's area is dropped before the neighbour FSM advances (no adjacency); `ze_ospf_packets_dropped_total{reason="option-mismatch"}` (owned by ospf-3) increments.
3. **Flood-acceptance filter (ospf-7 path, extended here):** when flooding into or accepting from a stub/NSSA area, Type 4 and Type 5 LSAs are rejected; Type 7 LSAs are accepted/flooded ONLY within an NSSA. Self Router-LSA origination clears the E-bit in a stub/NSSA.
4. **ABR default + suppression (ospf-9 path, extended here):** for each attached stub area the ABR originates a Type 3 default `0.0.0.0/0` at `default-cost`; for each attached NSSA the ABR originates a Type 7 default (per config); with `no-summary` the ABR suppresses all other Type 3 summaries into that area.
5. **NSSA Type 7 origination (ospf-10 consumer path, extended here):** when the local router is an NSSA ASBR, each redistributed route is originated as a Type 7 (Type 5 body builder reused) flooded only within the NSSA, with the P-bit set per config (default set, i.e. translation desired) and a non-zero, intra-NSSA-reachable Forwarding Address (§2.3).
6. **Translator election (new):** among the ABRs attached to an NSSA (learned from their Router-LSAs / the area ABR set), elect the highest-Router-ID translate-capable candidate; sticky, with a `stability-interval` hysteresis; record whether THIS router is the translator.
7. **Type 7→5 translation (new, translator only):** for each P=1, non-zero-FA Type 7 in the NSSA, build a Type 5 with the P-bit cleared, Advertising Router = this router, Forwarding Address / metric / E-bit / tag / LS-ID preserved (LS-ID range-select on collision, §3.2), LS Age reset, Fletcher checksum recomputed; flood AS-wide via the ospf-10 Type 5 originator; increment `ze_ospf_nssa_translations_total{area}`. On loss of candidacy or P-bit/FA change, withdraw (MaxAge) the translated Type 5.
8. **External route computation (ospf-8/10 path, extended here):** when the same external prefix is reachable via a Type 7 (P=1), a Type 5, and/or a Type 7 (P=0), select per RFC 3101 §2.5 (Type 7 P=1 > Type 5 > Type 7 P=0); then apply the §16.4 E1/E2 + forwarding-address cost. The winning external route is published as one `locrib.Path` (AdminDistance 110) exactly as ospf-10, after the umbrella intra > inter > external resolution.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ area runtime | ospf-4 resolves `area-type`/`no-summary`/`default-cost`/`translate-role` into the area struct (value-typed) | [ ] |
| iface ↔ Hello (ospf-5) | E-bit/N-bit set-on-send, match-on-receive; mismatch drop before FSM advance | [ ] |
| LSDB ↔ flooding (ospf-7) | accept/originate filters keyed on area type (drop Type 4/5 in stub/NSSA; Type 7 only in NSSA) | [ ] |
| ABR originator ↔ area (ospf-9) | stub Type 3 default at `default-cost`; NSSA Type 7 default; `no-summary` Type 3 suppression | [ ] |
| ASBR consumer ↔ NSSA (ospf-10) | redistributed route → Type 7 (reuse Type 5 body builder) flooded within the NSSA | [ ] |
| translator ↔ backbone (ospf-10) | P=1 Type 7 → Type 5 (clear P, set Adv Router, preserve FA), flood AS-wide | [ ] |
| SPF external compute (ospf-8/10) | RFC 3101 §2.5 preference selects the external LSA before §16.4 cost; one winning `locrib.Path` | [ ] |

### Integration Points
- `internal/component/ospf/lsdb/` - Type 7 originator (reuse the ospf-10 Type 5 body builder), E/N-bit option policy, stub/NSSA default injection + `no-summary` suppression (filter the ospf-9 summary originator), translator election + Type 7→5 translation (reuse the ospf-10 Type 5 originator).
- `internal/component/ospf/spf/` - RFC 3101 §2.5 preference in the external route computation; stub/NSSA accept-filter (no Type 5 routes inside a stub/NSSA).
- `internal/component/ospf/iface/` - E/N-bit Hello set-on-send and match-on-receive in the existing Hello validation.
- `internal/component/ospf/yang/ze-ospf-conf.yang` - the NSSA `translate-role` leaf (and NSSA default-originate leaf) if not already shipped by ospf-4; the `area-type`/`no-summary`/`default-cost` leaves already exist (umbrella schema owner ospf-4).
- Prometheus: this spec OWNS and registers `ze_ospf_nssa_translations_total{area}` (per the umbrella canonical metrics table); ospf-13 only scrapes/asserts it. No other `ze_ospf_*` series are added here.
- CLI: `show ip ospf database nssa-external` (Type 7) and the area-type/translator state in `show ip ospf` rendering are owned by ospf-13; this spec provides the snapshot data (translator identity, Type 7 inventory, translation count).

### Architectural Verification
- [ ] No bypassed layers (area-type policy gates the existing ospf-7 LSDB / ospf-8 SPF / ospf-9 ABR / ospf-10 ASBR paths; no second LSDB or route table)
- [ ] No unintended coupling (stub/NSSA logic lives in `lsdb/` and `spf/`; no NSSA spelling in generic packages; SPF reads the LSDB, does not re-flood)
- [ ] No duplicated functionality (Type 7 reuses the Type 5 body builder; translation reuses the Type 5 originator; default injection reuses the ospf-9 summary originator)
- [ ] Value-typed boundary preserved (area runtime + translator identity are value types; translated LSA bytes are copied, not aliased across the flood boundary)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The ospf-2 codec already encodes/decodes the Type 7 body (= Type 5 body), the P-bit in the LSA-header Options, and the N-bit in the Hello/DD Options, so this spec only sets/reads them | umbrella "LSA header + body layout" (Type 7 carries the NSSA P-bit in the Options field) + ospf-2 scope | This spec would have to extend the codec, widening scope into ospf-2 | grep the ospf-2 codec for the P/N/E option bits and the Type 7 body; `TestOSPFType7Origination` round-trips a Type 7 | unvalidated |
| A-2 | The ospf-4 `areas/area` schema already carries `area-type` (`normal`/`stub`/`nssa`), `no-summary`, `default-cost`, and `ranges`; only the NSSA `translate-role` (and NSSA default-originate) leaf may be new | umbrella "Area + interface config model" (schema owner ospf-4) | This spec adds the missing leaves (with full native YANG constraints + `CompleteFn`) | read `ze-ospf-conf.yang` during the audit; `ze config validate` on a stub + NSSA config | unvalidated |
| A-3 | The ospf-9 ABR originator and ospf-10 Type 5 originator expose reusable seams (origination request / suppression hook) so default injection, Type 3 suppression, and Type 7→5 translation layer on top without forking them | ospf-9 / ospf-10 design | This spec adds origination filters at the area boundary instead of reusing the originators | read the ospf-9/10 originator interfaces; `TestOSPFStubDefaultInjection`, `TestOSPFNSSATranslation` | unvalidated |
| A-4 | The external route computation (§16.4, ospf-10) is a single selection step into which the RFC 3101 §2.5 Type 7/Type 5 preference can be inserted before the E1/E2 cost, still publishing one winning `locrib.Path` (AdminDistance 110) per prefix per the umbrella contract | umbrella "Route preference / path types"; guide §6f / §6d | A standalone NSSA route step is needed; the §2.5 preference must still feed the single-winner install | `TestOSPFNSSAPreference` (Type 7 P=1 beats Type 5 beats Type 7 P=0); `ospf-nssa.ci` install assertion | unvalidated |
| A-5 | The set of ABRs attached to an NSSA (the translator-election candidate set) is derivable from the area LSDB (other ABRs' Router-LSAs with the B-bit + backbone attachment) without a new neighbour protocol | RFC 3101 §3.5; ospf-9 ABR detection | Translator election needs an extra signalling path | `TestOSPFNSSATranslatorElection` builds a candidate set from a hand-built LSDB | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Every NSSA ABR translates → duplicate Type 5 in the backbone (guide trap #9) | two Type 5 for the same prefix from different ABRs in a multi-ABR NSSA test | Implement the §3.5 election: exactly one translator (highest Router ID candidate); only the translator runs translation; `TestOSPFNSSATranslatorElection` + `ospf-nssa.ci` dual-ABR step asserts a single Type 5 |
| R-2 | E-bit (stub) / N-bit (NSSA) mismatch silently blocks adjacency (guide trap #11) and is hard to diagnose | adjacency never reaches Full across a misconfigured stub/NSSA boundary | Validate the option bits on receive with a NAMED drop reason (`option-mismatch`) surfaced in `show ip ospf` and the dropped-packets metric; `TestOSPFStubEbitMismatch`, `TestOSPFNSSANbitMismatch` |
| R-3 | Translator flap on a transient current-translator outage churns the backbone Type 5 set | Type 5 add/withdraw storms on a translator restart | Sticky election with a `stability-interval` (default 40 s) hysteresis; only re-elect on a real candidate-set change; `TestOSPFNSSATranslatorStability` |
| R-4 | Type 7→5 translation drops or zeroes the Forwarding Address, blackholing the route in the backbone | translated Type 5 has FA 0.0.0.0; backbone routers cannot resolve the next hop | Preserve the FA verbatim; require a non-zero FA on the source Type 7 (skip translation if FA is zero, §2.3); `TestOSPFNSSATranslationPreservesFA` |
| R-5 | RFC 3101 §2.5 preference ignored → a Type 5 (farther) chosen over a closer Type 7 P=1, wrong next hop | route uses the AS-wide Type 5 path when an intra-NSSA Type 7 exists | Implement §2.5 ordering before the §16.4 cost; `TestOSPFNSSAPreference` asserts Type 7 P=1 wins |
| R-6 | Totally-stubby/NSSA suppresses the default too, or leaks Type 3s it should hide | spoke router has no default, or has unexpected inter-area routes | Suppress Type 3 EXCEPT the injected default; `TestOSPFTotallyStubbyOnlyDefault`, `TestOSPFTotallyNSSAOnlyDefault` |
| R-7 | A stale translated Type 5 lingers after the translator loses candidacy or the source Type 7 is withdrawn | backbone keeps a Type 5 with no live source | On candidacy loss / source withdraw, MaxAge-purge the translated Type 5; `TestOSPFNSSATranslationWithdraw` |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `area-type stub` config applied | → | ABR originates a Type 3 default `0.0.0.0/0` at `default-cost`; Type 4/5 dropped in/out | `TestOSPFStubDefaultInjection` |
| `area-type nssa` + redistributed route at an NSSA ASBR | → | Type 7 NSSA-LSA originated (P=1, non-zero FA), flooded within the NSSA only | `TestOSPFType7Origination` |
| NSSA with two ABRs, a P=1 Type 7 present | → | one translator elected (highest Router ID); only it emits a single Type 5 onto the backbone | `test/ospf/ospf-nssa.ci` |
| Same external prefix via Type 7 P=1 and Type 5 | → | SPF external computation selects the Type 7 P=1 (RFC 3101 §2.5); one `locrib.Path` installed | `test/ospf/ospf-nssa.ci` (preference step) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An interface in a stub area sends/receives Hellos | Outgoing Hello has the E-bit CLEAR; a received Hello with the E-bit SET is discarded and no adjacency forms (drop reason `option-mismatch`) |
| AC-2 | An interface in an NSSA sends/receives Hellos | Outgoing Hello has the E-bit CLEAR and the N-bit SET; a received Hello with a mismatched N-bit is discarded and no adjacency forms |
| AC-3 | A stub area attached to an ABR | The ABR originates exactly one Type 3 Summary-LSA for `0.0.0.0/0` with metric = `default-cost`; no Type 4 or Type 5 LSA is flooded into the area |
| AC-4 | A totally-stubby area (`no-summary`) | The ABR suppresses all Type 3 Summary-LSAs into the area EXCEPT the injected `0.0.0.0/0` default |
| AC-5 | An NSSA ASBR redistributes an external route (P-bit configured set) | A Type 7 NSSA-LSA is originated (body = Type 5 body, P-bit set, non-zero intra-NSSA Forwarding Address) and flooded ONLY within the NSSA; no Type 5 is originated locally into the NSSA |
| AC-6 | An NSSA with two or more candidate ABRs | Exactly one ABR (highest Router ID among translate-capable candidates) is the elected translator; the election is sticky with a stability interval |
| AC-7 | The elected translator sees a P=1, non-zero-FA Type 7 | It re-originates a Type 5 with the P-bit CLEARED, Advertising Router = translator, Forwarding Address / metric / E-bit / tag preserved, flooded AS-wide; `ze_ospf_nssa_translations_total{area}` increments; a non-translator ABR does NOT translate (no duplicate Type 5) |
| AC-8 | A Type 7 with P=0 or a zero Forwarding Address | It is NOT translated to Type 5 and stays local to the NSSA |
| AC-9 | The same external prefix is reachable via a Type 7 (P=1), a Type 5, and a Type 7 (P=0) | External route selection prefers Type 7 P=1 over Type 5 over Type 7 P=0 (RFC 3101 §2.5); one winning `locrib.Path` is installed at AdminDistance 110 |
| AC-10 | The elected translator loses candidacy, or a translated source Type 7 is withdrawn | The translated Type 5 is MaxAge-purged from the backbone; a newly elected translator re-originates it |
| AC-11 | An NSSA configured to originate a default | The ABR originates a Type 7 default `0.0.0.0/0` into the NSSA; with `no-summary` (totally-NSSA) Type 3 summaries are suppressed except the default; the NSSA default Type 7 is not translated to Type 5 |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a stub area on a spoke | config `area-type stub` → ABR injects Type 3 default at `default-cost`, drops Type 4/5 → spoke holds intra-area + one default | `test/ospf/ospf-stub.ci` |
| 2 | Configures a totally-stubby spoke | config `area-type stub no-summary` → ABR suppresses Type 3 except the default → spoke holds intra-area + only the default | `test/ospf/ospf-stub.ci` (no-summary step) |
| 3 | Redistributes a route into an NSSA at an internal ASBR | redistribution event → ospf-10 consumer → Type 7 origination (P=1, FA) → flood within NSSA → translator re-originates a Type 5 onto the backbone → backbone routers learn the external | `test/ospf/ospf-nssa.ci` |
| 4 | Runs a multi-ABR NSSA and expects no duplicate Type 5 | two ABRs → §3.5 election picks one translator → only it floods the Type 5 | `test/ospf/ospf-nssa.ci` (dual-ABR step) |
| 5 | Expects the closest external path chosen when both Type 7 and Type 5 exist | SPF external computation applies §2.5 (Type 7 P=1 > Type 5 > Type 7 P=0) then §16.4 cost → one `locrib.Path` | `test/ospf/ospf-nssa.ci` (preference step) |
| 6 | Runs `show ip ospf database nssa-external` and `show ip ospf` | CLI → RPC → LSDB Type 7 snapshot + translator-state snapshot (rendering owned by ospf-13) | `test/ospf/ospf-nssa.ci` (show step; full render in ospf-13) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFStubEbitMismatch` | `internal/component/ospf/iface/hello_nssa_test.go` | stub interface clears the E-bit on send; a received E-bit-set Hello is dropped (`option-mismatch`), no adjacency | |
| `TestOSPFNSSANbitMismatch` | `internal/component/ospf/iface/hello_nssa_test.go` | NSSA interface sets the N-bit (E-bit clear) on send; a mismatched-N-bit Hello is dropped, no adjacency | |
| `TestOSPFStubFloodFilter` | `internal/component/ospf/lsdb/area_type_test.go` | Type 4 and Type 5 LSAs are rejected on flood-in and not accepted from a stub/NSSA area; self Router-LSA clears the E-bit | |
| `TestOSPFStubDefaultInjection` | `internal/component/ospf/lsdb/area_type_test.go` | ABR originates exactly one Type 3 `0.0.0.0/0` at `default-cost` into a stub area | |
| `TestOSPFTotallyStubbyOnlyDefault` | `internal/component/ospf/lsdb/area_type_test.go` | `no-summary` suppresses all Type 3 except the injected default | |
| `TestOSPFType7Origination` | `internal/component/ospf/lsdb/nssa_test.go` | NSSA ASBR redistributed route → Type 7 (body = Type 5 body, P-bit set, non-zero FA) flooded only within the NSSA; round-trips through the ospf-2 codec | |
| `TestOSPFType7FloodScope` | `internal/component/ospf/lsdb/nssa_test.go` | Type 7 is flooded within the NSSA and never leaves it (no Type 7 into the backbone or other areas) | |
| `TestOSPFNSSATranslatorElection` | `internal/component/ospf/lsdb/nssa_translate_test.go` | among candidate ABRs the highest Router ID translate-capable one is elected; `always`/`never` roles honoured | |
| `TestOSPFNSSATranslatorStability` | `internal/component/ospf/lsdb/nssa_translate_test.go` | election is sticky; a transient current-translator outage within the stability interval does not re-elect | |
| `TestOSPFNSSATranslation` | `internal/component/ospf/lsdb/nssa_translate_test.go` | translator re-originates a Type 5 with P cleared, Adv Router = translator, FA/metric/E-bit/tag preserved; counter increments; non-translator does not translate | |
| `TestOSPFNSSATranslationPreservesFA` | `internal/component/ospf/lsdb/nssa_translate_test.go` | the translated Type 5 carries the source Type 7's non-zero Forwarding Address; a zero-FA source Type 7 is skipped | |
| `TestOSPFNSSATranslationWithdraw` | `internal/component/ospf/lsdb/nssa_translate_test.go` | on candidacy loss or source-Type 7 withdraw, the translated Type 5 is MaxAge-purged | |
| `TestOSPFNSSAPbitNotTranslated` | `internal/component/ospf/lsdb/nssa_translate_test.go` | a P=0 Type 7 (and an ABR-originated NSSA default) is not translated to Type 5 | |
| `TestOSPFNSSAPreference` | `internal/component/ospf/spf/external_nssa_test.go` | external route selection prefers Type 7 P=1 > Type 5 > Type 7 P=0 (RFC 3101 §2.5), then §16.4 cost; one winning route | |
| `TestOSPFNSSADefaultOriginate` | `internal/component/ospf/lsdb/nssa_test.go` | NSSA ABR originates a Type 7 default; totally-NSSA suppresses Type 3 except the default | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `default-cost` (stub/NSSA ABR default metric, 24-bit Summary-LSA metric) | 0..16777215 | 16777215 | N/A | 16777216 |
| NSSA `stability-interval` (translator hysteresis, seconds) | 0..65535 | 65535 (default 40) | N/A | 65536 |
| Type 7 / Type 5 external metric (24-bit, E-bit in the high bit of the first metric byte) | 0..16777215 | 16777215 | N/A | 16777216 |
| External Route Tag (32-bit, preserved verbatim on translation) | 0..4294967295 | 4294967295 | N/A | N/A (full 32-bit field) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-stub` | `test/ospf/ospf-stub.ci` | stub spoke: ABR injects a default at `default-cost`, no Type 5 in the area; `no-summary` leaves only intra-area + the default | |
| `ospf-nssa` | `test/ospf/ospf-nssa.ci` | NSSA: ASBR originates a Type 7, the elected translator re-originates one Type 5 onto the backbone (single, in a dual-ABR step), §2.5 preference picks Type 7 P=1, NSSA default originated | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (owned by ospf-13) | `test/interop/scenarios/` | FRR `ospfd` | `ospf-stub-frr` and `ospf-nssa-frr` validate stub default injection, Type 7 origination, translator election, and Type 7→5 translation against FRR; mandatory, owned by ospf-13 per the umbrella "Test + interop wiring" | |

### Future (if deferring any tests)
- FRR interop for stub/NSSA is owned by ospf-13 (`ospf-stub-frr`, `ospf-nssa-frr`); this spec proves stub/NSSA behaviour with Ze-to-Ze unit + functional tests. Live raw-IP/multicast multi-router flows run as QEMU integration tests (`ai/rules/qemu-testing.md`), not plain `.ci`.

## Files to Modify
- `internal/component/ospf/lsdb/` - add the Type 7 originator (reuse the ospf-10 Type 5 body builder), the E/N-bit option policy for self Router-LSA / area interfaces, the stub/NSSA default injection + `no-summary` Type 3 suppression (filter the ospf-9 summary originator), and the translator election + Type 7→5 translation (reuse the ospf-10 Type 5 originator)
- `internal/component/ospf/spf/` - add the RFC 3101 §2.5 Type 7/Type 5 preference into the external route computation and the stub/NSSA accept-filter (no Type 5 routes computed inside a stub/NSSA)
- `internal/component/ospf/iface/` - add the E-bit (stub+NSSA) / N-bit (NSSA) set-on-send and match-on-receive into the existing Hello validation
- `internal/component/ospf/yang/ze-ospf-conf.yang` - add the NSSA `translate-role` leaf (`candidate`/`always`/`never`, default `candidate`) and the NSSA default-originate leaf IF ospf-4 did not already ship them (the `area-type`/`no-summary`/`default-cost`/`ranges` leaves already exist; confirm in the audit). Every added leaf gets full native YANG constraints (`enumeration`, `range`) plus a `CompleteFn`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Maybe | `ze-ospf-conf.yang`: NSSA `translate-role` + NSSA default-originate leaves IF not already in ospf-4 (`area-type`/`no-summary`/`default-cost` already exist) |
| YANG validation constraints | Yes (if leaves added) | `enumeration` for `translate-role`; `range` for `default-cost`/`stability-interval`; reuse `ze-types.yang` |
| YANG custom validators | Maybe | `CompleteFn` for `translate-role` enum tab-completion if a custom validator is needed beyond the native enum |
| CLI commands/flags | Yes | `show ip ospf database nssa-external` (Type 7) + translator/area-type state in `show ip ospf`; RPCs registered + rendered in ospf-13; this spec provides the snapshot data |
| CLI grammar (action before identifier) | Yes | `show ip ospf database nssa-external` follows `ai/rules/cli-grammar.md` (owned by ospf-13) |
| Editor autocomplete | Yes (if enum leaf added) | automatic for the `translate-role` enumeration leaf |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-stub.ci`, `test/ospf/ospf-nssa.ci` |
| Pipe completeness | Yes | `show ip ospf database nssa-external` output through `ApplyPipes`/`ProcessPipes` (ospf-13) |
| Doctor check for runtime dependencies | No | no new runtime dependency (raw-IP transport + multicast owned by ospf-3) |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_ospf_nssa_translations_total{area}` (per the umbrella canonical table); per-owner registration here, ospf-13 only scrapes/asserts |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPF stub/NSSA row) |
| 2 | Config syntax changed? | Maybe | `docs/guide/configuration.md` (NSSA `translate-role` / default-originate) if leaves added |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show ip ospf database nssa-external`, in ospf-13) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (NSSA database/translator snapshot) |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` (stub/NSSA section) |
| 7 | Wire format changed? | No | Type 7 body + N/P/E option bits owned by ospf-2 |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc3101.md`, `rfc/short/rfc2328.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/ospf-stub.ci`, `ospf-nssa.ci`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (OSPF stub/NSSA row) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (area-type policy over LSDB/SPF; NSSA translator) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (`ze_ospf_nssa_translations_total` owned + registered here; surfaced in ospf-13) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md` (NSSA translator + Type 7 database) |
| 16 | Changed files referenced by doc source anchors? | No | grep at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion |

## Files to Create
- `internal/component/ospf/lsdb/area_type.go` - stub/NSSA flood-acceptance filter (drop Type 4/5 in/out), self Router-LSA E-bit policy, stub Type 3 default injection at `default-cost`, `no-summary` Type 3 suppression
- `internal/component/ospf/lsdb/nssa.go` - Type 7 NSSA-LSA origination (reuse the ospf-10 Type 5 body builder; set P-bit + non-zero FA), Type 7 flood scope (within the NSSA only), NSSA Type 7 default origination
- `internal/component/ospf/lsdb/nssa_translate.go` - RFC 3101 §3.5 translator election (highest Router ID candidate, sticky, `stability-interval` hysteresis) + §3.6 Type 7→Type 5 translation (clear P, set Adv Router, preserve FA/metric/tag, reset LS Age, recompute Fletcher, flood AS-wide via the ospf-10 Type 5 originator); owns + registers `ze_ospf_nssa_translations_total{area}`
- `internal/component/ospf/spf/external_nssa.go` - RFC 3101 §2.5 Type 7/Type 5 external preference, layered on the ospf-10 §16.4 external route computation, and the stub/NSSA accept-filter
- `internal/component/ospf/iface/hello_nssa.go` - E-bit (stub+NSSA) / N-bit (NSSA) set-on-send and match-on-receive helpers used by the ospf-5 Hello path
- `internal/component/ospf/lsdb/area_type_test.go`, `internal/component/ospf/lsdb/nssa_test.go`, `internal/component/ospf/lsdb/nssa_translate_test.go`, `internal/component/ospf/spf/external_nssa_test.go`, `internal/component/ospf/iface/hello_nssa_test.go` - unit tests from the TDD plan
- `test/ospf/ospf-stub.ci` - stub + totally-stubby end-to-end (default injection, Type 5 absent, no-summary)
- `test/ospf/ospf-nssa.ci` - NSSA end-to-end (Type 7 origination, single-translator election, Type 7→5 translation, §2.5 preference, NSSA default)

Note: the area-type SCHEMA and config resolve are owned by ospf-4; the ABR summary originator by ospf-9; the Type 5 originator + §16.4 external computation + redistribution consumer by ospf-10. This spec adds policy/election/translation on those seams, not new copies of them.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan; confirm A-1/A-2/A-3 seams exist (ospf-2 Type 7/option codec, ospf-4 area-type schema, ospf-9/10 originator seams) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - wire area-type policy to the area runtime and add a failing stub/NSSA test
   - Tests: `TestOSPFStubDefaultInjection`, `TestOSPFType7Origination` (fail: no policy yet), `test/ospf/ospf-stub.ci`, `test/ospf/ospf-nssa.ci` (skeleton)
   - Files: `lsdb/area_type.go` (area-type hook stub), `lsdb/nssa.go` (Type 7 originator stub), confirm `translate-role` leaf in `ze-ospf-conf.yang`
   - Verify: the area runtime exposes `IsStub()`/`IsNSSA()`/`NoSummary()`; the wiring tests fail because the policy is a stub
2. **Phase: Stub areas + E-bit** - clear E-bit, drop Type 4/5, inject the Type 3 default, `no-summary` suppression
   - Tests: `TestOSPFStubEbitMismatch`, `TestOSPFStubFloodFilter`, `TestOSPFStubDefaultInjection`, `TestOSPFTotallyStubbyOnlyDefault`
   - Files: `lsdb/area_type.go`, `iface/hello_nssa.go`
   - Verify: stub Hello E-bit clear + mismatch drop; Type 4/5 rejected; one Type 3 default at `default-cost`; `no-summary` suppresses Type 3 except the default
3. **Phase: NSSA Type 7 origination + N-bit** - Type 7 at the NSSA ASBR, flood within the NSSA, N-bit Hello policy
   - Tests: `TestOSPFNSSANbitMismatch`, `TestOSPFType7Origination`, `TestOSPFType7FloodScope`, `TestOSPFNSSADefaultOriginate`
   - Files: `lsdb/nssa.go`, `iface/hello_nssa.go`
   - Verify: Type 7 originated (P-bit set, non-zero FA, body = Type 5 body), flooded only within the NSSA; N-bit set + mismatch drop; NSSA Type 7 default + totally-NSSA suppression
4. **Phase: Translator election + Type 7→5 translation** - §3.5 election and §3.6 translation
   - Tests: `TestOSPFNSSATranslatorElection`, `TestOSPFNSSATranslatorStability`, `TestOSPFNSSATranslation`, `TestOSPFNSSATranslationPreservesFA`, `TestOSPFNSSATranslationWithdraw`, `TestOSPFNSSAPbitNotTranslated`
   - Files: `lsdb/nssa_translate.go`
   - Verify: highest-Router-ID candidate elected, sticky with the stability interval; translator clears P, sets Adv Router, preserves FA/metric/tag, floods AS-wide; counter increments; non-translator does not translate; P=0 / zero-FA / NSSA-default not translated; withdraw on candidacy loss
5. **Phase: §2.5 external preference** - Type 7 P=1 > Type 5 > Type 7 P=0 in SPF
   - Tests: `TestOSPFNSSAPreference`
   - Files: `spf/external_nssa.go`
   - Verify: the §2.5 order is applied before the §16.4 cost; one winning `locrib.Path` (AdminDistance 110) installed
6. **Phase: metric + snapshot** - register `ze_ospf_nssa_translations_total{area}`; provide translator/Type 7 snapshot data
   - Files: `lsdb/nssa_translate.go` (counter), snapshot accessors for ospf-13
   - Verify: counter increments on each translation; the snapshot exposes the translator identity, Type 7 inventory, and translation count for `show ip ospf` (rendered in ospf-13)
7. **Functional tests** - finalise `test/ospf/ospf-stub.ci`, `test/ospf/ospf-nssa.ci`
8. **RFC refs** - add `// RFC 3101 Section X.Y` / `// RFC 2328 Section 3.6` comments above the enforcing code
9. **Full verification** - `make ze-verify`
10. **Complete spec** - fill audit tables, write the learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (stub default, totally-stubby, Type 7 origination, single-translator, §2.5 preference, NSSA default, show) |
| Correctness | E-bit clear in stub/NSSA + mismatch drop; N-bit set in NSSA; one translator (highest Router ID, sticky); translation clears P / sets Adv Router / preserves FA; §2.5 order Type 7 P=1 > Type 5 > Type 7 P=0; no duplicate Type 5 |
| Naming | YANG `translate-role` enum (`candidate`/`always`/`never`); CLI `show ip ospf database nssa-external`; metric `ze_ospf_nssa_translations_total{area}` exactly |
| Data flow | area-type policy gates the existing LSDB/SPF/ABR/ASBR paths; Type 7 reuses the Type 5 body builder; translation reuses the Type 5 originator; one winning `locrib.Path` per prefix |
| CLI grammar | `show ip ospf database nssa-external` action before identifier (owned by ospf-13) |
| YANG validation | `translate-role` native `enumeration`; `default-cost`/`stability-interval` `range`; no bare `type string` |
| Prometheus counters | `ze_ospf_nssa_translations_total{area}` defined, registered here, scraped in ospf-13; no other `ze_ospf_*` added |
| Rule: plugin-self-containment | all stub/NSSA policy/election/translation under `internal/component/ospf/`; no NSSA spelling in generic packages |
| Rule: memory-architecture | translated LSA bytes copied (not aliased); area runtime + translator identity value-typed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Stub/NSSA policy package | `ls internal/component/ospf/lsdb/area_type.go internal/component/ospf/lsdb/nssa.go` |
| Translator election + translation | `grep -rn 'translat' internal/component/ospf/lsdb/nssa_translate.go` |
| §2.5 external preference | `grep -rn 'P-bit\|Type 7\|3101' internal/component/ospf/spf/external_nssa.go` |
| NSSA translations metric | `grep -rn 'ze_ospf_nssa_translations_total' internal/component/ospf/` |
| Functional tests | `ls test/ospf/ospf-stub.ci test/ospf/ospf-nssa.ci` |
| No duplicate Type 5 in a multi-ABR NSSA | `ospf-nssa.ci` dual-ABR step asserts a single Type 5 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Received Hello E/N option bits validated against the area type before the FSM advances; a malformed/oversized Type 7 body is rejected by the ospf-2 codec, not panicked; the Forwarding Address is validated non-zero + intra-NSSA before translation |
| Resource exhaustion | Translator election + translation bounded by the NSSA ABR/Type 7 count; no unbounded re-flood loop; the stability interval damps flap-driven churn |
| Loop prevention | Type 7 never leaves the NSSA; a translated Type 5 carries Adv Router = translator (not the original ASBR) so it is not re-imported into the NSSA as a Type 7; only the elected translator translates (no duplicate Type 5) |
| Error leakage | Translation/election failures logged, not panicked; one malformed Type 7 excludes that LSA, not the whole area |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test setup |
| Test fails behavior mismatch | Re-read RFC 3101 / RFC 2328 §3.6 summary / Current Behavior |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| 3 fix attempts fail | STOP; report all 3 approaches; ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Area type as a flooding/origination POLICY over the existing ospf-7 LSDB and ospf-8 route table | A separate stub/NSSA LSDB or route table | Stub/NSSA differ only in WHICH LSA types are accepted/originated/translated; a separate datastore would duplicate the §13 flooding and §16 SPF code |
| Type 7 origination reuses the ospf-10 Type 5 body builder | A standalone Type 7 body encoder | The Type 7 body IS the Type 5 body (RFC 3101 §2); only the LS Type and the Options P-bit differ |
| Type 7→5 translation reuses the ospf-10 Type 5 originator | A separate AS-wide flood path | The translated LSA is a normal Type 5 once the P-bit is cleared and the Advertising Router is set; reuse keeps one Type 5 flood path |
| Exactly one translator (highest Router ID candidate), sticky with a stability interval | Let every NSSA ABR translate | RFC 3101 §3.5 + guide trap #9: multiple translators inject duplicate Type 5 into the backbone; stickiness damps flap |
| RFC 3101 §2.5 preference (Type 7 P=1 > Type 5 > Type 7 P=0) layered on §16.4, still publishing one winning `locrib.Path` at AdminDistance 110 | Treat Type 7 and Type 5 as equivalent externals | §2.5 mandates the order; equal treatment picks the farther Type 5 over a closer intra-NSSA Type 7 (wrong next hop) and contradicts the umbrella preference contract |

## Known Limitations
- IPv4 only (OSPFv2; OSPFv3 NSSA is the separate v3 umbrella).
- Translator role is per-area `candidate`/`always`/`never`; per-prefix translation policy and Type 7 address-range aggregation on translation (RFC 3101 §3.1 ranges beyond LS-ID collision handling) are not in v1.
- FRR `ospf-stub-frr` / `ospf-nssa-frr` interop is owned by ospf-13; this spec proves behaviour Ze-to-Ze.
- Virtual links through an NSSA/stub transit area are out of scope (umbrella out-of-scope: virtual links).

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- `// RFC 2328 Section 3.6` above the stub E-bit-clear / Type 4-5-drop / ABR-default code.
- `// RFC 3101 Section 2` above the Type 7 origination (P-bit, body = Type 5 body, N-bit) code.
- `// RFC 3101 Section 2.3` above the Forwarding-Address requirement and the "P=0 / zero-FA not translated" check.
- `// RFC 3101 Section 2.5` above the external route-selection preference (Type 7 P=1 > Type 5 > Type 7 P=0).
- `// RFC 3101 Section 3.5` above the translator election (highest Router ID candidate, sticky, stability interval).
- `// RFC 3101 Section 3.6` above the Type 7→5 translation (clear P, set Advertising Router, preserve Forwarding Address/metric/tag, AS-wide flood).

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Stub area: E-bit clear, no Type 5, ABR default at `default-cost` | unit + functional | `TestOSPFStubEbitMismatch`, `TestOSPFStubDefaultInjection`, `ospf-stub.ci` |
| Totally-stubby: Type 3 suppressed except default | unit + functional | `TestOSPFTotallyStubbyOnlyDefault`, `ospf-stub.ci` (no-summary step) |
| NSSA Type 7 origination + flood scope | unit + functional | `TestOSPFType7Origination`, `TestOSPFType7FloodScope`, `ospf-nssa.ci` |
| One translator (highest Router ID, sticky), no duplicate Type 5 | unit + functional | `TestOSPFNSSATranslatorElection`, `TestOSPFNSSATranslatorStability`, `ospf-nssa.ci` (dual-ABR step) |
| Type 7→5 translation preserves FA, clears P, sets Adv Router | unit | `TestOSPFNSSATranslation`, `TestOSPFNSSATranslationPreservesFA` |
| RFC 3101 §2.5 external preference | unit + functional | `TestOSPFNSSAPreference`, `ospf-nssa.ci` (preference step) |
| `ze_ospf_nssa_translations_total{area}` increments | unit + interop | counter assertion in `TestOSPFNSSATranslation`; scrape in `ospf-nssa-frr` (ospf-13) |
| FRR interop (stub + NSSA) | interop (owned by ospf-13) | `ospf-stub-frr`, `ospf-nssa-frr` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/ospf/lsdb/`, `spf/`, `iface/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification - owned by ospf-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-11-stub-nssa.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-11-stub-nssa.md` only

## Related Specs
- `plan/spec-ospf-9-inter-area-abr.md` - ABR + Type 3/4 Summary-LSA + area ranges (dependency; stub default injection + `no-summary` suppression extend it)
- `plan/spec-ospf-10-as-external-asbr.md` - Type 5 origination + §16.4 external computation + redistribution consumer (dependency; Type 7 origination + translation + §2.5 preference reuse it)
- `plan/spec-ospf-7-lsdb-flooding.md` - the per-area LSDB + §13 flooding the area-type policy gates
- `plan/spec-ospf-12-auth.md` - authentication (sibling, independent of area type)
- `plan/spec-ospf-13-cli-diag-interop.md` - renders `show ip ospf database nssa-external` + translator state, scrapes `ze_ospf_nssa_translations_total`, FRR `ospf-stub-frr` / `ospf-nssa-frr` interop
