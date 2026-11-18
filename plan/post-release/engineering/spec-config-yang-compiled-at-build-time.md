# Spec: config-yang-compiled-at-build-time

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P3 |
| Release reason | Release-independent config yang compiled at build time work; historical findings do not establish a current product defect. |
| Scope | config |
| Depends | `plan/release/fixes/spec-config-yang-definition-order.md` and `plan/release/fixes/spec-config-yang-one-definer-per-node.md` land first (working order of 2026-11-18, "Order against the two release fixes") |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decisions and statements, in his words:

| Date | Subject | Words |
|------|---------|-------|
| 2026-10-16 | The goal | "compile [the YANG] at build time, use go:generate script and import" |
| 2026-10-16 | Why the compiler stays in `ze` | "I want people to be able to write their own plugins in Go or any other language and load them at startup" |
| 2026-11-17 | Start the work | "Complete in that case, write a spec for that optimization and complete the work we have done there. We can start that optimization in a different tree closed phase nine." |
| 2026-11-18 | Q11 / OD-1: per-module generated Go committed beside each `.yang`, against "way 3" (one prepared file per build) | "Per module." |
| 2026-11-18 | OD-5: may the generator refuse a leafref whose target lies outside its module's import closure | "No leaf ref must work cross module." |
| 2026-11-18 | OD-4: child order, after a before and after example | "Yes, I keep the order of the definition." (his decision D-1 of 2026-10-28 stands) |
| 2026-11-18 | OD-2: may the `--plugin` text hand-over and the test of its case be deleted | "Okay, do what you need." |

OD-3 is not a new owner answer. The main session told him on 2026-11-18 that
it would treat both release fixes as landing first unless he objected, and he
did not object. That sequencing stands under his "ok" of 2026-11-17 on the
one-definer fix.

History, one line: on 2026-11-17 the owner answered "3" to three ways of
pre-parsing the YANG, before the recorded design was put to him; on
2026-11-18 he chose per-module compilation, and way 3 is no longer an
alternative.

Today every process that needs the schema parses the YANG text of every linked
module and resolves it with goyang at run time. A `go:generate` step MUST
instead produce Go that the packages import, so no compiled-in module is parsed
or resolved at run time. Every check `Loader.Resolve` runs today (extensions,
patterns, `checkStructure`, enum assignment, imports, leafref paths, duplicate
augmented nodes) and every check the config schema build records
(`recordSchemaBuildError`) MUST run at generation, so a bad module fails the
build and never reaches a daemon. A leafref that crosses a module boundary
MUST work, compiled-in or external, and no valid YANG is refused to keep the
design simple.

This spec also owns the cost of the second schema build that a declared
external plugin adds to a start, handed over by
`plan/post-release/features/spec-plugin-external-yang-at-startup.md` (R-19,
Q10). AC-18 closes it.

Scope widened 2026-11-18 (a small defect found in step P2 and fixed in the work
in hand, `91aa35862c`): an `augment` that adds a node its target already holds
was dropped in silence. Every resolve now refuses it (AC-20).

| Symptom | Evidence |
|---------|----------|
| Forty non-test call sites in 31 files ask for or build a loader (re-measured 2026-11-18) | Listed under Current Behavior. Nine build a loader of their own with `NewLoader`; 31 call `DefaultLoader` and share its one cached set |
| A broken module is found at daemon start, not at build | `DefaultLoader` returns the joined error at run time; `exaBGPSchema` (`internal/exabgp/migration/schema.go`) answers nil on any load error |
| The schema build is the largest single cost of every start | Indicative profile of 2026-11-17 below: about 290 of 430 ms |
| A start that declares one external plugin builds the module set twice | `yang.resolveDefault`, then `resolveJoined` over the same compiled-in texts plus the plugin's module |
| No longer a symptom (checked 2026-11-18): the editor validator rebuilt the schema on every call, and the CLI editor tests timed out under `-race` | `config.ValidateCustomSections` calls `YANGValidator()`, built once per module set by `yangValidatorCache` |

### Indicative evidence of 2026-11-17

Gathered on the server, a VM on a host of unknown speed. No figure here is a
recorded figure (`ai/META-DECISIONS.md`, "Test systems and labs"); the figures
that close an acceptance criterion come from the owner's Mac.

| Config, median of 12 to 15 starts | Before the external-plugin work | After it |
|-----------------------------------|---------------------------------|----------|
| No external plugin | 429 ms | 431 ms |
| Go fixture plugin declared | 524 ms | 839 ms |
| Python example declared | 500 ms | 799 ms |

CPU profile, 13 starts merged: `yang.resolveDefault` about 290 ms per start;
a declared plugin adds `joinDeclaredPlugins` → `PublishExternalModules` →
`Join` → `resolveJoined`, about 275 ms more. One build splits roughly into
110 ms of text load and 170 ms of `Resolve`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - the loader, `Resolved`, every check, the join, the publish, the decided build-time design (section 3)
  → Constraint: `Resolved` is the only type a reader of a checked module set takes; the linked compiled schema MUST keep that property.
  → Constraint: the structural refusals (`ErrLengthOrder`, `ErrEnumValue`, `ErrEnumRestriction`, `ErrExtensionSubstatement`), `ErrUndeclaredExtension` and `ErrLeafrefPath` each name module, file location and fault; the generator MUST print the same wrapped errors with the same location.
  → Constraint: an enum's value is Ze's (`parseEnumAssignment`), never goyang's.
  → Decision: "A binary links every module a linked module imports, because the generated `register.go` of each `yang/` package blank-imports the packages registering its modules' imports". This makes a module's import closure fixed and always linked with it.
- [ ] `docs/architecture/config/syntax.md` - D-1, the order every printer follows (after the definition-order fix)
  → Constraint: a block prints its own children in source order, a `uses` at its statement's position, then augmented children by module name, then augment order. The linked value MUST give that order whatever Go init order is.
- [ ] `docs/architecture/command-ownership.md` - "YANG as Data, Not Code"
  → Constraint: the only hand-written file in a `yang/` package is the `.yang`; every Go file there is generated by one tool (`./le yang glue`).
- [ ] `docs/architecture/module-tiers.md` and `feature-gates.txt` - compile-out
  → Constraint: a gated feature's `yang/` package is reached only through a tagged blank import in `plugin/all`; the `yang/` package files carry no build tag, so a generated file there compiles exactly when its module is linked.
  → Decision: no generated output may depend on the tag set chosen at `go build`.
- [ ] `docs/contributing/ze-go-style.md` - "One type per lifecycle state", "A limit on everything"
  → Constraint: the linked schema is a state type of its own; a zero or nil value ends in a `BUG` panic as `Resolved` does today.
- [ ] `plan/post-release/engineering/spec-validated-construction-and-state-types.md` T1 and T4 - why `Resolved` exists
  → Constraint: nothing is best-effort; the generator inherits the strict policy.

### RFC Summaries (Scope: protocol)
- `rfc/short/rfc7950.md`: no wire change. The leafref rows (Sections 6.4, 6.4.1, 9.9, 9.9.2) and their tests keep their meaning and move with the code (Documentation row 9).

**Key insights:**
- Module census of 2026-11-18 (every `.yang` under `internal`, `cmd`, `pkg`, `testdata` excluded): 235 module files, 105 `augment`, 0 `deviation`, 0 `if-feature`, 0 `feature`, 7 `identity` (none used by an `identityref`), 0 `identityref`.
- Leafref census of 2026-11-18: 18 `type leafref` in 9 modules. 17 are relative paths inside their own module: `ze-geodns-conf` 1, `ze-isis-conf` 4, `ze-ospf-conf` 2, `ze-traffic-control-conf` 1, `ze-ipsec-conf` 4, `ze-authz-conf` 1, `ze-bfd-conf` 2, `ze-firewall-conf` 2 (inside `grouping then-block`, used only in its own module). One crosses a module boundary: `ze-tacacs-conf`, absolute and prefixed, `/authz:system/authz:authorization/authz:profile/authz:name`, into `ze-authz-conf`, which it imports. None is in a typedef, in an `augment`, or in a grouping another module uses. After the one-definer fix the tacacs path becomes `/sys:system/authz:authorization/...` (that spec, "Search evidence"): still a module it must import.
- A leafref verdict depends only on the leaf's import closure (section "Cross-module leafrefs"). The set-level refusals of the join (name and namespace clash, idle module, top-level redefinition, command path conflict, unheld registration, first-pass addition, duplicate augmented node) judge a set and stay at the link.
- An augmenter's `yang/` package blank-imports its target's package, so Go init order registers the target first.
- goyang cannot be fed from Go literals: `Statement` keeps its substatements in an unexported field with no constructor, `EnumType` has unexported fields, `Entry.Dir` is a map.
- Measured on 2026-10-16 (90 modules, not re-measured at 224): 2,081 entries, 1,146 leaves, 2,506 extension statements, 308 enum values, 134 patterns, depth at most 8.
- The generator MUST NOT import a package holding output of the YANG generator, or a stale generated file stops the generator from building (bootstrap). Read 2026-11-18 with `go list -deps`: `config/yang` imports goyang, `internal/component/command` and core packages; the deps of `config/yang` and of `internal/le/yang/glue` hold no `yang/` package and no YANG-glue output. They do hold `plugin/registry/gated_plugins_gen.go`, the output of `./le plugin gated-table update`, which is not this generator's.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/register.go` - `RegisterModule(name, content)`; `RegisterModuleForTest`; `Modules()`
- [ ] `internal/component/config/yang/loader.go` - `Loader`, `Resolved`, `DefaultLoader` (`moduleSetLocked`, `resolveProcessSet`, `resolveDefault`), `sourcedModules` (`markJudged`, `sourceModules`, `augmentErrors`)
- [ ] `internal/component/config/yang/join.go` (`Join`, `ExternalModule`, `ExternalModuleFromText`, `declaredModules`, `resolveJoined`, `Resolved.reloaded`, `failingModule`, the set refusals), `join_command.go` (`refuseCommandConflicts`), `publish.go` (`PublishExternalModules`, `swapModuleSet`, `ModuleSetFixed`, `KeepsModuleSet`, `PublishedModule`, `rebuildCommandArgDefs`), `registration_held.go` (`unheldName`, `RefuseUnheldCompiledRegistrations`)
- [ ] `internal/component/config/yang/loader_reference.go` (`checkReferences`, `checkLeafrefSchema`, `checkLeafrefLoop`) and `reference_path.go` (`leafrefTarget`, `topLevelParents`, `prefixedModuleRoot`, `checkPathPrefix`, `checkStepNamespace`, `pathStatement`, `newPathContext`, `rootPathFix`); `reference_source.go` (`RegisterReferenceSource`, the `ze:reference` sources)
- [ ] `internal/component/config/yang/loader_structure.go`, `loader_abnf.go`, `loader_grammar.go`, `loader_source.go`, `loader_stages.go`, `enum_assignment.go`, `pattern.go` - the Resolve-time checks and the staged benchmark entry points
- [ ] `internal/component/config/yang_schema.go` - `PluginOnlySchema`, `YANGSchema`, `YANGValidator` (`resolvedCache`), `loadYANGModules`, `JoinExternalModules`, `PublishExternalModules`, `RefuseUnheldRegistrations`, `YANGValidatorWithPlugins`, `YANGSchemaWithPlugins`, `schemaFromResolved`, `validatorFromResolved`, `sortedKeys`, `yangChildOrder`, `validateTypedefDefault`, `validateDefaultNotIfFeature`, `recordSchemaBuildError`; `schema.go` `Schema.Define`, `mergeContainer`
- [ ] `internal/component/config/declared_schemas.go`, `cmd/ze/hub/declared_plugins.go` (`joinDeclaredPlugins`), `cmd/ze/hub/build_registrations.go` (`refuseWronglyBuilt`), `internal/component/plugin/server/stage1_schema.go` (`refuseChangedSchema`)
- [ ] `internal/component/plugin/inprocess.go` (`CollectPluginYANG`, `GetInternalPluginYANG`), `internal/component/config/loader.go` (`loadConfig`, `MergeCliPlugins`), `internal/component/bgp/config/loader.go` (`loadReactorFile`), `cmd/ze/ze_core_dispatch.go` (`zeParseGlobalFlags`)
- [ ] `internal/component/plugin/registry/registry.go` - `Registration.YANG string`; 73 non-test sites set it, each from a generated `<Var>YANG` variable
- [ ] `internal/component/config/schema/cli/main.go`, `internal/exabgp/migration/schema.go`, `internal/le/yang/glue/*.go`
- [ ] goyang with `internal/le/vendorpatch/patches/goyang-*.patch`: `pkg/yang/entry.go`, `types.go`

