# Spec: ospf-14-must-remediation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-12-auth.md, spec-ospf-7-lsdb-flooding.md, spec-ospf-11-stub-nssa.md, spec-ospf-4-component-config.md, spec-ospf-5-interface-ism.md |
| Phase | 5/5 (code + tests complete; pending commit) |
| Updated | 2026-06-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ospf-v2-must-audit.md` (repo root) - the source audit, independently re-verified
4. `tmp/session/session-state-760f29b8-fabd-4bd2-bc47-059216034519.md` - file digests
5. Source: `internal/plugins/ospf/packet/auth_verify.go`, `auth_keystore.go`, `auth_wiring.go`,
   `lsdb/flooding.go`, `lsdb/origination.go`, `nssa.go`, `lsdb/nssa.go`, `config.go`,
   `yang/ze-ospf-conf.yang`
6. RFCs: `rfc/short/rfc2328.md`, `rfc/short/rfc3101.md`, `rfc/short/rfc5709.md`, `rfc/short/rfc7474.md`

## Task

Remediate the verified MUST-compliance gaps in the OSPFv2 implementation
(`internal/plugins/ospf/`) found by `ospf-v2-must-audit.md` and independently
re-verified. 18 findings across authentication, config validation, flooding,
and NSSA. OSPFv3 (`internal/plugins/ospfv3/`) is a separate plugin sharing no
code; it is PAUSED during this work and must not be touched.

## Required Reading

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` - base OSPFv2 (flooding §13, InfTransDelay App C.3, auth App D)
  -> Constraint: §13 step 5b MaxAge+MaxSeqNumber older instance is discarded silently (no ack).
  -> Constraint: §13.4 self-originated LSA with no DB copy is flushed by premature aging.
  -> Constraint: App C.3 InfTransDelay MUST be > 0; interface output cost MUST be > 0.
- [ ] `rfc/short/rfc3101.md` - NSSA
  -> Constraint: §2.4 NSSA ABR MUST originate a default into every attached NSSA; P-bit clear on it.
  -> Constraint: Type-7 default installed only if P-bit set; P-bit policy bound to origination.
  -> Constraint: §3.1 translator = reachable NSSA border router with Nt set OR highest Router ID.
- [ ] `rfc/short/rfc5709.md` - HMAC-SHA
  -> Constraint: §3.2 algorithm+key selected implicitly from packet Key ID; trailer length == Auth Data Len.
- [ ] `rfc/short/rfc7474.md` - AuType 3 extended sequence
  -> Constraint: §4 first 4 octets of Apad = packet IP source address.
  -> Constraint: aggregate 64-bit sequence strictly increases for router lifetime incl. cold restart.

**Key insights:**
- OSPFv3 is a separate plugin; every edit target here is v3-safe.
- Boot-count persistence (AC-18) uses ZeFS per user direction.

## Current Behavior (MANDATORY)

**Source files read:** see `tmp/session/session-state-760f29b8-fabd-4bd2-bc47-059216034519.md` for per-file digests.
- [ ] `internal/plugins/ospf/packet/auth_verify.go` - Verify accepts extra trailer bytes; Apad fixed.
- [ ] `internal/plugins/ospf/auth_keystore.go` - verify loops all keys (no Key-ID match); recvSeq never reset.
- [ ] `internal/plugins/ospf/auth_wiring.go` - sign/verify hooks; no source IP threaded.
- [ ] `internal/plugins/ospf/config.go` - parseInterface accepts cost 0 and transmit-delay 0.
- [ ] `internal/plugins/ospf/lsdb/flooding.go` - Older branch sends DB copy back; retransmit last unset.
- [ ] `internal/plugins/ospf/lsdb/origination.go` - self-orig with no local copy dropped, not flushed.
- [ ] `internal/plugins/ospf/nssa.go` - translator election Router-ID only; default origination config-gated.

**Behavior to preserve:**
- AuType 0/1/2/3 sign/verify wire formats unchanged; hitless key rotation on receive.
- Existing config defaults (cost defaults to 1, transmit-delay to 1 when unset).
- Flooding/LSDB install path through `normaliseLSA` (rejects unknown LS types).

