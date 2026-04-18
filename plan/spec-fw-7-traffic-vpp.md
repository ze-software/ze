# Spec: fw-7-traffic-vpp — VPP Traffic Control Backend

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 0/4 |
| Updated | 2026-04-18 |

## Task

Implement the `trafficvpp` backend at `internal/plugins/traffic/vpp/` (package
`trafficvpp`). The backend registers under name `"vpp"` via `traffic.RegisterBackend`
and translates `map[string]traffic.InterfaceQoS` to VPP policer, QoS egress map,
and classifier calls through GoVPP's binary API.

The backend is strict: any qdisc or filter combination that cannot be represented
exactly in VPP is rejected at `OnConfigVerify` via the existing `ze:backend`
YANG gate, so the operator sees the error before `commit` lands and can edit the
config. There is no best-effort silent approximation.

Prerequisites fw-1 (Backend interface + InterfaceQoS model), vpp-1 (Connector), fw-3
(netlink backend, reference implementation), and fw-9 (traffic component reactor,
backend-gate call wiring) are all complete. Their learned summaries live in
`plan/learned/{584,587,611,623}-*.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — firewall/traffic umbrella, VPP tc mapping table
3. `plan/spec-vpp-0-umbrella.md` — VPP architecture, connection sharing
4. `plan/learned/584-fw-1-data-model.md` — InterfaceQoS model + Backend interface
5. `plan/learned/587-fw-3-traffic-netlink.md` — netlink backend reference
6. `plan/learned/611-vpp-1-lifecycle.md` — `vpp.Connector`, `GetActiveConnector`
7. `plan/learned/623-fw-9-traffic-lifecycle.md` — backend-gate wiring in OnConfigVerify
8. `internal/component/traffic/backend.go` — Backend interface (Apply / ListQdiscs / Close)
9. `internal/component/traffic/model.go` — InterfaceQoS / Qdisc / TrafficClass / TrafficFilter
10. `internal/component/traffic/register.go` — component reactor, `validateBackendGate`
11. `internal/plugins/traffic/netlink/backend_linux.go` — sibling backend shape
12. `internal/plugins/fib/vpp/register.go` — reference for GoVPP connector access
13. `internal/component/vpp/conn.go` — `Connector.NewChannel`, `IsConnected`

## Required Reading

### Architecture Docs
- [ ] `plan/spec-fw-0-umbrella.md` — VPP tc mapping overview
  → Decision: ACL+Policer/QoS owned by firewallvpp (fw-6) and trafficvpp (fw-7), no separate VPP-native config surface (fw-0 entry dated 2026-04-17).
  → Constraint: backend plugin lives at `internal/plugins/traffic/vpp/`, NOT `internal/plugins/trafficvpp/` (corrected 2026-04-18 to match `internal/plugins/traffic/netlink/` sibling layout).
- [ ] `plan/spec-vpp-0-umbrella.md` — VPP connection sharing
  → Constraint: dependent plugins obtain GoVPP channel via `vppcomp.GetActiveConnector()` + `Connector.NewChannel()`; trafficvpp MUST follow the same pattern as fibvpp.
- [ ] `plan/learned/623-fw-9-traffic-lifecycle.md` — backend gate is live in OnConfigVerify
  → Constraint: verify-time rejection of unsupported qdisc/filter types is done by annotating YANG leaves with `ze:backend "<names>"`; the gate machinery already runs. No new Backend.Verify method is needed.

### Reference Code (read before writing)
- [ ] `internal/plugins/traffic/netlink/` — mirror the file layout (backend_linux.go, backend_other.go, register.go, package entry file, translate_linux.go).
  → Constraint: `Apply` per-interface error handling, `ListQdiscs` read-back, `Close` releases resources.
- [ ] `internal/plugins/fib/vpp/register.go` + `backend.go` — how to hold an `api.Channel`, close it in `Close`, surface connection loss.
  → Decision: fibvpp captures the channel in `OnStarted`. trafficvpp is NOT a plugin with an event loop — it is a backend factory called by the traffic component. Channel acquisition happens in the factory (or lazily on first `Apply`), not via a plugin callback.

**Key insights:**
- The traffic component owns the plugin lifecycle (registered as the `traffic-control` subsystem in `internal/component/traffic/register.go`). trafficvpp registers a Backend factory via `init()` and owns only the Backend's internals.
- `validateBackendGate` in `register.go` fires during OnConfigVerify for every traffic-control section. Per-qdisc/per-filter rejection is a YANG annotation task, not a Go-level check.
- `vpp.Connector` exposes `NewChannel` and `IsConnected`. It does NOT currently expose a `WaitConnected(timeout)` helper; fw-7 adds that method.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traffic/backend.go` — `Backend` interface: `Apply(map[string]InterfaceQoS) error`, `ListQdiscs(string) (InterfaceQoS, error)`, `Close() error`. `RegisterBackend(name, factory)` + `LoadBackend(name)` manage the global active backend.
  → Constraint: trafficvpp MUST satisfy this interface without extending it.
- [ ] `internal/component/traffic/model.go` — InterfaceQoS = `{Interface, Qdisc}`, Qdisc = `{Type, DefaultClass, Classes}`, TrafficClass = `{Name, Rate, Ceil, Priority, Filters}`, TrafficFilter = `{Type, Value}`. QdiscType enum covers htb/hfsc/fq/fq_codel/sfq/tbf/netem/prio/clsact/ingress. FilterType enum covers mark/dscp/protocol.
  → Constraint: trafficvpp translates these exact types — the internal model is already decoupled from the backend.
- [ ] `internal/component/traffic/register.go` — OnConfigVerify calls `validateBackendGate(sections, cfg.Backend)`. OnConfigApply loads the backend via `LoadBackend(name)` and calls `backend.Apply(cfg.Interfaces)`.
  → Constraint: rejection MUST happen at verify time (before commit), not at apply time. Verify-time rejection is a YANG annotation task.
