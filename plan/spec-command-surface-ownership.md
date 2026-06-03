# Spec: command-surface-ownership

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | Phases 1-4 + 7-enforcement + 8 done; 5, 6, 7-generator, 9 remain |
| Updated | 2026-06-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/planning.md` - spec workflow and verification rules.
3. `ai/patterns/registration.md` - core registration pattern.
4. `ai/patterns/cli-command.md` - offline and online command registration rules.
5. `ai/rules/doctor-checks.md` - owner-owned doctor check placement.
6. `cmd/ze/main.go` - current shell entry point and static dispatch.
7. `cmd/ze/internal/cmdregistry/registry.go` - current offline command registry.
8. `internal/component/plugin/server/rpc_register.go` and `internal/component/plugin/server/command.go` - current in-process daemon command registration.
9. `scripts/codegen/plugin_imports.go` and `internal/component/plugin/all/all.go` - current blank-import aggregation.

## Task

Make Ze's command surface owner-registered. A command, daemon RPC, command schema, doctor check, and unit test should live in the plugin, component, backend, or command package that owns the behavior. `cmd/ze` remains the process entry point and global bootstrap layer. It should consume command registrations and run user-entry functional tests, rather than owning owner-specific command logic.

This does not mean every command becomes an external plugin process. Ze needs an in-process command plugin model: a package can register commands and doctor checks through leaf registries without connecting to the bus, starting a plugin process, or implementing fake engine handlers.

If no plugin, component, backend, or command package owns a behavior, keep the command or check in `cmd/ze` and make that explicit in the registration metadata or test name.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - registration pattern and component isolation.
  → Constraint: core discovers capabilities through registries and startup imports, not by central code importing every implementation detail.
- [ ] `docs/architecture/api/commands.md` - user-facing verb-first command model and RPC command surface.
  → Constraint: user command grammar remains `<verb> <noun> [action] [identifier]`; moving ownership must not change paths, output shape, or pipe behavior.
- [ ] `docs/architecture/api/process-protocol.md` - external plugin process command declarations.
  → Constraint: external plugin command registration remains separate from built-in in-process command registration.
- [ ] `docs/architecture/cli/plugin-modes.md` - CLI and command plugin modes.
  → Constraint: command registration must work for offline shell commands, daemon/YANG commands, and interactive CLI command mode.
- [ ] `ai/patterns/registration.md` - registry and blank-import conventions.
  → Constraint: new registries must be leaf packages so owners can import them safely from `init()`.
- [ ] `ai/patterns/cli-command.md` - command package structure and registration.
  → Constraint: current guidance still points at `cmd/ze/internal/cmdregistry`; this spec must update it after moving the registry to an importable leaf package.
- [ ] `ai/rules/cli-grammar.md` - action-before-identifier CLI grammar.
  → Constraint: ownership migration cannot introduce noun-first aliases or new grammar.
- [ ] `ai/rules/doctor-checks.md` - runtime dependency check ownership.
  → Constraint: doctor check registration, check function, and unit test belong to the owning package when an owner exists.
- [ ] `ai/rules/discovery-updates.md` - discoverability for new registries and gates.
  → Constraint: new command ownership enforcement must update rules, docs, inventory, and verification paths in the same change.
- [ ] `ai/rules/pipe-completeness.md` - command output pipe contract.
  → Constraint: moving command handlers must preserve all pipe operator support for commands that produce output.

### Learned Decisions

- [ ] `plan/learned/448-handler-reorg.md` - split between SDK plugin registration and in-process RPC registration.
  → Constraint: `registry.Register()` is for SDK plugins; `pluginserver.RegisterRPCs()` is for engine-side in-process handlers. Do not mix them in one auto-discovery path.
- [ ] `plan/learned/632-op-1-easy-wins.md` - original `cmdregistry` design.
  → Constraint: the existing leaf-registry idea was correct, but its location under `cmd/ze/internal` prevents internal owners from registering commands.
- [ ] `plan/learned/829-command-verb-first.md` - verb-first command grammar.
  → Constraint: registered command paths must stay verb-first and deprecated alias support must remain near the registration owner.
- [ ] `plan/learned/830-typed-inter-plugin-dispatch.md` - internal dispatch must avoid tokenizer round-trips where typed paths exist.
  → Constraint: owner-owned command registration must not force internal control flow through string tokenization when a typed API already exists.
- [ ] `plan/learned/838-doctor-check-ownership.md` - doctor check ownership.
  → Decision: use the same proximity rule for command registration and doctor registration.

**Key insights:**
- The desired model already exists in fragments: `pluginserver.RegisterRPCs()` gives in-process daemon command registration, and owner packages such as `internal/component/iface/cmd`, `internal/component/pki`, `internal/plugins/firewall/nft`, and `internal/plugins/ntp` already register owner-side command handlers.
- The offline command registry is structurally right, but it is placed under `cmd/ze/internal`, which prevents imports from `internal/component` or `internal/plugins`.
- `cmd/ze/main.go` still owns a large static dispatch switch. That switch is the main barrier to command ownership because root command registration currently drives help only, not dispatch.
- Generated blank imports are part of the contract. Owner registration through `init()` is invisible unless a generated aggregator imports the owner packages.
- The SDK plugin registry must not be stretched to cover this work. It requires process/engine concepts and would create fake plugins for commands that only need in-process registration.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `cmd/ze/internal/cmdregistry/registry.go` - stores root metadata, local handlers, and help sections.
  → Constraint: `RegisterRoot` currently stores metadata only; root dispatch itself lives in `cmd/ze/main.go`.
  → Constraint: `LookupLocal` uses longest-prefix matching and returns a copied remainder slice; preserve behavior or document a deliberate replacement.
- [ ] `cmd/ze/main.go` - parses global flags, handles config-file startup, dispatches YANG verbs, then uses a static switch before falling back to `cmdregistry.LookupLocal`.
  → Constraint: global flags, config-file startup, `start`, `help`, version stamping, panic capture, and storage bootstrap stay in `cmd/ze`.
  → Constraint: owner-specific roots such as `interface`, `firewall`, `sysctl`, `tacacs`, `l2tp`, `resolve`, `bgp`, `plugin`, and `schema` should stop being dispatched by a central switch once registry dispatch can carry their runtime dependencies.
- [ ] `cmd/ze/*/register.go` - about 33 root registrations live under `cmd/ze` packages.
  → Constraint: many current registration packages are command wrappers around internal owners. Registration should move to the internal owner where one exists.
- [ ] `cmd/ze/config/register.go` - storage-dependent shortcuts are bound from `main.go` after storage setup via `BindStorageCommands`.
  → Constraint: registry handlers need an explicit runtime context for dependencies that are unavailable during `init()`.
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - command execution helper delegates local command registration to `cmdregistry` and runs YANG commands.
  → Constraint: any registry move must keep `cmdutil` cycle-free and update import paths for callers.
- [ ] `internal/component/plugin/server/rpc_register.go` - package-level `RegisterRPCs` appends built-in RPC handlers from owner package `init()` functions.
  → Decision: this is the model for in-process command plugins. It has no bus or external process requirement.
- [ ] `internal/component/plugin/server/command.go` - `LoadBuiltins` maps registered RPC wire methods to YANG command paths and dispatch options.
  → Constraint: moving handler packages must preserve WireMethod-to-YANG mapping and aliases.
- [ ] `internal/component/plugin/server/command_registry.go` - external plugin command registry, owned by running plugin processes.
  → Constraint: this registry is runtime process state. Do not use it to register built-in command packages.
- [ ] `internal/component/plugin/registry/registry.go` - SDK plugin registration requires `RunEngine` and `CLIHandler`.
  → Constraint: command-only owners must not be forced to register fake SDK plugins.
- [ ] `internal/component/plugin/all/all.go` - generated blank imports bring plugins, schemas, event namespaces, and some RPC command packages into the binary.
  → Constraint: owner registration through `init()` requires generated imports to keep packages linked.
- [ ] `scripts/codegen/plugin_imports.go` - discovers SDK plugins, schema packages, event namespaces, and `RegisterRPCs` packages from configured directories.
  → Constraint: import aggregation is already checked by `TestGeneratedPluginImportsCurrent`; extend the generator instead of creating hand-maintained import lists.
- [ ] `internal/component/cmd/show/show.go` - central show package registers generic RPCs and many owner-specific RPCs.
  → Constraint: generic show commands can stay central, but owner-specific show handlers should move to their owner packages.
- [ ] `internal/component/iface/cmd/cmd.go` and `internal/component/iface/cmd/clear.go` - interface owner already registers command RPCs from its own package.
  → Decision: use this as one migration pattern for component-owned daemon commands.
- [ ] `internal/component/pki/show.go` - PKI owner registers `show pki` RPCs directly.
  → Decision: owner package can register RPCs without living under `internal/component/cmd/show`.
- [ ] `internal/plugins/firewall/nft/cmd_show.go` and `internal/plugins/ntp/register.go` - plugin owners register their own show RPCs.
  → Decision: SDK plugin packages can also own in-process command registration when they already import the needed server types safely.
- [ ] `ai/rules/doctor-checks.md` - updated rule requires owner package to own doctor checks unless no owner exists.
  → Constraint: command ownership migration must not leave runtime dependency checks centralized in `cmd/ze/doctor` for migrated owners.

**Behavior to preserve:**
- Every existing command path, alias, exit code, JSON/text output, help text meaning, and pipe behavior remains user-compatible unless a specific AC names a change.
- `ze help`, `ze help --ai`, `make ze-command-list`, and inventory outputs remain derived from registries.
- External plugin process command declarations remain handled by the plugin process command registry.
- YANG command dispatch remains driven by WireMethod mappings from registered schemas.
- Global shell startup behavior remains in `cmd/ze/main.go`.

**Behavior to change:**
- Root command dispatch becomes registry-driven for owner-backed commands.
- Offline command registration moves from `cmd/ze/<domain>/register.go` to the owning internal package where one exists.
- Owner-specific daemon command RPC registration and schema ownership move out of central verb packages where practical.
- Doctor check registration moves to the same owner packages for migrated command owners when the check validates that owner behavior.
- Verification fails when a new owner-backed command is registered centrally without an explicit no-owner allowance.

## Design

### Alternatives Considered

| Approach | How it works | Rejected or chosen | Reason |
|----------|--------------|--------------------|--------|
| Reuse SDK plugin registry | Add command metadata to `internal/component/plugin/registry.Registration` and require every command owner to register as a plugin | Rejected | It requires `RunEngine` and `CLIHandler`, confuses process plugins with in-process command providers, and creates fake plugins for pure command ownership. |
| Move existing `cmdregistry` to an internal leaf package and add root handlers | Keep the offline registry model but make it importable by internal owners; add runtime context and root dispatch support | Chosen | It preserves the existing registry shape, avoids bus/process requirements, and makes owner-side `init()` registration possible. |
| Keep root dispatch central and move only metadata | Owner packages register help metadata, but `main.go` keeps calling every owner package directly | Rejected | This leaves the real command surface centralized and does not satisfy ownership. Help would claim ownership that dispatch does not follow. |
| Route all shell commands through YANG RPC dispatch | Convert offline commands into daemon-style YANG commands and dispatch them through `cmdutil.RunCommand` | Rejected | Offline setup and recovery commands must work without daemon state, storage may need shell-specific setup, and this would overfit the YANG path. |

### Recommended Architecture

Introduce two leaf registries for in-process command providers:

| Registry | Purpose | Importers | Consumers |
|----------|---------|-----------|-----------|
| Offline command registry | Root shell commands, local shortcuts, owner metadata, runtime context requirements | `internal/component/*`, `internal/plugins/*`, remaining `cmd/ze/*` no-owner packages | `cmd/ze/main.go`, help, inventory, MCP/help generation where relevant |
| Doctor check registry | Readiness checks with phase/order/component/dependency metadata | same owner packages | `cmd/ze/doctor` runner and `show doctor` provider |

The offline command registry should move out of `cmd/ze/internal`. A likely location is `internal/component/command/registry` because `internal/component/command` already owns command tree types, help, completion, pipe handling, and command validation. The registry package must stay leaf-like: no imports from concrete command owners, no storage, no plugin server, no CLI package, no hub package.

Root registrations need handlers, not just metadata. A root handler receives a runtime context assembled by `cmd/ze/main.go` after global flag parsing. That context carries dependencies that cannot be captured in `init()`: storage resolver, plugin list, config file override, version printer, web/MCP flags, and any other process entry dependencies. Simple handlers can ignore the context.

Daemon/YANG commands continue using `pluginserver.RegisterRPCs`, but owner-specific registrations should move to owner packages. Central verb packages keep generic cross-system commands and helpers. The import aggregator ensures all owner command packages are blank-imported into `ze`, tests, and inventory tools.

Doctor checks follow the same owner boundary. A command owner that owns a runtime dependency owns the doctor check and its unit test. `cmd/ze/doctor` owns phase execution, JSON/text output, functional coverage, and checks with no narrower owner.

### Ownership Rules

| Surface | Owner rule | Central allowance |
|---------|------------|-------------------|
| Offline root command | Package that owns the behavior registers root metadata and handler | `cmd/ze` only for process-global commands or no-owner commands |
| Offline local shortcut | Package that owns the behavior registers the local path and handler | `cmd/ze` only for no-owner shell glue |
| Daemon/YANG RPC | Package that owns the behavior calls `pluginserver.RegisterRPCs` | `internal/component/cmd/<verb>` only for generic cross-cutting verb commands |
| YANG command schema | Same owner as the RPC handler, or an owner schema subpackage blank-imported by aggregator | Central verb schema only for generic cross-cutting commands |
| Doctor check | Package that owns the dependency registers check, function, and unit test | `cmd/ze/doctor` only when no narrower owner exists |
| Functional test | User-entry surface owns end-to-end test | Stays in existing functional suites |

### No-Owner Allowlist

The implementation must create an explicit allowlist for command registrations that remain in `cmd/ze` or central command packages. The initial expected allowlist is:

| Command or area | Why it can stay central |
|-----------------|-------------------------|
| `help` | Describes the whole process command surface. |
| `version` and `show version` | Uses binary stamp and process build metadata. |
| `start` | Starts the daemon and wires global process dependencies. |
| config-file argument startup | This is process entry behavior, not a subcommand. |
| `--plugins` | Process inventory shortcut until replaced by a registered command. |
| `completion` | Shell integration for the whole binary, unless a command owner is introduced. |
| installer/service/uninstall/support/skills | Stay central only if no clearer internal owner exists after audit. Each remaining item must name the reason. |
| generic `show warnings`, `show errors`, `show health`, `show doctor` | Aggregate cross-system views. Owner-specific data producers still live with owners. |

The allowlist is a test fixture, not a comment. Adding to it requires naming the owner audit result.

## Data Flow (MANDATORY)

### Entry Point

- Shell: user runs `ze <args>`.
- Interactive CLI and SSH: user command resolves through the YANG command tree and dispatcher.
- API/MCP/web: command execution routes through registered command inventory and dispatcher.
- Doctor: user runs `ze doctor`, `ze show doctor`, or equivalent API path.

### Offline Shell Transformation Path

1. `cmd/ze/main.go` initializes crash logging, version stamping, diagnostic codes, and command registration imports.
2. Global flags are parsed exactly as today.
3. YANG verb commands still dispatch through `cmdutil.RunCommand`.
4. Config-file arguments still enter hub startup handling.
5. Root command lookup asks the importable offline registry for the longest registered root or exact root, depending on the final API.
6. The registry returns handler plus metadata and remaining args.
7. `main.go` builds a runtime context and invokes the owner handler.
8. The handler runs owner code and returns an exit code.
9. Unknown command suggestions use the same registered root list used by help.

### Daemon/YANG Transformation Path

1. Owner package blank import runs `init()`.
2. Owner schema package registers YANG module.
3. Owner handler package calls `pluginserver.RegisterRPCs` with WireMethod and handler.
4. Plugin server startup builds WireMethod-to-path mappings from YANG.
5. `LoadBuiltinsWithAliases` registers owner handlers with dispatcher options.
6. User command executes through CLI, SSH, web, API, MCP, or internal dispatch.
7. Handler returns `plugin.Response`; output and pipe processing remain unchanged.

### Doctor Transformation Path

1. Owner package blank import runs `init()`.
2. Owner check registers with doctor registry metadata.
3. `cmd/ze/doctor` groups registered checks by phase and runs them.
4. Diagnostics use registered diagnostic codes.
5. Unit tests live with the owner; functional tests assert user-visible `ze doctor` behavior.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| `cmd/ze` to internal owner | Root handler registry with runtime context | [ ] `TestRootDispatchUsesRegisteredOwnerHandler` |
| Internal owner to command registry | Owner imports leaf registry from `init()` | [ ] `TestOwnerCommandRegistrationHasNoCmdZeImport` |
| Owner schema to YANG tree | Schema package blank-imported by aggregator | [ ] `TestCommandSchemaImportsCurrent` |
| Owner RPC to dispatcher | `pluginserver.RegisterRPCs` and WireMethod mapping | [ ] `TestAllYangCommandsHaveRegisteredRPC` |
| Doctor owner to runner | Owner check registered in doctor registry | [ ] `TestDoctorChecksHaveOwnerMetadata` |
| Generated imports to binary | Aggregator generator check | [ ] `TestGeneratedCommandImportsCurrent` |

### Integration Points

- `cmd/ze/main.go` - consumes root handlers and keeps global startup.
- `internal/component/command` - likely home for offline command registry and shared command ownership metadata.
- `internal/component/plugin/server` - existing in-process daemon command registry remains the RPC handler registry.
- `internal/component/plugin/all` or new `internal/component/command/all` - generated blank-import aggregation.
- `scripts/codegen/plugin_imports.go` or a new generator - discovers command provider packages and schema packages.
- `make ze-command-list`, `make ze-inventory`, `make ze-doc-test`, and `make ze-verify-wiring-docs` - enforcement and inventory surfaces.

### Architectural Verification

- [ ] No owner package imports `cmd/ze` or `cmd/ze/internal`.
- [ ] `cmd/ze/main.go` imports the registry and process-global owners only, not every feature owner.
- [ ] External SDK plugin registry remains separate from in-process command registration.
- [ ] No fake `RunEngine`, fake bus connection, or no-op process plugin is introduced for command ownership.
- [ ] Existing dispatcher hot paths do not gain per-command allocations beyond startup registration and command parse boundaries.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze interface show` | -> | interface owner root handler registered in offline registry | `TestRootDispatchUsesRegisteredOwnerHandler` plus `test/ui/interface-show.ci` or existing equivalent |
| `ze show interface` | -> | `internal/component/iface/cmd` RPC registration | `TestAllYangCommandsHaveRegisteredRPC` plus existing interface show functional test |
| `ze firewall ...` | -> | firewall owner root handler registered outside `cmd/ze` | `TestOwnerRootCommandsDoNotDependOnCmdZeInternal` plus firewall CLI functional test |
| `ze show firewall ruleset` | -> | firewall plugin owner RPC registration | `TestOwnerRPCRegistrationsAreImported` plus firewall show functional test |
| `ze doctor --json` | -> | owner-registered doctor checks for migrated owners | `TestDoctorChecksHaveOwnerMetadata` plus doctor JSON functional test |
| `ze help --ai` | -> | registry-owned command inventory | `TestHelpAIUsesOwnerRegistry` |
| `make ze-command-list-json` | -> | generated command inventory from owner registrations | `TestCommandInventoryIncludesOwnerRegisteredRoots` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An internal owner package registers an offline root command | The package imports only an internal leaf command registry, registers metadata and handler from `init()`, and has no import path reaching `cmd/ze/internal`. |
| AC-2 | `cmd/ze/main.go` dispatches an owner-backed root command | The command is found through the registry, receives runtime context, runs the same behavior as before, and is absent from the central static owner-specific switch. |
| AC-3 | A command needs runtime dependencies such as storage, plugin list, version printer, web/MCP flags, or config override | The registry handler receives those dependencies through an explicit context from `main.go`; no dependency is read or opened in `init()`. |
| AC-4 | A command has no narrower owner | It remains in `cmd/ze` only if listed in the no-owner allowlist with a reason, and tests assert it is allowed. |
| AC-5 | An owner-specific YANG RPC exists | Its `pluginserver.RegisterRPCs` call and unit test live in the owner package, not a central verb package, unless the command is generic and allowlisted. |
| AC-6 | A YANG command schema maps to a WireMethod | Exactly one handler registration exists for the WireMethod, and the generated import aggregation imports both schema and handler packages needed for the binary. |
| AC-7 | An owner has a runtime dependency checked by doctor | The owner package registers the doctor check, owns its check function, owns its unit test, and `cmd/ze/doctor` only runs it through the registry. |
| AC-8 | A new owner-backed command is added centrally under `cmd/ze` or `internal/component/cmd/show` | Verification fails unless the command is explicitly added to the no-owner or generic allowlist with source-aware justification. |
| AC-9 | Existing commands are exercised through shell, CLI, API, MCP, or web command surfaces | User-visible command path, exit code, output shape, help text meaning, aliases, and pipe behavior remain unchanged. |
| AC-10 | External SDK plugin commands register as plugin process commands | They continue using the existing plugin process command registry; in-process owner registration does not replace or shadow process command declarations. |
| AC-11 | Generated import files are stale after moving a command owner | The generator check fails and names the missing or extra command provider import. |
| AC-12 | Documentation and rules mention command registration ownership | `ai/patterns/cli-command.md`, `ai/patterns/registration.md`, relevant architecture docs, and inventory documentation describe owner-owned command registration. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterRootHandlerRejectsEmptyName` | new registry test | Root handler registry validates metadata and handler presence. | |
| `TestRegisterRootHandlerRejectsDuplicateOwner` | new registry test | Duplicate root command ownership fails deterministically. | |
| `TestRootDispatchUsesRegisteredOwnerHandler` | `cmd/ze` dispatch test | `main.go` dispatches via registered root handler instead of owner-specific switch. | |
| `TestRootDispatchPassesRuntimeContext` | `cmd/ze` dispatch test | Storage resolver, plugin list, version printer, and process flags are passed explicitly. | |
| `TestNoOwnerAllowlistIsEnforced` | command ownership verification test | Central registrations must be allowlisted with reason. | |
| `TestOwnerCommandRegistrationHasNoCmdZeImport` | command ownership verification test | Owner packages do not import `cmd/ze` or `cmd/ze/internal`. | |
| `TestAllYangCommandsHaveRegisteredRPC` | command/YANG contract test | Every YANG command WireMethod has exactly one registered RPC handler. | |
| `TestOwnerRPCRegistrationsAreImported` | import aggregation test | Owner RPC packages with `RegisterRPCs` are blank-imported by generated aggregator. | |
| `TestGeneratedCommandImportsCurrent` | generator test | Generated command import file matches discovery. | |
| `TestDoctorChecksHaveOwnerMetadata` | doctor registry test | Registered checks declare owner component/plugin and do not centralize owner-specific checks. | |
| `TestHelpAIUsesOwnerRegistry` | help test | Help and AI inventory derive from registry metadata after migration. | |
| `TestCommandInventoryIncludesOwnerRegisteredRoots` | inventory test | `make ze-command-list-json` sources owner-registered roots. | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Root command name length | Existing command token validation rules | Longest existing root command | Empty string | Over project-defined command token limit if introduced |
| Local command path parts | Existing longest-prefix behavior | Full multi-word shortcut | Empty path | Path with invalid empty segment |
| Handler lookup remainder | Number of args in command invocation | All remaining args passed unchanged | No args | Large arg list preserves order and does not mutate input |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-interface-owner-dispatch` | `test/ui/` or existing interface `.ci` | `ze interface show` still works after owner registration moves. | |
| `cli-firewall-owner-dispatch` | `test/ui/` or existing firewall `.ci` | `ze firewall ...` still reaches firewall owner code. | |
| `cli-sysctl-owner-dispatch` | `test/ui/` or existing sysctl `.ci` | `ze sysctl ...` still reaches sysctl owner code. | |
| `yang-show-interface-owner-rpc` | existing command `.ci` suite | `ze show interface` still dispatches through owner RPC handler. | |
| `yang-show-firewall-owner-rpc` | existing command `.ci` suite | `ze show firewall ruleset` still dispatches through firewall plugin owner. | |
| `doctor-owner-checks-json` | doctor functional suite | `ze doctor --json` includes owner-registered checks with same diagnostic codes. | |
| `help-owner-command-inventory` | UI or command inventory suite | `ze help --ai` and command-list inventory include owner-registered commands. | |

### Interop Tests

This is not a wire protocol feature. No BGP, IPsec, L2TP, or other protocol wire interop tests are required unless a migrated owner command changes protocol behavior. If a command migration touches protocol output semantics, add a protocol-specific functional or interop test in that phase.

### Future

None. Any command left central must be justified by the no-owner allowlist, not deferred silently.

## Files to Modify

| File or area | Changes |
|--------------|---------|
| `internal/component/command/` | Add importable offline command registry package or subpackage, with root handler registration, local shortcut registration, metadata, owner metadata, and test helpers. |
| `cmd/ze/internal/cmdregistry/` | Replace with compatibility shim during migration or delete after callers move. It must not remain the canonical registry for internal owners. |
| `cmd/ze/main.go` | Replace owner-specific root switch cases with registry dispatch, keep global startup and no-owner commands. |
| `cmd/ze/help_ai.go` | Read command inventory from the new registry location. |
| `cmd/ze/internal/cmdutil/` | Update imports and registry access while keeping cycle-free command execution. |
| `cmd/ze/*/register.go` | Delete, shrink, or turn into no-owner registration only after owner packages register their commands. |
| `internal/component/iface/cmd/` | Pilot owner-side offline plus daemon registration for interface command surface. |
| `internal/component/firewall/` and `internal/plugins/firewall/nft/` | Move firewall root and show command ownership out of `cmd/ze/firewall` and central show packages where applicable. |
| `internal/plugins/sysctl/` and `cmd/ze/sysctl/` | Move sysctl root command and doctor ownership to sysctl owner where applicable. |
| `internal/component/tacacs/`, `internal/component/l2tp/`, `internal/component/resolve/`, `internal/component/bgp/`, `internal/component/plugin/` | Audit and migrate owner-backed command registrations. |
| `internal/component/cmd/show/`, `internal/component/cmd/update/`, `internal/component/cmd/clear/`, `internal/component/cmd/set/` | Keep generic command handlers central; move owner-specific handlers and schema to owners. |
| `internal/component/plugin/server/` | Preserve `RegisterRPCs` as in-process daemon registration; add ownership metadata only if needed for enforcement. |
| `internal/component/plugin/all/all.go` or new `internal/component/command/all/all.go` | Generated blank imports for owner command providers. |
| `scripts/codegen/plugin_imports.go` or new generator | Discover owner command provider packages, schemas, RPC registrations, and doctor checks. |
| `cmd/ze/doctor/` and doctor registry location | Expose or move registry so owner packages can register checks; keep runner and output in `cmd/ze/doctor`. |
| `internal/core/diagnostic/` | Keep diagnostic code lookup unchanged; add codes only if migration exposes missing owner-specific codes. |
| `mk/inventory.mk`, inventory scripts, command-list scripts | Extend command and registration inventory to report owner package and central allowlist status. |
| `ai/patterns/cli-command.md` | Update registration guidance from `cmd/ze/internal/cmdregistry` to owner-owned leaf registry. |
| `ai/patterns/registration.md` | Add in-process command provider registry and owner rules. |
| `ai/rules/doctor-checks.md` | Cross-reference command owner registration after implementation clarifies registry location. |
| `docs/architecture/api/commands.md` | Document command ownership and in-process command providers. |
| `docs/architecture/cli/plugin-modes.md` | Clarify that command plugins can be in-process and need no bus/process. |
| `docs/guide/command-reference.md` and generated command inventory docs | Update only if user-visible help or inventory format changes. |

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | Owner schema packages for moved daemon commands. |
| YANG validation constraints | [ ] | Only if command schema leaves move or new leaves are added. |
| YANG custom validators | [ ] | Only if moved command schemas expose dynamic values not already validated. |
| CLI commands/flags | [ ] | `cmd/ze/main.go`, owner offline command packages, new registry package. |
| CLI grammar | [ ] | `ai/rules/cli-grammar.md`; all moved paths stay verb-first. |
| Editor autocomplete | [ ] | Automatic through YANG tree if schema import remains correct. |
| Functional test for new RPC/API | [ ] | Existing command `.ci` tests plus new migration-specific tests where gaps exist. |
| Pipe completeness | [ ] | Required for any moved command that produces output. |
| Env var registration | [ ] | Not expected unless a moved command exposes new environment leaves. |
| Doctor check for runtime dependencies | [ ] | Required for owner packages with runtime dependency checks. |
| Prometheus counters/metrics | [ ] | Not expected. This is registration ownership, not new runtime state. |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | No new command behavior expected; update `docs/features.md` only if inventory output changes. |
| 2 | Config syntax changed? | [ ] | No. |
| 3 | CLI command added/changed? | [ ] | Yes if help/inventory format changes: `docs/guide/command-reference.md`. |
| 4 | API/RPC added/changed? | [ ] | Yes for command ownership docs: `docs/architecture/api/commands.md`. |
| 5 | Plugin added/changed? | [ ] | Yes: `docs/architecture/cli/plugin-modes.md` for in-process command providers. |
| 6 | Has a user guide page? | [ ] | Command reference only if user-visible text changes. |
| 7 | Wire format changed? | [ ] | No. |
| 8 | Plugin SDK/protocol changed? | [ ] | No SDK change expected; update process protocol only if external plugin command declarations change. |
| 9 | RFC behavior implemented? | [ ] | No. |
| 10 | Test infrastructure changed? | [ ] | Yes if inventory or generator gates change: `docs/functional-tests.md` or docs for documentation testing. |
| 11 | Affects daemon comparison? | [ ] | No. |
| 12 | Internal architecture changed? | [ ] | Yes: `docs/architecture/core-design.md` or `docs/architecture/api/commands.md`. |
| 13 | Route metadata keys added/changed? | [ ] | No. |
| 14 | Prometheus counters added/changed? | [ ] | No. |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | Yes: command inventory docs and plugin overview if inventory shape changes. |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Search docs for changed-file source anchors during implementation. |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | Verify examples for migrated commands. |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/command/registry/registry.go` or equivalent | Importable offline command registry. |
| `internal/component/command/registry/registry_test.go` | Registry validation and lookup tests. |
| `internal/component/command/all/all.go` or generated extension to `internal/component/plugin/all/all.go` | Blank-import command provider packages. |
| `internal/component/command/all/all_test.go` or generator test extension | Staleness check for generated command imports. |
| `scripts/codegen/command_imports.go` or extension to `scripts/codegen/plugin_imports.go` | Command provider discovery. |
| `scripts/checks/command_ownership.go` or equivalent | Enforcement for central registrations and owner imports. |
| `plan/learned/NNN-command-surface-ownership.md` | Learned summary at completion. |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file. |
| 2. Audit | Current Behavior, Files to Modify, Files to Create. |
| 3. Wiring phase | Wiring Test table and AC-1 through AC-4. |
| 4. Implement (TDD) | Implementation phases below. |
| 5. /ze-review gate | Review Gate section. |
| 6. Full verification | `make ze-lint-changed`, targeted unit and functional tests, then required full gate per implementation scope. |
| 7. Critical review | Critical Review Checklist below. |
| 8. Fix issues | Fix every issue from review. |
| 9. Re-verify | Re-run affected tests and gates. |
| 10. Repeat 7-9 | Until clean. |
| 11. Deliverables review | Deliverables Checklist below. |
| 12. Security review | Security Review Checklist below. |
| 13. Re-verify | Re-run final gates. |
| 14. Present summary | Executive Summary Report per `ai/rules/planning.md`. |