**Runtime functions that build or ask for a loader today (non-test; 40 sites in 31 files, 2026-11-18):**

| File | Functions |
|------|-----------|
| `internal/component/config/yang/join.go`, `loader.go`, `enum.go`, `command_argdefs.go` | `Resolved.reloaded` (the join), `resolveDefault` (the shared set), a direct `DefaultLoader` call, the `modelArgDefs` fill |
| `internal/component/config/yang_schema.go`, `infra/authz.go`, `schema/cli/main.go`, `cli/cmd_edit.go`, `yang/cli/tree.go` | `loadYANGModules`, `JoinExternalModules`, `SchemaOf`; `validateMatchEntries`; `loadAPIRPCs`; `buildEditorCommandTree`; `addConfigNodes`, `addCommandNodes`, `AllRPCDocs`, `loadRPCParams` |
| `internal/component/l2tp/pppconfig/lcp.go` | `loadLCPDefaults` (reads `DefaultLoader` since P3) |
| `cmd/ze/hub/build_registrations.go`, `session_factory.go`, `command_meta.go`, `service_web.go` | `refuseWronglyBuilt`, `buildCommandTree`, `commandMetaSource`, `startWebServer` |
| `internal/component/plugin/server/server.go`, `stage1_commands.go` | `NewServer`, the Stage 1 command check |
| `internal/component/aihelp/aihelp.go`, `iface/validate.go`, `cli/completer.go`, `cli/validator.go`, `cli/client/main.go`, `cli/testing/headless.go` | `CLISubcommands`, `loadSchema`, `Build`; `loadReservedIfaceNames`; `NewCompleter`; `NewConfigValidator`; `buildYANGState`; the headless setup |
| `internal/exabgp/migration/schema.go` | `exaBGPSchema` |
| `internal/le/...` | `arch/enumeration/corpus.go`, `doc/yangcontract/contract.go`, `usage.go`, `helpshape.go`, `config/claims/configclaims.go`, `cli/dispatch/resolver.go`, `cli/list/commandlist.go`, `cli/grammar/cligrammar.go`, `repo/inventory/inventory.go` |

**goyang reach** (2026-11-18): 47 non-test files import `github.com/openconfig/goyang/pkg/yang` (39 runtime, 8 under `internal/le`); 315 `*yang.Entry` type references in non-test code; 43 test files import goyang; 156 test files call `DefaultLoader` or `NewLoader`; 112 load fixture text; 200 distinct test files do one or the other; 50 call `RegisterModuleForTest`; 175 `embed.go` in `yang/` packages.

**Allocations per build, 224 modules** (P0, `BenchmarkSchemaBuildStages`, Linux VM; allocation figures are valid on any host):

| Stage | Allocations | Bytes |
|-------|-------------|-------|
| load (module texts) | 243,062 | 19.5 MB |
| process (goyang) | 191,737 | 11.3 MB |
| check-extensions | 598 | |
| check-patterns | 11,132 | |
| check-structure | 3,879 (5,510 before P1) | 526,248 (1,114,544 before P1) |
| check-references | 114 | |
| whole resolve | 207,462 | |
| lower to `config.Schema` | 35,206 | 3.3 MB |

Text load and goyang's processing are about 96% of a build's allocations; Ze's
own checks under 4%. The lowering adds about 7% more.

**External plugins today** (read at `8fb9a9295e`):

| Fact | Evidence |
|------|----------|
| An external plugin's modules (`-conf`, `-cmd`, `-api`) are joined at `ze start`, between the first pass and the full parse | `runYANGConfig` → `joinDeclaredPlugins` → `config.PublishExternalModules` → `yang.PublishExternalModules` → `yang.Join` |
| The join works from the TEXT of every compiled-in module: it parses each again and runs goyang's processing over the whole set; Ze's extension, pattern and structure checks skip the compiled-in modules (`markJudged`, P2); the leafref check judges the whole set | `resolveJoined`, `Resolved.reloaded` |
| The join judges the joined set with refusals no single module answers | `declaredModules`, `refuseIdleModules`, `refuseRedefinedRoots`, `refuseUnheldRegistrations`, `refuseFirstPassAddition`, `refuseCommandConflicts`; the table "The join refuses each case below" of the design page |
| The set is published once per process, before any keeper reads it, and never changes while the daemon runs (owner ruling 2026-11-15, "the plugin can only be changed on restarting ze") | `swapModuleSet`, `ModuleSetFixed`, `KeepsModuleSet`; `RefuseChangedDeclarations`; `refuseChangedSchema` |
| The schema caches follow the publish | `resolvedCache` keys on the `*yang.Resolved`; `rebuildCommandArgDefs` |
| With no daemon, no external plugin starts and its section is "not available" (owner ruling 2026-11-16) | `config.SchemaWithDeclaredPlugins`, `DeclaredSchemas.ParseRefusal` |
| A compiled-in module naming an unheld `ze:validate` or `ze:reference` name refuses `ze start`; an external one refuses the join; one judge | `refuseWronglyBuilt`, `RefuseUnheldCompiledRegistrations`, `unheldName` |
| An external and a compiled-in plugin differ only in transport (owner ruling 2026-11-17) | `docs/architecture/plugin/plugin-system.md`, "One plugin, two ways to reach the bus" |

**What `--plugin` does, and the text hand-over it carries** (read 2026-11-18):

| Question | Answer |
|----------|--------|
| Which command carries it | `ze` itself: `--plugin <name>` is a global flag of the `ze` binary, read by `zeParseGlobalFlags` (`cmd/ze/ze_core_dispatch.go`) into the plugin list `ze start`, `ze <config>` and the hub receive (`hub.Run`, `cmdStart`) |
| What it does for an operator | It adds a plugin to the start as though the config's `plugin` block declared it: `ze.<name>` for a plugin compiled into `ze`, a program for an external one (`MergeCliPlugins`, `config/loader.go`). That behaviour is not touched by this spec |
| The text hand-over it also carries | For each `--plugin` name that is a compiled-in plugin, `CollectPluginYANG` (`plugin/inprocess.go`) takes that registration's `Registration.YANG` text, and `loadConfig` and `loadReactorFile` hand it to `YANGSchemaWithPlugins` / `YANGValidatorWithPlugins`, which join it (`loadYANGModules`) unless the set already holds the identical text (`Resolved.HoldsModuleText`). An external program name yields no text |
| For whom it does work | Only for a compiled-in registration whose module no `init()` registered. Every non-test `Registration.YANG` names a generated variable of its own `yang/` package, and naming that variable imports the package whose `init()` registers the module, so the set always holds the text and the hand-over adds nothing. Only a test that registers a plugin with a literal text reaches it (`TestReloadVerifierRetainsSelectedPluginSchema`, `reloadOptionalYANG`) |
| Under per-module compilation | The case cannot exist: a compiled `Registration.YANG` is the module value, which its own package registers |

**Behavior to preserve:**
- The published external module set and the publish-once rule (`PublishExternalModules`, `ModuleSetFixed`, `KeepsModuleSet`, `PublishedModule`), and every refusal of the join with its text.
- The `--plugin` flag: a plugin named on the command line still starts, and config under its root still parses.
- The config corpus sweep that publishes fixture modules before it parses; the build-tag gate tests (`TestBuildTag_Gate*` in `cmd/ze/hub`): a generated input compiles under every tag set.
- Every refusal text and error sentinel the loader produces, now produced by the generator, or by the join for an external module, with the same file location.
- `ze schema show <module>` prints the module's YANG source (`test/parse/cli-schema-show.ci`).
- Child order D-1 as the definition-order fix builds it (owner, 2026-10-28, reaffirmed 2026-11-18).
- One definer per config node as the one-definer fix builds it: a shared root is defined by one module and augmented by the others.
- The `Resolved` accessor surface and the `BUG` panic on a zero value.
- The held-registration judge at start for compiled-in modules and at the join for external ones (`unheldName`): registrations are made by `init()`, so generation cannot judge them.
- The first-pass schema cut from the full one (`PluginOnlySchema`) and the refusal of an external node under a first-pass root.
- Every binary links the import closure of every linked module (generated blank imports).