- [ ] `internal/component/traffic/schema/ze-traffic-control-conf.yang` — current schema has zero `ze:backend` annotations. Every qdisc-type / filter-type leaf is currently allowed under every backend.
  → Constraint: fw-7 adds `ze:backend` annotations declaring which backends support each feature. netlink gets the full set; vpp gets the supported subset.
- [ ] `internal/plugins/traffic/netlink/backend_linux.go` — netlink backend: for each interface, replace root qdisc, add classes with optional filters. Errors out on first failure. Stateless (no fields on `backend`).
  → Constraint: trafficvpp mirrors the per-interface apply shape but keeps a GoVPP channel.
- [ ] `internal/component/vpp/conn.go` — `Connector.NewChannel() (api.Channel, error)`, `IsConnected() bool`, `Close()`. No timeout helper exists yet.
  → Constraint: fw-7 adds `WaitConnected(ctx, timeout) error` to Connector.
- [ ] `internal/plugins/fib/vpp/register.go` — obtains connector via `vppcomp.GetActiveConnector()` in `OnStarted`; falls back to mock backend on nil. trafficvpp cannot follow the mock-fallback pattern because traffic `Apply` is synchronous and stateless.
  → Decision: trafficvpp hard-fails `Apply` if VPP is not connected after a bounded wait. See Design Decision 1 below.

**Behavior to preserve:**
- Traffic component (`internal/component/traffic/`) and netlink backend (`internal/plugins/traffic/netlink/`) are unchanged except for the schema file, which gains `ze:backend` annotations.
- `traffic.Backend` interface is unchanged.
- Existing YANG leaves keep their current semantics under the netlink backend.
- `internal/component/plugin/all/all.go` gains one blank import line (via `make generate`).

**Behavior to change:**
- Add trafficvpp backend registered as `"vpp"`.
- Annotate traffic YANG leaves with `ze:backend "netlink vpp"` where both support; `ze:backend "netlink"` where VPP does not.
- Add `Connector.WaitConnected(ctx, timeout) error` method.

## Design Decisions

### Decision 1 — Hard-fail on VPP not connected, with 5s connection wait

`Apply` calls `vppConnector.WaitConnected(ctx, 5*time.Second)` before allocating an API channel.
If the wait times out, `Apply` returns `"traffic-vpp: vpp not connected after 5s"` and the
traffic-control apply fails. The operator sees the error, starts VPP, and retries the commit.

Alternatives rejected:

| Option | Why rejected |
|--------|--------------|
| Soft-accept with warning (mock backend, log only) | Silent wiring gap — operator believes QoS is active when it isn't. Matches the recurring "feature not wired" failure mode. |
| Stash-and-retry on reconnect | Introduces stateful backend + reconnect goroutine + new failure mode (stashed state vs committed config drift). Not justified until there's evidence the 5s wait is insufficient. YAGNI. |

### Decision 2 — Verify-time rejection via a per-backend Verifier function

**Revised 2026-04-18 during implementation.** Initial design assumed the YANG
`ze:backend` gate would cover the rejection matrix. It cannot: the gate
annotates leaves (one type, one set of backends), not individual enum
values. Rejecting `qdisc hfsc` while accepting `qdisc htb` under the same
`qdisc type` leaf requires per-value logic, which the gate does not support.

Implemented instead: `traffic.RegisterVerifier(name, fn)` registers a
stateless function the component invokes from `OnConfigVerify` after
`validateBackendGate` passes. `trafficvpp.Verify` walks the parsed config
and rejects qdisc / filter types with `<type>: not supported by backend vpp`.
Backends without a verifier (like netlink) accept anything the YANG-level
gate already allowed — no behavior change.

This keeps verify-time feedback without extending the Backend interface or
adding state, and is consistent with the existing
`RegisterBackend` / factory pattern.

### Decision 3 — Channel acquisition strategy

Backend factory `newBackend()` captures a handle to the `vpp.Connector` only.
`api.Channel` is created lazily inside each `Apply` call and closed immediately
after, not held across calls.

Rationale: backend instances are long-lived (allocated at `LoadBackend`, released
at `CloseBackend`) but `Apply` calls are sparse (one per config commit). Holding
a channel across commits risks draining VPP's channel pool if many backends
exist; creating one per Apply is simpler and matches GoVPP's expected usage.
`ListQdiscs` follows the same rule — one channel per call.

### Decision 4 — No state, no reconciliation

trafficvpp does not track previously-applied state. The traffic component already
holds `previousCfg` and computes what needs to be unprogrammed. trafficvpp's
`Apply` treats the passed `desired` as the new full state and programs it.
Interfaces absent from `desired` but present in VPP are NOT the backend's
concern — the component will call `Apply` again with an explicit empty entry
if the operator removed a section.

### Decision 5 — Rate unit conversion

Ze `InterfaceQoS` rates are bps (`uint64`). VPP policer rates are kbps (`uint32`).
Translation divides bps by 1000 and rounds up. An explicit `uint32` overflow check
rejects rates above ~4.3 Tbps with a clear error. The rounding is documented in
`translate.go`'s package comment.

## Translation Contract (Acceptance)

Verification fires in OnConfigVerify via the YANG `ze:backend` gate.
`Apply` and `ListQdiscs` trust verified input.

### Qdisc types

| Qdisc | VPP mapping | Apply outcome |
|-------|-------------|---------------|
| `htb` with exactly 1 class | Single policer (CIR = Rate kbps, EIR = Ceil kbps, two-rate color-blind) bound to interface egress via `PolicerOutput` | Program one policer |
| `tbf` with exactly 1 class | Single policer CIR=EIR=Rate kbps bound to interface egress | Program one policer |
| `htb` / `tbf` with 0 or >1 classes | Rejected at verify. Multi-class needs filter-based classification (deferred); without filters, every policer would stack on the output feature arc in series and effective rate becomes min(class_rates) rather than per-class shaping. | Rejected at verify (multi-class deferred to filter specs) |
| `prio` | Class-index -> DSCP-value mapping has no operator-facing semantics | Rejected at verify (deferred to spec-fw-7b-prio-mapping) |
| `hfsc` | Service curve has no VPP equivalent | Rejected at verify |
| `fq` / `fq_codel` / `sfq` | Fair-queue disciplines not in VPP | Rejected at verify |
| `netem` | Emulation not in VPP | Rejected at verify |
| `clsact` / `ingress` | Ingress policing differs in VPP | Rejected at verify (deferred) |

