# Spec: iface-resolve-1-model — permanent MAC + os-name selector + show

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-resolve-0-umbrella |
| Phase | 1/4 (model/state foundation) |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-resolve-0-umbrella.md` - parent umbrella (model, decisions, consumer matrix)
3. `plan/learned/523-iface-mac-discovery.md` - the os-name / mac-binding decision
4. `internal/component/iface/iface.go` (InterfaceInfo), `internal/plugins/iface/netlink/show_linux.go`

## Task

Lay the data/state foundation for logical-name → device resolution, WITHOUT yet building the
resolver algorithm (sub-spec 2) or migrating any consumer (sub-specs 3-7). Specifically:

1. **Read the permanent MAC** (`IFLA_PERM_ADDRESS` via `netlink.LinkAttrs.PermHWAddr`) and store it
   on `InterfaceInfo` as a new `PermanentMAC` field, distinct from the operational `MAC`.
2. **Surface `os-name` in `show interface`** as a visible `InterfaceInfo.OsName` field (the OS/kernel
   device name). Today it equals `Name`; once the resolver (sub-spec 2) maps an operator-chosen
   logical name to a kernel device, `Name` carries the logical name and `OsName` keeps the kernel
   device so `show interface` shows both sides of the mapping.
3. **Surface the permanent MAC** in `show interface` (`show interface name <x> detail` serializes the
   whole `InterfaceInfo`, so the new fields appear with no formatter change).
4. **Guard** the existing `mac` optional+unique model (regression guard against 523's documented
   linter-revert gotcha; do NOT make mac required).

**Scope boundary (decided this session):** os-name as a *resolution selector* — un-hiding the config
`os-name` leaf, default-to-name, and using it to bind a logical name to a kernel device — is
**dormant until a consumer exists**, so it moves to **sub-spec 2** (the resolver). This sub-spec only
makes os-name *visible* in `show interface` and reads the permanent MAC. No resolution behavior
changes; every consumer still resolves as today.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `plan/spec-iface-resolve-0-umbrella.md` - parent; model + decisions
  → Constraint: binding = os-name (default name) default; mac optional+unique authoritative when present; match only for matched kinds (ethernet/loopback).
  → Constraint: map-only — never rename the kernel device.
- [ ] `plan/learned/523-iface-mac-discovery.md` - os-name leaf + mac-binding origin
  → Constraint: `os-name` was added to preserve the original OS name after the user renames the config entry; `link.Type()` is `device` for both ethernet and loopback (detect `lo` by name first).
- [ ] `ai/rules/config-naming.md`, `ai/patterns/config-option.md` - leaf naming + validation
  → Constraint: every leaf needs max native validation; os-name stays `type string` (kernel ifnames have no fixed grammar) but must reuse `ValidateIfaceName` semantics.

**Key insights:**
- `InterfaceInfo` (`iface.go:103`) has `MAC` but no permanent-MAC field — add `PermanentMAC`.
- iface event topics (`TopicUp/Down/Created/Deleted/AddrAdded`) already exist — sub-spec 2 builds Subscribe on them; not this sub-spec.
- No resolution behavior changes here.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/iface.go` (L103-119) - `InterfaceInfo{Name,Index,Type,State,MTU,MAC,Addresses,...}`; **no permanent-MAC field**.
  → Constraint: `InterfaceInfo` is a cross-boundary value type (has `plugin.DataMarker`); adding a field is additive and safe.