**Behavior to change (all RFC-driven, user approved scope A+B+C):**
- AC-14: a totally-stubby (no-summary) NSSA ABR auto-originates the Type-7 default
  (RFC 3101 §2.3 -- its internal routers have no other path to externals); a regular
  NSSA stays operator-gated via `default-originate`. (Previously all NSSA defaults were
  config-gated, leaving totally-stubby NSSAs without a mandatory default.)
- AC-3/AC-6/AC-7: reject explicit out-of-range config values previously silently accepted.

## Data Flow (MANDATORY)

### Entry Point
- Received OSPF packets enter via `transport` RX, then `dispatch`, then `verifyPacket` (auth), then handlers.
- Config enters via the YANG tree, then `parseInterface` / validators, producing `ospfConfig`.

### Transformation Path
1. RX: `transport.RawPacket` -> `engine.verifyPacket` -> `authStore.verify` -> `packet.Verify`.
2. TX: handler builds packet -> `engine.signPacket` -> `authStore.signKey` -> `packet.Sign`.
3. Flooding: `lsdb.ReceiveUpdate` -> `install`/`normaliseLSA` -> retransmit lists / acks.
4. NSSA: translator/default logic in `nssa.go` -> `lsdb.OriginateNSSA` -> area store.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Transport ↔ engine | `RawPacket` (IfIndex, Source, Payload) | [ ] |
| Engine ↔ packet codec | `Sign`/`Verify` over wire bytes | [ ] |
| Config ↔ engine | `ospfConfig` struct from YANG | [ ] |

### Integration Points
- `engine.verifyPacket` / `signPacket` (auth_wiring.go) - sign/verify hooks.
- `lsdb.LSDB` install/flood/ack - flooding edges.
- `validators_register.go` - config validators.

### Architectural Verification
- [ ] No bypassed layers (auth stays in the verify/sign chokepoints)
- [ ] No unintended coupling (no OSPFv3 imports)
- [ ] No duplicated functionality (extends existing auth/flooding/nssa code)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | OSPFv3 shares no code with `internal/plugins/ospf/` | sharing-map agent + `ospfv3/types/imports_test.go` | edits could break v3 | grep importers | confirmed |
| A-2 | ZeFS provides durable persistence usable from the ospf plugin | user direction | boot-count not persistable | read zefs API before AC-18 | unvalidated |
| A-3 | Packet Key ID is recoverable on receive (AuType 2 = 1 byte, AuType 3 = 4 bytes) | auth_verify.go af layout | Key-ID match impossible | implemented in AC-2 | unvalidated |
| A-4 | Source IP is available at the verify hook (rp) and sign hook (interface addr) | auth_wiring.go RawPacket | Apad source-IP impossible | read transport.RawPacket | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | AC-14 automatic default origination changes operator-visible behavior | interop/functional test diff | keep config control; document in guide |
| R-2 | Source-IP Apad (AC-4) breaks interop with peers using fixed Apad | interop test vs FRR/BIRD | match RFC 7474 exactly; interop test |
| R-3 | Key-ID strict match (AC-2) breaks rotation | functional auth test | per-ID chain lookup preserves rotation |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Received AuType 2/3 packet with trailing bytes | → | `packet.Verify` exact-length check | `TestOSPFAuthCryptoRejectsExtraTrailerBytes` |
| Received crypto packet, wrong Key ID | → | `packet.Verify` Key-ID match | `TestOSPFAuthCryptoRejectsKeyIDMismatch` |
| `ze config validate` with `key-id 256` (AuType 2) | → | `validateConfig` `ErrKeyIDTooWide` via `verifyOSPFConfigSections` | `test/ospf/ospf-config.ci` (AC-3 case) |
| Neighbor transitions to Down | → | `authStore.resetNeighbor` via `nsmAdapter` | `TestNeighborDownResetsCryptoSeq` |
| `ze config validate` with `cost 0` | → | `validateConfig` `ErrInterfaceCostZero` | `test/ospf/ospf-config.ci` (AC-6 case) |
| `ze config validate` with `transmit-delay 0` | → | YANG range `1..3600` + `ErrTransmitDelayZero` | `test/ospf/ospf-config.ci` (AC-7 case) |
| Totally-stubby (no-summary) NSSA ABR | → | `applyNSSADefaults` auto Type-7 default | `TestOSPFNSSATotallyStubbyAutoDefault`; interop `ospf-stub-nssa-frr` |