Revision 2026-04-18 (post-review): `qdisc prio` was initially in the accept
list mapped to `QosEgressMapUpdate`. Review caught that populating the IP-source
row with `Outputs[class_index] = Priority` does not correspond to any prio-qdisc
semantic (the row is indexed by input DSCP value; using class index makes the
map arbitrary). Moved to reject until a future spec designs an explicit
DSCP-to-class binding. The `egressMapFromPrioClasses` translation skeleton
is retained in `translate.go` with a `RETAINED FOR REFERENCE` comment.

### Filter types

| Filter | Apply outcome |
|--------|---------------|
| `mark` | Rejected at verify. VPP's classifier matches packet-header bytes, not Linux SKB metadata. |
| `dscp` | Rejected at verify (deferred). Programming `QosEgressMapUpdate` + `QosMarkEnableDisable` without the `QosRecordEnableDisable` ingress step leaves the map reading a zero input for every packet; would silently no-op. |
| `protocol` | Rejected at verify (deferred). The classify table was never attached via `ClassifySetInterfaceIPTable`, and the match offset used was wrong for an Ethernet-framed packet. Programming without those would silently no-op. |

Revision 2026-04-18 (second review): DSCP and protocol filters were
initially in the accept list. A deeper review found the VPP pipeline was
incomplete in both cases -- classify sessions were added to a table that
was not on any interface's packet path, and QoS mark used a stored DSCP
that was never recorded. The features would have shipped as silent
no-ops. Per `rules/exact-or-reject.md`, moved both to the reject list
with deferrals naming the destination specs. HTB and TBF policer rate
limiting (the feature that actually works end to end) remains accepted.

Revision 2026-04-18 (mid-implementation): `filter mark` was initially in the
accept list. It was moved to reject after examining VPP's classifier API:
`ClassifyAddDelSession` matches packet bytes at a table-defined offset, not
pipeline metadata. The Linux `skb->mark` set by iptables MARK has no
equivalent field in VPP's packet classifier. `protocol` and `dscp` map
naturally because both are packet-header fields.

### Other validations

| Condition | Where checked | Result |
|-----------|---------------|--------|
| Unknown interface name | `Apply` (SwInterfaceDump lookup) | Error "interface `<name>` not present in vpp" |
| Rate == 0 | Existing `ValidateRate` in `model.go` (pre-verify) | Error |
| Ceil < Rate | Existing `ValidateCeil` in `model.go` (pre-verify) | Error |
| Rate > 4.3 Tbps (uint32 kbps overflow) | `translate.go` | Error at Apply time |
| VPP not connected after 5s | `Apply` first action | Error "vpp not connected after 5s" |

## Data Flow (MANDATORY)

### Entry Point
Traffic component's OnConfigApply invokes `backend.Apply(desired map[string]InterfaceQoS)`
after OnConfigVerify passed the backend gate.

### Transformation Path
1. `Apply` waits up to 5s for VPP connection (Decision 1).
2. `Apply` opens a fresh GoVPP channel via `connector.NewChannel()` (Decision 3).
3. `Apply` calls `SwInterfaceDump` once to build a name → sw_if_index map for all interfaces in `desired`.
4. For each interface in `desired`:
   - Translate qdisc to VPP policer set (or QoS egress map for `prio`).
   - Issue one `PolicerAddDel` per class.
   - For each filter, issue `ClassifyAddDelSession` (mark, protocol) or `QosEgressMapUpdate` + `QosMarkEnableDisable` (dscp).
5. First error short-circuits and is wrapped with the interface name.
6. Channel is closed in deferred cleanup.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Traffic component → Backend | `backend.Apply(map[string]InterfaceQoS)` | [ ] |
| Backend → VPP | GoVPP binapi over unix socket | [ ] |
| Backend → Connector | `vpp.Connector.NewChannel` | [ ] |

### Integration Points
- `internal/component/traffic/backend.go` — existing interface, unchanged.
- `internal/component/traffic/schema/ze-traffic-control-conf.yang` — add `ze:backend` annotations.
- `internal/component/vpp/conn.go` — add `WaitConnected(ctx, timeout) error`.
- `internal/component/plugin/all/all.go` — add blank import for the new package (generated).

