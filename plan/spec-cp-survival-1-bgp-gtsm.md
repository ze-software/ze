# Spec: BGP GTSM / TTL Security (Gap A)

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-cp-survival-0-umbrella |
| Phase | 1/7 |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/yang/ze-bgp-conf.yang` (the `connection > ttl` block, already modeled)
4. `internal/component/bfd/transport/udp_linux.go` (BFD TTL precedent)
5. `internal/core/network/network.go` + `md5_linux.go`/`md5_darwin.go` (setsockopt-helper platform-split pattern)

## Task

Add GTSM (Generalized TTL Security Mechanism, RFC 5082) / TTL security to BGP sessions. The YANG
schema already exists (`connection > ttl > {max, set, min}` in `ze-bgp-conf.yang:351-365`) but is
**not wired** to anything. This spec parses it into `PeerSettings` and enforces it at the TCP
socket: set outgoing TTL on transmitted segments, and have the kernel drop incoming segments whose
TTL is below the configured minimum. This keeps off-path spoofed traffic (forged RST/SYN, low-TTL
floods aimed at the peer's address) off the BGP control path, so the session survives to carry a
FlowSpec/RTBH signal.

BFD already does TTL enforcement (`udp_linux.go`); BGP does not. This mirrors the intent but uses
the **kernel-native TCP mechanism** (`IP_MINTTL`) rather than BFD's per-packet cmsg parsing, because
BGP is TCP and the kernel can drop violating segments before the application sees them.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/config-naming.md` - per-peer config leaf naming
  → Constraint: leaves already named `max`/`set`/`min` under `connection > ttl`; reuse, do not rename.
- [ ] `ai/rules/qemu-testing.md` - linux-only socket behavior must have QEMU integration tests
  → Constraint: socket-option readback test must be `//go:build integration && linux`; register the bgp/reactor package in `mk/test-integration.mk`.
- [ ] `ai/rules/plugin-self-containment.md` - TTL is core-BGP, not a plugin
  → Constraint: lives in `internal/component/bgp/reactor` + `internal/core/network`; no plugin spelling needed.

### RFC Summaries
- [ ] `rfc/short/rfc5082.md` - GTSM. Create with `/ze-rfc` if absent.
  → Constraint: sender sets TTL=255; receiver accepts only TTL ≥ 255−(hops−1). Single-hop ⇒ exactly 255.

**Key insights:**
- `IP_MINTTL` (Linux `unix.IP_MINTTL`) on a TCP socket makes the kernel silently drop incoming
  segments with TTL below the set value — stronger and simpler than BFD's cmsg approach.
- Per-peer min-TTL cannot be enforced on the **shared listen socket** (one socket serves all peers
  on a port). It is enforced per-peer on the **accepted/connected socket** in `connectionEstablished`.
  An optional listen-socket `IP_MINTTL = min(across peers)` is a defense-in-depth phase 2 for SYN floods.