### Implementation Phases

Each phase ends with a self-critical review and targeted tests before proceeding.

1. **Phase: Registry foundation**
   - Tests: `TestRegisterRootHandlerRejectsEmptyName`, `TestRegisterRootHandlerRejectsDuplicateOwner`, `TestRootDispatchPassesRuntimeContext`.
   - Files: new internal command registry package, `cmdutil` import updates, compatibility shim if needed.
   - Verify: internal owner package test can register a command without importing `cmd/ze/internal`.

2. **Phase: Root dispatch cutover**
   - Tests: `TestRootDispatchUsesRegisteredOwnerHandler`, `TestNoOwnerAllowlistIsEnforced`, `TestHelpAIUsesOwnerRegistry`.
   - Files: `cmd/ze/main.go`, `cmd/ze/help_ai.go`, registry tests.
   - Verify: owner-backed root command dispatch uses registry before static fallback; static switch contains only allowlisted no-owner/process-global commands.

3. **Phase: Pilot owner migration**
   - Tests: interface or resolve owner unit tests plus one functional `.ci` command test.
   - Files: one small owner package with offline root command, local shortcuts, and any matching daemon RPC registration.
   - Verify: behavior is unchanged from user surface, and deleting the old `cmd/ze/<domain>/register.go` registration does not remove help or dispatch.

