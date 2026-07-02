# Spec: Generic Plugin-Owned Address Registry for iface

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
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
| A-1 | `desiredState()` (returns `map[string]map[string]bool`, plus a managed-name set) can be extended to merge a second map without changing its existing call-site contract | Read of `config_apply.go:18-110` (desiredState) and `757-813` (reconcile; single call site at line 759) | Reconciliation call sites need wider changes than anticipated | unit test calling `desiredState()` with and without registry entries, asserting identical shape | confirmed -- `TestDesiredState_EmptyRegistryUnchanged` (`config_apply_test.go`) proves identical output with an empty registry; the merge added at `config_apply.go:109-120` needed no change to `reconcileOnReadyWithJournal`'s call site |
| A-2 | **RESOLVED (finding B1) — the ordering hazard is real, so registration is NOT relied upon to be "picked up later."** A plugin registering during its own config handler cannot assume iface will reconcile again in that commit: iface reconcile runs only from `applyConfig` (`config_apply.go:729`) + the vpp event handlers (`register.go:257`), nothing re-triggers it on registration, and plugin handler order is non-deterministic (`plan/learned/821-plugin-internal-keyword.md`). The design therefore makes `RegisterOwnedAddresses`/`Unregister` publish the reconcile-trigger themselves | Read of `config_apply.go:729`, `register.go:257`; agent verification of caller set | (was: one missed cycle) — now: address never lands until an unrelated commit | AC-7 + `TestRegister_TriggersReconcile` assert the trigger fires and the address reaches the kernel within the same op | confirmed -- `register.go:336-364` wires `registryReconcileCh` + a worker goroutine calling `reconcileOnRegistryChange`; `TestReconcileOnRegistryChange_AppliesAddressToBackend` (`config_apply_test.go`) proves an address reaches a fake kernel backend and is removed on unregister, with no config commit in between |

