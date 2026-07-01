# Spec: Generic Plugin-Owned Address Registry for iface

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-as112-0-umbrella.md` - parent spec, RFC compliance context
4. `internal/component/iface/config_apply.go` - reconciliation this registers into
5. `internal/component/iface/backend.go:216-229` - existing registry pattern this mirrors

## Task

Add a generic, plugin-facing address-ownership registration API to
`internal/component/iface` so that a plugin can declare "these addresses
belong to me on this interface," and have `desiredState()` treat them as
desired (never as stray/removed) for as long as the plugin holds the
registration — without the operator separately declaring the same addresses
under the interface's YANG config. Generic and not AS112-specific: the API
must not name `as112` anywhere inside `internal/component/iface`. The AS112
DNS plugin (`spec-as112-2-dns-server.md`) is the first and, in this spec set,
only consumer.

**Registration must trigger reconciliation (review finding B1).** A registration
that merely mutates a map is not enough: iface's reconcile runs only from
`applyConfig` (`config_apply.go:729`) and the vpp `EventConnected`/
`EventReconnected` handlers (`register.go:257` `subscribeReconcileOnReady`) — so
an address registered during a plugin's config handler would NOT reach `lo`
until some later, unrelated config commit (and plugin handler order is
non-deterministic — `plan/learned/821-plugin-internal-keyword.md`, so we cannot
rely on ordering as112's handler before iface's). Therefore
`RegisterOwnedAddresses`/`UnregisterOwnedAddresses` MUST publish a
reconcile-trigger (reusing the existing `subscribeReconcileOnReady` trigger
path) so iface re-runs reconciliation promptly after the registry changes, in
the same enable/disable operation. This is the crux the earlier draft left as an
unvalidated assumption.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` - registration-over-hardcoding pattern
  → Constraint: core/shared packages discover registrants through a registry; they never special-case a plugin name. This registry must work the same way for any future plugin, not just as112.
- [ ] `ai/rules/module-tiers.md` - dependency direction between components and plugins
  → Constraint: `internal/component/iface` is a component; plugins depend on components, not vice versa. The registry lives in iface and is called *by* plugins through the plugin SDK / a small exported Go API — iface must not import any plugin package.