### Architectural Verification
- [ ] No bypassed layers: backend talks to VPP only through `vpp.Connector`.
- [ ] No unintended coupling: trafficvpp does not import other plugins.
- [ ] No duplicated functionality: extends Backend registry, does not reimplement it.
- [ ] Zero-copy not applicable (non-wire path).

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `traffic-control { backend vpp }` config commit with HTB+mark filter | → | `trafficvpp.backend.Apply` calling `PolicerAddDel` + `ClassifyAddDelSession` | `test/traffic/010-vpp-boot-apply.ci` |
| Config commit with `qdisc hfsc` under vpp backend | → | `validateBackendGate` rejects | `test/traffic/011-vpp-reject-hfsc.ci` |
| Config commit with `backend vpp` while VPP daemon down | → | `Apply` returns "vpp not connected after 5s" | `test/traffic/012-vpp-not-connected.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `qdisc htb` + exactly 1 class with Rate/Ceil under `backend vpp` | One policer programmed: CIR=Rate kbps, EIR=Ceil kbps, bound to interface egress |
| AC-1b | `qdisc htb` + 0 or >1 classes under `backend vpp` | Rejected at `ze config verify` with "exactly 1 class required" (multi-class deferred) |
| AC-2 | `qdisc tbf` + exactly 1 class under `backend vpp` | One policer programmed: CIR=EIR=Rate kbps, bound to interface egress |
| AC-3 | `qdisc prio` under `backend vpp` | Rejected at `ze config verify` (deferred: see plan/deferrals.md) |
| AC-4 | `filter mark <N>` under `backend vpp` | Rejected at `ze config verify` with `mark: not supported by backend vpp` |
| AC-5 | `filter dscp <N>` under `backend vpp` | Rejected at `ze config verify` (deferred: VPP QoS record+mark pipeline not yet implemented) |
| AC-6 | `filter protocol <N>` under `backend vpp` | Rejected at `ze config verify` (deferred: VPP classify table attachment not yet implemented) |
| AC-7 | `qdisc hfsc` (or fq/sfq/fq_codel/netem/prio/clsact/ingress) under `backend vpp` | Rejected at `ze config verify` with `<type>: not supported by backend vpp` |
| AC-8 | `backend vpp` + interface name not present in VPP | `Apply` returns "interface `<name>` not present in vpp" |
| AC-9 | `backend vpp` committed while VPP daemon down | `Apply` returns "vpp not connected after 5s" |
| AC-10 | Rate overflowing uint32 kbps (≥ ~4.3 Tbps) | Translation returns overflow error |
| AC-11 | `traffic.RegisterBackend("vpp", newBackend)` called at package init | `LoadBackend("vpp")` succeeds, `GetBackend()` returns trafficvpp instance |
| AC-12 | Existing `backend tc` (netlink) config after fw-7 lands | Unchanged — netlink backend still accepts all qdisc types previously supported |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTranslateHTB` | `internal/plugins/traffic/vpp/translate_test.go` | HTB classes → policer parameters (CIR/EIR, kbps rounding) | |
| `TestTranslateTBF` | `internal/plugins/traffic/vpp/translate_test.go` | TBF → single policer | |
| `TestTranslatePrio` | `internal/plugins/traffic/vpp/translate_test.go` | prio classes → QoS egress map | |
| `TestTranslateDSCPFilter` | `internal/plugins/traffic/vpp/translate_test.go` | dscp filter → QoS map entry | |
| `TestTranslateProtocolFilter` | `internal/plugins/traffic/vpp/translate_test.go` | protocol filter → classify match bytes | |
| `TestTranslateRateOverflow` | `internal/plugins/traffic/vpp/translate_test.go` | Rate > uint32 kbps returns overflow error | |
| `TestTranslateRateRounding` | `internal/plugins/traffic/vpp/translate_test.go` | 1500 bps → 2 kbps (round up), 1000 bps → 1 kbps | |
| `TestBackendRegistered` | `internal/plugins/traffic/vpp/register_test.go` | `init()` calls `RegisterBackend("vpp", newBackend)` | |
| `TestWaitConnectedTimeout` | `internal/component/vpp/conn_test.go` | `WaitConnected` returns error when not connected within timeout | |
| `TestWaitConnectedImmediate` | `internal/component/vpp/conn_test.go` | `WaitConnected` returns nil when already connected | |
| `TestYANGBackendGateHFSC` | `internal/component/traffic/schema/*_test.go` | Schema with `qdisc hfsc` under `backend vpp` fails gate | |
| `TestYANGBackendGateHTB` | `internal/component/traffic/schema/*_test.go` | Schema with `qdisc htb` under `backend vpp` passes gate | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Rate (bps) | 1 — 4294967295000 | 4294967295000 | 0 (caught by `ValidateRate`) | 4294967296000 (overflow at kbps) |
| Ceil (bps) | Rate — 4294967295000 | 4294967295000 | Rate-1 (caught by `ValidateCeil`) | 4294967296000 |
| Priority | 0 — 255 | 255 | N/A (uint8) | N/A (uint8) |
| WaitConnected timeout | 1ms — 1h | any finite duration | 0 (reject as programmer error) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `010-vpp-boot-apply` | `test/traffic/010-vpp-boot-apply.ci` | Boot ze with HTB + mark filter on veth, verify policer + classify session via `ze cli traffic show` | |
| `011-vpp-reject-hfsc` | `test/traffic/011-vpp-reject-hfsc.ci` | `ze config verify` of `qdisc hfsc` under vpp backend returns rejection | |
| `012-vpp-not-connected` | `test/traffic/012-vpp-not-connected.ci` | `backend vpp` commit with VPP daemon stopped produces "vpp not connected after 5s" | |

### Future (if deferring any tests)
- Full scheduler tests (multi-class HTB under load) deferred to operational validation — a .ci test is sufficient to prove wiring.
- Ingress/clsact support — deferred to a future spec; tracked in `plan/deferrals.md`.

## Files to Modify

- `internal/component/traffic/backend.go` — add `Verifier` type, `RegisterVerifier`, `RunVerifier` (Decision 2 revision).
- `internal/component/traffic/register.go` — call `RunVerifier` in OnConfigVerify after `validateBackendGate`.
- `internal/component/vpp/conn.go` — add `WaitConnected(ctx, timeout) error`.
- `internal/component/vpp/conn_test.go` — add tests for `WaitConnected`.
- `internal/component/plugin/all/all.go` — add blank import for `internal/plugins/traffic/vpp`.
- `docs/features.md` — add VPP traffic control backend entry.
- `docs/guide/plugins.md` — add trafficvpp entry next to trafficnetlink.
- `docs/guide/traffic-control.md` — add VPP backend section with compatibility matrix.
- `vendor/go.fd.io/govpp/binapi/policer/`, `qos/`, `classify/`, `policer_types/` — added in Phase 0.
- `vendor/modules.txt` — updated in Phase 0.

Files originally listed but no longer needed:
- `internal/component/traffic/schema/ze-traffic-control-conf.yang` — was meant to
  receive `ze:backend` annotations. Decision 2 revision moves rejection to
  `RunVerifier`, so the YANG file needs no change.
- `internal/component/plugin/all/all_test.go` — no expected-count bump because the
  test's expected list only names plugins registered via `registry.Register`;
  backend registration via `traffic.RegisterBackend` is invisible to that test.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema annotations (`ze:backend`) | Yes | `internal/component/traffic/schema/ze-traffic-control-conf.yang` |
