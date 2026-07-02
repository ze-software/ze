# Spec: cli-verb-first-followup

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/cli-grammar.md` - BLOCKING verb-first grammar + typed selectors
4. `plan/learned/829-command-verb-first.md` - prior verb-first pass; deferred YANG-tree commands to this follow-up
5. `internal/plugins/debug/{debug.go,register.go}`, `internal/plugins/debug/yang/ze-debug-cmd.yang`
6. `cmd/ze/ze_core_dispatch.go` (yangVerbs:554), `cmd/ze/internal/cmdutil/cmdutil.go` (RunCommand:53), `internal/component/cli/client/main.go` (runBGP:240)

## Implementation Note (2026-07-02): VyOS-aligned pivot

Per user directive "follow docs.vyos.io", the debug grammar was changed from the
invented `enable`/`disable` verbs to **`set`/`delete`** (VyOS models per-subsystem
debug log levels as `set system syslog ... level <l>` configuration, and has no
enable/disable operational verb). This ELIMINATED the new-verb infrastructure
(no `yangVerbs` edit, no verb-root anchor modules, no 5-site verb-list ripple).
Final debug grammar (all existing verbs, offline locals under set/delete/show/clear):
`set debug module <n> [level|flag|scope ...]`, `delete debug module <n> [flag|scope ...]`,
`set debug timeout <d>`, `set debug profile name <n>`, `set debug active name <n>`,
`delete debug profile name <n>`, `clear debug`, `show debug profile [name <n>]`.
The AC table below still uses the old enable/disable wording; the implemented
behavior maps `set debug module`↔enable and `delete debug module`↔disable.

## Task

Convert remaining noun-first CLI commands to verb-first grammar (`ai/rules/cli-grammar.md`)
WITHOUT shadowing existing daemon commands. Three surfaces:

1. **debug** (offline, edits the stored profile in debug.zefs) → verb-first management
   commands using new `enable`/`disable` verbs + existing `set`/`clear`/`delete`/`show`.
   The daemon `show debug` (`ze-debug:debug-state`, live runtime state) is a DIFFERENT
   command and stays untouched.
2. **event**: `event list` → `show event list` (under the existing `show event` container
   that already holds `recent`/`namespaces`).
3. **host / crashes**: add an **offline-fallback mechanism** so `show host` / `show crashes`
   (already daemon verb-first commands) run in-process when no daemon is reachable, then
   delete the noun-first offline `host show` / `crashes show`. This unifies each into one
   verb-first command that works daemon-attached OR offline (crashes especially: you inspect
   crashes when the daemon has died).

**Confirmed decisions (user, 2026-07-02):**
- Full grammar-correct **typed selectors** (`enable debug module <n>`, `set debug profile name <n>`).
- **Hard removal** of noun-first forms; no deprecation aliases. Conscious owner override of
  cli-grammar.md "Backward Compatibility" (A-1).
- `enable`/`disable` as first-class verbs (A-2), chosen over `set ... enabled <bool>`.
- **Do not shadow existing commands** — the governing constraint that reshaped this spec.
- `command` (Meta group) and `plugin` (config-tree `set plugin` collision) are OUT of scope.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli-grammar.md` - BLOCKING verb-first grammar
  → Constraint: first token after the noun MUST be a closed keyword; free-form values (module/profile names) MUST be introduced by a typed selector keyword. Applies to online AND offline commands.
  → Constraint: "Backward Compatibility" requires a 2-cycle deprecation window for released grammar. User overrides (A-1) — hard removal.
  → Constraint: "Engine-Owned Tree Mutation" — `set plugin` is config-tree; do NOT add an operational `set plugin` (why plugin is out of scope). Debug store is NOT config-tree (separate debug.zefs), so operational verbs are legal.
  → Constraint: "YANG Module Ownership" — `event-list` stays owned by the meta plugin module; merge a `show/event/list` subtree onto the show root, do not relocate ownership.
