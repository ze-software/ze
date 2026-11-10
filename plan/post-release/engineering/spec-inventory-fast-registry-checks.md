# Spec: inventory-fast-registry-checks

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Replaces a development check (the inventory's whole-tree external-effect proof) with three fast checks; the only shipped change is that the plugin registry seals after start-up and hands out copies, which hardens behaviour without changing any operator-visible answer |
| RFC gaps | - |
| Depends | - |
| Phase | 4/8 |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Design approval — 2026-11-09

The options were put to Thomas in conversation (keep chasing the whole-tree
proof, or replace it). He answered: "yes replace". The design below is the one
he approved: three fast checks replace the proof, the proof is deleted rather
than kept beside them (`ai/rules/no-layering.md`), the gated-table and coupling
checks move to `repoinventory.DeclaredPlugins`, and the "Approved
external-effect contract" section of `docs/architecture/plugin/feature-gates.md`
is rewritten to the new checks. His standing rule in `ai/META-DECISIONS.md`
("Website design") still holds: building the website runs no check, and a
proof over the code runs in CI and at release.

One point of the approved wording does not match the code. The implementation
follows the reading recorded under Key Design Decisions (row "Where a
registering call may sit") once the main thread confirms or corrects it.

## Task

The inventory's external-effect proof (`Snapshot.externalCertificate`,
`internal/le/repo/inventory/externalcertificate.go`, with 56 `external*` and
`assembly*` source files and 93 test files beside them) models arbitrary
third-party execution over every vendored package to show that no plugin
registration hides behind a call the declared walk cannot follow. One run
takes about an hour and 32 GB, it still refuses on vendored code
(`vendor/google.golang.org/protobuf/compiler/protogen/protogen.go`, per
`plan/handover/35-tooling-one-off-actual-root-chain-2026-11-09.md`), and
everything that calls `Snapshot.Plugins()` is red in CI because of it:
`TestAnswerAnswersTheInventory`, `./le plugin gated-table check` and
`./le plugin coupling check`.

The realistic risk the proof covers is narrower than what it models. The
registry lives under `internal/`, so Go refuses any import of it from outside
this module: vendored code can only register a plugin, or change a
registration, if Ze code hands it a reference. Three fast checks cover that
risk:

1. **The registry seals after start-up.** A registration after the seal
   fails loudly, and every lookup returns a copy, so no caller can change a
   registration's metadata. Enforced in product code, with unit tests and a
   functional test through the real binary.
2. **Registration sites are only the declared ones.** An AST check over Ze's
   own packages (never `vendor/`) that every registering call of the plugin
   registry sits in package initialization of a `register.go` file, and that
   nothing takes a registering function as a value or calls one from a
   closure, so no reference reaches code outside the module.
3. **The static list equals the runtime list.** On Linux, the built `ze`
   binary answers `show plugin list`, and a test compares it with
   `repoinventory.DeclaredPlugins` (commit 524c12c482).

Then the proof is deleted, and every reader of `Plugins()` reads the declared
walk.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/feature-gates.md` - "Committed plugin source population" and "Approved external-effect contract"
  → Decision: the declared walk (`DeclaredPlugins`) already lists what the source declares with the same metadata refusals; only completeness was the proof's job
  → Constraint: the contract section is replaced in the same commit that deletes the proof, never left describing deleted code
- [ ] `docs/contributing/gh-pages.md` - the paragraph on the declared registrations (around line 582)
  → Constraint: it names the proof and `repoinventory.Plugins` as the CI check; it is rewritten to name the three checks
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md`, `ai/doors/hub-engine.md`
  → Decision: registration from `init()` in `register.go` is the system-wide pattern (system door, "The core knows nothing, and everything registers"); check 2 enforces it rather than inventing a new one
  → Constraint: a shipped `ze` never links `internal/le` (`TestNormalZeLinksNoInternalLe`), so the seal lives in `internal/component/plugin/registry` and `cmd/ze`, and the checks live in `internal/le`
  → Constraint: a test that mutates the registry takes `registry.Snapshot()` and restores it (plugin-infra door, Traps), so the seal state travels with the snapshot
- [ ] `ai/rules/no-layering.md` - the proof is deleted, not kept as a slow second check
- [ ] `ai/rules/testing.md` and `ai/META-DECISIONS.md` "Test changes" - deleting or rewriting the proof's tests goes to an independent `ze-test-judge`

**Key insights:**
- The declared walk and the proof share one walker (`Snapshot.walkRegistrations`); the proof only changes the call authority (`callProved`, `callDiagnostic`) and feeds admitted call sites. Deleting the proof leaves `callDeclared` as the only behaviour, so the authority enum goes too.
- Three helpers the declared walk uses live in proof files and must move before those files go: `externalClosureBinding` and `closureCalledOnly` (`externalformals.go`, called from `callablealiases.go`) and `externalEmbeddedTypeName` (`externaleffecttypes.go`, called from `methodpromotion.go`). `externalSite` is used only by the proof branch of `registrations.node` and goes with it. `source.go` carries assembly population fields (`assemblyInvalid`, `assemblyPredefines`, `localHeaders`, `headerPresence`, `sourcePackage.assembly`) that only the proof reads.
- `Snapshot.collect` (`collectactualfiles.go`), the `./le repo inventory` report, reads through `readRegistrations`, so it runs the proof too and moves to the declared walk.
- `show plugin list` (`internal/component/plugin/register.go`, `dataPlugins`) already dumps the runtime registry as data, with a setup outcome per row; check 3 reads it rather than adding a new dump.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - `Register` copies the registration into the package map under `mu`; nothing stops a call after start-up. `Lookup` returns the stored `*Registration` and `All` returns the stored pointers, so a caller can write through them. `AddConfigRead`, `RegisterBuiltinFamilies` and `AddRPCHandlers` also mutate registry state. `Reset`, `Snapshot` and `Restore` are test-only save and restore.
- [ ] `internal/component/plugin/all/all.go` - the generated composition root: blank imports only.
- [ ] `internal/le/repo/inventory/registrations.go` - `Snapshot.Plugins` runs `readRegistrations`, which runs `externalCertificate` and then the walk under `callProved` (or `callDiagnostic` after a refusal). `Snapshot.DeclaredPlugins` runs the same walk under `callDeclared`, where an unresolved call is opaque (operands walked, no body followed) and a bodyless function is not certified.
- [ ] `internal/le/repo/inventory/plugins.go` - the package functions `Plugins(root)` and `DeclaredPlugins(root)`.
- [ ] `internal/le/repo/inventory/externalcertificate.go` - decides whether the proof is needed and runs `newExternalSource` and `newExternalEffects(...).solve()`. It sets `needed` for any non-module import whose first path segment holds NO dot, which is every standard-library import, so the proof runs on every real tree.
- [ ] `internal/le/repo/inventory/source.go` - `ReadCommitted` and the `Snapshot` and `sourcePackage` fields, including the proof-only assembly population.
- [ ] `internal/le/plugin/gatedtable/gatedtable.go` - `Derive` reads `snapshot.Plugins()`.
- [ ] `internal/le/plugin/coupling/coupling.go` - `load` reads `snapshot.Plugins()` and builds its model from the same snapshot's sources.
- [ ] `internal/le/verify/engine/stages.go` - `fullStages` runs `plugin gated-table check` and `plugin coupling check` beside `plugin imports check` and `plugin declarations check`.
- [ ] `internal/le/repo/inventory/collectactualfiles.go` - `Snapshot.collect` reads `readRegistrations`.
- [ ] `internal/component/plugin/register.go` - `show plugin list` answers rows of name, description, RFCs, capabilities, families, outcome and reason.
- [ ] `cmd/ze/main.go` - `main` defers the crash-log flush and calls `dispatchMain`.
- [ ] `internal/le/register.go` - le's composition: one blank import per tool package, the `internal/le/plugin/*` tools included.

**Behavior to preserve:**
- The `show plugin list` payload and columns.
- `DeclaredPlugins` output and its refusals (unresolved metadata, mutation after declaration, an empty population), and what the site publishes from it.
- Every `Register` refusal (empty name, duplicate, nil engine, invalid family, dependencies, data plane, attribute decoder, doctor check).
- Test isolation through `registry.Snapshot` and `registry.Restore`.
- The gated-table and coupling verdicts over the same committed snapshot, independent of the host and dirty files.

**Behavior to change:**
- After the seal, every registering call refuses instead of changing the registry.
- `Lookup` and `All` hand out copies.
- `Plugins`, the external-effect proof and the proved and diagnostic authorities are deleted; gated-table, coupling and the inventory report read the declared walk.
- Two new checks exist: registration sites (verify stage) and runtime equals declared (Linux test).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin registry | `internal/component/plugin/registry/registry.go` `Register`, `Lookup`, `All`, `Snapshot`, `Restore`, `Reset` | Process-global map filled from `init()`, readable and writable for the life of the process | Extends: a seal flag checked by one guard that every registering function calls, and copy-on-read in `Lookup` and `All`; the snapshot carries the seal flag |
| Composition root | `internal/component/plugin/all/all.go` (generated by `./le repo update`) | Blank imports that run every plugin's `register.go` `init()` | Does not apply: unchanged; it is why every registration runs before `main` |
| Declared registration walk | `internal/le/repo/inventory/registrations.go` `Snapshot.DeclaredPlugins`, `walkRegistrations` | Lists what the committed Linux le source registers, refusing unresolved metadata | Extends: becomes the only reader; gated-table, coupling and the report move to it; its `callDeclared` behaviour becomes the walker's only behaviour |
| External-effect proof | `internal/le/repo/inventory/externalcertificate.go` `Snapshot.externalCertificate`, the `external*` and `assembly*` files, `Snapshot.Plugins` | Whole-tree model of third-party execution; about an hour and 32 GB; refuses on vendored protobuf | Replaces: deleted whole, with its tests, after its shared helpers move |
| Gated-plugin table check | `internal/le/plugin/gatedtable/gatedtable.go` `Derive`, `rowsOf` | Judges the generated table against the manifest and the proved plugin list | Extends: reads `DeclaredPlugins` from the same snapshot |
| Coupling check | `internal/le/plugin/coupling/coupling.go` `load`, `buildSnapshotModel` | Judges undeclared cross-gate consumers over the proved plugin list | Extends: reads `DeclaredPlugins` from the same snapshot |
| Verify stages | `internal/le/verify/engine/stages.go` `fullStages` | Runs `plugin imports`, `plugin gated-table`, `plugin coupling`, `plugin declarations` checks | Extends: adds `plugin registration-sites check` beside them |
| Committed snapshot sources | `internal/le/repo/inventory/source.go` `ReadCommitted`, `Snapshot.Sources`, and the evaluator's `callableImport` | Parsed committed sources under the Linux le context, with import-alias resolution | Extends: check 2 reads the Ze-module packages of the same snapshot and resolves the registry import through `callableImport`, so no second source reader or alias resolver is written |
| Runtime registry dump | `internal/component/plugin/register.go` `dataPlugins`, `show plugin list` | Answers the binary's registry as data with a setup outcome per row | Extends by use: check 3 reads its JSON rendering from a built `ze`; no new dump command |
| Feature tag derivation | `internal/le/repo/featuretags` `DaemonTags` | The tag set a daemon build carries | Extends by use: check 3 builds `ze` with these tags, the context `DeclaredPlugins` reads |
| le tool composition | `internal/le/register.go` | One blank import per tool package | Extends: one blank import for the registration-sites tool |
| Hand-kept plugin lists | `TestAllPluginsRegistered` (`internal/component/plugin/all/all_test.go`), `TestAvailablePlugins` (`cmd/ze/main_test.go`) | Compare a test binary's registry with a hand-written list | Does not apply: they judge a test binary against a hand list; check 3 judges the shipped binary against the derived list. Replacing the hand lists is not this spec's work |
| Architecture tier checks | `./le arch tier check` (`internal/le/arch/tier`) | Placement and import ownership | Does not apply: it judges imports, not where a call sits |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Check 1: any call into a registering function of `internal/component/plugin/registry` at run time, and any `Lookup` or `All`.
- Check 2: `./le plugin registration-sites check`, from a shell or the verify stage.
- Check 3: the Linux test in `internal/le/plugin/registered`, run by the CI unit suite.

### Transformation Path
1. Every `init()` runs, registering plugins as today.
2. `main` in `cmd/ze` seals the registry before dispatch.
3. A later registering call meets the guard and refuses (an error where the function returns one, a `BUG:` panic where it returns nothing).
4. `Lookup` and `All` clone each registration (its slices and maps; function values are shared, being immutable) before returning.
5. Check 2 reads the committed snapshot, keeps Ze-module packages, finds each reference to a registering function (the set is the registry functions that call the seal guard), and judges each site.
6. Check 3 builds `ze` for linux/amd64 with the daemon tags and cgo off, runs `show plugin list` in its JSON rendering, reads `DeclaredPlugins` for the same commit, and compares.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| init → main | The seal call is the first statement after the deferred flush in `main` | No |
| Test binary → built `ze` | Check 3 runs the binary as a child process and parses its JSON | No |
| le → committed source | Checks 2 and 3 read `ReadCommitted`, never the working tree | No |

### Changed-Wire Census

**Search evidence:** No wire changes. The only machine-read output this spec reads is `show plugin list` in its JSON rendering, whose shape it preserves. Search scope: `internal/`, `cmd/`, `test/`, `docs/` for `show plugin list` and `keyPlugins`; producer `dataPlugins` (`internal/component/plugin/register.go`), new consumer check 3. The new `plugin registration-sites check` output is a le check verdict rendered through the shared renderer, read only by people and the verify engine's exit status.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show plugin list` JSON | writer | `internal/component/plugin/register.go` `dataPlugins`, `pluginRows` | registered with `command.MustRegisterLocalData`, payload `{plugins: [...]}` | Preserved: no field changes |
| `show plugin list` JSON | reader | new `internal/le/plugin/registered` test | parses the same payload | New reader, reads existing fields only |
| `show plugin list` JSON | reader | operators and API clients through every surface (`command.AnswerSurface`) | outside party | Preserved |

### Integration Points
- `registry.Register` and the other registering functions call one new seal guard.
- `cmd/ze` `main` calls the new seal function.
- `Snapshot.walkRegistrations` loses its `admitted` and `authority` parameters.
- `gatedtable.Derive`, `coupling.load` and `Snapshot.collect` call the declared walk.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Registrations still arrive only through `init()`; the seal closes the path after it |
| No unintended coupling (components stay isolated) | Yes | The registry package gains no import; `cmd/ze` already reaches it through the plugin host; `internal/le` stays out of shipped `ze` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Check 2 reuses `ReadCommitted` and `callableImport`; check 3 reuses `show plugin list` and `DaemonTags`; the proof is deleted, not kept |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No wire path; `Lookup` and `All` are startup, config and command paths (census in A-4) |
| Registration over hardcoding, outbound | Yes | No feature name is added to a core package |
| Registration over hardcoding, inbound | Yes | Check 2 derives the registering-function set from the seal guard's callers, so a new registry mutator is covered with no edit to the check; check 3 derives both lists |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No vendored package receives a registry reference from Ze code | The registry is under `internal/`, so no third-party import compiles (Go internal-package rule); owner's framing | Vendored code could register or mutate after the declared walk ran | Check 2 itself, green at HEAD (AC-7) | unvalidated: at d53989cfad the check refused only the two sites phase 3 moved; green owed once those moves are committed |
| A-2 | The registry can be sealed without breaking tests that register late | 89 test files call `registry.Register`, 67 call `Reset` or `Restore`; none goes through `cmd/ze` `main`, which is the only caller of the seal | Tests panic or fail after an unrelated test sealed | Grep: no test calls the seal function except the registry's own unit tests, which restore through `Snapshot`/`Restore`; scoped runs of `internal/component/plugin/...` and `cmd/ze` | confirmed (phase 2: 90 test files call `Register`, 68 `Reset`/`Restore`, none reaches `main`; registry, plugin/server, bgp/server and cmd/ze scoped runs green) |
| A-3 | No production code registers after `main` starts | Census of non-test `registry.Register` callers: 107 `register.go` files; the other hits (`plugin/server/startup.go`, `subsystem.go`, `config/schema/cli/main.go`) are other types' `Register` methods | A real runtime registration path breaks at start | Check 2 at HEAD, plus the functional suites that start the daemon (`./le test functional plugin`) | unvalidated |
| A-4 | No hot path calls `Lookup` or `All` per route or per event | Census of 48 non-test callers: config verify, startup, autoload, help, command declarations; `bgp/server/events.go` calls `Lookup` while ordering a startup selection | Copying adds allocations per UPDATE | Read each caller in the census; allocation benchmark of `Lookup` and `All` (AC-4) | confirmed with two exceptions moved (phase 2): the per-event callers `sortByReverseDependencyTier` (`bgp/server/events.go`) and `declaresSessionReady` (`plugin/server/events.go`) read `registry.StartOrderEdges` and `registry.RegisteredSessionReady`, which answer values |
| A-5 | No caller relies on pointer identity of a returned registration | Grep for comparisons of `Lookup` results found only nil checks | A comparison between two lookups turns false | Grep at implementation time for `==` on `*registry.Registration` values | confirmed (phase 2: no `==` on a looked-up registration) |
| A-6 | The `ze_le` tag adds no plugin registration, and no registration file is selected by GOARCH | `DeclaredPlugins` reads the ze_le linux/amd64 context; `find` shows Linux-selected registration files only under `internal/test/fixture` (test DUT) | Check 3 reports a difference that is a context mismatch, not a defect | Check 3 green at HEAD on linux/amd64 (AC-10) | confirmed: `TestBuiltZeRegistersTheDeclaredPlugins` green on linux/amd64 at 10a0471714, 96 plugins on both sides (the build carries `ze_core ze_distro`, the walk `ze_le`) |
| A-7 | Every Ze package that holds a registering call is in the committed snapshot's population | `ReadCommitted` takes the transitive import graph of `cmd/ze` under the Linux le context | A registering file outside that graph escapes check 2, but it is also outside the binary | The check reports its package count; a fixture outside the graph is not reported and also not linked | confirmed for the graph (phase 3): the check reads only `Snapshot.Sources`, and the zetest-only `internal/test/plugins/lateregister`, whose engine registers at run time on purpose, is outside it and not reported |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A plugin registers from a helper or another file today, so check 2 is red at HEAD | The census: seven plugins call `Register` from a named helper of their `register.go` (rsvpte, vrrp, ldp, ospf, isis, watchdog, healthcheck); the BGP message package registers builtin families from a package-variable initializer outside `register.go`; the policy component calls `AddConfigRead` from a function other packages' `init()` reach | The site rule in Key Design Decisions accepts helpers reached only from initialization; the two outside `register.go` move into one, or the main thread picks the literal rule and the calls move |
| R-2 | Deleting the proof's tests reads as a weakening | Commit gate refuses without approval lines | Each deleted or rewritten test goes to `ze-test-judge` with the owner decision as the source; replacement coverage is AC-1 to AC-11 |
| R-3 | Check 3 builds `ze` inside a test, which is slow locally | Package test takes minutes | The test sits in its own package so only it pays; it runs on Linux only |
| R-4 | A check written for a host differs by machine (journal class `gate-verdict-depends-on-the-machine`) | Check 3 differs on darwin | Check 3 is a `linux` build-tagged test file, so darwin does not compile it; check 2 reads only the committed Linux context, so it answers the same on every host |
| R-5 | Copy-on-read misses a nested reference type, so a caller can still write through a shared map | AC-3's mutation test stays red for one field | A test walks the `Registration` type by reflection and fails on any slice, map or pointer field the clone does not deep-copy |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin that truly registered late would fail at start with a `BUG:` message; a caller that wrote through a looked-up registration would lose that write. Neither exists in the census. le loses no check that worked: the proof never went green |
| How is it reverted? | Each commit reverts alone: the seal, check 2, check 3, the reader moves, then the proof deletion |
| Who else touches this path? | `spec-tooling-one-off-defects` (claimed, in progress) owns the proof work this replaces; its handover is `plan/handover/35-tooling-one-off-actual-root-chain-2026-11-09.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze` start-up, then a test-only plugin calling `registry.Register` from its engine | → | seal call in `cmd/ze` `main`, seal guard in `registry.Register` | `test/plugin/registry-sealed-after-start.ci` with test plugin `internal/test/plugins/lateregister` |
| `ze show plugin list` on linux/amd64 | → | `dataPlugins` against `repoinventory.DeclaredPlugins` | `TestBuiltZeRegistersTheDeclaredPlugins` (`internal/le/plugin/registered/registered_linux_test.go`) |
| `./le plugin registration-sites check` | → | the registration-sites action over `ReadCommitted` | `TestRegistrationSitesCheckAcceptsTheCommittedTree` (`internal/le/plugin/registrationsites/registrationsites_test.go`) |
| `./le plugin gated-table check`, `./le plugin coupling check` | → | `gatedtable.Derive`, `coupling.load` over `DeclaredPlugins` | `TestGatedPluginTableMatchesManifest`, `TestCouplingReadsTheDeclaredPopulation` |
| `./le repo inventory` | → | `Snapshot.collect` over the declared walk | `TestAnswerAnswersTheInventory` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `registry.Register` called after the registry is sealed | Returns an error wrapping a new `ErrRegistrySealed` sentinel that names the plugin and says registration is only allowed during package initialization; the registry's names, families and filter types read the same before and after | Every existing refusal keeps its own error before the seal; a call before the seal registers as today, because every `init()` runs before `main` |
| AC-2 | `AddConfigRead`, `RegisterBuiltinFamilies` or `AddRPCHandlers` called after the seal | `AddConfigRead` returns the same sealed error naming the plugin and root; the two functions that return nothing panic with a `BUG:` message naming the function and its argument | The runtime seams `SetEventBus`, `SetPluginServer`, `SetMetricsRegistry`, `InjectPluginMetrics`, `SetRuntimeSessionReady`, `SetRuntimeRegistryNames`, `SetNTPSyncProvider` stay callable after the seal: they install process wiring the daemon sets during start, not registration metadata |
| AC-3 | A caller writes into every slice, map and scalar field of a registration returned by `Lookup` and by `All`, before and after the seal | A second `Lookup` and `All` return the original values for every field; a reflection test fails on any reference-typed field the clone shares | Function-valued fields (`RunEngine`, `CLIHandler` and the other callbacks) are shared, because a function value cannot be changed through a copy |
| AC-4 | Allocation benchmark of `Lookup` and `All` over the full registry, and the census of their callers | The benchmark's bytes and allocations per operation are recorded in the spec, and every caller in the census is named as start-up, config, command or help time | None on a per-route or per-event path; if one is found it reads the field it needs through a registry accessor that returns a value, not a clone |
| AC-5 | `registry.Snapshot`, `Restore` and `Reset` in a test that seals | `Snapshot` records the seal state, `Restore` restores it, `Reset` clears it, so a test that seals and restores leaves the next test able to register | Tests that never seal are unchanged |
| AC-6 | The real `ze` binary starts with a test-only plugin whose engine calls `registry.Register` | The functional test observes the sealed refusal text in the plugin's report, the daemon keeps serving, and `show plugin list` lists the test plugin under its `init()` name | The test-only plugin's own `init()` registration still succeeds, which proves start-up registration still works |
| AC-7 | `./le plugin registration-sites check` over the committed tree | Answers pass and reports how many packages it read and each accepted registering site (package, file, function) | Sites accepted by the rule in Key Design Decisions; `_test.go` files and `testdata` are outside the population because no shipped binary links them |
| AC-8 | Fixture packages with: a registering call in a file not named `register.go`; in a function reached from something other than initialization; inside a function literal; a registering function used as a value (assigned, passed, returned); the registry package dot-imported; an aliased import used correctly | Each wrong fixture refuses with the file, line, the registering function, the rule it breaks and the command that shows the rule; the aliased-import fixture passes | A `vendor/` fixture holding a same-named function is never read, because Go refuses its import of the registry anyway |
| AC-9 | A fixture registry package with a new mutator that calls the seal guard, and fixture callers of it | Check 2 lists the new mutator's `init()` call among its accepted sites and refuses its fixture call outside initialization, with the check's own source unchanged | Registry functions that do not call the guard (readers, runtime seams) are not registering functions and their callers are not judged |
| AC-10 | `TestBuiltZeRegistersTheDeclaredPlugins` on linux/amd64 at HEAD | The built `ze`'s `show plugin list` names exactly the plugins `DeclaredPlugins` lists, with equal description, families, RFCs and capability codes, and every row's outcome says it registered | Fields `show plugin list` does not carry (dependencies, config roots, features, YANG) are covered by the declared walk's metadata refusals at the source and by check 1's copy rule at run time |
| AC-11 | A recorded break: a plugin's registration moved behind a call the declared walk treats as opaque, then the same test run | The test goes red naming the plugin as registered at run time and absent from the declared list; restored, green. The red is recorded in the spec | A difference in the other direction (declared, not registered) is reported the same way, by name |
| AC-12 | `./le plugin gated-table check` and `./le plugin coupling check` at HEAD | Both answer from `DeclaredPlugins` over one committed snapshot, finish in seconds, and keep their current verdicts on their fixtures; a Linux-only registration stays in their population when read on darwin | None: neither keeps a path to the deleted proof |
| AC-13 | `./le repo inventory` and `TestAnswerAnswersTheInventory` | The report collects from the declared walk and the test passes with a non-empty plugin list | The report's family, capability, schema and RPC fields keep their current derivation from the same snapshot |
| AC-14 | The tree after the deletion commit | `walkRegistrations` takes no authority and reads opaque calls as `callDeclared` reads them today; the closure-binding, called-only and embedded-type-name helpers live under names that do not start with `external`; the repository compiles in every flavor; `Snapshot.Plugins`, `repoinventory.Plugins`, `externalCertificate`, `externalProof`, the `callProved` and `callDiagnostic` authorities, every `external*` and `assembly*` production file and the proof-only fields of `source.go` are gone | `INVENTORY-TRAVERSAL-REVIEW.md` stays: `spec-tooling-one-off-defects` cites it as the record of a decision |
| AC-15 | Every deleted or rewritten inventory, site, gated-table and coupling test | Each carries a `ze-test-judge` verdict recorded through the approval route, citing this spec's owner decision; tests of the declared walk that ran through `Plugins()` are rewritten to `DeclaredPlugins` with their assertions kept | Tests whose only subject is the deleted proof (the `external*` and `assembly*` test files, `TestDeclaredPluginsMatchesTheProvedPopulation`, `TestSiteRegistryRunsNoExternalEffectProof`) are deleted with that verdict, their coverage replaced by AC-1 to AC-11 |
| AC-16 | The documentation after the change | `feature-gates.md` replaces "Approved external-effect contract" with a section on the three registry checks and their carriers, and its "Committed plugin source population" paragraphs keep only what the declared walk does; `gh-pages.md` names the three checks; the `ai/META-DECISIONS.md` website rule's example names the registry checks instead of the external-effect proof; `plugin-system.md` says registration closes when `main` starts; `INVENTORY-EXTERNAL-EFFECTS.md` is removed; `./le doc check all` passes | The proof's history stays in git and in `plan/learned/DESIGN-HISTORY.md`, where one row records that the whole-tree proof was replaced and why |
| AC-17 | `spec-tooling-one-off-defects` and its handover after this lands | AC-32 and P-32 there are restated to name these checks as the completeness proof; the handover's "Next" items 1 and 2 and its actual-root stop are marked superseded by this spec; the obsolete reboot-asset diagnostics `inventory-flowspec-range` and `inventory-fixture-refusal` are removed | AC-32's other obligations (Linux registrations read on darwin, dirty edits never published) stay there, carried by the declared walk |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterAfterSealRefuses` | `internal/component/plugin/registry/seal_test.go` | AC-1 | |
| `TestRegistryMutatorsAfterSealRefuse` | `internal/component/plugin/registry/seal_test.go` | AC-2, including the runtime seams staying callable | |
| `TestLookupAndAllReturnCopies` | `internal/component/plugin/registry/seal_test.go` | AC-3 | |
| `TestRegistrationCloneCoversEveryReferenceField` | `internal/component/plugin/registry/seal_test.go` | R-5, AC-3 by reflection over the type | |
| `BenchmarkLookupAndAll` | `internal/component/plugin/registry/seal_test.go` | AC-4 allocation figures | |
| `TestSnapshotRestoreCarriesTheSeal` | `internal/component/plugin/registry/seal_test.go` | AC-5 | |
| `TestMainSealsBeforeDispatch` | `cmd/ze/main_seal_test.go` | the seal call precedes `dispatchMain` (AST read of `main`) | |
| `TestRegistrationSitesCheckAcceptsTheCommittedTree` | `internal/le/plugin/registrationsites/registrationsites_test.go` | AC-7 | |
| `TestRegistrationSitesRefusesEachWrongSite` | `internal/le/plugin/registrationsites/registrationsites_test.go` | AC-8, one subtest per fixture | |
| `TestRegistrationSitesDerivesTheMutatorSet` | `internal/le/plugin/registrationsites/registrationsites_test.go` | AC-9 | |
| `TestBuiltZeRegistersTheDeclaredPlugins` | `internal/le/plugin/registered/registered_linux_test.go` | AC-10, AC-11 | |
| `TestGatedPluginTableMatchesManifest` (existing, reader moved) | `internal/le/plugin/gatedtable/gatedtable_test.go` | AC-12 | |
| `TestCouplingReadsTheDeclaredPopulation` | `internal/le/plugin/coupling/coupling_test.go` | AC-12 | |
| `TestAnswerAnswersTheInventory` (existing) | `internal/le/repo/inventory/inventory_test.go` | AC-13 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The checks take no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `registry-sealed-after-start` | `test/plugin/registry-sealed-after-start.ci` | A plugin that tries to register after start gets the sealed refusal and the daemon keeps running (AC-6) | |

### Interop Tests (Scope: protocol)
N-A: tooling scope, no wire-visible change.

## Files to Modify
- `internal/component/plugin/registry/registry.go` - seal flag and guard, the seal function, copy-on-read in `Lookup` and `All`, seal state in `Snapshot`, `Restore`, `Reset`, guard calls in `Register`, `AddConfigRead`, `RegisterBuiltinFamilies`, `AddRPCHandlers`
- `cmd/ze/main.go` - seal the registry before `dispatchMain`
- `internal/le/repo/inventory/registrations.go` - delete `Plugins`, `readRegistrations`, `registrations()`, the authority enum and the `external` and `assembly` fields; `walkRegistrations` keeps the opaque behaviour only
- `internal/le/repo/inventory/plugins.go` - delete the package function `Plugins`; `DeclaredPlugins`'s comment stops pointing at the proof
- `internal/le/repo/inventory/collectactualfiles.go` - `collect` reads the declared walk
- `internal/le/repo/inventory/source.go` - drop the proof-only assembly population fields
- `internal/le/repo/inventory/callablealiases.go`, `internal/le/repo/inventory/methodpromotion.go` - call the moved helpers
- `internal/le/plugin/gatedtable/gatedtable.go` - `Derive` reads `DeclaredPlugins`
- `internal/le/plugin/coupling/coupling.go` - `load` reads `DeclaredPlugins`
- `internal/le/verify/engine/stages.go` - add `plugin registration-sites check` beside `plugin coupling check`
- `internal/le/register.go` - blank import of the registration-sites tool
- `docs/architecture/plugin/feature-gates.md` - replace the contract section and trim the population section's proof paragraphs
- `docs/architecture/plugin/plugin-system.md` - registration closes when `main` starts
- `docs/contributing/gh-pages.md` - the declared-registrations paragraph
- `ai/META-DECISIONS.md` - the example in the website rule (a main-session edit, committed in the same turn)
- `plan/learned/DESIGN-HISTORY.md` - one row: whole-tree proof replaced by three fast checks
- `plan/post-release/engineering/spec-tooling-one-off-defects.md`, `plan/handover/35-tooling-one-off-actual-root-chain-2026-11-09.md` - AC-17
- Files to delete: `internal/le/repo/inventory/externalcertificate.go`, every other `internal/le/repo/inventory/external*.go` and `assembly*.go` and their tests, `INVENTORY-EXTERNAL-EFFECTS.md`, and the reboot-asset diagnostics `inventory-flowspec-range` and `inventory-fixture-refusal` under `plan/post-release/engineering/tooling-one-off-reboot-assets/diagnostics/`

## Files to Create
- `internal/component/plugin/registry/seal_test.go` - unit tests for AC-1 to AC-5
- `cmd/ze/main_seal_test.go` - the seal precedes dispatch
- `internal/le/plugin/registrationsites/registrationsites.go` and `register.go` - check 2 and its action
- `internal/le/plugin/registrationsites/registrationsites_test.go` and `testdata/` fixtures - AC-7 to AC-9
- `internal/le/plugin/registered/registered_linux_test.go`, with a `registered.go` holding the build and compare helpers - check 3
- `internal/le/repo/inventory/callsites.go` - the moved closure-binding, called-only and embedded-type-name helpers
- `internal/test/plugins/lateregister/register.go` - test-only plugin whose engine attempts a late registration
- `test/plugin/registry-sealed-after-start.ci` - AC-6

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No RPC or config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | le only: `./le plugin registration-sites check`, registered from `init()` in its own package |
| CLI grammar (keyword before value) | Yes | Takes no value; `./le cli grammar` judges it |
| Editor autocomplete | N-A | le action; completion derives from the registry |
| Functional test for new RPC/API | N-A | No RPC; the seal's functional test is `registry-sealed-after-start` |
| Pipe completeness | N-A | le check verdict through the shared renderer |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency: the seal is in-process state |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Operators see no new behaviour |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | `ze` commands unchanged; the le action appears in `le help` by registration |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | The test-only plugin is not shipped |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | Internal registry only; `docs/architecture/plugin/plugin-system.md` gains one sentence that registration closes when `main` starts |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/plugin/feature-gates.md` (the checks), `docs/contributing/gh-pages.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/feature-gates.md`, `docs/architecture/plugin/plugin-system.md`, `plan/learned/DESIGN-HISTORY.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The inventory's reader changes: `docs/contributing/gh-pages.md` and `feature-gates.md`; the plugin list itself does not change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/post-release/engineering/spec-inventory-fast-registry-checks.md` at implementation; known now: every `<!-- source: -->` anchor in `feature-gates.md` naming an `external*` or `assembly*` file goes with the section, and `ai/DOCS-TO-CODE.md` and `ai/CODE-TO-DOCS.md` regenerate. Declared by the changed files' `// Design:` headers and judged here: `docs/architecture/api/architecture.md` and `docs/architecture/system-architecture.md` (the registry's place in the plugin system: unaffected unless they describe registration as open after start, which the implementer checks and corrects if so), `docs/architecture/core-design.md` (`cmd/ze` composition: gains one sentence that `main` seals the plugin registry before dispatch), `docs/architecture/testing/verify-freshness-scope.md` (verify stages: gains the `plugin registration-sites check` stage if the page lists stages, otherwise unaffected) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the seal call in `main`, the guard, the test-only plugin, failing wiring tests
   - Tests: `TestMainSealsBeforeDispatch`, `registry-sealed-after-start`
   - Files: `cmd/ze/main.go`, `registry.go`, `internal/test/plugins/lateregister/register.go`
   - Verify: the functional test fails while the guard is a stub
