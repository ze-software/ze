# Spec: tunnel-ttl-default

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

**Notes:** Promoted to ready per user instruction 2026-07-10 (followup-wave impact review session) authorizing conversion to ready.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/iface/yang/ze-iface-conf.yang` - tunnel `ttl` / `hoplimit` leaves
4. `internal/plugins/iface/netlink/tunnel_linux.go` - `link.Ttl` application
5. `internal/component/iface/config.go` - tunnel leaf parsing

## Task

Ze's IPv4-underlay tunnels (gre, gretap, ipip, sit) default their outer-header
`ttl` to `0`, meaning "inherit from the inner packet". For a locally-originated
packet whose inner TTL is small, or across a multi-hop underlay, an inherited TTL
can expire the encapsulated packet prematurely, so the tunnel appears to work for
directly connected endpoints but silently blackholes over multiple hops. The
IPv6-underlay counterpart (ip6gre) already defaults its `hoplimit` to `64`, so the
behaviour is inconsistent across the tunnel family.

Change the default outer TTL for the IPv4 tunnel kinds from `0` (inherit) to a
fixed, sane value (`64`), matching the existing IPv6 default. Operators who
deliberately want inherit can still set `ttl 0` explicitly.

**Scope (USER DECISION 2026-07-10): NETLINK-ONLY.** The ttl default 64 applies
to netlink-backed tunnels only. VPP-backed gre/gretap/ipip (these kinds are now
`ze:backend "netlink vpp"`, yang :644/:676/:779) are out of scope because the
VPP tunnel programming path carries no outer-TTL field: `createGRETunnel`
(`internal/plugins/iface/vpp/tunnel.go`) builds its request from
type/mode/src/dst only, and `createIPIPTunnel` (`tunnel.go`) from
src/dst/mode only -- neither has a TTL or hop-limit field.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - tunnel interface configuration.
  → Constraint: the TTL is applied on the netlink link via `link.Ttl`, keyed per tunnel kind.
- [ ] `ai/rules/config.md` - changing a YANG `default`.
  → Constraint: an explicit `ttl 0` must still be honoured as inherit; only the unset default changes.

**Key insights:**
- `ttl 0` = inherit is a legitimate mode; the fix changes only the *default* when the leaf is unset, not the meaning of `0`.
- The IPv6 tunnel already defaults hoplimit 64, so 64 is the consistent house default.
- This is a one-value change per tunnel kind plus a functional test proving multi-hop delivery.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/netlink/tunnel_linux.go` - `buildGretun`/`buildGretap` apply `link.Ttl = spec.TTL` only when `spec.TTLSet` (tunnel_linux.go, :174-175); the IPv6 path applies `link.Ttl = spec.HopLimit` (:146-147). With the default unset to 0, the kernel receives TTL 0 = inherit.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `ttl` defaults to `0` "inherit" for gre (:654-658), gretap (:685-689), ipip (:783-787), sit (:808-812); ip6gre `hoplimit` defaults to `64` (:726-729).
- [ ] `internal/component/iface/config.go` - tunnel leaves (`ttl`/`hoplimit`) parse into `spec.TTL`/`spec.TTLSet` (config.go, unchanged; `hoplimit` :619-625).

### Post-wave corrections (2026-07-10)

All refs re-verified against current code after the followup-spec wave:

- Line drift corrected in place above (old -> new): yang gre ttl :650-654 -> :654-658
  (default 0 at :656); gretap :680-684 -> :685-689; ipip :777-781 -> :783-787; sit
  :802-806 -> :808-812; ip6gre hoplimit :721-725 -> :726-729 (default 64 at :728).
  tunnel_linux.go buildGretun TTL :133-135 -> :140-141, hoplimit :139-141 -> :146-147,
  buildGretap :167-169 -> :174-175. config.go :600-606 unchanged.
- ipip/sit builders verified (settles R-2 for the audit): `buildIptun` applies
  `link.Ttl = spec.TTL` at tunnel_linux.go and `buildSittun` at :228-229,
  same field and gate as gre/gretap.
- Dual-backend conflict (wave impact): gre, gretap, and ipip are now
  `ze:backend "netlink vpp"` (yang :644, :676, :779). The VPP tunnel path
  (`internal/plugins/iface/vpp/tunnel.go` `createGRETunnel` :73,
  `createIPIPTunnel` :113) has no TTL/hop-limit field, so neither the new
  default nor an explicit ttl reaches a VPP-programmed tunnel. USER DECISION
  2026-07-10: this spec is netlink-only (see Task scope and Known Limitations).