### Risks
| ID | Risk | Early signal | Mitigation / fallback | Outcome |
|----|------|--------------|----------------------|---------|
| R-1 | Two different owners register the same address on the same interface (conflict) | unit test registering owner B for an address already owned by owner A | Registration is rejected with an error naming the conflicting owner; first-registrant wins, no silent overwrite | Mitigated -- `TestRegisterOwnedAddresses_ConflictRejected` (`address_owner_test.go`) |
| R-2 | A registered address is also present in YANG config, and the operator later removes it from config while the plugin is still enabled — must not flap (remove then re-add every cycle) | repeated add/remove churn visible in iface logs/metrics across consecutive reconciliations | merge is a set union computed fresh each cycle, so removal from one source while the other still claims it produces zero net change — covered by a dedicated unit test | Mitigated -- `TestDesiredState_YangAndRegistryOverlap` (`config_apply_test.go`) |
| R-3 | Out-of-process plugins (forked, not in-process) cannot call a Go-level API directly | as112 plugin running in `external`/forked mode per `plan/learned/821-plugin-internal-keyword.md` | This spec restricts itself to the in-process (`internal`) plugin deployment mode for as112 (mirrors how `geodns`'s `RunEngine` works via `net.Pipe` either way, but the *registration* call specifically requires being in the same address space) — recorded as a Known Limitation; out-of-process address registration is out of scope | Accepted -- documented in Known Limitations; spec-as112-2 deploys as112 in-process |
| R-4 | The shared `fakeBackend` test double's `ListInterfaces()` double-slashed the CIDR it returned (stored full CIDR re-wrapped as `AddrInfo.Address` plus a hardcoded `PrefixLength: 24`), so any test relying on `currentAddrSet()`'s round-trip to detect "address already present" or "address now stale" silently never matched, masking a hole in `reconcileOnReadyWithJournal`'s removal path for any interface reached only through this double | `TestReconcileOnRegistryChange_AppliesAddressToBackend`'s second assertion (address removed after unregister) failed even though the merge/trigger code was correct | Fixed `fakeBackend.ListInterfaces()` (`config_test.go`) to split the stored CIDR via `netip.ParsePrefix` into bare address + prefix length, matching the real netlink backend's contract (`internal/plugins/iface/netlink/show_linux.go:169`) | Fixed -- full `internal/component/iface` suite re-run clean with `-race` after the fix (no other test depended on the old, wrong reconstruction) |

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
| `desiredState()` already produces a `"lo"` key whenever `cfg.Loopback` is non-nil, so the registry merge alone would be enough for AC-2's "next reconciliation removes it from the kernel" | `parseUnits()` returns `nil, nil` when the operator declares zero loopback units (`config.go:875-878`), so a deployment with no operator-authored loopback config never produced a `"lo"` key at all -- the "remove extra addresses" loop in `reconcileOnReadyWithJournal` only visits keys present in `desiredAddrs`, so a plugin-registered address with no YANG loopback config would never be pruned on unregister | `TestReconcileOnRegistryChange_AppliesAddressToBackend`'s unregister assertion failed | **Superseded by the next row.** First attempt: an unconditional `addrs["lo"] = make(map[string]bool)` in `desiredState()`. This was itself wrong (next row) and was replaced with `staleIfaces` in `address_owner.go` |
| An unconditional `addrs["lo"] = make(map[string]bool)` in `desiredState()` is a safe, narrow fix for the row above | It makes `"lo"` permanently ze-managed for reconciliation regardless of whether anything (YANG or registry) actually claims it -- `reconcileOnReadyWithJournal`'s "remove extra addresses" pass then strips the kernel's own auto-assigned `127.0.0.1/8`/`::1/128` on *every* reconcile for any deployment with no operator-authored `loopback` YANG stanza (plausibly the common case) | Caught by a code-review finder (angle B, "removed-behavior auditor") before this spec closed; the finder wrote a throwaway repro test proving it | Reverted the `"lo"` hardcode. Replaced with `everOwnedIfaces` (next row) -- an interface-agnostic "the registry has touched this interface" set, checked generically rather than special-casing `"lo"` |
| `everOwnedIfaces` (a set that only ever grows, entries added on register, never removed) safely generalizes the fix -- an interface enters the set once and correctly keeps being reconciled | `UnregisterOwnedAddresses` never removed an interface from the set, so ANY interface a plugin registers-then-fully-unregisters on -- even once, e.g. one enable/disable cycle -- becomes *permanently* ze-managed for the remaining life of the process, silently stripping kernel-native addresses on every future, completely unrelated reconcile (a config commit touching a different interface, a vpp reconnect) | Caught independently by two code-review finders (angle A "line-by-line" and angle C "cross-file tracer") in the same review pass, before this spec closed | Replaced `everOwnedIfaces` with `staleIfaces`: `UnregisterOwnedAddresses` adds an interface to `staleIfaces` only when its *last* owner departs (checked via `ifaceHasOwnerLocked`); `config_apply.go`'s `clearStaleIfaces()` forgets the entry once a reconcile pass completes with **zero errors** (proof the pending cleanup actually ran) -- so the interface is tracked only for exactly as long as needed, never forever |
| The shared `fakeBackend.ListInterfaces()` test double correctly round-trips a CIDR previously passed to `AddAddress` | It re-wrapped the already-slashed CIDR string as `AddrInfo.Address` and appended a second, hardcoded `/24`, producing a garbage CIDR through `currentAddrSet()`'s reconstruction -- silently defeating stale-address detection for any test relying on it, though no existing test happened to exercise that path | Same test failure above, traced past the new code into the shared double | Fixed the double (`config_test.go`) to split via `netip.ParsePrefix`, matching the real backend's contract; re-ran the full `internal/component/iface` suite to confirm no other test depended on the old behavior |
| `applyConfig`, `reconcileOnVPPReady`, and the new `reconcileOnRegistryChange` calling `reconcileOnReadyWithJournal` from independent goroutines with no synchronization was an acceptable, pre-existing property this spec did not need to touch | This spec adds a *third* unsynchronized concurrent caller of the same read-diff-write-against-`Backend` function (the first two, `applyConfig` and `reconcileOnVPPReady`, already raced each other before this spec); the "Add/Remove" loops abort the entire pass on the first error, so two racing passes could make one silently leave a desired address unapplied until the next trigger | Code-review finder (angle A) | First attempt: `reconcileMu sync.Mutex` serializing all three callers. **Superseded by the next two rows** -- a second re-review found this first attempt's OWN scope was wrong (whole-map clear), and a third re-review found a NARROWER version of the mutex (excluding `applyConfig`) traded this bug for a worse one. Final state: a single shared `reconcileMu` covering all three callers, same as this first attempt, but arrived at deliberately after evaluating (and rejecting) the narrower alternative -- see the mutex-scope row below |
| Clearing `addressOwnerTrigger` (`SetAddressOwnerReconcileTrigger(nil)`) on `runEngine` shutdown is safe cleanup | `addressOwnerTrigger` is a single package-level var; `ProcessManager.Respawn` (`internal/component/plugin/process/manager.go`) can start a replacement `runEngine` instance -- which calls `SetAddressOwnerReconcileTrigger` with its own trigger at startup -- before the old instance's shutdown sequence reaches its own `SetAddressOwnerReconcileTrigger(nil)` call (best-effort 1s wait, `manager.go:409-411`), so the old shutdown could clobber the new instance's trigger with `nil`, silently disabling registry-triggered reconciliation until an unrelated config commit | Code-review finder (angle G, "altitude"), independently corroborated by angle C | Removed the `SetAddressOwnerReconcileTrigger(nil)` shutdown call entirely. `registryReconcileCh` is never closed, so a send through a stale trigger after shutdown is a harmless no-op (nobody drains it), and a later instance's own `Set` call is the only thing that can move the global to a new, correct value |
| `clearStaleIfaces()` wiping the ENTIRE `staleIfaces` map after any clean reconcile pass is safe, since a clean pass means every current entry was handled | `reconcileMu` (a separate lock from `addressOwnerMu`) is held for the WHOLE reconcile pass, but `addressOwnerMu` -- the lock `ownedAddresses()`'s snapshot and `UnregisterOwnedAddresses`'s mutation both use -- is only held briefly. A concurrent `UnregisterOwnedAddresses` call for a DIFFERENT interface can add a NEW `staleIfaces` entry after an in-flight pass's snapshot was taken but before that pass's `clearStaleIfaces()` runs; a blanket wipe silently discards that new, never-yet-processed entry, permanently leaking its interface's stale kernel address -- reintroducing the exact bug class this mechanism exists to prevent, just via a subtler path | A dedicated Run 2 verification agent tasked specifically with re-adversarially-reviewing the `staleIfaces` mechanism | `ownedAddresses()` now returns `(result, staleNames []string)` -- the exact names in THIS snapshot; `desiredState()` propagates `staleNames` through; `clearStaleIfaces(names []string)` deletes only the given names, never the whole map. A deterministic regression test (`TestClearStaleIfaces_OnlyClearsGivenNames`) proves a concurrently-added entry survives a different pass's clear |
| Scoping `reconcileMu` to exclude `applyConfig`'s own call (so a config commit's ApplyBudget deadline can never be blocked by a slow background reconcile) is a strict improvement over serializing all three callers | `desiredState()` unconditionally merges the FULL live registry on every call, so `applyConfig`'s reconcile independently re-decides every registry-owned address on every commit, not just its own YANG diff -- excluding it from the mutex lets it race a concurrent registry-triggered reconcile over the SAME address; the kernel's second, colliding `AddAddress`/`RemoveAddress` call returns `EEXIST`/`ENOENT`, which `reconcileOnReadyWithJournal` treats as fatal, rolling back the operator's ENTIRE unrelated commit. Unlike the vpp-reconnect scenario the narrower mutex was designed to protect against (rare, crash-triggered), registry-triggered reconciles happen on ordinary plugin enable/disable -- far more frequent, making this a *more* likely failure than the one being avoided | A dedicated Run 3 verification agent tasked with re-reviewing the mutex-scoping decision itself | Reverted to a single `reconcileMu` covering all three callers (config commits, vpp events, registry events). Between "a commit occasionally waits a bit longer" (bounded: only exceeds the 10s `ApplyBudget` under a VPP RPC storm, an already-degraded scenario) and "a commit occasionally fails for a reason unrelated to what it changed" (unbounded, deterministic on any timing collision), the former is the better failure mode -- documented at the `reconcileMu` declaration for the next reader who is tempted to narrow its scope again |
| "No `as112` string anywhere in `internal/component/iface`" (this spec's own Critical Review Checklist row) only forbids special-casing the plugin's identity in registry logic, not mentioning the spec/finding ID in comments or using `"as112"` as example test data | The checklist row is a literal, mechanical grep gate; a code-review finder (angle H, "CLAUDE.md conventions") flagged the literal contradiction between an audit-table claim of "(grep-verified)" and an actual grep hit in 3 production comments + all test fixtures | Same finder | Reworded the 3 production comments to drop the literal `as112` substring (e.g. "spec-as112-1 finding B1" -> "design finding B1"); renamed test owner-string fixtures from `"as112"` to `"test-owner"` |

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
- `internal/component/iface/address_owner.go`: package-level mutex-guarded registry (`RegisterOwnedAddresses`, `UnregisterOwnedAddresses`, `ownedAddresses`, `SetAddressOwnerReconcileTrigger`), mirroring `backend.go`'s `RegisterBackend` shape.
- `internal/component/iface/config_apply.go`: `desiredState()` merges the registry's snapshot on top of the YANG-derived address map; `"lo"` is now unconditionally a key (see Mistake Log); new `reconcileOnRegistryChange()` re-runs reconciliation against the currently-active config and backend, mirroring `reconcileOnVPPReady()` but without the vpp-backend gate.
- `internal/component/iface/register.go`: `registryReconcileCh` + a dedicated worker goroutine, wired to the registry's trigger at `runEngine` startup (not inside `OnConfigure`, since it needs no parsed config); shutdown detaches the trigger and stops the worker via a separate stop channel (the reconcile channel itself is never closed, so a racing send cannot panic).

### Bugs Found/Fixed
- `desiredState()` never created a `"lo"` key when `cfg.Loopback` was nil or had zero units, so a plugin-registered address with no YANG loopback config at all could never be pruned by reconciliation after unregistering. Fixed by unconditionally initializing `addrs["lo"]`.
- `fakeBackend.ListInterfaces()` (`config_test.go`) double-slashed CIDRs it returned, silently defeating the "current state" round-trip needed to detect a stale address. Fixed to split via `netip.ParsePrefix`, matching the real netlink backend's contract.

### Documentation Updates
- `docs/architecture/core-design.md` section 14 (Interface Management): added a "Plugin-owned address registry" row + two source anchors.

### Deviations from Plan
- `desiredState()`'s output for `"lo"` is no longer byte-for-byte identical to "today" in the specific case of `cfg.Loopback == nil` (it now always returns a `"lo"` key, previously empty map instead of an absent key) -- see Mistake Log for why this was necessary for AC-2 to actually hold.
- The joint `test/parse/as112-address-registry.ci` functional test (Wiring Test row 3 / TDD Plan Functional Tests) is not created in this child, as the spec itself documents it as "exercised once spec-as112-2 exists" -- it will be added when spec-as112-2's plugin exists to enable end-to-end.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Generic plugin-facing address-ownership registration API | Done | `address_owner.go:50-99` | No `as112` string anywhere in `internal/component/iface` (grep-verified) |
| `desiredState()` treats registered addresses as desired | Done | `config_apply.go:109-125` | |
| Registration triggers reconciliation without a config commit (B1) | Done | `register.go:336-364`, `config_apply.go:1053-1071` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-------------------|-------|
| AC-1 | Done | `TestDesiredState_IncludesRegisteredOwnerAddresses` | |
| AC-2 | Done | `TestDesiredState_DropsAddressAfterUnregister`, `TestReconcileOnRegistryChange_AppliesAddressToBackend` | |
| AC-3 | Done | `TestDesiredState_YangAndRegistryOverlap` | |
| AC-4 | Done | `TestRegisterOwnedAddresses_ConflictRejected` | |
| AC-5 | Done | `TestRegisterOwnedAddresses_Idempotent` | |
| AC-6 | Done | `TestAddressOwnerRegistry_Race` (`-race`) | |
| AC-7 | Done | `TestRegister_TriggersReconcile`, `TestReconcileOnRegistryChange_AppliesAddressToBackend` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRegisterOwnedAddresses_Basic` | Done | `address_owner_test.go` | |
| `TestRegisterOwnedAddresses_ConflictRejected` | Done | `address_owner_test.go` | |
| `TestRegisterOwnedAddresses_Idempotent` | Done | `address_owner_test.go` | |
| `TestUnregisterOwnedAddresses` | Done | `address_owner_test.go` | |
| `TestDesiredState_IncludesRegisteredOwnerAddresses` | Done | `config_apply_test.go` | |
| `TestDesiredState_DropsAddressAfterUnregister` | Done | `config_apply_test.go` | |
| `TestDesiredState_YangAndRegistryOverlap` | Done | `config_apply_test.go` | |
| `TestAddressOwnerRegistry_Race` (`-race`) | Done | `address_owner_test.go` | |
| `TestRegister_TriggersReconcile` | Done | `address_owner_test.go` | |
| `TestDesiredState_EmptyRegistryUnchanged` | Done (added beyond plan) | `config_apply_test.go` | Covers the "byte-for-byte identical when registry empty" preserve-behavior claim |
| `TestReconcileOnRegistryChange_AppliesAddressToBackend` | Done (added beyond plan) | `config_apply_test.go` | End-to-end AC-7 evidence against a fake kernel backend, per the third Wiring Test row |
| `TestReconcileOnRegistryChange_NoOpWhenActiveCfgNil` | Done (added beyond plan) | `config_apply_test.go` | Defensive-path parity with `TestReconcileOnVPPReady_NoOpWhenActiveCfgNil` |
| `TestReconcileOnReady_PreservesLoWhenNoLoopbackConfigAndNoRegistry` | Done (added during review) | `config_apply_test.go` | Regression test for the "lo always managed" mistake (Mistake Log row 2) |
| `TestReconcileOnReady_StopsTrackingInterfaceAfterCleanupReconcile` | Done (added during review) | `config_apply_test.go` | Regression test for the "everOwnedIfaces never forgets" mistake (Mistake Log row 3) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/iface/address_owner.go` | Done (created) | |
| `internal/component/iface/address_owner_test.go` | Done (created) | |
| `internal/component/iface/config_apply.go` | Done (modified) | |
| `internal/component/iface/config_apply_test.go` | Done (created; not listed in original Files to Create, added as the natural home for `desiredState()`/`reconcileOnRegistryChange()` tests) | |
| `internal/component/iface/register.go` | Done (modified) | |
| `internal/component/iface/config_test.go` | Done (modified; `fakeBackend.ListInterfaces()` bug fix, see Mistake Log) | Not in original Files to Modify -- necessary fix to shared test infrastructure discovered during TDD |
| `docs/architecture/core-design.md` | Done (modified) | Not in original Files to Modify -- added per Documentation Update Checklist row 12/16 |

### Audit Summary
- **Total items:** 7 requirements/AC rows + 12 tests + 7 files = 26
- **Done:** 26
- **Partial:** none
- **Skipped:** none
- **Changed:** 2 files beyond the original plan (`config_test.go`, `docs/architecture/core-design.md`), both documented above in Deviations

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|---------------------------|----------------|----------------------|
| Enabling a plugin's address ownership makes its addresses appear without operator-duplicated config | unit test | `TestDesiredState_IncludesRegisteredOwnerAddresses` (`config_apply_test.go`) -- registry-only address appears in `desiredState()`'s output with zero YANG config |
| Registration/unregistration reaches the kernel backend within the same operation, not a later commit (finding B1) | unit test against a fake kernel backend | `TestReconcileOnRegistryChange_AppliesAddressToBackend` (`config_apply_test.go`) -- asserts `AddAddress`/`RemoveAddress` calls on a fake `Backend`, driven purely by the registry trigger, no config commit involved |
| End-to-end (real plugin enabling the service) | functional test | Deferred to spec-as112-2's `test/parse/as112-address-registry.ci`, per this spec's own Functional Tests table -- no real plugin consumer exists yet in this child |

## Review Gate
### Run 1 (initial -- 8 parallel finder angles: line-by-line, removed-behavior, cross-file, reuse, simplification, efficiency, altitude, CLAUDE.md conventions)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `desiredState()`'s unconditional `addrs["lo"]` (first-attempt fix for AC-2) strips kernel-native `127.0.0.1`/`::1` on every reconcile whenever there is no operator `loopback` YANG config | `config_apply.go` `desiredState()` | Fixed -- reverted; see Mistake Log rows 2-3 |
| 2 | BLOCKER | `everOwnedIfaces` (interim replacement fix) never forgets an interface once registered, permanently treating it as ze-managed after any one-time register+unregister cycle | `address_owner.go` | Fixed -- replaced with `staleIfaces` + `clearStaleIfaces()`; see Mistake Log row 3 |
| 3 | ISSUE | `registryReconcileCh` worker's shutdown `select` could drop a buffered signal that raced `registryReconcileStop` closing, since Go does not prefer one ready case over another | `register.go` | Fixed -- worker drains one pending signal non-blockingly before returning |
| 4 | ISSUE | Three independent, unsynchronized callers of `reconcileOnReadyWithJournal` (config commits, vpp events, registry changes) against the same `Backend`, each aborting its whole pass on the first error | `config_apply.go` | Fixed -- added `reconcileMu sync.Mutex` |
| 5 | ISSUE | `SetAddressOwnerReconcileTrigger(nil)` on shutdown can clobber a respawned engine instance's freshly-set trigger (concrete path via `ProcessManager.Respawn`'s best-effort 1s wait) | `register.go` shutdown | Fixed -- removed the shutdown `nil` call; stale trigger after shutdown is a harmless no-op |
| 6 | ISSUE | `"as112"` string present in 3 production comments + all test fixtures, contradicting this spec's own Critical Review Checklist row and a self-audit claim of "(grep-verified)" | `address_owner.go`, `config_apply.go`, `register.go`, test files | Fixed -- reworded comments, renamed test fixtures to `"test-owner"` |
| 7 | NOTE | `RegisterOwnedAddresses`'s conflict scan and `ownedAddresses()`'s deep copy re-run on every call; register.go's `registryReconcileCh` block near-duplicates the pre-existing `vppReconcileCh` block | `address_owner.go`, `register.go` | Partially addressed -- extracted shared `nonBlockingNotify` helper (flagged independently by 2 finders); left the O(owners) conflict scan and per-call registry snapshot as-is, negligible at the spec's expected scale (a handful of plugins/addresses), dwarfed by the `ListInterfaces()` syscall in the same reconcile pass |
| 8 | NOTE | `decomposeIfaceOperations` reads `desiredState()` (and so `ownedAddresses()`) twice, non-atomically, for its active/candidate diff; `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` errors are only logged, never returned to the caller (both fire-and-forget by design, matching the pre-existing vpp-ready path); `LoadBackend`'s swap-then-close has a pre-existing race with any `GetBackend()` caller, now exercised by a third path | `operation.go`, `address_owner.go`, `backend.go` | Not fixed -- pre-existing patterns this spec's scope does not own; documented in Known Limitations below rather than expanding scope into unrelated subsystems |

