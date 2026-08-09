# OSPF stub and NSSA areas

Stub and totally-stubby areas (RFC 2328 Section 3.6) and NSSA (RFC 3101):
Hello option policy, flood scope, Type 7 origination, the translator election
and Type 7 to Type 5 translation.

## Decisions

- **Extend the existing scaffolding, do not copy it.** The area-type table, a
  partial flood filter, the E-bit and NP-bit options, Type 7 sharing the Type 5
  body codec, and the ISM expected-options check already existed. The work added
  Type 4 and Type 7 scope to the flood filter, the N-bit to the Hello check, and
  origination, election and translation on top.
  <!-- source: internal/plugins/ospf/lsdb/nssa.go -- OriginateNSSA -->
- **Stub default injection lives in the SPF summary originator, not in the
  LSDB.** The desired Type 3 and Type 4 set per destination area is computed
  there: drop Type 4, suppress Type 3 under `no-summary`, and inject one Type 3
  default for a stub area.
  <!-- source: internal/plugins/ospf/spf/area_type.go -- applyAreaTypePolicy -->
- **The P-bit rule decides the need for translation at ORIGINATION time.** The
  external scope function returns the attached NSSAs with a representative
  intra-NSSA forwarding address, and whether this router can inject a Type 5
  directly. A redistributed route becomes a Type 7 in each NSSA with
  `P = cannot-inject-Type-5 AND forwarding address is non-zero`, and a Type 5
  AS-wide only when it can inject directly.
  <!-- source: internal/plugins/ospf/redist_wiring.go -- externalScope, externalScopeFor -->
- **The translator election is computed locally** from the NSSA Router-LSAs
  whose flags carry BOTH the B-bit and the RFC 3101 Nt-bit. The highest Router
  ID wins, with `always` and `never` overrides. Requiring the Nt-bit matters: a
  filter on the B-bit alone lets a higher-Router-ID `translate never` ABR wedge
  translation off. A stability grace keeps a router translating after it loses
  the election, so a transient flap opens no Type 5 gap.
- **RFC 3101 Section 2.5 preference is a NEW primary key on the external
  candidate**, compared ahead of the Section 16.4 E1 and E2 rank and cost:
  Type 7 with P=1, then Type 5, then Type 7 with P=0.
  <!-- source: internal/plugins/ospf/spf/external_nssa.go -- ExternalPrefType7P1, ExternalPrefType5, ExternalPrefType7P0 -->

## Traps

- **NSSA reconciliation runs from two goroutines**, the config-apply path and
  the 1-second tick. Its read, compute and write spans a lock release, so it is
  serialized by a dedicated NSSA mutex. Lock order: NSSA mutex, then engine
  mutex, then LSDB mutex. LSDB origination and purge run with the engine mutex
  released, and the NSSA and default-information mutexes are never held
  together.
- **A P-bit toggle on an unchanged body must still re-originate.** A body-only
  comparison drops a `candidate` to `always` transition silently. The stored
  header P-bit is compared as well.
- **A translated Type 5 shares the AS-wide store and key with a self-redistributed
  Type 5.** Without an ownership protocol the translator clobbers a network it
  also redistributes, and a peer Type 7 withdrawal MaxAge-purges the
  redistributed Type 5 AS-wide. The translator skips a network it already
  redistributes and never purges a redistribute-owned key (RFC 3101 Section 3.6).
  Any two uncoordinated reconcilers writing the same self-LSA key need an
  explicit claim protocol. "They use different prefixes in practice" is not a
  guarantee, and a disjoint-prefix test hides it.