- Edit targets sit inside the `ze-platform-vet` gate scope (Makefile:337-341
  vets `internal/component/iface/...` and `internal/plugins/iface/...` under
  GOOS=darwin and GOOS=freebsd); keep any Go edits building on both.
- Functional test location corrected everywhere in this spec: `test/ci/` does
  not exist. The test lives at `test/plugin/tunnel-ttl-default.ci` and needs
  `option=needs-linux` (it applies netlink tunnel config; see
  `ai/rules/platform-linux.md`).

**Behavior to preserve:**
- Explicit `ttl 0` still means inherit-from-inner.
- Any explicitly configured TTL value is applied unchanged.
- IPv6 tunnels (ip6gre) already default 64 and stay as-is.
- `tos` inherit behaviour is unchanged (out of scope).

**Behavior to change:**
- The unset default `ttl` for gre/gretap/ipip/sit becomes `64` instead of `0`.

## Data Flow (MANDATORY)

### Entry Point
- Config: a tunnel interface (gre/gretap/ipip/sit) with `ttl` left unset.

### Transformation Path
1. YANG default for `ttl` on the four IPv4 tunnel kinds changes from `0` to `64`.
2. Config parsing yields `spec.TTL = 64`, `spec.TTLSet = true` for an unset leaf (default applied), so the netlink builder sets `link.Ttl = 64`.
3. An explicit `ttl 0` still parses to `spec.TTL = 0` and is applied as inherit.
4. The tunnel is created with a fixed outer TTL, surviving a multi-hop underlay.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG default ↔ iface spec | default 64 flows into `spec.TTL` | [ ] |
| iface spec ↔ netlink | `link.Ttl = spec.TTL` on link creation | [ ] |
| netlink ↔ kernel | outer-header TTL set on the tunnel device | [ ] |

### Integration Points
- `internal/component/iface/yang/ze-iface-conf.yang` - change `default 0` → `default 64` for gre/gretap/ipip/sit `ttl`.
- `internal/plugins/iface/netlink/tunnel_linux.go` - no logic change; verify the default reaches `link.Ttl`.
- `internal/component/iface/config.go` - confirm default application yields `TTLSet=true`.

