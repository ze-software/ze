# Spec: firewall-vpp-apply-before-connect

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | An operator who boots with `firewall { backend vpp; }` can have the boot refused with `firewall-vpp: vpp not reachable` when VPP takes longer to accept API clients than the firewall's config stage waits: a shipped-daemon defect on the VPP data plane. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Source: split out of `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`
(child 1 of `plan/release/fixes/spec-single-owner-lifecycle.md`) on 2026-11-11.
That child carried this item as its AC-9 and phase 6. It left because the fix
decides when "config applied" is true for a data-plane backend, which is a
forwarding design decision and not a plugin-process lifecycle defect.

The journal row (`plan/journal/plugin-startup-barrier-deadlock.md`, 2026-08-10)
records two things under one symptom. With `firewall { backend vpp; }`:

1. **The barrier deadlock.** The firewall waited for VPP inside its config
   stage while the `vpp` plugin waited at the same tier's barrier, so VPP never
   connected and both failed after 5 s with `stage stalled ... waiting_for=Capability`
   and `firewall-vpp: vpp not reachable`. The firewall has since declared an
   ordering edge to `vpp` (Current Behavior), so this half is probably gone.
   **Unverified: no run since the edge landed was observed by the author of
   this spec.** The lab run that settles it is under "Evidence owed before
   design".
2. **The residual.** What an operator meets at boot today, read at the
   producers: the firewall's config stage loads the vpp backend and calls its
   apply. The apply polls up to 5 s for the VPP component to publish its
   connector, then up to 5 s more for the connection to be established. VPP is
   started by the `vpp` plugin from its started callback, in a background
   goroutine, and that goroutine allows itself up to ten connect attempts one
   second apart under a 30 s bound. When VPP is not connected inside the
   firewall's wait, the firewall's config stage fails with
   `firewall config apply: ... firewall-vpp: vpp not reachable`, which is a
   startup stage failure. When VPP does connect in time, the boot has still
   spent that wait inside a startup stage, holding the firewall reconcile lock
   that every other firewall table owner needs.

Goal: a boot with `firewall { backend vpp; }` succeeds whether VPP connects
before, during or after the firewall's config stage, the operator can see
whether the tables are in the data plane, and a table VPP cannot hold is still
refused at verify. How that is achieved is the open design question below; this
skeleton does not choose.

The same question stands for every VPP backend that applies config, and this
spec owns all three (2026-11-11, "Owned journal rows"). The traffic vpp backend
waits in its config stage exactly as the firewall does and fails the stage when
VPP has not connected. Static routes choose their backend once, before the
handshake: with no VPP connector published yet they keep the kernel backend for
the daemon's life, so a named-table static route is written to the kernel by
netlink (main-table routes go through the Loc-RIB and are not affected).
Whether such a kernel route forwards anything on a VPP deployment was not
confirmed. The file keeps its name because other records cite its path.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/forwarding.md`, `ai/doors/system-plugins.md`, `ai/doors/plugin-infra.md`
  → Constraint: "A backend that cannot apply a config exactly refuses it at verify. It never approximates" (`ai/doors/system.md`). Any answer to the open question says what a successful commit means while the tables are not yet in VPP.
  → Constraint: the report bus is process-local and so is the firewall table registry (`ai/doors/forwarding.md`); the VPP connector is a package-level reference in the VPP component, so the backend reaches it only in process.
  → Constraint: an engine call or a wait for another plugin from the config stage stalls or is refused (`ai/doors/plugin-infra.md`, Traps, which cites this journal row).
  → Constraint: the engine never names a plugin; the fix stays in the firewall component, its vpp backend and the declared dependency edge.
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md`, "Startup ordering"
  → Constraint: the page states the edge "orders the firewall after VPP whenever both components are selected, so VPP completes its startup tier before firewall configuration applies ACLs", and that "The VPP manager starts from `OnStarted`". It does not state what the apply does while VPP is still connecting. The rest of the page was not read by the author of this spec.
- [ ] `docs/architecture/plugin/plugin-system.md`, "Startup phases and callbacks"
  → Constraint: "A plugin's `OnStarted` therefore fires after its own handshake, and before the plugins of a later phase are loaded." Whether it also fires before the next tier of the same phase begins its config stage was not read at the producer.