2. **Phase: Seal and copies** -- AC-1 to AC-5
   - Tests: the `seal_test.go` units, the benchmark
   - Files: `registry.go`
   - Verify: red then green; scoped `internal/component/plugin/...` and `cmd/ze` package runs; allocation figures recorded
3. **Phase: Registration sites** -- AC-7 to AC-9, resolving R-1 per the confirmed site rule
   - Tests: the `registrationsites` units with fixtures
   - Files: the new package, `stages.go`, `internal/le/register.go`
   - Verify: red fixtures, green tree
4. **Phase: Runtime equals declared** -- AC-10, AC-11
   - Tests: `TestBuiltZeRegistersTheDeclaredPlugins` with its recorded break
   - Files: the new `registered` package
   - Verify: on Linux (the dev VM or CI), red under the break, green restored
5. **Phase: Readers move** -- AC-12, AC-13
   - Tests: gated-table, coupling, inventory answer
   - Files: `gatedtable.go`, `coupling.go`, `collectactualfiles.go`
6. **Phase: Delete the proof** -- AC-14, AC-15, after a `ze-test-judge` verdict on each deleted or rewritten test
   - Files: the deletion set, `registrations.go`, `plugins.go`, `source.go`, `callsites.go`
   - Verify: `./le repo compiles check`; scoped `internal/le/repo/inventory/...`, `internal/le/site/...`, `internal/le/plugin/...`