4. **Phase: Command owner migration by domain**
   - Tests: per-owner unit tests and existing functional tests for each migrated command.
   - Files: interface, firewall, sysctl, tacacs, l2tp, resolve, bgp, plugin, schema/config/storage owners as audited.
   - Verify: each migrated command is removed from central dispatch and central registration, or explicitly listed as no-owner.

5. **Phase: Daemon/YANG owner migration**
   - Tests: `TestAllYangCommandsHaveRegisteredRPC`, owner handler registration tests, existing command `.ci` tests.
   - Files: owner command packages, owner schema packages, central verb packages.
   - Verify: owner-specific `pluginserver.RegisterRPCs` calls live with owners; central verb packages contain only generic cross-system commands.

6. **Phase: Doctor ownership alignment**
   - Tests: `TestDoctorChecksHaveOwnerMetadata`, owner doctor unit tests, doctor JSON functional test.
   - Files: doctor registry location, owner packages with runtime dependency checks, `cmd/ze/doctor` runner.
   - Verify: migrated owners own their doctor checks; `cmd/ze/doctor` contains no owner-specific checks unless no-owner allowlisted.

7. **Phase: Import aggregation and inventory enforcement**
   - Tests: `TestGeneratedCommandImportsCurrent`, `TestOwnerRPCRegistrationsAreImported`, command inventory tests.
   - Files: generator, generated all package, inventory scripts, Makefile hooks.
   - Verify: removing a command provider import makes tests fail; adding a provider without generator update fails.

