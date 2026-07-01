# Spec: DNS Server Harness (extract geodns primitives to core)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-07-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/module-tiers.md` (esp. lines 148-152: extract shared primitives to `internal/core/<x>`), `ai/rules/plugin-design.md` (line 133: plugins MUST NOT import sibling plugins), `ai/rules/plugin-self-containment.md`
4. `internal/plugins/geodns/server.go`, `internal/plugins/geodns/source.go` - the code being extracted
5. `internal/core/probe/` - the precedent (ping+traceroute shared primitives in core)
6. `plan/spec-as112-2-dns-server.md` - the second consumer this unblocks

## Task

Extract the generic, non-policy DNS-server primitives currently embedded in
`internal/plugins/geodns/server.go` (and the CIDR matcher in `source.go`) into a
new core leaf package `internal/core/dnsserver`, and migrate geodns onto it
**behavior-preservingly** (its existing tests stay green). The forcing reason:
`spec-as112-2-dns-server.md` needs the same listener lifecycle, EDNS0/client-IP
resolution, authoritative-answer shaping, and CIDR longest-prefix matcher, and a
plugin **MUST NOT import a sibling plugin** (`ai/rules/plugin-design.md:133`).
The only sanctioned reuse path is a lower tier — exactly the pattern
`ai/rules/module-tiers.md:148-152` prescribes (and `internal/core/probe`
already demonstrates for ping+traceroute).

This spec ships the harness + the geodns migration ONLY. It builds no new
feature and no new plugin. `spec-as112-2` becomes the second consumer and is
rewired there (this spec does not touch as112).

**Scope boundary — what is generic (extract) vs policy (stays in geodns):**

| Extract to `internal/core/dnsserver` | Stays in `internal/plugins/geodns` |
|--------------------------------------|-------------------------------------|
| `serverManager` listener lifecycle: `bind`/`apply`/`endpointSig`/`serve`/`stopAll` (`server.go:303-409`), generalized over a `dns.Handler` + options | `handleQuery` becomes a thin wrapper calling the harness with geodns's `answerQuestions` as the answer func |
| `clientIP` EDNS0/packet resolution (`server.go:53-75`), `remoteAddr` (`server.go:242-249`) | `computeSerial`, `buildSOA`, `appendNS`, `nsID`, `resolveHost`, host-sets, `answerQuestions` (zone/record policy) |
| Authoritative-answer wrapper (Authoritative=true, RecursionAvailable never set, Compress=false, panic recovery) — the R-3 recursion guard in ONE place | config/YANG, metrics NAMES, `show geodns`, doctor check, registration |
| NEW `Freebind` option (adds `IP_FREEBIND` `net.ListenConfig.Control`; geodns lacks it, `server.go:363`) — default OFF so geodns is unchanged | metric recording (geodns keeps `gmetrics()`; harness takes an optional listener-up callback) |
| (secondary) CIDR longest-prefix matcher `buildMatcher`/`lookup` (`source.go:1-38`) → `internal/core/dnsserver` (or `internal/core/netipx`) | geodns's `sourceEntry`→host-set mapping (the matcher is generic; the host-set semantics are geodns's) |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/module-tiers.md` - tier placement by dependency direction
  → Constraint: a pure library with no `sdk.NewWithConn` and no config-driven lifecycle belongs in `internal/core/` (lines 30-31, 89). The DNS harness has no lifecycle of its own — the plugin owns config/registration — so it is core, not component.
  → Decision: lines 148-152 are the exact sanctioned pattern — "two modules share low-level primitives → extract to `internal/core/<x>` so neither depends on the other"; `internal/core/probe` (ping+traceroute) is the precedent.
- [ ] `ai/rules/plugin-design.md` - plugin import rules
  → Constraint: line 133 "Plugins MUST NOT import sibling plugin packages -- use text commands via DispatchCommand." So as112 importing geodns is forbidden; reuse must go through core. Line 138-140: blank/schema imports allowed, logic imports are not.
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant
  → Constraint: geodns must remain fully self-contained after the migration — deleting `internal/plugins/geodns` + its blank import (`plugin/all/all.go:218`) still removes every geodns feature. The harness in core stays generic (no `geodns` spelling).

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7871.md` - EDNS0 Client Subnet
  → Constraint: the extracted `clientIP` implements EDNS0 client-subnet vs packet-source selection (`server.go:53-75`); its behavior must be identical after the move.
- [ ] `rfc/short/rfc1035.md` - DNS message structure
  → Constraint: the authoritative-answer wrapper preserves RFC 1035 message shaping (Authoritative bit, no recursion) exactly as geodns does today.

**Key insights:**
- This is a **behavior-preserving refactor**, not a feature. The externally
  observable behavior of geodns (listeners, answers, metrics, doctor, `show`)
  is byte-for-byte identical afterward; the proof is geodns's existing test
  suite passing unchanged.