7. **Phase: Documentation and the superseded spec** -- AC-16, AC-17, the contract section in the same commit as phase 6

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The seal guard runs under `mu` before any write; a refused call leaves every index (`familyIndex`, `filterTypes`) untouched |
| Correctness | The clone deep-copies every slice and map field, nested ones included (`AttributeJSONDecoders`, `FilterInsertions`, doctor checks) |
| Data flow | Check 2 reads only the committed snapshot; check 3 builds only from the committed tree's tags |
| Rule: no-layering | No path to the proof survives: no flag, no slow mode, no `Plugins` alias |
| Rule: principles, silent zero | Check 3 refuses an empty runtime list or an unparsable payload rather than comparing two empties |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Seal and copies | `seal_test.go` passes under `./le job run` |
| Registration-sites check | `./le plugin registration-sites check` passes at HEAD; the verify stage list names it |
| Runtime equals declared | `TestBuiltZeRegistersTheDeclaredPlugins` passes on linux/amd64 with the break recorded |
| Proof deleted | `git ls-files internal/le/repo/inventory` lists no `external*` or `assembly*` file; grep finds no `externalCertificate` or `Snapshot.Plugins` |
| Docs | `./le doc check all` passes; the contract section heading is gone |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open guard | A registering function that skips the guard: AC-9's derived set, and a test that lists every exported registry function writing the plugin map or a registration |
| Shared state | A returned registration still sharing a map with the registry (R-5) |
| Test seam leak | The seal can be undone only by the test-only `Reset` and `Restore`; no exported unseal |

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