## Acceptance Criteria

### Phase 1 -- Authentication security (Tier A + B)
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | AuType 2/3 packet with extra bytes after the digest trailer | `Verify` rejects: wire length MUST equal `plen(+8)+L` exactly, not `>=`. |
| AC-2 | Crypto packet whose Key ID does not match the candidate chain key | `verify` selects the chain key by packet Key ID; a digest under a different Key ID is rejected. |
| AC-3 | Config sets `key-id` > 255 with a non-extended (AuType 2) key | Validator rejects (AuType 2 wire Key ID is 1 octet). |
| AC-4 | AuType 3 sign/verify | First 4 octets of Apad are the packet IPv4 source address (RFC 7474 §4). |
| AC-5 | Neighbor transitions to Down | Crypto receive-sequence high-water for that neighbor is cleared. |

### Phase 2 -- Config validation (Tier A)
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-6 | Interface `cost 0` configured explicitly | Rejected (range 1..65535) at parse/validate, not silently accepted. |
| AC-7 | Interface `transmit-delay 0` configured explicitly | Rejected (range 1..3600); YANG range corrected from 0..3600. |

### Phase 3 -- Flooding edges (Tier A + B)
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-8 | DB copy is MaxAge with MaxSequenceNumber; older instance received | Received LSA discarded silently (no DB copy sent back, no ack). |
| AC-9 | LSA newly queued to a retransmission list | First `RetransmitTick` waits a full RxmtInterval (initial `last` set on queue). |
| AC-10 | LSA received as DR/BDR per RFC 2328 Table 19 | Ack behavior follows Table 19, including the flooded-back-out-receiving-interface case. |
| AC-11 | Self-originated LSA received with no local DB copy | Flushed by premature aging (MaxAge), not silently dropped. |

### Phase 4 -- NSSA (Tier B + C)
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-12 | Any caller of `OriginateNSSA` | P-bit policy (P=1 requires non-zero FA; clear when same net is Type-5) enforced at the LSDB origination boundary. |
| AC-13 | Type-7 default (0.0.0.0/0) received at an NSSA border router | Installed into the Type-5 path only if its P-bit is set. |
| AC-14 | Router is the ABR of a totally-stubby (no-summary) NSSA | Auto-originates the Type-7 default (0.0.0.0/0, P=0) per RFC 3101 §2.3; a regular NSSA stays operator-gated (`default-originate`). |
| AC-15 | Translator election with a peer advertising Nt | Election considers the Nt bit, not only highest Router ID. |
| AC-16 | Another equivalent Type-5 from a higher-Router-ID translator exists | Local translation suppressed; no transient duplicate Type-5 during stability grace. |

### Phase 5 -- Key management + extended-sequence persistence (Tier C)
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-17 | Key chain with send/accept lifetimes and rollover | Send key selected by lifetime; rollover overlap validated; no revert to unauthenticated when last key expires. |
| AC-18 | Router cold-restart | Boot count persisted via ZeFS; aggregate 64-bit sequence strictly increases across reboots. |

## 🧪 TDD Test Plan

