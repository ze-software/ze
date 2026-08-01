# 971 - OSPFv2 MUST-compliance remediation (spec-ospf-14)

## Context

Audit-then-fix over the working OSPFv2 engine (ospf-1..13). `plan/audits/ospf-v2-must-audit.md` listed
62 RFC-MUST findings; independently re-verified (~94% accurate) and remediated 18 real gaps
across authentication (RFC 2328 App D / RFC 5709 / RFC 7474), config validation (RFC 2328
App C.3), flooding (RFC 2328 §13 + Table 19), and NSSA (RFC 3101). OSPFv3
(`internal/plugins/ospfv3/`) was paused for the duration and is a separate plugin sharing no
code -- every edit target was v3-safe (guarded by `ospfv3/types/imports_test.go`).

## Decisions

- **AC-14 scoped to totally-stubby (no-summary) NSSAs only**, not blanket auto-origination.
  RFC 3101 §2.3: a no-summary NSSA's internal routers have no other path to AS-externals, so
  its ABR MUST auto-originate the Type 7 default; a *regular* NSSA stays operator-gated
  (`default-originate`). Matches FRR/Cisco; avoids a behavior change for plain NSSAs.
  (`nssa.go applyNSSADefaults`: `isABR && (a.NoSummary || a.NSSADefaultOriginate)`.)
- **AC-18 boot count = ZeFS-persisted, hashed-clock fallback.** RFC 7474 §3 needs the
  aggregate 64-bit sequence to strictly increase across cold restart. Authoritative source is
  a ZeFS blob (`KeyOSPFAuthBootCount`, registered in `pkg/zefs/keys.go`) read+incremented once
  per boot in `newEngine`. When ZeFS is unavailable, the seed is SHA-1(`time.UnixNano()`)
  truncated to 32 bits -- a plain seconds-granularity wall clock collides on a fast restart, so
  the nanosecond clock is *diffused*, not used directly.
- **P-bit policy and higher-RID-Type5 suppression live at the LSDB origination boundary**,
  not at callers. `OriginateNSSA` clamps the P-bit (zero-FA or self-Type-5 -> P=0); a translator
  consults `HigherRIDType5Exists` so only the highest-Router-ID translator injects the Type 5.
  No caller can bypass the invariant.
- **AC-1 trailer length is exact (`!= len(wire)`), not `>=`.** Accepting trailing bytes after
  the AuType 2/3 digest is a parse/auth gap; both branches now require an exact match.
- **AC-5 recvSeq reset wired through the NSM**, not just exposed. `nsmAdapter` gained an `auth`
  field; `NeighborDown`/`InterfaceDown` call `resetNeighbor`/`resetInterface`, so the crypto
  replay high-water actually clears on real transitions.

## Gotchas

- **Injected "new-diagnostics" / lint warnings LAG and go STALE.** They repeatedly reported
  freshly-added symbols as undefined / imports as unused when they were correct. Authoritative
  signal is `go vet` + `go test` + grep, never the injected diagnostics.
- **Two flooding tests encoded the OLD buggy behavior.** `TestOSPFRetransmitTimer` (first tick)
  and `TestOSPFAckDecisionTable` (implied-ack) had to be updated to the RFC-correct expectation
  when the bug was fixed. This is a correctness sync, not weakening a test -- comments cite the RFC.
- **`floodExcept` signature change rippled to 6 callers** (origination.go, aging.go) for the
  DR-reflood-back / sender-RouterID args. A `replace_all` missed one site first pass; sibling
  call-site audit is mandatory after a flood-helper signature change.
- **AC-3/6/7 are wired to `ze config validate`, not just the engine.**
  `verifyOSPFConfigSections` (register.go:99 `InProcessConfigVerifier`) routes the validate CLI
  through `parseOSPFConfig` -> `validateConfig`, so the functional `.ci` rejection cases prove
  the user-facing surface. Cost 0 / transmit-delay 0 are caught first by the YANG range
  (`invalid value for cost`); key-id 256 (YANG `uint32`, no range) is caught by `ErrKeyIDTooWide`.
- **AC-14 end-to-end is Linux-CI/QEMU-only.** The engine-level unit test
  (`TestOSPFNSSATotallyStubbyAutoDefault`) proves the auto-origination, but a daemon `.ci`
  cannot: an ABR needs running interfaces in TWO areas (backbone + NSSA) and the loopback-only
  daemon harness has a single always-present interface. The interop variant belongs in
  `ospf-stub-nssa-frr` (which currently tests only the *explicit* `default-originate` path) and
  is left as an open item -- `test/interop/interop.py` was being rewritten by a concurrent session.
- **OSPFv3 isolation held throughout.** v2 and v3 share no code; `go vet`/`go test` on
  `internal/plugins/ospfv3/...` stayed green after every v2 change.

## Verification anchors

- Unit (per AC): `TestOSPFAuthCryptoRejectsExtraTrailerBytes`/`...KeyIDMismatch` (AC-1/2),
  `TestAuType2KeyIDBoundary` (AC-3), `TestOSPFAuthType3SourceBinding` (AC-4),
  `TestNeighborDownResetsCryptoSeq`/`TestInterfaceDownResetsAllCryptoSeq` (AC-5),
  `TestInterfaceCostAndTransmitDelayBoundary` (AC-6/7),
  `TestOSPFMaxSeqMaxAgeSilentDiscard` (AC-8), `TestOSPFRetransmitTimer` (AC-9),
  `TestOSPFAckDecisionTable`/`TestOSPFDRRefloodsBackOutReceivingInterface` (AC-10),
  `TestOSPFSelfOriginatedNoLocalCopyFlush` (AC-11), `TestOSPFNSSAPBitBoundaryPolicy` (AC-12),
  `TestOSPFNSSAPbitNotTranslated` (AC-13), `TestOSPFNSSATotallyStubbyAutoDefault` (AC-14),
  `TestOSPFNSSANoTranslateWhenNotElected`/`...NonCandidateDoesNotWedge` (AC-15),
  `TestOSPFHigherRIDType5Exists`/`TestOSPFNSSAHigherRIDType5Suppresses` (AC-16),
  `TestSignKeySelectsByLifetime`/`TestKeyRolloverGapRejected` (AC-17),
  `TestBootCountMonotonicAcrossRestart`/`...NilStoreFallsBack`/`TestSetBootCountSeedsSequence` (AC-18).
- Functional (darwin): `test/ospf/ospf-config.ci` seq 5/6/7 (cost/transmit-delay/key-id
  rejection through `ze config validate`); `bin/ze-test ospf` 13/13.
- Sweep: `go vet` ospf+zefs clean; `go test ./internal/plugins/ospf/... ./pkg/zefs/` green;
  `make ze-lint-changed` 0 issues (26 packages); OSPFv3 compiles + tests pass.
- Open item: AC-14 no-summary auto-default interop variant (Linux-CI/QEMU, not yet added).

## Files

None recorded.