- Superseded by this spec, in `spec-tooling-one-off-defects`: the open actual-root refusal chain (its "Actual-root inventory progress" section, and handover 35's "Next" items 1 and 2 and "Last run" actual-root stop); the package fixture work on `TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection`, which judges the proof's metadata write rule; and the completeness half of AC-32 and P-32 (`TestInventoryUsesCompleteCommittedPopulation` keeps its darwin and dirty-edit cases). No other AC there depends on the proof.
- Journal row that closes when AC-12 lands, still owned by `spec-tooling-one-off-defects`, which moves it to its "Rows found already fixed": `gate-verdict-depends-on-the-machine` 2026-10-29, "coupling check judges with the host plugin list". The coupling check already reads a committed snapshot; only the proof's refusal keeps it red.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Replace the whole-tree proof with three fast checks | Keep fixing the proof's refusals one vendored construct at a time | Owner decision ("yes replace"); the proof cost an hour and 32 GB per run and never went green |
| Where a registering call may sit: in a `register*.go` file (the convention the edit hook's init() rule already holds, `internal/le/hookruntime/writeedit.go`), as a direct call never inside a function literal or a method, inside `init()`, a package-level initializer, or a plain function whose every reference is in turn a direct call that only such initialization of a `register*.go` file reaches, followed across Ze packages; an exported function under `pkg/` never qualifies; the registry is never dot-imported | The owner's literal wording, `init()` of `register.go` only; the earlier reading here, which stopped at one helper level inside the same file | Phase 3 implemented this reading (`internal/le/plugin/registrationsites`, `judge`, `initializationOnly`) and the main thread still confirms it. The literal rule is red at HEAD (R-1) on the seven helpers. The one-level reading is red on the policy chain: `AddConfigRead` (`policy/register.go` `addPolicyConfigRead`) is reached from `policy.MustRegisterRIBImport` (`rib.go`), which the Loc-RIB importers call from their own `register.go` `init()`; the policy package cannot know those roots at its own init, so moving the call would make each importer write another plugin's registration. The check at HEAD refused exactly two sites under this reading, both moved in phase 3 with behaviour kept: `bgp/message/family.go` `registerBuiltinFamilies` to `register_families.go` (same package-variable initializer), and the memlock lock-and-record `init()` from `memlock_linux.go` to `register_lock.go` |
| The registering-function set is the registry functions that call the seal guard | A hand-kept list of names in the check | One declaration (`ai/rules/principles.md`): a new mutator is covered when it calls the guard, and the security checklist makes skipping the guard visible |
| Seal in `cmd/ze` `main`, not in the plugin server's start | Seal when the plugin server starts | `main` is reached only after every `init()`, in every personality, and no test calls it, so test registrations keep working; the server start is later and leaves a window |
| `Lookup` and `All` return pointers to clones | Return `Registration` values | Keeps the 48 callers' nil checks and signatures; the clone gives the same protection |
| Check 3 reads `show plugin list` from a built `ze` | Compare `registry.All()` inside the le process | The owner named the built binary; the shipped personality, not le, is what must match |
| Check 3 is a Linux build-tagged test, not a verify stage | A stage that refuses on darwin | A stage red on the Mac is a machine-dependent verdict; the test does not compile off Linux, which states the platform scope in the build itself |

## Phase results

- Phase 2 (AC-4), go1.27.3 linux/amd64, 110 synthetic registrations with one
  element in every slice and map (so it over-states real registrations):
  `Lookup` 3419 B/op and 81 allocs/op, `All` 378792 B/op and 8912 allocs/op
  (`BenchmarkLookupAndAll`). Census of the 46 non-test callers: start-up
  (plugin/inprocess, resolve, server startup and autoload, declarations, boot),
  config (plugin_verify, graph, optional_provider, bgp/config, bgp/plugin
  insertion set), command and help (command/declared, plugin/cli, plugins/help,
  aihelp, le tooling), one warning path (reactor `rsRelayProcessName`). The two
  per-event callers moved to value accessors (A-4).
- Phase 2: `SetNTPSyncProvider` stays a runtime seam because AC-2 names it one,
  although its only caller is `internal/plugins/ntp/register.go` `init()`.
- Phase 3 (AC-7 to AC-9): `./le plugin registration-sites check`
  (`internal/le/plugin/registrationsites`) derives the registering functions
  as the registry's exported functions that reach `refuseSealedLocked`, judges
  each site by the rule in Key Design Decisions, and runs as a verify stage
  beside `plugin declarations check`. Import aliases resolve through
  `repoinventory.Snapshot.ImportedDirectory`, a wrapper over `callableImport`.
- Phase 4 (AC-10): `internal/le/plugin/registered` builds `ze` with
  `DaemonBuildTags(root, DaemonBase)` and cgo off, reads
  `ze cli -c "show plugin list | json"` (served in-process by
  `command.ServeLocal`, so a running daemon never answers; the `| json`
  rendering is the bare row array, not the `{plugins: [...]}` envelope), and
  compares it with `DeclaredPlugins` in `Compare`. Green at 10a0471714 with 96
  plugins. The binary is built from the working tree while the walk reads
  HEAD, so a failure on a dirty checkout lists the uncommitted Go files.
  AC-11's recorded break is prepared and not yet run: the declared walk
  treating `bgp-hostname`'s `registry.Register` call as opaque (expected red:
  registered at run time and absent from the declared list), and the other
  direction, the hostname blank import dropped from `all_ze_bgp.go`.

## Known Limitations
- Check 3 compares the fields `show plugin list` carries. The other registration fields are guarded by the declared walk's metadata refusals at the source and by the seal and copies at run time.
- The hand-kept lists in `TestAllPluginsRegistered` and `TestAvailablePlugins` stay; they are not this spec's work.

## RFC Documentation (Scope: protocol)

N-A: tooling scope.

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
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-inventory-fast-registry-checks.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
