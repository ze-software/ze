# Spec: doctor-coverage-6-bgp-engine

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the `recent-events` check and the gate, landed in ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a, which registers the cross-cutting codes `doctor-internal-defect`, `doctor-plugin-setup-failed`, `doctor-plugin-stopped`, `doctor-plugin-request-refused` and `doctor-plugin-stop-cleanup-late`, the transient-code registration that `doctor-recent-events` reads, and the convention that a report-bus raise carries its own fix (the `Issue` fix field and its `recent-events` rendering, its AC-3), which phase P1 here needs first; `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`, the route field on `DoctorCheckContext` (its AC-1, A-2), for R-6 |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`. Source: the BGP engine section of the audit (`plan/audits/doctor-problem-inventory.md`, area A5, scope `internal/component/bgp` minus `internal/component/bgp/plugins/**`).

## Task

Thomas: "Not only the one you have, but all problems. So we will have to audit
the code for all the things we need to be added to it."

The audit placed all 316 problem sites of the BGP engine (315 log or raise
sites, one health row) in 48 problem classes and seven groups of lines that
are not problem classes. 43 classes are gaps, 3 are partly covered (C17, C24,
C47) and 2 are covered (C18, C20a). This spec closes every gap:

| # | Work |
|---|------|
| 1 | `ze doctor` (local) and `show doctor` (every daemon surface) report every BGP engine problem class with its cause in plain words and the exact command that fixes it on its own line |
| 2 | Every Warn and Error line, and every report-bus raise, in the BGP engine names a doctor code that a registered check declares (O-2), so `./le arch doctor-coverage report` counts the area covered |
| 3 | The eleven report-bus codes the reactor raises today become doctor codes, and the `bgp` health row that filters on three of them reads the same declaration |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - outside parties, plugin process boundary, registration
  → Constraint: a check registers from `init()` in the package that owns the dependency it probes; no edit to a central list
  → Constraint: every operator command runs through the daemon on every surface; `show doctor` is that surface for runtime state
- [ ] `ai/doors/bgp-engine.md` - the engine holds no policy and compiles out with `ze_bgp`
  → Constraint: every new check lives under `internal/component/bgp` (gated by `ze_bgp`)
  → Constraint: the reactor never names a plugin package; a check that needs plugin state reads what the plugin registered
  → Constraint: an untagged package test does not link BGP; every test here runs with `-tags ze_bgp`
- [ ] `ai/doors/core.md` - `internal/core` is the leaf tier
  → Constraint: `internal/core/report` and `internal/core/diagnostic` import no component; a BGP code constant in `internal/core/diagnostic/codes.go` is a spelling only, as `CodeDoctorBGPListen` already is
- [ ] `ai/doors/plugin-infra.md`, `ai/doors/bgp-plugins.md`, `ai/doors/commands-cli.md`, `ai/doors/le-tooling.md` - the census readers outside the engine
  → Constraint: no `rpc` field, Stage 1 declaration or SDK callback changes; the bus code is a `show warnings` answer value, not plugin wire
  → Constraint: a `.ci` expected output changed to follow a behavior change is converted in the same commit, never weakened
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the `runtime` phase, the fix line
  → Decision: a `runtime` check receives `Platform` only and reads its owner's live state, never `Tree` (parent spec, Data Flow 1b)
  → Constraint: a transient code counts as covered only when its owner's check declares it; `recent-events` then reports it (parent spec, Data Flow 2a amended)
- [ ] `spec-doctor-coverage-1-audit-and-gate` - Task, O-1 to O-3, AC-6, AC-9, Data Flow amendments
  → Decision: O-2, no transient marker; a transient class is a recent-event row from the report bus
  → Constraint: a health row takes no code and is never refused by the gate; the audit maps it to a check by hand (Data Flow 4b)
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` - sibling child
  → Constraint: it owns no BGP engine row (it names BGP only as one IPsec own-traffic flow); nothing here respecs it
- [ ] `plan/release/fixes/spec-dropped-error-1-bgp-engine.md` - raises 57 Debug-dropped BGP engine errors to Warn
  → Constraint: every line it raises to Warn becomes a problem site the gate judges, and `./le commit create` refuses an added Warn site whose `code` no check declares. That spec names no doctor code today; the code of each such line is the class code this spec assigns (Data Flow map), carried by whichever spec lands second (R-3)
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem first, why, then the fixing command on a line of its own; JSON carries no color

**Key insights:**
- The reactor raises its report-bus codes from string constants (`reportSourceBGP` plus `reportCode*` in `internal/component/bgp/reactor/session_prefix.go` and `internal/component/bgp/reactor/session_health.go`), and the `bgp` health row in `internal/component/bgp/plugin/register.go` repeats three of them as literals, because that package imports neither `bgp/config` nor `bgp/reactor`
- `recent-events` already renders every active warning and every recent error event; a bus code becomes a described row once `ze show diagnostic` knows it, and the gate counts it once the owner's check declares it
- Several classes keep no state today (C23, C26, C31, C33, C40, C43, C49): the log line is the only trace, so a counter comes before the check

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_health.go` - `sessionHealth` raises `session-stuck` after five minutes outside Established, `session-flap` on three drops in five minutes, `eor-timeout` past the GR restart time; `reportCodeOpQueueFull` is declared here
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `reportSourceBGP` ("bgp") and the codes `prefix-threshold`, `prefix-stale`, `prefix-hold`, `notification-sent`, `notification-received`, `session-dropped`, `route-count-anomaly`; `raise*`/`clear*` helpers; `familyKeyString`; `installedPrefixFamilies` and `prefixLimits` Warn on an unrecognized family
- [ ] `internal/component/bgp/reactor/config.go` - `parseFamiliesFromTree` refuses an unknown family with an error before `parsePrefixLimitFromFamily` runs; the peer loop skips a peer on `ErrIncompleteConfig` with a Warn
- [ ] `internal/component/bgp/config/resolve.go` - `requiredFieldGaps`, `CheckRequiredFields` (Warn, never refuses startup), `refuseIncompletePeers` (refuses a reload)
- [ ] `internal/component/bgp/reactor/reactor.go` - `Reactor.peerStartRefusal`, `StartPeers`, `refusePeerStart`, `logPeerHeldByRefusalLocked`, `validatePeerFamilies`, `Reactor.neighborWatch`
- [ ] `internal/component/bgp/plugin/register.go` - `init` registers the `bgp` health row with `report.HealthProbeDegraded` over three literal codes; the start path logs `bgp: start peers failed` and keeps the daemon running
- [ ] `internal/core/report/health_probe.go` - `HealthProbeDegraded` degrades the row while any active warning carries one of the given codes
- [ ] `internal/core/report/report.go` - `Issue{Source, Code, Severity, Subject, Message, Detail, Raised, Updated}`; no fix field today
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` (runtime) rows per (kind, source, code, subject); a row carries the bus code only when `diagnostic.Lookup` finds it; fix is always `show errors` or `show warnings`
- [ ] `internal/component/bgp/config/doctor_checks.go` - `bgp-peer-config`, `bgp-md5`, `bgp-capture-directory` (post-config); `doctorTree`; codes declared locally
- [ ] `internal/component/bgp/config/register.go` - registers `bgp-bfd-strict-engine`, `bgp-filter-references`, `bgp-neighbor-table`; `policyAbsencesFromTree` behind `infra.SetBGPPolicyValidator`, read only by `ze config validate`; `writeGRMarker` logs a write failure and returns nothing
- [ ] `internal/component/bgp/reactor/rfc8212_warning.go` - `RFC8212Absences`, `warnRFC8212AbsentPolicy` (Warn at Established, with a `fix` attribute), `rfc8212Fix`
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - the accept path logs a `createDynamicPeer` failure at Debug and drops the connection
- [ ] `internal/component/bgp/reactor/peer.go` - `Peer.raiseOpQueueFull` and the Warn lines of the drop paths
- [ ] `internal/component/bgp/reactor/gtsm.go` - `publishGTSMKernelState` Warns when `setGTSMPeers` fails
- [ ] `internal/component/gtsm/doctor.go` - `gtsm-kernel-state` (post-config) reads the installed kernel state when the daemon published a set, and route resolvability otherwise; code `doctor-gtsm-kernel-state`
- [ ] `internal/component/bgp/reactor/reactor_address_watch.go` - `addressWatchWorker` keeps its status in a local variable; `addressWatchFix`; `addressSync.apply` logs a read failure at Error
- [ ] `internal/component/bgp/server/validate.go` - `askOpenValidators` Warns when a `validate-open` plugin has no connection or its RPC failed, and leaves it pending
- [ ] `internal/component/bgp/server/events.go` - the UPDATE envelope refusal (Warn) and the event write failure (Error) per process
- [ ] `internal/component/bgp/redistribute/consumer.go` - `BGPConsumer.InjectRoute` Warns on `update-route` failure and on a route that reached no peer
- [ ] `internal/component/bgp/policychain/chain.go` - `Chain.judgeMember` logs a recovered filter panic at Error and fails the member under its `on-error`
- [ ] `internal/component/bgp/rib/commit.go` - `CommitService.buildMPReachNLRI` Warns on a next hop with no wire form or an unusable one
- [ ] `internal/component/doctor/checks_listener.go` - `extractBGPListeners` feeds the pre-start bind probe under `doctor-bgp-listen`
- [ ] `internal/core/diagnostic/codes.go` - exported code spellings, "declared once so that no other package writes it out"; `CodeDoctorBGPListen`; metadata entries
- [ ] `internal/le/arch/doctorcoverage/sites.go` - the gate reads a site's doctor code from its `code` log attribute (`codeKey`)
- [ ] `internal/core/opererr/opererr.go` - a log line carries `why`, `fix` and a caller-supplied `code`

**Behavior to preserve:**
- every existing check, its name, phase and code (`doctor-config-bgp-peer`, `doctor-bgp-md5`, `doctor-bgp-capture-directory`, `doctor-bgp-bfd-strict-without-engine`, `doctor-bgp-neighbor-table`, `doctor-config-reference`, `doctor-bgp-listen`, `doctor-gtsm-kernel-state`)
- the startup tolerance for an incomplete peer (`CheckRequiredFields` never refuses a start) and the reload refusal (`refuseIncompletePeers`)
- the RFC 8212 verdict and its wording, from `RFC8212Absences`
- the `bgp` health row: degraded while a session is stuck, flapping or past its EOR time

**Behavior to change:**
- the eleven report-bus codes are renamed to doctor codes, so `show warnings`, `show errors`, the login banner and the health row show the new spelling
- a report-bus raise carries the command that fixes it, and the `recent-events` row draws that command
- new local (post-config) and runtime checks for the gap classes
- every Warn and Error line of the engine carries a `code` attribute naming its class's doctor code; a `code` attribute that holds a NOTIFICATION or attribute code today is renamed
- the dynamic peer build failure is logged at Warn and counted

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| BGP config-phase checks | `internal/component/bgp/config/doctor_checks.go` `bgpDoctorChecks`; `bfd_strict_doctor.go`, `filter_reference_doctor.go`, `neighbor_table_doctor.go` | Six checks over the config tree, the platform and the filesystem: five `post-config`, and `bgp-filter-references` at `parsed-config`; all through `diagnostic.RegisterDoctorCheck`, so each runs on the local route and, through `doctor.Answer`, on the daemon route | extends: the new local checks (C8, C13, C14 config half, C21, C22, C25 config half, C27) join this table; the covered rows C18 and C20a keep their checks |
| Doctor check registry and phases | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`, `DoctorCheck` | Registered checks with codes, by phase; `runtime` runs only through `doctor.RunRuntimeChecks` on the daemon route | extends: the reactor registers one `runtime` check from its own `init()` through `diagnostic.RegisterDoctorCheck`. Not as a `registry.DoctorCheckDef` on the `bgp` plugin's registration, which `validateDoctorCheckDef` refuses in the `runtime` phase (commit 149497ce12), and not as a Stage 1 declaration: the reactor is never forked (`runBGPEngine` attaches it to the coordinator in process, `coord.AttachReactor`), so the daemon-process read is the live one (A-1) |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` constants; `diagnostic.Register` of `CodeMeta` from each owner's `init()` | One spelling per code, metadata for `ze show diagnostic` | extends: every code below gets one exported spelling there, and its metadata registered by `bgp/config` beside the check that declares it |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `RaiseError`, `Warnings`, `Errors`, `Issue` | Active warnings and a bounded ring of error events, keyed (source, code, subject) | extends: the eleven reactor codes are renamed to doctor codes; a raise carries its fix command through child 12 phase P0a's `Issue` fix field (`Issue` has no fix field today); new transient classes (C36, C40, C46 read and remove) raise here |
| Recent-events check | `internal/component/doctor/checks_recent_events.go` `recentEventsCheck` | One runtime row per bus key, fix `show errors`/`show warnings` | uses: child 12 phase P0a (its AC-3) makes a row draw the raise's own fix command when the raise carries one, keeping `show errors`/`show warnings` otherwise; this spec builds no second convention and only passes a fix on each BGP raise |
| Health probe over bus codes | `internal/core/report/health_probe.go` `HealthProbeDegraded`; `internal/component/bgp/plugin/register.go` `init` | `bgp` row degraded on three literal codes | extends: the row names the exported code constants (journal row `helper-bypassed-by-an-open-coded-copy`, 2026-11-10) |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `./le arch doctor-coverage report` and `check` | Lists uncovered sites; refuses added ones | does not apply as a change: it is the measure; this spec moves the BGP engine's covered count |
| `opererr` log attributes | `internal/core/opererr` `LogAttrs` | `why`, `fix`, and a caller's `code` | extends: a line with a fix logs it through the same `fix` key; the `code` key holds the doctor code only |
| RFC 8212 absence walk | `reactor.RFC8212Absences`, `bgp/config` `policyAbsencesFromTree` | Read by `ze config validate` and the Established Warn | extends: the C22 check calls the same walk, so doctor, validate and the log word the problem once |
| Required-field walk | `bgp/config` `requiredFieldGaps` | Read by `CheckRequiredFields` (log) and `refuseIncompletePeers` | extends: the C13 check calls it |
| Peer start refusal record | `Reactor.peerStartRefusal`, `refusePeerStart` | Holds why `StartPeers` refused, for the run | extends: the runtime check reads it (C12) |
| Neighbor watch state | `Reactor.neighborWatch` (`neighborWatchState`) | Published state of the neighbor watch | extends: the runtime check reads it (C17); the address watch publishes the same shape (C16) |
| GTSM kernel-state check | `internal/component/gtsm/doctor.go` `kernelStateDoctorCheck` | Daemon route reads the installed GTSM state | extends: C19 is covered by it; the reactor's Warn line names `doctor-gtsm-kernel-state` |
| BGP listen probe | `internal/component/doctor/checks_listener.go` `extractBGPListeners`, `doctor-bgp-listen` | Pre-start bind probe from the config | extends: the runtime check compares the reactor's listener set with the addresses the config requires (C15) |
| Reactor metrics | `internal/component/bgp/reactor/reactor_metrics.go` (`asSetWithdrawn` and the prefix metrics) | Per-peer Prometheus counters | extends: C42 reads the count the reactor keeps beside its metric; the new per-peer refusal record (P5) is incremented at the same sites, never instead of a metric |
| Prefix-limit staleness | `internal/component/bgp/reactor/session_prefix.go` `isPrefixDataStale` | Decides the stale verdict at peer add and reload | extends: the C8 local check calls it |
| Dropped-error spec | `plan/release/fixes/spec-dropped-error-1-bgp-engine.md` | Raises 57 Debug-dropped errors to Warn | does not apply as a change here: each line it raises names the class code this spec assigns; Risks R-3 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor` (local route, post-config checks; the daemon's answer when it answers, O-1)
- `show doctor` on every daemon surface (SSH, MCP, REST, gRPC, web), which runs the `runtime` phase in the daemon process

### Transformation Path
1. Local: `runChecks` runs the post-config phase; `bgp/config` checks read the config tree and answer diagnostics with code, severity, message and fix.
2. Runtime: `HandleDoctor` runs `RunRuntimeChecks`; the reactor's `bgp-runtime` check reads the live state the running reactor publishes (a value-type snapshot the reactor fills, never a pointer into reactor data) and answers one row per finding.
3. Transient: a reactor raise on the report bus carries its doctor code and fix; `recent-events` answers the row under that code with that fix.
4. Health: the `bgp` health row's `HealthProbeDegraded` names the exported constants of the three session codes.
5. Gate: every Warn/Error line carries a `code` attribute naming a code that `bgp/config` or the reactor's `runtime` check declares. The gate links no registry: it reads the covered set from the parsed source, every composite literal of a check declaration type (`checkDeclarations`, `coveredCodes` in `internal/le/arch/doctorcoverage/codes.go`; `diagnostic.DoctorCheck` among them), so each `Codes` entry of `bgp-runtime` is a literal or a constant the scan resolves, never a value built at run time.

**Class to code map** (the canonical names; `doctor-bgp-session-stuck` is the one the audit's code table already fixed, every other `doctor-bgp-*` name is new and follows its pattern):

| Class | Code | Route | Declared by |
|-------|------|-------|-------------|
| C1 | `doctor-bgp-session-stuck` (was `session-stuck`) | runtime, bus warning | `bgp-runtime` |
| C2 | `doctor-bgp-session-flap` (was `session-flap`) | runtime, bus warning | `bgp-runtime` |
| C3 | `doctor-bgp-eor-timeout` (was `eor-timeout`) | runtime, bus warning | `bgp-runtime` |
| C4 | `doctor-bgp-notification-sent`, `doctor-bgp-notification-received`, `doctor-bgp-session-dropped` | runtime, bus error event | `bgp-runtime` |
| C5 | `doctor-bgp-open-refused` | runtime | `bgp-runtime` |
| C6 | `doctor-bgp-open-validation-unavailable` | runtime | `bgp-runtime` |
| C7 | `doctor-bgp-prefix-threshold`, `doctor-bgp-prefix-hold` | runtime, bus warning | `bgp-runtime` |
| C8 | `doctor-bgp-prefix-stale` | local and bus warning | `bgp-prefix-data` (post-config) and `bgp-runtime` |
| C9 | `doctor-bgp-route-count-anomaly` | runtime, bus error event | `bgp-runtime` |
| C10 | `doctor-internal-defect` | runtime, bus error event | plugin-infra |
| C11 | `doctor-bgp-op-queue-full` (was `op-queue-full`) | runtime, bus warning | `bgp-runtime` |
| C12 | `doctor-bgp-peers-not-started` | runtime | `bgp-runtime` |
| C13 | `doctor-bgp-peer-incomplete` | local | `bgp-peer-complete` (post-config) |
| C14 | `doctor-bgp-dynamic-peer-refused` | local and runtime | `bgp-dynamic-group` (post-config) and `bgp-runtime` |
| C15 | `doctor-bgp-listener-down` | runtime | `bgp-runtime` |
| C16 | `doctor-bgp-address-watch-stopped` | runtime | `bgp-runtime` |
| C17 | `doctor-bgp-neighbor-table` (unreadable, kept) and `doctor-bgp-neighbor-watch-stopped` | local and runtime | `bgp-neighbor-table` and `bgp-runtime` |
| C19 | `doctor-gtsm-kernel-state` (kept) | through the gtsm check | `gtsm-kernel-state` |
| C20b | `doctor-bgp-bfd-unavailable` | runtime | `bgp-runtime` |
| C21 | `doctor-bgp-bfd-ebgp-no-auth` | local | `bgp-bfd-auth` (post-config) |
| C22 | `doctor-bgp-rfc8212-no-policy` | local and runtime | `bgp-rfc8212` (post-config) and `bgp-runtime` |
| C23 | `doctor-bgp-filter-failing` | runtime | `bgp-runtime` |
| C24 | `doctor-bgp-filter-not-compiled` | runtime | `bgp-runtime` |
| C25 | `doctor-bgp-default-originate-not-sent` | local and runtime | `bgp-default-originate` (post-config) and `bgp-runtime` |
| C26 | `doctor-bgp-send-refused` | runtime | `bgp-runtime` |
| C27 | `doctor-bgp-route-server-not-attached` | local | `bgp-route-server-attach` (post-config) |
| C28 | `doctor-bgp-plugins-not-ready` | runtime | `bgp-runtime` |
| C30 | `doctor-bgp-family-not-negotiated` | runtime | `bgp-runtime` |
| C31 | `doctor-bgp-route-not-sent` | runtime | `bgp-runtime` |
| C32 | `doctor-bgp-redistribute-unsent` | runtime | `bgp-runtime` |
| C33 | `doctor-bgp-forward-suppressed` | runtime | `bgp-runtime` |
| C35 | `doctor-bgp-peer-congested` | runtime | `bgp-runtime` |
| C36 | `doctor-bgp-send-hold-expired` | runtime, bus error event | `bgp-runtime` |
| C37 | `doctor-bgp-read-paused` | runtime | `bgp-runtime` |
| C38 | `doctor-bgp-cache-pressure` | runtime | `bgp-runtime` |
| C40 | `doctor-bgp-malformed-from-peer` | runtime, counter and bus error event | `bgp-runtime` |
| C41 | `doctor-bgp-paths-limit-unenforced` | runtime | `bgp-runtime` |
| C42 | `doctor-bgp-as-set-withdrawn` | runtime | `bgp-runtime` |
| C43 | `doctor-bgp-aspath-edit-skipped` | runtime | `bgp-runtime` |
| C44 | `doctor-bgp-context-exhausted` | runtime | `bgp-runtime` |
| C45 | `doctor-bgp-config-not-in-effect` | runtime | `bgp-runtime` |
| C46 | `doctor-bgp-gr-marker` | runtime, bus error event | `bgp-runtime` |
| C47 | `doctor-bgp-capture-directory` (kept) and `doctor-bgp-capture-stopped` | local and runtime | `bgp-capture-directory` and `bgp-runtime` |
| C48 | `doctor-bgp-recovery-owner-missing` | runtime | `bgp-runtime` |
| C49 | `doctor-bgp-plugin-events-undelivered` | runtime | `bgp-runtime` |
| C50 | `doctor-plugin-setup-failed`, `doctor-plugin-stopped` | runtime | plugin-infra |
| C18, C20a | `doctor-bgp-md5`, `doctor-bgp-bfd-strict-without-engine` (kept) | local | existing checks |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to doctor | the reactor registers its `runtime` check from its own `init()`; the check reads a value-type snapshot the running reactor publishes | No: A-1 |
| BGP plugin to reactor codes | the health row and the reactor both name the constants in `internal/core/diagnostic/codes.go` | No |
| Engine to plugin | no change: no event, RPC or SDK field changes | Yes: no `pkg/` file is in Files to Modify |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`; terms the eleven bus codes (`session-stuck`, `session-flap`, `eor-timeout`, `op-queue-full`, `prefix-threshold`, `prefix-hold`, `prefix-stale`, `route-count-anomaly`, `session-dropped`, `notification-sent`, `notification-received`) and the log key `"code", ` under `internal/component/bgp` minus `plugins/`. The bus-code search returned 53 files; a hit may be the same word in another sense (a YANG leaf, an event name), so the implementer classifies each hit before P1. The log-key search returned 12 reactor sites at the audit and 11 on 2026-11-11 (`forward_build.go` 4, `peer_initial_sync.go` 2, one Debug, and one each in `session.go`, `session_handlers.go`, `announce_build.go`, `session_write.go`, `session_connection.go`); the implementer re-counts at P6. Positive controls: `raisePrefixThreshold` (`internal/component/bgp/reactor/session_prefix.go`) writes `prefix-threshold`; `test/plugin/errors-received-show.ci` reads `notification-received`; `codeKey` (`internal/le/arch/doctorcoverage/sites.go`) reads `code`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| report-bus code in `show warnings` / `show errors` JSON and text, login banner | writer | `internal/component/bgp/reactor/session_health.go`, `session_prefix.go`, `peer.go` raise helpers | the `reportCode*` constants | change: raise under the doctor code constants |
| same | reader | `internal/component/bgp/plugin/register.go` health row | three literals | change: the constants |
| same | reader | `test/plugin/errors-received-show.ci`, `test/plugin/errors-sent-show.ci`, `test/reload/reload-prefix-updated-clears-stale.ci` | expect the old spelling | change: expected output converted in the same commit (`ai/META-DECISIONS.md`, a behavior change converts every user) |
| same | reader | `internal/component/cmd/show/show_test.go`, `internal/component/cmd/show/yang/ze-cli-show-api.yang`, `internal/component/bgp/plugins/cmd/peer/peer.go`, `internal/component/plugin/types_bgp.go`, `internal/test/fixture/misc_fixture_observers.go`, `internal/test/fixture/plugin_fixture_06_events.go`, `internal/test/fixture/plugin_fixture_16.go`, `internal/le/test/integration/stress.go` | hits of the search; classified at P1 | change where the hit is the bus code; preserve where it is another word |
| same | reader | docs: `docs/guide/health-checks.md`, `docs/guide/operational-reports.md`, `docs/guide/bgp-peering.md`, `docs/architecture/api/commands.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/core-design.md` and the other doc hits | prose naming the codes | change in the same commit |
| same | reader | an API client outside the repository reading `show warnings` | pre-release, no consumer of `main` (`ai/META-DECISIONS.md`, "Ze is pre-release") | change, recorded in the commit body |
| `code` log attribute | writer | the reactor lines whose `code` key holds a NOTIFICATION or attribute code (`forward_build.go`, `session_handlers.go`, `session.go`, `announce_build.go`, `session_write.go`, `peer_initial_sync.go`, `session_connection.go` under `internal/component/bgp/reactor/`) | 11 sites of the log-key search today | change: renamed `notify-code` or `attr-code`, so `code` holds the doctor code only |
| `code` log attribute | writer | `internal/core/opererr/opererr.go` `LogAttrs` callers | "a caller-supplied code passed through" | preserve: already a doctor code |
| `code` log attribute | reader | `internal/le/arch/doctorcoverage/sites.go` `codeKey` | the gate resolves `code` as the site's doctor code | preserve: the renames turn the code-dynamic reads into resolved ones |
| `code` log attribute | reader | an operator's log tooling outside the repository | pre-release | change, recorded in the commit body |

### Integration Points
- `internal/component/bgp/config/register.go` `init` - registers the new post-config checks and their code metadata
- the reactor package `init` - registers `bgp-runtime`
- `internal/component/doctor/checks_recent_events.go` - draws a raise's fix (built by child 12 phase P0a, used here)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | local checks run in `runChecks`, runtime checks in `RunRuntimeChecks`, transient rows through `recent-events` |
| No unintended coupling (components stay isolated) | Yes | the doctor component names no BGP symbol; BGP registers into it |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: the RFC 8212 walk, the required-field walk, the staleness verdict and the gtsm check are reused |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the hot-path additions are atomic counter increments; the snapshot is built only when a check asks |
| Registration over hardcoding, outbound | Yes | each check registers from its owner's `init()` |
| Registration over hardcoding, inbound | Yes | searched: `internal/component/doctor` (no BGP name added), `internal/core/report` (no code list), the gate (reads the registry) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The reactor runs in the daemon process, so a `runtime` check registered from the reactor package through `diagnostic.RegisterDoctorCheck` reads the live reactor | `ai/doors/system.md`: BGP is a subsystem whose reactor `internal/component/bgp/plugin/register.go` builds in the daemon (`runBGPEngine` calls `coord.AttachReactor` and `registry.GetPluginServer`, both in-process); the parent spec's A-3 for in-process owners. The plugin-registry route is closed to it: `validateDoctorCheckDef` refuses a `runtime` `registry.DoctorCheckDef` (149497ce12) | the check reads an empty copy | `TestBGPRuntimeCheckReadsPublishedReactor` | unvalidated |
| A-2 | With no reactor published, the BGP engine is either not configured or reported by plugin-infra's `doctor-plugin-setup-failed`/`doctor-plugin-stopped` (C50) | the audit's cross-cutting codes, owned by plugin-infra; the slice left the overlap with `checks_plugin_registry.go` untraced | a configured BGP that never started reads healthy | `TestBGPRuntimeCheckNoReactorAnswersNothing` with child 12's (`spec-doctor-coverage-12-plugin-infra.md`) test over a failed `bgp` engine | unvalidated |
| A-3 | `writeGRMarker`'s seam can return its error to the restart command's caller | `infra.SetGRMarkerWriter(writeGRMarker)` in `internal/component/bgp/config/register.go`; the seam returns nothing today | the write failure reaches only the log | reading the seam's callers at P5 | unvalidated |
| A-4 | Renaming the eleven bus codes breaks no consumer outside the repository that must keep working | `ai/META-DECISIONS.md`: pre-release, no consumer of `main` | none to migrate | census | validated by the owner rule |
| A-5 | The C10 Warn lines are unreachable from configuration, because `parseFamiliesFromTree` refuses an unknown family before the limits are parsed | `internal/component/bgp/reactor/config.go` `parseFamiliesFromTree` | a config path reaches them, and C10 is an operator class | `TestPrefixLimitUnknownFamilyRefusedAtParse` | unvalidated |
| A-6 | A missing encoding context ID (C44) degrades what the peer can forward; the fix command follows from that effect | the slice left the consumer untraced | the row names a wrong remedy | P4 traces `recvCtxID`/`sendCtxID` consumers before writing the row | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Duplicate rows: a bus warning reported by both `recent-events` and `bgp-runtime` | two rows for one stuck peer | `bgp-runtime` declares the bus codes and reports no row for them; it answers only classes whose state is not on the bus |
| R-2 | A hot-path counter adds cost to the UPDATE path | `./le perf` regression | one atomic add per refused or suppressed route, no allocation; measured on a known host before release, never on the Linux development VM |
| R-3 | Collision with `spec-dropped-error-1-bgp-engine` on the same lines | both specs edit one log line | the line's level comes from that spec and its code from the map above; the second to land carries both |
| R-4 | A second convention for a raise's fix | two fix fields or two renderers | resolved: child 12 phase P0a (its AC-3) owns the one convention, the `Issue` fix field and its `recent-events` rendering; this spec's P1 waits for it and builds none (`ai/rules/no-layering.md`) |
| R-6 | One fact drawn twice on `show doctor`: a config-phase check (C8, C14, C22, C25 local halves) runs in the daemon too (`doctor.Answer` runs `runChecks` on the daemon route), beside the bus row or the `bgp-runtime` row under the same code | two rows for one stale family or one EBGP absence | where the two halves report the same subject, the local half answers on route `local` only, through the route field on `DoctorCheckContext` that `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` adds (its AC-1, A-2); halves about different subjects (a group's router-id against per-member refusals) stay as they are |
| R-5 | A bus flood from a misbehaving peer evicts other events from the ring | many C40 events | C40 raises one event per (peer, RFC 7606 action) per session and counts the rest |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show warnings`/`show errors` codes change spelling; a doctor row missing or wrong; a hot-path counter cost. No wire change to peers or plugins |
| How is it reverted? | one commit revert per phase |
| Who else touches this path? | `spec-dropped-error-1-bgp-engine` (same log lines), `spec-bgp-one-off-defects` (owns the `op-queue-full` never-cleared row), `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` (P0a cross-cutting codes, the raise fix), `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` (its AC-16 plans a local check `doctor-bgp-relay-grant-missing` over the same route-server fast-path grant condition as AC-25 here, the A6 grants row beside A5 C27, and moves the reactor's grant predicate to a shared package), `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` (the route field, R-6) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with a peer that never establishes | → | bus warning `doctor-bgp-session-stuck` through `recent-events`, fix drawn from the raise | `test/plugin/doctor-bgp-session-stuck.ci` |
| `show doctor` with a family no decoder serves | → | `bgp-runtime` reading `Reactor.peerStartRefusal` | `test/plugin/doctor-bgp-peers-not-started.ci` |
| `ze doctor` on a config with an EBGP peer and no export policy | → | `bgp-rfc8212` post-config check | `test/ui/doctor-bgp-rfc8212.ci` |
| `ze doctor` on a config with an incomplete peer | → | `bgp-peer-complete` post-config check | `test/ui/doctor-bgp-peer-incomplete.ci` |
| `show health` with a stuck peer | → | `bgp` health row on `doctor-bgp-session-stuck` | `TestBGPHealthRowDegradesOnSessionStuck` |
| `./le arch doctor-coverage report` | → | BGP engine sites resolved against the check declarations the gate parses from source | `TestBGPEngineSitesCovered` |

## Acceptance Criteria

Every finding below is a `Diagnostic` with code, severity, a message that names the peer, group, family or process and the cause in plain words, and `Fix` holding the exact command alone. "Runtime" means the `bgp-runtime` check in `show doctor`; "local" means a post-config check in `ze doctor` and in the daemon's answer.

### Shared mechanisms

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The eleven reactor bus codes | each is raised under its doctor code (Data Flow map), spelled once in `internal/core/diagnostic/codes.go`; `ze show diagnostic <code>` describes each; the `bgp-runtime` check declares each; every census reader converted | none: the old spellings leave the tree (no-layering) |
| AC-2 | A stuck, flapping or EOR-late session | the `bgp` health row degrades, reading the exported constants, the only spelling of the three codes (fixes journal row `helper-bypassed-by-an-open-coded-copy`, 2026-11-10) | none |
| AC-3 | A BGP engine report-bus raise that names a fix | the `recent-events` row carries that fix; a raise with none keeps `show errors` or `show warnings`. The fix field and its rendering are child 12 phase P0a's (its AC-3, `TestRecentEventsRendersRaiseFix`); this AC proves it for the BGP raises | rows from producers outside BGP keep today's fix until they name one |
| AC-4 | Every reactor log line whose `code` attribute holds a NOTIFICATION or attribute code | the protocol value moves to `notify-code` or `attr-code`, and `code` holds the line's doctor code | none |
| AC-5 | The running reactor | publishes a value-type doctor snapshot (per-peer state, refusal record, watch states, listener set, cache occupancy) that `bgp-runtime` reads; with no reactor published the check answers nothing, the guard named in its comment (A-2) | none |
| AC-6 | A route refused, suppressed or skipped for a reason C23, C26, C31, C33, C40, C43 or C49 names | a per-peer (or per-process) refusal record counts it by class and keeps the last reason and time; the hot path pays one atomic add | none |

### Sessions (C1 to C7, C9, C11, C12, C36)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-7 | C1: a peer outside Established for five minutes | row `doctor-bgp-session-stuck` (warning) naming the peer and its FSM state; fix `show bgp peer <address>`, the raise's own fix | a peer held in idle-hold (C7) or administratively down is not stuck (existing `sessionHealth` rule) |
| AC-8 | C2: three Established drops in five minutes | row `doctor-bgp-session-flap` (warning) naming the peer and the drop count; fix `show bgp peer <address>` | none |
| AC-9 | C3: GR families still owing End-of-RIB past the restart time | row `doctor-bgp-eor-timeout` (warning) naming the peer and the pending families; fix `show bgp peer <address>` | none |
| AC-10 | C4: a NOTIFICATION sent or received, or a session dropped without one | recent-event row under `doctor-bgp-notification-sent`, `doctor-bgp-notification-received` or `doctor-bgp-session-dropped` with peer, code, subcode or reason, count and last time; fix `show bgp peer <address>` | none |
| AC-11 | C5: an OPEN refused for a bad peer AS or BGP Identifier | runtime row `doctor-bgp-open-refused` (error) naming the peer, the AS the peer sent and the configured `session asn remote`; fix the `set bgp peer <name> session asn remote <asn>` command | the row clears when the peer reaches Established |
| AC-12 | C6: OPEN refused because a `validate-open` plugin has no connection or its RPC failed | runtime row `doctor-bgp-open-validation-unavailable` (error) naming the plugin and the peer; fix `show plugin` | none |
| AC-13 | C7: prefix warning threshold reached, maximum exceeded, peer held in idle-hold | rows `doctor-bgp-prefix-threshold` (warning) and `doctor-bgp-prefix-hold` (error) naming peer, family, count and limit; fix the `set bgp peer <name> session family <family> prefix maximum <n>` command, or for a hold the command that recreates the peer | none |
| AC-14 | C9: received count dropped over 50% in one UPDATE | recent-event row `doctor-bgp-route-count-anomaly` with before and after; fix `show bgp peer <address>` | tables under 100 prefixes are not judged (existing `minRouteCountAnomalyThreshold`) |
| AC-15 | C11: op queue at `behavior op-queue-size` | row `doctor-bgp-op-queue-full` (error) naming peer and cap; fix `set bgp peer <name> behavior op-queue-size <n>` | the never-cleared warning is fixed by `spec-bgp-one-off-defects`; this AC does not respec it |
| AC-16 | C12: `StartPeers` refused (a configured family with no decoder) | runtime row `doctor-bgp-peers-not-started` (error) naming the refusal and every held peer; fix names the plugin that decodes the family or the `delete` of the family | none |
| AC-17 | C36: send hold timer expired (RFC 9687) | recent-event row `doctor-bgp-send-hold-expired` naming the peer; fix `show bgp peer <address>` | none |

### Config-derived (C8, C10, C13, C14, C21, C22, C25, C27)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-18 | C8: a family's `prefix ... updated` date over 180 days old | local row `doctor-bgp-prefix-stale` (warning) from `isPrefixDataStale`, naming peer, family and date; the daemon's bus warning carries the same code; fix the command that refreshes the limit data | none |
| AC-19 | C10: the two unrecognized-family Warns in `installedPrefixFamilies` and `prefixLimits` | shown unreachable from config by a test (A-5); each line carries `doctor-internal-defect` | none: reaching it is a Ze defect |
| AC-20 | C13: a peer missing remote AS, remote IP or local AS, or `ip dynamic` on a peer | local row `doctor-bgp-peer-incomplete` (error) per gap from `requiredFieldGaps`, naming peer and missing path; fix the `set` of that path | startup tolerance preserved: the daemon still starts |
| AC-21 | C14: a dynamic group whose members cannot be built (no router-id); a member refused at accept | local row `doctor-bgp-dynamic-peer-refused` (error) naming the group and the missing router-id, fix `set bgp router-id <address>`; runtime row naming refusals per group and the last error; the accept-path line is a Warn (fixes journal row `dropped-error-logged-below-warn`, 2026-11-10) | none |
| AC-22 | C21: BFD on an EBGP peer with no authentication | local row `doctor-bgp-bfd-ebgp-no-auth` (warning) naming peer and profile; fix the BFD profile authentication command | an IBGP peer is not reported: RFC 5882 Section 10.2 names EBGP |
| AC-23 | C22: an EBGP peer or group with no import or export policy | local row `doctor-bgp-rfc8212-no-policy` (warning) per absence from `RFC8212Absences`, message and fix from that one value; runtime row for a dynamic member whose OPEN decided EBGP | a dynamic group with no remote AS is judged at runtime only (`policyAbsencesFromTree` rule) |
| AC-24 | C25: default-originate whose filter is raw, a bad `plugin:filter` reference or does not compile; runtime refusal per peer and family | local row `doctor-bgp-default-originate-not-sent` (error) for the config shape; runtime row naming peer, family and refusal; fix names the filter reference to set | none |
| AC-25 | C27: a route-server process whose attach block lacks `receive` or `send [ update ]` on a peer | local row `doctor-bgp-route-server-not-attached` (warning) naming peer and process; fix the `set bgp peer <name> process <name> ...` command | none |

### Listeners, watches and kernel state (C15, C16, C17, C19, C20b)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-26 | C15: a configured local address and port with no running listener | runtime row `doctor-bgp-listener-down` (error) naming address, port and the bind error; fix names the address to configure on an interface or the port to free | `connection local accept false` and `auto` are not listened on (`extractBGPListeners` rule) |
| AC-27 | C16: the address watch stopped | the worker publishes its status as `neighborWatch` does; runtime row `doctor-bgp-address-watch-stopped` (warning) with `addressWatchFix` as fix | none |
| AC-28 | C17: the neighbor watch stopped or its refresh failed | runtime row `doctor-bgp-neighbor-watch-stopped` (warning) from `Reactor.neighborWatch`; the unreadable table stays `doctor-bgp-neighbor-table` | none |
| AC-29 | C19: GTSM kernel state not published | the `publishGTSMKernelState` Warn carries `doctor-gtsm-kernel-state`; the gtsm check reports the missing state | none: covered by `gtsm-kernel-state` |
| AC-30 | C20b: BFD service absent, `EnsureSession` failed, BFD engine stopped, strict hold expiry | runtime row `doctor-bgp-bfd-unavailable` naming peer and strictness (error for strict, warning otherwise); fix `show bfd session` or the `bfd` config command | none |

### Policy, origination and forwarding (C23, C24, C26, C28, C30 to C33, C35, C37, C38, C41 to C45, C47 to C50)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-31 | C23: a filter errors, panics or returns unappliable changes | runtime row `doctor-bgp-filter-failing` (error) naming filter, peer, direction, count and `on-error` outcome; fix `show policy test` for the filter | none |
| AC-32 | C24: a filter with no compiled chain, or a dynamic chain naming an undefined filter | runtime row `doctor-bgp-filter-not-compiled` (error) naming peer, direction and filter; fix the `set policy` or `delete` command | a literal undefined name stays `doctor-config-reference` (local) |
| AC-33 | C26: a route send refused for a missing `attach process ... send` | runtime row `doctor-bgp-send-refused` naming process, peer and count; fix the `set bgp peer <name> process <name> send ...` command | none |
| AC-34 | C28: plugins not ready, startup or API timeout, peer-up barrier timed out | runtime row `doctor-bgp-plugins-not-ready` (error) naming the plugin and the peers whose EOR was sent without its routes; fix `show plugin` | none |
| AC-35 | C30: a configured family not negotiated | runtime row `doctor-bgp-family-not-negotiated` (warning) naming peer and family; fix names the family the peer must enable or the `delete` of it here | a family in `ignore-mismatch` is still reported, as a warning |
| AC-36 | C31: an originated route not sent or not withdrawn | runtime row `doctor-bgp-route-not-sent` naming peer, count by reason (next hop, build, size) and last reason; fix names the next-hop or route command | none |
| AC-37 | C32: a redistributed route reached no peer or `update-route` failed | runtime row `doctor-bgp-redistribute-unsent` naming source and count; fix names the redistribute or attach command | none |
| AC-38 | C33: a forwarded route suppressed toward a destination | runtime row `doctor-bgp-forward-suppressed` naming destination, count by reason and last reason | none |
| AC-39 | C35: a congested destination (write failures, overflow, forced teardown) | runtime row `doctor-bgp-peer-congested` naming peer and overflow depth | none |
| AC-40 | C37: session reads paused at the read buffer budget | runtime row `doctor-bgp-read-paused` naming the starved sessions and the budget | none |
| AC-41 | C38: recent-UPDATE cache over its soft limit, entries force-evicted, a consumer not acking | runtime row `doctor-bgp-cache-pressure` naming entries, limit and the plugin with pending acks | none |
| AC-42 | C40: malformed UPDATEs from a peer | counters per peer and RFC 7606 action; one recent-event row `doctor-bgp-malformed-from-peer` per (peer, action) per session; runtime row with the counts | none |
| AC-43 | C41: PATHS-LIMIT negotiated for a family no decoder counts | runtime row `doctor-bgp-paths-limit-unenforced` naming peer and family | none |
| AC-44 | C42: routes withdrawn for AS_SET (RFC 9774 Section 3) | runtime row `doctor-bgp-as-set-withdrawn` naming peer and count; fix `set bgp peer <name> session as-set accept` | none |
| AC-45 | C43: an AS-path edit skipped | runtime row `doctor-bgp-aspath-edit-skipped` naming peer, edit and count | none |
| AC-46 | C44: encoding context registry exhausted | runtime row `doctor-bgp-context-exhausted` (error) naming each peer whose receive or send encoding context holds an unset ID, and the registry occupancy; fix named from the effect P4 traces (A-6) | none |
| AC-47 | C45: reload failed, rollback error, filter insertions not placed, import invalidation hold expired | runtime row `doctor-bgp-config-not-in-effect` (error) naming what is not in effect; fix `show config diff` then `commit` | none |
| AC-48 | C46: GR marker read or remove failed at start, write failed at restart | recent-event row `doctor-bgp-gr-marker` at start; the write failure returned to the restart command (A-3) | none |
| AC-49 | C47: capture not running for a configured peer | runtime row `doctor-bgp-capture-stopped` naming peer and error | the directory stays `doctor-bgp-capture-directory` |
| AC-50 | C48: no recovery owner, or the owner not attached to the source peer | runtime row `doctor-bgp-recovery-owner-missing` naming peer; fix the attach command | none |
| AC-51 | C49: a plugin does not receive BGP events | runtime row `doctor-bgp-plugin-events-undelivered` naming process and count | none |
| AC-52 | C50: the bgp plugin's SDK run failed | the Error line carries `doctor-plugin-setup-failed`; plugin-infra's check reports it | none |

### Not problem classes, and coverage

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-53 | The 35 programmer-invariant and 6 recovered-panic lines | each carries `doctor-internal-defect` | none |
| AC-54 | The 5 transient-race lines (session already ending) | each carries `doctor-bgp-session-dropped` | none |
| AC-55 | The 3 plugin-misuse refusals | each carries `doctor-plugin-request-refused` | none |
| AC-56 | The 5 policy dry-run lines and the signal status dump | each moves to Info, because the command answer itself shows its content | none: no state stays degraded |
| AC-57 | The shutdown NOTIFICATION budget line, the 2 stop-cleanup lines, the le-only fault injection line | carry `doctor-bgp-session-dropped`, `doctor-plugin-stop-cleanup-late` and `doctor-internal-defect` | none |
| AC-58 | C18 and C20a lines | the md5 Warn carries `doctor-bgp-md5`; the strict-BFD lines carry `doctor-bgp-bfd-strict-without-engine` | none |
| AC-59 | `./le arch doctor-coverage report` after all phases | every site under `internal/component/bgp` minus `plugins/` reads covered; the health row mapped to `bgp-runtime` | none |
| AC-60 | AC-9 of the parent (checks reading daemon state outside runtime) | the audit found none in this area; the six existing checks read config, platform, filesystem or the host neighbor table, each named in Current Behavior | none |
| AC-61 | Every check above | its unit test asserts a non-empty `Fix` naming a `ze` command or a `set`/`delete` path, and goes red when the detection is removed | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPReportCodesAreDoctorCodes` | `internal/component/bgp/reactor/session_health_test.go` | AC-1 | |
| `TestBGPHealthRowDegradesOnSessionStuck` | `internal/component/bgp/plugin/register_test.go` | AC-2 | |
| `TestBGPRaiseCarriesFix` (one BGP raise with its fix, through child 12's renderer) | `internal/component/bgp/reactor/session_health_test.go` | AC-3 | |
| `TestReactorLogCodeAttributeIsDoctorCode` | `internal/component/bgp/reactor/doctor_runtime_test.go` | AC-4 | |
| `TestBGPRuntimeCheckReadsPublishedReactor`, `TestBGPRuntimeCheckNoReactorAnswersNothing` | `internal/component/bgp/reactor/doctor_runtime_test.go` | AC-5, A-1, A-2 | |
| `TestPeerRefusalRecordCountsByClass` | `internal/component/bgp/reactor/doctor_refusals_test.go` | AC-6 | |
| `TestBGPRuntimeOpenRefused`, `TestBGPRuntimeOpenValidationUnavailable`, `TestBGPRuntimePeersNotStarted` | `internal/component/bgp/reactor/doctor_runtime_test.go` | AC-11, AC-12, AC-16 | |
| `TestPrefixLimitUnknownFamilyRefusedAtParse` | `internal/component/bgp/reactor/config_test.go` | AC-19, A-5 | |
| `TestDoctorBGPPrefixStale`, `TestDoctorBGPPeerIncomplete`, `TestDoctorBGPDynamicGroupNoRouterID`, `TestDoctorBGPBFDEBGPNoAuth`, `TestDoctorBGPRFC8212`, `TestDoctorBGPDefaultOriginateShape`, `TestDoctorBGPRouteServerAttach` | `internal/component/bgp/config/doctor_checks_test.go` | AC-18, AC-20 to AC-25 | |
| `TestDynamicPeerRefusalLoggedAtWarn` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-21 | |
| `TestBGPRuntimeListenerDown`, `TestBGPRuntimeAddressWatchStopped`, `TestBGPRuntimeNeighborWatchStopped`, `TestBGPRuntimeBFDUnavailable` | `internal/component/bgp/reactor/doctor_runtime_test.go` | AC-26 to AC-30 | |
| `TestBGPRuntimeFilterFailing` and one test per AC-32 to AC-51 class, named `TestBGPRuntime<Class>` | `internal/component/bgp/reactor/doctor_runtime_test.go` | AC-31 to AC-51 | |
| `TestBGPEngineSitesCovered` | `internal/le/arch/doctorcoverage/check_test.go` (over the real BGP engine tree) | AC-53 to AC-59 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix data age | 0..180 days fresh | 180 days | N/A | 181 days reports AC-18 |
| route-count drop | over 50% at 100 or more prefixes | 50% | 99 prefixes not judged | 51% reports AC-14 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-bgp-session-stuck` | `test/plugin/doctor-bgp-session-stuck.ci` | operator runs `show doctor` on a daemon whose peer never answers and reads the stuck row with its fix | |
| `doctor-bgp-peers-not-started` | `test/plugin/doctor-bgp-peers-not-started.ci` | operator configures a family no plugin decodes and reads why no peer starts | |
| `doctor-bgp-rfc8212` | `test/ui/doctor-bgp-rfc8212.ci` | operator runs `ze doctor` on a config with an EBGP peer and no export policy | |
| `doctor-bgp-peer-incomplete` | `test/ui/doctor-bgp-peer-incomplete.ci` | operator runs `ze doctor` on a peer with no remote AS | |
| `errors-received-show`, `errors-sent-show`, `reload-prefix-updated-clears-stale` | the existing `.ci` files of the census | the renamed codes appear in `show errors` and `show warnings` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | N/A | N/A | no wire-visible change: no message, attribute or capability changes; the work reads and reports state | N/A |

## Files to Modify
- `internal/core/diagnostic/codes.go` - one exported spelling per code in the Data Flow map
- `internal/component/bgp/plugin/register.go` - health row on the constants; the start-refusal and SDK-run lines carry codes
- `internal/component/bgp/reactor/session_health.go`, `internal/component/bgp/reactor/session_prefix.go`, `internal/component/bgp/reactor/peer.go` - raise under the doctor codes, with fixes
- `internal/component/bgp/reactor/reactor.go` - publish the doctor snapshot; refusal lines carry codes
- `internal/component/bgp/reactor/reactor_dynamic.go` - Warn and count the dynamic peer refusal
- `internal/component/bgp/reactor/reactor_address_watch.go` - publish the watch status
- `internal/component/bgp/reactor/gtsm.go` - the Warn carries `doctor-gtsm-kernel-state`
- `internal/component/bgp/reactor/rfc8212_warning.go` - the Warn carries `doctor-bgp-rfc8212-no-policy`
- every other Warn/Error site under `internal/component/bgp/reactor/` the audit lists, by class: a `code` attribute, and the refusal record at the C23, C26, C31, C33, C40, C43 and C49 sites; the 11 `code` attribute collisions renamed (Changed-Wire Census)
- `internal/component/bgp/config/doctor_checks.go`, `internal/component/bgp/config/register.go` - the new post-config checks and their code metadata
- `internal/component/bgp/server/validate.go`, `internal/component/bgp/server/events.go` - C6 and C49 lines and records
- `internal/component/bgp/redistribute/consumer.go` - C32 record and codes
- `internal/component/bgp/policychain/chain.go`, `internal/component/bgp/rib/commit.go` - C23 and C31 line codes
- every census reader (Changed-Wire Census)
- `docs/architecture/doctor-and-health-checks.md` - the BGP checks; `docs/guide/health-checks.md`, `docs/guide/operational-reports.md`, `docs/guide/bgp-peering.md` - the renamed codes and new rows

## Files to Create
- `internal/component/bgp/reactor/doctor_runtime.go` (+ test) - the `bgp-runtime` check and the snapshot it reads
- `internal/component/bgp/reactor/doctor_refusals.go` (+ test) - the per-peer refusal record
- `test/plugin/doctor-bgp-session-stuck.ci`, `test/plugin/doctor-bgp-peers-not-started.ci`, `test/ui/doctor-bgp-rfc8212.ci`, `test/ui/doctor-bgp-peer-incomplete.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `show doctor` exists |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | none added |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | Yes | the four `.ci` files above over `show doctor` and `ze doctor` |
| Pipe completeness | No | `show doctor` pipes unchanged |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | this spec is the BGP engine's doctor coverage |
| Prometheus counters/metrics | No | the refusal record is doctor state; existing metrics unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the doctor feature page under `features/` |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: `show warnings`/`show errors` codes renamed |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md`, `docs/guide/operational-reports.md`, `docs/guide/bgp-peering.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 8212 and RFC 5882 advice is reported, not changed |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the changed files: `docs/features/ai-first.md` (`internal/component/bgp/config/doctor_checks.go`): updated, the BGP readiness checks gain the new rows; `docs/guide/bgp-policy.md` (`internal/component/bgp/reactor/rfc8212_warning.go`): updated, the RFC 8212 absence is also a doctor row; `docs/architecture/bgp/interface-event-reactions.md` (`internal/component/bgp/reactor/reactor_address_watch.go`): updated, the address watch status is published to doctor; `rfc/short/rfc5082.md` (`internal/component/bgp/reactor/gtsm.go`): unaffected, the Warn gains only a `code` attribute and no GTSM behavior changes; the rest derived at implementation by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-6-bgp-engine.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the doc hits of the census show the old codes |

## Implementation Steps

Each phase is one agent of about 60 tool calls. Agents edit only; every test runs in the main thread, `./le job run label <label> command go test -tags ze_bgp <package>`.

| Phase | Owns | Covers | Depends on |
|-------|------|--------|------------|
| P1 codes and bus | `codes.go`, the eleven renames with a fix on each raise, the health row, the census readers, the `bgp-runtime` skeleton declaring the bus codes | AC-1 to AC-4, AC-7 to AC-10, AC-13 to AC-15, AC-17, `test/plugin/doctor-bgp-session-stuck.ci` | `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the `Issue` fix field and its `recent-events` rendering) |
| P2 runtime snapshot | `doctor_runtime.go`, the snapshot, C5, C6, C12 | AC-5, AC-11, AC-12, AC-16 | P1 |
| P3 local checks | `bgp/config` checks for C8, C13, C14, C21, C22, C25, C27; the dynamic accept Warn | AC-18, AC-20 to AC-25, the two `test/ui` files | P1 |
| P4 watches and listeners | C15, C16, C17, C19, C20b, C28, C30, C37, C38, C41, C44, C47, C48 | AC-26 to AC-30, AC-34, AC-35, AC-40, AC-41, AC-43, AC-46, AC-49, AC-50 | P2 |
| P5 refusal record | `doctor_refusals.go`; C23, C24, C26, C31, C32, C33, C35, C36, C40, C42, C43, C45, C46, C49 | AC-6, AC-31 to AC-33, AC-36 to AC-39, AC-42, AC-44, AC-45, AC-47, AC-48, AC-51 | P2 |
| P6 remaining lines | the not-problem groups, `code` collisions, C10, C18, C20a, C50 lines, docs | AC-19, AC-52 to AC-61 | P1 to P5, and `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (codes, transient-code registration, raise fix) |

1. **Phase: Wiring** -- P1: codes, bus renames, the `bgp-runtime` registration, the stuck `.ci`
   - Tests: `TestBGPReportCodesAreDoctorCodes`, `TestBGPHealthRowDegradesOnSessionStuck`, `TestBGPRaiseCarriesFix`
   - Files: `codes.go`, the three reactor raise files, the bgp plugin `register.go` (the fix field in `report.go` and its rendering in `checks_recent_events.go` land first, in child 12 phase P0a)
   - Verify: the `.ci` red against the old code, green after
2. **Phase: P2 to P6** -- as the table above, each test written first and seen red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every class of the Data Flow map has a check that declares its code and a test |
| Correctness | no runtime check reads `ctx.Tree`; no local check reads reactor state; a bus code is reported once (R-1) |
| Naming | every code starts `doctor-bgp-` or is a cross-cutting canonical code; kebab-case log keys |
| Data flow | the doctor component names no BGP symbol; the health row and the reactor share one declaration |
| Rule: user-facing-errors | every finding names the peer or group and the cause; the fix is a command alone |
| Rule: performance | the refusal record is an atomic add on the hot path, no allocation |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| BGP engine fully covered | `./le arch doctor-coverage report` shows every site in the area covered |
| Renamed codes | grep for each old code returns only unrelated words |
| New checks | `ze show diagnostic <code>` for each code |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Information exposure | a finding never carries an MD5 password, a BFD key, or route content beyond prefix counts |
| Authorization | `show doctor` stays behind authz |
| Resource exhaustion | the refusal record is bounded per peer and class; bus raises are per session, not per route (R-5) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A site's code cannot be resolved statically | name it with a constant from `codes.go` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The eleven bus codes are renamed rather than registered under their old names, to keep the naming convention every BGP doctor code follows (`doctor-bgp-*`, Critical Review "Naming"). No registry forces it: `diagnostic.RegisterDoctorCheck` accepts any lower-kebab code (`validateDoctorCheckCodesReg`; `config-plugin-verify` is such a code) and `diagnostic.Register` refuses only a duplicate; the `doctor-` prefix is enforced only on a plugin's `registry.DoctorCheckDef` (`validateDoctorCheckDef`) and a Stage 1 `rpc.DoctorCheckDecl` (`validateDoctorCheckDecls`), neither of which `bgp-runtime` uses.
- C10 is not an operator class: `parseFamiliesFromTree` refuses an unknown family before the prefix limits are parsed, so the two Warns can only be reached through a Ze defect.
- C19 is covered by `gtsm-kernel-state`, which already reads the installed state on the daemon route; only its producer line lacked the code.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-11-10 | `bgp` health row repeats the reactor's report codes as literals |
| `dropped-error-logged-below-warn.md` | 2026-11-10 | dynamic peer build failure at accept logged at Debug |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bus-held classes report through `recent-events`, with the raise's fix | `bgp-runtime` reports them again from reactor state | one row per problem; the bus already holds the state and its lifetime |
| One `bgp-runtime` check in the reactor package | one check per class | one snapshot read per `show doctor`; classes stay separate rows and codes |
| A per-peer refusal record for counter-less classes | a bus raise per refused route | a route-rate raise would flood the bounded ring and cost the hot path |
| Code spellings in `internal/core/diagnostic/codes.go` | a BGP leaf package | the established home of every doctor code spelling (`CodeDoctorBGPListen`); both the bgp plugin registration and the reactor can import `diagnostic` |

## Known Limitations
- The cost of the refusal record is measured on a known host only, never on the Linux development VM (`ai/META-DECISIONS.md`, "Test systems and labs").

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
- [ ] AC-1..AC-61 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package tests with `-tags ze_bgp`: `./internal/component/bgp/reactor/`, `./internal/component/bgp/config/`, `./internal/component/bgp/plugin/`, `./internal/component/bgp/server/`, `./internal/component/bgp/redistribute/`, `./internal/component/doctor/`, `./internal/core/report/`, `./internal/core/diagnostic/`; the named `.ci` files, each run alone: `./le job run label doctor-bgp command bin/le test bgp plugin <exact-test-name>` for the `test/plugin` and `test/reload` files, `./le job run label doctor-bgp command bin/le test runner <exact-test-name>` for the `test/ui` files. No agent runs any `./le verify` command or a whole functional suite
- [ ] One `.ci` at a time through the suite selector: `./le test bgp plugin --pattern doctor-bgp-session-stuck`, `./le test bgp plugin --pattern doctor-bgp-peers-not-started`, `./le test bgp plugin --pattern errors-received-show`, `./le test bgp plugin --pattern errors-sent-show`, `./le test bgp reload --pattern reload-prefix-updated-clears-stale`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, no wire-visible change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-6-bgp-engine.md` only