### Architectural Verification
- [ ] No bypassed layers (default flows through the normal YANG→spec→netlink path)
- [ ] No unintended coupling (per-kind leaf change; no shared-package edit)
- [ ] No duplicated functionality (reuse existing `spec.TTL` plumbing)
- [ ] Registration over hardcoding - this is a schema default change; no per-kind switch is added to a core/shared package.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A YANG `default 64` is applied so an unset leaf yields `TTLSet=true` | config.go parse path | default not applied → still 0 | unit test asserting `spec.TTL==64` when unset | unvalidated |
| A-2 | `ttl 0` remains a valid explicit inherit value after the default change | leaf type uint8, 0 in range | operators lose inherit mode | test explicit `ttl 0` still inherits | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing a default surprises a config that relied on inherit | existing tunnels change TTL after upgrade | documented behaviour change; explicit `ttl 0` restores inherit |
| R-2 | ipip/sit apply TTL via a different netlink field than gre | TTL not set on ipip/sit | audit ipip/sit builders; add per-kind test (2026-07-10: pre-verified, both use `link.Ttl`, tunnel_linux.go, :228-229) |
| R-3 | An EXPLICIT ttl value configured on a VPP-backed gre/gretap/ipip tunnel is silently unused today, and the new default makes the leaf look authoritative | operator sets ttl on a vpp-backed tunnel and the device shows no effect | to be settled at implement time -- two options left open (user decision pending): warn at config verify that ttl is ignored on the vpp backend, or reject ttl on vpp-backed tunnels outright |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| gre tunnel, `ttl` unset | → | `link.Ttl = 64` on the device | `test/plugin/tunnel-ttl-default.ci` |
| gre tunnel, explicit `ttl 0` | → | `link.Ttl = 0` (inherit) | `test/plugin/tunnel-ttl-default.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | gre with `ttl` unset | device outer TTL is 64 |
| AC-2 | gretap with `ttl` unset | device outer TTL is 64 |
| AC-3 | ipip with `ttl` unset | device outer TTL is 64 |
| AC-4 | sit with `ttl` unset | device outer TTL is 64 |
| AC-5 | gre with explicit `ttl 0` | device inherits inner TTL (0) |
| AC-6 | gre with explicit `ttl 200` | device outer TTL is 200 |
| AC-7 | ip6gre (unchanged) | hoplimit default stays 64 |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | creates a GRE tunnel without setting TTL and it works over a multi-hop underlay | YANG default 64 → spec → `link.Ttl` | `test/plugin/tunnel-ttl-default.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTunnelTTLDefault64` | `internal/component/iface/config_test.go` | unset `ttl` yields `spec.TTL==64, TTLSet==true` for gre/gretap/ipip/sit | |
| `TestTunnelTTLExplicitZeroInherits` | `internal/component/iface/config_test.go` | explicit `ttl 0` yields `spec.TTL==0` | |
| `TestBuildGretunTTLApplied` | `internal/plugins/iface/netlink/tunnel_linux_test.go` | `link.Ttl` reflects `spec.TTL` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ttl | 0..255 | 255 | - | 256 (uint8 caps) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tunnel-ttl-default` | `test/plugin/tunnel-ttl-default.ci` | unset TTL → 64 on device; explicit 0 → inherit | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - kernel tunnel default; validated by functional test | - | - | outer TTL is a kernel device attribute | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `default 0` → `default 64` for gre/gretap/ipip/sit `ttl`
- `internal/plugins/iface/netlink/tunnel_linux.go` - verify default reaches `link.Ttl` (audit ipip/sit builders)
- `internal/component/iface/config.go` - confirm default application (`TTLSet=true`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (default change) | [ ] yes | `ze-iface-conf.yang` tunnel `ttl` leaves |
| Functional test | [ ] yes | `test/plugin/tunnel-ttl-default.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` (behaviour change note) |
| 2 | Config syntax changed? | [ ] yes | `docs/features/interfaces.md`, `docs/guide/configuration.md` |

## Files to Create
- `test/plugin/tunnel-ttl-default.ci` - functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - failing `test/plugin/tunnel-ttl-default.ci` asserting a device TTL of 64 for an unset gre tunnel.
2. **Phase: Default change** - set `default 64` on the four IPv4 tunnel `ttl` leaves.
   - Tests: `TestTunnelTTLDefault64`, `TestTunnelTTLExplicitZeroInherits`
3. **Phase: Netlink audit** - confirm ipip/sit apply the TTL like gre/gretap.
   - Tests: `TestBuildGretunTTLApplied`
4. **Functional test (device TTL check)**
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | unset → 64; explicit 0 → inherit; ip6gre unchanged |
| Behaviour-change hygiene | documented; explicit inherit still available |
| Registration over hardcoding | schema default change only; no core/shared edit |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| default | `go test ./internal/component/iface -run TTL` |
| netlink | `go test ./internal/plugins/iface/netlink -run TTL` |
| functional | `test/plugin/tunnel-ttl-default.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No downgrade | change never lowers an explicitly configured TTL |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Netlink-only scope: the ttl default 64 applies to netlink-backed tunnels only (USER DECISION 2026-07-10) | Extending the VPP tunnel path so the default (and explicit values) reach VPP-programmed devices | The VPP tunnel programming path carries no outer-TTL field (`internal/plugins/iface/vpp/tunnel.go` `createGRETunnel` :73 sends type/mode/src/dst; `createIPIPTunnel` :113 sends src/dst/mode); plumbing TTL through the binapi is separate work, out of this spec |
| Explicit-ttl-on-VPP semantics left open | (a) warn at verify that ttl is ignored on the vpp backend; (b) reject ttl on vpp-backed tunnels | Deliberately NOT decided here; settle at implement time (see R-3). Both options recorded so the implementer presents the choice rather than silently picking |

## Known Limitations
- VPP-backed gre/gretap/ipip tunnels (`ze:backend "netlink vpp"`, yang :644/:676/:779) are out of scope: the VPP binapi calls carry no TTL/hop-limit field, so neither the new default 64 nor an explicitly configured ttl reaches a VPP-programmed tunnel. The default change is netlink-only by user decision (2026-07-10).
- The semantics of an EXPLICIT ttl value on a VPP-backed tunnel (silently unused today) are unresolved: warn vs reject is an open implement-time decision (R-3).

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (ttl 0..255)
- [ ] Functional tests for end-to-end behavior