- [ ] `plan/learned/829-command-verb-first.md` - prior verb-first pass
  → Decision: 8 root verbs chosen deliberately (small vocabulary). Adding `enable`/`disable` expands it — accepted (A-2).
  → Constraint: this IS the follow-up 829 named for YANG-tree commands.
  → Constraint (gotcha): `replace_all` on strings with trailing spaces drops the space; secondary dispatch tables exist — grep every dispatch site.
- [ ] `ai/patterns/cli-command.md` - CLI command pattern
  → Constraint: offline `show X` = `registry.MustRegisterLocal("show X", handler)`; RunCommand checks local registry (cmdutil.go:72) BEFORE the daemon. A local at a daemon path therefore SHADOWS it (local-first) — the exact thing we must avoid for host/crashes, which is why they need the fallback mechanism, not a plain local.
- [ ] `ai/patterns/registration.md` - verb-root anchor + subdispatch patterns (for enable/disable anchors)

**Key insights:** Offline dispatch: `yangVerbs = {show,set,clear,request,delete,update,validate,monitor}` (ze_core_dispatch.go:554). For a yangVerb, `cmdutil.RunCommand` (cmdutil.go:53) checks `registry.LookupLocal` FIRST (72) → offline wins; else `cli.Run` → daemon over SSH (runBGP, client/main.go:240; unreachable error at 264/276). A non-yangVerb first word falls to `registry.LookupLocal` at ze_core_dispatch.go:423 (longest-prefix). `readOnlyVerbs = {show,validate,monitor}` (help.go:17). `help_test.go:249` hardcodes the 8-verb list. Daemon `show host` and offline `host show` both call `host.DetectSection` (host-cmd/cmd/show_host.go:30, host/host.go:71) — same data. Daemon `ze-show:crashes` handler in crashes/cmd/register.go:13; offline `RunShow` in crashes.go:20 — both read `crashlog`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/debug/debug.go` - offline debug. `Run` (40) dispatches subcommand map (show/restore/clear/profile/timeout, 31) else `cmdToggle` (74) → `p.ToggleModule` (85). `handleModuleArgs` (104): level/flag/scope. `cmdRestore` (234) loads a profile → `applyProfile`, does NOT persist to default. `cmdProfileSave` (287) persists current→named. `applyProfile` (391) writes slog levels; `SaveProfile` (86) → debug.zefs.
  → Constraint: `debug <module>` is ambiguous today — a subsystem named `show`/`level` collides with the action. Typed selectors fix this.
  → Constraint: two directions on the store — restore (named→live) vs save (current→named) — map to `set debug active name <n>` vs `set debug profile name <n>`.
- [ ] `internal/plugins/debug/register.go` - root `debug` + locals `debug show/clear/restore/profile`. All to be REMOVED.
- [ ] `internal/plugins/debug/yang/ze-debug-cmd.yang` - daemon `show debug` (`ze-debug:debug-state`), LIVE state. UNCHANGED. Its description already distinguishes itself from offline `debug show` (stored).
- [ ] `internal/plugins/host/register.go` - `RegisterRoot("host")` + `host show`/`host` locals (RunShow/RunHint). To be REMOVED; host becomes offline-fallback for `show host`.
- [ ] `internal/plugins/host/host.go` - `RunShow` (52) → `hostinv.DetectSection` (71). Local, no daemon.
- [ ] `internal/plugins/host-cmd/cmd/show_host.go` - daemon `ze-show:host-<section>` (20) → `host.DetectSection` (30). SAME function as offline.
- [ ] `internal/plugins/crashes/crashes.go` - `RunShow` (20) → `crashlog` reads (latest/name/list). Local, no daemon.
- [ ] `internal/plugins/crashes/register.go` - `crashes show`/`crashes` locals. To be REMOVED; crashes becomes offline-fallback for `show crashes`.
- [ ] `internal/plugins/crashes/cmd/register.go` - daemon `ze-show:crashes` handler (13). UNCHANGED.
- [ ] `internal/plugins/meta/yang/ze-command-meta-cmd.yang` - `container event { list }` (44) → `ze-bgp:event-list`; handler `handleBgpEventList` (meta/cmd/help.go:29), path-independent.
- [ ] `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - existing `container event { recent, namespaces }` (276). `show event list` merges here.
- [ ] `cmd/ze/ze_core_dispatch.go` - offline dispatch (yangVerbs 554, LookupLocal 423).
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - `RunCommand` (53), local-first (72), `cli.Run` (118).
- [ ] `internal/component/cli/client/main.go` - `runBGP` (240): loads SSH creds (254), unreachable error (264/276), one-shot `-c` exec (283). Offline-fallback hooks at the unreachable point.
- [ ] `internal/component/command/help.go` - `readOnlyVerbs` (17), `IsReadOnlyVerb` (26).