**Behavior to change:**
- No run-time parse or resolve of a compiled-in module. The compiler stays in `ze` for external modules; a start with no external plugin calls it zero times (AC-7).
- Schema defects fail `go generate` / `./le yang glue update` and the freshness check, never a daemon start.
- A start that declares an external plugin compiles that plugin's modules against their import closure and links them to the compiled-in value; it does not parse or resolve the whole compiled-in set (AC-18).
- The `--plugin` text hand-over (`CollectPluginYANG`, the module-text arguments of `YANGSchemaWithPlugins` and `YANGValidatorWithPlugins`, `loadYANGModules`, `Resolved.HoldsModuleText`) is deleted in C3, with the design page section "Joining a module that no `init()` registered" corrected and `TestReloadVerifierRetainsSelectedPluginSchema` removed in the same commit (owner go-ahead 2026-11-18). The flag stays.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The loader and its checks | `config/yang/loader.go` `Loader`, `Resolve`; `loader_structure.go`, `loader_abnf.go`, `loader_grammar.go`, `enum_assignment.go`, `pattern.go` | Parses every registered text and resolves the set in each process | Replaces at run time for compiled-in modules: the same code becomes the compiler, run at generation and for external modules; it moves to `config/yang/compiler` in C3 (package layout) |
| The checked set type | `loader.go` `Resolved` | The only type a reader of a checked module set takes | Extends: it becomes the linked compiled schema, same guarantee and accessors |
| The generated YANG glue | `internal/le/yang/glue`, `./le yang glue update` and `check` | Generates `embed.go` and `register.go` of every `yang/` package, committed, with a freshness check | Extends: it also emits `compiled.go`; `embed.go` goes |
| Freshness of generated Go inputs | `TestYANGGlueCurrent`, `TestGeneratedPluginImportsCurrent`, `generationActions` and `generationChecks` (`internal/le/repo/generate.go`) | Every generated Go build input is committed and checked | Extends: the compiled output joins the same lists |
| Module registration from `init()` | `config/yang/register.go` `RegisterModule`; each `yang/` package's `register.go` | Registers module text; blank imports order a module after its imports | Extends: registers the compiled module and links its grafts |
| The schema caches | `config/yang_schema.go` `resolvedCache`, `yangSchemaCache`, `yangValidatorCache` | Build the schema and validator once per module set, keyed on the `Resolved`, so they follow the publish | Extends (OD-8): kept, keyed on the linked value. The lowering is not compiled |
| `DefaultLoader`'s one-entry cache and the registry generation | `loader.go` `DefaultLoader`, `moduleSetLocked`, `resolveDefault`; `register.go` `modulesGeneration`, `RegisterModuleForTest` | Resolves the registered set once per generation; after a publish holds the joined set | Replaces its producer (C3): the linked value init builds. The generation key goes with `RegisterModuleForTest` (AC-14); the swap of the publish stays |
| The join of an external module | `config/yang/join.go` `Join`, `declaredModules`, `resolveJoined`, `reloaded`, `failingModule`, the set refusals; `join_command.go` | Parses every compiled-in text again, resolves the whole set with the external modules, then judges it | Extends: the seam the external-plugin spec built for this spec. The compile of the external modules against their import closure moves before it (`compiler.CompileExternal`); `Join` links compiled modules and keeps every set refusal and its text. Its input type changes from texts to compiled modules; its callers stay |
| The `--plugin` text hand-over | `config/yang_schema.go` `loadYANGModules`, `YANGSchemaWithPlugins`, `YANGValidatorWithPlugins`; `Resolved.HoldsModuleText`; `plugin/inprocess.go` `CollectPluginYANG`; `config/loader.go` `loadConfig`; `bgp/config/loader.go` `loadReactorFile` | Joins the `Registration.YANG` text of a compiled-in plugin named with `--plugin` when no `init()` registered it | Replaced by nothing in C3 (owner go-ahead 2026-11-18): under per-module compilation a registration's module is always registered by its own package |
| The published external layer | `config/yang/publish.go` | Publishes the joined set once per process, before any keeper, and fixes it | Extends: it publishes what `Join` answers, now a linked value |
| The offline schema step and the first-pass schema | `config/declared_schemas.go`; `config/yang_schema.go` `PluginOnlySchema`, `yang.FirstPassRoots` | With no daemon, fixes the set to the compiled-in one; cuts `plugin` and `environment` from `YANGSchema` | Does not apply a change: both read the compiled-in linked value |
| Held-registration judge | `config/yang/registration_held.go`; `cmd/ze/hub/build_registrations.go` | Refuses a module naming an unheld `ze:validate` or `ze:reference` name | Extends: stays a start-time check over the linked value, ported to the compiled node in C2 |
| The leafref checks | `config/yang/loader_reference.go` `checkReferences`; `reference_path.go` | Refuse a path naming no leaf, a prefix the module where the path is written does not declare, an unprefixed name outside the referring node's namespace, a config leaf requiring an instance of state, a loop | Extends: ported to the compiled form in C2 (reader code in `config/yang`), run by the compiler over each compile unit; closure-local (section "Cross-module leafrefs") |
| The duplicate-augment refusal and the judged mark | `loader.go` `augmentErrors`, `markJudged`, `sourceModules`; `loader_stages.go` `ErrRecheckJoinedSet` | Refuse a node an augment adds twice; skip compiled-in modules in the statement checks of a join | Extends: both move to the compiler; `Link` carries the duplicate check for compiled grafts (table "Choices of the redesign") |
| The rule every module obeys | `join.go` `moduleUseOf`, `IdleModuleNames`, `EmptyModuleNames`; `TestNoShippedModuleAddsNothing`; `TestCompiledModulesNameOnlyHeldRegistrations` | One judge for both sets | Extends: the generator runs the same judges over the whole tree |
| The Stage 1 schema comparison | `plugin/server/stage1_schema.go` `refuseChangedSchema` | Compares a restarted plugin's declaration with the published set | Does not apply a change: it compares declarations |
| Values kept for the process | `plugin/server/server.go` `NewServer`, `gnmi/server.go` `NewServer`, the backend gate schemas, `iface.ValidateIfaceName`, `cli/client` `loadYANGState`, `cli/testing/headless.go`, `Completer.valueSchema` | Built once from the module set; each keeper records itself or keys on the `Resolved` | Extends: each reads the linked value; C3 measures the other caches and deletes those with no measured need |
| Resolve error census | `config/yang/schema_census_test.go` | Refuses a caller that drops the schema's error | Extends: narrows to the external compile in C3 |
| The D-1 order function | `plan/release/fixes/spec-config-yang-definition-order.md` (replaces `sortedKeys` and `yangChildOrder`) | Orders a block's children by D-1 over goyang entries, once that fix lands | Extends: the emitter uses it for declared children, `Link` orders grafts by the same key, and it is the reference of the equivalence test |
| One definer per config node | `plan/release/fixes/spec-config-yang-one-definer-per-node.md` (`Schema.Define` refuses a second definition) | Converts the shared roots to `augment`, once that fix lands | Does not apply a change: the compiled form compiles the converted roots as grafts and carries no merge |
| goyang with vendor patches | `vendor/github.com/openconfig/goyang`, `internal/le/vendorpatch/patches/goyang-*.patch` | Upstream goyang with Ze's fixes applied at build time | Does not apply at run time for compiled-in modules after C3; the compiler keeps it |
| Design history entry | `plan/learned/DESIGN-HISTORY.md`, Configuration, Abandoned approaches, "Generating a Go schema from YANG at build time" (166) | Records the reversal of 2026-10-16 | Does not apply a further change |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: a `.yang` file in a `yang/` package (and `exabgp.yang`, moved into `internal/exabgp/migration/yang/`). Format: YANG 1.1 text.
- Run time: Go package init of each linked `yang/` package; at `ze start`, the Stage 1 module list of each declared external plugin.

### Transformation Path
1. Generation (`go:generate` directive, or `./le yang glue update`): discover every module; for each module M, compile its unit (M plus its transitive import closure, every graft of the unit applied) with goyang through the compiler; run every Ze check over the unit, the leafref check over the leaves M instantiates; emit M's own resolved tree (uses, groupings and typedefs expanded against the closure, declared children in D-1 source order), its RPCs and notifications, and each `augment` as a graft record (target path with module-qualified segments, its D-1 key, the nodes to attach). Write `compiled.go` and `register.go` beside the `.yang`.
2. Whole-tree check at generation: link every module in the tree at once. A conflict two modules raise against each other (a node added twice, a command path held twice) needs both modules linked, so any tag set that could meet it is met here; per-module output does not depend on the set (leafrefs included: section "Cross-module leafrefs").
3. Init: each generated `register.go` registers its compiled module, which sets the current linked value to `Link(current, module)`. `Link` never writes its input: it copies the path from the root to each graft target and attaches the grafts there, in D-1 position. It answers an error value for a conflict; registration turns that into a `BUG` panic naming the module, because generation proved no conflict exists.
4. Start, with external plugins: `runYANGConfig` runs the first pass over the compiled-in value, starts each declared external plugin and reads its Stage 1 module list. `config.PublishExternalModules` calls `compiler.CompileExternal`, which compiles the declared modules against their import closure, rebuilt from the closure modules' `Source` text (never the whole set), and refuses an invalid module naming the plugin and the module. `yang.PublishExternalModules` then calls `yang.Join`, which links the compiled modules into the compiled-in value, turns a `Link` conflict into the operator refusal of today, runs every set refusal on the linked value, and the publish swaps it in. The set is fixed until `ze` restarts.
5. Read: every reader takes the one linked value (the successor of `Resolved`): `config.Schema` build, `BuildCommandTree`, validator, completer, metadata, RPC extraction, aihelp, authz, iface, MCP and web metadata, the join's set refusals, the held-registration judge.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build time to run time | Generated Go literal of the Ze-owned compiled module type, static data, no decode | No |
| `yang/` package to the schema registry | `init()` in the generated `register.go` | No |
| Plugin protocol Stage 1 schema list (`DeclareRegistrationInput.Schema`) | Each module's text is compiled at `ze start` and linked before the full parse; the wire is unchanged | Yes, for today's join (`test/plugin/external-yang-command-argument.ci` and the external-plugin spec's other `.ci` tests) |

### Integration Points
- `Resolved` (`config/yang/loader.go`) becomes the linked compiled schema; its consumers keep taking it.
- `./le yang glue` (`internal/le/yang/glue`): the one generator of `yang/` package Go files.
- `Link`: one function, at init over compiled-in modules and in `yang.Join` over external ones.
- `yang.Join`, `yang.PublishExternalModules`: the seam the external-plugin spec built; the body changes, every refusal stays.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | Every reader goes through the linked value; no reader keeps a loader |
| No unintended coupling | Yes (design) | Components register through `init()`; the schema registry imports no component |
| No duplicated functionality | Yes (design) | The compiler is today's loader and checks, moved; the generator is `./le yang glue`, extended; the order is the D-1 function of the release fix |
| Zero-copy where applicable | Yes (design) | Compiled nodes are static data referenced by pointer; a link copies only the paths to graft targets |
| Registration over hardcoding, outbound | Yes (design) | Each module's package registers its compiled module |
| Registration over hardcoding, inbound | Yes (design) | No list of module names: the generator discovers `.yang` files, the registry holds what init registered; `PluginOnlySchema` looks `ze-plugin-conf` up and errors when absent |

## Design: the compiled form

### Alternatives

| # | Form | Verdict |
|---|------|---------|
| A1 | Per-module goyang ASTs as Go literals, goyang resolve at init | Rejected, not buildable: `Statement` substatements are unexported with no constructor |
| A2 | One resolved schema per tag set | Rejected: `go generate` cannot know the tags a later `go build -tags` uses; 143 gates |
| A3 | Generated goyang `Entry` literals | Rejected: `EnumType` and `Statement` carry unexported fields |
| A4 | Gob or JSON blob per module, decoded at init | Rejected: a run-time decode where a Go literal costs nothing |
| A5 | **Chosen (owner, 2026-11-18, "Per module.").** Per-module compiled Ze-owned tree as a static Go literal, augments as graft records linked at registration | Chosen |

### Compiled module type (Ze-owned, in `config/yang`)

| Field | Type | Description |
|-------|------|-------------|
| Name, Namespace, Prefix | text | from the module header (replaces `ParseYANGMetadata`) |
| Imports | module names | direct imports, for `ze schema` display and the idle-module judge |
| Source | text | the module text: display (`ze schema show`), and the input of an external module's compile unit when this module is in its closure; never parsed at run time otherwise |
| Nodes | ordered node list | the module's own top-level data nodes, D-1 source order |
| RPCs, Notifications | ordered node lists | input and output as child nodes |
| Grafts | list of graft records | target path (module-qualified segments), D-1 key (augmenting module, augment ordinal in its module), the nodes to attach in statement order |

| Node field | Type | Description |
|------------|------|-------------|
| Name, Kind | text, typed enum (container, list, leaf, leaf-list, choice, case, rpc, input, output, notification) | zero kind is `Unspecified` |
| Module | module name | the instantiating module; an augmented node keeps its augmenter's (RFC 7950 Section 4.2.8: "If a module augments another module, the XML elements that are added to the encoding are in the namespace of the augmenting module.") |
| Location | file, line, column | where the statement is written, for every refusal text |
| Order | D-1 key | declared: the statement's source position (a `uses` at its position, a choice's cases in source order); grafted: the graft record's key, then statement order |
| Config, Mandatory | typed tri-state | resolved, inheritance applied |
| Presence | bool | replaces `Entry.Extra["presence"]` |
| Description, Units | text | |
| Default | text list | |
| Key, Unique | text, text lists | |
| MinElements, MaxElements, OrderedByUser | numbers, bool | |
| When, Must | text lists | carried as text, as today |
| Children | ordered node list | D-1 order: declared first, then grafted by key |
| Extensions | list of (module, keyword, argument, substatements) | prefix resolved to the declaring module |
| Type | compiled type | leaf and leaf-list only |

