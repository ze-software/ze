# 965 - OSPFv2 stub and NSSA areas (RFC 3101)

## Context

Completed `plan/spec-ospf-11-stub-nssa.md`: stub / totally-stubby areas (RFC 2328 §3.6) and NSSA (RFC 3101). Stub: E-bit-clear Hello + mismatch drop, Type 4/5 flood-block, a Type 3 default at `default-cost`, totally-stubby Type 3 suppression. NSSA: N-bit Hello policy, Type 7 origination (P-bit + forwarding address), Type 7-only flood scope, the highest-Router-ID translator election with a stability grace, Type 7→Type 5 translation, the NSSA Type 7 default, and the RFC 3101 §2.5 external preference. Owns `ze_ospf_nssa_translations_total{area}`.

## Decisions

- **Reuse, don't duplicate.** A large amount of scaffolding already existed: `LSDB.areaTypes` + `SetAreaTypes`, a partial `shouldDropByArea`, `OptionE`/`OptionNP` bits, `LSTypeNSSA`=7 sharing the Type 5 body codec, the ISM `expectedOptionsLocked`, and the Hello E-bit check. The work was extending these (Type 4 + Type 7 scope in the flood filter, the N-bit in the Hello check) and adding origination/election/translation on top — not new copies.
- **Stub default injection lives in the SPF summary originator, not the LSDB.** `applyAreaTypePolicy` rewrites the desired Type 3/4 set per destination area (drop Type 4; suppress Type 3 on `no-summary`; inject one Type 3 default for stub), because that is where the desired-summary set is computed. Threaded `AreaSummaryPolicy` through `SummaryInput.Policies` and `Computer.areaPolicies`. (Spec mapped it to `lsdb/area_type.go`; it lives in `spf/area_type.go` — documented deviation.)
- **The P-bit rule decides translation need at origination.** `externalScope` returns the attached NSSAs (with a representative intra-NSSA forwarding address) and `canType5` (the router can inject Type 5 directly: a normal/backbone attachment, OR no NSSA attachment at all — preserving plain-ASBR behaviour and the existing redist tests). A redistributed route becomes a Type 7 in each NSSA with `P = !canType5 && FA != 0`, and a Type 5 AS-wide only when `canType5`.
- **The translator election is computed locally** from the NSSA's Router-LSAs whose flags have BOTH the B-bit and the RFC 3101 Nt-bit set (the translator-candidate set); the highest Router ID wins (role `candidate`), with `always`/`never` overrides. This router sets its own Nt-bit (`RouterFlagNt`, threaded via `OriginInput.NSSATranslator` from `LSDB.SetNSSATranslatorAreas`) for any attached NSSA whose role is not `never`. **Review fix:** filtering on B-bit alone let a higher-Router-ID `translate never` ABR wedge translation off; requiring Nt excludes non-candidates so a willing lower-ID candidate still translates (`TestOSPFNSSANonCandidateDoesNotWedge`). The stability grace (`translatorEffective`) keeps a router translating for `stability-interval` after it loses the election, so a transient flap opens no Type 5 gap.
- **§2.5 preference is a new primary key on the external candidate.** `externalCand.pref` (Type7-P1 < Type5 < Type7-P0) is compared in `betterExternal` ahead of the §16.4 E1/E2 rank and cost, so trap #7 (E1>E2) still applies within a pref class.

## Gotchas

- **NSSA reconciliation runs from two goroutines.** `translateNSSA` (and `applyNSSADefaults`) are called from BOTH the config-apply `reconcile` path AND the 1s retransmit tick. Their read-compute-write of `e.translations` / `e.translatorState` spans `e.mu` releases, so two concurrent passes could double-originate or lose a withdraw. Serialized with a dedicated `nssaMu` (mirrors `defaultInfoMu`). Lock order `nssaMu > e.mu > LSDB d.mu`; LSDB origination/purge happens with `e.mu` released, and `nssaMu`/`defaultInfoMu` are never held together.
- **A Type 7 P-bit toggle on an unchanged body must still re-originate.** `existingSelfBodyUnchanged` compares the body only; `OriginateNSSA` additionally checks that the stored header's P-bit matches `propagate`, else it falls through to re-originate. Without that, a `candidate`→`always` P-bit change on an identical body would be silently dropped.
- **The translated Type 5 shares the AS-wide store/key with self-redistributed Type 5s** (both `OriginateExternal(self, network, ...)`). The review (B1, BLOCKER) showed this could blackhole a redistributed route: the translator clobbered a network it redistributes, and a peer's Type 7 withdrawal MaxAge-purged the redistributed Type 5 AS-wide. Fix (RFC 3101 §3.6): a `redistExternals` claim set -- the translator SKIPS a network it already redistributes and NEVER purges a redistribute-owned key. Lesson: any two uncoordinated reconcilers writing the same self-LSA key (here redistribution vs translation, as with the spec-10 `0.0.0.0/0` default) need an explicit ownership/claim protocol; "they use different prefixes in practice" is not a guarantee, and disjoint-prefix tests hide it.
- **`instance.go` crossed the 1000-line hard cap** when the engine gained the NSSA fields. Extracted the self-contained `dispatcher` (type + methods) to `dispatcher.go` — a clean split, no behaviour change.
- **The spec-10 OSPF metrics were never documented.** While adding `ze_ospf_nssa_translations_total`, found `ze_ospf_asbr` / `ze_ospf_external_lsas` / `ze_ospf_redist_*` missing from `docs/plugin-development/metrics.md` and backfilled them.

## Verification anchors

- `TestOSPFStubDefaultInjection` / `TestOSPFTotallyStubbyOnlyDefault` (`spf/area_type_test.go`) — Type 3 default at default-cost; no-summary keeps only the default.
- `TestOSPFStubFloodFilter` (`lsdb/area_type_test.go`) — Type 4/5 dropped in stub/NSSA, Type 7 NSSA-only, both flood directions.
- `TestOSPFStubEbitMismatch` / `TestOSPFNSSANbitMismatch` (`iface/hello_nssa_test.go`) — E-bit/N-bit Hello mismatch drops the adjacency.
- `TestOSPFType7Origination` / `TestOSPFType7Withdraw` (`lsdb/nssa_test.go`) — Type 7 in the area store, P-bit, FA preserved, not in the AS-wide store.
- `TestEngineInjectExternalNSSAOnly` / `TestEngineInjectExternalNSSAandBackbone` / `TestEngineNSSADefaultOriginate` (`redist_wiring_test.go`) — the P-bit rule + NSSA default.
- `TestOSPFNSSATranslatorElection` / `TestOSPFNSSATranslation` / `TestOSPFNSSAPbitNotTranslated` / `TestOSPFNSSANoTranslateWhenNotElected` / `TestOSPFNSSATranslatorStability` (`nssa_test.go`) — election, translation, no-duplicate, P=0/zero-FA skip, stability grace.
- `TestOSPFNSSAPreference` (`spf/external_nssa_test.go`) — Type 7 P=1 > Type 5 > Type 7 P=0.
- `test/ospf/ospf-stub.ci`, `test/ospf/ospf-nssa.ci` — config surface. FRR `ospfd` stub/NSSA interop is owned by spec-ospf-13 (Linux/QEMU).

## Files

None recorded.