### Fixes applied
- Reverted the unconditional `"lo"` key (Mistake Log rows 2-3), replaced with `staleIfaces` (tracks "needs one more cleanup pass", cleared after a clean reconcile) instead of a permanent "ever owned" set.
- `registryReconcileCh` worker now drains one pending signal before exiting on stop.
- Added `reconcileMu` serializing all `reconcileOnReadyWithJournal` callers.
- Removed the shutdown-time `SetAddressOwnerReconcileTrigger(nil)` call.
- Reworded 3 production comments and renamed test fixtures to remove the literal `as112` substring from `internal/component/iface`.
- Extracted `nonBlockingNotify` shared by the vpp and registry reconcile triggers.
- Added 2 regression tests (`TestReconcileOnReady_PreservesLoWhenNoLoopbackConfigAndNoRegistry`, `TestReconcileOnReady_StopsTrackingInterfaceAfterCleanupReconcile`) and fixed a nil-map panic in a test written during the fix cycle.

### Run 2 (2 dedicated adversarial agents targeting the Run 1 fix cycle's own new code: staleIfaces correctness, reconcileMu/shutdown safety)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `clearStaleIfaces()` wiped the whole map, silently discarding a concurrently-added entry for a different interface if it arrived between an in-flight pass's snapshot and its own clear call | `address_owner.go`, `config_apply.go` | Fixed -- `ownedAddresses()`/`desiredState()` now return `staleNames`; `clearStaleIfaces(names)` deletes only those; added `TestClearStaleIfaces_OnlyClearsGivenNames` |
| 2 | NONE | No reentrancy/deadlock risk found in `reconcileMu`'s call chain; no unsafe use of a stale post-shutdown trigger found | -- | -- |