- [ ] `internal/plugins/iface/netlink/show_linux.go` (L50-75) - `GetInterface` builds `InterfaceInfo` from `netlink.LinkByName`; reads `Attrs().HardwareAddr` (operational MAC) only.
  → Constraint: permaddr must come from `Attrs().PermHWAddr` (vishvananda/netlink) or a raw `IFLA_PERM_ADDRESS` parse if the wrapper lacks it — verify in research.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` (L32-36, L505/536/556) - `os-name` hidden leaf ("Original OS interface name at discovery time"); `unique "mac/address"` on ethernet/veth/bridge with no `ze:required`.
  → Constraint: un-hiding os-name changes editor/show visibility; keep it optional, default = name. Preserve mac optional+unique exactly; a regression test must assert mac stays non-required.
- [ ] `internal/component/iface/cmd/show_interface.go` - `show interface` formatter, current columns.
  → Constraint: add os-name + permanent MAC without breaking existing column parsing in `.ci` tests.

**Behavior to preserve:**
- All resolution behaves exactly as today (no consumer migrated).
- `mac` stays optional + unique.
- Existing `show interface` columns/output that `.ci` tests assert.

**Behavior to change:**
- `InterfaceInfo` gains `PermanentMAC`.
- `os-name` becomes user-visible + documented as the binding selector (still optional, default name).
- `show interface` adds os-name + permanent MAC.

## Data Flow (MANDATORY)

### Entry Point
- `show interface [<name>]` CLI/API; and config `interface <name> { os-name <dev> }`.

### Transformation Path
1. netlink `LinkByName` → `Attrs()` → read `PermHWAddr` (or raw `IFLA_PERM_ADDRESS`).
2. populate `InterfaceInfo.PermanentMAC` alongside `MAC`.
3. `show interface` formatter renders name, os-name, MAC, PermanentMAC.
4. config parse stores `os-name` (defaulting to name) on the interface model.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| netlink ↔ iface | `InterfaceInfo.PermanentMAC` added | [ ] |
| iface ↔ CLI | show formatter renders new fields | [ ] |
| config ↔ iface | os-name parsed + defaulted | [ ] |

### Integration Points
- `GetInterface`/`ListInterfaces` populate the new field; sub-spec 2 consumes it.

### Architectural Verification
- [ ] No bypassed layers (permaddr read only in the netlink backend)
- [ ] No unintended coupling (no consumer reads permaddr directly)
- [ ] No duplicated functionality (single permaddr read path)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | vishvananda/netlink exposes `Attrs().PermHWAddr` | netlink v1.3.1 `link.go:58` has `PermHWAddr net.HardwareAddr` | n/a — confirmed available | grep of module cache | confirmed |
| A-2 | Adding `OsName`/`PermanentMAC` (both `omitempty`) to `InterfaceInfo` is additive and breaks no existing show consumer | new struct fields with `omitempty` emit no JSON key when empty | netlink package tests pass in linux (Docker) | confirmed |
| A-3 | Virtual/created kinds have empty permaddr; that is acceptable (blank in show) | veth/bridge/tunnel have no factory MAC | `TestLinkToInfoNoPermanentMAC` asserts empty `PermanentMAC` for a device with no `PermHWAddr` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | (resolved) netlink v1.3.1 exposes `PermHWAddr`; no raw parse needed | n/a | confirmed available (A-1) |
| R-2 | `show interface` column addition breaks `.ci` string matches | functional test diff | additive column / structured field; update `.ci` assertions deliberately |
| R-3 | Un-hiding os-name confuses operators (discovered vs intended) | docs/UX feedback | document os-name as "OS device this interface binds to (default: the name)" |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze interface show <name>` | → | `GetInterface` → `linkToInfo` (PermHWAddr) → `showOne` renders `OS Name:`/`Perm MAC:` | `test/parse/cli-show-interface-osname.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `linkToInfo`/`GetInterface` on a link with a permanent address | `InterfaceInfo.PermanentMAC` populated from `Attrs().PermHWAddr` (`IFLA_PERM_ADDRESS`), a field distinct from `MAC` (`TestLinkToInfoPermanentMAC`) |
| AC-2 | `show interface name <x> detail` | serialized output includes `os-name` and `permanent-mac-address` alongside `mac-address` (no formatter change — the detail view returns the whole struct) |
| AC-3 | link with `PermHWAddr` set + a different operational `HardwareAddr`; and a virtual link with neither | permaddr distinct from the operational MAC override; empty `PermanentMAC` for the virtual kind (`TestLinkToInfoPermanentMAC` / `TestLinkToInfoNoPermanentMAC`) |
| AC-4 | the embedded `ze-iface-conf.yang` | `unique "mac/address"` retained on ethernet/veth/bridge and no `ze:required` on mac — optional+unique guard (`TestMacBindingUniqueRetained`) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | `ze interface show eth0` to see its os-name + permanent vs current MAC | GetInterface → linkToInfo (PermHWAddr) → showOne renders | `test/parse/cli-show-interface-osname.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkToInfoPermanentMAC` | `internal/plugins/iface/netlink/show_linux_test.go` | permaddr + os-name read into InterfaceInfo; distinct from operational MAC | PASS (linux/Docker) |
| `TestLinkToInfoNoPermanentMAC` | `internal/plugins/iface/netlink/show_linux_test.go` | empty permaddr for a virtual kind | PASS (linux/Docker) |
| `TestMacBindingUniqueRetained` | `internal/component/iface/yang/mac_binding_test.go` | mac stays optional + unique | PASS (host) |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| mac/address | 6 octets hex | `02:42:ac:11:00:02` | malformed (5 octets) | malformed (7 octets) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-show-interface-osname` | `test/parse/cli-show-interface-osname.ci` | `ze interface show lo` renders `OS Name:` | PASS (QEMU) |

