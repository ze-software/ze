# Spec: osx-interface-listing-check

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

<!-- Scope tooling: the work is running and judging code already committed on a
     platform the Linux sessions cannot reach. No wire, config or CLI surface
     changes unless a check below fails. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Commit `52eb8b4f77` gave the non-Linux interface backend a real listing: on
macOS, `ListInterfaces` and `GetInterface` now read the host through Go's
`net.Interfaces`, so `ze init` discovers interfaces and writes an initial
config there. Before it, the macOS backend refused every call, `ze init`
warned `interface discovery: interface management not supported on darwin`,
and no initial config was written. Commit `cd4f2afe1e` made that config's file
name follow the instance name, which is what a bare `ze start` reads.

None of the macOS code has run. It was written on Linux, and the only evidence
is `go vet` with `GOOS=darwin`, which compiles the code and its tests but
executes nothing. The tests written alongside it have never been seen to fail.

This spec runs the code on macOS and proves it: the tests pass, the tests go
red when the listing is broken, and the user path from `ze init` to `ze start`
works on a Mac. A defect found here is fixed here.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - "Stub (non-Linux)" and "Interface Discovery" describe what the Mac backend now claims
  → Constraint: the page says listing works and every other method returns "not supported on <GOOS>"; the run must confirm both halves
- [ ] `docs/guide/configuration.md` - "Discovery During Init" names the file init writes
  → Constraint: init writes `<name>.conf`, and an invalid instance name falls back to `ze.conf`
- [ ] `ai/rules/platform-linux.md` - what is allowed to be Linux-only
  → Decision: listing is not Linux-only any more; management still is

**Key insights:**
- The backend is still called "netlink" on macOS; the stub registers under that name, so `iface.LoadBackend("netlink")` succeeds there.
- The macOS listing reports no link kind. The loopback is typed by its flag; anything else with a hardware address is classified ethernet by `infoToZeType`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/netlink/show_other.go` - stdlib `ListInterfaces`/`GetInterface`; a failed address read is an error, IPv6 link-local is marked
- [ ] `internal/plugins/iface/netlink/backend_other.go` - every other method returns "not supported"
- [ ] `internal/component/iface/discover.go` - `DiscoverInterfaces`, `infoToZeType` (loopback by Type, ethernet by MAC)
- [ ] `internal/plugins/init/main.go` - discovery, then `zefs.KeyFileActive.Key(resolve.DefaultConfig(store))`
- [ ] `internal/core/resolve/resolve.go` - `DefaultConfig`, the instance-name rule

**Behavior to preserve:**
- Linux behavior is untouched: the netlink backend and its tests are not in scope.
- Every macOS management method keeps refusing with "interface management not supported on darwin".

**Behavior to change:**
- None planned. A check that fails turns into a fix in this spec.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze init` on macOS, answers on stdin: username, password, host, port, instance name.

### Transformation Path
1. `iface.LoadBackend("netlink")` loads the stub backend.
2. `iface.DiscoverInterfaces` calls the stub's `ListInterfaces`, which calls `net.Interfaces` and `Addrs`.
3. `infoToZeType` classifies each interface; `iface.EmitConfig` writes the config text.
4. init stores it at `file/active/<instance>.conf` in the new store.
5. A bare `ze start` reads `resolve.DefaultConfig(store)`, the same key.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ macOS kernel | `net.Interfaces` and `Interface.Addrs` (sysctl route dump under the standard library) | No |
| init ↔ daemon | the ZeFS store, key `file/active/<instance>.conf` | No |

