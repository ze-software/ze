# Spec: wire-or-delete-unreached-code

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P1 |
| Release reason | Shipped code that no entry point reaches is either a feature the operator was told exists and cannot use (BGP flush, RADIUS helpers, the remote-access pool, the L2TP IPv6 path) or dead weight a reviewer reads as live. Both falsify what Ze claims to do. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Family S5 of the journal backlog analysis (2026-10-23).

## Task

**Root cause.** Features are built library-first: the function, method, field
or parameter is written and unit-tested, and the step that connects it to an
entry point (a command, a config reader, a dispatcher, a registry) never
happens. The two checks that look for this judge only exported symbols, so an
unexported function, a method, a package variable that is only read, a field
only tests fill and a parameter every caller fills with the same constant all
pass unseen.

**Goal.** One check, run on every commit over the Go it carries, refuses a
function, method, package variable, struct field or parameter with no
production reach: no non-test caller, no non-test writer, or a parameter every
caller fills identically. Every existing instance in the journal is then wired
to its entry point, with a test through that entry point, or deleted.

**Why it matters for shipping.** `Server.DecodeNLRI` has no caller, so the
engine never asks a plugin to decode NLRI; `fwdDedupMetricsPtr` is never
stored, so the dedup counters never reach Prometheus; `builtinReadOnlyProfile`
is reachable from tests only, so the profile the docs describe is not the one
the daemon applies. Each row is a claim the binary does not keep.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` (the cross-package wiring check and the added-symbol wiring check)
  -> Constraint: the check runs on the commit's own files, so the cost is paid where the defect is cheapest to fix.
- [ ] `ai/rules/no-layering.md`
  -> Decision: the new check replaces both existing checks; neither survives beside it.
- [ ] `ai/rules/principles.md` ("Work MUST NOT be called done until a user reaches the behavior through the real entry point")

**Key insights:**
- Two checks exist today and overlap: `internal/le/repo/wiring.go` (cross-package, exported, changed files) and `internal/le/doc/wiring/wiring.go` (added exported symbols, non-test reference). Both match by whole-word text, not by type information.

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

**Source files read:**
- [ ] `internal/le/repo/wiring.go` - header: "answers one question about every exported symbol a session added: does anything OUTSIDE its own package call it", by whole-word search
- [ ] `internal/le/repo/repository.go` - `Run` calls `checkCrossPackageWiring` among the repository checks
- [ ] `internal/le/doc/wiring/wiring.go` - header: checks "whether every ADDED exported symbol has a non-test reference", reading lines rather than a Go parser, with an allowlist in `allowlist.go`
- [ ] `internal/component/bgp/reactor/peer_send.go` - `(*Peer).SendAnnounce`
- [ ] `internal/component/authz/authz.go` - `builtinReadOnlyProfile`
- [ ] `internal/component/cli/editor.go` - `(*Editor).HasPendingLive`
- [ ] `internal/component/bgp/reactor/forward_dedup.go` - `fwdDedupMetricsPtr`
- [ ] `internal/component/plugin/server/events.go` - `(*Server).DecodeNLRI`

| Symbol | Non-test references on 2026-10-23 | Why neither check saw it |
|--------|-----------------------------------|--------------------------|
| `(*Peer).SendAnnounce` | none; the only production `SendAnnounce` call is the session method inside it | a method; same-package |
| `builtinReadOnlyProfile` | none | unexported |
| `(*Editor).HasPendingLive` | none | exported method on a type that has callers; word match finds the type |
| `fwdDedupMetricsPtr` | four `Load` calls, no `Store` | a variable that is read is "referenced" |
| `(*Server).DecodeNLRI` | none | predates the added-symbol check |
| `(*Transaction).QueueAnnounce` | none; the production `QueueAnnounce` call is on a `Peer` | word match counts the other receiver's call |
| `installerKernelFallbackPath` | two callers, but its input `build/kernel/.variant` is written only by `internal/test/fixture/install_fixture.go` | the function is called; its input has no production writer |

The analysis evidence for S5 (`SendAnnounce`, `builtinReadOnlyProfile`,
`HasPendingLive`, `fwdDedupMetricsPtr`) holds today.

**Behavior to preserve:** every exemption the two checks carry for a real
reach the text search cannot see (interface dispatch by an external library,
generated gRPC registration, `init` registration, templ and embed use) stays
an exemption, declared at the symbol rather than in a central list.

**Behavior to change:** the two checks become one; its population widens from
exported symbols to every declaration kind above.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create` with Go files in the commit; `./le repository check`.