8. **Phase: Documentation and rule updates**
   - Tests: `make ze-doc-test`, `make ze-verify-wiring-docs`.
   - Files: rules, patterns, architecture docs, command inventory docs.
   - Verify: future agents can discover the owner registration requirement from `ai/INDEX.md`, patterns, and docs.

9. **Phase: Full compatibility sweep**
   - Tests: all targeted command functional tests, command inventory, doc tests, lint-changed, then the required final verification gate.
   - Files: any missed owner wrappers or central leftovers.
   - Verify: command list before and after migration is semantically unchanged except ownership metadata and allowed documentation changes.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every owner-backed command root, local shortcut, daemon RPC, schema, and doctor check is either migrated or explicitly no-owner allowlisted. |
| Correctness | Dispatch passes the same args in the same order and returns the same exit code as the old path. |
| Naming | Command paths remain verb-first and kebab-case where applicable. Registry owner names use lower-kebab identifiers. |
| Data flow | `cmd/ze` builds runtime context and invokes registry handlers; owners do not read process-global state from `init()`. |
| Import cycles | New registry packages are leaf-like and owner packages do not import `cmd/ze/internal`. |
| Registration split | SDK plugin process registry remains separate from in-process command provider registry. |
| Doctor checks | Owner dependency checks live with owners; central doctor package owns runner and no-owner checks only. |
| Generated imports | Every owner provider is imported by a generated aggregator and checked by tests. |
| Pipe completeness | Moved output-producing commands still route through existing pipe processing. |
| Documentation | Rules and architecture docs describe the new ownership model and no longer point new code at `cmd/ze/internal/cmdregistry`. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Importable offline command registry exists | Read new registry file and run registry unit tests. |
| `cmd/ze/main.go` consumes registry for owner-backed roots | Unit test plus read of static allowlist. |
| Owner packages register commands | Search for registrations in owner packages and absence from old central files. |
| Central allowlist exists | Unit test rejects unallowlisted central registration. |
| Daemon RPC owner registrations migrated | YANG/RPC contract test plus owner registration tests. |
| Doctor owner checks migrated for affected owners | Doctor registry metadata test plus owner unit tests. |
| Generated command imports current | Generator `--check` test passes. |
| User-visible command behavior preserved | Functional tests for representative migrated commands pass. |
| Rules and docs updated | `make ze-doc-test` and `make ze-verify-wiring-docs` pass. |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Argument preservation | Registry dispatch must pass user args unchanged to the owner handler. No lossy string rejoin/split for shell args after parsing. |
| Unknown command handling | Suggestions must not execute commands or pick ambiguous prefixes. |
| Runtime context exposure | Owner handlers receive only the process dependencies they need; no global mutable context bag exposed unnecessarily. |
| External plugin separation | External plugin process command registry cannot shadow built-in root commands through this migration. |
| Initialization safety | `init()` functions only register metadata and handlers. They must not open files, sockets, storage, or start goroutines. |
| Resource use | Generated inventory and registry snapshots allocate at startup or command-list time, not on hot packet paths. |
| Error leakage | Help and registry errors should name command paths and owner packages, not secrets from config or environment. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle from owner to registry | Move registry dependency lower or split metadata types into a leaf package. |
| Command behavior changes after migration | Restore old behavior in owner handler, then update tests only if the old behavior was explicitly wrong. |
| Runtime dependency unavailable in `init()` | Add dependency to runtime context, never open it during registration. |
| YANG command loses handler | Fix aggregator or owner `RegisterRPCs` registration before changing command schema. |
| Doctor check cannot import registry | Move or expose doctor registry through a leaf package before migrating that check. |
| Generator misses package | Extend discovery rules and add a regression test for that package shape. |
| Central command has disputed owner | Stop that migration item and record owner decision in the spec before coding further. |

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