### Integration Points
- `iface.Backend` - the stub satisfies the whole interface; only two methods answer.
- `TestZeInitActiveConfigFollowsInstanceName` - now runs on every platform.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `net.Interfaces` on macOS returns `lo0` with `FlagLoopback` | Go standard library, BSD route sysctl | the init test fails on the Mac and no loopback entry is written | `TestStubListInterfacesFindsLoopback` on macOS | unvalidated |
| A-2 | `Addrs` does not fail for any interface a normal Mac has (utun, awdl0, llw0, bridge0, anpi) | none: the listing now errors on a failed address read, where the deleted version ignored it | `ze init` gets no interfaces at all on a Mac with such an interface | `ze init` on the owner's Mac | unvalidated |
| A-3 | `EmitConfig` produces a config `ze config validate` accepts from the macOS listing | Linux runs only | `ze start` refuses the config init wrote | AC-4 | unvalidated |
| A-4 | Every macOS interface classified ethernet is acceptable in the initial config | owner not yet asked | Wi-Fi, AirDrop and VPN tunnels appear as ethernet entries | owner decision, see R-2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | One unreadable interface makes the whole listing fail (A-2) | `ze init` warns `interface discovery: iface: addresses of "<name>"` | decide with the owner: skip that interface with a log line, or keep the refusal |
| R-2 | The initial config carries many virtual macOS interfaces as ethernet | the written config lists awdl0, llw0, utun*, bridge0, anpi* | show the owner the config; filtering is a design change and needs his decision |
| R-3 | golangci-lint has never seen the darwin files | lint run with `GOOS=darwin` reports findings | fix the findings in this spec |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `ze init` on macOS writes no config, a wrong one, or fails; Linux is unaffected |
| How is it reverted? | single commit revert of `52eb8b4f77` restores the refusing stub |
| Who else touches this path? | the interface component (`internal/component/iface`), init, the appliance seed path (Linux only) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init` stdin with instance `edge` | → | stub `ListInterfaces` via `iface.DiscoverInterfaces`, then `resolve.DefaultConfig` key | `TestZeInitActiveConfigFollowsInstanceName` (`internal/plugins/init/instance_config_test.go`), run on macOS |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go test ./internal/plugins/iface/netlink/ ./internal/plugins/init/` on macOS | passes, output pasted |
| AC-2 | `ListInterfaces` in `show_other.go` temporarily returns the old "not supported" error | `TestStubListInterfacesFindsLoopback` and `TestZeInitActiveConfigFollowsInstanceName` fail; restored, both pass; red output pasted |
| AC-3 | `ze init` on the Mac with instance name `edge` | prints `discovered N interface(s), wrote initial config`; the store holds `file/active/edge.conf` and no `file/active/ze.conf` |
| AC-4 | bare `ze start` on that store | reads `edge.conf`: the log says `Starting ze with config: edge.conf` and no second config is bootstrapped |
| AC-5 | `ze show interface` against the running Mac daemon | lists the host's interfaces with addresses; a create or address change is refused with "not supported on darwin" |
| AC-6 | golangci-lint over the two netlink files with `GOOS=darwin` | no finding |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStubListInterfacesFindsLoopback` | `internal/plugins/iface/netlink/show_other_test.go` | the macOS listing answers and marks the loopback | |
| `TestStubGetInterfaceMatchesList` | `internal/plugins/iface/netlink/show_other_test.go` | single lookup agrees with the listing | |
| `TestStdlibAddrInfo` | `internal/plugins/iface/netlink/show_other_test.go` | family, prefix length, link-local mark, non-prefix address skipped | |
| `TestZeInitActiveConfigFollowsInstanceName` | `internal/plugins/init/instance_config_test.go` | init writes `<instance>.conf` where `ze start` reads it | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| instance name length | 1-64 | 64 characters | empty falls back to `ze.conf` | 65 characters falls back to `ze.conf` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| manual run, AC-3 to AC-5 | owner's Mac, transcript pasted in this spec | an operator initializes and starts Ze on a Mac | |

### Interop Tests (Scope: protocol)
N-A: no wire behavior.

## Files to Modify
- `internal/plugins/iface/netlink/show_other.go` - only if a check fails
- `internal/plugins/iface/netlink/show_other_test.go` - only if a check fails
- `docs/features/interfaces.md` - only if the run contradicts the "Stub (non-Linux)" section

## Files to Create
- None planned.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no schema change |
| YANG validation constraints | N-A | no schema change |
| YANG custom validators | N-A | no schema change |
| CLI commands/flags | N-A | no command change |
| CLI grammar (keyword before value) | N-A | no command change |
| Editor autocomplete | N-A | no command change |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | the standard library needs no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | evidence only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` "Discovery During Init", if the run contradicts it |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/interfaces.md` anchors `show_other.go` and `backend_other.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the wiring test on macOS
   - Tests: `TestZeInitActiveConfigFollowsInstanceName`
   - Files: none
   - Verify: passes on the Mac; output pasted
2. **Phase: Discrimination** -- AC-2
   - Tests: the four unit tests
   - Files: `show_other.go`, broken then restored
   - Verify: red with the old stub answer, green restored
3. **Phase: User path** -- AC-3 to AC-5 by hand on the Mac
   - Files: none, unless a step fails
   - Verify: transcript pasted; R-1 and R-2 answered from what the run showed
4. **Phase: Lint** -- AC-6
   - Verify: no finding

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has pasted output from a Mac |
| Correctness | the discrimination break is the real regression (the "not supported" answer), not a stub that cannot fail |
| Data flow | init and `ze start` agree on the key through `resolve.DefaultConfig` alone |
| Rule: platform-linux | nothing Linux-only leaked into the darwin files |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| macOS test run | `go test ./internal/plugins/iface/netlink/ ./internal/plugins/init/` output in this spec |
| discrimination record | red then green output in this spec |
| user-path transcript | `ze init`, `ze start`, `ze show interface` output in this spec |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `GetInterface` validates the name before the lookup (`iface.ValidateIfaceName`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Restore the standard-library listing deleted by `564d8b6ff5` inside the stub backend | correct the docs to say discovery is Linux-only | owner chose to build the missing discovery, 2026-10-09 |

## Known Limitations
- The macOS backend still manages nothing: creating, addressing and changing interfaces stays Linux-only.

## RFC Documentation (Scope: protocol)

N-A: no protocol.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