### Transformation Path
1. Load the packages of the commit's Go files with type information.
2. For each function, method, package variable, struct field and parameter the
   commit adds or whose body it changes, collect its references across the
   module, excluding `_test.go` files.
3. Refuse when there is no non-test call, no non-test write (variables and
   fields), or every non-test caller passes the same constant (parameters).
4. Honour an exemption only when it is declared on the symbol with a reason
   naming the reach (registry, interface dispatch, generated code).
5. The refusal names the symbol, what is missing (caller, writer, varying
   argument), and the two ways out: wire it with a test through its entry
   point, or delete it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le <-> the Go module | type-checked package load of the commit's packages | No |

### Integration Points
- `internal/le/commit` structural gates: the check joins them.
- `internal/le/repo` `Run`: the check replaces `checkCrossPackageWiring`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | two checks become one |
| Zero-copy preserved where applicable | No | N/A, tooling |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | exemptions live at the symbol, no allowlist file |

## The fix

1. Replace `internal/le/repo/wiring.go` and `internal/le/doc/wiring/wiring.go`
   (with its `allowlist.go`) by one type-checked reachability check. Delete the
   two first, then build the one (`ai/rules/no-layering.md`).
2. Run it on every commit that carries Go, over the declarations the commit
   adds or changes, and in `./le repository check` over the whole tree.
3. Wire or delete every owned row. Wiring means the entry point reaches the
   symbol and a test drives that entry point; a seam kept on purpose
   (`FeatureEntry.SrcAS`) is wired to its consumer or removed with its writer.
4. The whole-tree run is clean at closure: no ledger of tolerated findings.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `./le commit create` with a planted unexported function with no caller | -> | reachability check | `TestReachabilityRefusesUncalledUnexportedFunc` (new) |
| `./le commit create` with a planted variable that is only loaded | -> | reachability check | `TestReachabilityRefusesNeverWrittenVar` (new) |
| `./le repository check` over the tree | -> | reachability check, whole-tree mode | `TestReachabilityTreeModeOnRealCheckout` (new) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A commit adds an unexported function or a method with no non-test caller | refused, naming symbol and the wire-or-delete fix |
| AC-2 | A commit adds a package variable or struct field never written outside tests | refused |
| AC-3 | A commit adds a parameter every non-test caller fills with the same constant | refused |
| AC-4 | A symbol reached only through a declared exemption (registry, interface dispatch, generated code) | accepted; an exemption without a reason is refused |
| AC-5 | The old checks | `internal/le/repo/wiring.go` and `internal/le/doc/wiring/wiring.go` with `allowlist.go` are gone; no caller names them |
| AC-6 | `./le repository check` over the tree at closure | zero reachability findings |
| AC-7 | Each owned row | its symbol is reached from the entry point named in the row, with a test through that entry point, or it is deleted together with its tests |
| AC-8 | Reproduce: revert the wiring of one owned row (`fwdDedupMetricsPtr` store) | the check refuses it again |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReachabilityRefusesUncalledUnexportedFunc` | `internal/le/repo/reachability_test.go` | AC-1 | |
| `TestReachabilityRefusesNeverWrittenVar` | `internal/le/repo/reachability_test.go` | AC-2 | |
| `TestReachabilityRefusesConstantParameter` | `internal/le/repo/reachability_test.go` | AC-3 | |
| `TestReachabilityHonoursDeclaredExemption` | `internal/le/repo/reachability_test.go` | AC-4 | |
| `TestReachabilityTreeModeOnRealCheckout` | `internal/le/repo/reachability_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| one `.ci` per wired feature row | `test/plugin/`, `test/reload/`, `test/exabgp-compat/` | the operator reaches the feature (flush, BoRR and EoRR, decode-nlri, pipe alias in the TUI, remote-access pool, L2TP IPv6) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | tooling check; protocol rows that get wired carry their interop proof in their own test | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- delete the two checks, build the reachability check, register it in the commit gate and the repository check
   - Tests: AC-1 to AC-5
2. **Phase: owned rows** -- wire or delete each, one commit per row group, each with its entry-point test
   - Tests: AC-7
3. **Phase: tree clean** -- run the whole-tree mode, resolve what it finds beyond the journal rows the same way
   - Tests: AC-6, AC-8

## Files to Modify
- `internal/le/repo/repository.go`
- `internal/le/commit/` structural gate registration
- the file of each owned symbol (Owned journal rows)