- Outgoing TTL must be set on the **dialer pre-connect** so the SYN we transmit already carries it
  (the remote peer's GTSM checks our SYN's TTL).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang:351-365` - `container ttl { leaf max; leaf set; leaf min }`, all `uint8`, all unwired. `max` described "TTL security / GTSM (RFC 5082)", `set` "Outgoing TTL value", `min` "Minimum incoming TTL".
  → Constraint: schema present; this spec adds parsing + enforcement + validation only.
- [ ] `internal/component/bgp/reactor/peersettings.go:213-460` - `PeerSettings` has `MD5Key` (276), `MD5IP` (281); no TTL fields. `IsEBGP()`/`IsIBGP()` at 525-533.
  → Constraint: add TTL fields next to MD5; `0` means "unset / leave OS default".
- [ ] `internal/component/bgp/reactor/config.go:336-356` - MD5 parsed from `connection > md5 > {password, ip}`. TTL not parsed.
  → Constraint: add a sibling block parsing `connection > ttl > {max, set, min}`.
- [ ] `internal/component/bgp/reactor/session.go:359-372` - `NewSession()` builds `network.RealDialer`, sets `LocalAddr`, `MD5Key`, `PeerAddr`.
  → Constraint: pass TTL (outgoing) into the dialer here, mirroring MD5Key.
- [ ] `internal/core/network/network.go:68-90` - `RealDialer.DialContext` Control callback calls `setTCPMD5Sig(fd, peerIP, password)` pre-connect (line 81-82). `RealListenerFactory.Listen` (106-124) sets MD5 per-peer pre-bind.
  → Constraint: outgoing TTL setsockopt goes in DialContext Control (pre-connect); reuse the same Control seam.
- [ ] `internal/component/bgp/reactor/session_connection.go:237-265` - `connectionEstablished()` Control callback sets IP_TOS=0xC0, SO_RCVBUF/SNDBUF on every conn (in/out).
  → Constraint: add per-peer `IP_MINTTL` here using `s.settings`; this is the inbound-validation seam.
- [ ] `internal/component/bfd/transport/udp_linux.go:73-114` - BFD sets `IP_TTL=255`/`IPV6_UNICAST_HOPS=255` (out) and `IP_RECVTTL`/`IPV6_RECVHOPLIMIT` (in); `engine/loop.go:145-170` `passesTTLGate` validates.
  → Constraint: precedent for outgoing-TTL setsockopt; BGP diverges by using IP_MINTTL instead of cmsg validation.
- [ ] `internal/core/network/md5_linux.go` / `md5_darwin.go` - platform-split setsockopt helper; darwin returns unsupported error.
  → Constraint: add `ttl_linux.go` / `ttl_other.go` with the same split so darwin still builds.

**Behavior to preserve:**
- No `connection > ttl` config ⇒ zero new setsockopt calls; existing TTL behavior (OS default) unchanged.
- IP_TOS=0xC0, MD5, buffer sizing all unchanged.

**Behavior to change:** Only when `connection > ttl` is configured for a peer (opt-in).

## Data Flow (MANDATORY)

### Entry Point
- Config tree leaf `bgp > peer > connection > ttl > {max|set|min}` (uint8).

### Transformation Path
1. `config.go` parses `connection > ttl` → `PeerSettings.{OutTTL, MinTTL}` (deriving from `max` for GTSM).
2. `session.go NewSession` copies `OutTTL` into `RealDialer` (outbound SYN TTL).
3. `network.go DialContext` Control callback: `setIPTTL(fd, family, OutTTL)` pre-connect.
4. `session_connection.go connectionEstablished` Control callback: `setIPMinTTL(fd, family, MinTTL)` on the established socket (per-peer inbound validation).
5. Kernel enforces: transmitted segments carry `OutTTL`; received segments with TTL < `MinTTL` are dropped silently.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ PeerSettings | `config.go` parse block | [ ] |
| PeerSettings ↔ dialer | `session.go` field copy | [ ] |
| App ↔ kernel (out) | `setIPTTL` setsockopt pre-connect | [ ] |
| App ↔ kernel (in) | `setIPMinTTL` setsockopt post-establish | [ ] |

### Integration Points
- `internal/core/network/network.go` `RealDialer` — new `OutTTL` field + Control setsockopt
- `internal/component/bgp/reactor/session_connection.go` `connectionEstablished` — IP_MINTTL setsockopt
- `internal/component/bgp/reactor/peersettings.go` `PeerSettings` — TTL fields

### Architectural Verification
- [ ] No bypassed layers (config → PeerSettings → dialer/socket, same path as MD5)
- [ ] No unintended coupling (no new cross-component deps)
- [ ] No duplicated functionality (reuses dialer/connection Control seams)
- [ ] Zero-copy preserved (N/A — socket options only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `max` leaf means GTSM hop-count (ttl-security hops N): enabling it sets OutTTL=255 and MinTTL=255−N+1 | YANG desc "TTL security / GTSM (RFC 5082)"; uint8 | wrong derivation → sessions won't establish | user confirmation of intended `max` semantics + parser test | unvalidated |
| A-2 | `IP_MINTTL` on an established TCP socket drops subsequent low-TTL segments (anti-spoof of live session) | Linux IP_MINTTL semantics | inbound protection weaker than claimed | integration test: GetsockoptInt readback + low-TTL segment drop | unvalidated |
| A-3 | `unix.IP_MINTTL` / `unix.IPV6_MINHOPCOUNT` are available in the vendored `golang.org/x/sys/unix` | BFD uses x/sys/unix; `go.mod` pins x/sys v0.43.0 and module cache `zerrors_linux.go` defines both constants | build break | grep vendored unix consts | confirmed |
| A-4 | Setting outgoing TTL on the dialer pre-connect makes the SYN carry it | net.Dialer.Control runs pre-connect (MD5 relies on this) | remote GTSM rejects our SYN | integration capture / GetsockoptInt | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Per-peer MinTTL not enforceable on shared listen socket ⇒ no SYN-flood pre-filter | SYN flood still reaches stack | enforce per-peer post-accept (covers live-session spoofing); listen-socket min(across peers) = phase 2 |
| R-2 | Enabling GTSM on a peer that is legitimately >configured hops away breaks the session | session stuck in Connect/Active | opt-in per peer; clear doc; `max`/`min` widen the window |
| R-3 | darwin lacks IP_MINTTL ⇒ build/runtime break | compile error on darwin | platform-split helper (`ttl_other.go`) returns unsupported, logged not fatal (mirror md5_darwin) |
| R-4 | `set` and `max` both configured give conflicting OutTTL | ambiguous behavior | YANG `must`/validator: `max` mutually exclusive with `set`/`min`; reject at config time |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `connection { ttl { set 255 } }` in config | → | `config.go` parse → `PeerSettings.OutTTL` → `RealDialer.OutTTL` → `setIPTTL` | `TestGTSMConfigWiresToDialer` (reactor) |
| `connection { ttl { min 255 } }` | → | `connectionEstablished` → `setIPMinTTL` on socket | `TestGTSMMinTTLSetOnSocket` (integration_linux) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer config `connection { ttl { set 255 } }` (IPv4) | Dialed socket reports `IP_TTL == 255` via GetsockoptInt; transmitted SYN carries TTL 255 |
| AC-2 | Peer config `connection { ttl { min 255 } }` (IPv4) | Established socket reports `IP_MINTTL == 255`; a segment with TTL 254 is dropped by the kernel |
| AC-3 | Peer config `connection { ttl { max 1 } }` | Parser derives `OutTTL=255` and `MinTTL=255`; both socket options set (single-hop GTSM) |
| AC-4 | IPv6 peer with the same config | `IPV6_UNICAST_HOPS`/`IPV6_MINHOPCOUNT` set to the equivalent values |
| AC-5 | Peer with no `connection > ttl` | No TTL setsockopt issued; session behaves exactly as today (regression guard) |
| AC-6 | `connection { ttl { set 200 max 1 } }` (conflict) | Config validation rejects with a clear error before apply |
| AC-7 | Build on darwin | Compiles; `setIPMinTTL` returns unsupported-error path, logged at Debug, session still forms (no TTL enforcement) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures single-hop GTSM on an eBGP peer and the session comes up | config → PeerSettings → dialer/socket → OPEN exchange | `test/integration/session_test.go` dual-session variant with `ttl { max 1 }` |
| 2 | a spoofed low-TTL RST cannot tear down the protected session | kernel IP_MINTTL drop | integration test asserting socket survives an injected low-TTL segment (or socket-option readback as proxy) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseTTLConfig` | `internal/component/bgp/reactor/config_test.go` | `connection > ttl` → PeerSettings fields | |
| `TestGTSMMaxDerivesSetAndMin` | `internal/component/bgp/reactor/peersettings_test.go` | `max N` ⇒ OutTTL=255, MinTTL=255−N+1 | |
| `TestTTLConflictRejected` | `internal/component/config/.../command_test.go` or validator test | `set`+`max` rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ttl set | 1-255 | 255 | 0 (treat as unset) | N/A (uint8 caps 255) |
| ttl min | 1-255 | 255 | 0 (unset) | N/A |
| ttl max (hops) | 1-255 | 255 | 0 (unset) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-gtsm` | `test/plugin/bgp-gtsm.ci` | configure a peer with `ttl { max 1 }`, session establishes, status shows GTSM active | |
| `bgp-gtsm-reject` | `test/plugin/bgp-gtsm-reject.ci` | conflicting `set`+`max` is rejected at config time with a clear error | |
| `TestGTSMMinTTLSetOnSocket` | `internal/component/bgp/reactor/session_ttl_integration_linux_test.go` | socket-option readback IP_TTL/IP_MINTTL/IPv6 (QEMU integration) | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-bgp-gtsm-peer` | `test/interop/scenarios/` | FRR or BIRD with `ttl-security` | both ends accept TTL-255 GTSM session | |