### RFC Summaries
N/A — this child is infrastructure, not protocol behavior. (Required by
`spec-as112-2-dns-server.md`'s RFC-driven addressing; covered there.)

**Key insights:**
- `internal/component/iface/backend.go:216-229` already shows the exact shape
  to mirror: a package-level `sync.Mutex` (`backendsMu`) guarding a
  registration map, with a `RegisterBackend(name, factory)` function
  (`backend.go:229`). The new address registry follows the same shape:
  package-level mutex-guarded map, register/unregister functions.
- `internal/component/iface/cmd/manage.go:80-94` (`handleAddrAdd`, RPC
  `ze-iface:interface-addr-add`) is NOT the mechanism — it's imperative-only
  and bypasses `desiredState()`, so anything added through it is removed again
  on the next reconciliation pass (`config_apply.go:778-813`). The registry
  must be consulted *inside* `desiredState()` itself.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config_apply.go` - `desiredState()` (lines 18-110) builds a `map[string]map[string]bool` (OS interface name → CIDR → true) plus a managed-name set, purely from parsed YANG `unitEntry.Addresses`; reconciliation (`reconcileOnReadyWithJournal`, lines 757-813) diffs this against `currentAddrSet()` (live kernel state) and calls `AddAddress`/`RemoveAddress` for the difference — any kernel address absent from `desiredState()`'s output is removed as stray.
- [ ] `internal/component/iface/config.go` - `parseUnits()` (lines 875-954) is the sole current producer of per-unit `Addresses` that feeds `desiredState()`; no other source exists today.
- [ ] `internal/component/iface/backend.go` - `RegisterBackend`/`LoadBackend`/`GetBackend` (lines 216-282): the existing precedent for a package-level, mutex-guarded registry in this exact package.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `container loopback` (lines 1114-1119): "Always present; ze manages its addresses and units" — the loopback interface always exists, so a plugin registering against it never races interface creation.
- [ ] `internal/component/iface/manage_linux.go` - `netlinkBackend.AddAddress`/equivalent `RemoveAddress` (lines 204-220 and neighboring) — the actual kernel-application step reconciliation calls; unchanged by this spec.

**Behavior to preserve:**
- `desiredState()`'s existing output for any interface with no registered
  plugin-owned addresses is byte-for-byte identical to today (pure function of
  YANG config only, when the registry is empty for that interface).
- An address that is both YANG-config-declared and plugin-registered is added
  once and removed only when **both** sources stop declaring it (no double-add,
  no premature removal when only one source drops it).

**Behavior to change:**
- `desiredState()` gains a second input (the registry's current contents),
  merged with the YANG-derived map before the existing diff/reconcile logic
  runs. This is the only change to existing iface code.

## Data Flow (MANDATORY)

### Entry Point
- A plugin's `OnConfigure` handler (or equivalent config-apply-time callback)
  calling the new registration API when its service is enabled, and the
  unregister API when disabled.

### Transformation Path
1. Plugin calls `iface.RegisterOwnedAddresses(ifaceName, owner, addrs []string)` with its fixed CIDR list and a stable owner name.
2. The registry stores `owner → ifaceName → []string` under a package-level mutex, replacing any prior registration for that owner (idempotent re-registration).
3. **The registry fires a reconcile-trigger** (the `trigger func()` iface already wires through `subscribeReconcileOnReady`, `register.go:257`) so iface re-runs `reconcileOnReady` promptly — this is what makes the address land in the same enable operation instead of a later commit (finding B1).
4. `desiredState()` (`config_apply.go:18-110`) is extended to, for each interface, union the YANG-derived address set with every registered owner's address set for that interface before returning.
5. Reconciliation (`reconcileOnReadyWithJournal`, `config_apply.go:757-813`) runs unchanged against the merged set — it cannot tell a plugin-owned address from an operator-declared one, by design.
6. On `iface.UnregisterOwnedAddresses(owner)` (e.g. plugin disabled), the owner's entries are removed and the same reconcile-trigger fires, so the now-undesired kernel addresses are removed in the same disable operation (unless still present in YANG config from a different source).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ iface registry | direct Go function call (`internal/component/iface` exported API), since this is an in-process component, not a wire RPC | [ ] |
| Registry ↔ `desiredState()` | in-process merge inside the same package | [ ] |
| `desiredState()` ↔ kernel | existing netlink reconciliation, unchanged | [ ] |

### Integration Points
- `internal/component/iface/config_apply.go` `desiredState()` - merge point
- `internal/component/iface/backend.go` `RegisterBackend` pattern - structural precedent for the new registry's mutex/map shape
- `internal/component/iface/register.go:257` `subscribeReconcileOnReady(bus, trigger)` - the existing reconcile-trigger the registry reuses so registration re-runs reconciliation (finding B1)

### Architectural Verification
- [ ] No bypassed layers (registered addresses flow through the same reconciliation path as YANG-declared ones)
- [ ] No unintended coupling (iface does not import the as112 plugin or any plugin package; the plugin imports iface's small exported API)
- [ ] No duplicated functionality (this replaces the need for any plugin to invent its own address-management code)
- [ ] Registration over hardcoding — no `as112`-specific identifier anywhere in `internal/component/iface`

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `desiredState()` (returns `map[string]map[string]bool`, plus a managed-name set) can be extended to merge a second map without changing its existing call-site contract | Read of `config_apply.go:18-110` (desiredState) and `757-813` (reconcile; single call site at line 759) | Reconciliation call sites need wider changes than anticipated | unit test calling `desiredState()` with and without registry entries, asserting identical shape | unvalidated |
| A-2 | **RESOLVED (finding B1) — the ordering hazard is real, so registration is NOT relied upon to be "picked up later."** A plugin registering during its own config handler cannot assume iface will reconcile again in that commit: iface reconcile runs only from `applyConfig` (`config_apply.go:729`) + the vpp event handlers (`register.go:257`), nothing re-triggers it on registration, and plugin handler order is non-deterministic (`plan/learned/821-plugin-internal-keyword.md`). The design therefore makes `RegisterOwnedAddresses`/`Unregister` publish the reconcile-trigger themselves | Read of `config_apply.go:729`, `register.go:257`; agent verification of caller set | (was: one missed cycle) — now: address never lands until an unrelated commit | AC-7 + `TestRegister_TriggersReconcile` assert the trigger fires and the address reaches the kernel within the same op | design resolves it |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two different owners register the same address on the same interface (conflict) | unit test registering owner B for an address already owned by owner A | Registration is rejected with an error naming the conflicting owner; first-registrant wins, no silent overwrite |
| R-2 | A registered address is also present in YANG config, and the operator later removes it from config while the plugin is still enabled — must not flap (remove then re-add every cycle) | repeated add/remove churn visible in iface logs/metrics across consecutive reconciliations | merge is a set union computed fresh each cycle, so removal from one source while the other still claims it produces zero net change — covered by a dedicated unit test |
| R-3 | Out-of-process plugins (forked, not in-process) cannot call a Go-level API directly | as112 plugin running in `external`/forked mode per `plan/learned/821-plugin-internal-keyword.md` | This spec restricts itself to the in-process (`internal`) plugin deployment mode for as112 (mirrors how `geodns`'s `RunEngine` works via `net.Pipe` either way, but the *registration* call specifically requires being in the same address space) — recorded as a Known Limitation; out-of-process address registration is out of scope |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `iface.RegisterOwnedAddresses("lo", "test-owner", []string{"10.99.0.1/32"})` called, then config-apply reconciliation runs | → | `desiredState()` merge logic | `TestDesiredState_IncludesRegisteredOwnerAddresses` (`internal/component/iface/config_apply_test.go`) |
| `iface.UnregisterOwnedAddresses("test-owner")` called after registration, then reconciliation runs | → | `desiredState()` merge logic | `TestDesiredState_DropsAddressAfterUnregister` (`internal/component/iface/config_apply_test.go`) |
| `iface.RegisterOwnedAddresses(...)` called with NO subsequent config commit | → | reconcile-trigger fires → `reconcileOnReady` runs → address applied to kernel backend | `TestRegister_TriggersReconcile` (`internal/component/iface/address_owner_test.go`, fake backend records the `AddAddress` call) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers an address for an interface with no matching YANG config | `desiredState()` includes the address for that interface |
| AC-2 | Plugin unregisters a previously-registered address, and the address is not separately YANG-declared | `desiredState()` no longer includes it; next reconciliation removes it from the kernel |
| AC-3 | An address is both YANG-declared and plugin-registered | `desiredState()` includes it exactly once; unregistering the plugin's claim alone does not remove it (still YANG-declared) |
| AC-4 | A second owner registers an address already owned by a different owner on the same interface | Registration call returns an error naming the conflicting owner; the original registration is unchanged |
| AC-5 | A plugin re-registers the identical address set for the same owner | No error, no duplicate entries, idempotent |
| AC-6 | Concurrent goroutines register/unregister/read simultaneously | `go test -race` passes — no data race |
| AC-7 | `RegisterOwnedAddresses` (or `Unregister`) called with no following config commit | A reconcile-trigger fires and the address is applied to (or removed from) the kernel backend within the same operation — NOT deferred to a later unrelated commit (finding B1). Verified with a fake backend recording `AddAddress`/`RemoveAddress` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables a plugin that uses this registry (as112, spec-as112-2) | plugin `OnConfigure` → `RegisterOwnedAddresses` → `desiredState()` → kernel reconciliation | `TestDesiredState_IncludesRegisteredOwnerAddresses` plus spec-as112-2's end-to-end wiring test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterOwnedAddresses_Basic` | `internal/component/iface/address_owner_test.go` | registration appears in registry contents | |
| `TestRegisterOwnedAddresses_ConflictRejected` | `internal/component/iface/address_owner_test.go` | AC-4: conflicting owner rejected with named error | |
| `TestRegisterOwnedAddresses_Idempotent` | `internal/component/iface/address_owner_test.go` | AC-5: re-registration is a no-op, no duplicates | |
| `TestUnregisterOwnedAddresses` | `internal/component/iface/address_owner_test.go` | AC-2: entries removed after unregister | |
| `TestDesiredState_IncludesRegisteredOwnerAddresses` | `internal/component/iface/config_apply_test.go` | AC-1: merge into `desiredState()` | |
| `TestDesiredState_DropsAddressAfterUnregister` | `internal/component/iface/config_apply_test.go` | AC-2 end-to-end through `desiredState()` | |
| `TestDesiredState_YangAndRegistryOverlap` | `internal/component/iface/config_apply_test.go` | AC-3: dual-source address counted once, survives single-source removal | |
| `TestAddressOwnerRegistry_Race` (`-race`) | `internal/component/iface/address_owner_test.go` | AC-6: concurrent register/unregister/read | |
| `TestRegister_TriggersReconcile` | `internal/component/iface/address_owner_test.go` | AC-7 / B1: registration fires the reconcile-trigger and the fake backend sees `AddAddress`; unregister fires it and the backend sees `RemoveAddress` | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|----------------|
| N/A — no numeric inputs in this child; addresses are CIDR strings validated by existing `netlink.ParseAddr` at apply time, unchanged | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|--------------------|--------|
| `as112-address-registry` | `test/parse/as112-address-registry.ci` | enabling the as112 service (spec-as112-2) results in the four addresses appearing on `lo` without any operator-authored loopback config — exercised once spec-as112-2 exists; this child's own functional coverage is the unit tests above plus this end-to-end `.ci` owned jointly with spec-as112-2 | |

### Interop Tests
N/A — no wire protocol behavior in this child.

## Files to Modify
- `internal/component/iface/config_apply.go` - `desiredState()` (lines 18-110 area) merges registry contents before returning
- `internal/component/iface/register.go` - hand the registry the reconcile-trigger `func()` (the same one passed to `subscribeReconcileOnReady`, line 257) so `Register`/`Unregister` can re-run reconciliation (finding B1)

## Files to Create
- `internal/component/iface/address_owner.go` - the registry: `RegisterOwnedAddresses`, `UnregisterOwnedAddresses`, an internal accessor `desiredState()` uses, and a settable reconcile-trigger callback invoked on every mutation
- `internal/component/iface/address_owner_test.go` - unit tests above

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No — this is a Go-level API, no new config surface | n/a |
| YANG validation constraints | [ ] No | n/a |
| YANG custom validators | [ ] No | n/a |
| CLI commands/flags | [ ] No — not directly operator-facing; visible indirectly via `show interface` once addresses are present | n/a |
| CLI grammar | [ ] No | n/a |
| Editor autocomplete | [ ] No | n/a |
| Functional test for new RPC/API | [x] Yes (joint with spec-as112-2) | `test/parse/as112-address-registry.ci` |
| Pipe completeness | [ ] No — no new command output | n/a |
| Env var registration | [ ] No | n/a |
| Doctor check for runtime dependencies | [ ] No — no new runtime dependency (uses existing netlink path); spec-as112-2 owns the as112-specific doctor check | n/a |
| Prometheus counters/metrics | [ ] No — registry has no independently observable state beyond what `show interface` already exposes once addresses land | n/a |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|-----------------|
| 1 | New user-facing feature? | [ ] No — internal mechanism, not directly operator-facing | n/a |
| 2 | Config syntax changed? | [ ] No | n/a |
| 3 | CLI command added/changed? | [ ] No | n/a |
| 4 | API/RPC added/changed? | [x] Yes — new Go-level API in iface, internal only | noted in `docs/architecture/core-design.md`'s component list if it already enumerates iface's exported surface; otherwise N/A (grep shows no such enumeration today) |
| 5 | Plugin added/changed? | [ ] No | n/a |
| 6 | Has a user guide page? | [ ] No — covered by `docs/guide/as112.md` (spec-as112-0) from the consumer side | n/a |
| 7 | Wire format changed? | [ ] No | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] No — this is a component-internal API, not the plugin wire SDK | n/a |
| 9 | RFC behavior implemented? | [ ] No | n/a |
| 10 | Test infrastructure changed? | [ ] No | n/a |
| 11 | Affects daemon comparison? | [ ] No | n/a |
| 12 | Internal architecture changed? | [x] Yes — iface gains a new merge input | `docs/architecture/core-design.md` if it documents `desiredState()`'s inputs (grep first; update only if a stale claim exists) |
| 13 | Route metadata keys added/changed? | [ ] No | n/a |
| 14 | Prometheus counters added/changed? | [ ] No | n/a |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] No | n/a |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] To verify — grep `docs/` for `config_apply.go` before implementing | n/a unless grep hits |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] No | n/a |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review / fix / re-verify | Critical Review Checklist below |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — create `address_owner.go` with `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` and an empty/no-op merge point in `desiredState()`; write `TestDesiredState_IncludesRegisteredOwnerAddresses` as a failing wiring test.
   - Tests: `TestDesiredState_IncludesRegisteredOwnerAddresses`
   - Files: `internal/component/iface/address_owner.go`, `internal/component/iface/config_apply.go`
   - Verify: test fails because the merge is a stub; registration API is reachable