## Files to Create
- `internal/le/repo/reachability.go` and `reachability_test.go`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| unwired-feature | 2026-10-17 | `Server.DecodeNLRI` no caller |
| unwired-feature | 2026-10-14 | `L3VPNRoute` has no PathID; vpn `EncodeRoute` never sets it |
| unwired-feature | 2026-10-10 | `Editor.HasPendingLive` no non-test caller |
| unwired-feature | 2026-10-08 | `findRPC`, `findRPCByCommand`, `registerCLICommand` test-only |
| unwired-feature | 2026-10-08 | bridge `formatFlushRequest`; `newSocketReloadNotifier` |
| unwired-feature | 2026-09-06 | `fwdDedupMetricsPtr` never stored |
| unwired-feature | 2026-09-05 | `(*Transaction).QueueAnnounce` test-only |
| unwired-feature | 2026-09-05 | `builtinReadOnlyProfile` test-only |
| unwired-feature | 2026-08-31 | radius exported symbols with no cross-package caller |
| unwired-feature | 2026-08-11 | iface vpp logger is `DiscardLogger` with no setter |
| unwired-feature | 2026-08-10 | l2tp ppp `IfaceBackend` onlink gateway implemented by no backend |
| unwired-feature | 2026-08-14 | ike engine exported symbols with no cross-package caller |
| unwired-feature | 2026-08-14 | five web config templates reach no caller |
| unwired-feature | 2026-08-16 | ike remote-access virtual IP pool allocator built, never used |
| unwired-feature | 2026-08-19 | cli `formatChangeEntry` no production caller |
| unwired-feature | 2026-08-21 | plugin pipe alias unreachable from the `ze cli` TUI client |
| unwired-feature | 2026-08-25 | iface `LinkPayload`, `StatePayload` no caller |
| unwired-feature | 2026-08-30 | l2tp PPP IPv6 subscriber path unreachable (RA sender, DHCPv6-PD) |
| unwired-feature | 2026-09-05 | trafficfeature `FeatureEntry.SrcAS` written, never consumed |
| unwired-feature | 2026-09-28 | `rfcgap.SkipNow`, `storage.HistorySeverityUnspecified` no non-test reference |
| unwired-feature | 2026-10-13 | `Peer.SendAnnounce` test-only |
| parameter-no-caller-ever-fills | 2026-10-10 | `installerKernelFallbackPath`, `installerKernelBuildMatches` need `.variant` only a fixture writes |
| parameter-no-caller-ever-fills | 2026-10-08 | rsvpte `resvFrom` `style` constant |
| parameter-no-caller-ever-fills | 2026-09-08 | le qemu `vrrpKeepalivedConfig` `priority` constant |
| parameter-no-caller-ever-fills | 2026-09-12 | pppoe interop helpers `timeout`, `username`, `peer`, `needle` constant |
| parameter-no-caller-ever-fills | 2026-09-15 | reactor test `yangEnumValues` `module` constant |
| production-function-only-tests-call | 2026-10-17 | filter_irr `irrPrefixList.evaluateUpdate` test-only |
| unwired-feature | 2026-09-14 | debug `set debug profile name` and `set debug active name` reached by no `.ci` |

30 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| unwired-feature | 2026-08-15 | rib storage `(*RouteEntry).ToWireBytes` no caller | called by `internal/component/bgp/plugins/rib/rib_recovery.go` (best-path recovery) and `rib_flowspec_validation.go` |

## Rows considered and not owned

All other open `unwired-feature` rows (45) have another root cause:

| Destination | Dates of the rows |
|-------------|-------------------|
| S1, YANG leaf, RPC or notification with no reader, or the parser value mode | 10-23, 10-22, 10-10, 09-04 (seven rows), 09-03 (three rows), 08-29, 09-06 (manual-eor), 09-09, 09-20 |
| S3 lifecycle (`spec-single-owner-lifecycle.md`) | 10-13 (`addPeerLocked`), 09-08 (`handleState`) |
| S3 reload (`spec-single-owner-reload-reconcile.md`) | 10-11, 09-20 (isis), 08-31 (bmp environment) |
| S4, one fact coded twice | 08-12 (large-community enrich), 08-30 (legacy env alias) |
| S7, ExaBGP bridge and migration | 09-05 (conf-watchdog), 10-17 (three rows) |
| T2 or other tooling | 09-07, 09-02 (codeql), 08-31 (interop inject), 08-19 (two rows), 08-28, 09-06 (reload runner), 09-14 |
| Protocol and product one-offs | 09-14 (kernel AH), 09-08 (GR capability tuples), 09-03 (RADIUS accounting), 09-02 (loop-detection), 08-22, 08-15 (two mcp rows), 08-17, 08-23 |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row before implementation starts
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-8 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: `./le test unit` over `internal/le/repo` and each package an owned row touches, and `./le verify current fast`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Owned rows re-pointed to this spec's closure