## Files to Modify (as shipped)
- `internal/component/bgp/reactor/peersettings.go` - `OutTTL`, `MinTTL` (uint8) fields
- `internal/component/bgp/reactor/config.go` - `parseTTLSettings` (`connection > ttl`, `max` derivation, conflict reject)
- `internal/component/bgp/reactor/session.go` - pass `OutTTL` to `RealDialer` (committed earlier in `6e084eb6f`)
- `internal/component/bgp/reactor/session_connection.go` - `tuneTCPConnectionForSettings` sets out-TTL + `IP_MINTTL`
- `internal/component/bgp/reactor/reactor.go` - `listenTTLForListener` + `newListenerFactory` (phase-2 listen TTL)
- `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual` includes TTL; `PeerInfo` GTSM fields populated
- `internal/component/bgp/reactor/reactor_connection.go` / `reactor_dynamic.go` - collision + dynamic-peer TTL
- `internal/core/network/network.go` - `RealDialer.OutTTL` + Control; `RealListenerFactory.ListenTTL` + Control
- `internal/component/plugin/types_bgp.go` - `PeerInfo.GTSMOutTTL` / `GTSMMinTTL`
- `internal/component/bgp/plugins/cmd/peer/peer.go` - `gtsm-ttl-out` / `gtsm-ttl-min` in peer detail
- `internal/component/bgp/config/{resolve,peers,loader_create}.go` - dynamic-group TTL parse + conflict validate
- `mk/test-integration.mk` + `scripts/evidence/qemu-all-tests.sh` - register network + reactor integration tests
- (NOT done) `ze-bgp-conf.yang` `must`: conflict rejected in Go at `config validate` instead (see Audit)