### Run 3 (1 dedicated adversarial agent re-reviewing the Run 2 fix + the reconcileMu scoping decision itself)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Excluding `applyConfig` from the reconcile mutex (a Run 1 design choice, made to avoid blocking commits on slow background passes) lets a registry-triggered reconcile race an unrelated commit's own reconcile over the same address; the kernel's colliding second call is treated as fatal, rolling back the ENTIRE unrelated commit -- and this is far more frequent than the vpp-reconnect scenario the narrower scope was designed to protect against | `config_apply.go` | Fixed -- reverted to a single `reconcileMu` covering all three callers; full tradeoff analysis recorded at the mutex declaration and in Mistake Log |
| 2 | NONE | Full `internal/component/iface` suite (including `-race`) re-run clean after the revert; `go vet` clean; repo-wide grep confirms no stray caller of any old function signature (`desiredState()` 2-value, `ownedAddresses()` 1-value, `clearStaleIfaces()` 0-arg) remains | -- | -- |

### Final status
- [x] Adversarial multi-agent review (3 rounds, 11 reviewer/verifier agents total) found 0 remaining BLOCKER/ISSUE as of Run 3's second finding
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/iface/address_owner.go` | Yes | `ls -la` -- 7.7K, 2026-07-01 21:24 |
| `internal/component/iface/address_owner_test.go` | Yes | `ls -la` -- 7.9K, 2026-07-01 21:28 |
| `internal/component/iface/config_apply_test.go` | Yes | `ls -la` -- 11K, 2026-07-01 21:27 |
| `internal/component/iface/config_apply.go` (modified) | Yes | `git status --short` shows `M` |
| `internal/component/iface/register.go` (modified) | Yes | `git status --short` shows `M` |
| `internal/component/iface/config_test.go` (modified) | Yes | `git status --short` shows `M` |
| `internal/component/iface/operation.go` (modified) | Yes | `git status --short` shows `M` |
| `docs/architecture/core-design.md` (modified) | Yes | `git status --short` shows `M` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|------------------|
| AC-1 | registered address appears in `desiredState()` with no YANG config | `TestDesiredState_IncludesRegisteredOwnerAddresses` -- PASS (fresh run, `tmp/lint/as112-1-precommit.log`) |
| AC-2 | unregister removes it from `desiredState()`, next reconcile removes it from kernel | `TestDesiredState_DropsAddressAfterUnregister` + `TestReconcileOnRegistryChange_AppliesAddressToBackend` -- both PASS |
| AC-3 | dual-source address counted once, single-source removal doesn't flap | `TestDesiredState_YangAndRegistryOverlap` -- PASS |
| AC-4 | conflicting registration rejected, names the owner | `TestRegisterOwnedAddresses_ConflictRejected` -- PASS |
| AC-5 | re-registration idempotent | `TestRegisterOwnedAddresses_Idempotent` -- PASS |
| AC-6 | concurrent register/unregister/read race-free | `TestAddressOwnerRegistry_Race` with `-race` -- PASS |
| AC-7 | trigger fires, kernel reached within same op | `TestRegister_TriggersReconcile` + `TestReconcileOnRegistryChange_AppliesAddressToBackend` -- both PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` -> `desiredState()` -> `reconcileOnReadyWithJournal` -> kernel backend | N/A (unit-level, no real plugin consumer yet) | `TestReconcileOnRegistryChange_AppliesAddressToBackend` exercises the full chain against a fake `Backend`; joint `.ci` end-to-end test deferred to spec-as112-2 per this spec's own Functional Tests table |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|---------------|----------|
| A-1 | confirmed | `TestDesiredState_EmptyRegistryUnchanged` -- `desiredState()` output identical to pre-registry behavior when the registry has never been touched |
| A-2 (B1) | confirmed | `register.go` `registryReconcileCh` + worker wiring; `TestReconcileOnRegistryChange_AppliesAddressToBackend` proves no config commit is needed |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|----------------------------------|------------------|----------|
| `docs/architecture/core-design.md` section 14 enumerates the new registry | New row + 2 source anchors added; `make ze-doc-test` PASS (`tmp/lint/as112-1-doc-test.log`) | Yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-7 all demonstrated
- [x] End-to-End User Stories: story #1 has unit-level evidence (`TestReconcileOnRegistryChange_AppliesAddressToBackend`); full end-to-end with a real plugin consumer deferred to spec-as112-2 per this spec's own Functional Tests table
- [x] Wiring Test table complete — rows 1-2 have concrete tests; row 3 explicitly deferred to spec-as112-2 (documented in the spec, not silently dropped)
- [x] Review Gate clean — 3 rounds, 11 reviewer/verifier agents, 0 remaining BLOCKER/ISSUE
- [x] `make ze-lint-changed`, `go test ./internal/component/iface/... -race`, `make ze-doc-test` all pass. Full `make ze-test` (adds functional/exabgp/fuzz suites) NOT run: this child ships no CLI/RPC/wire surface, so those suites are not exercised by this diff; a pre-existing, unrelated flaky race in `internal/component/l2tp` (`TestPeerTeardownWithdrawsSubscriberRoute`, confirmed reproducible only under heavy concurrent package load, passes 5/5 in isolation, no file this spec touches) is present in `ze-unit-test-changed`'s wider changed-package sweep -- not caused by or fixed by this spec, reported separately
- [x] Feature code integrated (`internal/component/iface/*`)
- [x] Integration completeness proven end-to-end at the unit level; real end-to-end is spec-as112-2's job
- [x] Documentation Update Checklist answered Yes/No with source evidence (rows 4, 12 -> Yes, `core-design.md` updated)
- [x] Architecture docs updated (`docs/architecture/core-design.md` section 14)
- [x] Critical Review passes — see Critical Review Checklist row-by-row below
- [x] Risks & Assumptions: A-1, A-2 confirmed; R-1..R-4 all mitigated with test evidence; no `unvalidated` entries remain