2. **Phase: Registry core** — implement the mutex-guarded map, conflict rejection, idempotent re-registration, unregistration.
   - Tests: `TestRegisterOwnedAddresses_Basic`, `TestRegisterOwnedAddresses_ConflictRejected`, `TestRegisterOwnedAddresses_Idempotent`, `TestUnregisterOwnedAddresses`, `TestAddressOwnerRegistry_Race`
   - Files: `internal/component/iface/address_owner.go`, `internal/component/iface/address_owner_test.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: desiredState() merge** — wire the registry into `desiredState()`'s output.
   - Tests: `TestDesiredState_IncludesRegisteredOwnerAddresses`, `TestDesiredState_DropsAddressAfterUnregister`, `TestDesiredState_YangAndRegistryOverlap`
   - Files: `internal/component/iface/config_apply.go`, `internal/component/iface/config_apply_test.go`
   - Verify: tests fail → implement → tests pass → wiring test passes
4. **Phase: reconcile-trigger (finding B1)** — give the registry a settable trigger `func()`; wire it in `register.go` to the same trigger used by `subscribeReconcileOnReady` (line 257); fire it on every Register/Unregister.
   - Tests: `TestRegister_TriggersReconcile`
   - Files: `internal/component/iface/address_owner.go`, `internal/component/iface/register.go`
   - Verify: test fails (no trigger) → implement → fake backend sees `AddAddress`/`RemoveAddress` without a config commit → AC-7 passes
5. **Functional tests** → joint `.ci` test deferred to spec-as112-2 (needs a real plugin consumer to exercise end-to-end).
6. **Full verification** → `make ze-verify`
7. **Complete spec** → fill audit tables, write learned summary, two-commit close per `ai/rules/planning.md`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-7 each have file:line implementation |
| Reconcile-trigger (B1) | Register/Unregister actually re-run reconciliation without a config commit; `TestRegister_TriggersReconcile` proves the kernel backend is touched |
| Correctness | Conflict detection actually compares CIDR strings correctly (case/format normalization matches existing `netlink.ParseAddr` behavior) |
| Naming | Exported function/type names match Go conventions used elsewhere in `internal/component/iface` (cf. `RegisterBackend`) |
| Data flow | `desiredState()` remains a pure function of (YANG config, registry snapshot) — no hidden global state beyond the registry itself |
| Registration over hardcoding | No `as112` string anywhere in `internal/component/iface` |
| Rule: no-layering | iface does not import any plugin package |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/iface/address_owner.go` exists | `ls -la internal/component/iface/address_owner.go` |
| All unit tests pass | `go test ./internal/component/iface/... -run TestRegisterOwnedAddresses -run TestDesiredState -run TestUnregister -race` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-------------------|
| Input validation | Registered CIDR strings are validated the same way as YANG-declared ones before being handed to `AddAddress` (no bypass of existing `netlink.ParseAddr` validation) |
| Resource exhaustion | An unbounded number of distinct owners/addresses cannot be registered by an external, untrusted actor — only in-process plugin code (not network input) can call this API |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read `config_apply.go` from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; coordinate with spec-as112-2 if the joint `.ci` test is involved |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- **L2 — generic registry with one consumer, on purpose.** The Design checklist
  asks "No premature abstraction (3+ use cases?)" and this registry has exactly
  one consumer today (as112). This is a deliberate choice of the
  registration-over-hardcoding pattern (`ai/patterns/registration.md`) over the
  3-use-case heuristic: the alternative is an `if owner == "as112"` special case
  inside `internal/component/iface`, which `ai/rules/plugin-self-containment.md`
  forbids. The registry stays plugin-name-agnostic; the cost of genericity here
  is ~one map and one mutex, not a speculative framework.