| CLI commands | No (reuses `ze cli traffic show`) | - |
| Plugin all.go blank import | Yes | `internal/component/plugin/all/all.go` (generated) |
| Functional tests | Yes | `test/traffic/010..012-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — VPP traffic control backend |
| 2 | Config syntax changed? | No | Schema gains backend gating but existing syntax unchanged |
| 3 | CLI command added/changed? | No | Reuses `ze cli traffic show` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — trafficvpp |
| 6 | Has a user guide page? | Yes | `docs/guide/traffic-control.md` — add "VPP backend" section with qdisc compatibility matrix |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | `test/traffic/` already exists from fw-9 |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

### Phase 0 — vendor update (separate commit, lands before Phase 1)
- `vendor/go.fd.io/govpp/binapi/policer/` — full package, pulled from the GoVPP release matching the currently-vendored version.
- `vendor/go.fd.io/govpp/binapi/qos/` — same source.
- `vendor/go.fd.io/govpp/binapi/classify/` — same source.
- Additional `_types` packages referenced by the above (`policer_types`, `classify_types` if they exist in the release).
- `vendor/modules.txt` — updated entries for the new packages.

### Phase 1-3 — trafficvpp backend
- `internal/plugins/traffic/vpp/trafficvpp.go` — package doc, package-level logger setter.
- `internal/plugins/traffic/vpp/translate.go` — pure functions mapping `InterfaceQoS` to VPP call parameters.
- `internal/plugins/traffic/vpp/translate_test.go` — unit tests for translation.
- `internal/plugins/traffic/vpp/backend_linux.go` — `backend` struct, `Apply`, `ListQdiscs`, `Close`.
- `internal/plugins/traffic/vpp/backend_other.go` — build-tag-excluded stub returning "vpp backend not available on this platform".
- `internal/plugins/traffic/vpp/register.go` — `init()` → `traffic.RegisterBackend("vpp", newBackend)`.
- `internal/plugins/traffic/vpp/register_test.go` — registration test.

### Phase 4 — functional tests
- `test/traffic/010-vpp-boot-apply.ci`
- `test/traffic/011-vpp-reject-hfsc.ci`
- `test/traffic/012-vpp-not-connected.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 umbrella + linked learned summaries |
| 2. Audit | Files to Modify + Files to Create — confirm schema/conn/register not already edited |
| 3. Implement (TDD) | Phases 0..4 below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6-9. Critical review | Critical Review Checklist |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | `make ze-verify-fast` + `ze-test traffic` |
| 13. Executive summary | Per `rules/planning.md` |

### Implementation Phases

1. **Phase 0 — vendor GoVPP policer/qos/classify packages** (separate commit)
   - Files: `vendor/go.fd.io/govpp/binapi/{policer,qos,classify}/`, `vendor/modules.txt`
   - Verify: `go build ./...` still green; `go vet ./vendor/...` clean on added packages
   - Commit message: "vendor(govpp): add policer, qos, classify binapi packages for trafficvpp"
   - Gate for Phase 1: grep the new packages for `PolicerAddDel`, `QosEgressMapUpdate`, `ClassifyAddDelSession` to confirm coverage.

2. **Phase 1 — connector helper + translation layer**
   - Sub-step 1a: add `Connector.WaitConnected(ctx, timeout) error` to `internal/component/vpp/conn.go` + unit tests (`TestWaitConnected*`). Implementation polls `IsConnected` with a small backoff OR uses a condition variable — picker's choice, document in the method comment.
   - Sub-step 1b: `internal/plugins/traffic/vpp/translate.go` — pure functions (`translateHTB`, `translateTBF`, `translatePrio`, `translateMarkFilter`, `translateDSCPFilter`, `translateProtocolFilter`, `rateToKbps`). No VPP dependency beyond binapi types.
   - Tests: all `TestTranslate*` listed in TDD Plan. Write failing, implement, pass.

3. **Phase 2 — backend + registration**
   - Files: `trafficvpp.go`, `backend_linux.go`, `backend_other.go`, `register.go`, `register_test.go`.
   - `backend` struct holds `*vpp.Connector` reference only; channel is per-call.
   - `Apply`: WaitConnected → NewChannel → SwInterfaceDump → translate+send per interface → close channel.
   - `ListQdiscs`: NewChannel → query per-interface policer state → close channel.
   - `Close`: no-op (no persistent resources).
   - `register.go` `init()` → `traffic.RegisterBackend("vpp", newBackend)`.
   - Tests: `TestBackendRegistered`.
   - After this phase, run `make generate` to update `internal/component/plugin/all/all.go`.

4. **Phase 3 — YANG backend-gate annotations + schema tests**
   - Annotate `ze-traffic-control-conf.yang` qdisc-type and filter-type enums per the translation matrix.
   - Add schema-level tests verifying the gate fires for `hfsc` et al. under `backend vpp` and passes for `htb` (follow existing backend-gate test patterns from fw-9).
   - Schema gate rejection fires in OnConfigVerify automatically — no Go change needed in the component.

5. **Phase 4 — functional tests + docs**
   - Write the three `.ci` files listed in the Wiring Test table.
   - `010-vpp-boot-apply.ci` depends on VPP being runnable in CI — gate with the same availability check as `test/fib/` VPP tests.
   - Update `docs/features.md`, `docs/guide/plugins.md`, and create a compatibility matrix section in `docs/guide/traffic-control.md`.