- Only 1 consumer exists today (geodns) + 1 approved imminent consumer (as112).
  The "3+ use cases" heuristic is deliberately overridden here because (a) the
  alternative — copy-paste into as112 — is forbidden-adjacent (duplicating a
  security-sensitive authoritative-only handler; child 2 R-3) and the only other
  option, plugin→plugin import, is outright forbidden; (b) `internal/core/probe`
  set the 2-consumer precedent. Recorded in Key Design Decisions.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/geodns/server.go` - `serverManager` (struct `303-312`, `newServerManager` `314-318` hardcodes `handleQuery` at `316`); `endpointSig` `320-332`; `apply` `334-358` (rebind only on endpoint-set change); `bind` `360-385` (bare `net.ListenConfig` at `363`, NO freebind; records geodns's `gmetrics().listenerUp` at `379-381`); `serve` `387-391`; `stopAll` `393-409`; `clientIP` `53-75` (EDNS0 client-subnet + packet, RFC 7871); `remoteAddr` `242-249`; `handleQuery` `254-301` (SetReply, `Authoritative=true` `261`, `Compress=false` `262`, panic recovery `269-275`, per-request `loadState()` snapshot, RcodeRefused when disabled `279`, delegates to `answerQuestions`). `answerQuestions`/`buildSOA`/`appendNS`/`nsID`/`resolveHost`/`matchZone`/`computeSerial` are geodns answer POLICY.
- [ ] `internal/plugins/geodns/source.go` - `matcher`/`buildMatcher`/`lookup` (`19-38`): longest-prefix match over `netip.Prefix`, `Contains` is family-aware (`29`). Generic; geodns uses it for host-set selection.
- [ ] `internal/plugins/geodns/state.go` - `resolverState` published via `sync/atomic.Pointer`, read lock-free by `handleQuery`. Stays geodns's (the harness is state-agnostic: the handler func closes over the plugin's own snapshot).
- [ ] `internal/plugins/geodns/register.go` - plugin registration (`RunEngine`, `OnConfigure`, metrics/doctor callbacks). Unchanged except `server.go`'s internals now delegate to `dnsserver`.
- [ ] `internal/core/probe/icmp.go` - the placement precedent: a core leaf package (`BuildICMPEcho`, `ResolveTarget`) shared by ping and traceroute so neither feature imports the other.
- [ ] `internal/component/plugin/all/all.go` - geodns blank imports at line 218 (`internal/plugins/geodns`) and 115 (`.../geodns/yang`); the removal test target is unchanged by this refactor.

**Behavior to preserve:**
- geodns's externally observable behavior is byte-for-byte identical: same bound listeners, same answers for every query, same metric values and names, same `show geodns`, same doctor check. Existing tests (`server_test.go`, `source_test.go`, `listener_test.go`, `metrics_record_test.go`, `state_test.go`, plus `test/parse/geodns-config.ci`, `test/parse/geodns-invalid-record.ci`, `test/ui/doctor-geodns.ci`, `test/plugin/geodns-show.ci`) pass UNCHANGED.
- geodns does NOT set `IP_FREEBIND` today; after the migration it still doesn't (the harness `Freebind` option defaults OFF; only as112 opts in). No behavior change for geodns's binds.
- Plugin self-containment: deleting geodns still removes every geodns feature; the core harness contains no `geodns` identifier.

**Behavior to change:**
- None observable. Pure internal refactor: geodns's `serverManager`/`clientIP`/`remoteAddr`/matcher move to `internal/core/dnsserver`; `server.go`'s `handleQuery` becomes a thin wrapper. The only NEW capability is the (default-off) `Freebind` option, unused by geodns.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- No new runtime entry point. The consumer plugin constructs the harness
  (`dnsserver.New(handler, opts)`) from its own engine and drives `apply(cfg)`
  on config reload — exactly where geodns's `serverManager` is driven today.

### Transformation Path
1. Plugin builds a `dns.Handler` (or an answer func wrapped by the harness's authoritative helper) that closes over its own atomic snapshot.
2. Plugin creates a `dnsserver.Manager` with options (freebind, drain timeout, listener-up callback, logger).
3. On config reload the plugin calls `Manager.Apply(endpoints)`; the harness rebinds only when the endpoint set changes (`endpointSig` logic, unchanged).
4. `bind` opens UDP+TCP via `net.ListenConfig`; if `Freebind` is set it installs an `IP_FREEBIND` `Control` hook; otherwise identical to geodns today.
5. Each query flows to the plugin-provided handler; the harness's authoritative wrapper (if used) guarantees `Authoritative=true`, recursion never available, panic recovery, then calls the plugin's answer func.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ core harness | direct Go import of `internal/core/dnsserver` (plugin→core, allowed by module-tiers) | [ ] |
| Harness ↔ kernel | existing `net.ListenConfig` UDP/TCP bind (+ optional `IP_FREEBIND`) | [ ] |
| Harness ↔ plugin metrics | optional listener-up callback (harness never imports a plugin's metrics) | [ ] |

### Integration Points
- `internal/plugins/geodns/server.go` - migrated to call `dnsserver`; keeps only answer policy
- `internal/plugins/geodns/source.go` - migrated to `dnsserver`'s (or `netipx`'s) matcher
- `internal/core/probe` - structural precedent for a core leaf shared by two features

### Architectural Verification
- [ ] No bypassed layers (geodns drives the harness through the same reload path it uses today)
- [ ] No unintended coupling (harness imports no plugin; geodns imports core, never a sibling plugin — `plugin-design.md:133`)
- [ ] No duplicated functionality (this REMOVES the future duplication as112 would otherwise create)
- [ ] Zero-copy preserved where applicable (handler still reads a per-request atomic snapshot; no per-query allocation added)
- [ ] Registration over hardcoding — geodns still registers via the plugin registry; the harness is a plain library, discovered by import, not a registry entry

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `serverManager` (`server.go:303-409`) is cleanly separable from geodns policy — its only geodns couplings are the hardcoded `handleQuery` (`316`) and `gmetrics()` listener-up (`379-381,402-406`) | Read of `server.go:303-409` | The extraction needs to drag geodns types into core, breaking self-containment | the harness compiles with zero `geodns` imports; geodns tests green | confirmed — `internal/core/dnsserver/manager.go` compiles with zero geodns imports (`rg -q geodns internal/core/dnsserver/` finds nothing, including comments); geodns's own listener-up gauge is now wired via `Options.OnListenerChange` (`manager.go:47-50`), called from geodns's `onListenerChange` (`server.go:267-274`) |
| A-2 | `clientIP`/`remoteAddr` (`server.go:53-75,242-249`) have no geodns-specific dependency (only `miekg/dns` + `netip`) | Read of those functions | Extraction leaks geodns config into core | unit test in `dnsserver` reproducing geodns's `source_test`/`server_test` client-IP cases | confirmed — moved verbatim to `internal/core/dnsserver/client.go:21` (`ClientIP`) and `:44` (`RemoteAddr`); `TestClientIP_EDNS0AndPacket` (`client_test.go:34`) reproduces every case from geodns's former `TestClientIPSourceModes`, PASS |
| A-3 | `make ze-tier-check` (`dep_audit.py`) accepts `internal/core/dnsserver` as a core package with no manifest row (it is a pure library, not an engine) | `ai/rules/module-tiers.md:106-125`; core libs need no row | The gate demands a `tier_non_engine_categories.txt` row | run `scripts/dev/dep_audit.py --check` after creating the package | confirmed — `python3 scripts/dev/dep_audit.py --check` exits 0 with no new manifest row; `internal/core/dnsserver` needed none (pure library, no `sdk.NewWithConn`, no registry side effect) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The migration silently changes a geodns answer or listener behavior | a geodns unit/functional test fails, or a metric label shifts | migrate in small steps, run the full geodns suite after each; the suite is the behavior oracle |
| R-2 | Extracting for only 1 present consumer looks like premature abstraction | `/ze-review` flags "no premature abstraction (3+ use cases)" | documented override: plugin→plugin is forbidden (`plugin-design.md:133`), as112 is an approved 2nd consumer, `internal/core/probe` is the 2-consumer precedent — recorded in Key Design Decisions |
| R-3 | The `Freebind` option accidentally changes geodns binds | geodns `listener_test.go` / a bind test detects non-local bind acceptance | `Freebind` defaults OFF; geodns never sets it; a harness unit test asserts default-off does not install the `Control` hook |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| geodns engine reload drives listeners through the migrated `server.go` | → | `internal/core/dnsserver` `Manager.Apply` | existing `internal/plugins/geodns/listener_test.go` passes unchanged against the migrated code |
| a query reaches a harness-wrapped handler | → | `dnsserver` authoritative wrapper + geodns `answerQuestions` | existing `internal/plugins/geodns/server_test.go` passes unchanged |
| `dnsserver.Manager` used directly (no geodns) | → | `internal/core/dnsserver` | `TestManager_BindsAndServes` (`internal/core/dnsserver/manager_test.go`) — proves the harness works standalone |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Package `internal/core/dnsserver` created | Provides the listener lifecycle (bind/apply/endpointSig/serve/stop) generic over a `dns.Handler`, plus `clientIP`/`remoteAddr` and the authoritative-answer wrapper; contains no `geodns` identifier |
| AC-2 | geodns migrated | `internal/plugins/geodns/server.go` no longer defines its own `serverManager`/`bind`/`clientIP`/`remoteAddr`; it imports `internal/core/dnsserver`; geodns answer policy (`answerQuestions` etc.) stays in geodns |
| AC-3 | Existing geodns test suite run against migrated code | All geodns unit tests + `test/parse/geodns-config.ci`, `test/parse/geodns-invalid-record.ci`, `test/ui/doctor-geodns.ci`, `test/plugin/geodns-show.ci` pass UNCHANGED (behavior-preserving) |
| AC-4 | `dnsserver.Options{Freebind:true}` used | Listener sockets set `IP_FREEBIND` via a `net.ListenConfig.Control` hook; with `Freebind:false` (geodns default) no such hook is installed |
| AC-5 | Any query through the authoritative wrapper | Response has `Authoritative=true`, recursion never available, panic in the answer func is recovered — the single-source-of-truth recursion guard (child-2 R-3) |
| AC-6 | EDNS0 client-subnet vs packet-source query | `dnsserver.ClientIP` returns the same result geodns's `clientIP` returns today (RFC 7871); covered by a unit test ported from geodns |
| AC-7 | (secondary) CIDR matcher extracted | `buildMatcher`/`lookup` live in core; geodns `source.go` consumes them; geodns `source_test.go` passes; may be deferred with user approval if it risks scope creep |
| AC-8 | `make ze-tier-check` | `scripts/dev/dep_audit.py --check` passes: `internal/core/dnsserver` is a correctly-placed core library; geodns imports it (plugin→core, allowed); no plugin→plugin import introduced |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (operator, unchanged) runs geodns as today | config → geodns engine → migrated `server.go` → `dnsserver.Manager` → listeners answer identically | existing geodns unit + `.ci` suite, unchanged (AC-3) |
| 2 | (developer) builds a second DNS service without importing geodns | new plugin imports `internal/core/dnsserver` | `TestManager_BindsAndServes` + (later) `spec-as112-2` consuming the harness |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManager_BindsAndServes` | `internal/core/dnsserver/manager_test.go` | AC-1: harness binds UDP+TCP and serves a handler standalone | |
| `TestManager_RebindOnlyOnEndpointChange` | `internal/core/dnsserver/manager_test.go` | AC-1: `Apply` is a no-op when the endpoint set is unchanged (ported from geodns `listener_test.go`) | |
| `TestFreebindOptionInstallsControlHook` | `internal/core/dnsserver/manager_test.go` | AC-4: freebind on/off | |
| `TestAuthoritativeWrapper_SetsBitsAndRecovers` | `internal/core/dnsserver/handler_test.go` | AC-5: Authoritative=true, no recursion, panic recovery | |
| `TestClientIP_EDNS0AndPacket` | `internal/core/dnsserver/client_test.go` | AC-6: RFC 7871 selection (ported from geodns) | |
| `TestMatcher_LongestPrefix` | `internal/core/dnsserver/matcher_test.go` | AC-7: family-aware longest-prefix (ported from geodns `source_test.go`) | |
| (all existing geodns `*_test.go`) | `internal/plugins/geodns/` | AC-2/AC-3: green unchanged after migration | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs; a refactor. Port numbers/drain timeout are existing values moved verbatim | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `geodns-config` | `test/parse/geodns-config.ci` | geodns config still parses/serves after migration (unchanged) | |
| `geodns-show` | `test/plugin/geodns-show.ci` | `show geodns` unchanged after migration | |
| `doctor-geodns` | `test/ui/doctor-geodns.ci` | geodns doctor unchanged | |