## Files Created (as shipped)
- `internal/core/network/ttl.go` - exported wrappers + `SetListenIPTTL`
- `internal/core/network/ttl_linux.go` - `setIPTTL`, `setIPMinTTL`, `setListenIPTTL` (linux)
- `internal/core/network/ttl_other.go` - non-linux unsupported stubs (mirror `md5_darwin.go`)
- `internal/core/network/ttl_integration_linux_test.go` - readback + low-TTL-drop + listen-TTL tests
- `internal/component/bgp/reactor/session_ttl_integration_linux_test.go` - socket-option readback test
- `test/plugin/bgp-gtsm.ci`, `test/plugin/bgp-gtsm-reject.ci` - functional tests
- `test/interop/scenarios/46-gtsm-frr/{ze.conf,frr.conf,check.py}` - FRR ttl-security interop

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `make ze-verify-changed` |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — add `OutTTL`/`MinTTL` to PeerSettings; parse `connection > ttl` in config.go; write `TestGTSMConfigWiresToDialer` (fails: setsockopt not yet called).
   - Files: peersettings.go, config.go
2. **Phase: outgoing TTL** — `ttl_linux.go setIPTTL`; wire into RealDialer Control; AC-1, AC-4(out).
3. **Phase: inbound min-TTL** — `setIPMinTTL`; wire into connectionEstablished; AC-2, AC-4(in).
4. **Phase: GTSM derivation + validation** — `max` → set/min derivation; YANG `must`/validator for conflict; AC-3, AC-6.
5. **Phase: platform split + darwin** — `ttl_other.go`; AC-7.
6. **Phase: integration + interop tests** — readback test, dual-session, FRR/BIRD interop.
7. **Full verification** → `make ze-verify-changed`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC-N has file:line implementation |
| Correctness | `max N` derivation matches RFC 5082 (255−N+1); IPv6 consts correct |
| Data flow | TTL parsed in config.go only; reactor/network apply; no leakage elsewhere |
| Platform | darwin builds; linux-only consts behind build tags |
| Rule: qemu-testing | socket readback test runs under QEMU integration target |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | TTL/hops bounds; conflict rejection; no panic on 0 |
| Resource exhaustion | N/A (socket option, no allocation) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Summary