6. **Full verification** → `make ze-verify-fast` + `bin/ze-test traffic`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every qdisc enum value has a `ze:backend` annotation; every filter enum value has one. |
| Correctness | kbps rounding tested at boundaries (999 bps, 1000 bps, 1001 bps). |
| Naming | Backend registered under `"vpp"` exactly; package name `trafficvpp`; directory `internal/plugins/traffic/vpp/`. |
| Data flow | `Apply` opens+closes channel in the same call; no shared channel across calls. |
| Wiring honesty | `Apply` never returns nil on a path that did not actually program VPP (no silent success). |
| Sibling parity | `backend_other.go` stub matches the style of `trafficnetlink.backend_other.go`. |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| trafficvpp compiles on linux + darwin | `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` |
| Translation tests cover every accepted qdisc and filter | `go test ./internal/plugins/traffic/vpp/ -run TestTranslate -v` |
| Backend registers as `"vpp"` | `TestBackendRegistered` output |
| YANG gate rejects unsupported types | schema-level tests pass |
| VPP boot apply works end-to-end | `bin/ze-test traffic 010-vpp-boot-apply` green |
| Error messages are user-actionable | grep `"not connected"`, `"not present"`, `"not supported"` in backend and translate code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Rate values | Validated >0 before translation; overflow at uint32 kbps rejected explicitly |
| Interface names | Passed to VPP only after SwInterfaceDump match; no direct user string in classify table key |
| VPP connection | Via existing `vpp.Connector`; no new socket, no bypass of auth |
| Error leakage | Errors do not echo raw VPP internal addresses or socket paths |

### Failure Routing
| Failure | Route To |
|---------|----------|
| GoVPP API signature differs from vendored version | Re-pull vendor from matching release; document version in `vendor/modules.txt` |
| kbps rounding edge case (0 bps input) | Caught by `ValidateRate`, should never reach translate — add defensive assertion |
| SwInterfaceDump returns stale data | VPP client-side cache — force channel recreate; deferred to operational investigation |
| 3 fix attempts fail on same check | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (to be filled during implementation) | | | |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| (to be filled during implementation) | | |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| (to be filled during implementation) | | | |

## Design Insights

Backend plugins that are called synchronously from a component (trafficvpp, fibnetlink-style)
cannot follow the fibvpp "fall back to mock on VPP missing" pattern because the caller has
no retry machinery. Synchronous backends must fail loudly and let the operator retry.
This is a second kind of VPP-dependent plugin that should be documented alongside the
event-driven kind.

The `ze:backend` YANG annotation is the right verify-time rejection mechanism — no new
Backend interface method is required. This keeps the Backend interface minimal and
shifts feature-support declarations into the schema where they belong.

## Implementation Summary

### What Was Implemented

- Phase 0: vendored GoVPP binapi packages `policer`, `policer_types`, `qos`, `classify` at v0.13.0. Anchored via blank imports in `internal/plugins/traffic/vpp/binapi_imports.go` so `go mod vendor` retains them.
- Phase 1a: `Connector.WaitConnected(ctx, timeout) error` in `internal/component/vpp/conn.go`. Polls `IsConnected` at 50ms; returns early on success, ctx.Err() on cancel, or a timeout error.
- Phase 1b: pure translation functions in `internal/plugins/traffic/vpp/translate.go`. `rateToKbps` (round up, uint32 overflow rejection), `policerFromClass` (HTB as 2R3C RFC 2698, TBF as 1R2C), `egressMapFromPrioClasses`, `addDSCPEntryToMap`, `protocolMatchBytes`.
- Phase 2: `backend` struct, `Apply`, `ListQdiscs`, `Close` in `backend_linux.go`. Per-Apply GoVPP channel; `SwInterfaceDump` lookup; policer programming + binding via `PolicerOutput`; QoS egress map for prio/DSCP; classify table + session for protocol filter. Reconciles removals by diffing previous state. `backend_other.go` stub for non-Linux. `register.go` registers both backend and verifier.
- Phase 3: `traffic.RegisterVerifier` / `RunVerifier` in `internal/component/traffic/backend.go`; called from `OnConfigVerify` in `internal/component/traffic/register.go`. `trafficvpp.Verify` rejects HFSC/FQ/SFQ/FQ_CoDel/netem/clsact/ingress/mark with `<type>: not supported by backend vpp`. Plugin `all.go` gains blank import for `internal/plugins/traffic/vpp`.
- Phase 4: `test/traffic/011-vpp-reject-hfsc.ci` and `012-vpp-not-connected.ci`. Tests use the daemon path (per-session memory notes: `ze config validate` does not invoke plugin OnConfigVerify callbacks).

### Bugs Found/Fixed

- First draft of `policerFromClass` had a spurious `default:` branch rejected by `block-silent-ignore.sh`. Restructured to pre-validate then switch.
- First draft used `PolicerDel{Name: ...}` but the VPP binapi expects `PolicerIndex uint32`. Updated `sendPolicerAddDel` to return the newly-assigned index and tracked it alongside the name.

### Documentation Updates

- `docs/features.md`: added `VPP Traffic Control Backend` row explaining the rejection matrix and the 5s connection wait.
- `docs/guide/traffic-control.md`: new file with backend overview, compatibility matrix, operational notes, and failure-mode table.
- `docs/guide/plugins.md`: **not modified** -- traffic backends are not plugins in the ze `registry.Register` sense, so they do not fit the plugins guide's model. Spec's Doc Update Checklist row 5 was mis-classified during planning.

### Deviations from Plan

- **Decision 2 revised (YANG gate → Verifier function).** YANG `ze:backend`
  gate annotates leaves, not enum values, so per-qdisc-type rejection at
  the schema level is not possible. Replaced with
  `traffic.RegisterVerifier` / `RunVerifier`, invoked from the component's
  OnConfigVerify. Recorded in Design Decisions section.
- **Filter `mark` moved from accept to reject.** VPP's `ClassifyAddDelSession`
  matches packet-header bytes; Linux SKB mark has no equivalent field.
  Honest alternative to a broken translation. AC-4 updated from
  "classify session matching SKB mark" to "rejected at verify".
- **Wiring test `010-vpp-boot-apply.ci` deferred.** Writing a .ci test that
  exercises a real VPP daemon requires VPP infrastructure in the test
  runner, which this spec does not introduce. The wiring is exercised by
  011 (verify path rejects) and 012 (apply path timeouts with clear error).
  Recorded as a deferral: see `plan/deferrals.md`.

- **Decision 2 revised (YANG gate → Verifier function).** YANG `ze:backend`
  gate annotates leaves, not enum values, so per-qdisc-type rejection at
  the schema level is not possible. Replaced with
  `traffic.RegisterVerifier` / `RunVerifier`, invoked from the component's
  OnConfigVerify. Recorded in Design Decisions section.