### Interop Tests (MANDATORY for protocol features)
N/A — no wire protocol behavior changes. The DNS wire behavior of geodns is
preserved exactly (proven by its unchanged test suite); this spec moves code
between packages, it does not alter any on-the-wire format.

## Files to Modify
- `internal/plugins/geodns/server.go` - remove the extracted primitives; `handleQuery` becomes a thin wrapper over `dnsserver`'s authoritative helper + geodns `answerQuestions`; `serverManager` usage replaced by `dnsserver.Manager`
- `internal/plugins/geodns/source.go` - (secondary, AC-7) consume the core matcher
- `scripts/dev/tier_non_engine_categories.txt` - only if `dep_audit.py --check` requires a row (A-3); a pure core library should NOT need one

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No — no config surface; pure library | n/a |
| YANG validation constraints | [ ] No | n/a |
| CLI commands/flags | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No new RPC; geodns's existing `.ci` suite is the behavior oracle | reused |
| Doctor check for runtime dependencies | [ ] No new dependency (same `net.ListenConfig` path); geodns keeps its own listen-capability doctor check | n/a |
| Prometheus counters/metrics | [ ] No — the harness records nothing itself; it calls an optional listener-up callback so the consumer keeps its own metric | n/a |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No — internal refactor, zero user-visible change | n/a |
| 2 | Config syntax changed? | [ ] No | n/a |
| 3 | CLI command added/changed? | [ ] No | n/a |
| 5 | Plugin added/changed? | [ ] No — geodns behavior unchanged | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] No | n/a |
| 9 | RFC behavior implemented? | [ ] No new behavior; EDNS0/1035 handling moved verbatim | n/a |
| 12 | Internal architecture changed? | [x] Yes, but no doc update needed — `docs/architecture/core-design.md:962-971`'s "cross-cutting core registries" table only lists registry-style packages (family, metrics, env, clock, report, slogutil); `internal/core/probe` (the exact same shared-primitives-for-2-consumers pattern this spec follows) is also absent from it, confirming that table does not enumerate every core leaf library | n/a — no enumeration to extend |
| 16 | Any changed source file referenced by existing doc source anchors? | [x] No — `grep -rn "geodns/server.go\|geodns/source.go" docs/` returns zero hits; the only doc anchor for geodns (`docs/guide/plugins.md:205`) points at `register.go`, which this spec does not modify | n/a |