### Interop Tests
N/A — no wire-protocol behavior; this is local interface state only.

## Files to Modify
- `internal/component/iface/iface.go` - add `PermanentMAC` to `InterfaceInfo`
- `internal/component/iface/iface.go` - add `OsName` + `PermanentMAC` fields to `InterfaceInfo`
- `internal/plugins/iface/netlink/show_linux.go` - `linkToInfo` populates both from `Attrs()` (`PermHWAddr`)
- `internal/plugins/iface/netlink/show_linux_test.go` - add permaddr/os-name unit tests
- `internal/component/iface/cli/show.go` - `showOne` renders `OS Name:` and `Perm MAC:` lines via a new host-testable `formatInterfaceDetail` (text formatter previously omitted them; only `--json` showed them)
- `internal/component/iface/cli/show_test.go` - `TestFormatInterfaceDetail` / `…NoPermMAC` (covers the Perm MAC render path loopback can't)
- `internal/component/iface/health.go`, `internal/component/web/page_interfaces.go`, `internal/component/web/page_ip_addresses.go`, `internal/component/web/page_traffic.go` - range `[]InterfaceInfo` by index (the struct grew 152→184 B, crossing gocritic `rangeValCopy` threshold 160; `/ze-review`-caught cascade in unchanged web code)
- `docs/guide/command-reference.md` - note that `show interface detail` / `ze interface show` exposes `OS Name` + `Perm MAC` (source-anchored)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | additive runtime field; no config-leaf change in this sub-spec |
| CLI show output | Yes (automatic) | `show interface name <x> detail` returns the whole struct; no formatter change |
| Functional test for show | Yes | `test/parse/cli-show-interface-osname.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | additive fields to existing `show interface detail` output, not a new feature; no interface feature row in `docs/features.md` (grep) |
| 2 | Config syntax changed? | No | no config-leaf change (os-name leaf untouched; resolution is sub-spec 2) |
| 4 | API/RPC changed? | No | `ze-show:interface-detail` response gains two `omitempty` fields; additive, no doc field-table exists to update (grep) |

## Files to Create
- `internal/component/iface/yang/mac_binding_test.go` - mac optional+unique invariant guard
- `test/parse/cli-show-interface-osname.ci` - show-interface os-name visibility (QEMU/CI)

## Implementation Steps

### Implementation Phases
1. **Phase: fields + populate** — add `OsName`+`PermanentMAC` to `InterfaceInfo`; `linkToInfo` sets `OsName=attrs.Name`, `PermanentMAC=attrs.PermHWAddr`
   - Files: `iface.go`, `show_linux.go`
   - Verify: host vet + `GOOS=linux` vet clean
2. **Phase: unit tests** — `TestLinkToInfoPermanentMAC`, `TestLinkToInfoNoPermanentMAC`
   - Files: `show_linux_test.go`
   - Verify: PASS in linux (`make ze-linux-test`)
3. **Phase: mac guard** — `TestMacBindingUniqueRetained`
   - Files: `yang/mac_binding_test.go`
   - Verify: PASS (host)
4. **Phase: functional test** — `test/parse/cli-show-interface-osname.ci` (os-name in show; QEMU/CI)
5. **Complete spec** → audit + learned summary (949); two commits

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each have impl + test at file:line |
| Correctness | permaddr distinct from operational MAC; os-name defaults correctly |
| Data flow | permaddr read only in netlink backend; no consumer reads it directly |
| Naming | `PermanentMAC` field; `os-name` YANG leaf kebab-case |
| YANG validation | os-name reuses iface-name validation; mac stays unique+optional |
| Rule: no resolution change | no consumer migrated; grep shows resolution untouched |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `PermanentMAC` field | `grep PermanentMAC internal/component/iface/iface.go` |
| permaddr read | `grep -i PERM_ADDRESS\|PermHWAddr internal/plugins/iface/netlink/` |
| os-name visible | YANG: `os-name` no longer `ze:hidden` |
| show output renders OS Name | `cli-show-interface-osname.ci` PASS in QEMU |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | os-name validated via `ValidateIfaceName` semantics |
| Info leakage | permaddr is not secret; safe to show |

### Failure Routing
| Failure | Route To |
|---------|----------|
| netlink lacks PermHWAddr | raw IFLA_PERM_ADDRESS parse (R-1) |
| show `.ci` breaks | update assertions deliberately (R-2) |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `OsName` is a runtime `InterfaceInfo` field (= kernel device name) for show visibility | un-hide the config `os-name` leaf now | the config leaf is dormant (read by nothing); exposing an inert binding knob is speculative — os-name *resolution* moves to sub-spec 2 |
| Add `OsName`/`PermanentMAC` to `InterfaceInfo` (additive, `omitempty`) | new struct | additive, cross-boundary-safe, minimal churn; auto-surfaces in `show interface detail` |
| No resolution change in this sub-spec | wire resolver here too | keeps the foundation reviewable; resolver is sub-spec 2 |

## Known Limitations
- No resolution behavior changes; consumers still resolve as today (sub-spec 2+).
- Virtual/created kinds report empty permaddr (no factory MAC).

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| permanent MAC readable, distinct from operational MAC | unit test (linux) | `TestLinkToInfoPermanentMAC` PASS via `make ze-linux-test` (Docker: `ok ... iface/netlink 0.010s`) |
| os-name visible in `show interface` | functional (QEMU) | `test/parse/cli-show-interface-osname.ci` **PASS in QEMU** (`ze interface show lo` renders `OS Name:`; `PASS 48 cli-show-interface-osname`); `TestLinkToInfoPermanentMAC` asserts `OsName` populated |
| mac stays optional + unique | host test | `TestMacBindingUniqueRetained` PASS (host) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `OsName == Name` until the resolver (sub-spec 2) makes `Name` logical; transitional redundancy, documented on the field | `iface.go` InterfaceInfo | acknowledged (intentional foundation) |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/iface.go` | yes | `OsName` + `PermanentMAC` fields added |
| `internal/plugins/iface/netlink/show_linux.go` | yes | `linkToInfo` populates both from `Attrs()` |
| `internal/component/iface/cli/show.go` | yes | `showOne` renders `OS Name:` / `Perm MAC:` (QEMU-verified) |
| `internal/plugins/iface/netlink/show_linux_test.go` | yes | `TestLinkToInfoPermanentMAC` / `TestLinkToInfoNoPermanentMAC` |
| `internal/component/iface/yang/mac_binding_test.go` | yes | `TestMacBindingUniqueRetained` |
| `test/parse/cli-show-interface-osname.ci` | yes | functional (QEMU PASS) |
| `internal/component/iface/cli/show_test.go` | yes | `TestFormatInterfaceDetail` (Perm MAC render, host PASS) |
| `internal/component/iface/health.go` + `web/page_{interfaces,ip_addresses,traffic}.go` | yes | rangeValCopy cascade fixed (web lint 0 issues) |
| `docs/guide/command-reference.md` | yes | os-name / Perm MAC note (source-anchored) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `PermanentMAC` from `PermHWAddr`, distinct from `MAC` | `TestLinkToInfoPermanentMAC` PASS (Docker linux: `ok iface/netlink 0.010s`) |
| AC-2 | `ze interface show <name>` renders os-name (+ perm MAC) | `cli-show-interface-osname.ci` **PASS in QEMU** (`PASS 48 cli-show-interface-osname`); `showOne` prints `OS Name:` / `Perm MAC:` |
| AC-3 | distinct from override; empty for virtual | `TestLinkToInfoPermanentMAC` + `TestLinkToInfoNoPermanentMAC` PASS |
| AC-4 | mac optional + unique | `TestMacBindingUniqueRetained` PASS (host) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `PermHWAddr` in vishvananda/netlink v1.3.1 `link.go:58` |
| A-2 | confirmed | `omitempty` fields are additive; netlink package tests pass in linux |
| A-3 | confirmed | `TestLinkToInfoNoPermanentMAC` (empty permaddr for a device with no `PermHWAddr`) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