### Unit Tests (actual, as implemented)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFAuthCryptoRejectsExtraTrailerBytes` | `packet/auth_verify_test.go` | AC-1 | PASS |
| `TestOSPFAuthCryptoRejectsKeyIDMismatch` | `packet/auth_verify_test.go` | AC-2 | PASS |
| `TestAuType2KeyIDBoundary` | `auth_keystore_test.go` | AC-3 | PASS |
| `TestOSPFAuthType3SourceBinding`, `TestApadSrcBoundaries` | `packet/auth_verify_test.go` | AC-4 | PASS |
| `TestNeighborDownResetsCryptoSeq`, `TestInterfaceDownResetsAllCryptoSeq` | `auth_keystore_test.go` | AC-5 | PASS |
| `TestInterfaceCostAndTransmitDelayBoundary` | `config_interface_validate_test.go` | AC-6, AC-7 | PASS |
| `TestOSPFMaxSeqMaxAgeSilentDiscard` | `lsdb/flooding_edges_test.go` | AC-8 | PASS |
| `TestOSPFRetransmitTimer` | `lsdb/flooding_test.go` | AC-9 | PASS |
| `TestOSPFAckDecisionTable`, `TestOSPFDRRefloodsBackOutReceivingInterface` | `lsdb/flooding_test.go`, `lsdb/flooding_edges_test.go` | AC-10 | PASS |
| `TestOSPFSelfOriginatedNoLocalCopyFlush` | `lsdb/flooding_edges_test.go` | AC-11 | PASS |
| `TestOSPFNSSAPBitBoundaryPolicy` | `lsdb/nssa_pbit_test.go` | AC-12 | PASS |
| `TestOSPFNSSAPbitNotTranslated` | `nssa_test.go` | AC-13 | PASS |
| `TestOSPFNSSATotallyStubbyAutoDefault` | `nssa_ac14_16_test.go` | AC-14 | PASS |
| `TestOSPFNSSANoTranslateWhenNotElected`, `TestOSPFNSSANonCandidateDoesNotWedge` | `nssa_test.go` | AC-15 | PASS |
| `TestOSPFHigherRIDType5Exists`, `TestOSPFNSSAHigherRIDType5Suppresses` | `lsdb/nssa_higher_rid_test.go`, `nssa_ac14_16_test.go` | AC-16 | PASS |
| `TestSignKeySelectsByLifetime`, `TestSignKeyNoRevertWhenAllExpired`, `TestSignKeyUnsetLifetimeUsesFirst`, `TestKeyRolloverGapRejected` | `auth_keystore_test.go`, `config_test.go` | AC-17 | PASS |
| `TestBootCountMonotonicAcrossRestart`, `TestBootCountNilStoreFallsBack`, `TestSetBootCountSeedsSequence` | `auth_keystore_test.go` | AC-18 | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interface cost | 1..65535 | 1 | 0 | 65536 |
| transmit-delay | 1..3600 | 1 | 0 | 3601 |
| key-id (AuType 2) | 0..255 | 255 | N/A | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-config` seq 5 | `test/ospf/ospf-config.ci` | `cost 0` rejected by `ze config validate` (exit 1, `invalid value for cost`) | PASS (13/13 suite) |
| `ospf-config` seq 6 | `test/ospf/ospf-config.ci` | `transmit-delay 0` rejected (exit 1, `invalid value for transmit-delay`) | PASS |
| `ospf-config` seq 7 | `test/ospf/ospf-config.ci` | AuType 2 `key-id 256` rejected (exit 1, `AuType 2 key-id must be 0..255`) | PASS |

AC-14 (totally-stubby NSSA auto-default) is runtime LSA origination, not a config-validate
behavior: proven by the unit test `TestOSPFNSSATotallyStubbyAutoDefault` (engine-level). A daemon
`.ci` cannot exercise it because an ABR needs running interfaces in two areas (backbone + NSSA)
and the loopback-only daemon harness has a single always-present interface; the end-to-end path
belongs in QEMU interop (see below).

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-auth-frr` | `test/interop/scenarios/ospf-auth-frr/` | FRR | OSPFv2 cryptographic auth interoperates (AC-1..5 surface) | exists (QEMU-only; not run on darwin) |
| `ospf-stub-nssa-frr` | `test/interop/scenarios/ospf-stub-nssa-frr/` | FRR | Type-7 default reaches FRR via **explicit** `default-originate` (AC-13/18 path) | exists (QEMU-only) |
| AC-14 no-summary auto-default | `test/interop/scenarios/ospf-stub-nssa-frr/` (variant) | FRR | totally-stubby NSSA auto-originates the default **without** `default-originate` | NOT YET ADDED -- see Open Item |

**Open Item (AC-14 interop):** the `ospf-stub-nssa-frr` scenario currently configures an explicit
`default-originate true`, so it does not exercise the no-summary auto-trigger AC-14 adds. A
variant (no-summary NSSA, no `default-originate`, assert FRR still learns 0.0.0.0/0) is the
correct end-to-end coverage. It requires QEMU (cannot run/verify on darwin) and the
`test/interop/` tree is being actively modified by a concurrent session, so it is left as an
explicit open item for the user to schedule rather than added blind. AC-14 behavior itself is
implemented and unit-proven.