**Behavior to preserve:**
- Debug `Profile` store semantics (toggle/level/flag/scope/save/restore/clear/timeout) — only grammar changes.
- Daemon `show debug` (live), `show host`, `show crashes`, `show event recent/namespaces` — unchanged handlers/output.
- `handleBgpEventList` return value — only YANG path changes.
- `host.DetectSection` / `crashlog` outputs unchanged.

**Behavior to change:** Command grammar only. Old noun-first forms removed (A-1). New `enable`/`disable` verbs. New offline-fallback path for read-only `show` commands with a registered fallback.

## Data Flow (MANDATORY)

### Entry Point
- Offline: `ze <verb> ...` → `dispatchMain` → ze_core_dispatch.go.
- Daemon: `ze show ...` → RunCommand → (no local) → `cli.Run` → SSH → daemon RPC.

### Transformation Path — debug (offline)
1. `ze enable debug module bgp.reactor` → `enable` (new yangVerb) → RunCommand → LookupLocal `enable debug module` → handler(["bgp.reactor"]) → debug store write + applyProfile.
2. `ze set debug timeout 30m` / `ze clear debug` / `ze delete debug profile name x` / `ze show debug profile [name x]` → existing verb roots, local-first, offline.

### Transformation Path — event (daemon)
1. `ze show event list` → `show` → RunCommand → no local → YANG tree (meta-merged `show/event/list`) → `cli.Run` → daemon → `handleBgpEventList`.

### Transformation Path — host/crashes (offline-fallback)
1. `ze show crashes` with daemon UP → RunCommand → no local → `cli.Run` → daemon `ze-show:crashes`.
2. `ze show crashes` with daemon DOWN → `cli.Run`/`runBGP` detects unreachable (264/276) → `registry.LookupOfflineFallback("show crashes")` → in-process `crashes.RunShow`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| cmd/ze ↔ registry | LookupLocal / LookupRoot / LookupOfflineFallback | [x] :423,:535 (new fallback added) |
| cmd/ze ↔ daemon | cli.Run (SSH) | [x] cmdutil.go:118, client/main.go:274 |
| daemon-unreachable ↔ in-process | new offline-fallback hook in runBGP | [ ] to build |
| meta plugin ↔ show verb root | YANG container-merge (show/event/list) | [ ] to build |

### Integration Points
- `registry.LookupLocal` / `LookupRoot` (ze_core_dispatch.go:423,535) - offline dispatch; new `LookupOfflineFallback` added alongside.
- `cmdutil.RunCommand` (cmdutil.go:53) - local-first bridge; unchanged, but enable/disable become recognized verbs it can be invoked for.
- `runBGP` (client/main.go:240) - daemon connection; new offline-fallback hook at the unreachable branch (264/276).
- `debug.Profile` ops (SaveProfile:86, applyProfile:391) - reused unchanged by the new verb handlers.
- Central `show event` container (show/yang/ze-cli-show-cmd.yang:276) - meta plugin merges `show event list` onto it.