- **Direct Go call is well-precedented (corrects an earlier worry).** Plugins
  already import `internal/component/iface` directly in ~72 places (e.g.
  `iface.AddrPayload` in `internal/plugins/flowspec-firewall/localaddr.go`,
  `internal/plugins/ospf/interface_addr.go`). An in-process plugin calling a new
  package-level `iface` function is normal and allowed by module-tiers
  (plugins → components). Config is *delivered* to the plugin over the SDK
  `net.Pipe`, but the plugin's in-process engine goroutine is free to call
  component functions — the pipe is not a sandbox.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|--------------------------|-----------|
| Generic mutex-guarded registry mirroring `backend.go`'s `RegisterBackend` pattern | (a) reuse the existing `ze-iface:interface-addr-add` RPC at plugin startup | (a) is imperative-only and gets reconciled away on the next config-apply cycle (`config_apply.go:757-813`); a registry consulted inside `desiredState()` is the only option that survives reconciliation |
| Registry mutation fires a reconcile-trigger (finding B1) | Rely on the next config commit to pick up the registration | Nothing re-runs iface reconcile on registration (`applyConfig` + vpp events only), and plugin handler order is non-deterministic, so relying on ordering leaves `lo` unconfigured until an unrelated commit. Reusing the `subscribeReconcileOnReady` trigger (`register.go:257`) makes enable/disable atomic and needs no new machinery |
| Restrict to in-process plugins only (R-3) | Support out-of-process (forked) plugins too via a wire RPC | An out-of-process (forked) plugin runs in a separate OS process and cannot call a package-level Go function in the main binary; a wire-RPC equivalent is a larger, separate piece of work out of proportion to this feature, and as112 is deployed in-process |

## Known Limitations
- Only in-process (`internal` per `plan/learned/821-plugin-internal-keyword.md`) plugins can use this registry, since registration is a direct Go function call, not a wire RPC. Out-of-process plugin support is not implemented.
- The registry is loopback-agnostic in implementation (keyed by arbitrary `ifaceName`) but only exercised against `lo` in this spec set.

## RFC Documentation
N/A — this child implements no RFC-mandated wire behavior.

## Implementation Summary
### What Was Implemented
- [Filled at closure]

### Bugs Found/Fixed
- [Filled at closure]

### Documentation Updates
- [Filled at closure]

### Deviations from Plan
- [Filled at closure]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-------------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|---------------------------|----------------|----------------------|
| Enabling a plugin's address ownership makes its addresses appear without operator-duplicated config | functional test | `test/parse/as112-address-registry.ci` |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [Filled during implementation]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|------------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|---------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|----------------------------------|------------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-as112-1-iface-address-registry.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-as112-1-iface-address-registry.md` only