- In-process command provider is the right term for this work. It avoids overloading SDK plugin registration while preserving the project-wide plugin/registration ownership model.
- The main technical blocker is registry location and dispatch shape, not command handler code itself.
- The migration should start with one small owner to prove runtime context and import aggregation before moving broad command families.
- Central verb packages should become aggregators of truly generic commands, not dumping grounds for any command starting with `show`, `set`, `clear`, or `update`.

## Core Insight

The command surface has two different kinds of plugins. External SDK plugins are runtime processes. Built-in command providers are Go packages registered in-process at startup. Treating both as plugins conceptually is useful, but forcing them through the same registry would make the code lie. The correct design is separate leaf registries with the same ownership rule.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use an importable in-process command registry | SDK plugin registry, central switch | Preserves ownership without fake engine handlers or bus connections. |
| Move root dispatch behind registry handlers | Keep central switch plus owner metadata | Real ownership requires dispatch ownership, not only help metadata. |
| Keep global process commands in `cmd/ze` | Force every command into internal owners | Some commands genuinely belong to the process entry point. |
| Extend generated import aggregation | Hand-maintained blank imports | Init registration is only reliable when imports are generated and checked. |
| Enforce no-owner allowlist | Rely on review comments | Ownership drift is easy to reintroduce without a failing gate. |