## Files to Modify
- `internal/plugins/ospf/packet/auth_verify.go` - AC-1, AC-4 (trailer length, source-IP Apad)
- `internal/plugins/ospf/auth_keystore.go` - AC-2, AC-5, AC-17 (Key-ID match, neighbor-down reset, lifetimes)
- `internal/plugins/ospf/auth_wiring.go` - AC-4 (thread source IP), AC-5 (neighbor-down hook)
- `internal/plugins/ospf/config.go` - AC-3, AC-6, AC-7 (parser guards)
- `internal/component/config/validators_ospf*.go` - AC-3 (key-id width validator)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - AC-7 (transmit-delay range)
- `internal/plugins/ospf/lsdb/flooding.go` - AC-8, AC-9, AC-10
- `internal/plugins/ospf/lsdb/origination.go` - AC-11
- `internal/plugins/ospf/lsdb/nssa.go`, `internal/plugins/ospf/nssa.go` - AC-12..AC-16
- `internal/plugins/ospf/packet/lsa_router.go` - AC-15 (Nt bit)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG validation constraints | yes | `yang/ze-ospf-conf.yang` (transmit-delay range) |
| YANG custom validators | yes | `validators_ospf*.go` (key-id width) |
| Functional test for behavior | yes | `test/cli/*.ci` |

## Files to Create
- `test/cli/ospf-auth-keyid.ci` - AC-3 functional test
- `test/cli/ospf-iface-cost.ci` - AC-6 functional test
- `test/cli/ospf-nssa-default.ci` - AC-14 functional test

## Implementation Steps

### Implementation Phases
1. **Phase 1 — Auth security (AC-1..AC-5):** narrow, high-value, security-sensitive. TDD each.
   - Tests: TestVerifyRejectsExtraTrailerBytes, TestVerifyKeyIDSelectsChainKey, TestNeighborDownResetsCryptoSeq, TestAuType3ApadUsesSourceIP, key-id validator test.
   - Files: auth_verify.go, auth_keystore.go, auth_wiring.go, validators_ospf*.go.
2. **Phase 2 — Config validation (AC-6..AC-7):** parser + YANG + validators + boundary tests.
3. **Phase 3 — Flooding edges (AC-8..AC-11):** RFC 2328 §13 receive/ack/flush semantics.
4. **Phase 4 — NSSA (AC-12..AC-16):** P-bit policy at boundary, auto default, Nt bit, dup suppression.
5. **Phase 5 — Key mgmt + ZeFS persistence (AC-17..AC-18):** lifetimes, rollover, boot-count.

## Implementation Audit

All 18 ACs implemented in `internal/plugins/ospf/` (OSPFv2 only; OSPFv3 untouched and
still compiling/passing). Per-AC enforcement site:

| AC | Enforcement site | Notes |
|----|------------------|-------|
| AC-1 | `packet/auth_verify.go` (AuType 2 + AuType 3 branches) | trailer length `!= len(wire)` (was `>`) |
| AC-2 | `packet/auth_verify.go` | reject when packet Key ID != candidate key |
| AC-3 | `config.go` `validateConfig` (`ErrKeyIDTooWide`) | AuType 2 key-id must be 0..255 |
| AC-4 | `packet/auth_verify.go` `apadSrc` + `auth_wiring.go` (source IP threaded) | RFC 7474 §4 source-IP Apad |
| AC-5 | `auth_keystore.go` `resetNeighbor`/`resetInterface`, wired in `instance.go` nsmAdapter | clears recvSeq on Down |
| AC-6 | `config.go` `validateConfig` (`ErrInterfaceCostZero`) | cost > 0 |
| AC-7 | `config.go` (`ErrTransmitDelayZero`, `HasTransmitDelay`) + `yang/ze-ospf-conf.yang` range `1..3600` | InfTransDelay > 0 |
| AC-8 | `lsdb/flooding.go` (Older branch) | MaxAge+MaxSeq older instance silently discarded |
| AC-9 | `lsdb/flooding.go` `queueRetransmit` stamps `last` | first tick waits full RxmtInterval |
| AC-10 | `lsdb/flooding.go` `floodExcept`/`ackForReceive` (Table 19 + DR reflood-back) | corrected ack matrix |
| AC-11 | `lsdb/origination.go` `flushReceivedSelfLSA` | self LSA with no local copy flushed (MaxAge, seq.Next) |
| AC-12 | `lsdb/nssa.go` `OriginateNSSA` (P-bit policy) | P=1 requires non-zero FA; clear if self Type-5 |
| AC-13 | `nssa.go` `translateNSSA` (OptionNP gate) | P=0 Type-7 never translated |
| AC-14 | `nssa.go` `applyNSSADefaults` (`isABR && (NoSummary || NSSADefaultOriginate)`) | totally-stubby NSSA auto-default |
| AC-15 | `nssa.go` `nssaABRs` (Nt-bit filter) | election considers Nt-bit, not only Router ID |
| AC-16 | `lsdb/nssa.go` `HigherRIDType5Exists`, used in `nssa.go` `translateNSSA` | suppress duplicate Type-5 |
| AC-17 | `auth_keystore.go` `selectSendKey`/lifetime fields + `config.go` `validateKeyRollover` | lifetime selection, rollover gap, no revert |
| AC-18 | `auth_keystore.go` `loadOSPFBootCount`/`bootCountFromClock`/`setBootCount` + `pkg/zefs` `KeyOSPFAuthBootCount` | ZeFS-persisted boot count, hashed-clock fallback |