### Architectural Verification
- [ ] Registration over hardcoding — offline-fallback via a registry (RegisterOfflineFallback), not a switch; enable/disable via verb-root anchors + registry.
- [ ] No shadow — host/crashes register a FALLBACK (tried only when daemon down), never a plain local at the daemon path.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Hard removal (no alias) is an accepted owner override of cli-grammar.md Backward Compat | User 2026-07-02 "no back-compat / remove outright" | Rule violation shipped | User confirmation (SCOPE gate) | unvalidated |
| A-2 | Adding `enable`/`disable` verbs is acceptable despite 829's 8-verb minimalism | User 2026-07-02 "enable/disable" | Verb sprawl | User confirmation (DESIGN gate) | unvalidated |
| A-3 | `handleBgpEventList` is invocation-path-independent (only YANG path changes) | help.go:29; returns static data | Handler rework | Read handler body in IMPLEMENT | unvalidated |
| A-4 | Daemon `show host`/`show crashes` and offline read identical data (DetectSection / crashlog) so fallback is transparent | show_host.go:30 == host.go:71; crashes.go:20 | User sees different output by mode | Functional test both modes | unvalidated |
| A-5 | `enable`/`disable` as new yangVerbs is the right integration (vs pure-local via :423) | Uniformity with clear/set; completion/help discovery | Extra core edits unneeded | DESIGN alternative decision | unvalidated |
| A-6 | Offline `host`/`crashes` are not consumed by other code paths that break on removal | grep: only their own register.go references them | Build break | grep + build in IMPLEMENT | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Hard removal breaks functional tests + docs | test/plugin/debug-toggle.ci, test/ui/debug-enable-show.ci, test/parse/cli-host-show-*.ci fail; doc grep hits | Update all .ci + docs in the same commit; enumerate via grep first |
| R-2 | Offline-fallback accidentally shadows the daemon (runs offline when daemon IS up) | daemon-attached test returns offline data | Fallback lookup ONLY after connection failure; test daemon-up returns daemon data |
| R-3 | Adding enable/disable verbs misses a verb-list site (help_test:249, completion generators, valuehints, knownCommands) | test fails / verb missing in completion | grep every `yangVerbs`/verb-list reference; update all |
| R-4 | Debug typed-selector redesign changes flag/scope sub-grammar (toggle→enable/disable) and the stored-view naming (`show debug profile` vs daemon `show debug`) | ambiguous grammar in review | Full sub-grammar table in AC; verify `show debug` (bare) still routes to daemon |
| R-5 | `set debug active name <n>` (restore) semantics (apply without persist vs persist) confuse users | review confusion | Preserve cmdRestore semantics exactly; document in help text |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze enable debug module X` | → | debug enable handler → SaveProfile/applyProfile | `test/plugin/debug-enable.ci` |
| `ze disable debug module X` | → | debug disable handler | `test/plugin/debug-disable.ci` |
| `ze set debug module X level Y` | → | debug set-level handler | `test/plugin/debug-set-level.ci` |
| `ze show debug profile [name X]` | → | debug show-profile handler | `test/plugin/debug-show-profile.ci` |
| `ze clear debug` / `ze delete debug profile name X` | → | debug clear/delete handlers | `test/plugin/debug-clear-delete.ci` |
| `ze show event list` | → | meta `handleBgpEventList` via merged YANG | `test/plugin/show-event-list.ci` |
| `ze show crashes` (no daemon) | → | offline-fallback → crashes.RunShow | `test/parse/cli-show-crashes-offline.ci` |
| `ze show host cpu` (no daemon) | → | offline-fallback → host.RunShow | `test/parse/cli-show-host-offline.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze enable debug module bgp.reactor` | Enables debug for the subsystem in debug.zefs; applies live; idempotent (re-run stays enabled) |
| AC-2 | `ze disable debug module bgp.reactor` | Disables it; idempotent |
| AC-3 | `ze set debug module bgp.reactor level debug` | Sets level; rejects invalid level with error |
| AC-4 | `ze enable debug module X flag F` / `ze disable debug module X flag F` | Adds/removes flag F for module X |
| AC-5 | `ze enable debug module X scope K V` / `ze disable debug module X scope K V` | Adds/removes scope filter |
| AC-6 | `ze set debug timeout 30m` | Sets auto-disable timer; `0` disables; >1440m rejected |
| AC-7 | `ze set debug profile name deep` | Saves current default state as profile `deep` |
| AC-8 | `ze set debug active name deep` | Loads profile `deep` and applies it (restore semantics: preserves cmdRestore behavior) |
| AC-9 | `ze delete debug profile name deep` | Deletes profile `deep`; missing name → error |
| AC-10 | `ze show debug profile` | Lists stored profile names |
| AC-11 | `ze show debug profile name deep` | Shows contents of profile `deep` (table) |
| AC-12 | `ze clear debug` | Clears the default stored profile |
| AC-13 | `ze debug ...` (any old form) | Unknown command (removed); suggestion hint may point to new verb |
| AC-14 | `ze show event list` | Returns event types (was `event list`); `event list` removed |
| AC-15 | `ze show event recent` / `ze show event namespaces` | Still work (unchanged), proving no regression to the sibling container |
| AC-16 | `ze show crashes` with daemon running | Served by daemon `ze-show:crashes` (NOT the offline fallback) |
| AC-17 | `ze show crashes` with NO daemon | Served in-process by the offline fallback; returns same crash data |
| AC-18 | `ze show host cpu` with NO daemon | Served in-process by offline fallback; same DetectSection data |
| AC-19 | `ze host show` / `ze crashes show` (old forms) | Unknown command (removed) |
| AC-20 | `ze show debug` (bare, no `profile`) | Routes to the daemon live-state command — NOT shadowed by any offline local |
| AC-21 | `enable`/`disable` appear in `ze help`, completion, and the verb list; both classified non-read-only | Verb integration complete |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables debug for a subsystem before restarting daemon | `ze enable debug module X` → LookupLocal → debug store → debug.zefs | `test/plugin/debug-enable.ci` |
| 2 | Lists event types to pick a subscription | `ze show event list` → YANG merge → daemon → handleBgpEventList | `test/plugin/show-event-list.ci` |
| 3 | Inspects a crash after the daemon died | `ze show crashes` → daemon unreachable → offline fallback → crashlog | `test/parse/cli-show-crashes-offline.ci` |
| 4 | Reads hardware inventory pre-daemon | `ze show host cpu` → offline fallback → DetectSection | `test/parse/cli-show-host-offline.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDebugEnableDisable` | `internal/plugins/debug/debug_test.go` | enable/disable module writes store, idempotent | |
| `TestDebugSetLevelInvalid` | `internal/plugins/debug/debug_test.go` | invalid level rejected | |
| `TestDebugProfileVerbs` | `internal/plugins/debug/debug_test.go` | set/show/delete profile name | |
| `TestOfflineFallbackRegistry` | `internal/component/command/registry/registry_test.go` | Register/LookupOfflineFallback | |
| `TestEnableDisableAreVerbs` | `internal/component/command/help_test.go` | verb list + non-read-only | |
| `TestRunBGPOfflineFallback` | `internal/component/cli/client/main_test.go` | unreachable → fallback invoked; reachable → not | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| debug timeout (minutes) | 0-1440 | 1440 | N/A (0 disables) | 1441 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `debug-enable`/`disable`/`set-level`/`show-profile`/`clear-delete` | `test/plugin/*.ci` | debug verb-first surface | |
| `show-event-list` | `test/plugin/show-event-list.ci` | `show event list` | |
| `cli-show-crashes-offline` | `test/parse/*.ci` | crashes offline fallback | |
| `cli-show-host-offline` | `test/parse/*.ci` | host offline fallback | |
| updated `cli-host-show-*` → `cli-show-host-*` | `test/parse/` | renamed for new grammar | |
| updated `debug-toggle.ci`, `debug-enable-show.ci` | `test/plugin/`, `test/ui/` | new grammar | |

### Interop Tests
N/A — not a wire-protocol change (CLI grammar only).

## Files to Modify
- `cmd/ze/ze_core_dispatch.go` - add `enable`,`disable` to `yangVerbs` (554); update `knownCommands`
- `internal/component/command/help.go` - confirm `readOnlyVerbs` excludes enable/disable (they mutate)
- `internal/component/command/help_test.go` - update expected verb list (249)
- `internal/component/command/registry/registry.go` - add `RegisterOfflineFallback`/`LookupOfflineFallback`
- `internal/component/cli/client/main.go` - `runBGP`: on daemon-unreachable, try offline fallback before erroring
- `internal/plugins/debug/register.go` - remove old root + locals; register `enable/disable debug module`, `set debug ...`, `clear debug`, `delete debug profile name`, `show debug profile` locals
- `internal/plugins/debug/debug.go` - restructure `Run` dispatch to verb-first typed-selector sub-grammar (preserve Profile ops)
- `internal/plugins/debug/debug_test.go`, `profile_test.go`, `show_test.go` - update for new dispatch
- `internal/plugins/meta/yang/ze-command-meta-cmd.yang` - remove `container event { list }`; add `show event list` container-merge (meta-owned)
- `internal/plugins/host/register.go` - remove `host show`/`host`; register offline fallback for `show host`
- `internal/plugins/crashes/register.go` - remove `crashes show`/`crashes`; register offline fallback for `show crashes`
- `cmd/ze/main.go` - blank imports for new enable/disable anchor packages if needed
- `test/plugin/debug-toggle.ci`, `test/ui/debug-enable-show.ci`, `test/parse/cli-host-show-*.ci` - update to new grammar

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `internal/component/cmd/enable/yang/`, `disable/yang/` (verb-root anchors); meta `show event list` merge |
| YANG validation constraints | N/A | grammar/command tree only; no config leaves added |
| YANG custom validators | Yes (completion) | debug module/profile-name `CompleteFn` for tab-completion (subsystem list, stored profile names) |
| CLI commands/flags | Yes | debug/host/crashes register.go; runBGP fallback |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md` — typed selectors throughout |
| Editor autocomplete | Yes | enable/disable verbs + debug subtree in completion tree |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci`, `test/parse/*.ci` above |
| Pipe completeness | Yes | `show debug profile`, `show event list` produce output → route through ApplyPipes |
| Env var registration | N/A | no new env leaves |
| Doctor check for runtime dependencies | N/A | no new file path/socket/port introduced (reuses debug.zefs, crashlog, /proc) |
| Prometheus counters/metrics | N/A | no new observable state |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (verb-first debug; offline-fallback) |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/guide/debugging-tools.md`, `docs/guide/operations.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (verb taxonomy; event list→show event list), `docs/architecture/hub-api-commands.md` |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | Yes | `docs/guide/debugging-tools.md` |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | No | — |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if fallback test harness needs a no-daemon mode note |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/*` — offline-fallback dispatch; `docs/architecture/api/commands.md` |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | No | — |
| 15 | Registered command/inventory changed? | Yes | `docs/guide/command-reference.md`; `ai/patterns/cli-command.md` "Meta"/inventory list; `docs/features/ai-first.md` |
| 16 | Changed source referenced by doc anchors? | Yes | grep `docs/` for anchors on changed files; update |
| 17 | Docs show examples for this area? | Yes | verify/replace `debug show`/`host show`/`event list` examples across docs |

## Files to Create
- `internal/component/cmd/enable/doc.go` + `internal/component/cmd/enable/yang/ze-cli-enable-cmd.yang` - verb-root anchor
- `internal/component/cmd/disable/doc.go` + `internal/component/cmd/disable/yang/ze-cli-disable-cmd.yang` - verb-root anchor
- `test/plugin/debug-enable.ci`, `debug-disable.ci`, `debug-set-level.ci`, `debug-show-profile.ci`, `debug-clear-delete.ci`, `show-event-list.ci`
- `test/parse/cli-show-crashes-offline.ci`, `test/parse/cli-show-host-offline.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review + fixes | Checklists below |
| 11-14. Deliverables/security/present | Checklists below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — add `RegisterOfflineFallback`/`LookupOfflineFallback` to the registry; add enable/disable verb-root anchors + `yangVerbs` entries; write failing wiring tests. Verify entry points reachable, feature stubbed.
2. **Phase: debug grammar** — restructure `debug.go` Run into verb-first typed-selector dispatch; rewrite `register.go` to register the new locals; preserve Profile ops. Tests: debug-* unit + .ci.
3. **Phase: offline-fallback** — hook `runBGP` to try `LookupOfflineFallback` on daemon-unreachable; register host/crashes fallbacks; delete offline `host`/`crashes` roots+locals. Tests: cli-show-{crashes,host}-offline, R-2 daemon-up test.
4. **Phase: event** — meta YANG: remove `event/list`, add merged `show/event/list`. Test: show-event-list.ci; verify show event recent/namespaces unaffected.
5. **Functional tests + doc updates** — update renamed .ci; sweep docs per checklist (grep `debug show`/`host show`/`event list`).
6. **Full verification** — `make ze-verify`.
7. **Complete spec** — audit tables + `plan/learned/NNN-cli-verb-first-followup.md`; two commits.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation file:line |
| No shadow | `show debug` (bare), `show crashes`, `show host` still hit the DAEMON when up; offline only on fallback (AC-16, AC-20, R-2) |
| CLI grammar | Typed selectors everywhere (`module <n>`, `profile name <n>`); no untyped positional |
| Registration over hardcoding | Offline-fallback via registry; enable/disable via anchors; no new switch case |
| Old code deleted | No `debug show`/`host show`/`crashes show`/`event list` registrations remain (grep) |
| Verb-list completeness | enable/disable added to every verb-list site (help_test, completion, knownCommands, valuehints) |
| Data flow | Debug store ops unchanged; only grammar/dispatch changed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| enable/disable are verbs | `grep -n 'enable' cmd/ze/ze_core_dispatch.go` (yangVerbs); `ze help` shows them |
| Old forms gone | `grep -rn 'debug show\|host show\|crashes show\|"event"' internal/plugins/*/register.go internal/plugins/meta/yang` returns nothing |
| Offline fallback works | `test/parse/cli-show-crashes-offline.ci` passes |
| No shadow | daemon-up functional test returns daemon data for show crashes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | debug module/profile names validated (no path traversal into debug.zefs; existing SaveProfile guards preserved) |
| Offline fallback scope | Fallback only for read-only `show` commands with an explicitly registered handler; never for mutating verbs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read Current Behavior → RESEARCH |
| Fallback runs when daemon up | Fix runBGP ordering (fallback strictly after connection failure) |
| 3 fix attempts fail | STOP, report, ask user |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Do NOT convert `debug show`→`show debug` | Naive rename | `show debug` already exists (daemon live state, `ze-debug:debug-state`); would shadow it. Offline stored view → `show debug profile [name]` |
| host/crashes: offline-fallback mechanism, delete noun-first | (a) leave noun-first; (b) plain local (shadows daemon) | User chose unify; crashes MUST work with no daemon; local-first would shadow — fallback tried only on connection failure avoids shadow |
| `event list`→`show event list` (under existing `show event`) | new `show events` (plural) | Existing `show event recent/namespaces` container; plural would fork the noun |
| `enable`/`disable` as first-class yangVerbs + anchors | pure-local via :423 (no core edit); `set ... enabled <bool>` | User chose enable/disable; first-class gives completion/help uniformity like clear/set |
| Typed selectors (`module <n>`, `profile name <n>`) | untyped | BLOCKING cli-grammar.md; resolves `debug show` collision |
| Hard removal, no alias | RegisterDeprecated | Owner override (A-1) |
| `command`/`plugin` OUT of scope | convert them | `command` cohesive Meta group; `set plugin` collides with config-tree plugin node |

## Known Limitations
- Offline-fallback covers only read-only `show` commands that explicitly register a fallback (host, crashes). Other daemon commands still require a daemon.
- The debug offline stored-view is `show debug profile [name <n>]` (verbose vs old `debug show`), a deliberate cost of not shadowing daemon `show debug`.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | (pending /ze-review) | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
<!-- Filled during IMPLEMENT -->

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
- [ ] AC-1..AC-21 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] No-shadow proven end-to-end (daemon-up test)
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (debug timeout 0..1440)
- [ ] Functional tests for end-to-end behavior (debug verbs, show event list, offline fallback)
- [ ] Interop tests for protocol features (N/A — CLI grammar only)
- [ ] Goal Validation: no-shadow proven (daemon-up returns daemon data)