- [ ] `ai/rationale/exact-or-reject.md`, `docs/architecture/core-design.md` sections 14a and 14b - not read by the author of this spec; the design pass reads them first.

### RFC Summaries (Scope: protocol)
N-A: Scope is plugin.

**Key insights:** (minimal context to resume after compaction)
- The interface component already records an apply the vpp backend could not take and performs it on the VPP connected event; the VPP FIB plugin already programs on that event. The firewall and traffic vpp backends wait in the config stage instead.
- The firewall apply runs under the process-wide firewall reconcile lock, which copp's apply takes through the same `ApplyAll` (read); policy-routes and ddos-local as further owners are unverified.
- The deployment lab's firewall scenario runs VPP in external mode, so it does not cover the case where ze starts VPP itself.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - `Apply` takes the backend mutex, calls `waitConnector` under a 5 s context (`waitConnectedTimeout`), then `Connector.WaitConnected` with the same 5 s, then opens a channel and reconciles. Both wait failures return `firewall-vpp: vpp not reachable`. `waitConnector` polls the connector function every 50 ms (`waitConnectorPoll`); the function is the VPP component's `GetActiveConnector`. `Apply` builds its own background context and takes none from its caller, so a daemon stop does not cut the wait short.
- [ ] `internal/plugins/firewall/vpp/timeout_linux.go` - the reply deadline for one VPP request defaults to 10 s (env key `ze.firewall.vpp.reply-timeout`). `asDataplaneTimeout` deliberately does not tag an absent VPP as a dataplane timeout: "Not there" is not "not answering". Its comment states that `firewall.ApplyAll` holds the process-wide reconcile lock across `Backend.Apply`, so one stalled apply stalls every firewall owner.
- [ ] `internal/component/firewall/engine.go` - `runEngine` registers `OnConfigure`, which validates, calls `LoadBackend` for the configured backend, then `registerConfigTables`, which registers the engine's two table owners and calls `ApplyAll`. An error returns `firewall config apply: ...` from the config stage. On success it logs `firewall config applied`. A reload goes through `OnConfigVerify` and `OnConfigApply`, and the apply reaches the same `registerConfigTables` inside a journal whose rollback re-applies the previous config.
- [ ] `internal/component/firewall/registry.go` - `ApplyAll` takes the reconcile lock, merges every owner's tables, loads the OS default backend on demand when none is loaded, runs the backend's verifier over the merged set, calls `Backend.Apply`, and records the merged set as last applied only when the apply succeeded.
- [ ] `internal/component/firewall/register.go` - the registration declares `OptionalDependencies` on `vpp` and a `ConfigNeed` (`firewall backend` with value `vpp` pulls provider `vpp`). Its comment says the optional edge orders the firewall after vpp whenever both run, "as the StartAfter it replaces did".
- [ ] `internal/component/vpp/register.go` - `runVPPEngine`: `OnConfigure` only parses and validates settings. `OnStarted` creates the manager and runs it in a goroutine, then returns. Nothing in the config stage starts VPP.
- [ ] `internal/component/vpp/vpp.go` - the manager's `Run` publishes the connector (`setActiveConnector`) after writing the startup file and binding the NICs, and clears it on return. `runOnce` starts the VPP binary unless the settings say external, then calls the connector's `Connect` with ten attempts one second apart under a 30 s context, and on success emits `connected` on the first run and `reconnected` on later ones. `GetActiveConnector` returns the package-level reference, nil when the manager is not running.
- [ ] `internal/component/vpp/conn.go` - `WaitConnected` polls `IsConnected` every 50 ms until its timeout or its context ends. `NewChannel` answers a not-connected error when the connection is not established.
- [ ] `internal/core/vpp/events/events.go` - namespace `vpp`, events `connected`, `disconnected`, `reconnected`.

**Commits verified with `git show --stat` and the diff of the firewall registration:**
- 13c3d6f5db (2026-10-06, "fix: reconcile persistent VPP firewall ACL ownership") adds a start-after edge to `vpp` in the firewall registration.
- 30422ebe30 (2026-10-24, "plugin: refuse configs needing a provider the build lacks") replaces that edge with the optional dependency and the config need.
- Both are later than the journal row (2026-08-10).