| Compiled type field | Description |
|---------------------|-------------|
| Name, Kind | type name and base kind |
| TypedefChain | typedef names from the leaf's type to the built-in |
| Ranges, Lengths | resolved bound lists, typed numbers |
| Patterns | indexes into the module's pattern table, compiled once at registration |
| FractionDigits, Enums (name, value, description, value order, Ze's assignment), Bits, Union members, Default | as today |
| Leafref | the path as steps, each step's name qualified with the module its prefix (or its absence) names, resolved at generation in the module where the path is written; the `../` count; the key predicates, qualified the same way; require-instance |

### Cross-module leafrefs (OD-5, under "No leaf ref must work cross module.")

RFC 7950 text, read in `rfc/full/rfc7950.txt`:

| Section | Sentence |
|---------|----------|
| 6.4 | "The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1)." |
| 6.4.1 | "The set of namespace declarations is the set of all "import" statements' prefix and namespace pairs in the module where the XPath expression is specified, and the "prefix" statement's prefix for the "namespace" statement's URI." |
| 6.4.1 | "Names without a namespace prefix belong to the same namespace as the identifier of the current node.  Inside a grouping, that namespace is affected by where the grouping is used (see Section 7.13).  Inside a typedef, that namespace is affected by where the typedef is referenced." |
| 9.9.2 | "If the "path" statement is defined within a typedef, the context node is the leaf or leaf-list node in the data tree that references the typedef." |
| 5.1 | "There MUST NOT be any circular chains of imports." |

So a prefixed name names a module the module where the path is written imports
(or that module itself), and an unprefixed name names the referring node's own
module, which for a leaf an augment adds is the augmenting module. Ze applies
exactly that: `checkPathPrefix` refuses a prefix the written module does not
declare and a node outside the module it names; `checkStepNamespace` refuses an
unprefixed name whose node belongs to another module; key predicates take the
same two checks (`checkPathKeys`, `keyExprTarget`). Every named step therefore
lies in the transitive import closure of the module that instantiates the leaf,
which every binary links with it. `ze:reference` is not a path: it names a
source registered from `init()`, judged at start by `unheldName`.

| Case | Where its names resolve | In the instantiating module's closure? |
|------|------------------------|-----------------------------------------|
| Relative path in the leaf's own module (17 today) | that module | Yes |
| Absolute prefixed path into an imported module (`ze-tacacs-conf` → `ze-authz-conf`) | the imported module | Yes |
| A node a third module N augments into the target | named with N's prefix, so the written module imports N | Yes |
| A leaf a module M augments into T, relative path | `../` climbs into T, which M imports to name the target; unprefixed names are M's; T's nodes need T's prefix | Yes |
| A leafref in a grouping of module G used by module U | unprefixed names are U's; prefixes are G's imports, and U imports G | Yes, transitively |
| A leafref in a typedef of module D referenced by a leaf of L | unprefixed names are L's; prefixes are D's imports, and L imports D | Yes, transitively |
| `require-instance` (default true) on a config leaf | the target's config property, set by its own statements and its ancestors, all in the closure; only a `deviation` from outside could change it, and the generator refuses `deviation` | Yes |
| The loop check (`checkLeafrefLoop`) | each target's own leafrefs resolve in the target's closure, a subset of the leaf's | Yes |
| A path naming a module the written module does not import | invalid YANG (Section 6.4); refused today with the import to add | Not valid, refused |

The reading holds for every case. The design that follows from it:

| Where | What is checked |
|-------|-----------------|
| Generation, per compile unit | Every leafref of the leaves module M instantiates, with every check of today, once. A cross-module leafref works because M's unit holds the modules its path names |
| Init link | Nothing: the verdict cannot change with the set |
| The join, for external modules | Every leafref of the leaves the external modules instantiate, inside `compiler.CompileExternal`'s unit (the external modules plus their closure). An external leafref into a compiled-in module, or into a node another compiled-in module augments in, works when the external module imports the modules its path names, as the RFC requires |
| Still set-dependent | Only the fix text of a refused path: `rootPathFix` and `writtenPrefix` search the loaded modules for one that holds the path, to name the import to add. The generator computes it over the whole tree; the join over the linked compiled-in value. The verdict never depends on it |

`A-1` of 2026-10-16 is therefore true for leafrefs, and the row that would have
refused a leafref leaving its closure is deleted: nothing valid is refused.

### Package layout (OD-9)

Read with `go list` on 2026-11-18: `config/yang` imports goyang, `command` and
core packages; `config` imports `config/yang`.

| Package, after C3 | Holds | Imports |
|-------------------|-------|---------|
| `internal/component/config/yang` | the compiled module type, `Link`, the linked schema (`Resolved`), the registry of compiled modules and the current linked value, the publish, `Join` (link plus every set refusal), every reader (validator, command tree, RPCs, enums, metadata, leafref path resolution and the leafref check, the held judge) | core packages and `command`, as today; no goyang |
| `internal/component/config/yang/compiler` | the loader, goyang's processing, the statement checks (extensions, patterns, structure, ABNF, grammar, enum assignment, include, `augmentErrors`, the typedef-default checks moved from `config`), `Emit`, `Compile`, `CompileExternal` (declared texts plus the closure's `Source` texts, answering compiled modules or the operator refusal naming the plugin and the module) | `config/yang`, goyang, core |
| `internal/component/config/yang/coreyang` | `ze-extensions.yang`, `ze-types.yang`, generated files | `config/yang` |
| each `yang/` package | the `.yang` and its generated `compiled.go`, `register.go` | `config/yang` and the `yang/` packages of its imports |
| `internal/component/config` | `JoinExternalModules`, `PublishExternalModules`: `compiler.CompileExternal`, then `yang.Join` / `yang.PublishExternalModules` | `config/yang`, `compiler` |
| `internal/le/yang/glue/gen` (generator `main`) | finds the root from `go.mod`, runs the glue | `internal/le/yang/glue`, `compiler` |

Each commit has no import cycle. C1 adds the compiled type, `Link` and `Emit`
inside `config/yang`, where the loader still lives, and the glue emitter
calls `Emit`. C2 ports the readers; the producer is `Emit` plus `Link` over the
registered text, still inside `config/yang`. C3 switches the producer to the
generated literals; from then no `config/yang` function calls the loader, and
the loader, the checks and `Emit` move to `compiler`, called by the generator,
`internal/le` tools, fixture tests and `config`. Bootstrap: the generator's
dependencies hold no `yang/` package and no YANG-glue output once the core
modules leave `config/yang` for `coreyang`.

| Alternative set aside | Why |
|-----------------------|-----|
| The compiled type in a new leaf package, the loader staying in `config/yang` | Builds, but the readers' package keeps goyang, so "no reader reads goyang" cannot be proved by the import graph (AC-23), and every reader imports two schema packages |
| The compiler as `config/yang`'s callee through a function variable a compiler `init()` fills | A seam that answers nothing when the compiler is not linked, the silent zero `ai/rules/principles.md` refuses |
| Moving the loader to `compiler` in C1, as written on 2026-10-16 | Import cycle: `resolveDefault`, `Resolved.reloaded` and `PublishExternalModules` in `config/yang` call it until C3 |

### Choices of the redesign (OD-6 to OD-8, and the checks built in P2)

