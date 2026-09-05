# Spec: tunnel-ttl-default

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-09-05 |

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
- Edit targets sit inside the `ze-platform-vet` gate scope (the native action tables under `internal/le/`:337-341
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
| A-1 | A YANG `default 64` is applied so an unset leaf yields `TTLSet=true` | config.go parse path | default not applied → still 0 | unit test asserting `spec.TTL==64` when unset | **broken**, then made true. The iface config path never materialized a schema default: `ApplyDefaults` (`internal/component/config/schema_defaults.go`) had three callers and none was iface, so the YANG default reached nothing. `parseTunnelEntry` now applies the matched case's defaults. See the Mistake Log |
| A-2 | `ttl 0` remains a valid explicit inherit value after the default change | leaf type uint8, 0 in range | operators lose inherit mode | test explicit `ttl 0` still inherits | confirmed: `TestTunnelTTLExplicitZeroInherits` and the `ttdinherit` row of `test/plugin/tunnel-ttl-default.ci` |

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
| `TestTunnelTTLDefault64` | `internal/component/iface/config_test.go` | unset `ttl` yields `spec.TTL==64, TTLSet==true` for gre/gretap/ipip/sit | PASS |
| `TestTunnelTTLExplicitZeroInherits` | `internal/component/iface/config_test.go` | explicit `ttl 0` yields `spec.TTL==0` | PASS |
| `TestTunnelTTLExplicitValueSurvivesDefault` | `internal/component/iface/config_test.go` | explicit 1, 200 and 255 are applied unchanged (the numeric boundary row) | PASS |
| `TestTunnelHopLimitDefault64` | `internal/component/iface/config_test.go` | unset `hoplimit` yields 64 for ip6gre/ip6gretap/ip6tnl/ipip6 | PASS |
| `TestTunnelDefaultsReachAnEmptyCase` | `internal/component/iface/config_test.go` | a case written with no block still takes its defaults | PASS |
| `TestTunnelDefaultsRefuseAnUnresolvedSchema` | `internal/component/iface/config_test.go` | an unloaded `tunnelSchema` fails the parse rather than applying no default | PASS |
| `TestCreateTunnelTTLReachesTheDevice` | `internal/plugins/iface/netlink/tunnel_linux_test.go` | `link.Ttl` reflects `spec.TTL` for gre/gretap/ipip/sit (this is the spec's `TestBuildGretunTTLApplied`, widened to all four kinds) | NOT RUN: `//go:build integration && linux` plus CAP_NET_ADMIN, which this host does not hold |
| `TestCreateTunnelTTLZeroInherits` | `internal/plugins/iface/netlink/tunnel_linux_test.go` | an explicit 0 is applied as 0 at the kernel boundary | NOT RUN: same gate |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ttl | 0..255 | 255 | - | 256 (uint8 caps) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tunnel-ttl-default` | `test/plugin/tunnel-ttl-default.ci` | unset TTL → 64 on device; explicit 0 → inherit; explicit 200 → 200; ip6gre hoplimit → 64 | PASS in the QEMU guest, 2026-09-05, `1/1 PASS 728 tunnel-ttl-default` |

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
5. **Full verification** → `./le verify current mode full`
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
| Changing the YANG `default` is the whole fix, because the parse applies schema defaults (A-1) | Nothing applied schema defaults on the iface path. `config.ApplyDefaults` had three callers (`bgp/config/peers.go`, `bgp/plugins/filter_modify/config.go`, `sysrib/sysrib.go`) and the interface section was delivered to the plugin straight from `ExtractConfigSubtree` (`internal/component/plugin/server/reload.go`). A `default 64` alone would have changed nothing on the device | `TestBackendGateIPv6AcceptRADefaultIsNotMaterialized` (`internal/component/config/backend_gate_test.go`) states it in its own comment: "the iface commit path walks the tree the plugin delivers rather than one with schema defaults materialized into it" | The spec gained a second edit: `parseTunnelEntry` resolves the encapsulation container from the schema and applies the matched case's defaults. It also revealed that the ip6gre/ip6gretap/ip6tnl/ipip6 `hoplimit` default of 64, which this spec's Task cites as the consistent house default, had never reached a device either. Both are fixed by the one call |
| A `default 0` reverted on the sit leaf would turn the sit row of the functional test red | It does not. `addSittunAttrs` (`vendor/github.com/vishvananda/netlink/link_linux.go`) sends `IFLA_IPTUN_TTL` only above 0, so a spec asking for 0 sends no attribute and the sit driver's device default of 64 stands | The recorded discrimination run named `ttdgre`, `ttdgretap` and `ttdipip` and not `ttdsit` | The `.ci` comment was corrected to say which rows move under which break, and the netlink read-back test uses 33 rather than 64 so a `buildSittun` that dropped the field cannot pass |

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

| AC | Producing code | Proof |
|----|----------------|-------|
| AC-1..AC-4 (gre, gretap, ipip, sit with `ttl` unset carry 64) | `internal/component/iface/yang/ze-iface-conf.yang`, the four `leaf ttl` declarations, now `default 64`; carried to the spec by `tunnelSchema.applyDefaults` (`internal/component/iface/tunnel.go`) called from `parseTunnelEntry` (`internal/component/iface/config.go`) | `TestTunnelTTLDefault64`; `ttdgre`, `ttdgretap`, `ttdipip`, `ttdsit` in `test/plugin/tunnel-ttl-default.ci` |
| AC-5 (explicit `ttl 0` inherits) | unchanged: `parseTunnelLeaves` reads the explicit value, and `ApplyDefaults` writes only an absent key | `TestTunnelTTLExplicitZeroInherits`; `ttdinherit` |
| AC-6 (explicit `ttl 200` applied unchanged) | same path | `TestTunnelTTLExplicitValueSurvivesDefault`; `ttdexplicit` |
| AC-7 (ip6gre `hoplimit` default stays 64) | the YANG leaf is unchanged; the same `applyDefaults` call now carries it to the device for the first time | `TestTunnelHopLimitDefault64`; `ttdip6gre` |

### Where the default is stated, and why once
`ze-iface-conf.yang` declares it, and nothing else does. `parseTunnelEntry`
resolves `interface/tunnel/encapsulation` from the schema once for the whole
interface section and applies the matched case container's defaults into the
config map before `parseTunnelLeaves` reads it. No Go constant repeats 64, so
the schema, the CLI completion, the config diff and the device cannot disagree.
This is the shape `internal/component/sysrib/distance_bootstrap_test.go` exists
to prevent, and the same `config.ApplyDefaults` helper `sysrib.go` and
`filter_modify/config.go` already use.

### Functional evidence
Run in a QEMU guest on 2026-09-05, before the owner deferred heavy testing:

- GREEN: `./le job run label tunttl-qemu quiet command ./le qemu run packages "iproute2" command "sh /workspace/<scratch>/guest-one.sh"` -> `1/1 PASS 728 tunnel-ttl-default`.
- RED, with `default 64` reverted to `default 0` on the four leaves and `ze` rebuilt: `ZE-OBSERVER-FAIL: ttdgre outer TTL is 0, want 64; ttdgretap outer TTL is 0, want 64; ttdipip outer TTL is 0, want 64`, `QEMU VM: FAIL (exit code 1)`.
- The YANG was restored and `bin/ze` rebuilt from the restored tree.

`ttdsit` does not move under that break, for the reason the `.ci` header now
records: the vendored netlink library omits `IFLA_IPTUN_TTL` at 0 and the sit
driver's device default is 64.

### Scope not taken
VPP-backed gre/gretap/ipip are untouched, per the user decision of 2026-07-10.
R-3 (warn or reject an explicit `ttl` on a VPP-backed tunnel) is a user
decision this session could not put to the owner, and neither option was
implemented. It is unchanged by this work: an explicit `ttl` on a VPP-backed
tunnel was silently unused before and still is.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/tunnel-ttl-default-zeclose-tunttl2.md` |
| `./le spec session review check` | `review_gate: OK (7 code files, clean, hashes match tmp/review/tunnel-ttl-default-zeclose-tunttl2.md)`. It also NOTEs that the running model could not be determined, so the tool leaves the review-model boundary UNCHECKED; the phase boundary is what carries independence here |
| Rounds | 1 |
| Reviewer lenses used | wiring + logic, removed-behavior + guard audit, style + simplicity + performance |

The reviewer is not the author: `/ze-implement` produced the diff at `90412c682`
and ended, and this gate read that diff from source.

### Run 1
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `loadTunnelSchema` calls `config.YANGSchema`, which builds a fresh loader and parses every embedded and registered module on each call. `parseIfaceConfig` runs it once per section with a non-empty `tunnel` map, and there are four such call sites per commit: verify (`register.go`), configure (`register.go`), and the two sides of the decompose (`operation.go`). MEASURED: the four sub-tests of `TestTunnelTTLDefault64` take 0.11s, so about 27ms per parse and about 110ms added to a tunnel-bearing commit | `internal/component/iface/tunnel.go` -- `loadTunnelSchema`; `internal/component/config/yang_schema.go` -- `YANGSchemaWithPlugins` | acknowledged, not changed. Config load is the cold path `/ze-review` step 15 exempts, and step 17 refuses a new cache without a measurement showing the problem. The measurement is here and it does not show one |
| 2 | NOTE | The `docs/features.md` behaviour-change sentence (checklist row 1) is in the tree and correct, but it landed in `f196afd5e`, an unrelated OSPF commit, rather than in `90412c682`. Several sessions share this checkout, and the doc edit was swept into whichever commit named the file first | `docs/features.md` line 15, the Interfaces row | acknowledged. The text and its anchors are right, and history cannot be unmixed. Recorded so the audit trail is not read as a missing doc edit |
| 3 | NOTE | `TestCreateTunnelTTLReachesTheDevice` and `TestCreateTunnelTTLZeroInherits` have still never executed. Run at closure under `unshare -Urmn`: they compile and SKIP, because `withTunnelNetNS` calls `netns.NewNamed`, which needs to write `/run/netns`, and a user namespace does not map the owner of that directory. Result: `requires CAP_NET_ADMIN: open /run/netns/TestCreateTunne: permission denied`, five skips, package `ok` | `internal/plugins/iface/netlink/tunnel_linux_test.go` -- `withTunnelNetNS` | recorded as verification debt. The same acceptance criteria ARE proven in the QEMU guest by `test/plugin/tunnel-ttl-default.ci`, which ran green with a recorded red |
| 4 | NOTE | R-3 is unresolved and the default makes it sharper: an explicit `ttl` on a VPP-backed gre, gretap or ipip is accepted by the schema and reaches no device, and `default 64` now makes the leaf look authoritative on a backend that cannot honor it. `createGRETunnel` sends type, mode, src and dst; `createIPIPTunnel` sends src, dst and mode. Neither carries a TTL field | `internal/plugins/iface/vpp/tunnel.go` -- `createGRETunnel`, `createIPIPTunnel` | not fixed: warn-or-reject is the owner's decision, which this session cannot put to him. One row in `plan/journal/unwired-feature.md`, one row in Work Not Done |
| 5 | NOTE | The same four-arm type switch over `*netlink.Gretun`, `*netlink.Gretap`, `*netlink.Iptun` and `*netlink.Sittun` is written twice, once in the fixture and once in the netlink test | `internal/test/fixture/plugin_fixture_08_tunnel_ttl_linux.go` -- `tunnelOuterTTL08`; `internal/plugins/iface/netlink/tunnel_linux_test.go` -- `tunnelDeviceTTL` | acknowledged. Test-only, in two packages, and sharing it would mean exporting a reader from a product package for a test's benefit |
| 6 | NOTE | The docs say a tunnel that names no `ttl` carries 64. True of every tunnel Ze creates, and not of a netdev that outlives the process: the apply skips a tunnel whose spec is unchanged, and on EEXIST for a device of the same kind it deliberately keeps the existing netdev rather than rebuilding a link that carries traffic. So a device made before this change keeps its TTL until it is deleted and recreated | `internal/component/iface/config_apply.go` -- the tunnel phase of `applyTunnels`, "Spec unchanged: keep the existing netdev" and the EEXIST branch | acknowledged. Pre-existing apply policy, general to every tunnel leaf rather than to `ttl`, and the goal does not depend on it. Qualifying only the TTL section would state a general property in a specific place |

### Fixes applied
- None. Every finding of run 1 is a NOTE, and NOTEs do not block
  (`/ze-review` step 22). No product line was changed by this closure.

### Run 2
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | none. Run 1 changed no code, so there is nothing new to review | - | - |

### Final status
- [ ] `/ze-review` run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (six)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The unset `ttl` default for gre, gretap, ipip and sit becomes 64 | Done | `internal/component/iface/yang/ze-iface-conf.yang`, four `leaf ttl` declarations | the value is declared in the schema and in no Go constant |
| The default reaches the device | Done | `internal/component/iface/tunnel.go` -- `tunnelSchema.applyDefaults`, called by `parseTunnelEntry` (`internal/component/iface/config.go`) | this is the edit A-1 did not predict: nothing on the iface path materialized a schema default before |
| An explicit `ttl 0` still means inherit | Done | `internal/component/config/schema_defaults.go` -- `applyChildDefault` writes only an absent key | so a present `0` is never overwritten |
| Netlink-only scope | Done | no VPP file is touched | the VPP consequence is finding 4 above |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-4 | Done | `TestTunnelTTLDefault64` (four kinds); rows `ttdgre`, `ttdgretap`, `ttdipip`, `ttdsit` of `test/plugin/tunnel-ttl-default.ci` read back through `tunnelOuterTTL08` | `ttdsit` holds by declaration rather than by discrimination, and the `.ci` header says so |
| AC-5 | Done | `TestTunnelTTLExplicitZeroInherits`; row `ttdinherit` | |
| AC-6 | Done | `TestTunnelTTLExplicitValueSurvivesDefault` at 1, 200 and 255; row `ttdexplicit` | covers the numeric boundary table |
| AC-7 | Done | `TestTunnelHopLimitDefault64` (ip6gre, ip6gretap, ip6tnl, ipip6); row `ttdip6gre` | the leaf was unchanged; the `applyDefaults` call is what first carried it to a device |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The six `internal/component/iface` unit tests | Done | `internal/component/iface/config_test.go` | green at closure: `ok github.com/ze-software/ze/internal/component/iface 1.024s` |
| `tunnel-ttl-default` | Done | `test/plugin/tunnel-ttl-default.ci` | `1/1 PASS 728` in the QEMU guest, 2026-09-05, with a recorded red |
| `TestCreateTunnelTTLReachesTheDevice`, `TestCreateTunnelTTLZeroInherits` | NOT RUN | `internal/plugins/iface/netlink/tunnel_linux_test.go` | they compile and SKIP on this host. See Review Gate finding 3 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/iface/yang/ze-iface-conf.yang` | Done | four `default 0` -> `default 64`, plus the `description` and `ze:help` that state why |
| `internal/plugins/iface/netlink/tunnel_linux.go` | Changed | audited, not edited. `buildGretun`, `buildGretap`, `buildIptun` and `buildSittun` all set `link.Ttl` from `spec.TTL`, which settles R-2 |
| `internal/component/iface/config.go` | Changed | it did NOT confirm the default; it had to CARRY it. `parseTunnelEntry` gained the `applyDefaults` call and `parseIfaceConfig` the schema load |
| `internal/component/iface/tunnel.go` | Added, not in the plan | `tunnelSchema`, `loadTunnelSchema`, `applyDefaults` |
| `test/plugin/tunnel-ttl-default.ci` | Done | with `internal/test/fixture/plugin_fixture_08_tunnel_ttl_linux.go` |

### Audit Summary
- **Total items:** 16
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (`config.go` and `tunnel_linux.go` differ from what the plan predicted, and `tunnel.go` is a file the plan did not name; all three follow from the broken A-1 and are in the Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An IPv4-underlay tunnel with no `ttl` carries a fixed outer TTL instead of inherit, so it survives a multi-hop underlay | functional, with a recorded red | `test/plugin/tunnel-ttl-default.ci`: `1/1 PASS 728 tunnel-ttl-default` in the QEMU guest. With `default 64` reverted to `default 0` and `ze` rebuilt: `ttdgre outer TTL is 0, want 64; ttdgretap outer TTL is 0, want 64; ttdipip outer TTL is 0, want 64`. The assertion reads the kernel device through `netlink.LinkByName`, not `show interface`, because the outer TTL is a device attribute no command reports |
| The behaviour matches the IPv6-underlay kinds, which already declared 64 | functional | row `ttdip6gre` of the same `.ci`, plus `TestTunnelHopLimitDefault64`. The schema had declared 64 for four v6 kinds since the tunnel work landed, and nothing carried it to a device until `applyDefaults` |
| Inherit stays reachable | functional + unit, negative | row `ttdinherit` asserts the device reads 0 for a config that writes `ttl 0`, and `TestTunnelTTLExplicitZeroInherits` asserts the spec does. `applyChildDefault` writes only an absent key, so a present 0 cannot be overwritten |
| No configured value is lowered (the spec's security row) | unit, at the boundary | `TestTunnelTTLExplicitValueSurvivesDefault` at 1, 200 and 255 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| R-3: warn or reject an explicit `ttl` on a VPP-backed gre, gretap or ipip tunnel | It is an owner decision the spec deliberately left open, and this session cannot put a question to him. Neither option was implemented, so the behaviour is unchanged: the leaf was silently unused on that backend before and still is | No spec, and it needs the owner's answer before one can be written. It is recorded as a row in `plan/journal/unwired-feature.md`, per the 2026-08-10 directive that a defect met while working gets one journal row |
| Carrying the outer TTL through the VPP binapi so the default reaches a VPP-programmed device | Out of scope by the user decision of 2026-07-10: `createGRETunnel` and `createIPIPTunnel` (`internal/plugins/iface/vpp/tunnel.go`) carry no TTL field, and adding one is separate work | Not started. It is the larger half of the same owner question as R-3 |
| The two netlink read-back tests | They need CAP_NET_ADMIN over `/run/netns`, which this host does not grant even under `unshare -Urmn` | `plan/verification-debt/eb87be3f.md` carries this commit's debt rows. The `.ci` proves the same criteria in the guest |
| The whole-tree gate over this diff | Heavy testing is deferred by owner instruction, 2026-09-05 | `plan/verification-debt/eb87be3f.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/tunnel-ttl-default.ci` | yes | in `90412c682`, 157 lines; read at closure, it declares `option=needs-linux:caps=net-admin` and seven tunnel stanzas |
| `internal/test/fixture/plugin_fixture_08_tunnel_ttl_linux.go` | yes | in `90412c682`, 121 lines, `//go:build linux`, registered by `registerPlugin08("plugin/tunnel-ttl-default", ...)` |
| `internal/component/iface/tunnel.go` | yes | carries `tunnelSchema`, `loadTunnelSchema` and `applyDefaults` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | four kinds take 64 | `go test ./internal/component/iface/ -run TestTunnelTTLDefault64 -v`: four sub-tests, all PASS, 0.11s |
| AC-5, AC-6 | explicit values survive | the same run covers `TestTunnelTTLExplicitZeroInherits` and `TestTunnelTTLExplicitValueSurvivesDefault`; `applyChildDefault` (`internal/component/config/schema_defaults.go`) writes only when `_, exists := m[name]` is false |
| AC-7 | ip6gre stays 64 | `TestTunnelHopLimitDefault64`, four sub-tests, PASS |
| the guard is reachable and closed | a schema nobody loaded fails the parse | `TestTunnelDefaultsRefuseAnUnresolvedSchema` asserts the error names `tunnelEncapSchemaPath` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| gre tunnel with `ttl` unset | `test/plugin/tunnel-ttl-default.ci` | yes: the `.ci` writes `tunnel ttdgre { encapsulation { gre { local ...; remote ...; } } }` with no `ttl`, and `tunnelTTLExpectations08` requires 64 back from `netlink.LinkByName("ttdgre")` |
| gre tunnel with explicit `ttl 0` | same | yes: the `ttdinherit` stanza writes `ttl 0`, and the expectation is 0 |
| the new code has a caller | n/a | `loadTunnelSchema` is called by `parseIfaceConfig`, `applyDefaults` by `parseTunnelEntry`, and `parseIfaceConfig` is the ONLY producer of a `TunnelSpec` in the tree (`grep -rn "TunnelSpec{" internal/` finds no other constructor), so no tunnel can reach a backend without passing through it |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, then made true | nothing on the iface path materialized a schema default. `config.ApplyDefaults` had three callers and none was iface. Mistake Log row, and the fix is `tunnelSchema.applyDefaults` |
| A-2 | confirmed | `TestTunnelTTLExplicitZeroInherits` and the `ttdinherit` row; the producer is `applyChildDefault`, which writes only an absent key |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 1, `docs/features.md` behaviour-change note | line 15, the Interfaces row, now reads "each carrying an outer-header TTL of 64 when the config names none", with anchors to `ze-iface-conf.yang` and `internal/component/iface/tunnel.go -- tunnelSchema, applyDefaults` | yes, in the tree. It landed in `f196afd5e` rather than in this spec's commit; see Review Gate finding 2 |
| Row 2, `docs/features/interfaces.md` | the new "Outer-header TTL" section, with four `<!-- source: -->` anchors including the VPP one that names why the default stops at netlink | yes, in `90412c682` |
| Row 2, `docs/guide/configuration.md` | no update owed. Its "Interface Configuration" section holds Interface Types, the `os-name` selector, MAC binding and offload, and declares no tunnel container and no tunnel leaf. The tunnel config surface is documented in `docs/features/interfaces.md` | verified by reading the section headings under `## Interface Configuration` |
| `./le doc check verify` | red at 3484 issues and not one of them names `docs/features/interfaces.md`, `internal/component/iface/` or `ze-iface-conf.yang`. The iface hits in the log are gh-pages command-catalog rows for `show capture interface` | recorded, not repaired: the findings belong to other sessions and to the sibling published checkouts |
| `./le docvalid help-shape` | red with 3 summary findings, in `ze-bgp-conf` and `ze-vpp-conf`. The four rewritten `ttl` descriptions and `ze:help` texts pass | recorded |
| `./le repository check` | 24 issues, none in `internal/component/iface`, `internal/plugins/iface` or `internal/test/fixture`. No stale source anchor on `docs/features/interfaces.md` | recorded |

## Core Insight

A YANG `default` is a declaration, not a mechanism. This spec was written as a
one-value change and it was not one: `config.ApplyDefaults` had three callers,
each applying defaults for its own container, and the interface path was not
among them. So `default 64` on four leaves would have changed nothing on any
device, and the tests that would have caught it did not exist because nobody
doubted that a schema default is applied.

The consequence reached further than this spec's own subject. The four
IPv6-underlay kinds had declared `hoplimit 64` since the tunnel work landed, and
that value had never reached a device either. One call fixed both.

The class is `plan/journal/declared-default-applied-by-each-consumer.md`, whose
2026-09-04 row predicted this exact session: "A fourth consumer that forgets gets
0 for every unwritten leaf, with no error and no log line". This is the fourth
consumer. It did not forget, and it still wrote its own lookup, which is the
argument for the repair that row already names.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
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