**Behavior to preserve:** (unless the user explicitly said to change it)
- A table the vpp backend cannot hold is refused by the backend verifier before any apply.
- An absent VPP is not reported as a wedged dataplane (`asDataplaneTimeout`).
- `firewall { backend nft; }` and a configuration with no firewall block do not load or wait for VPP.

**Behavior to change:** (only what the user asked for)
- The boot outcome with `firewall { backend vpp; }` when VPP is not yet connected, as the open design question settles it.

## Existing Mechanisms (MANDATORY)

A design starts from the mechanisms Ze already has. Each row below was read at
the producer named, on 2026-11-11. The last column says what the mechanism
offers each candidate answer; this skeleton chooses none.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interface component: owed apply, performed on the connected event | `internal/component/iface/config_apply.go`, `applyConfig`, `applyOwedConfig`, `owedApply`; `internal/component/iface/register.go`, `subscribeReconcileOnReady`; `internal/plugins/iface/vpp/ifacevpp.go`, `ensureChannel` | The vpp backend answers `ErrBackendNotReady` when no connector is published or it is not connected, without waiting. `applyConfig` then records the whole apply as owed, returns no error, and logs at Info that the committed config "is not in the dataplane yet". The engine subscribes to `connected` and `reconnected`; the handler only signals a worker, which performs the owed apply. After subscribing it signals once more if an apply is owed, because the connection can complete between the apply and the subscription. A failed owed apply stays owed | Undecided. It is the existing precedent for "record and program on the connected event", including the commit that reports success before the dataplane holds the config |
| VPP FIB plugin: writer opened on the connected event | `internal/plugins/fib/vpp/register.go`, the `OnStarted` callback and its `restart` function | Declares a hard dependency on `vpp`. In `OnStarted` it subscribes to `connected` and `reconnected`, then tries to open its writer from `GetActiveConnector`. When that fails it keeps no writer, logs at Error that no route is programmed into VPP, keeps the cause for `show fib vpp`, and the two events run the same function again, whose replay installs every route. Its config callbacks were not read | Undecided. Second precedent for programming on the event, with the cause kept for an operator command |
| Traffic component and its vpp backend: wait in the config stage | `internal/component/traffic/register.go`, `runEngine` `OnConfigure`; `internal/plugins/traffic/vpp/backend_linux.go`, `Apply`, `waitConnector` | The same shape as the firewall: `OnConfigure` loads the backend and calls `Apply`, which waits up to 5 s for the connector and up to 5 s for the connection, and fails the config stage otherwise. It differs in one way: `Apply` takes the plugin's run context, so a stop cuts the wait short. Declares the same optional dependency and config need on `vpp` | Undecided. It is the sibling with the same residual, owned by this spec; whichever answer is chosen says whether traffic follows it |
| Static routes: backend chosen once at plugin start | `internal/plugins/static/register.go`, `runStaticPlugin`; `internal/plugins/static/backend_linux.go`, `newStaticBackend`; `internal/plugins/static/backend_vpp_linux.go`, `newVPPStaticBackend` | Before its handshake the plugin asks for a VPP backend, gets one only if a connector is published and a channel opens, and otherwise uses the kernel backend. It does not wait. Its registration declares no edge to `vpp`. No file of the package names a VPP connection event | Does not apply as a model: it neither waits, refuses nor reconciles. It is the third owned defect: only a named-table route uses this backend (`applyProgrammed`, `internal/plugins/static/locrib.go`), and nothing reselects the backend after start |
| IKE VPP dataplane: refuses when there is no connector | `internal/component/ike/dataplane/vpp.go`, `newVPPBackend` | Answers `ErrNotSupported` when no connector is published; does not wait. Its caller was not read | Does not apply directly; it is the existing "refuse" shape, at backend construction rather than at verify |
| VPP connection events | `internal/core/vpp/events/events.go`; emitted by `internal/component/vpp/vpp.go`, `runOnce`, after `Connect` succeeds | `connected` once per daemon life, `reconnected` after each later connect, `disconnected` on loss | Candidate "record and program on the connected event" consumes these; the other candidates do not need them |
| Dependency edge and config need | `internal/component/firewall/register.go`, `OptionalDependencies`, `ConfigNeeds` | Orders the firewall after `vpp` and pulls `vpp` when the backend leaf selects it | Every candidate keeps it. Candidate "start VPP earlier" relies on it to mean more than ordering |
| Backend verifier (exact or refuse) | `internal/component/firewall/registry.go`, `ApplyAll`, the verifier captured with the active backend | Refuses a merged table set the backend cannot hold, before `Backend.Apply` | Every candidate keeps it. It judges the tables, not whether VPP is reachable |
| Firewall table registry and reconcile lock | `internal/component/firewall/registry.go`, `RegisterTables`, `ApplyAll` | `ApplyAll` takes `reconcileMu` on entry and holds it across `Backend.Apply`, so the vpp backend's wait is spent under it (read in `ApplyAll`). Other owners reach the same call: `applyCoppPolicy` (`internal/plugins/copp/register.go`) registers its table and calls `ApplyAll`, and the flowspec firewall engine does the same (call sites seen, function not read). That policy-routes and ddos-local are owners too is UNVERIFIED: it comes from comments only. The last-applied snapshot states what the data plane holds and is replaced only by a successful apply | Every candidate must answer for all owners, not only the firewall engine's config stage: each of them reaches the same `Apply`. A candidate that returns before programming also says what the last-applied snapshot holds meanwhile |