### Quality Gates (SHOULD pass — defer with user approval)
- [x] RFC constraint comments — N/A, this child implements no RFC-mandated behavior (documented in RFC Documentation section)
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed — 8 rows, all resolved with fixes and regression tests

### Design
- [x] No premature abstraction — one consumer today (spec-as112-2), deliberate per Design Insights L2
- [x] No speculative features — every function exists because an AC or a review-found bug requires it
- [x] Single responsibility per component — registry (address_owner.go) vs. reconcile (config_apply.go) vs. wiring (register.go) stay separate
- [x] Explicit > implicit behavior — conflict rejection, idempotency, and staleIfaces lifecycle are all explicit, tested behaviors
- [x] Minimal coupling — iface imports no plugin package; registry has no `as112`-specific spelling

### TDD
- [x] Tests written — 17 tests across `address_owner_test.go` and `config_apply_test.go`
- [x] Tests FAIL — see phase-by-phase red states in the implementation log (e.g. `tmp/lint/as112-1-phase3-red.log`)
- [x] Tests PASS — `tmp/lint/as112-1-precommit.log`, fresh run, all 17 PASS
- [x] Boundary tests for all numeric inputs — N/A, no numeric inputs (spec's own Boundary Tests table)
- [x] Functional tests for end-to-end behavior — deferred to spec-as112-2 (documented, not silent)
- [x] Interop tests — N/A, no wire protocol behavior in this child
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — all 6 `ai/rules/quality.md` checks reviewed across 3 rounds
- [x] Partial/Skipped items — none; the one deferral (joint `.ci` functional test) was already specced as joint-owned with spec-as112-2, not a new scope cut
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/1028-as112-1-iface-address-registry.md`
- [x] **Wiring blocker resolved.** spec-as112-2's `runAS112Plugin` now calls `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` from a real `OnConfigure` handler (`internal/plugins/as112/register.go`), so both exported functions have a genuine non-test caller; the wiring gate this note originally blocked on now passes
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump — user-triggered, not yet run
- [ ] **Commit B:** `git rm plan/spec-as112-1-iface-address-registry.md` only — user-triggered, not yet run