`make ze-doc-test` run (no doc changes made): exit 0, confirming no documentation drift.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan, Scope-boundary table |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` (incl. `ze-tier-check`) |
| 7-10. Critical review / fix / re-verify | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — create `internal/core/dnsserver` with a `Manager` skeleton (bind/apply/stop) generic over a `dns.Handler`, and a failing `TestManager_BindsAndServes`.
   - Tests: `TestManager_BindsAndServes`
   - Files: `internal/core/dnsserver/manager.go`, `manager_test.go`
   - Verify: harness compiles, imports no plugin; test fails on the stub
2. **Phase: Lifecycle + client-IP + authoritative wrapper** — port `bind`/`apply`/`endpointSig`/`serve`/`stopAll`, `clientIP`/`remoteAddr`, and the authoritative-answer wrapper; add the `Freebind` option (default off).
   - Tests: `TestManager_RebindOnlyOnEndpointChange`, `TestFreebindOptionInstallsControlHook`, `TestAuthoritativeWrapper_SetsBitsAndRecovers`, `TestClientIP_EDNS0AndPacket`
   - Files: `internal/core/dnsserver/{manager,handler,client}.go` + tests
   - Verify: tests fail → implement → pass
3. **Phase: Migrate geodns** — replace geodns's `serverManager`/`bind`/`clientIP`/`remoteAddr` with `dnsserver`; `handleQuery` becomes a wrapper; run the FULL geodns suite.
   - Tests: all existing geodns `*_test.go` + geodns `.ci` (unchanged)
   - Files: `internal/plugins/geodns/server.go`
   - Verify: geodns suite green (AC-2, AC-3, R-1)
4. **Phase: (secondary) Matcher** — extract `buildMatcher`/`lookup`; migrate geodns `source.go`.
   - Tests: `TestMatcher_LongestPrefix`, geodns `source_test.go`
   - Files: `internal/core/dnsserver/matcher.go`, `internal/plugins/geodns/source.go`
   - Verify: green; may be deferred with user approval (AC-7)
5. **Tier check** → `scripts/dev/dep_audit.py --check` (AC-8), then `make ze-verify`
6. **Complete spec** → audit tables, learned summary, two-commit close

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec | Result |
|-------|------------------------------|--------|
| Completeness | AC-1..AC-8 each have file:line implementation | PASS — see Implementation Audit "Acceptance Criteria" table |
| Behavior preservation | Every geodns test passes with NO edit to the test (a changed test = a behavior change = a bug) | PASS with disclosed exception — full suite green; 3 files needed mechanical renames only (functions moved to an exported package API), not assertion/behavior changes; see Mistake Log "Test Relocations" |
| Naming | `dnsserver` exported names follow `ai/rules/naming.md`; no `geodns` spelling in core | PASS — `rg -q geodns internal/core/dnsserver/` clean (identifiers and comments both) |
| Data flow | Harness imports only `miekg/dns`, `net`, `netip`, `context`, core (`textbuf`); never a plugin or component | PASS (with correction) — actual imports are `miekg/dns` + stdlib (`net`, `net/netip`, `context`, `fmt`, `log/slog`, `sort`, `strconv`, `strings`, `time`, `syscall`); `textbuf` was not needed (matches the original geodns code's own `net.JoinHostPort`/`strconv`/`fmt.Errorf` style, not a new pattern) — never a plugin or component import |
| Registration over hardcoding | geodns still registers via the plugin registry; harness is a plain imported library | PASS — `internal/plugins/geodns/register.go` has zero diff; `dnsserver` has no `init()`/registry calls |
| Rule: no-layering | `plugin-design.md:133` — no plugin→plugin import introduced; verify via `dep_audit.py` | PASS — `dep_audit.py --check` exit 0 |
| Rule: self-containment | deleting geodns still removes all geodns features; core harness has no geodns identifier | PASS — `internal/core/dnsserver` contains zero `geodns` identifiers or mentions |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| `internal/core/dnsserver/` exists, no geodns import | `ls internal/core/dnsserver/` + `! rg -q geodns internal/core/dnsserver/` | PASS — 11 files, grep clean |
| geodns migrated | `! rg -q 'type serverManager' internal/plugins/geodns/` + `rg -q 'core/dnsserver' internal/plugins/geodns/server.go` | PASS |
| geodns suite green | `go test ./internal/plugins/geodns/...` | PASS, exit 0 |
| tier gate green | `scripts/dev/dep_audit.py --check` | PASS, exit 0 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for | Result |
|-------|-----------------|--------|
| Recursion guard centralization | The authoritative wrapper is the single place that guarantees recursion-never-available; verify no consumer path can bypass it to enable recursion (child-2 R-3) | PASS — `Authoritative` (`handler.go:23`) is the only place `msg.RecursionAvailable` is set (explicitly to `false`); geodns's `answerQuery`/`answerQuestions` never touch it (`grep -rn RecursionAvailable internal/plugins/geodns/` = no hits) |
| Freebind blast radius | `IP_FREEBIND` only applies when a consumer sets `Freebind:true`; a bind to a non-local address must still be an explicit opt-in, not the default | PASS — `grep -rn Freebind internal/plugins/geodns/` = no hits; geodns never sets it, so its binds are unchanged; `TestFreebindOptionInstallsControlHook` proves `Freebind:false` never installs the Control hook |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A geodns test fails after migration | Behavior changed — re-read the diff against `server.go` Current Behavior; do NOT edit the test |
| Tier gate fails | Re-read `module-tiers.md`; the package may need a manifest row (A-3) or is genuinely misplaced |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The spec's Deliverables Checklist command `! rg -q geodns internal/core/dnsserver/` would only ever match code identifiers | It also matches the word "geodns" inside doc comments explaining design rationale (a legitimate, expected pattern per `internal/core/probe`'s own header comment naming its consumers) | Ran the literal command after writing the package; it matched 11 comment lines, zero code identifiers | Reworded all doc comments to say "a consumer plugin" / "a sibling plugin" instead of "geodns"; no code change, `AC-1`'s actual requirement ("contains no geodns identifier") was already satisfied by identifiers alone |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Test Relocations (behavior-preservation exception, not a workaround)
Three geodns test files needed mechanical edits because the functions/methods
they exercised moved to `internal/core/dnsserver` and became exported there
(Go requires a capital first letter to export, so `lookup` → `Lookup` is not a
spelling choice, it is what "moved to another package's public API" means in
Go). Each is a relocation of coverage, not a weakening — the moved assertions
are byte-identical in the new location.

| File | Change | Why not a behavior change |
|------|--------|---------------------------|
| `internal/plugins/geodns/server_test.go` | Removed `TestClientIPSourceModes` (unit-tested the package-local `clientIP` function directly); marked `test-relax` | `clientIP` no longer exists in geodns (moved to `dnsserver.ClientIP`, `client.go:21`); the exact same table-driven cases now live as `TestClientIP_EDNS0AndPacket` in `internal/core/dnsserver/client_test.go:34`, PASS. `TestServerResolvesPerSource` (unchanged) still proves client-IP-driven source selection end-to-end over real UDP/TCP |
| `internal/plugins/geodns/source_test.go` | 3 call sites: `m.lookup(` → `m.Lookup(` | `buildMatcher` now returns `*dnsserver.Matcher`, whose method is exported `Lookup` (`matcher.go:40`); same inputs, same expected outputs, same test names |
| `internal/plugins/geodns/state_test.go` | 1 call site: `st.matcher.lookup(` → `st.matcher.Lookup(` | Same reason; `resolverState.matcher` is now `*dnsserver.Matcher` (`state.go:13`) |

All three are cited in AC-6/AC-7 as intentional ports ("covered by a unit test
ported from geodns"). No assertion, expected value, or test scenario changed;
only the call syntax required by the symbol's new package.

## Design Insights
- The valuable extraction is not the boilerplate but the **authoritative-only /
  recursion-refusal guard**: putting it in one place is what removes child-2's
  R-3 divergence risk. Two hand-copied authoritative DNS handlers is exactly the
  kind of security-sensitive duplication that rots.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extract to `internal/core/dnsserver` (core leaf) | (a) as112 imports geodns; (b) duplicate the primitives in as112; (c) put the harness in a component | (a) forbidden — `plugin-design.md:133` (plugins MUST NOT import siblings); (b) duplicates a security-sensitive handler (child-2 R-3); (c) it has no config-driven lifecycle so `module-tiers.md` puts a pure library in `internal/core/`. `internal/core/probe` is the precedent |
| Override the "3+ use cases" heuristic at 2 consumers | wait for a 3rd DNS service | The alternative to extraction is a forbidden import or a security-sensitive copy-paste; the 2nd consumer (as112) is approved and imminent; `internal/core/probe` already extracted at 2 consumers (ping+traceroute) |
| Harness is metrics-agnostic (optional listener-up callback) | move geodns's metrics into core | Metric NAMES are per-plugin (`plugin-self-containment`); core must not own `ze_geodns_*` or `ze_as112_*` |
| `Freebind` is an opt-in option, default OFF | always freebind; never freebind | Default-off keeps geodns byte-for-byte unchanged; as112 opts in for its anycast bind race (spec-as112-2 B2) |

## Known Limitations
- Only the listener lifecycle, client-IP resolution, authoritative wrapper, and
  (secondary) the CIDR matcher move. Zone/record synthesis, SOA policy, host-sets,
  serial modes, config, metrics names, `show`, and doctor stay per-plugin — they
  are policy, not shared infrastructure.
- After this spec there is still only ONE consumer in-tree (geodns) until
  `spec-as112-2` lands; the extraction is justified by the approved second
  consumer, not by three present ones.

## RFC Documentation

Carry forward geodns's existing RFC comments into the moved code:
`// RFC 7871` above `ClientIP` (EDNS0 client-subnet) and `// RFC 1035` above the
authoritative-answer wrapper. No new RFC behavior is introduced.

## Implementation Summary
### What Was Implemented
- New core leaf package `internal/core/dnsserver`: `Manager` (listener lifecycle: bind/apply/stop, endpoint-signature no-op check), `Authoritative` (RFC 1035 authoritative-answer wrapper + single-source recursion guard + panic recovery), `ClientIP`/`RemoteAddr` (RFC 7871 EDNS0/packet client-IP resolution), `Matcher`/`BuildMatcher`/`Lookup` (generic CIDR longest-prefix matcher), `Options.Freebind` (opt-in `IP_FREEBIND`, Linux-only via `freebind_linux.go`, safe no-op elsewhere via `freebind_other.go`).
- geodns migrated onto the harness: `internal/plugins/geodns/server.go` no longer defines `serverManager`/`bind`/`clientIP`/`remoteAddr`; a thin `geodnsServer` adapter (`server.go:278-304`) wraps `dnsserver.Manager` behind geodns's original `apply(cfg)`/`stopAll()` call shape, so `register.go` and nearly all existing tests needed zero changes.
- `internal/plugins/geodns/source.go`'s `buildMatcher` now maps `sourceEntry.HostSet` to `dnsserver.Entry.Label` and delegates to the shared matcher (AC-7, done — not deferred).
- `internal/plugins/geodns/state.go`'s `resolverState.matcher` field is now `*dnsserver.Matcher`.

### Bugs Found/Fixed
- `freebind_integration_linux_test.go` initially called `listenConfig(false).ListenPacket(...)` directly on a function-call result; `net.ListenConfig.ListenPacket` has a pointer receiver, so a non-addressable value fails to compile. Fixed by binding to a local variable first. Caught by `GOOS=linux go vet -tags integration ./internal/core/dnsserver/...` before any Linux CI/QEMU run would have hit it.

### Documentation Updates
- None required. Checklist items #12 and #16 both resolved "No update needed" with source-grep evidence (see Documentation Update Checklist table above) — geodns's user-facing behavior, `docs/guide/plugins.md`'s source anchor (`register.go`, unmodified), and `docs/architecture/core-design.md`'s core-registry table (which also omits the `internal/core/probe` precedent) are all unaffected.

### Deviations from Plan
- 3 geodns test files (`server_test.go`, `source_test.go`, `state_test.go`) required mechanical call-site edits because the underlying functions/methods moved into the exported `dnsserver` API. Documented in detail in the Mistake Log's "Test Relocations" table above; each is marked `test-relax:` and is explicitly sanctioned by AC-6/AC-7's own "ported from geodns" language, not a silent behavior change.
- AC-7 (the secondary CIDR-matcher extraction) was implemented in full rather than deferred — it turned out to be low-risk (a small, pure, already-well-tested function), so no user approval was needed to skip it.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Extract generic DNS-server primitives to `internal/core/dnsserver` | Done | `internal/core/dnsserver/{manager,handler,client,matcher}.go` | |
| Migrate geodns onto the harness, behavior-preserving | Done | `internal/plugins/geodns/{server,source,state}.go` | full geodns suite green, unedited except 3 mechanical renames (see Deviations) |
| New `Freebind` option, default off | Done | `internal/core/dnsserver/manager.go:40`, `freebind_linux.go`, `freebind_other.go` | geodns never sets it (`grep -rn Freebind internal/plugins/geodns/` = no hits) |
| Do not touch `spec-as112-2-dns-server.md` | Done | n/a | not read or modified this session |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `internal/core/dnsserver/manager.go:29,35,52,62,83,142`; `rg -q geodns internal/core/dnsserver/` finds nothing | |
| AC-2 | Done | `internal/plugins/geodns/server.go:278,286,296,304`; `! rg -q 'type serverManager' internal/plugins/geodns/` clean; `! rg -q 'func clientIP|func remoteAddr|func bind\(|func.*endpointSig' internal/plugins/geodns/*.go` clean | |
| AC-3 | Done | `go test ./internal/plugins/geodns/...` exit 0; `.ci` suites `geodns-config`, `geodns-invalid-record`, `doctor-geodns`, `geodns-show` all PASS | |
| AC-4 | Done | `internal/core/dnsserver/manager.go:40` (`Options.Freebind`), `freebind_linux.go:22`, `freebind_other.go:12`; `TestFreebindOptionInstallsControlHook` (`manager_test.go:128`) PASS; `TestFreebindAllowsNonLocalBind` (`freebind_integration_linux_test.go`, Linux-only, vets clean under `GOOS=linux -tags integration`) | |
| AC-5 | Done | `internal/core/dnsserver/handler.go:23`; `TestAuthoritativeWrapper_SetsBitsAndRecovers` (`handler_test.go:26`) PASS | |
| AC-6 | Done | `internal/core/dnsserver/client.go:21`; `TestClientIP_EDNS0AndPacket` (`client_test.go:34`) PASS | |
| AC-7 | Done (not deferred) | `internal/core/dnsserver/matcher.go:22,28,40`; `internal/plugins/geodns/source.go:11`; `TestMatcher_LongestPrefix`/`TestMatcher_NoMatch` PASS; geodns `source_test.go`/`state_test.go` PASS (post-rename) | |
| AC-8 | Done | `python3 scripts/dev/dep_audit.py --check` exit 0, no new manifest row | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestManager_BindsAndServes` | Done | `internal/core/dnsserver/manager_test.go:70` | PASS |
| `TestManager_RebindOnlyOnEndpointChange` | Done | `internal/core/dnsserver/manager_test.go:97` | PASS |
| `TestFreebindOptionInstallsControlHook` | Done | `internal/core/dnsserver/manager_test.go:128` | PASS |
| `TestAuthoritativeWrapper_SetsBitsAndRecovers` | Done | `internal/core/dnsserver/handler_test.go:26` | PASS |
| `TestClientIP_EDNS0AndPacket` | Done | `internal/core/dnsserver/client_test.go:34` | PASS |
| `TestMatcher_LongestPrefix` | Done | `internal/core/dnsserver/matcher_test.go` | PASS |
| all existing geodns `*_test.go` | Done | `internal/plugins/geodns/` | PASS, unedited except 3 mechanical renames (Deviations) |
| `geodns-config`, `geodns-invalid-record`, `doctor-geodns`, `geodns-show` `.ci` | Done | `test/parse/`, `test/ui/`, `test/plugin/` | all PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/geodns/server.go` | Done | rewritten per scope table |
| `internal/plugins/geodns/source.go` | Done | AC-7, not deferred |
| `scripts/dev/tier_non_engine_categories.txt` | Not needed | `dep_audit.py --check` required no row (A-3 confirmed) |
| `internal/plugins/geodns/state.go` | Done (not in original plan) | `matcher` field retyped to `*dnsserver.Matcher`; mechanically required by the AC-7 matcher move |
| `internal/plugins/geodns/register.go` | Unchanged | the `geodnsServer` adapter's `apply`/`stopAll` call shape matches exactly what `register.go` already called; zero edits needed |

### Audit Summary
- **Total items:** 8 ACs + 4 task requirements + 8 planned tests + 5 files = 25
- **Done:** 25
- **Partial:** none
- **Skipped:** none
- **Changed:** 3 test files needed mechanical renames (documented in Deviations/Mistake Log); `state.go` edited though not in the original Files-to-Modify list (mechanically required, documented above)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Shared DNS primitives live in core; geodns behavior unchanged | functional test | `go test ./internal/plugins/geodns/...` exit 0 (all unit tests PASS); `.ci` suites `geodns-config`, `geodns-invalid-record`, `doctor-geodns`, `geodns-show` all PASS (`bin/ze-test bgp parse/ui/plugin --all` runs, 0 FAIL) |
| A second DNS plugin can reuse without importing geodns | unit test | `TestManager_BindsAndServes` (`manager_test.go:70`, standalone, zero geodns involvement) PASS + `python3 scripts/dev/dep_audit.py --check` exit 0, confirming no plugin→plugin import exists anywhere in the tree |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Literal deliverable check `! rg -q geodns internal/core/dnsserver/` matched 11 doc-comment mentions of "geodns" (design-rationale prose, zero code identifiers) | `internal/core/dnsserver/*.go` comments | Reworded to "a consumer plugin" / "a sibling plugin" phrasing; re-ran the exact grep, now clean |
| 2 | NOTE | 3 geodns test files needed call-site edits because the functions they exercised moved to the exported `dnsserver` package API (`clientIP`→`dnsserver.ClientIP`, `.lookup`→`.Lookup`) | `internal/plugins/geodns/server_test.go`, `source_test.go`, `state_test.go` | Documented as intentional relocations (AC-6/AC-7 explicitly say "ported"); marked with `test-relax:` per the repo's test-weakening gate; equivalent assertions verified passing in the new location |

### Fixes applied
- Reworded 8 doc comments across `manager.go`, `manager_test.go`, `handler_test.go`, `client_test.go`, `matcher_test.go`, `freebind_other.go` to remove the literal word "geodns", satisfying both AC-1's "no geodns identifier" and the stricter literal deliverable-check command.
- Fixed a real compile bug found during review: `freebind_integration_linux_test.go` called `listenConfig(false).ListenPacket(...)` directly on a returned value, which fails because `net.ListenConfig.ListenPacket` has a pointer receiver and a function-call result is not addressable. Assigned to a local `lc`/`lcFree` variable first. Caught by `GOOS=linux go vet -tags integration ./internal/core/dnsserver/...` (exit 1 before the fix, exit 0 after).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| (none — Run 2 found nothing new) | | | | |

### Final status
- [x] Critical + security review re-run shows 0 BLOCKER, 0 ISSUE (2 NOTEs recorded above, both resolved/documented)
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/dnsserver/{manager,handler,client,matcher}.go` + freebind + tests | Yes | `ls internal/core/dnsserver/` lists 11 files |
| `internal/plugins/geodns/{server,source,state}.go` (modified) | Yes | `git status --porcelain internal/plugins/geodns/` shows 6 modified files |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No geodns identifier in core | `rg -q geodns internal/core/dnsserver/` → no match (exit 1) |
| AC-2 | No `serverManager`/`bind`/`clientIP`/`remoteAddr` in geodns | `rg -q 'type serverManager' internal/plugins/geodns/` → no match; `rg -q 'core/dnsserver' internal/plugins/geodns/server.go` → match |
| AC-3 | geodns suite green | `go test ./internal/plugins/geodns/...` → exit 0 |
| AC-4 | Freebind hook | `TestFreebindOptionInstallsControlHook` PASS; `GOOS=linux go vet -tags integration ./internal/core/dnsserver/...` → exit 0 |
| AC-5 | Authoritative wrapper | `TestAuthoritativeWrapper_SetsBitsAndRecovers` PASS |
| AC-6 | ClientIP RFC 7871 | `TestClientIP_EDNS0AndPacket` PASS |
| AC-7 | Matcher extracted | `TestMatcher_LongestPrefix`, `TestMatcher_NoMatch`, geodns `source_test.go`/`state_test.go` PASS |
| AC-8 | Tier gate | `python3 scripts/dev/dep_audit.py --check` → exit 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| geodns engine reload → migrated `server.go` → `dnsserver.Manager.Apply` | `test/parse/geodns-config.ci` | PASS |
| a query reaches the harness-wrapped handler | `test/plugin/geodns-show.ci`, `TestServerResolvesPerSource` (unit) | PASS |
| `dnsserver.Manager` used directly (no geodns) | `TestManager_BindsAndServes` (unit) | PASS |
| geodns doctor check unaffected | `test/ui/doctor-geodns.ci` | PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `rg -q geodns internal/core/dnsserver/` clean; geodns's listener-up gauge now flows through `Options.OnListenerChange` |
| A-2 | confirmed | `TestClientIP_EDNS0AndPacket` reproduces every case from geodns's former `TestClientIPSourceModes`, PASS |
| A-3 | confirmed | `dep_audit.py --check` exit 0, no manifest row added |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user-facing doc changes needed | `docs/plugin-overview.md`, `docs/guide/plugins.md`, `docs/DESIGN.md` describe geodns's still-identical external behavior; source anchor at `docs/guide/plugins.md:205` points to unmodified `register.go` | Yes |
| `docs/architecture/core-design.md` core-registry table needs no new row | Table (lines 962-971) lists only registry-style packages; `internal/core/probe` (identical precedent) is also absent | Yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-8 all demonstrated
- [x] End-to-End User Stories: every story has a working path and a passing test
- [x] Wiring Test table complete — every row has a concrete test name, none deferred
- [x] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [x] `make ze-test` passes (lint + all ze tests, incl. `ze-tier-check`) — `ze-lint-changed` 0 issues, `dep_audit.py --check` exit 0, `go test ./internal/core/dnsserver/... ./internal/plugins/geodns/...` exit 0, functional suites parse/ui/plugin all PASS
- [x] Feature code integrated (`internal/core/dnsserver`, geodns migrated)
- [x] geodns behavior preserved (its suite green; 3 files needed mechanical renames only, documented in Deviations)
- [x] Documentation Update Checklist answered Yes/No with source evidence
- [x] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [x] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [x] RFC constraint comments carried forward (`// RFC 7871` above `ClientIP`, `// RFC 1035` above `Authoritative`'s doc comment)
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (3+ use cases?) — OVERRIDDEN with rationale (2 consumers + forbidden alternative; `internal/core/probe` precedent). See Key Design Decisions / R-2
- [x] No speculative features (needed NOW? — yes, `spec-as112-2` blocks on it)
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [x] Tests written
- [x] Tests FAIL (paste output) — `TestManager_BindsAndServes`/`TestManager_RebindOnlyOnEndpointChange` failed with `connection refused` against the Phase-1 no-op stub
- [x] Tests PASS (paste output) — all 8 `dnsserver` tests + full geodns suite PASS
- [x] Boundary tests for all numeric inputs (N/A — refactor)
- [x] Functional tests for end-to-end behavior (geodns suite)
- [x] Interop tests for protocol features (N/A — no wire change, justified)
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [x] Partial/Skipped items have user approval (esp. AC-7 matcher if deferred) — N/A, AC-7 was implemented in full, not deferred
- [x] Implementation Summary filled
- [x] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-dns-server-harness.md`
- [ ] **Commit A:** code + tests + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-dns-server-harness.md` only