## Open design question

**When VPP is not yet connected and the firewall is asked to apply, what does
the firewall do, and what does a successful boot or commit then mean?**

The question is one question for the firewall, traffic and static backends: each
is asked to put config into VPP before VPP is connected, and today they give
three different answers (wait and fail, wait and fail, silently use the kernel).
The design starts from the mechanism Ze already has for it, the interface
component's owed apply reconciled on the connected event (Existing Mechanisms,
first row), and says for each of the three whether it extends that mechanism or
why it cannot. Starting there is not choosing it: the candidates below remain
open, and the answer may differ per backend if the design shows why.

Four candidate answers. None is chosen here; the design pass chooses, with the
owner where the choice is his.

| Candidate | What it is | What it forecloses or costs |
|-----------|------------|-----------------------------|
| Wait in the config stage, as today | Keep the bounded wait in `Apply`, possibly with a bound tied to the VPP connect bound and a context so a stop cuts it | "Config applied" keeps meaning "in the data plane". Forecloses a boot that succeeds when VPP is slower than the bound, keeps a wait for another plugin inside a startup stage (the trap the plugin infrastructure door names), and keeps the reconcile lock held for the wait, stalling the other table owners |
| Refuse at verify | A config that selects the vpp backend is refused when VPP is not connected | Matches "exact or refuse" literally, but makes reachability a config property. Forecloses boot with VPP and the firewall in one config, unless boot is exempted, and an exemption is one of the other candidates under another name |
| Record and program on the connected event | The apply records the desired tables, returns, and the tables are programmed when VPP reports connected; the pending state is visible to the operator | The shape the interface component and the VPP FIB plugin already have. Forecloses "a successful commit means the tables are in VPP": between the commit and the connect, traffic is forwarded without the firewall's rules, which for a packet filter is a security statement the design must make explicitly. Needs an answer for a programming failure after the commit already succeeded |
| Start VPP earlier | The `vpp` plugin connects before its startup tier completes, so a later tier's config stage always finds VPP connected or the boot already failed | Keeps "config applied" exact with no pending state. Forecloses a boot that completes while VPP is still starting, moves the 30 s connect bound into the `vpp` plugin's own startup stage and its stage timeout, and changes what every other VPP consumer may assume |

Child 1 had proposed the third candidate, with this draft criterion: "Boot with
`firewall { backend vpp; }` and VPP not yet connected: the firewall's config
stage returns within its stage timeout with the desired tables recorded, the
tables are programmed when VPP reports connected, and `ze doctor` names the
tables as pending until then. A table VPP cannot hold is still refused at
verify, because exact-or-reject is decided by the backend verifier, not by
reachability." Its stated reason: "A wait in the config stage is the defect
class of the row; reachability is not a config property a verifier can judge."
That is recorded here as input, not as a decision. The tests it named for that
candidate were `TestFirewallVPPConfigureRecordsWithoutWaiting` and
`TestFirewallVPPProgramsOnConnected` in `internal/plugins/firewall/vpp/apply_test.go`
and `test/vpp/firewall-vpp-boot-before-connect.ci`; the pages it expected to
edit were `docs/architecture/firewall/fw-6-firewall-vpp.md` and the VPP doctor
row.