| Item | Choice | Alternative set aside | Reason |
|------|--------|-----------------------|--------|
| OD-6: a `Link` that fails | `Link` answers an error value for each conflict (a graft target absent, a name the target already holds, a pattern that does not compile) and never panics. The registration of a compiled-in module turns it into a `BUG` panic naming the module; `Join` turns it into today's operator refusal naming the plugin and the module (AC-20's text) | The join checks the external module before `Link`, so `Link` keeps a `BUG` | A node added twice is visible only against the whole linked set, so a pre-check would duplicate `Link`'s conflict detection: two judges of one fact |
| OD-7: a non-mutating `Link` | Kept: `Link` copies the path to each graft target and never writes its input; one `Link` at init and at the join | Graft in place during init, a copying link for the join | Reload is not the reason (the set changes at most once, before any keeper). The reasons that hold: a refused join MUST leave the compiled-in value as it was; the equivalence test links one `Emit` output into several subsets; the generated literals are shared package data; two link functions would be two implementations of one rule. Cost bounded: 105 grafts, depth at most 8, measured by AC-6 |
| D-1 order in `Link` | A grafted child is inserted among the target's grafted children by its key (augmenting module name, augment ordinal in that module, statement order), after every declared child. Declared children come in source order from generation. The result does not depend on which module registered first | Append in registration order and sort at the lowering | Go init order follows imports, not module names; a sort at the lowering would leave the YANG tree, the completer and the command tree in another order than the printers |
| OD-8: which caches survive | `resolvedCache` (`yangSchemaCache`, `yangValidatorCache`) stays, keyed on the linked value, because it carries the publish contract and the lowering to `config.Schema` stays a run-time build (35,206 allocations, 3.3 MB per build). `rebuildCommandArgDefs` and the keepers stay. `DefaultLoader`'s generation key and `RegisterModuleForTest` go | Compile `config.Schema`, the command tree and the validator too | Widens C1 to C3 beyond the agreed scope; the lowering is about 7% of a build's allocations today. AC-6 reports its time on the Mac |
| Duplicate-augment refusal (`augmentErrors`, `91aa35862c`) | Moves to the compiler and runs on every compile unit and on the whole tree at generation; `Link` carries the same check for compiled grafts. A compiled-in conflict is impossible at init (both modules are in the whole tree, which generation linked); an external one is refused at the join with today's text | Rely on `Link` alone | A conflict inside one compile unit is found where goyang records it, with its location, before any emit |
| The judged mark (`markJudged`, `sourceModules`, `ErrRecheckJoinedSet`) | Stays in the compiler: the closure modules of an external compile unit come from compiled-in `Source` text, are marked judged, and skip the extension, pattern and structure checks; goyang's processing and the leafref check see the unit. `BenchmarkSchemaBuildStages` measures the compiler over every module and adds a link stage | Re-check the closure modules | They passed at generation; the P2 argument (each check reads one module and the modules its names reach) still holds |

### Generator wiring

| Item | Decision |
|------|----------|
| Generator | `./le yang glue update` (extended); the standalone `main` imports the glue and the compiler only |
| `go:generate` directive | One directive in `internal/component/config/yang/generate.go`, `go run github.com/ze-software/ze/internal/le/yang/glue/gen`; the `main` finds the root from `go.mod`, as `lepath.Root` does. It does not run `./le`, which links the generated output (Q3) |
| Core modules | `ze-extensions.yang` and `ze-types.yang` leave the `go:embed modules` of `loader.go` for `config/yang/coreyang` in C3, with generated files; every generated `register.go` blank-imports `coreyang` through the normal import derivation |
| `Registration.YANG` | Changes from text to a pointer to the compiled module in C3. Each `compiled.go` exports one variable per module named after it (`ZeDdosFlowspecConf` where the text variable was `ZeDdosFlowspecConfYANG`); the 73 sites name it. `YANGSchemas` and the schema CLI read name, namespace, imports and source from it; `ParseYANGMetadata` is deleted. `CollectPluginYANG` is deleted (OD-2); `GetInternalPluginYANG` and `GetAllInternalPluginYANG` stay as display lookups answering the compiled module |
| Output per module package | `compiled.go` (header "Code generated by ./le yang glue update; DO NOT EDIT.") and `register.go`; `embed.go` deleted |
| Freshness | `./le yang glue check` regenerates in memory and compares bytes; `TestYANGGlueCurrent` and the `./le repo update` check run it |
| Compiler at run time | Linked into `ze`; after C3 only `compiler.CompileExternal` is called, from the join. A call counter on the compiler's entry proves zero calls without an external plugin (AC-7) |

## Order against the two release fixes (OD-3, OD-4)

State on 2026-11-18: neither fix is built. `plan/release/fixes/spec-config-yang-one-definer-per-node.md`
is at `design` (last commit `7504b8c5d2`); `Schema.Define` still merges
(`config/schema.go` `mergeContainer`). `plan/release/fixes/spec-config-yang-definition-order.md`
is at `ready` (last commits `c340d79ea1`, `07be15f079`, the latter building the
sibling list-entry order, not D-1); `sortedKeys` and `yangChildOrder` still
order children. Neither is in progress or claimed (`./le spec wip`). Working
order: both land on `main` before C1 lands; the owner then lands this branch
with `git rebase`.