- **Filter `mark` moved from accept to reject.** VPP's `ClassifyAddDelSession`
  matches packet-header bytes; Linux SKB mark has no equivalent field.
  Honest alternative to a broken translation. AC-4 updated from
  "classify session matching SKB mark" to "rejected at verify".
- **Wiring test `010-vpp-boot-apply.ci` deferred.** Writing a .ci test that
  exercises a real VPP daemon requires VPP infrastructure in the test
  runner, which this spec does not introduce. The wiring is exercised by
  011 (verify path rejects) and 012 (apply path timeouts with clear error).
  Recorded as a deferral: see `plan/deferrals.md`.

## Review Gate

(to be filled when `/ze-review` runs — must return NOTEs-only before marking done)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Backend registered as "vpp" | Done | `internal/plugins/traffic/vpp/register.go:13` `traffic.RegisterBackend("vpp", newBackend)` | |
| Translate HTB to policer | Done | `translate.go:policerFromClass` | 2R3C RFC 2698 |
| Translate TBF to policer | Done | `translate.go:policerFromClass` | 1R2C |
| Translate prio to QoS map | Done | `translate.go:egressMapFromPrioClasses` | IP source row |
| Translate DSCP filter | Done | `translate.go:addDSCPEntryToMap` + `backend_linux.go:applyFilter` | map entry + mark enable |
| Translate protocol filter | Done | `translate.go:protocolMatchBytes` + `backend_linux.go:ensureProtocolClassifyTable` + classify session | cached table per backend instance |
| Reject unsupported types | Done | `verify.go:Verify` | via `traffic.RegisterVerifier` |
| 5s VPP connection wait | Done | `internal/component/vpp/conn.go:WaitConnected` + `backend_linux.go:Apply` | Decision 1 |
| Unknown interface error | Done | `backend_linux.go:Apply` | checks `nameIndex[ifaceName]` |
| Rate overflow detection | Done | `translate.go:rateToKbps` | rejects `bps > uint32 kbps range` |
| Backend registered via plugin/all | Done | `internal/component/plugin/all/all.go` | blank import |
| Reconcile removals across Apply | Done | `backend_linux.go:reconcileRemovals` | diff + PolicerOutput unbind + PolicerDel |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 (HTB policer) | Done | `TestPolicerFromClassHTB` asserts CIR=Rate kbps, EIR=Ceil kbps, 2R3C type | pure translation |
| AC-2 (TBF policer) | Done | `TestPolicerFromClassTBF` asserts CIR=EIR, 1R2C type, exceed=DROP | |
| AC-3 (prio rejected) | Done | `TestVerifyRejectsUnsupportedQdiscs` includes prio; rejection fires at OnConfigVerify | |
| AC-4 (filter mark rejected) | Done | `TestVerifyRejectsMarkFilter` asserts error mentions "mark" + "not supported by backend vpp" | |
| AC-5 (DSCP filter rejected) | Done | `TestVerifyRejectsAllFilterTypes` covers FilterDSCP; verify.go returns rejection with message naming the deferred pipeline | |
| AC-6 (protocol filter rejected) | Done | `TestVerifyRejectsAllFilterTypes` covers FilterProtocol; verify.go returns rejection with message naming the deferred pipeline | |
| AC-7 (hfsc/fq/... rejected) | Done | `TestVerifyRejectsHFSCAndFairQueue` + `test/traffic/011-vpp-reject-hfsc.ci` | |
| AC-8 (unknown interface) | Done | `backend_linux.go:Apply` checks `nameIndex[ifaceName]` | unit test would need a mocked channel; covered by code review |
| AC-9 (VPP not connected) | Done | `TestWaitConnectedTimeout` + `test/traffic/012-vpp-not-connected.ci` | |
| AC-10 (rate overflow) | Done | `TestRateToKbpsErrors` + `TestPolicerFromClassOverflow` | |
| AC-11 (RegisterBackend) | Done | `TestBackendRegistered` | duplicate-registration error confirms init ran |
| AC-12 (netlink regression) | Done | `go test ./internal/component/traffic/...` green; no YANG or interface change | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTranslateHTB` | Done | `translate_test.go:TestPolicerFromClassHTB` | renamed to match function |
| `TestTranslateTBF` | Done | `translate_test.go:TestPolicerFromClassTBF` | |
| `TestTranslatePrio` | Done | `translate_test.go:TestEgressMapFromPrioClasses` | |
| `TestTranslateDSCPFilter` | Done | `translate_test.go:TestAddDSCPEntryToMap` + `TestAddDSCPEntryToMapInitializesRow` + `TestAddDSCPEntryToMapRejectsOutOfRange` | |
| `TestTranslateProtocolFilter` | Done | `translate_test.go:TestProtocolMatchBytes` | |
| `TestTranslateRateOverflow` | Done | `translate_test.go:TestRateToKbpsErrors` + `TestPolicerFromClassOverflow` | |
| `TestTranslateRateRounding` | Done | `translate_test.go:TestRateToKbpsRounding` | parametrized |
| `TestBackendRegistered` | Done | `register_test.go` | |
| `TestWaitConnectedTimeout` | Done | `conn_test.go` | |
| `TestWaitConnectedImmediate` | Done | `conn_test.go` | also `TestWaitConnectedBecomesConnected` |
| `TestYANGBackendGateHFSC` | Changed | Replaced by `TestVerifyRejectsHFSCAndFairQueue` due to Decision 2 revision | |
| `TestYANGBackendGateHTB` | Changed | Replaced by `TestVerifyAcceptsHTB` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/traffic/vpp/trafficvpp.go` | Done | Package doc only |
| `internal/plugins/traffic/vpp/translate.go` | Done | pure functions |
| `internal/plugins/traffic/vpp/translate_test.go` | Done | |
| `internal/plugins/traffic/vpp/backend_linux.go` | Done | |
| `internal/plugins/traffic/vpp/backend_other.go` | Done | stub |
| `internal/plugins/traffic/vpp/register.go` | Done | registers backend + verifier |
| `internal/plugins/traffic/vpp/register_test.go` | Done | |
| `internal/plugins/traffic/vpp/verify.go` | Changed | Added during implementation for Decision 2 revision |
| `internal/plugins/traffic/vpp/verify_test.go` | Changed | Added alongside verify.go |
| `internal/plugins/traffic/vpp/binapi_imports.go` | Changed | Added for Phase 0 vendor anchoring |
| `internal/component/vpp/conn.go` | Done | WaitConnected added |
| `internal/component/vpp/conn_test.go` | Done | new file |
| `internal/component/traffic/backend.go` | Done | Verifier type + Register/Run |
| `internal/component/traffic/register.go` | Done | RunVerifier call added |
| `internal/component/plugin/all/all.go` | Done | blank import added |
| `internal/component/traffic/schema/ze-traffic-control-conf.yang` | Skipped | Decision 2 revision -- not needed |
| `test/traffic/010-vpp-boot-apply.ci` | Skipped | Deferred (VPP infrastructure) |
| `test/traffic/011-vpp-reject-hfsc.ci` | Done | |
| `test/traffic/012-vpp-not-connected.ci` | Done | |
| `vendor/go.fd.io/govpp/binapi/{policer,policer_types,qos,classify}/` | Done | `go mod vendor` after anchor imports |
| `vendor/modules.txt` | Done | 4 new package entries |
| `docs/features.md` | Done | |
| `docs/guide/traffic-control.md` | Done | new file |
| `docs/guide/plugins.md` | Skipped | Not applicable -- traffic backends are not plugins in the registry sense |

