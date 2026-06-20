# Spec: iface-resolve-1-model — permanent MAC + os-name selector + show

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-iface-resolve-0-umbrella |
| Phase | 1 (model/state foundation) |
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

1. **Read the permanent MAC** (`IFLA_PERM_ADDRESS`) and store it on `InterfaceInfo` as a new
   `PermanentMAC` field, distinct from the operational `MAC`.
2. **Promote `os-name`** from a hidden discovery-only leaf to a real (still optional) binding
   selector: it defaults to the interface `name`; when set, it names the OS device this logical
   interface maps to.
3. **Surface both** in `show interface`: logical name, os-name, operational MAC, permanent MAC.
4. **Confirm + guard** the existing `mac` optional+unique model (regression guard; do NOT make it
   required).

This sub-spec changes no resolution behavior — every consumer still resolves as today. It only
exposes the state the resolver (sub-spec 2) and consumers will use.

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
| A-2 | Un-hiding `os-name` does not break existing `.ci`/editor expectations | os-name is currently hidden, absent from output | show/editor `.ci` diffs; gate the new column | run iface `.ci` after change | unvalidated |
| A-3 | Virtual/created kinds have empty permaddr; that is acceptable (blank in show) | veth/bridge/tunnel have no factory MAC | show must render blank, not error | unit test on a dummy iface | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | (resolved) netlink v1.3.1 exposes `PermHWAddr`; no raw parse needed | n/a | confirmed available (A-1) |
| R-2 | `show interface` column addition breaks `.ci` string matches | functional test diff | additive column / structured field; update `.ci` assertions deliberately |
| R-3 | Un-hiding os-name confuses operators (discovered vs intended) | docs/UX feedback | document os-name as "OS device this interface binds to (default: the name)" |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show interface <name>` | → | `GetInterface` → `InterfaceInfo.PermanentMAC` → formatter | `test/iface/iface-permaddr-show.ci` |
| config `interface x { os-name eth0 }` | → | config parse → iface model stores os-name | `test/iface/iface-osname-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `GetInterface` on a physical NIC | `InterfaceInfo.PermanentMAC` populated from `IFLA_PERM_ADDRESS`, distinct field from `MAC` |
| AC-2 | `show interface <name>` | output shows logical name, os-name, operational MAC, and permanent MAC |
| AC-3 | NIC configured with `mac { address … }` override | `PermanentMAC` != operational `MAC`; the permaddr is unchanged by the override |
| AC-4 | config `interface uplink { os-name eth0 }` then re-read | os-name stored as `eth0`; when omitted, os-name defaults to the interface name |
| AC-5 | two interfaces with identical `mac/address`; and one interface with no mac | duplicate rejected (unique); no-mac accepted (not required) — regression guard |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | `show interface eth0` to see its permanent vs current MAC | GetInterface → permaddr read → show | `test/iface/iface-permaddr-show.ci` |
| 2 | binds `interface uplink { os-name eth0 }` | config parse → model stores os-name | `test/iface/iface-osname-config.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPermanentMACRead` | `internal/plugins/iface/netlink/show_linux_test.go` | permaddr read into InterfaceInfo | |
| `TestPermanentMACDistinctFromOverride` | `internal/plugins/iface/netlink/manage_linux_test.go` | permaddr stable after mac override | |
| `TestOsNameDefaultsToName` | `internal/component/iface/config_test.go` | os-name defaults to name when omitted | |
| `TestMacOptionalUniqueRegression` | `internal/component/iface/config_test.go` | mac stays optional + unique | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| mac/address | 6 octets hex | `02:42:ac:11:00:02` | malformed (5 octets) | malformed (7 octets) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-permaddr-show` | `test/iface/iface-permaddr-show.ci` | show interface displays permanent + current MAC | |
| `iface-osname-config` | `test/iface/iface-osname-config.ci` | os-name binding parses and round-trips | |

### Interop Tests
N/A — no wire-protocol behavior; this is local interface state only.

## Files to Modify
- `internal/component/iface/iface.go` - add `PermanentMAC` to `InterfaceInfo`
- `internal/plugins/iface/netlink/show_linux.go` - populate `PermanentMAC` from `Attrs().PermHWAddr`
- `internal/component/iface/yang/ze-iface-conf.yang` - un-hide + document `os-name`; keep mac unique
- `internal/component/iface/cmd/show_interface.go` - render os-name + permanent MAC
- `internal/component/iface/config*.go` - os-name default-to-name handling

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (os-name visibility) | [ ] | `ze-iface-conf.yang` |
| YANG validation constraints | [ ] | os-name reuses iface-name validation semantics |
| CLI show output | [ ] | iface `cmd/` show formatter |
| Functional test for show | [ ] | `test/iface/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (permanent MAC in show) |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` (os-name binding) |
| 6 | Has a user guide page? | [ ] | `docs/guide/` interface page |
| 17 | Existing docs show iface examples? | [ ] | verify show-interface examples |

## Files to Create
- `test/iface/iface-permaddr-show.ci` - show permaddr functional test
- `test/iface/iface-osname-config.ci` - os-name binding functional test

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add `PermanentMAC` field + failing `iface-permaddr-show.ci`
   - Tests: `iface-permaddr-show`
   - Files: `iface.go`, show formatter
   - Verify: field exists, show renders it (empty), functional test fails on missing permaddr value
2. **Phase: permaddr read** — read `IFLA_PERM_ADDRESS` in the netlink backend
   - Tests: `TestPermanentMACRead`, `TestPermanentMACDistinctFromOverride`
   - Files: `show_linux.go`, `show_other.go`, `manage_linux.go`
   - Verify: permaddr populated; stable under override
3. **Phase: os-name selector** — un-hide + default-to-name + config storage
   - Tests: `TestOsNameDefaultsToName`, `iface-osname-config`
   - Files: `ze-iface-conf.yang`, `config*.go`
   - Verify: os-name parses, defaults, round-trips
4. **Phase: mac regression guard** — `TestMacOptionalUniqueRegression`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit + learned summary; two commits

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
| show output | run `test/iface/iface-permaddr-show.ci` |

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
| Single `os-name` field serves both discovered + intended binding | separate `match/os-name` leaf | one field is simpler; the discovered value IS the intended binding by default |
| Add `PermanentMAC` to `InterfaceInfo` (additive) | new struct | additive, cross-boundary-safe, minimal churn |
| No resolution change in this sub-spec | wire resolver here too | keeps the foundation reviewable; resolver is sub-spec 2 |

## Known Limitations
- No resolution behavior changes; consumers still resolve as today (sub-spec 2+).
- Virtual/created kinds report empty permaddr (no factory MAC).

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| permanent MAC readable + shown | functional test | `test/iface/iface-permaddr-show.ci` |
| os-name as optional binding selector | functional test | `test/iface/iface-osname-config.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [from /ze-review] | file:line | fixed / deferred / acknowledged |

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

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