| | One definer per node (OD-3) | Definition order D-1 (OD-4) |
|---|------------------------------|------------------------------|
| What an operator or a schema reader sees before | Seven top-level containers are defined by several modules, and Ze merges them by name: `environment` 13, `service` 6, `l2tp` 5, `system` 4 (`ze-authz-conf`, `ze-radius-conf`, `ze-system-conf`, `ze-tacacs-conf`), `interface` 2 (`ze-cos-conf`, `ze-iface-conf`), `observe` 2 and `detect` 2 (the `ddos` and `anomaly` plugins). The module that sorts first wins: with `ze_cos` built, `?` on `interface` reads "Interface-level class-of-service profile bindings." and `interface ethernet` loses the interface module's description, required, suggest and unique | The children of `system` print in name order: `archive`, `authentication`, `authorization`, `commit-revisions`, `conntrack`, `console`, `crash-dump`, `dns`, `dns-source`, `domain`, `host`, `irr`, `name-server`, `peeringdb`, `rir`, `tuning`, `update-check` (in a build with the `irr` plugin) |
| After | Each root has one definer and the others augment it (`ze-authz-conf` writes `augment "/sys:system"`); `interface` reads "Network interface configuration." and keeps every list declaration, with the CoS leaves added under the lists. No config file changes | The order `ze-system-conf` wrote, then the augmenters by module name: `host`, `domain`, `name-server`, `dns-source` (its `uses` position), `dns`, `peeringdb`, `rir`, `tuning`, `crash-dump`, `console`, `conntrack`, `update-check`, `commit-revisions`, `archive`, then the augmenters by module name: `authentication` and `authorization` from `ze-authz-conf` (once the one-definer fix makes it augment `system`), then `irr` from `ze-irr-conf` |
| What this spec would build twice if it went first | `Link` and the equivalence test would carry a merge of same-named roots from several namespaces into one node, and the redefinition refusal would judge merged roots; the fix would then delete that merge from the compiled form, and the converted modules' `compiled.go` and the tacacs leafref path would be regenerated | The emitter and `Link` would carry today's order (name order, `yangChildOrder`'s list remainder), and the fix would then write its D-1 function against the compiled type instead of goyang entries and change the equivalence test's expected order |
| What the fix would redo if it went second | Its conversion and its `Define` refusal would land in the compiler and in `Link` rather than in `schema.go`, after C2 had ported every reader of the merged roots | Its order function would replace the emitter's and `Link`'s order rather than `sortedKeys` and `yangChildOrder`, and every expectation it refreshes would be refreshed again |
| Found while checking | The fix's table names five shared roots; `observe` and `detect` are also defined twice (`ze-ddos-observe-conf`, `ze-anomaly-observe-conf`; `ze-ddos-detect-conf`, `ze-anomaly-detect-conf`). Reported to the main session for that spec | - |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A module's compiled output depends on the linked set only through augments; a leafref is closure-local | RFC 7950 Sections 6.4, 6.4.1, 9.9.2, 5.1; `checkPathPrefix`, `checkStepNamespace`; census of 2026-11-18 (0 deviation, 0 if-feature, 0 identityref) | Some node differs by tag set; the graft model misses it | AC-5 over six subsets; AC-22 | unvalidated |
| A-2 | An augmenter's `yang/` package always imports the target's package | `yang-config-design.md` section 3; glue `imports.go` | A graft finds no target at init | AC-5; `TestGraftMissingTargetIsBUG` | unvalidated |
| A-3 | Readers need only the facts in the compiled type tables | Field census of 2026-10-16: `.Node` 40 reads, `.Exts` 50, `.Extra` 6, `.RPC` 8, `.ListAttr` 11; no reader of `Augmented`, `Uses`, `Deviations`, `Identities`, `Annotation` | A reader needs a statement-level fact not captured | C2 compiles; every existing test green | unvalidated |
| A-4 | The D-1 order function of the definition-order fix can be applied at generation to one module's declared children, and its augment clause reduces to a key per graft | That spec's Task table: augmented children after the block's own, by module name, then augment order, then statement order | `Link` cannot reproduce D-1 for some child | `TestLinkOrderIsIndependentOfRegistrationOrder`; AC-5 compares child order with the D-1 function | unvalidated |
| A-5 | A static Go literal of about 2,100 nodes and 2,500 extensions builds fast and stays small | Go compiles large static tables routinely | Build time or binary size regress | C1 measures `go build` time and binary size | unvalidated |
| A-6 | Compiling at most 134 patterns at init costs a few milliseconds | Typical Go regexp compile is tens of microseconds | Every `ze` CLI start pays more | AC-6 | unvalidated |
| A-7 | No run-time reader needs a compiled-in module's text except `ze schema show` and an external module's compile unit | grep of `.Content` and `Modules()` readers | Another surface loses its text | grep in C3; `Source` keeps the text | unvalidated |
| A-8 | An external module's import closure is small next to the whole set | A `-conf` module imports a handful of modules | The join still costs most of a build | AC-18 profile | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Bootstrap: a stale generated file breaks the build of the generator | `go generate` fails to compile its `main` | `TestGeneratorImportsNoGeneratedPackage` walks `go list -deps` of the generator `main` and fails on a `yang/` package, on `coreyang`, and on any file carrying the YANG-glue header. It does not refuse other generators' output, such as `plugin/registry/gated_plugins_gen.go`, which `config/yang` reaches through `command` today |
| R-2 | The linked tree differs from goyang's whole-set resolve on a subtle field | AC-5 red | Fix the emission; never relax the comparison |
| R-3 | Validator semantics drift when range, length and union checks move to Ze types | Validator and `rfc7950_validator_*_test.go` red | Port the tests first, unchanged |
| R-4 | `plan/release/fixes/spec-config-yang-loader-structural-checks.md` (design) edits the same loader files | Conflict, or a check added after the move | Q6 |
| R-5 | C2 and C3 each sit on the worktree branch about two weeks | `main` moves under the branch | Q7: the branch `worktree-yang-compiled`, landed by the owner with `git rebase`; each work package writes the per-spec state file |
| R-6 | Test migration is larger than the reader migration: 200 test files | Test compile errors in C2 and C3 | C2 ports the 43 goyang test files; C3 moves the 200 to the linked value or to `compiler.Compile` |
| R-7 | A reader holds a linked value while `Link` builds the next one (init, and the one publish) | `-race` reports a write to a literal | `Link` never writes its input: `TestLinkNeverWritesItsInput`, `TestLinkedSchemaConcurrentReaders` |
| R-8 | The generated literals grow the binary | Size measured in C1 | Growth above 2 MB is reported before continuing |
| R-9 | The release fixes land on `main` after C1's work packages started | C1's emitter written against today's order or merged roots | C1 lands only after both fixes are on the branch (Implementation Steps) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every schema reader: config parsing, editor validation, completion, the command tree, API metadata. A wrong node refuses valid config or accepts invalid config |
| How is it reverted? | Per-commit revert; no config or on-disk format changes |
| Who else touches this path? | The two release fixes (they land first), `spec-config-yang-loader-structural-checks` (design), `plan/post-release/features/spec-plugin-external-yang-judges-decorate-and-backend.md` (the join), the `spec-config-yang-*` skeletons, any session adding a `.yang` module |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go generate ./internal/component/config/yang/` | → | glue generator writes `compiled.go` per module | `TestGenerateWritesCompiledModuleBesideEachYANG` (`internal/le/yang/glue/yangglue_test.go`) |
| Package init of `plugin/all` | → | registration and link into the linked value | `TestLinkedSchemaHoldsEveryLinkedModule` (`internal/component/plugin/all/schema_test.go`) |
| `ze config validate` on a config using an augmented leaf | → | `config.Schema` built from the linked value | `test/parse/yang-compiled-augmented-leaf.ci` |
| Editor `set` of an out-of-range value | → | `YANGValidator()` over the linked value | `test/editor/commands/yang-compiled-range-refused.et` |
| `ze start` with an external plugin whose module carries a leafref into a compiled-in module | → | `compiler.CompileExternal`, `yang.Join` | `test/plugin/external-yang-leafref-cross-module.ci` |
| Broken module in a fixture tree | → | generator refuses | `TestGenerateRefusesBrokenModule` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go generate ./internal/component/config/yang/` on a clean tree | Writes `compiled.go` and `register.go` beside every `yang/` module and in `internal/exabgp/migration/yang/`; a second run changes no byte; output equals `./le yang glue update` |
| AC-2 | A fixture module with an undeclared `ze:comand` extension | Generation exits non-zero and prints the `ErrUndeclaredExtension` text naming module, file and line; writes no file |
| AC-3 | Fixture modules each carrying one of: uncompilable pattern, descending `length`, duplicate enum value, restricted enum the base lacks, bad extension substatement, unresolved import, invalid typedef default, default naming an `if-feature` enum, absent augment target, a node two augments add, a leafref prefix the module does not import, `deviation` | Each refused at generation with today's sentinel and location; `deviation`, `if-feature` and `identityref` with the refusal naming the spec that will build them |
| AC-4 | A `.yang` edited, generation not rerun | `./le yang glue check` exits non-zero and names each stale file |
| AC-5 | In memory: `Emit` over every module, then `Link` per subset, each compared with a goyang whole-set resolve of the same subset's text, child order compared with the D-1 order function. Subsets: (1) every module; (2) the default daemon tags (`featuretags` plus the `go list -deps` of `plugin/all` that `internal/le/plugin/imports` derives); (3) the `ze_core` tags; (4) the closure of `ze-bgp-conf` plus `ze-graceful-restart`; (5) the closure of `ze-iface-conf` with no augmenter; (6) the default set plus a fixture external module that augments a compiled-in container and carries a leafref into it, linked by `Join` | Equal node by node: name, kind, module, config, mandatory, presence, default, key, unique, list bounds, when, must, extensions, type facts including the qualified leafref path, child order |
| AC-6 | Benchmarks on the owner's Mac, no `-race` | Registration and link of every module at most 5 ms; `YANGValidator()` at most 1 ms per call; `config.YANGSchema()` build time reported |
| AC-7 | `ze` starts with a config naming no external plugin | The compiler's call counter reads zero after start (`TestStartWithoutExternalPluginsCompilesNothing`) |
| AC-8 | `ze schema show ze-bgp-conf` | Prints the module source unchanged (`test/parse/cli-schema-show.ci`) |
| AC-9 | Config setting graceful-restart under `bgp peer session capability` | Parses and validates through the grafted node (`test/parse/yang-compiled-augmented-leaf.ci`) |
| AC-10 | Editor test package under `-race` | Completes in the default timeout; an out-of-range value is refused with today's message |
| AC-11 | `ze exabgp migrate` | Output unchanged (`test/exabgp`); schema compiled; no nil-on-error path |
| AC-12 | grep of the run-time tree after C3 | No `DefaultLoader`'s text producer (`resolveDefault`, `LoadRegistered`, text `RegisterModule`, `RegisterModuleForTest`), no `ParseYANGMetadata`, no `CollectPluginYANG`, no module-text argument of `YANGSchemaWithPlugins` or `YANGValidatorWithPlugins`, no `loadYANGModules`, no `Resolved.HoldsModuleText`, no `embed.go` in a `yang/` package. `AddModuleFromText` exists only in `compiler` |
| AC-13 | Every completion, `ze schema`, inline-list and command-tree test | Green with the D-1 expectations the definition-order fix set |
| AC-14 | The 200 test files that call `DefaultLoader`/`NewLoader` or load fixture text | Read the init-linked value for real modules, or compile fixture text with `compiler.Compile`; no test mutates the global registry; grep finds no `RegisterModuleForTest` |
| AC-15 | `checkStructure` over every module | One walk per module; substatement fields computed once per type; every refusal unchanged. Built `fa47934b25` |
| AC-16 | A join of external modules | Ze's statement checks run over the external modules only; a refused join names the failing module without a whole-set resolve per module where an error belongs to a declared module. Built `91aa35862c`, with the scope its correction row states; absorbed by AC-18 |
| AC-17 | `loadLCPDefaults` | Reads the set `DefaultLoader` resolved; no loader of its own. Built `c1b1ad36c6` |
| AC-18 | `ze start` with one external plugin declared, against the same binary with none | The added cost is the plugin's own launch and the compile of its modules against their import closure: the profile shows no parse or resolve of a compiled-in module outside that closure under `joinDeclaredPlugins`. Shown on the owner's Mac, both configs interleaved; the figure is written into `docs/plugin-development/schema.md`. Closes R-19 and Q10 of the external-plugin spec |
| AC-19 | `BenchmarkSchemaBuildStages` | Time and allocations per stage; allocation figures valid on any host, time only from the Mac. Built `d0d2fd1d88` (time owed from the Mac) |
| AC-20 | A module whose `augment` adds a node its target already holds: between two external modules, and into a compiled-in container | Refused, compiled-in or external, naming the augmenting module. Built `91aa35862c` (`TestJoinNamesTheModuleThatAugmentsADuplicateNode`); after C3 the same test passes through `Link`'s conflict |
| AC-21 | The same compiled modules linked in two registration orders | Identical linked values, children in D-1 order (`TestLinkOrderIsIndependentOfRegistrationOrder`) |
| AC-22 | Cross-module leafrefs: `ze-tacacs-conf`'s absolute path at generation; a fixture leaf augmented into another module's container with a relative path to a sibling of the target; a fixture leafref in a typedef of an imported module; a fixture leafref in a grouping used by another module; an external module whose leafref names a compiled-in leaf and one that names a node a third module augments in | Each is accepted and its value validates against the target; a fixture path whose prefix names a module the written module does not import is refused with today's text and fix |
| AC-23 | `go list -deps ./internal/component/config/yang` after C3 | No goyang package (`TestRuntimeSchemaPackageImportsNoGoyang`) |
| AC-24 | `ze start --plugin ze.<name>` naming a compiled-in plugin, after C3 | The plugin starts and config under its root parses, its module coming from its compiled registration. The deleted `TestReloadVerifierRetainsSelectedPluginSchema` case (a registration whose module no `init()` registered) is not constructible once `Registration.YANG` is a compiled module |

Correction kept from 2026-11-17: AC-16 built scope. "Without one more whole-set
resolve per declared module" holds when an error belongs to a declared module
(a goyang error located in its file, a leafref error on a leaf it wrote, a Ze
check error found over that module); a goyang error with no location or one in
a compiled-in file still resolves each declared module alone.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGenerateWritesCompiledModuleBesideEachYANG`, `TestGenerateIsByteStable` | `internal/le/yang/glue/yangglue_test.go` | AC-1 | |
| `TestGenerateRefusesBrokenModule` (one row per AC-3 fault) | same | AC-2, AC-3 | |
| `TestGlueCheckNamesStaleCompiledFile` | same | AC-4 | |
| `TestGeneratorImportsNoGeneratedPackage` | `internal/le/yang/glue/gen_imports_test.go` | R-1 | |
| `TestLinkedEqualsWholeSetResolve` (six subsets) | `internal/component/config/yang/link_test.go` (moves to `compiler` in C3) | AC-5 | |
| `TestLinkOrderIsIndependentOfRegistrationOrder` | `internal/component/config/yang/link_test.go` | AC-21 | |
| `TestLinkReportsConflictsAsValues`, `TestLinkNeverWritesItsInput` | same | OD-6, R-7 | |
| `TestGraftMissingTargetIsBUG`, `TestLinkedSchemaConcurrentReaders` (`-race`) | `internal/component/config/yang/registry_test.go` | A-2, R-7 | |
| `TestCrossModuleLeafrefs` (one row per AC-22 case) | `internal/component/config/yang/compiler/leafref_test.go` | AC-22 | |
| `TestRuntimeSchemaPackageImportsNoGoyang` | `internal/component/config/yang/imports_test.go` | AC-23 | |
| `TestLinkedSchemaHoldsEveryLinkedModule`, `TestStartWithoutExternalPluginsCompilesNothing` | `internal/component/plugin/all/schema_test.go` | wiring, AC-7 | |
| `BenchmarkRegisterAll`, `BenchmarkYANGValidator` | `internal/component/plugin/all/schema_bench_test.go` | AC-6 | |
| `TestCommandLinePluginRootParses` | `internal/component/bgp/config/loader_test.go` | AC-24 | |
| Ported: `rfc7950_validator_*_test.go`, `rfc7950_reference*_test.go`, `validator_test.go`, `enum_test.go`, `command_test.go`, `loader_*_test.go`, `join*_test.go` | their packages | R-3 | |
| `BenchmarkSchemaBuildStages` and `loader_stages_test.go` | `internal/component/plugin/all`, `config/yang` | AC-19 | built `d0d2fd1d88` |
| `loader_structure_walk_test.go` | `config/yang` | AC-15 | built `fa47934b25` |
| `TestJoinDoesNotCheckCompiledInModulesAgain`, `TestJoinNamesTheModuleThatAugmentsADuplicateNode` | `config/yang/join_checks_test.go` | AC-16, AC-20 | built `91aa35862c` |
| `TestLCPDefaultsReadTheProcessModuleSet`, `TestLCPDefaultsAreTheSharedGroupingDefaults` | `internal/component/l2tp/pppconfig/lcp_shared_set_test.go` | AC-17 | built `c1b1ad36c6` |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| enum value (RFC 7950 Section 9.6.4.2) | -2147483648..2147483647 | 2147483647 | -2147483649 | 2147483648 |
| range bound, `uint64` leaf | 0..18446744073709551615 | 18446744073709551615 | -1 | 18446744073709551616 |
| graft path depth | 1..8 today | 8 | 0 (empty path refused) | N/A (bounded by the tree) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `yang-compiled-augmented-leaf` | `test/parse/yang-compiled-augmented-leaf.ci` | Operator configures a leaf another module augments | |
| `yang-compiled-range-refused` | `test/editor/commands/yang-compiled-range-refused.et` | Operator types an out-of-range value in the editor | |
| `external-yang-leafref-cross-module` | `test/plugin/external-yang-leafref-cross-module.ci` | An external plugin's leaf refers to a compiled-in list; a valid value loads, an absent one is refused | |
| `cli-schema-show` (existing) | `test/parse/cli-schema-show.ci` | Operator prints a module's YANG | |
| existing `test/exabgp` suite | `test/exabgp/` | ExaBGP migration unchanged | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/config/yang/register.go`, `loader.go`, `join.go`, `join_command.go`, `publish.go`, `registration_held.go`, `loader_reference.go`, `reference_path.go`, `reference_source.go`, `validator.go`, `validator_mandatory.go`, `validator_registry.go`, `command.go`, `command_argdefs.go`, `enum.go`, `rpc.go`, `rpc_publish.go`, `metadata.go` - compiled form, `Link`, registry; the loader and checks move to `compiler` in C3
- `internal/component/config/yang_schema.go`, `schema.go`, `validate_sections.go`, `flatten.go`, `loader.go`, `declared_schemas.go`, `claims/schema.go` - read the linked value; `loadYANGModules` and the module-text arguments deleted; the schema-build checks move to the compiler
- `internal/component/config/yang/cli/tree.go`, `config/cli/cmd_edit.go`, `config/schema/cli/main.go`, `config/infra/authz.go`
- `internal/component/cli/completer.go`, `completer_validate.go`, `validator.go`, `cli/client/main.go`, `cli/testing/headless.go`
- `internal/component/aihelp/aihelp.go`, `internal/component/iface/validate.go`, `internal/component/plugin/server/server.go`, `internal/component/plugin/inprocess.go` (`CollectPluginYANG` deleted), `internal/component/plugin/cli/test_cmd.go`, `internal/component/bgp/config/loader.go`, `internal/component/bgp/config/register.go`
- `cmd/ze/hub/session_factory.go`, `command_meta.go`, `service_web.go`, `declared_plugins.go` (only if the call changes)
- `internal/exabgp/migration/schema.go`
- `internal/component/plugin/registry/registry.go` and the 73 `Registration.YANG` sites
- `internal/le/repo/inventory/inventory.go`, `internal/le/arch/enumeration/corpus.go`, `internal/le/doc/yangcontract/*.go`, `internal/le/config/claims/configclaims.go`, `internal/le/cli/dispatch/resolver.go`, `internal/le/cli/list/commandlist.go`, `internal/le/cli/grammar/cligrammar.go` - linked value or compiler, per reader
- `internal/le/yang/glue/*.go` - emit `compiled.go`, stop emitting `embed.go`
- every `yang/` package: `embed.go` deleted, `register.go` regenerated
- `docs/architecture/config/yang-config-design.md` (section 3, the join and the decided design), `docs/architecture/command-ownership.md`, `docs/plugin-development/schema.md` (AC-18 figure), `ai/INDEX.md` `./le yang glue` row, `ai/patterns/registration.md`

## Files to Create
- `internal/component/config/yang/compiler/` (C3) - the moved loader, checks and `Emit`, `CompileExternal`
- `internal/component/config/yang/coreyang/` (C3) - the two core modules and their generated files
- `internal/component/config/yang/generate.go`, `internal/le/yang/glue/gen/main.go`
- `internal/exabgp/migration/yang/` - `exabgp.yang` moved here
- `test/parse/yang-compiled-augmented-leaf.ci`, `test/editor/commands/yang-compiled-range-refused.et`, `test/plugin/external-yang-leafref-cross-module.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No module content changes |
| YANG validation constraints | No | Constraints unchanged; their checks move to generation |
| YANG custom validators | No | `ze:validate` registration unchanged; the held judge reads compiled data |
| CLI commands/flags | No | `--plugin` keeps its behaviour; `ze schema show` output preserved |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | `internal/component/cli/completer.go` reads compiled nodes |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No new output |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | N-A | Removes a run-time dependency; adds none |
| Prometheus counters/metrics | No | None |
| BGP family surface | N-A | Not a family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Start cost improves |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/schema.md`: the start cost of an external plugin (AC-18), and that its leafref may name an imported module's node |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | The Stage 1 schema list is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 7950 tagged tests move to the compiler: `rfc/short/rfc7950.md` anchors updated in the commit that moves them |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if fixture YANG changes shape; the `docs/contributing/` page naming generated files |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` section 3 (the decided design is recorded there, 2026-11-18; each commit corrects what it makes wrong), `docs/architecture/command-ownership.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation: `./le spec citation anchors spec plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md`. Declared by `// Design:` headers: `syntax.md`, `core-design.md`, `hub-architecture.md`, `api/architecture.md`, `features/ai-first.md`, each checked in the commit that changes its source |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `yang-config-design.md` "Registration Pattern" shows `embed.go`; updated in C3 |

## Implementation Steps

`ai/rules/no-layering.md`: "When replacing X with Y, X MUST be deleted first and
Y MUST be implemented after. A hybrid, a gradual migration, a fallback to the
old path and any other shape that keeps both MUST NOT be written."

### Steps P0 to P3 (built on branch `worktree-yang-compiled`, 2026-11-18)

| Step | Commit | Criterion | Still owed |
|------|--------|-----------|------------|
| P0 staged benchmark | `d0d2fd1d88` | AC-19 | The time per stage, from the owner's Mac |
| P1 one walk per module in `checkStructure` | `fa47934b25` | AC-15 | Nothing |
| P2 the join checks the external modules only; duplicate augmented nodes refused | `91aa35862c` | AC-16, AC-20 | Nothing short of AC-18 |
| P3 `loadLCPDefaults` reads the shared set | `c1b1ad36c6` | AC-17 | Nothing |

Scoped evidence the main thread ran: `./le test unit pkg` green on `config/yang`,
`config`, `l2tp/pppconfig`, `l2tp/pppoe`, `l2tp/ppp/yang`; changed-code lint
clean except a finding at `command_test.go` that predates the series. Reds seen
and not caused by it: `plugin/all` `TestGeneratedPluginImportsCurrent` and
`TestFeatureGatedModulesEnumerated` (base `8fb9a9295e`: `all_ze_exabgp.go`
lacks `internal/plugins/exabgp/yang`); 16 `cmd/ze/hub` tests (three groups are
the external-plugin spec's); `l2tp` `TestKernelPPPAdmission*`, unattributed.

### Commits C1 to C3

| Replaced | X | Y | Commit |
|----------|---|---|--------|
| The form readers consume | goyang `Entry` and `Statement` | the Ze compiled node type | C2 |
| The producer of the schema | run-time parse and resolve of registered text | generated literals linked at init | C3 |

In each commit every reader reads one form and one producer is live. No commit
registers a module twice, ships `embed.go` beside `compiled.go`, or keeps a
fallback. Each commit is built from several work packages on the branch.

1. **C1: compiled type, `Emit`, `Link`, the generator chain; no run-time change.** All in `config/yang`, beside the loader, plus the glue emitter, the directive and the generator `main`.
   - Add the compiled type with `Location`, `Order` and the qualified leafref path; `Emit` (a resolved unit to compiled modules, declared children in D-1 order through the release fix's order function); `Link` (non-mutating, conflicts as values, grafts by D-1 key).
   - The emitter writes only into its tests' temporary fixture trees; C1 puts no generated file in the tree and adds no registry.
   - Tests: `TestGenerateWritesCompiledModuleBesideEachYANG`, `TestGenerateIsByteStable`, `TestGenerateRefusesBrokenModule`, `TestGlueCheckNamesStaleCompiledFile`, `TestGeneratorImportsNoGeneratedPackage`, `TestLinkedEqualsWholeSetResolve` (subset 6 against today's `yang.Join`), `TestLinkOrderIsIndependentOfRegistrationOrder`, `TestLinkReportsConflictsAsValues`, `TestLinkNeverWritesItsInput`. The equivalence test is red before emission is complete and green after.
   - Measure `go build` time and binary size against an emitted copy of the tree (A-5, R-8).
   - Depends on: neither OD-2 (C3's) nor the one-definer fix for buildability (two namespaced roots are representable), but the emitted tree and the equivalence expectations follow whatever the tree holds, so C1 lands after both fixes are on the branch (working order). Its D-1 order and the child-order assertion of AC-5 need the definition-order fix's order function. What can start now: the compiled type, `Link`'s graft, copy and conflict mechanics, the generator chain and refusal tests, the bootstrap test.
2. **C2: every reader moves to the compiled type (one commit).**
   - `Resolved` holds only the compiled linked set, built by `Emit` plus `Link` over the registered text inside `config/yang`: the same producer, the same run-time cost.
   - Port every run-time reader and the `internal/le` readers: validator (ranges, lengths, unions), command tree, argument definitions, enums, RPC extraction, metadata, config schema build and its order, completer, CLI client, aihelp, authz, iface, plugin server, schema CLI, hub, exabgp, the join's set refusals, the publish, the held judge, the leafref path resolver and check (`checkReferences`, now over the linked unit). Port the 43 test files importing goyang.
   - Afterwards goyang is referenced only by the loader, its checks and `Emit`.
   - Tests: the ported validator, enum, command, reference and join tests; AC-9, AC-11, AC-13 suites.
3. **C3: the producer switch (one commit).**
   - Generate `compiled.go` and `register.go` for every module; move the core modules to `coreyang`; add the registry linking at init.
   - Move the loader, the checks and `Emit` to `config/yang/compiler`; add `CompileExternal`; `Join` takes compiled modules; `config.JoinExternalModules` and `config.PublishExternalModules` compile then join.
   - Delete in the same commit: the run-time compile of compiled-in modules; text `RegisterModule`, `Module.Content`, `RegisterModuleForTest`, `Modules()`; every `embed.go`; `ParseYANGMetadata`; the generation key; the `--plugin` text hand-over (`CollectPluginYANG`, `loadYANGModules`, `HoldsModuleText`, the module-text arguments), with the design page section "Joining a module that no `init()` registered" corrected and `TestReloadVerifierRetainsSelectedPluginSchema` removed (owner go-ahead 2026-11-18; the commit body cites it).
   - Change the 73 `Registration.YANG` sites to the compiled module.
   - Migrate the 200 test files (AC-14).
   - Measure the other schema caches (`gnmi` `schemaOnce`, `traffic` and `firewall` `backendGateSchemaOnce`, `command.go` `validateOnce`) and delete each one with no measured need.
   - Tests: `TestLinkedSchemaHoldsEveryLinkedModule`, `TestGraftMissingTargetIsBUG`, `TestLinkedSchemaConcurrentReaders`, `TestCrossModuleLeafrefs`, `TestRuntimeSchemaPackageImportsNoGoyang`, `TestCommandLinePluginRootParses`, benchmarks, AC-7, AC-10, AC-12, AC-14, AC-18, AC-22, AC-24, `external-yang-leafref-cross-module.ci`.
4. **Docs:** each commit edits the pages its code makes wrong; the last work package of C3 runs the anchor sweep.

### Effort

Re-estimated 2026-11-18 from the re-measured reach.

| Commit | Reach | Estimate |
|--------|-------|----------|
| C1 | compiled type, `Emit`, `Link` with D-1 keys and conflicts, glue emitter, generator `main`, the equivalence test over six subsets | 5 to 6 days |
| C2 | 39 run-time and 8 `le` files importing goyang, 315 `*yang.Entry` references, 43 goyang test files; validator range, length and union port; the leafref resolver; the join's set refusals and the publish | 15 to 18 days |
| C3 | 175 generated packages, 73 registration sites, 200 test files, the compiler move and `CompileExternal`, the text hand-over deletion, the caches | 11 to 14 days |
| Total | | 31 to 38 agent-days (18 to 23 on 2026-10-16) |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | The equivalence test covers six subsets; no run-time reader reads goyang (AC-23) |
| Naming | One generated variable per module, named after the module; no `<Module>YANG` text variable remains |
| Data flow | Only generated `register.go` and the publish write the current value; readers never mutate |
| Rule: no-layering | No reader consults both the text loader and the compiled schema |
| Rule: principles (silent zero) | No reader answers empty when a module is absent; absence is an error or a `BUG` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No compile of a compiled-in module at start | `TestStartWithoutExternalPluginsCompilesNothing` output |
| Generated files fresh | `./le yang glue check` |
| Validator cost | `BenchmarkYANGValidator` output |
| Broken module fails the build | `TestGenerateRefusesBrokenModule` output |
| Cross-module leafrefs work | `TestCrossModuleLeafrefs`, `external-yang-leafref-cross-module.ci` output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | An external plugin's module text is operator input: `CompileExternal` and `Join` refuse it naming the plugin and the module, never panic |
| Fail closed | A link conflict at init panics with `BUG`, never skips the node |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the commit that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Open Owner Questions

| # | Question | State on 2026-11-18 | Recommendation |
|---|----------|---------------------|----------------|
| Q1, OD-2 | Delete the `--plugin` text hand-over? | **Answered:** "Okay, do what you need." (owner, 2026-11-18): deleted in C3 with the design page and the test (Behavior to change) | - |
| Q3 | `go:generate` runs a standalone generator `main` with `go run`, not `./le`, because `le` links the generated output. Acceptable? | Open; its premise holds: `le` links `plugin/all` (`ai/doors/le-tooling.md`) | Yes |
| Q4 | Keep each module's source text in the binary? | Open; the design now depends on yes: `ze schema show` and the compile unit of an external module read `Source` | Yes |
| Q5 | Ze-owned compiled type against a further goyang vendor patch exporting constructors | Open | Ze-owned type |
| Q6 | Order against `plan/release/fixes/spec-config-yang-loader-structural-checks.md` (design) | Open; P1 here already edited `loader_structure.go` | Land it first into the loader, which C3 moves to the compiler unchanged |
| Q7 | Carrier for C2 and C3 | **Answered:** the worktree branch `worktree-yang-compiled` (owner, 2026-11-17, "We can start that optimization in a different tree") | - |
| Q8 | Load order of the plugin block, external plugins, their YANG and the full config | **Answered** by built code and the owner's "It is in the design" (2026-11-15): first pass, run the plugins, join and publish, full parse | - |
| Q9 | Relink on reload | **Answered:** never; modules change only when `ze` restarts (owner, 2026-11-15) | - |
| Q10 | Start-time budget of an external module | **Answered by measurement:** about 300 ms on the server today, indicative; AC-18 owns the Mac figure | - |
| Q11, OD-1 | Per-module compiled literals or way 3 | **Answered:** "Per module." (owner, 2026-11-18) | - |
| Q2 | Bucket | Answered by placement: `plan/post-release/engineering/` (`4831b585d9`) | - |

### Design decisions (2026-11-18)

| # | Subject | State |
|---|---------|-------|
| OD-1 | Per-module compilation (A5) | Owner: "Per module." |
| OD-2 | The `--plugin` text hand-over | Owner go-ahead to delete it in C3 |
| OD-3 | Order against the one-definer fix | Working order: the fix lands first (main session's sequencing under the owner's "ok" of 2026-11-17; he did not object). The compiled form carries no `Schema.Define` merge |
| OD-4 | Child order | Owner: "Yes, I keep the order of the definition." D-1 stands; the definition-order fix lands first; `Link` orders grafts by D-1 key |
| OD-5 | Cross-module leafrefs | Owner: "No leaf ref must work cross module." The RFC makes every path closure-local; checked once per compile unit (section "Cross-module leafrefs") |
| OD-6 | `Link` failing | Decided: conflicts as values, `BUG` at init, operator refusal at the join |
| OD-7 | Non-mutating `Link` | Decided: kept, for the reasons in "Choices of the redesign" |
| OD-8 | Caches | Decided: `resolvedCache` and the publish contract stay; the lowering is not compiled |
| OD-9 | Package layout | Decided: "Package layout" |
| OD-10 | The transport-only rule (owner, 2026-11-17) | No conflict: one compiler and the same judges for both; only the time differs, by transport. Rechecked 2026-11-18 |

## Design Insights

- From the P0 baseline: text load and goyang's processing are about 96% of a build's allocations, so only a form that skips both at start changes a start much.
- A prepared compiled-in set does not by itself make a start with an external plugin cheaper: the external module's compile unit needs its closure. The saving is that the unit holds the closure, not the whole set (A-8, AC-18).
- Every Go build input generated today is committed and has a freshness test.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-module compiled tree plus graft records (owner, 2026-11-18) | A1 to A4; way 3 | Only augments vary with the linked set; grafts isolate that; static data costs nothing at init |
| Leafrefs checked once per compile unit | Refuse a leafref leaving the closure; re-check at link | The RFC binds every name of a path to the closure, so neither a refusal nor a re-check is needed |
| Non-mutating `Link` with D-1 keys | Graft in place at init; registration-order grafts | A refused join leaves the compiled-in value intact; order independent of init order |
| The loader moves to `compiler` in C3, not C1 | Move in C1; a leaf type package; a seam | No cycle in any commit; the import graph proves no reader reads goyang |
| The compiler stays linked into `ze` | A generator-only compiler | External plugin YANG arrives at start (owner goal 2026-10-16); AC-7 proves no compile without one |
| Generator independent of YANG-glue output | Run `./le` from the directive | Bootstrap |

## Known Limitations

- With no daemon, Ze starts no external plugin and holds no schema of one (owner ruling, 2026-11-16); an offline command parses against the compiled-in value.
- The plugins and their modules change only when `ze` restarts (owner ruling, 2026-11-15).
- Owner ruling (2026-10-16): "if a user use an external plugin the security is on him". No trust sandbox for external YANG; the join refuses invalid YANG and every case of its refusal table.
- The join does not judge `ze:decorate` and `ze:backend` names: `plan/post-release/features/spec-plugin-external-yang-judges-decorate-and-backend.md`, which changes the same function.

## Work Not Done (homed elsewhere)

| Item | Home |
|------|------|
| Joining external plugin YANG at startup, the offline rule, the restart rule | Built by `plan/post-release/features/spec-plugin-external-yang-at-startup.md` |
| One definer per config node, and the CoS-over-interface defect | `plan/release/fixes/spec-config-yang-one-definer-per-node.md` (lands first) |
| Printing children in D-1 order | `plan/release/fixes/spec-config-yang-definition-order.md` (lands first) |
| The structural checks owed by the loader | `plan/release/fixes/spec-config-yang-loader-structural-checks.md` (Q6) |
| Judging `ze:decorate` and `ze:backend` names at the join | `plan/post-release/features/spec-plugin-external-yang-judges-decorate-and-backend.md` |
| `deviation`, `if-feature`, `identityref` | `plan/post-release/features/spec-config-yang-deviation.md`, `spec-config-yang-if-feature.md`, `spec-config-yang-type-identityref.md` (the generator refuses them until those land) |
| Validating leafref values | `plan/post-release/features/spec-config-yang-type-leafref.md` |

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
- [ ] AC-1..AC-24 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The scoped test runs of every package and functional suite the change reaches pass, run by the main thread from the owed commands (`ai/rules/commands.md`); the whole-tree check is GitHub CI's `verify` workflow
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` and `.et` tests for end-to-end behavior
- [ ] Interop tests: N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md` only, in the same `./le commit create` script

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-11-18 |
| Checked at | `8fb9a9295e` (`main`'s tip, the base of branch `worktree-yang-compiled`) |
| Verdict | REWORK |
| Reworked | 2026-11-18: the owner answered OD-1, OD-2, OD-4 and OD-5; OD-3 takes the working order; OD-6 to OD-9 are decided in "Choices of the redesign" and "Package layout"; every count re-measured |