## Known Limitations

- This spec does not rename user commands. It moves ownership and registration only.
- This spec does not merge external plugin process commands with built-in command providers.
- Some process-global commands will remain in `cmd/ze`; each must be allowlisted with a reason.
- The exact registry package path is decided during implementation after import-cycle checks, but it must be importable by `internal/component` and `internal/plugins` owners.

## RFC Documentation

No RFC behavior is implemented by this spec.

## Implementation Summary

### What Was Implemented

**Phases 1-4 complete (foundation + all owner-backed root migrations). Tree green:
`go build ./...`, `make ze-lint-changed` (0 issues), all touched-package tests pass.**

- **Registry foundation** (`internal/component/command/registry`, new stdlib-only leaf
  package): moved the offline command registry out of `cmd/ze/internal/cmdregistry`
  (now a re-export shim) so any internal owner can register from `init()`. Added the
  owner-backed dispatch API: `RootHandler`, `RuntimeContext` (ResolveStorage/Plugins/
  ConfigOverride/PrintVersion/web+MCP flags), `RegisterRootHandler`/`MustRegisterRootHandler`
  (rejects empty name / nil handler / duplicate owner), `LookupRoot`, `HasRootHandler`,
  `StorageAs[T]`, and `SetRuntimeStorage`/`RuntimeStorage` (for storage-backed local
  shortcuts whose `func(args)int` signature gets no context).
- **Dispatch cutover** (`cmd/ze/main.go`): `dispatchRegisteredRoot` consults the registry
  before the legacy static switch; `newRuntimeContext` builds the context (storage resolved
  lazily). Help/AI inventory already derives from the registry, so owner roots appear
  automatically. Tests: `TestRootDispatchUsesRegisteredOwnerHandler`,
  `TestRootDispatchPassesRuntimeContext`, `TestHelpAIUsesOwnerRegistry`.
- **Shared leaf helpers relocated** to `internal/core/` with shims at the old cmd/ze paths
  (so all existing importers compile): `helpfmt`, `suggest`, `ssh/client`.
- **All 15 owner-backed root commands migrated** out of `cmd/ze/<domain>` into their
  `internal/` owners (each cmd/ze-free, `RegisterRootHandler` + any `show <x>` shortcuts,
  static-switch case removed, old package deleted, behavior preserved):
  interface→`internal/component/iface/cli`, firewall→`internal/component/firewall/cli`,
  sysctl→`internal/plugins/sysctl/cli`, tacacs→`internal/component/tacacs/cli`,
  resolve→`internal/component/resolve/cli`, l2tp→`internal/component/l2tp/cli`,
  traffic-control→`internal/component/traffic/cli`, plugin→`internal/component/plugin/cli`,
  yang→`internal/component/config/yang/cli`, env→`internal/core/env/cli`,
  data→`internal/component/config/storage/cli`, schema→`internal/component/config/schema/cli`,
  bgp→`internal/component/bgp/cli` (+ its YANG tools schema → `internal/component/bgp/cli/schema`,
  regenerated `plugin/all`), config→`internal/component/config/cli` (storage via StorageAs +
  RuntimeStorage; `cmd/ze/hub` repointed), cli→`internal/component/cli/client` (the command-tree
  facade; 6 importers repointed with alias `cli`).
- **No-owner roots** left in `cmd/ze` (to be recorded in the Phase 7 checker allowlist):
  init, passwd, debug, remote, exabgp — plus the spec's existing allowlist (help, version,
  start, --plugins, completion, update-serve, run, install/appliance/uninstall/service,
  support, skills, ping, generate, signal, status, config-file startup).
- Functional tests added: `test/ui/command-owner-interface-show.ci`,
  `test/firewall/command-owner-firewall-root.ci`; existing `test/parse/sysctl-list-profiles.ci`
  and `internal/component/cmd/show/schema` self-containment test kept green.

**Interim:** owner blank imports are hand-listed in `cmd/ze/main.go`; the generated
command-provider aggregator is deferred to Phase 7 per user direction.

**Remaining: Phases 5-9** — owner-specific YANG-RPC migration out of central `cmd/*` verb
packages, doctor-check ownership alignment, the import-aggregation generator +
`ze-command-ownership-check` gate + allowlist fixture, documentation, final sweep + closure.

### Bugs Found/Fixed