Wiring verified: `verifyOSPFConfigSections` (register.go:99 `InProcessConfigVerifier`) routes
`ze config validate` through `parseOSPFConfig` -> `validateConfig`, so AC-3/6/7 reject at the
user-facing validate surface. AC-5 reset is reachable from the NSM Down transitions via the
`nsmAdapter.auth` field. AC-18 boot count is seeded once per boot in `newEngine`.

## Review Gate

| Severity | Count | Detail |
|----------|-------|--------|
| BLOCKER | 0 | — |
| ISSUE | 0 | — |

Self-review + automated verification performed:
- `go vet ./internal/plugins/ospf/... ./pkg/zefs/` — clean (exit 0).
- `go test ./internal/plugins/ospf/... ./pkg/zefs/ -count=1` — all packages ok (incl. lsdb, packet, nssa, redistribute, zefs).
- OSPFv3 regression check: `go vet ./internal/plugins/ospfv3/...` clean; `go test ./internal/plugins/ospfv3/...` ok (v2 changes did not break the paused v3 plugin).
- `make ze-lint-changed` — 0 issues across 26 changed packages.

The formal `/ze-review` skill can be re-run at the user's discretion; the equivalent gates
(lint, vet, full changed-scope test) are green.

## Pre-Commit Verification

| Gate | Result |
|------|--------|
| `go vet` (ospf + zefs) | PASS |
| `go test` (ospf subpackages + zefs) | PASS |
| OSPFv3 compile + test (not broken) | PASS |
| `make ze-lint-changed` | PASS (0 issues) |
| Functional `.ci` (config-validate rejections AC-3/6/7) | PASS -- `bin/ze-test ospf` 13/13, `ospf-config` suite incl. 3 new cases |

Open item (not blocking the code fix, but tracked honestly):
- AC-14 end-to-end interop variant (no-summary NSSA auto-default without `default-originate`)
  is QEMU-only and not yet added; behavior is unit-proven. See Interop Tests > Open Item.

Not yet done (user-triggered closure, per `.claude/rules/planning.md`):
- Learned summary `plan/learned/NNN-ospf-14-must-remediation.md` + `.counter` bump + `ai/LEARNED-INDEX.md` entry.
- Commit A (code + tests + docs + spec + learned) then Commit B (`git rm` this spec).

## RFC Documentation

RFC constraints are annotated inline at the enforcing code (`// RFC NNNN §X: "<requirement>"`).
Primary: RFC 2328 §13 (flooding), App C.3 (positive cost/delay); RFC 3101 §2.4/§3.1 (NSSA);
RFC 5709 §3.2 (key selection); RFC 7474 §4 (source-IP Apad), §5/§6 (sequence/protocol-id).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Commit A: code + tests + docs + spec + learned summary + counter bump
- [ ] Commit B: `git rm plan/spec-ospf-14-must-remediation.md` only