## Evidence owed before design

The barrier-deadlock half of the journal row is unverified either way. The
main thread runs these; no agent runs a lab.

| Run | What to boot | What to observe | What it settles |
|-----|--------------|-----------------|-----------------|
| The deployment lab's firewall scenario | `./le test deployment vpp` (the row's `./le deployment vpp-test` is a retired name for it); its `firewall-acl` scenario boots ze with VPP in external mode and `firewall { backend vpp; }` with rules | The daemon log: `firewall config applied` present, and neither `stage stalled` with `waiting_for=Capability` nor `firewall-vpp: vpp not reachable` | Whether the deadlock is gone when VPP is already running. If the stall line still appears, the row is still a tier defect and the design is a tier fix first |
| A cold start with ze owning VPP | ze with `vpp { enabled true; }` not in external mode, and `firewall { backend vpp; }` with one table, on Ze's kernel with VPP installed. No existing test was found that does this for the firewall: `test/vpp/` holds no firewall case | The time between the `vpp` plugin's started callback and `vpp: GoVPP connected`, and whether the firewall's config stage returns before or after it, with `firewall config applied` or `vpp not reachable` | Whether the residual is reachable in practice: whether a normal VPP start fits inside the firewall's 5 s plus 5 s |

If the first run shows no stall, the deadlock half is recorded in the journal
row as fixed by 13c3d6f5db with that run as evidence, and the row stays open
for the residual until this spec closes.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: for example "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Changed-Wire Census

**Search evidence:** [scope, terms, positive control and source findings]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding and actual encode/write or decode/read path] | [change or preserve, with reason and proof] |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

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
| A-1 | The barrier deadlock of the 2026-08-10 row no longer occurs, because the firewall is ordered after `vpp` | The firewall registration declares the edge; 13c3d6f5db and 30422ebe30 are both later than the row (carried from child 1, where it was A-5, and re-verified at the diff) | The row is still a deadlock and needs a tier fix before any of the four candidates | The first run of "Evidence owed before design" | unvalidated |
| A-2 | The `vpp` plugin's started callback runs before the firewall's config stage begins | The plugin-system page says `OnStarted` fires after the plugin's own handshake; the firewall page says VPP "completes its startup tier before firewall configuration applies ACLs". The startup producer was not read for when the callback fires relative to the next tier | The connector is not published during the firewall's first 5 s wait for a reason no bound fixes | The design pass reads the server's startup phase and the SDK's started dispatch | unvalidated |
| A-3 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [what triggers the behavior] | [positive observable outcome] | [what remains permitted and why, or why none] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Doctor code per problem class | | For each problem class the feature can produce (refusal, `Warn`/`Error` log line, transient report-bus event): the doctor check, its code and its phase (`ai/rules/user-facing-errors.md`, `docs/architecture/doctor-and-health-checks.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | `features/<id>.md` (renders `docs/features.md`); `./le feature report` shows its level and the gap to the next |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/release/fixes/spec-firewall-vpp-apply-before-connect.md` lists them. A doc DECLARED by a changed file's `// Design:` header BLOCKS until named here; a doc that only `<!-- source: -->` mentions it is advisory. Naming it as unaffected, with the reason, satisfies the check |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| The first lab run still shows `stage stalled` | The row is a tier defect: back to DESIGN with the log, before any candidate is chosen (carried from child 1) |
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The four VPP consumers that apply config do not agree today: the interface component records and reconciles, the VPP FIB plugin reconciles on the event, the firewall and traffic backends wait and fail, and static routes pick a backend once. The answer chosen here is the first place that disagreement is decided on purpose.
- The firewall apply is reached by every table owner through `ApplyAll`, so the wait is not only a boot matter: a table owner that reconciles while VPP is down waits the same bound under the same lock.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-08-10 | firewall vpp backend waits for VPP in the config stage |
| `plan/journal/plugin-startup-barrier-deadlock.md` | 2026-11-11 | traffic vpp backend waits for VPP in the config stage |
| `plan/journal/probe-inability-read-as-absence.md` | 2026-11-11 | static picks the kernel backend once, while VPP is still starting |

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)
N-A: Scope is plugin.

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
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