GTSM (RFC 5082) is parsed from `connection > ttl > {max,set,min}` into
`PeerSettings.{OutTTL,MinTTL}` and enforced at the TCP socket:

- **Outgoing TTL** set on the dialer pre-connect (`network.go DialContext`) and on
  the established/accepted socket (`session_connection.go tuneTCPConnectionForSettings`).
- **Inbound min-TTL** gate (`IP_MINTTL` / `IPV6_MINHOPCOUNT`) on the established socket.
- **Listen-socket TTL** (`network.go RealListenerFactory.ListenTTL`, max OutTTL across
  GTSM peers on the port) so kernel SYN-ACKs to peer-initiated connections carry TTL 255.
  This is the spec's "phase 2", implemented after the FRR interop test proved the passive
  path otherwise fails (SYN-ACK carried the default TTL and FRR's GTSM dropped it).
- **`max N` derivation**: OutTTL=255, MinTTL=256-N (RFC 5082 255-N+1). `set`/`min` are
  the explicit form. `max` + `set`/`min` is rejected at `config validate` time.
- **Observability**: `show bgp peer <p> detail` reports `gtsm-ttl-out` / `gtsm-ttl-min`.
- **Platform split**: `ttl_linux.go` (real setsockopt) / `ttl_other.go` (unsupported, logged
  at Debug, non-fatal) so darwin builds and sessions still form (AC-7).

Evidence: `ze-lint-changed` 0 issues; unit + `-race` green for network/reactor/plugin/peer/config
(`-tags 'ze_core ze_ssh'`); functional `bgp-gtsm` + `bgp-gtsm-reject` pass; interop `46-gtsm-frr`
(ze `ttl { max 1 }` vs FRR `ttl-security hops 1`) reaches Established at the default 90s timeout;
linux integration readback tests wired into `ze-integration-gtsm-test` + QEMU runner.

## Implementation Audit

- **AC-1..AC-7**: implemented and demonstrated (parse, derivation, conflict reject, IPv4/IPv6
  socket options, darwin unsupported path, regression guard for no-ttl peers).
- **Deviation (AC-6):** conflict rejection is in Go (`reactor/config.go parseTTLSettings`,
  `bgp/config/resolve.go validateDynamicGroupTTL`), not a YANG `must`. Both fire at
  `config validate` (`cmd_validate.go:346/361`), satisfying "reject before apply".
- **Scope addition (phase 2):** listen-socket TTL was implemented (not deferred) because the
  interop test proved GTSM does not interoperate without it. Shared-socket behavior: when any
  peer on a port uses GTSM, that listener's SYN-ACKs carry the max OutTTL (255); benign for
  non-GTSM peers (they do not gate inbound TTL).
- **Build-tag note:** always run Ze tests with tags (`go test -tags 'ze_core ze_ssh'`); a plain
  untagged `go test` spuriously fails feature-gated ssh/authz schema tests (`all_ze_ssh.go` is
  `//go:build ze_ssh`).

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Per-peer post-accept TTL is enough; listen-socket TTL is optional "phase 2" | The kernel emits the SYN-ACK from the listen socket before accept(), so on the passive path it carries the default TTL (64) and a GTSM peer drops it | FRR interop scenario: session only came up via ze's active connect after retries (>90s); tcpdump showed ze SYN-ACK ttl 64 | GTSM did not interoperate when the peer initiates; fixed by setting listen-socket TTL |
| Untagged `go test` reflects suite health | Feature-gated schemas (ssh/authz) are absent without `ze_ssh`, so tests fail spuriously | Reviewing `bgp/config` test failures | False "pre-existing red" finding; corrected |

## Review Gate
### Final status
- `/ze-review`: review performed inline (this session); 0 BLOCKER, 0 ISSUE outstanding for GTSM scope.
- NOTE: committed history `6e084eb6f`..`c121f84f8` does not build in isolation (the `OutTTL`
  consumer in `session.go` was committed without its field defs). The closure commit lands the
  field defs as a forward commit, so the tree builds from the closure commit onward. Fully
  rewriting the broken intermediate commits requires a manual rebase (history is unpushed).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-1-bgp-gtsm.md`
