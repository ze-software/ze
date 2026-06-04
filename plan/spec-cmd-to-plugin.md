# Spec: cmd-to-plugin

| Field | Value |
|-------|-------|
| Status | completed |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-06-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/849-command-surface-ownership.md` - the ownership model this spec extends
4. `plan/learned/850-appliance-command-plugin.md` - the exact precedent (appliance moved from `cmd/ze/install/appliance/` to `internal/appliance/`)
5. `ai/rules/plugin-self-containment.md` - the removal test every move must satisfy

## Task

Move 17 command plugins from `cmd/ze/<name>/` to `internal/plugins/<name>/`. Each command currently registers via `init()` in `cmd/ze/<name>/register.go` using `cmdregistry.MustRegisterRootHandler` or `MustRegisterLocalMeta`. After this spec, each command lives in `internal/plugins/<name>/`, registers identically, and `cmd/ze/main.go` has a blank import instead of a subpackage import.

The principle: **things should be a plugin if they can be a plugin.** The `cmd/ze/` package should contain only binary lifecycle code (`start`, `version`, `help`, `--plugins`, `update-serve`, dispatch logic). Everything else is a self-contained feature that registers its own commands and can be stripped from the build.

**install/uninstall are dispatch infrastructure, not plugins.** The `ze install` and `ze uninstall` root handlers are thin subdispatch shells. The actual features (`local`, `systemd`, `provision`) are independent plugins that each register their own subcommands into the install/uninstall dispatchers. Removing a plugin removes its install/uninstall subcommands. The dispatch infrastructure stays in `cmd/ze/` (or moves to an importable location so plugins can register into it).

This spec does not change any command's name, arguments, output, or exit code. It moves ownership only.

### Relationship to existing specs

- `spec-clear-command-ownership` moves daemon RPC handlers (`clear dns`, `clear vpn ipsec sa`, etc.) from central verb packages to their owning component. This spec is orthogonal: it moves offline shell commands from `cmd/ze/` to `internal/plugins/`.
- `spec-build-tag-split` restructures build tags. This spec is a prerequisite enabler: once commands are plugins with blank imports, gating them behind build tags becomes a one-line change per command.
- `spec-cmd-deprecation` adds deprecation metadata. Unaffected: deprecation attaches at the registration site, which stays the same `MustRegisterRootHandler`/`MustRegisterLocalMeta` call.

### Migration table

| # | Command(s) | From | To | Notes |
|---|-----------|------|----|-------|
| 1 | `doctor` | `cmd/ze/doctor/` | `internal/plugins/doctor/` | Already consumes plugin doctor-check registry |
| 2 | `support` | `cmd/ze/support/` | `internal/plugins/support/` | Platform-specific files (build tags already present) |
| 3 | `crashes` | `cmd/ze/crashes/` | `internal/plugins/crashes/` | Trivial: reads crash files |
| 4 | `host` | `cmd/ze/host/` | `internal/plugins/host/` | Trivial: hardware inventory |
| 5 | `explain` | `cmd/ze/explain/` | `internal/plugins/explain/` | Trivial: diagnostic code lookup |
| 6 | `debug` | `cmd/ze/debug/` | `internal/plugins/debug/` | Toggles ZeFS debug flags |
| 7 | `skills` | `cmd/ze/skills/` | `internal/plugins/skills/` | Agent skill listing |
| 8 | `diag`/`generate` | `cmd/ze/diag/` | `internal/plugins/diag/` | Wireguard keygen |
| 9 | `local` | `cmd/ze/local/`, `cmd/ze/install/local/`, `cmd/ze/uninstall/local/` | `internal/plugins/local/` | Registers `ze install local` + `ze uninstall local` |
| 10 | `systemd` | `cmd/ze/systemd/`, `cmd/ze/install/systemd/`, `cmd/ze/uninstall/systemd/` | `internal/plugins/systemd/` | Registers `ze install systemd` + `ze uninstall systemd` |
| 11 | `provision` | `cmd/ze/provision/`, `cmd/ze/install/remote/` | `internal/plugins/provision/` | Registers `ze install remote` |
| 12 | `connect` | `cmd/ze/connect/` | `internal/plugins/connect/` | SSH credential management |
| 13 | `passwd` | `cmd/ze/passwd/` | `internal/plugins/passwd/` | Password management |
| 14 | `signal`/`status` | `cmd/ze/signal/` | `internal/plugins/signal/` | Daemon IPC over SSH |
| 15 | `completion` | `cmd/ze/completion/` | `internal/plugins/completion/` | Shell completion (queries registries at runtime) |
| 16 | `exabgp` | `cmd/ze/exabgp/` | `internal/plugins/exabgp/` | ExaBGP config migration |
| 17 | `init` | `cmd/ze/init/` | `internal/plugins/init/` | Database bootstrap |

### What stays in `cmd/ze/main.go`

| Command | Why |
|---------|-----|
| `start` | Orchestrates daemon boot, process entry point |
| `version` | Binary identity (single line) |
| `help` | Enumerates registered commands from the registry |
| `--plugins` | Meta: lists loaded plugins |
| `update-serve` | Firmware update server |
| `install` (dispatch) | Thin root handler + subdispatch; plugins register subcommands into it |
| `uninstall` (dispatch) | Thin root handler + subdispatch; plugins register subcommands into it |
| `show version` | Local meta shortcut |
| `help command` | Local meta shortcut |
| dispatch logic | The `main()` function itself |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration pattern
  -> Constraint: plugins register via init() and blank imports; core never imports plugins directly
- [ ] `ai/rules/plugin-self-containment.md` - the removal test
  -> Constraint: removing a plugin's directory + its blank import must remove its entire surface and keep the build green
- [ ] `ai/patterns/registration.md` - init + registry + blank import conventions
  -> Constraint: each plugin has a register.go with init() that calls MustRegisterRootHandler or MustRegisterLocalMeta

### Learned Decisions
- [ ] `plan/learned/849-command-surface-ownership.md` - ownership model
  -> Decision: handler lives with the owning package, not under cmd/ze
- [ ] `plan/learned/850-appliance-command-plugin.md` - exact precedent
  -> Decision: chose internal/appliance/ (not internal/plugins/ because appliance has no daemon role, not internal/component/ because no daemon component). For this spec, internal/plugins/ is correct because these are self-contained features that can be stripped.
  -> Constraint: imports must use importable leaf packages (internal/core/helpfmt, internal/core/suggest), not cmd/ze/internal/ packages
- [ ] `plan/learned/838-doctor-check-ownership.md` - doctor check migration pattern
  -> Constraint: doctor checks migrate incrementally via the bridge pattern

**Key insights:**
- The appliance migration (850) is the mechanical precedent. Each command follows the same steps: move package, update register.go imports, change blank import in main.go, update tests.
- Commands that import `cmd/ze/internal/*` packages (helpfmt, suggest, ssh/client, cmdregistry, subdispatch, resolve) need those imports updated. Importable equivalents exist for helpfmt (`internal/core/helpfmt`) and suggest (`internal/core/suggest`). The cmdregistry import path is `internal/component/command/registry`.
- The remaining `cmd/ze/internal/` dependencies (ssh/client, subdispatch, resolve) must be moved to importable locations before the commands that use them can migrate.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - blank imports for all 15 commands, dispatch via dispatchRegisteredRoot and LookupLocal
- [ ] `cmd/ze/doctor/register.go` - registers "doctor" via MustRegisterLocalMeta
- [ ] `cmd/ze/support/register.go` - registers "support" via MustRegisterLocalMeta
- [ ] `cmd/ze/crashes/register.go` - registers "crashes" and "crashes show" via MustRegisterLocalMeta
- [ ] `cmd/ze/host/register.go` - registers "host" and "host show" via MustRegisterLocalMeta
- [ ] `cmd/ze/explain/register.go` - registers "explain" via MustRegisterLocalMeta
- [ ] `cmd/ze/debug/register.go` - registers "debug" root + "debug enable/disable/show" local
- [ ] `cmd/ze/skills/register.go` - registers "skills" via MustRegisterLocalMeta
- [ ] `cmd/ze/diag/register.go` - registers "generate wireguard keypair" via MustRegisterLocalMeta
- [ ] `cmd/ze/install/register.go` - registers "install" root handler (dispatch infrastructure, stays in cmd/ze/)
- [ ] `cmd/ze/install/dispatch.go` - subdispatch: Register(), Dispatch(), Subcommands()
- [ ] `cmd/ze/install/local/register.go` - registers "local" into install dispatcher, delegates to cmd/ze/local
- [ ] `cmd/ze/install/systemd/register.go` - registers "systemd" into install dispatcher, delegates to cmd/ze/systemd
- [ ] `cmd/ze/install/remote/register.go` - registers "remote" into install dispatcher, delegates to cmd/ze/provision
- [ ] `cmd/ze/uninstall/register.go` - registers "uninstall" root handler (dispatch infrastructure, stays in cmd/ze/)
- [ ] `cmd/ze/uninstall/dispatch.go` - subdispatch: Register(), Dispatch(), Subcommands()
- [ ] `cmd/ze/uninstall/local/register.go` - registers "local" into uninstall dispatcher
- [ ] `cmd/ze/uninstall/systemd/register.go` - registers "systemd" into uninstall dispatcher
- [ ] `cmd/ze/local/main.go` - RunInstall(), RunUninstall() implementations
- [ ] `cmd/ze/systemd/main.go` - RunInstall(), RunUninstall() implementations
- [ ] `cmd/ze/provision/main.go` - Run() implementation
- [ ] `cmd/ze/connect/register.go` - registers "connect" root handler
- [ ] `cmd/ze/passwd/register.go` - registers "passwd" root handler
- [ ] `cmd/ze/signal/register.go` - registers "signal" and "status" root handlers
- [ ] `cmd/ze/completion/register.go` - registers "completion" root handler
- [ ] `cmd/ze/exabgp/register.go` - registers "exabgp" root handler
- [ ] `cmd/ze/init/register.go` - registers "init" root handler
- [ ] `internal/plugins/sysctl/cli/register.go` - model for plugin-owned root handler (uses `/cli/` because sysctl has runtime component)
- [ ] `internal/component/bgp/cli/register.go` - model for component-owned root handler

**Behavior to preserve:**
- Every command's name, arguments, output format, and exit code
- Registration metadata (Description, Mode, Section, Subs)
- Build tag gating for stripped builds (setup_features_full.go / setup_features_stripped.go)
- All existing tests

**Behavior to change:**
- Package location moves from cmd/ze/<name>/ to internal/plugins/<name>/
- Blank imports in main.go change from cmd/ze/<name> to internal/plugins/<name>

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- User runs `ze <command> [args]`
- `main()` parses global flags, then dispatches via `dispatchRegisteredRoot` or `LookupLocal`

### Transformation Path
1. `main()` extracts first arg after global flags
2. `dispatchRegisteredRoot(arg, rctx, rest)` looks up in `cmdregistry.LookupRoot`
3. If found, calls the registered handler function with remaining args
4. Handler executes and returns exit code

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| main.go -> plugin | blank import triggers init() registration; dispatch via registry lookup | [ ] |
| plugin -> core libraries | direct import of internal/core/* and internal/component/* | [ ] |

### Integration Points
- `internal/component/command/registry.MustRegisterRootHandler` - root command registration
- `internal/component/command/registry.MustRegisterLocalMeta` - local (multi-word) command registration
- `internal/component/command/registry.LookupRoot` / `LookupLocal` - dispatch lookup

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor` | -> | `internal/plugins/doctor/` handler | `TestDoctorRegistered` |
| `ze support` | -> | `internal/plugins/support/` handler | `TestSupportRegistered` |
| `ze crashes` | -> | `internal/plugins/crashes/` handler | `TestCrashesRegistered` |
| `ze host` | -> | `internal/plugins/host/` handler | `TestHostRegistered` |
| `ze explain` | -> | `internal/plugins/explain/` handler | `TestExplainRegistered` |
| `ze debug` | -> | `internal/plugins/debug/` handler | `TestDebugRegistered` |
| `ze skills` | -> | `internal/plugins/skills/` handler | `TestSkillsRegistered` |
| `ze generate wireguard keypair` | -> | `internal/plugins/diag/` handler | `TestDiagRegistered` |
| `ze install local` | -> | `internal/plugins/local/` handler | `TestLocalInstallRegistered` |
| `ze uninstall local` | -> | `internal/plugins/local/` handler | `TestLocalUninstallRegistered` |
| `ze install systemd` | -> | `internal/plugins/systemd/` handler | `TestSystemdInstallRegistered` |
| `ze uninstall systemd` | -> | `internal/plugins/systemd/` handler | `TestSystemdUninstallRegistered` |
| `ze install remote` | -> | `internal/plugins/provision/` handler | `TestProvisionInstallRegistered` |
| `ze connect` | -> | `internal/plugins/connect/` handler | `TestConnectRegistered` |
| `ze passwd` | -> | `internal/plugins/passwd/` handler | `TestPasswdRegistered` |
| `ze signal` | -> | `internal/plugins/signal/` handler | `TestSignalRegistered` |
| `ze completion` | -> | `internal/plugins/completion/` handler | `TestCompletionRegistered` |
| `ze exabgp` | -> | `internal/plugins/exabgp/` handler | `TestExabgpRegistered` |
| `ze init` | -> | `internal/plugins/init/` handler | `TestInitRegistered` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze <command>` for each of the 17 moved command plugins | Identical output and exit code as before the move |
| AC-2 | Delete `internal/plugins/<name>/` + its blank import | Build succeeds; command disappears; all other commands work |
| AC-3 | No `cmd/ze/internal/*` imports from `internal/plugins/` | All imports use importable paths (`internal/core/*`, `internal/component/*`, `pkg/*`) |
| AC-4 | `cmd/ze/` contains only main.go, hub/, internal/, install/, uninstall/, and blank-import files | No command implementation code remains in cmd/ze/ (install/ and uninstall/ contain only dispatch infrastructure) |
| AC-5 | All existing tests pass after the move | `make ze-test` green |
| AC-6 | `ze help command` lists all commands (derived from registry) | Same output as before |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorRegistered` | `internal/plugins/doctor/register_test.go` | doctor command registered in cmdregistry | |
| `TestSupportRegistered` | `internal/plugins/support/register_test.go` | support command registered | |
| `TestCrashesRegistered` | `internal/plugins/crashes/register_test.go` | crashes command registered | |
| `TestHostRegistered` | `internal/plugins/host/register_test.go` | host command registered | |
| `TestExplainRegistered` | `internal/plugins/explain/register_test.go` | explain command registered | |
| `TestDebugRegistered` | `internal/plugins/debug/register_test.go` | debug command registered | |
| `TestSkillsRegistered` | `internal/plugins/skills/register_test.go` | skills command registered | |
| `TestDiagRegistered` | `internal/plugins/diag/register_test.go` | generate wireguard keypair registered | |
| `TestLocalInstallRegistered` | `internal/plugins/local/register_test.go` | `ze install local` subcommand registered | |
| `TestLocalUninstallRegistered` | `internal/plugins/local/register_test.go` | `ze uninstall local` subcommand registered | |
| `TestSystemdInstallRegistered` | `internal/plugins/systemd/register_test.go` | `ze install systemd` subcommand registered | |
| `TestSystemdUninstallRegistered` | `internal/plugins/systemd/register_test.go` | `ze uninstall systemd` subcommand registered | |
| `TestProvisionInstallRegistered` | `internal/plugins/provision/register_test.go` | `ze install remote` subcommand registered | |
| `TestConnectRegistered` | `internal/plugins/connect/register_test.go` | connect command registered | |
| `TestPasswdRegistered` | `internal/plugins/passwd/register_test.go` | passwd command registered | |
| `TestSignalRegistered` | `internal/plugins/signal/register_test.go` | signal + status commands registered | |
| `TestCompletionRegistered` | `internal/plugins/completion/register_test.go` | completion command registered | |
| `TestExabgpRegistered` | `internal/plugins/exabgp/register_test.go` | exabgp command registered | |
| `TestInitRegistered` | `internal/plugins/init/register_test.go` | init command registered | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs in this migration.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing functional tests | `test/` | All existing .ci tests continue to pass | |

### Interop Tests (MANDATORY for protocol features)
N/A -- no protocol changes.

## Files to Modify

- `cmd/ze/main.go` - change blank imports from `cmd/ze/<name>` to `internal/plugins/<name>`
- `cmd/ze/setup_features_full.go` - update blank imports for build-tag-gated commands
- `cmd/ze/setup_features_stripped.go` - verify no changes needed (already empty)

### cmd/ze/internal dependency audit

These `cmd/ze/internal/` packages are imported by the commands being moved. They must either have importable equivalents or be moved to an importable location first.

| Package | Used by | Importable equivalent | Status |
|---------|---------|----------------------|--------|
| `cmd/ze/internal/cmdregistry` | all 15 | `internal/component/command/registry` (confirmed via LSP) | exists |
| `cmd/ze/internal/helpfmt` | doctor, debug, explain, exabgp, connect, passwd, signal, completion, skills, provision | `internal/core/helpfmt` | exists |
| `cmd/ze/internal/suggest` | exabgp, signal | `internal/core/suggest` | exists |
| `cmd/ze/internal/ssh/client` | connect, signal, completion | needs importable location | Phase 0 |
| `cmd/ze/internal/subdispatch` | install, uninstall dispatch (stays in cmd/ze/) | needs importable location so plugins can register into dispatchers | Phase 0 |
| `cmd/ze/internal/resolve` | doctor, support | needs importable location | Phase 0 |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | |
| CLI commands/flags | No | Commands unchanged |
| Functional test for new RPC/API | No | Existing tests cover |
| Pipe completeness | No | No output changes |
| Doctor check for runtime dependencies | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Commands move, not added |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - update source layout table if it references cmd/ze/ subpackages |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep docs/ for source anchors referencing cmd/ze/<name> |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `internal/plugins/doctor/` - moved from cmd/ze/doctor/
- `internal/plugins/support/` - moved from cmd/ze/support/
- `internal/plugins/crashes/` - moved from cmd/ze/crashes/
- `internal/plugins/host/` - moved from cmd/ze/host/
- `internal/plugins/explain/` - moved from cmd/ze/explain/
- `internal/plugins/debug/` - moved from cmd/ze/debug/
- `internal/plugins/skills/` - moved from cmd/ze/skills/
- `internal/plugins/diag/` - moved from cmd/ze/diag/
- `internal/plugins/local/` - moved from cmd/ze/local/, cmd/ze/install/local/, cmd/ze/uninstall/local/
- `internal/plugins/systemd/` - moved from cmd/ze/systemd/, cmd/ze/install/systemd/, cmd/ze/uninstall/systemd/
- `internal/plugins/provision/` - moved from cmd/ze/provision/, cmd/ze/install/remote/
- `internal/plugins/connect/` - moved from cmd/ze/connect/
- `internal/plugins/passwd/` - moved from cmd/ze/passwd/
- `internal/plugins/signal/` - moved from cmd/ze/signal/
- `internal/plugins/completion/` - moved from cmd/ze/completion/
- `internal/plugins/exabgp/` - moved from cmd/ze/exabgp/
- `internal/plugins/init/` - moved from cmd/ze/init/

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, cmd/ze/internal dependency audit |
| 3. Wiring phase | Phase 0: resolve cmd/ze/internal dependencies (sequential) |
| 4. Implement (TDD) | Phase 1: parallel worktree agents, one per plugin |
| 5. Consolidate | Phase 2: merge worktrees, update main.go blank imports |
| 6. /ze-review gate | Review Gate section |
| 7. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 8. Critical review | Critical Review Checklist |
| 9-11. Fix cycle | Fix issues, re-verify |
| 12. Deliverables | Deliverables Checklist |
| 13. Security review | Security Review Checklist |
| 14-15. Final verify | Re-run, present summary |

### Implementation Phases

**Parallelism model:** after Phase 0 resolves shared dependencies on main, Phase 1 fans out one worktree agent per plugin. Each agent works on an isolated branch, touches only its own plugin directory, and does NOT modify `cmd/ze/main.go` or `setup_features_full.go`. A sequential consolidation phase merges results and rewrites the blank imports.

#### Phase 0: Resolve cmd/ze/internal dependencies (sequential, on main)

   - Move `cmd/ze/internal/ssh/client/` to an importable location (e.g. `internal/component/ssh/client/` or `internal/core/ssh/client/`)
   - Move `cmd/ze/internal/subdispatch/` to an importable location -- plugins need to register into install/uninstall dispatchers, and those dispatchers use subdispatch
   - Move `cmd/ze/internal/resolve/` to an importable location (e.g. `internal/core/resolve/`)
   - Move install/uninstall dispatchers (`cmd/ze/install/dispatch.go`, `cmd/ze/uninstall/dispatch.go`) to importable locations so plugins can call `Register()` -- the root handler registration (`register.go`) stays in `cmd/ze/install/` and `cmd/ze/uninstall/`
   - Tests: existing tests still pass after package moves
   - Verify: no `cmd/ze/internal/` imports remain that block plugin moves

#### Phase 1: Move command plugins (parallel, one worktree agent per plugin)

   All 17 plugins migrate concurrently. Each agent follows the appliance precedent (learned 850):

   Per agent (one plugin):
   a. Move package from `cmd/ze/<name>/` to `internal/plugins/<name>/`
   b. Update package declaration
   c. Update imports (replace cmd/ze/internal/* with importable equivalents)
   d. Delete old `cmd/ze/<name>/` directory
   e. Write registration test
   f. Verify build + test within worktree

   For local, systemd, provision: also move the `cmd/ze/install/<target>/register.go` and `cmd/ze/uninstall/<target>/register.go` glue into the plugin itself. Each plugin owns its own subcommand registration into the install/uninstall dispatchers.

   **Agents do NOT touch:** `cmd/ze/main.go`, `cmd/ze/setup_features_full.go`, `cmd/ze/setup_features_stripped.go`. These are shared files updated in Phase 2.

   | Agent | Plugin | Worktree branch | Conflict risk |
   |-------|--------|-----------------|---------------|
   | 1 | doctor | `plugin/doctor` | None -- own directory only |
   | 2 | support | `plugin/support` | None |
   | 3 | crashes | `plugin/crashes` | None |
   | 4 | host | `plugin/host` | None |
   | 5 | explain | `plugin/explain` | None |
   | 6 | debug | `plugin/debug` | None |
   | 7 | skills | `plugin/skills` | None |
   | 8 | diag | `plugin/diag` | None |
   | 9 | local | `plugin/local` | Deletes `cmd/ze/install/local/`, `cmd/ze/uninstall/local/` |
   | 10 | systemd | `plugin/systemd` | Deletes `cmd/ze/install/systemd/`, `cmd/ze/uninstall/systemd/` |
   | 11 | provision | `plugin/provision` | Deletes `cmd/ze/install/remote/` |
   | 12 | connect | `plugin/connect` | None |
   | 13 | passwd | `plugin/passwd` | None |
   | 14 | signal | `plugin/signal` | None |
   | 15 | completion | `plugin/completion` | None |
   | 16 | exabgp | `plugin/exabgp` | None |
   | 17 | init | `plugin/init` | None |

#### Phase 2: Consolidate (sequential, on main)

   - Merge all 17 worktree branches
   - Rewrite blank imports in `cmd/ze/main.go` and `cmd/ze/setup_features_full.go`
   - Delete any remaining empty `cmd/ze/<name>/` directories
   - Verify cmd/ze/ contains only main.go, hub/, internal/, install/, uninstall/, and blank-import files (install/ and uninstall/ are dispatch-only, no subpackages)

#### Phase 3: Verification (sequential)

   - **Functional tests** -- all existing .ci tests pass
   - **Full verification** -- `make ze-verify`
   - **Complete spec** -- learned summary, spec closure

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 17 command plugins moved, AC-1 through AC-6 demonstrated |
| Correctness | No command behavior changed (same output, same exit codes) |
| Removal test | For each moved plugin: delete directory + blank import, build succeeds. For local/systemd/provision: their `ze install`/`ze uninstall` subcommands also vanish |
| No cmd/ze/internal leaks | grep -r 'cmd/ze/internal' internal/plugins/ returns nothing |
| Registry parity | `ze help command` output identical before and after |
| Build tag gating | stripped build still excludes gated commands |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| 17 command plugins in internal/plugins/ | `ls internal/plugins/{doctor,support,crashes,host,explain,debug,skills,diag,local,systemd,provision,connect,passwd,signal,completion,exabgp,init}/register.go` |
| No command code in cmd/ze/ subdirs | `ls cmd/ze/` shows only main.go, hub/, internal/, install/, uninstall/, and *_import.go files (install/ and uninstall/ are dispatch-only) |
| All tests pass | `make ze-test` |
| Help output unchanged | `ze help command` diff against baseline |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| No new attack surface | Move only, no new functionality |
| Import paths | No accidental exposure of internal packages |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle after move | Phase 0: resolve the dependency first |
| Build tag gate broken | Phase 2: check setup_features_full.go / stripped.go blank imports |
| Test fails after move | Check package path in test imports (agent can fix in worktree) |
| cmd/ze/internal dependency | Phase 0: must resolve before proceeding |
| Merge conflict in Phase 2 | Should not happen (agents touch disjoint directories). If it does: resolve manually |
| Agent fails on one plugin | Other agents unaffected. Fix and re-run that agent only |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `internal/plugins/<name>/` not `internal/component/<name>/` | component | These are not core components that other things depend on. They are self-contained features. Plugins can be stripped. |
| No `/cli/` subdirectory for CLI-only plugins | sysctl pattern with `/cli/` | The `/cli/` split exists to separate runtime plugin behavior from CLI surface. These commands are CLI-only, no runtime component. |
| One spec for all 17 command plugins | One spec per command | Migration is mechanical and follows a single pattern. Separate specs would be overhead without value. |
| Phase 0 resolves cmd/ze/internal dependencies first | Inline resolution per command | Resolving once avoids repeated work and keeps each command move simple. |
| local, systemd, provision as independent plugins | Single `internal/plugins/install/` | install/uninstall are dispatch infrastructure, not features. The actual features are local, systemd, provision. Each plugin owns its install/uninstall subcommands. Removing a plugin removes its subcommands from both dispatchers. |

## Known Limitations

- This spec does not add build-tag gating for individual commands. That is the scope of `spec-build-tag-split`.
- This spec does not move `cmd/ze/hub/` (it is the daemon runtime, not a command plugin).
- This spec does not move `cmd/ze/install/` or `cmd/ze/uninstall/` root handlers -- they are dispatch infrastructure, not plugins. After migration, their subpackages (`local/`, `systemd/`, `remote/`) are deleted; only `register.go` and `dispatch.go` remain.
- The `cmd/ze/internal/` packages that are NOT used by any moved command stay in place.

## Implementation Summary

### What Was Implemented
- [to be filled]

### Bugs Found/Fixed
- [to be filled]

### Documentation Updates
- [to be filled]

### Deviations from Plan
- [to be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| All 17 command plugins work identically after move | functional test | make ze-test green |
| Each plugin passes removal test | build test | delete + build for each |
| No cmd/ze/ command implementation code remains | grep | ls cmd/ze/ (install/ and uninstall/ are dispatch-only) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [to be filled]

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
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cmd-to-plugin.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cmd-to-plugin.md`