- To be filled during implementation.

### Documentation Updates

- To be filled during implementation.

### Deviations from Plan

- To be filled during implementation.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Owner packages register command surface where an owner exists | Planned | This spec | |
| Command providers do not need bus or external plugin process | Planned | This spec | |
| No-owner commands stay where they are | Planned | This spec | |
| Doctor checks follow owner package rule | Planned | This spec | |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Planned | Unit tests | |
| AC-2 | Planned | Unit and functional tests | |
| AC-3 | Planned | Unit tests | |
| AC-4 | Planned | Allowlist enforcement test | |
| AC-5 | Planned | YANG/RPC contract tests | |
| AC-6 | Planned | Generated import tests | |
| AC-7 | Planned | Doctor registry tests | |
| AC-8 | Planned | Verification gate | |
| AC-9 | Planned | Functional tests | |
| AC-10 | Planned | Plugin process registry tests | |
| AC-11 | Planned | Generator check | |
| AC-12 | Planned | Doc tests | |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All tests listed in TDD Test Plan | Planned | See TDD Test Plan | |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Files listed in Files to Modify and Files to Create | Planned | Audit during implementation. |

### Audit Summary

- **Total items:** To be filled during implementation.
- **Done:** To be filled during implementation.
- **Partial:** To be filled during implementation.
- **Skipped:** To be filled during implementation.
- **Changed:** To be filled during implementation.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Command surface is owner-registered | Inventory plus enforcement tests | Planned: command ownership inventory and no-owner allowlist tests. |
| In-process command providers need no bus or external process | Unit tests and code review | Planned: registry API tests and no fake SDK plugin registrations. |
| User-visible behavior is preserved | Functional tests | Planned: representative shell and YANG command tests. |
| Doctor checks are owner-registered where owner exists | Unit and functional tests | Planned: doctor registry metadata tests and doctor JSON functional test. |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

- To be filled during implementation.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE.
- [ ] All NOTEs recorded above or explicitly none.

## Pre-Commit Verification

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified

| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-12 all demonstrated.
- [ ] Wiring Test table complete with concrete tests.
- [ ] `/ze-review` gate clean: 0 BLOCKER, 0 ISSUE.
- [ ] Required verification gates pass.
- [ ] Feature code integrated in `internal/*`, `cmd/*`, scripts, docs, and tests.
- [ ] Integration completeness proven end-to-end.
- [ ] Documentation Update Checklist answered with source evidence.
- [ ] Architecture docs and guides updated where behavior or ownership model is documented.
- [ ] Critical Review passes.

### Quality Gates

- [ ] Implementation Audit complete.
- [ ] Mistake Log escalation reviewed.
- [ ] No owner-specific command remains central without allowlist entry.
- [ ] No owner-specific doctor check remains central without allowlist entry.

### Design

- [ ] No fake SDK plugin registrations for in-process command providers.
- [ ] No command registration from `cmd/ze/main.go` except no-owner process-global commands.
- [ ] Single responsibility per owner package.
- [ ] Explicit runtime context instead of `init()` side effects.
- [ ] Minimal coupling through leaf registries.

### TDD

- [ ] Tests written.
- [ ] Tests fail for missing wiring before implementation.
- [ ] Tests pass after implementation.
- [ ] Boundary tests for registry validation.
- [ ] Functional tests for representative end-to-end behavior.
- [ ] Interop tests marked N/A unless a protocol behavior changes.
- [ ] Goal Validation table filled with concrete evidence.

### Completion

- [ ] Critical Review passes.
- [ ] Partial or skipped items have user approval.
- [ ] Implementation Summary filled.
- [ ] Implementation Audit filled.
- [ ] Learned summary written to `plan/learned/NNN-command-surface-ownership.md`.
- [ ] Commit A planned: code, tests, docs, spec, learned summary, counter bump.
- [ ] Commit B planned: remove `plan/spec-command-surface-ownership.md` only after Commit A preserves final spec.

## Review Resolution Amendments

These amendments resolve the critical review from 2026-06-02. They supersede earlier ambiguous wording in this spec. If an earlier section conflicts with this section, implement this section.

### Phase Ordering Resolution

Phase 2 is a registry cutover phase, not the final central-switch cleanup gate. The corrected phase contract is:

| Phase | Corrected gate |
|-------|----------------|
| Phase 2: Root dispatch cutover | `cmd/ze/main.go` must ask the new registry before the legacy static switch. Registry-dispatched roots must prove runtime context passing and unchanged argument order. The legacy switch may still contain owner-backed roots that have not reached their migration phase. |
| Phase 3: Pilot owner migration | The selected pilot owner must be absent from the static switch and central registration after the owner package registers it. |
| Phase 4: Command owner migration by domain | Each migrated owner root must be absent from the static switch and central registration, or be recorded in the no-owner allowlist below. |
| Phase 9: Full compatibility sweep | The static switch must contain only process-global and no-owner commands from the allowlist below. This is where the final `TestNoOwnerAllowlistIsEnforced` gate becomes blocking for the whole command surface. |

`TestRootDispatchUsesRegisteredOwnerHandler` validates Phase 2. `TestNoOwnerAllowlistIsEnforced` validates the final Phase 9 state and may validate only the pilot owner before Phase 4 completes.

### Concrete Wiring and Functional Test Resolution

The Wiring Test table is resolved to these exact tests. Tests marked "new" must be created by the implementation before the corresponding phase is accepted.

| Entry Point | Feature Code | Exact test |
|-------------|--------------|------------|
| `ze interface show` | interface owner root handler registered in offline registry | `test/ui/command-owner-interface-show.ci` (new) plus `TestRootDispatchUsesRegisteredOwnerHandler` |
| `ze show interface` | `internal/component/iface/cmd` RPC registration and owner command schema | `test/plugin/command-owner-show-interface.ci` (new) plus `TestAllYangCommandsHaveRegisteredRPC` |
| `ze firewall ...` | firewall owner root handler registered outside `cmd/ze` | `test/firewall/command-owner-firewall-root.ci` (new) plus `TestOwnerRootCommandsDoNotDependOnCmdZeInternal` |
| `ze show firewall ruleset` | firewall plugin owner RPC registration | existing `test/firewall/004-cli-show.ci` plus `TestOwnerRPCRegistrationsAreImported` |
| `ze sysctl list-profiles` | sysctl owner root handler registered outside `cmd/ze` | existing `test/parse/sysctl-list-profiles.ci` plus `TestOwnerRootCommandsDoNotDependOnCmdZeInternal` |
| `ze doctor --json` | owner-registered doctor checks for migrated owners | existing owner-specific doctor tests such as `test/ui/doctor-firewall-nftables.ci`, `test/ui/doctor-l2tp-module.ci`, and `test/ui/doctor-bgp-listen.ci`, plus new `test/ui/doctor-owner-provider-bridge.ci` |
| `ze show doctor` | diagnostic provider bridge runs owner checks from the daemon command surface | `test/ui/doctor-owner-provider-bridge.ci` (new) |
| `ze support --json --module doctor` or equivalent support collection path | support collection uses `diagnostic.RunDoctorChecks` provider bridge | `test/ui/doctor-owner-provider-bridge.ci` (new) |
| `ze help --ai --json` | registry-owned command inventory | extend existing `test/ui/help-ai-json.ci` to assert at least `interface`, `firewall`, and `sysctl` owner roots appear |
| `make ze-command-list-json` | generated command inventory from owner registrations | `TestCommandInventoryIncludesOwnerRegisteredRoots` plus a fixture assertion for at least one owner root and one owner YANG command |