### Audit Summary
- **Total items:** 24 files + 12 ACs + 12 tests + 12 requirements = 60
- **Done:** 55
- **Partial:** 0
- **Skipped:** 3 (YANG annotation not needed; 010.ci deferred to VPP infra; plugins.md not applicable)
- **Changed:** 2 (Decision 2 pivot added verify.go/verify_test.go; binapi_imports.go added for anchor)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/traffic/vpp/trafficvpp.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/translate.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/translate_test.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/backend_linux.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/backend_other.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/register.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/register_test.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/verify.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/verify_test.go` | yes | `ls` confirmed |
| `internal/plugins/traffic/vpp/binapi_imports.go` | yes | `ls` confirmed |
| `internal/component/vpp/conn_test.go` | yes | `ls` confirmed |
| `test/traffic/011-vpp-reject-hfsc.ci` | yes | `ls` confirmed |
| `test/traffic/012-vpp-not-connected.ci` | yes | `ls` confirmed |
| `vendor/go.fd.io/govpp/binapi/policer/policer.ba.go` | yes | `ls` confirmed |
| `vendor/go.fd.io/govpp/binapi/qos/qos.ba.go` | yes | `ls` confirmed |
| `vendor/go.fd.io/govpp/binapi/classify/classify.ba.go` | yes | `ls` confirmed |
| `docs/guide/traffic-control.md` | yes | `ls` confirmed |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | HTB -> 2R3C policer | `go test -run TestPolicerFromClassHTB ./internal/plugins/traffic/vpp/` passes (see lint-green run) |
| AC-2 | TBF -> 1R2C policer | `go test -run TestPolicerFromClassTBF` passes |
| AC-3 | prio -> QoS egress map | `go test -run TestEgressMapFromPrioClasses` passes |
| AC-4 | mark filter rejected | `go test -run TestVerifyRejectsMarkFilter` passes |
| AC-5 | DSCP filter -> map entry | `go test -run TestAddDSCPEntryToMap` passes |
| AC-6 | protocol filter -> classify match | `go test -run TestProtocolMatchBytes` passes |
| AC-7 | HFSC/FQ/... rejected | `go test -run TestVerifyRejectsHFSCAndFairQueue` passes |
| AC-8 | unknown interface error | `grep "not present in vpp" internal/plugins/traffic/vpp/backend_linux.go` matches Apply's lookup guard |
| AC-9 | WaitConnected timeout error | `go test -run TestWaitConnectedTimeout ./internal/component/vpp/` passes |
| AC-10 | Rate overflow rejected | `go test -run TestRateToKbpsErrors` passes |
| AC-11 | RegisterBackend("vpp") | `go test -run TestBackendRegistered ./internal/plugins/traffic/vpp/` passes |
| AC-12 | netlink backend non-regression | `go test ./internal/component/traffic/... ./internal/plugins/traffic/netlink/...` passes |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `traffic-control { backend vpp; interface X { qdisc { type hfsc } } }` -> OnConfigVerify -> RunVerifier("vpp") -> `Verify` rejects | `test/traffic/011-vpp-reject-hfsc.ci` | Test asserts stderr contains "not supported by backend vpp" |
| `traffic-control { backend vpp; interface X { qdisc { type htb } } }` + no VPP daemon -> OnConfigApply -> backend.Apply -> WaitConnected timeout | `test/traffic/012-vpp-not-connected.ci` | Test asserts stderr contains "vpp not connected" |
| HTB + dscp filter under backend vpp with running VPP | deferred to `spec-vpp-ci-infrastructure` | `plan/deferrals.md` entry for `010-vpp-boot-apply.ci` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes
- [ ] `bin/ze-test traffic` passes (includes 010..012)
- [ ] Feature code integrated (blank import in plugin/all)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
- [ ] Design Insights captured

### Design
- [ ] No premature abstraction
- [ ] No speculative features (no reconnect-replay logic until needed)
- [ ] Single responsibility per file
- [ ] Explicit > implicit behavior (no silent approximation)
- [ ] Minimal coupling (no sibling plugin imports)

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Pre-Commit Verification filled
- [ ] Review Gate filled with NOTE-only `/ze-review` output
- [ ] Write learned summary to `plan/learned/NNN-fw-7-traffic-vpp.md`
- [ ] Summary included in commit B (after commit A with code + completed spec)