AC-9 remains scoped to all listed command surfaces. Add the following explicit compatibility tests:

| Surface | Exact test |
|---------|------------|
| Shell root command | `test/ui/command-owner-interface-show.ci`, `test/firewall/command-owner-firewall-root.ci`, `test/parse/sysctl-list-profiles.ci` |
| Interactive CLI and SSH command dispatch | `test/plugin/command-owner-show-interface.ci`, existing `test/firewall/004-cli-show.ci` |
| REST API command inventory and execution | extend existing `test/plugin/rest-api-commands.ci` or create `test/plugin/rest-api-owner-command.ci`; it must assert an owner-registered YANG command appears in `/api/v1/commands` and executes successfully through `/api/v1/execute` |
| MCP command inventory and execution | `test/plugin/mcp-owner-command.ci` (new); it must assert the MCP tool inventory includes the migrated command execution tool and can execute an owner-registered read-only command |
| Web command surface | `test/plugin/web-owner-command.ci` (new) or a named `internal/component/web` integration test; it must exercise the web command route for an owner-registered read-only command |

### Import Aggregation and Manual Import Islands

Generated imports are not sufficient unless every current import island is covered. The implementation must handle all three consumers:

| Consumer | Current reason | Required implementation decision |
|----------|----------------|----------------------------------|
| Main `ze` binary | Needs owner providers linked for root registrations, schemas, and RPC handlers | Use generated owner command provider aggregation. |
| `cmd/ze/yang/tree.go` | Manually blank-imports RPC packages not included in `plugin/all` because of import cycles | Either replace this manual list with the generated command provider aggregator, or generate a second import file specifically for YANG analysis. `TestCommandSchemaImportsCurrent` must fail if a moved owner schema is missing here. |
| `cmd/ze/cli/main.go` | Manually blank-imports RPC packages for interactive CLI command dispatch | Either replace this manual list with the generated command provider aggregator, or generate a second import file specifically for CLI dispatch. `TestOwnerRPCRegistrationsAreImported` must fail if a moved owner RPC handler is missing here. |

The generator must discover command providers from owner packages, owner schema packages, and owner RPC registration packages. Discovery must include packages that call `pluginserver.RegisterRPCs`, packages with command schema `register.go`, and packages registering offline root handlers. The implementation must not rely on a hand-maintained import list for any migrated owner.

### Doctor Provider Bridge Resolution

Doctor ownership migration must preserve the existing provider bridge:

1. Owner packages register checks through the new doctor check registry.
2. `cmd/ze/doctor` remains responsible for CLI output, phase execution, and registering a provider with `internal/core/diagnostic.RegisterDoctorProvider`.
3. `internal/core/diagnostic.RunDoctorChecks` remains the bridge used by `show doctor` and support collection.
4. `internal/component/cmd/show/doctor.go` and `cmd/ze/support/support.go` must continue to receive owner-registered checks through that bridge.

Add `TestDoctorProviderBridgeUsesOwnerRegistry`. It must fail if `ze doctor --json` works but `show doctor` or support collection sees `nil` from `diagnostic.RunDoctorChecks`.

### YANG Schema Ownership Resolution

Use this schema strategy:

| Rule | Decision |
|------|----------|
| Owner-specific YANG command schema | Move to the owning package's `schema` subpackage, or to a command subpackage under the owner when that is the existing shape. |
| Command path expression | Owner command modules declare the full user command path from the root, such as `show firewall ruleset` or `show interface`. Do not hide owner-specific leaves inside a central verb package. |
| Central verb schema packages | Keep only generic cross-system verb commands, such as generic `show warnings`, `show errors`, `show health`, and `show doctor`. |
| Augment strategy | Do not introduce a new augment-based schema pattern for this migration unless an existing owner already uses it. Prefer standalone owner command modules registered by the owner schema package. |
| WireMethod mapping | Keep one `ze:command` WireMethod per executable command leaf. `TestAllYangCommandsHaveRegisteredRPC` must assert exactly one handler registration for every WireMethod. |
| Import aggregation | `TestCommandSchemaImportsCurrent` must verify owner command schema packages are imported by every command tree consumer named above. |

### No-Owner Allowlist Resolution

The no-owner allowlist is resolved to exact entries. A command not listed here must either migrate to its owner package or be added to this table with source-aware justification before implementation proceeds.

| Command or area allowed to stay in `cmd/ze` | Reason |
|---------------------------------------------|--------|
| `help` | Describes the whole process command surface. |
| `version` and `show version` | Uses binary stamp and process build metadata. |
| `start` | Starts the daemon and wires global process dependencies. |
| config-file argument startup | Process entry behavior, not a subcommand. |
| `--plugins` | Process inventory shortcut until replaced by a registered command. |
| `completion` | Shell integration for the whole binary. |
| `update-serve` | Release and update test infrastructure helper around the running binary, not a router runtime owner command. |
| deprecated `run` shim | Compatibility error path for removed grammar; it intentionally does not dispatch owner behavior. |
| `install`, `install appliance`, `appliance`, `uninstall`, `service` | Host installation and service management for the `ze` binary. No internal runtime component owns host package installation today. |
| `support` | Cross-system support bundle aggregator. Owner-specific collectors remain with owners where they exist, but archive orchestration stays process-global. |
| `skills` | Agent skill inventory tied to the current binary and generated support files. No runtime component owns it. |
| `ping` | Offline host diagnostic wrapper around the OS ping tool. No Ze component or plugin owns the behavior today. |
| `generate` | Offline artifact generation helper. It may move only if a narrower crypto or PKI command owner is introduced; until then it is no-owner command glue. |
| `signal` and `status` | Daemon process control over the management channel. No narrower component owns the whole process lifecycle today. |

This table replaces the earlier umbrella row for installer/service/uninstall/support/skills.

### Ownership Enforcement Scope and Gate Resolution

AC-8 applies to all central command packages, not only `cmd/ze` and `internal/component/cmd/show`.

| Central area | Enforcement |
|--------------|-------------|
| `cmd/ze` | No owner-backed root or local command may remain unless allowlisted above. |
| `internal/component/cmd/show` | Only generic cross-system show commands may remain. |
| `internal/component/cmd/update` | Only generic update commands may remain. |
| `internal/component/cmd/clear` | Only generic clear commands may remain. |
| `internal/component/cmd/set` | Only generic set commands may remain. |
| `internal/component/cmd/del`, `commit`, `meta`, `metrics`, `subscribe`, and other verb packages | Keep only generic verb infrastructure or explicitly allowlisted generic commands. Owner-specific commands must move. |

Add a named gate:

| Gate | Implementation |
|------|----------------|
| `make ze-command-ownership-check` | Runs `go run scripts/checks/command_ownership.go` or equivalent. It verifies central registrations against the no-owner and generic allowlists, verifies owner packages do not import `cmd/ze` or `cmd/ze/internal`, and verifies generated command import coverage. |
| `make ze-verify-wiring-docs` | Must route changed command registry, command owner, command schema, `cmd/ze`, `internal/component/cmd`, generator, and inventory files to `ze-command-ownership-check`. |
| `make ze-doc-test` | Must include ownership documentation drift only if command inventory docs gain source-aware ownership claims. |

`TestNoOwnerAllowlistIsEnforced` validates the checker logic. The Make target validates that the checker is not orphaned.
