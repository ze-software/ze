# Spec: config-yang-compiled-at-build-time

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P3 |
| Release reason | Release-independent config yang compiled at build time work; historical findings do not establish a current product defect. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (Thomas, 2026-10-16): "compile [the YANG] at build time, use
go:generate script and import".

Owner statements (Thomas, 2026-11-17), in order, made while he looked at the
start-time comparison of the external-plugin work and before this spec was
found again:

1. On that comparison: "profile."
2. "Obviously, the plugin we are compiling in the code could be pre-parsed. Is
   that a way which makes things faster?"
3. "What would be the compilation time difference if we had that feature to be
   generated at each compilation?" He was given three ways and answered "3".
   Way 3 is described under Open Owner Questions, Q11.
4. "Complete in that case, write a spec for that optimization and complete the
   work we have done there. We can start that optimization in a different tree
   closed phase nine."

His "3" is recorded as what he said, not as a design: it departs from his
decision of 2026-10-16 and from alternatives this spec rejects, and he gave it
before that was put to him. Q11 puts the two side by side. Until he answers,
the design below (A5) stays the recorded one and this spec stays at `design`.

This spec also owns the cost of the second schema build that a declared
external plugin adds to a start. The external-plugin spec
(`plan/post-release/features/spec-plugin-external-yang-at-startup.md`, R-19 and
Q10) hands it over. What closes it is AC-18.

Today every process that needs the schema parses the YANG text of every linked
module and resolves it with goyang at runtime. A `go:generate` step MUST instead
produce Go that the packages import, so no YANG text is parsed or resolved at
runtime. Every check `Loader.Resolve` runs today (extensions, patterns,
`checkStructure`, enum assignment, imports) and every check the config schema
build records (`recordSchemaBuildError`) MUST run at generation time, so a bad
module fails the build and never reaches a daemon.

Symptoms this removes:

| Symptom | Evidence |
|---------|----------|
| The editor validator rebuilds the whole schema on every call | `config.ValidateCustomSections` (`internal/component/config/validate_sections.go`) calls `YANGValidatorWithPlugins(nil)`, which parses and resolves every module: 87 to 101 ms per call, about 525 ms under `-race` (scratch benchmark below) |
| CLI editor tests time out under `-race` | Consequence of the row above, multiplied by every validation an editor test performs |
| Thirty-four non-test call sites each build their own loader | `DefaultLoader()` / `NewLoader()` callers listed under Current Behavior |
| Three process caches exist only to hide the cost | `yangSchemaCache` and `yangValidatorCache`, both a `resolvedCache` (`config/yang_schema.go`; this row named `yangSchemaOnce` until 2026-11-17, which no longer exists), `modelLoader` (`config/yang/enum.go`), `modelArgDefs` with its mutex (`config/yang/command_argdefs.go`) |
| A broken module is found at daemon start, not at build | `DefaultLoader` returns the joined error at runtime; `exaBGPSchema` (`internal/exabgp/migration/schema.go`) answers nil on any load error |
| The schema build is the largest single cost of every start | Evidence of 2026-11-17 below: about 290 of 430 ms |
| A start that declares one external plugin builds the module set twice | Evidence of 2026-11-17 below: `yang.resolveDefault`, then `resolveJoined` over the same compiled-in texts plus the plugin's module |

### Evidence of 2026-11-17 (indicative)

Gathered by the session that supervises the external-plugin spec, on the
server. The server is a VM on a host of unknown speed, so no figure below is a
recorded figure or one to compare across machines (`ai/META-DECISIONS.md`,
"Test systems and labs"). Each compares two binaries or two configs side by
side on that one host. The figures that close an acceptance criterion come from
the owner's Mac.

Start to "Ze running.", old and new binaries interleaved, median of 12 to 15
starts, memory-backed directory:

| Config | Before the external-plugin work | After it | Added |
|--------|---------------------------------|----------|-------|
| No external plugin | 429 ms | 431 ms | nothing |
| Go fixture plugin declared | 524 ms | 839 ms | about 315 ms |
| Python example declared | 500 ms | 799 ms | about 300 ms |

The added time does not depend on the plugin. The plugin connects as fast as
before, about 140 ms after its launch. The time from its connection to ready
went from about 10 ms to about 400 ms.

CPU profile, 13 starts merged for each case:

| Case | Where the time is | Per start |
|------|-------------------|-----------|
| No plugin | `yang.resolveDefault`, 3.80 s over 13 starts | about 290 ms |
| Go fixture plugin | The same, plus `joinDeclaredPlugins` → `PublishExternalModules` → `Join` → `resolveJoined`, 3.57 s over 13 starts | about 275 ms more |

One build splits roughly into 110 ms reading module text (`AddModuleFromText`)
and 170 ms in `Resolve`, of which `checkStructure` takes about 100 ms.

These figures and the table "Measured cost" under Current Behavior (87 to
101 ms for one build, 2026-10-16, another host, 90 modules) do not agree, and
the comment on `config.YANGSchema` says about 200 ms. Nothing here explains the
difference: the hosts differ and the module count was not compared. Step P0
measures it on one known host.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - the loader, `Resolved`, every check, registration pattern
  → Constraint: `Resolved` is the only type a reader of a checked module set takes (`BuildCommandTree`, `PathTo*`, `WireMethodTo*`, `PublishedRPCs`, `ExtractRPCs`, `ExtractNotifications`, `NewValidator`, `CheckAllValidatorsRegistered`); the compiled linked schema MUST keep that property: no reader can be handed an unchecked set.
  → Constraint: the four structural refusals (`ErrLengthOrder`, `ErrEnumValue`, `ErrEnumRestriction`, `ErrExtensionSubstatement`) and `ErrUndeclaredExtension` each name module, file location and fault; the generator MUST print the same wrapped errors with the same location.
  → Constraint: an enum's value is Ze's (`parseEnumAssignment`), never goyang's; the compiled enum list MUST carry the values Ze assigns, in value order.
  → Decision: "A binary links every module a linked module imports, because the generated `register.go` of each `yang/` package blank-imports the packages registering its modules' imports". This is what makes per-module compilation possible: a module's import closure is fixed and always linked with it.
- [ ] `docs/architecture/command-ownership.md` - "YANG as Data, Not Code"
  → Constraint: the only hand-written file in a `yang/` package is the `.yang`; every Go file there stays generated by one tool (`./le yang glue`).
- [ ] `docs/architecture/module-tiers.md` and `feature-gates.txt` - compile-out
  → Constraint: 143 `<tag> <pkg>` gate rows; a gated feature's `yang/` package is reached only through a tagged blank import in `plugin/all`. The `yang/` package files carry no build tag of their own (checked: `internal/plugins/gnmi-cmd/yang/register.go`), so a generated file in that package compiles exactly when its module is linked.
  → Decision: no generated output may depend on the tag set chosen at `go build`, because `go generate` cannot know it.
- [ ] `docs/contributing/ze-go-style.md` - "One type per lifecycle state", "A limit on everything"
  → Constraint: the compiled schema is a state type of its own (linked and checked); a zero or nil value ends in a `BUG` panic as `Resolved` does today.
- [ ] `plan/post-release/engineering/spec-validated-construction-and-state-types.md` T1 and T4 - why `Resolved` exists
  → Constraint: `DefaultLoader` and `Resolve` share one strict policy; the generator inherits it: nothing is best-effort.

### RFC Summaries (Scope: protocol)
- N-A: no wire protocol changes. RFC 7950 checks move in time, not in content; the `rfc/short/rfc7950.md` rows and their tests keep their meaning and move with the code (Documentation row 9).

**Key insights:** (minimal context to resume after compaction)
- Resolution of one module depends on the linked set ONLY through augments: the tree has 91 `augment`, 0 `deviation`, 0 `if-feature`, 0 `identityref`, 0 `leafref` (grep of every `.yang` under `internal`, `cmd`, `pkg`). Uses, groupings and typedefs resolve against the import closure, which is fixed.
- An augmenter's `yang/` package blank-imports its target's package, so Go init order registers the target first. A graft at registration needs no lazy step.
- goyang cannot be fed from Go literals: `Statement` keeps its substatements in an unexported field with no constructor, `EnumType` has unexported fields, `Entry.Dir` is a map. Six runtime readers walk `Entry.Node` statements.
- Measured resolved size (default tags, 90 modules): 2,081 entries, 1,146 leaves, 2,506 extension statements, 308 enum values, 134 patterns, depth at most 8, 668,075 octets of YANG text.
- The generator MUST NOT import any package that imports generated output, or a stale generated file stops the generator from building (bootstrap).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/register.go` - `RegisterModule(name, content)` appends `Module{Name, Content, Registrar}`; `RegisterModuleForTest` swaps the slice; `Modules()` exposes the text list
- [ ] `internal/component/config/yang/loader.go` (542L) - `Loader` (`LoadEmbedded`, `LoadRegistered`, `AddModuleFromText`, `AddModuleFromFile`, `Resolve`), `Resolved` (`GetModule`, `GetEntry`, `ModuleNames`, `ConfModuleNames`, `APIModuleNames`), `DefaultLoader`, `sourcedModules`
- [ ] `internal/component/config/yang/loader_structure.go`, `loader_abnf.go`, `loader_grammar.go`, `loader_source.go`, `enum_assignment.go`, `pattern.go` - the Resolve-time checks
- [ ] `internal/component/config/yang_schema.go` - `PluginOnlySchema`, `YANGSchema` (through `yangSchemaOnce`), `loadYANGModules(pluginYANG)`, `YANGValidatorWithPlugins`, `YANGSchemaWithPlugins`, `yangToNode`; schema-build checks `validateTypedefDefault`, `validateDefaultNotIfFeature`, `recordSchemaBuildError`; `yangChildOrder` reads `Entry.Node` substatements for declaration order
- [ ] `internal/component/config/yang/command.go` - `declaredContainerOrder`, `declaredLeafNames` read `Entry.Node.Statement().SubStatements()`
- [ ] `internal/component/config/yang/enum.go` - `modelLoader` cache; `entryTypeStatement` type-switches `Entry.Node`; enum summaries walk `YangType.Base`
- [ ] `internal/component/config/yang/command_argdefs.go` - `modelArgDefs` mutex cache over `PathToArgDefs` of the default loader
- [ ] `internal/component/plugin/inprocess.go` - `CollectPluginYANG(plugins)` maps `ze-<name>.yang` to the `Registration.YANG` text of INTERNAL plugins
- [ ] `internal/component/plugin/registry/registry.go` - `Registration.YANG string`; 98 non-test sites set it from a generated `<Var>YANG` text variable
- [ ] `internal/component/config/schema/cli/main.go` - `ze schema`: `loadAPIRPCs` builds a loader; `registerInternalYANG` calls `ParseYANGMetadata(text)` (a goyang parse); `ze schema show` prints the module text
- [ ] `internal/exabgp/migration/schema.go` - `exaBGPSchema` embeds `exabgp.yang` outside any `yang/` package and resolves it at runtime; answers nil on error
- [ ] `internal/le/yang/glue/*.go` - `./le yang glue update|check` derives `embed.go` and `register.go` (blank imports from YANG `import`/`include`, `imports.go`) for every `yang/` package; `check` is the "yang glue" stage in `internal/le/verify/engine/stages.go` and a row of `generationActions` and `generationChecks` in `internal/le/repo/generate.go`
- [ ] goyang, upstream with the patches under `internal/le/vendorpatch/patches/goyang-*.patch` applied to `vendor/` (no fork), `pkg/yang/entry.go`, `types.go`, `types_builtin.go` - `Entry`, `YangType`, `EnumType`, `Statement` shapes

**Runtime functions that build a loader today (non-test):**

| File | Functions |
|------|-----------|
| `internal/component/aihelp/aihelp.go` | `CLISubcommands`, `loadSchema`, `Build` |
| `internal/component/plugin/server/server.go` | `NewServer` |
| `internal/component/config/yang_schema.go` | `PluginOnlySchema`, `loadYANGModules` |
| `internal/component/config/infra/authz.go` | `validateMatchEntries` |
| `internal/component/config/yang/command_argdefs.go` | the `modelArgDefs` fill |
| `internal/component/config/yang/cli/tree.go` | `addConfigNodes`, `addCommandNodes`, `AllRPCDocs`, `loadRPCParams` |
| `internal/component/config/schema/cli/main.go` | `loadAPIRPCs` |
| `internal/component/config/cli/cmd_edit.go` | `buildEditorCommandTree` |
| `internal/component/cli/completer.go`, `validator.go` | `NewCompleter`, `NewConfigValidator` |
| `internal/component/cli/client/main.go` | `buildYANGState` |
| `internal/component/cli/testing/headless.go` | package-level headless setup |
| `internal/component/iface/validate.go` | `loadReservedIfaceNames` |
| `cmd/ze/hub/session_factory.go`, `command_meta.go`, `service_web.go` | `buildCommandTree`, `commandMetaSource`, `startWebServer` |
| `internal/exabgp/migration/schema.go` | `exaBGPSchema` |
| `internal/le/...` (stay on the compiler or move to the linked schema, per reader) | `arch/enumeration/corpus.go`, `doc/yangcontract/contract.go`, `usage.go`, `helpshape.go`, `config/claims/configclaims.go`, `cli/dispatch/resolver.go`, `cli/list/commandlist.go`, `cli/grammar/cligrammar.go`, `repo/inventory/inventory.go` (reads `Modules()`) |

**goyang reach:** 27 non-test files import `github.com/openconfig/goyang/pkg/yang` (7 under `internal/le`), with about 133 `Entry` type references and 40 `.Node` reads; 35 test files import it; 78 test files call `DefaultLoader` or `NewLoader`; 46 test files load fixture text (`RegisterModuleForTest`, `AddModuleFromText`, `AddModuleFromFile`).

**Measured cost** (scratch `tmp/session/2026-10-15-1b0ec3e6-5946-4d37-8431-e1065e283551/scratch/_yangbench/` and `_yangsize/`, default tags, 88 registered plus 2 embedded modules):

| Step | Plain | `-race` |
|------|-------|---------|
| Parse every module text | 39 to 47 ms | about 305 ms |
| `Resolve` (goyang plus Ze checks) | 46 to 75 ms | 213 to 614 ms |
| `YANGValidatorWithPlugins(nil)` | 87 to 101 ms | 523 to 535 ms |

**Current state: external plugins** (verified at the producer, 2026-10-16):

| Fact | Evidence |
|------|----------|
| External plugin YANG is stored and never parsed or validated | `internal/component/plugin/server/startup.go` `handleRegistration` sets `reg.PluginSchema` from `input.Schema.YANGText`; no non-test code reads `reg.PluginSchema` (only `registration_test.go`). `subsystem.go` `onRegistration` stores `h.schema`; `SubsystemManager.RegisterSchemas` feeds the `ze schema` registry, which prints it |
| Config reaches a plugin only under compiled-in roots | `deliverConfigRPC` calls `BuildPluginConfigSections(configTree, reg.WantsConfigRoots)` over a tree parsed with the compiled-in schema only; `LoadConfig` passes `CollectPluginYANG(cliPlugins)`, which is internal registrations only |
| An external plugin's own top-level config block is refused as an unknown keyword | Inferred from the two rows above; `docs/plugin-development/schema.md` claimed the opposite (verified and corrected 2026-10-16, see the follow-on spec) |
| `PluginOnlySchema` (a two-phase parse helper) has no caller | grep of non-test code |
| A third-party Go plugin runs in process only when its package is added to the generated composition root (`internal/component/plugin/all`) and `ze` is rebuilt; there is no runtime Go plugin loading. Otherwise it runs as an external process through the SDK (`pkg/plugin/sdk`, Go; the protocol is language-agnostic), configured by `plugin { external <name> { run ... } }` (`ze-plugin-conf.yang`) | `plugin/all/all.go` generated by `./le repo update` |

Owner goal (Thomas, 2026-10-16): "I want people to be able to write their own
plugins in Go or any other language and load them at startup". This spec keeps
that goal reachable; `plan/post-release/features/spec-plugin-external-yang-at-startup.md` delivers it.

**Corrected 2026-11-17: the table above is history.** The external-plugin spec
is implemented through its phase 8 on the branch that carries this amendment.
Read at the code on 2026-11-17:

| Fact today | Evidence |
|------------|----------|
| An external plugin's module is joined to the schema at `ze start`, before the full parse | `cmd/ze/hub/main.go` `runYANGConfig` calls `joinDeclaredPlugins` (`cmd/ze/hub/declared_plugins.go`), which calls `config.PublishExternalModules`, which reaches `yang.PublishExternalModules` and `yang.Join` |
| The join works from the TEXT of every compiled-in module | `resolveJoined` (`internal/component/config/yang/join.go`) calls `sourcedModules.reloaded`, which parses every compiled-in text again into a new loader, then runs a full `Loader.Resolve`: goyang's processing and `checkExtensions`, `checkPatterns` and `checkStructure` over modules that already passed |
| A refused join that holds several external modules resolves each one alone again | `resolveJoined` calls itself once per declared module to name the one that fails |
| The module set is published once per process and then fixed | `yang.PublishExternalModules`, `yang.ModuleSetFixed`; a value built once for the process records itself with `yang.KeepsModuleSet`, and a publish after it is refused |
| The schema caches follow the publish | `resolvedCache` (`config/yang_schema.go`) keys on the `*yang.Resolved` that `DefaultLoader` answers |
| Another full resolve exists outside every cache | `loadLCPDefaults` (`internal/component/l2tp/pppconfig/lcp.go`) builds its own loader over every registered module, once per process. Not verified whether a start with no L2TP and no PPPoE reaches it (AC-17) |

What that changes for this spec:

1. The join is in the start path, and it MUST keep working from texts: an
   external module arrives as text and resolves against the compiled-in set. A
   prepared compiled-in set does not by itself remove the join's cost. The
   design below already keeps each module's `Source` text and adds an external
   module through the non-mutating `Link`, so the join's successor compiles the
   external module alone against that text and links it. AC-18 is the proof.
2. The rows of this spec that say external YANG is "stored and printed, never
   resolved" (Boundaries Crossed, Security Review, Known Limitations) describe
   the tree of 2026-10-16. External YANG is parsed and resolved at start today.
3. Q8, Q9 and Q10 of this spec are answered by the external-plugin spec's
   design and by the evidence above. Q10's estimate of 5 to 30 ms per external
   module was wrong for the join as built: it costs one more build of the whole
   set.

**Behavior to preserve:**
- Added 2026-11-17, from the external-plugin work:
  - The published external module set: `yang.PublishExternalModules` publishes once, `yang.ModuleSetFixed` answers whether the set can still change, each keeper (`yang.KeepsModuleSet`) refuses a later publish, and a restarted plugin's declaration is compared with `yang.PublishedModule`.
  - Every refusal of the join, with its text (the Refusals table of the external-plugin spec).
  - A test that registers modules in its own process keeps building from text. `RegisterModuleForTest` callers and the fixture loaders are today's form; AC-14 says where each goes.
  - The config corpus sweep that publishes fixture modules before it parses.
  - The build-tag gate tests (`TestBuildTag_Gate*` in `cmd/ze/hub`), which build `ze` under one tag set each: a generated input MUST compile under every one of them.
- Every refusal text and error sentinel the loader produces today, now produced by the generator with the same file location.
- `ze schema show <module>` prints the module's YANG source (`test/parse/cli-schema-show.ci`).
- Child order: declared children in declaration order (inline-list positional syntax depends on it); grafted children as today (assumption A-4).
- The `Resolved` accessor surface (`ModuleNames`, `ConfModuleNames`, `APIModuleNames`, entry lookup by module) and the `BUG` panic on a zero value.
- `CheckAllValidatorsRegistered` at startup: `ze:validate` names come from YANG, functions register at runtime, so the cross-check stays a runtime check over compiled data.
- `Schema.Define` merging same-named top-level containers across modules (`environment`).
- Every binary links the import closure of every linked module (generated blank imports).

**Behavior to change:**
- No runtime parse or resolve of a compiled-in module. The compiler stays in `ze`, because external plugin YANG is compiled at startup (owner goal of 2026-10-16; built by the external-plugin spec), but a start with no external plugin calls it zero times.
- Schema defects fail `go generate` / `./le yang glue update` and the verify stage, never a daemon start.
- The three caches and every per-caller loader disappear; one linked schema value serves every reader.
- A start that declares an external plugin compiles that plugin's module and links it. It does not parse or resolve a compiled-in module a second time (AC-18).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The loader and its checks | `internal/component/config/yang/loader.go` `Loader`, `Resolve`; `loader_structure.go` `checkStructure` | Parses every registered module text and resolves the set at run time, in each process | Replaces at run time for compiled-in modules: the same code becomes the compiler, run at generation and for an external module |
| The checked set type | `loader.go` `Resolved` | The only type a reader of a checked module set takes | Extends: it becomes the linked compiled schema, same guarantee |
| The generated YANG glue | `internal/le/yang/glue`, `./le yang glue update` and `check` | Generates `embed.go` and `register.go` of every `yang/` package, committed, with a freshness check | Extends: it also emits `compiled.go`; `embed.go` goes |
| Freshness of generated Go inputs | `TestYANGGlueCurrent` and `TestGeneratedPluginImportsCurrent` (`internal/component/plugin/all`), the lists `generationActions` and `generationChecks` (`internal/le/repo/generate.go`) | Every generated Go build input is committed, and a test and a check refuse a stale one | Extends: the compiled output joins the same lists. Way 3 of Q11 would not apply it: its output is written per build and not committed |
| Module registration from `init()` | `config/yang/register.go` `RegisterModule`; each `yang/` package's `register.go` | Registers module text; blank imports order a module after its imports | Extends: registers the compiled module and grafts its augments |
| The schema caches | `config/yang_schema.go` `resolvedCache`, `yangSchemaCache`, `yangValidatorCache` | Build the schema and the validator once per module set | Replaces: one linked value needs no cache |
| The join of an external module | `config/yang/join.go` `Join`, `resolveJoined` | Parses every compiled-in text again and resolves the whole set with the external modules | Replaces its body: compile the external module alone against the kept `Source` texts, then `Link`. Its callers and refusals stay |
| The published external layer | `config/yang/publish.go` `PublishExternalModules`, `ModuleSetFixed`, `KeepsModuleSet`, `PublishedModule` | Publishes the joined set once per process and fixes it | Does not apply a change: it publishes what the join answers |
| Design history entry | `plan/learned/DESIGN-HISTORY.md`, Configuration, Abandoned approaches, "Generating a Go schema from YANG at build time" (166) | Recorded the approach as rejected | Replaces: the owner's decision of 2026-10-16 reverses it; the entry is amended to say so |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: a `.yang` file in a `yang/` package (and `exabgp.yang`, moved into `internal/exabgp/migration/yang/`). Format: YANG 1.1 text.
- Run time: Go package init of each linked `yang/` package.

### Transformation Path
1. Generation (`go:generate` directive, or `./le yang glue update`): discover every module in the tree; parse and resolve every module with goyang through the compiler package; run every Ze check; for each module emit the module's OWN resolved data tree (uses, groupings, typedefs expanded against its import closure), its RPCs and notifications, and each `augment` as a graft record (target path with prefixes resolved to module names, plus the subtree to graft). Emit `compiled.go` and `register.go` beside the `.yang`.
2. Whole-set check at generation: link every module in the tree at once (the full tag set). Every tag set is a subset of it and per-module output does not depend on the set, so a graft conflict or a missing target that any subset could meet is met here.
3. Init: each generated `register.go` registers its compiled module with the schema registry. Registration sets the current linked schema to `Link(current, module)`. `Link` is non-mutating: it copies the spine (the path from the root to each graft target, depth at most 8) and attaches the module's augment records to the copies, so the generated literals are never written and a reader holding the previous value keeps a complete, consistent tree. The target package is initialized first because of the blank import. `Link` compiles the module's patterns once. A target that is absent or a pattern that does not compile is a Ze defect (generation proved both) and panics with `BUG`. The same `Link` adds external modules later (follow-on spec `plan/post-release/features/spec-plugin-external-yang-at-startup.md`).
4. Read: every reader takes the one linked schema value (the successor of `Resolved`) and reads Ze-owned nodes: `config.Schema` build, `BuildCommandTree`, validator, completer, metadata, RPC extraction, aihelp, authz, iface, MCP and web metadata.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build time to run time | Generated Go literal of the Ze-owned compiled module type, static data, no decode | No |
| `yang/` package to schema registry | `init()` in the generated `register.go` (registration pattern) | No |
| Plugin protocol `SchemaDecl.YANGText` | Unchanged: external text is stored and printed, never resolved | No |

### Integration Points
- `Resolved` (`internal/component/config/yang/loader.go`) - becomes the linked compiled schema type; its consumers keep taking it.
- `./le yang glue` (`internal/le/yang/glue`) - the one generator of `yang/` package Go files; extended to emit `compiled.go`; `embed.go` deleted.
- The "yang glue" verify stage and the `./le repo update` check - freshness gate, unchanged entry points.
- The linking function (`Link`) - one function, run at init over compiled-in modules and, in the follow-on spec, at startup over external modules compiled by the same compiler.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Every reader goes through the linked schema value; no reader keeps a loader |
| No unintended coupling (components stay isolated) | Yes (design) | Components still register through `init()`; the schema registry never imports a component |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | The compiler IS today's Loader and checks, moved; the generator IS `./le yang glue`, extended |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | Compiled nodes are static data referenced by pointer; nothing is decoded or copied at init |
| Registration over hardcoding, outbound | Yes (design) | Each module's package registers its compiled module; the registry discovers them |
| Registration over hardcoding, inbound | Yes (design) | No list of module names: the generator discovers `.yang` files (glue `discover`), the registry holds what init registered. Searched: `yang_schema.go` (journal `yang-module-not-in-schema.md` records two past hand-added names; none remains), `PluginOnlySchema` (looks up `ze-plugin-conf` by name; stays a registry lookup, an error when absent) |

## Design: the compiled form

### Alternatives

| # | Form | Cost at run time | Verdict |
|---|------|------------------|---------|
| A1 | Per-module pre-parsed goyang ASTs as Go literals, goyang resolve at init | Saves parse only (39 to 47 ms); resolve stays (46 to 75 ms, up to 614 ms under `-race`); goyang stays linked | Rejected, and not buildable: `Statement` substatements are unexported with no constructor |
| A2 | One fully resolved schema per tag set | Zero | Rejected: `go generate` cannot know the tags a later `go build -tags` uses; 143 gates make the sets combinatorial; any set not generated fails to build |
| A3 | Generated goyang `Entry` literals | Map and enum construction at init; six readers still need statement ASTs | Rejected: `EnumType` and `Statement` carry unexported fields; needs a deeper fork of goyang |
| A4 | Gob or JSON blob per module, decoded at init | One decode per module, milliseconds | Rejected: still a runtime parse of a serialized form; a Go literal is free |
| A5 | **Chosen.** Per-module compiled Ze-owned resolved tree as a static Go literal, augments as graft records applied at registration | Grafts: at most 91 records, each a path walk of depth at most 8; pattern compile of at most 134 patterns once | Chosen |

Why A5: per-module output is independent of the linked set except for
augments (Key insights), so a module can be compiled alone against its fixed
import closure and the varying part reduces to a list of grafts. Go init order
already sequences those grafts. goyang leaves the runtime entirely, and the Go
compiler lays the literal out as static data, so registration costs pointer
writes. The price is the migration of every goyang `Entry` reader to the
Ze-owned type (Effort).

### Compiled module type (Ze-owned, in the runtime schema package)

| Field | Type | Description |
|-------|------|-------------|
| Name | module name | `ze-bgp-conf` |
| Namespace, Prefix | text | from the module header (replaces `ParseYANGMetadata`) |
| Imports | module names | direct imports, for `ze schema` display |
| Source | text | the module text, display only (`ze schema show`); never parsed |
| Nodes | ordered node list | the module's own top-level data nodes, declaration order |
| RPCs, Notifications | ordered node lists | input and output as child nodes |
| Grafts | list of graft records | target path (module-qualified segments) and the nodes to attach |

| Node field | Type | Description |
|------------|------|-------------|
| Name, Kind | text, typed enum (container, list, leaf, leaf-list, choice, case, rpc, input, output, notification) | Zero kind is `Unspecified` |
| Module | module name | owning module; an augmented node keeps its augmenter's (RFC 7950 Section 7.17 namespace) |
| Config, Mandatory | typed tri-state | resolved, inheritance applied |
| Presence | bool | replaces `Entry.Extra["presence"]` |
| Description, Units | text | |
| Default | text list | |
| Key, Unique | text, text lists | replaces `Entry.Extra["unique"]` |
| MinElements, MaxElements, OrderedByUser | numbers, bool | from `ListAttr` |
| When, Must | text lists | carried as text, as today |
| Children | ordered node list | declaration order; grafted children after declared ones (A-4) |
| Extensions | list of (module, keyword, argument, substatements) | the `ze:*` statements, prefix resolved to the declaring module |
| Type | compiled type | leaf and leaf-list only |

| Compiled type field | Description |
|---------------------|-------------|
| Name, Kind | type name and base kind (the Ze equivalent of `TypeKind`) |
| TypedefChain | typedef names from the leaf's type to the built-in; replaces walks of `YangType.Base` |
| Ranges, Lengths | resolved bound lists, typed numbers, with the containment check ported from goyang's `YangRange` |
| Patterns | indexes into the module's pattern table, compiled once at registration |
| FractionDigits | decimal64 |
| Enums | (name, value, description) in value order, Ze's assignment |
| Bits | (name, position) |
| Union | member compiled types |
| Default | the type's own default (typedef default) |

### Feature tags and cross-module resolution

| Question | Answer |
|----------|--------|
| Which file holds a module's output | `compiled.go` in the module's own `yang/` package, so it compiles exactly when the package is linked |
| How imports resolve | At generation, against the import closure, which every binary links with the module (generated blank imports) |
| How augments resolve | Graft records, applied at registration; the target's package is initialized first because the augmenter's package imports it |
| Augment of an augmented node | The augmenter must import the module that added the node (to name its prefix), so init order covers chains |
| Statements whose meaning depends on the linked set (`deviation`, an `if-feature` naming another module's feature, `identityref` with cross-module derived identities) | Refused by the generator with an error naming this constraint; none exists today. The skeleton specs `plan/post-release/features/spec-config-yang-deviation.md`, `plan/post-release/features/spec-config-yang-if-feature.md`, `plan/post-release/features/spec-config-yang-type-identityref.md` inherit the graft mechanism as their design starting point |
| How every tag set is checked | The generator links the full tree once; every tag set is a subset, and per-module output is set-independent |

### Generator wiring

| Item | Decision |
|------|----------|
| Generator | `./le yang glue update` (extended), backed by a library that imports only the compiler and the glue code, never a package holding generated output |
| `go:generate` directive | One directive in `internal/component/config/yang/generate.go`. It runs `go run github.com/ze-software/ze/internal/le/yang/glue/gen`, a module path, because `go generate` runs in the package directory. The `main` finds the repository root from `go.mod`, the way `lepath.Root` does. It does not run `./le`: `le` links the generated output and the patched TLS, so a stale file would stop `le` from building (open question Q3) |
| Core modules | `ze-extensions.yang` and `ze-types.yang` leave the `go:embed modules` in `loader.go` and move to their own package, `internal/component/config/yang/coreyang/`, which gets generated `compiled.go` and `register.go` like any `yang/` package. Every generated `register.go` blank-imports `coreyang` through the normal import derivation (glue `imports.go`), so the "embedded bootstrap module" special case is deleted. `internal/component/config/yang` (registry and compiled type, hand-written) and the compiler then hold no generated output |
| `Registration.YANG` | The field changes from text to a pointer to the compiled module. Each generated `compiled.go` exports one variable per module, named after the module (`ZeDdosFlowspecConf` where the text variable was `ZeDdosFlowspecConfYANG`), whose `Source` field carries the text for display. The 98 sites change from the text variable to the module variable in C3. `registry.go` keeps the field. `YANGSchemas` and the schema CLI read name, namespace, imports and source from it, so `ParseYANGMetadata` is deleted. With Q1 = delete, `CollectPluginYANG` is deleted; `GetInternalPluginYANG` and `GetAllInternalPluginYANG` stay as display lookups for `ze schema`, returning the compiled module |
| Output per module package | `compiled.go` (header "Code generated by ./le yang glue update; DO NOT EDIT.") and `register.go`; `embed.go` deleted |
| Freshness | `./le yang glue check` regenerates in memory and compares bytes for every generated file; the existing verify stage and the `./le repo update` check run it |
| Compiler location | `internal/component/config/yang/compiler` (today's Loader, Resolve and checks moved, unchanged in behavior). It takes YANG text as input (the generator reads files from disk) and imports only `internal/component/config/yang` for the compiled type, never `coreyang` or any `yang/` package. Importers: the generator, `internal/le` tools that read YANG text at a git revision, tests that compile fixture text, and (follow-on spec) `ze` startup compiling external plugin modules |
| Compiler at runtime | The compiler stays linked into `ze`. Today it is reached only through `ze schema`, `le` and tests; the follow-on spec (External plugin schema) calls it at startup for external modules. The runtime proof that compiled-in modules are not parsed is a call counter on `compiler.Compile` asserted to be zero by `TestStartWithoutExternalPluginsCompilesNothing` (AC-7) |

### Editor validator after the change

`YANGValidator()` (the successor of `YANGValidatorWithPlugins(nil)`) builds a
validator over the already linked schema and registers custom validators: no
parse, no resolve. Expected cost below 1 ms per call, against 87 to 101 ms
today; AC-6 measures it.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A module's resolution depends on the linked set only through augments | grep of all `.yang`: 0 deviation, 0 if-feature, 0 identityref, 0 leafref, 91 augment | Some node differs by tag set; the graft model misses it | AC-5 equivalence test over five tag sets | unvalidated |
| A-2 | An augmenter's `yang/` package always imports the target's package | `yang-config-design.md` section 3; glue `imports.go` derives blank imports from `import` | A graft finds no target at init | AC-5, plus a unit test of the `BUG` path | unvalidated |
| A-3 | Readers need only the facts in the compiled type tables | Field census: `.Node` 40 reads, `.Exts` 50, `.Extra` 6, `.Base` typedef walks, `.RPC` 8, `.ListAttr` 11; no reader of `Augmented`, `Uses`, `Deviations`, `Identities`, `Annotation` | A reader needs a statement-level fact not captured | C2 compiles; every existing test green | unvalidated |
| A-4 | Grafted children appear after declared children today | `yangChildOrder` lists AST data statements first, then the names the statement lacks | Completion, `ze schema` or inline-list output reorders | Expected outputs of existing `.ci` and `.et` tests unchanged | unvalidated |
| A-5 | A static Go literal of about 2,100 nodes and 2,500 extensions builds fast and stays small | Measured counts; Go compiles large static tables (unicode) routinely | Build time or binary size regress | C1 measures `go build` time and binary size before and after | unvalidated |
| A-6 | `CollectPluginYANG` adds nothing to the compiled set | It returns `Registration.YANG` of INTERNAL plugins, whose modules `RegisterModule` already registers; the loader skips the duplicates; only `TestReloadVerifierRetainsSelectedPluginSchema` depends on it | Deleting it loses a module some binary reaches only this way | Owner answer to Q1, then a census of `Registration.YANG` modules against registered modules | unvalidated |
| A-7 | Compiling at most 134 patterns at init costs a few milliseconds | Typical Go regexp compile of an address pattern is tens of microseconds | Every `ze` CLI start pays more than measured | AC-6 benchmark of full registration | unvalidated |
| A-8 | No runtime reader needs the text of a compiled-in module except `ze schema show` | grep of `.Content` and `Modules()` readers: `schema/cli`, `aihelp` (names only), `le` tools | Another display surface loses its text | grep in C3 before deleting `Module.Content`; `Source` on the compiled module keeps the text for display | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Bootstrap: a stale generated file breaks the build of the generator itself | `go generate` fails to compile its own `main` | The core modules move to `coreyang`, so `internal/component/config/yang` holds no generated output. The generator `main` imports only glue, the compiler and `internal/component/config/yang`. `TestGeneratorImportsNoGeneratedPackage` walks the `go list -deps` of the generator `main` and fails on any package holding a file with the generated header, and on `coreyang` or any `yang/` package. That includes whatever `config/yang` imports transitively (`internal/component/command`) |
| R-2 | Equivalence gap: the linked compiled tree differs from goyang's whole-set resolve on a subtle field (config inheritance, mandatory, default) | AC-5 red | Fix the compiler emission; never relax the comparison |
| R-3 | Validator semantics drift when range, length and union checks move from goyang types to Ze types | Validator unit tests and `rfc7950_validator_*_test.go` red | Port the tests first (TDD), unchanged inputs and expected errors |
| R-4 | Concurrent specs edit the same loader files (`plan/release/fixes/spec-config-yang-loader-structural-checks.md`, status design) | Merge conflict, or a new check added to the runtime loader after it moved | The check lives in the compiler package; ordering is owner question Q6 |
| R-5 | A transient duplicate registration of `ze-mtu-cmd`, `ze-ping-cmd`, `ze-traceroute-cmd`, seen 2026-10-16. It comes from the in-flight import fix of `spec-plugin-ownership-imports` (route A'), whose untracked modules sit in `internal/component/{mtu,ping,traceroute}/yang/` beside the tracked `internal/plugins/*-cmd/yang/` | Generator or loader reports a duplicate module | Not this spec's; it clears when that work lands. The generator refuses duplicates exactly as `LoadRegistered` does today. C1 starts after that spec lands |
| R-6 | Test migration is larger than the reader migration: 78 test files call `DefaultLoader`/`NewLoader`, 46 register or load fixture text, 35 import goyang (overlapping) | Test compile errors when C2 and C3 land | C2 ports the 35 goyang test files. C3 moves about 100 distinct files to the linked schema (a test of the real modules) or to `compiler.Compile` (a fixture test). No test mutates the global registry. Counted in Effort |
| R-9 | C2 and C3 each sit uncommitted in the shared checkout for about a week while their work packages run | Another session edits a file C2 or C3 holds; a pull is owed and the tree is dirty | Q7: the owner chooses a worktree branch or the shared checkout. Each work package writes the per-spec state file |
| R-7 | A reader holds a linked schema value while a later `Link` produces a new one (at init today, at startup for external modules in the follow-on spec) | A reader sees a node change under it; `-race` reports a write to a generated literal | `Link` copies the path to each graft target and never writes its input; `TestLinkNeverWritesItsInput` compares the input value before and after a link; `TestLinkedSchemaConcurrentReaders` runs readers over the old value under `-race` while a new one is linked |
| R-8 | Generated literal size grows the binary | Binary size measured in C1 | The text was already embedded; compiled data replaces resolve-time heap. Growth above 2 MB is reported before continuing |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config parsing, editor validation, completion, CLI command tree, API metadata: every schema reader. A wrong node rejects valid config or accepts invalid config |
| How is it reverted? | Per-phase commits revert cleanly; no config or on-disk format changes |
| Who else touches this path? | `spec-config-yang-loader-structural-checks` (pre-release, design), the eight `spec-config-yang-*` skeletons, `spec-yang-feature-opt-ins-owned-by-feature` (design), any session adding a `.yang` module |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go generate ./internal/component/config/yang/` | → | glue generator writes `compiled.go` per module | `TestGenerateWritesCompiledModuleBesideEachYANG` (`internal/le/yang/glue/yangglue_test.go`) |
| Package init of `plugin/all` | → | registration and graft into the linked schema | `TestLinkedSchemaHoldsEveryLinkedModule` (`internal/component/plugin/all/schema_test.go`) |
| `ze config validate` on a config using an augmented leaf | → | `config.Schema` built from the linked schema | `test/parse/yang-compiled-augmented-leaf.ci` |
| Editor `set` of an out-of-range value | → | `YANGValidator()` over the linked schema | `test/editor/commands/yang-compiled-range-refused.et` |
| Broken module in a fixture tree | → | generator refuses | `TestGenerateRefusesBrokenModule` (`internal/le/yang/glue/yangglue_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go generate ./internal/component/config/yang/` on a clean tree | Writes `compiled.go` and `register.go` beside every `yang/` module and in `internal/exabgp/migration/yang/`; a second run changes no byte; output equals `./le yang glue update` |
| AC-2 | A fixture module with an undeclared `ze:comand` extension | Generation exits non-zero and prints the `ErrUndeclaredExtension` text naming module, file and line; writes no file |
| AC-3 | Fixture modules each carrying one of: uncompilable pattern, descending `length`, duplicate enum value, restricted enum the base lacks, bad extension substatement, unresolved import, invalid typedef default, default naming an `if-feature` enum, absent augment target, `deviation` | Each refused at generation with today's sentinel and location; the set-dependent ones with the new refusal naming the reason |
| AC-4 | A `.yang` edited, generation not rerun | `./le yang glue check` exits non-zero and names each stale generated file |
| AC-5 | In-memory: `compiler.Emit` over every module in the tree, then `Link` once per subset, each compared with `compiler.Resolve` over the same subset's text. The subsets: (1) every module in the tree; (2) the modules the default daemon tags link, read from `featuretags` plus the `go list -deps` of `plugin/all` that `internal/le/plugin/imports` already derives; (3) the same for the `ze_core` tags; (4) the closure of `ze-bgp-conf` plus `ze-graceful-restart` (one augmenter); (5) the closure of `ze-iface-conf` with no augmenter. At run time, `TestLinkedSchemaHoldsEveryLinkedModule` compares the init-linked schema with `compiler.Resolve` of the linked modules' `Source` text, under whatever tag profiles verify already runs tests with. No per-tag `go test` run is added | Equal, node by node (name, kind, module, config, mandatory, presence, default, key, unique, list bounds, when, must, extensions, type facts, child order), to the compiler's whole-set goyang resolve of the same modules |
| AC-6 | Benchmarks on the development host, no `-race` | Registration and link of every module at most 5 ms; `YANGValidator()` at most 1 ms per call (today 87 to 101 ms); `config.YANGSchema()` build time reported |
| AC-7 | `ze` starts with a config that names no external plugin (the compiler stays linked for external plugin YANG, owner goal 2026-10-16) | `compiler.Compile` is called zero times: a call counter in the compiler package reads zero after start (`TestStartWithoutExternalPluginsCompilesNothing`) |
| AC-8 | `ze schema show ze-bgp-conf` | Prints the module source unchanged (`test/parse/cli-schema-show.ci` green) |
| AC-9 | Config setting graceful-restart under `bgp peer session capability` | Parses and validates through the grafted node (`test/parse/yang-compiled-augmented-leaf.ci`) |
| AC-10 | Editor test package under `-race` | Completes inside the default test timeout; an out-of-range value is refused with today's message (`yang-compiled-range-refused.et`) |
| AC-11 | `ze exabgp migrate` on an ExaBGP config | Output unchanged (existing `test/exabgp` suite); schema compiled; no nil-on-error path |
| AC-12 | grep of the runtime tree | No `DefaultLoader`, `NewLoader`, `LoadRegistered`, `AddModuleFromText`, text `RegisterModule`, `RegisterModuleForTest`, `modelLoader`, `yangSchemaOnce`, `modelArgDefs`, `ParseYANGMetadata`; no `embed.go` in a `yang/` package |
| AC-13 | Every existing completion, `ze schema`, inline-list and command-tree test | Green with unchanged expected output (child order preserved) |
| AC-14 | Every test file that today calls `DefaultLoader`/`NewLoader` (78) or registers or loads fixture text (46), about 100 distinct | Reads the init-linked schema when it tests the real modules, or compiles fixture text with `compiler.Compile` and passes the value; no test mutates the global registry; grep finds no `RegisterModuleForTest` |

Added 2026-11-17. AC-15 to AC-17 and AC-19 depend on neither Q11 nor the
compiled form: each is built on today's loader, before C1, and stays valid
after it because the same code becomes the compiler.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-15 | `checkStructure` over every registered module (`loader_structure.go`). Today `moduleLengthErrors` and `moduleEnumErrors` each call `moduleTypeNodes`, so each module's statement tree is walked twice, and `appendSubstatementNodes` reads the struct tags of every field of every node by reflection (`isSubstatementField`, `substatementKeyword`) | One walk per module serves both checks, and the substatement fields of a statement type are computed once per type, not once per node. Every refusal keeps its sentinel, text and location: the existing `loader_structure` tests pass unchanged. The benchmark of AC-19 reports fewer allocations for the `checkStructure` stage than before |
| AC-16 | A join of one external module to the compiled-in set (`resolveJoined`), and a refused join that holds several external modules | The join runs Ze's checks (`checkExtensions`, `checkPatterns`, `checkStructure`) over the external modules only: a compiled-in module that passed them when `DefaultLoader` resolved the set is not checked again. A refused join names the failing module without one more whole-set resolve per declared module. Every `TestJoin*` test passes unchanged. This criterion is absorbed by AC-18 once the join's body is the compile-and-link of the design |
| AC-17 | `loadLCPDefaults` (`internal/component/l2tp/pppconfig/lcp.go`) | Verified by a run whether a start with no L2TP and no PPPoE config reaches it. Whatever the answer, it reads the set `DefaultLoader` already resolved and builds no loader of its own, so no start resolves the module set a second time for it |
| AC-18 | `ze start` with one external plugin declared, against the same binary with none | The start with the plugin costs about one schema build, not two: the profile shows no second parse or resolve of a compiled-in module under `joinDeclaredPlugins`, and the added start time is the plugin's own launch and the compile of its one module. Shown on the owner's Mac, both configs interleaved. This closes R-19 and Q10 of the external-plugin spec |
| AC-19 | A benchmark in a package where every module is registered (`internal/component/plugin/all`), staging today's path: load of the module texts, goyang `Process`, Ze's checks one by one, the lowering to `config.Schema` | Reports time and allocations per stage. The allocation figures are valid on any host; a time figure is recorded only from the owner's Mac. Run before any implementation of C1 or of way 3 (Implementation Steps, P0), so the design is chosen on where the time is |

Corrections of 2026-11-17 to the rows above, each left as written:

| Row | Correction |
|-----|------------|
| AC-6 | "The development host" is the owner's Mac for every time figure (`ai/META-DECISIONS.md`, "Test systems and labs"). The 87 to 101 ms it cites is the figure of 2026-10-16 and is not confirmed by the evidence of 2026-11-17 |
| AC-12 | `yangSchemaOnce` is `yangSchemaCache` and `yangValidatorCache` (`resolvedCache`) today. `AddModuleFromText` cannot leave the runtime tree: the join of an external module reads text. The row holds for compiled-in modules: no runtime call parses the text of one |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGenerateWritesCompiledModuleBesideEachYANG` | `internal/le/yang/glue/yangglue_test.go` | AC-1 over a fixture tree | |
| `TestGenerateIsByteStable` | same | AC-1 second run | |
| `TestGenerateRefusesBrokenModule` (one row per AC-3 fault) | same | AC-2, AC-3 | |
| `TestGlueCheckNamesStaleCompiledFile` | same | AC-4 | |
| `TestGeneratorImportsNoGeneratedPackage` | `internal/le/yang/glue/gen_imports_test.go` | R-1 | |
| `TestLinkedEqualsWholeSetResolve` (five subset rows, in-memory link, as named in AC-5) | `internal/component/config/yang/compiler/link_test.go` | AC-5 | |
| `TestGraftMissingTargetIsBUG` | `internal/component/config/yang/registry_test.go` | A-2 guard | |
| `TestLinkedSchemaConcurrentReaders` (`-race`) | same | R-7 | |
| `TestLinkedSchemaHoldsEveryLinkedModule` | `internal/component/plugin/all/schema_test.go` | wiring | |
| `BenchmarkRegisterAll`, `BenchmarkYANGValidator` | `internal/component/plugin/all/schema_bench_test.go` | AC-6 | |
| `TestStartWithoutExternalPluginsCompilesNothing` | `internal/component/plugin/all/schema_test.go` | AC-7 | |
| `TestLinkNeverWritesItsInput` | `internal/component/config/yang/registry_test.go` | R-7, non-mutating `Link` | |
| Ported: `rfc7950_validator_*_test.go`, `validator_test.go`, `enum_test.go`, `command_test.go`, `loader_*_test.go` (the last move to the compiler) | their packages | R-3, behavior unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| enum value (RFC 7950 Section 9.6.4.2) | -2147483648..2147483647 | 2147483647 | -2147483649 | 2147483648 |
| range bound, `uint64` leaf | 0..18446744073709551615 | 18446744073709551615 | -1 | 18446744073709551616 |
| graft path depth | 1..8 today | 8 | 0 (empty path refused) | N/A (bounded by the tree) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `yang-compiled-augmented-leaf` | `test/parse/yang-compiled-augmented-leaf.ci` | Operator configures a leaf another module augments; the config validates | |
| `yang-compiled-range-refused` | `test/editor/commands/yang-compiled-range-refused.et` | Operator types an out-of-range value in the editor and is refused | |
| `cli-schema-show` (existing) | `test/parse/cli-schema-show.ci` | Operator prints a module's YANG | |
| existing `test/exabgp` suite | `test/exabgp/` | ExaBGP migration unchanged | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/config/yang/register.go` - registry of compiled modules, graft at registration
- `internal/component/config/yang/loader.go` - `Resolved` becomes the linked compiled schema; the Loader moves to the compiler
- `internal/component/config/yang/validator.go`, `validator_mandatory.go`, `validator_registry.go`, `command.go`, `command_argdefs.go`, `enum.go`, `rpc.go`, `rpc_publish.go`, `metadata.go` - read compiled nodes; `modelLoader`, `modelArgDefs`, `ParseYANGMetadata` deleted
- `internal/component/config/yang_schema.go`, `validate_sections.go`, `flatten.go`, `loader.go`, `internal/component/config/claims/schema.go` - `yangSchemaOnce`, `loadYANGModules`, the `PluginOnlySchema` loader, schema-build checks (moved to the compiler)
- `internal/component/config/yang/cli/tree.go`, `internal/component/config/cli/cmd_edit.go`, `internal/component/config/schema/cli/main.go`, `internal/component/config/infra/authz.go`
- `internal/component/cli/completer.go`, `completer_validate.go`, `validator.go`, `internal/component/cli/client/main.go`, `internal/component/cli/testing/headless.go`
- `internal/component/aihelp/aihelp.go`, `internal/component/iface/validate.go`, `internal/component/plugin/server/server.go`, `internal/component/plugin/inprocess.go`, `internal/component/plugin/cli/test_cmd.go`, `internal/component/bgp/config/loader.go`, `internal/component/bgp/config/register.go` (pluginYANG, pending Q1)
- `cmd/ze/hub/session_factory.go`, `command_meta.go`, `service_web.go`
- `internal/exabgp/migration/schema.go` - reads the compiled `exabgp` module
- `internal/component/plugin/registry/registry.go` - `Registration.YANG` becomes the compiled module pointer; `YANGSchemas` returns modules
- the 98 non-test `Registration.YANG` sites (for example `internal/plugins/ddos/flowspec/register.go`, `internal/plugins/copp/register.go`) - name the module variable
- `internal/le/repo/inventory/inventory.go` - reads the linked schema instead of `Modules()`
- `internal/component/config/yang/loader.go` - `go:embed modules` deleted (core modules move to `coreyang`)
- `internal/le/yang/glue/*.go` - emit `compiled.go`, stop emitting `embed.go`, check compares compiled output
- `internal/le/arch/enumeration/corpus.go`, `internal/le/doc/yangcontract/*.go`, `internal/le/config/claims/configclaims.go`, `internal/le/cli/dispatch/resolver.go`, `internal/le/cli/list/commandlist.go`, `internal/le/cli/grammar/cligrammar.go` - linked schema or compiler, per reader
- every `yang/` package: `embed.go` deleted, `register.go` regenerated (generated, never hand-edited)
- `docs/architecture/config/yang-config-design.md` sections 1 and 3 and the loader refusal prose; `docs/architecture/command-ownership.md` "YANG as Data, Not Code"; `ai/INDEX.md` `./le yang glue` row; `ai/patterns/registration.md` where it shows `embed.go`

- Added 2026-11-17, for steps P0 to P3: `internal/component/config/yang/loader_structure.go` (`checkStructure`, `moduleTypeNodes`, `appendSubstatementNodes`), `internal/component/config/yang/join.go` (`resolveJoined`, `sourcedModules.reloaded`), `internal/component/l2tp/pppconfig/lcp.go` (`loadLCPDefaults`), and a benchmark file in `internal/component/plugin/all/`
- Added 2026-11-17, for AC-18: `internal/component/config/yang/join.go` and `publish.go` (the join's body becomes compile and link; its callers, refusals and the published layer stay), `cmd/ze/hub/declared_plugins.go` only if the call changes

## Files to Create
- `internal/component/config/yang/compiler/` - the moved Loader, Resolve and checks, plus emission and the link-equivalence test
- `internal/component/config/yang/generate.go` - the `go:generate` directive
- `internal/le/yang/glue/gen/main.go` - the standalone generator `main`
- `internal/exabgp/migration/yang/` - `exabgp.yang` moved here, generated files beside it
- `internal/component/config/yang/coreyang/` - `ze-extensions.yang` and `ze-types.yang` moved from `config/yang/modules/`, generated files beside them
- `test/parse/yang-compiled-augmented-leaf.ci`, `test/editor/commands/yang-compiled-range-refused.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No module content changes |
| YANG validation constraints | No | Constraints unchanged; their checks move to generation |
| YANG custom validators | No | `ze:validate` registration unchanged; `CheckAllValidatorsRegistered` reads compiled data |
| CLI commands/flags | No | No command added; `ze schema show` output preserved |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | `internal/component/cli/completer.go` reads compiled nodes; output unchanged |
| Functional test for new RPC/API | N-A | No new RPC; the functional tests above cover the path |
| Pipe completeness | N-A | No new output |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | N-A | Removes a runtime dependency (text resolution); adds none |
| Prometheus counters/metrics | No | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | No user-visible feature; editor latency improves |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | Plugin registration still names a YANG module |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | `SchemaDecl.YANGText` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 7950 tagged tests move to the compiler package: `rfc/short/rfc7950.md` source anchors updated in the phase that moves them |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if fixture YANG in tests changes shape; the `docs/contributing/` page naming generated files |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` sections 1 and 3, `docs/architecture/command-ownership.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation: `./le spec citation anchors spec plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md`; known: `yang-config-design.md` anchors on `loader.go`, `loader_structure.go`, `loader_abnf.go`, `loader_grammar.go`, `loader_source.go`, `enum_assignment.go`, `command.go`, `register.go`. Declared by `// Design:` headers of changed files: `docs/architecture/config/syntax.md` (affected where it says the schema is loaded from YANG text at startup; checked in C2), `docs/architecture/core-design.md` (affected: config pipeline start, YANG loading; C2), `docs/architecture/hub-architecture.md` (affected where it describes `buildCommandTree` building a loader; C2), `docs/architecture/api/architecture.md` (declared by `registry.go`; affected where it describes `Registration.YANG` as schema text; checked in C3), `docs/features/ai-first.md` (affected only if it names runtime YANG parsing for `ze help ai`; checked in C2, otherwise unaffected because output is unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `yang-config-design.md` "Registration Pattern" shows `embed.go`; updated in C3 |

## Implementation Steps

`ai/rules/no-layering.md`: "When replacing X with Y, X MUST be deleted first and
Y MUST be implemented after. A hybrid, a gradual migration, a fallback to the
old path and any other shape that keeps both MUST NOT be written."

Added 2026-11-17: four steps come first. None waits on Q11, none replaces a
form or a producer, and each lands as a commit of its own. The work starts in
a tree of its own, by the owner's order of 2026-11-17 ("We can start that
optimization in a different tree").

| Step | Work | Criterion | Why before C1 |
|------|------|-----------|---------------|
| P0 | The staged benchmark over every registered module | AC-19 | The cheapest experiment: it says where the time of one build is (text load, goyang `Process`, each Ze check, the lowering), which decides what C1 to C3, or way 3, can save |
| P1 | `checkStructure`: one walk per module, substatement fields computed once per type | AC-15 | About 100 of the 170 ms of `Resolve` in the indicative profile, and removable whatever Q11 decides |
| P2 | The join checks the external modules only, and names a failing module without a whole-set resolve per module | AC-16 | Lowers the cost of a start with an external plugin before the compiled form exists |
| P3 | `loadLCPDefaults` reads the resolved set | AC-17 | A full resolve outside every cache |

What P1 to P3 do not do: the join still parses every compiled-in text and runs
goyang's processing over the whole set. Only the compile-and-link of the
design (C3, AC-18) removes that.

Two things get replaced here, and each is replaced in exactly one commit:

| Replaced | X | Y | Commit that swaps it |
|----------|---|---|----------------------|
| The form readers consume | goyang `Entry` and `Statement` | the Ze compiled node type | C2 |
| The producer of the schema | runtime parse and resolve of registered text | generated literals registered at init | C3 |

That gives three commits. In each one, every reader reads exactly one form, and
exactly one producer is live. No commit registers a module twice, ships
`embed.go` beside `compiled.go`, or keeps a fallback. Each commit is built
from several work packages and lands as one commit.

1. **C1: compiler and compiled type, no runtime change (phase 1, wiring of the generator chain)**
   - Move the Loader, Resolve and every check into `internal/component/config/yang/compiler`, unchanged. Runtime callers import them from there; it is the same single path.
   - Add the compiled type, `Emit` (a resolved set to compiled modules, in memory), the in-memory `Link`, the glue emitter, the `go:generate` directive and the standalone generator `main`. The emitter writes only into the temporary fixture trees of its own tests. C1 puts no generated file in the tree and adds no registry.
   - Tests: `TestGenerateWritesCompiledModuleBesideEachYANG`, `TestGenerateIsByteStable`, `TestGenerateRefusesBrokenModule`, `TestGlueCheckNamesStaleCompiledFile`, `TestGeneratorImportsNoGeneratedPackage`, `TestLinkedEqualsWholeSetResolve`.
   - Verify: the equivalence test is red before emission is complete and green after. Measure `go build` time and binary size against an emitted tree in a temporary copy (A-5, R-8).
2. **C2: every reader moves to the compiled type (phases 2 and 3 as work packages, one commit)**
   - Delete goyang `GetEntry`, `GetModule` and `Statement` access from `Resolved`. `Resolved` then holds only the compiled linked set, built by `compiler.Emit` plus `Link` from the registered text: the same producer as today, so the runtime cost is unchanged.
   - Port every runtime reader and the `internal/le` readers in that one commit: validator, command tree, arg defs, enums, RPC extraction, metadata, config schema build, completer, CLI client, aihelp, authz, iface, plugin server, schema CLI, hub, exabgp. Also port the 35 test files that import goyang.
   - Afterwards goyang is referenced only inside the compiler package and its tests.
   - Tests: ported validator, enum and command tests, plus the AC-9, AC-11 and AC-13 suites.
3. **C3: the producer switch (phase 4, one commit)**
   - Generate `compiled.go` and `register.go` for every module and move the core modules to `coreyang`. Add the registry with init grafts.
   - Delete in the same commit: the runtime call of the compiler; text `RegisterModule`, `Module.Content`, `RegisterModuleForTest`, `Modules()`; every `embed.go`; `ParseYANGMetadata`; the three caches and every per-caller loader; the pluginYANG path (after Q1).
   - Change the 98 `Registration.YANG` sites to the compiled module reference (Generator wiring).
   - Migrate the test files that call `DefaultLoader`/`NewLoader` (78) or register fixture text (46). They overlap, so about 100 distinct files. Each goes to the linked schema or to the compiler.
   - Measure the other schema caches (`gnmi` `schemaOnce`, `traffic` and `firewall` `backendGateSchemaOnce`, `command.go` `validateOnce`) and delete each one with no measured problem (`ai/rules/simplicity.md`).
   - Tests: `TestLinkedSchemaHoldsEveryLinkedModule`, `TestGraftMissingTargetIsBUG`, `TestLinkedSchemaConcurrentReaders`, benchmarks, AC-7, AC-10, AC-12, AC-14.
4. **Docs:** each commit edits the pages its code makes wrong. The last work package of C3 runs the anchor sweep (`./le spec citation anchors`).

The cost of this shape: C2 and C3 are each built over about a week of
uncommitted work in the shared checkout before they land (R-9, Q7).

### Effort

| Commit | Estimate |
|--------|----------|
| C1 | 3 to 4 days |
| C2 | 9 to 11 days: about 20 runtime files, 8 `le` readers, 35 test files importing goyang, validator range and union port |
| C3 | 6 to 8 days: about 170 generated packages, 98 registration sites, about 100 test files, 3 to 7 caches |
| Total | 18 to 23 agent-days, about 20 to 25 work packages of 100 tool calls |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | The equivalence test covers five tag sets; no runtime reader reads goyang |
| Naming | One generated variable per module, named after the module; no `<Module>YANG` text variable remains; all 98 `Registration.YANG` sites name the module variable |
| Data flow | Only generated `register.go` writes the registry; only init writes; readers never mutate |
| Rule: no-layering | No reader consults both the text loader and the compiled schema |
| Rule: principles (silent zero) | No reader answers empty when a module is absent; absence is an error or a `BUG` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No compile of a compiled-in module at start | `TestStartWithoutExternalPluginsCompilesNothing` output pasted (compiler call counter zero) |
| Generated files fresh | `./le yang glue check` |
| Validator cost | `BenchmarkYANGValidator` output pasted |
| Broken module fails the build | `TestGenerateRefusesBrokenModule` output pasted |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | External plugin `YANGText` stays display-only; nothing parses it at runtime |
| Fail closed | A missing graft target or pattern at init panics with `BUG`, never skips the node |

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

## Open Owner Questions

| # | Question | Recommendation |
|---|----------|----------------|
| Q1 | Keep or delete the `pluginYANG` parameter path (`CollectPluginYANG`, the argument of `YANGValidatorWithPlugins` and `YANGSchemaWithPlugins`, `parseTreeWithYANG`, the `plugin test` command)? Only `TestReloadVerifierRetainsSelectedPluginSchema` depends on it. Producer confirmed 2026-10-16: `CollectPluginYANG` calls `GetInternalPluginYANG`, which reads `registry.Lookup(name).YANG` from the INTERNAL init registry, fed by the CLI `--plugin` names (`LoadConfig` `cliPlugins`). It is NOT the external plugin path: external YANG arrives over the plugin socket in `DeclareRegistrationInput.Schema` and never reaches it | Delete it; the external path is built in `plan/post-release/features/spec-plugin-external-yang-at-startup.md` |
| Q8 | External plugin YANG: in what order are the plugin block parsed, the external plugins started, their YANG compiled and linked, and the full config parsed? | Two-phase load; decided in `plan/post-release/features/spec-plugin-external-yang-at-startup.md` |
| Q9 | On config reload, does an external plugin restart relink its module, and what does a reader holding the old schema see? | Relink through the non-mutating `Link`; decided in the follow-on spec |
| Q10 | What startup time budget may compiling external modules take (estimate 5 to 30 ms per module, unmeasured)? | Owner sets the number in the follow-on spec |
| Q2 | Bucket: `plan/` (the release ships without it) or `plan/pre-release/` (editor tests time out under `-race`)? | `plan/` |
| Q3 | `go:generate` runs a standalone generator `main` with `go run`, not `./le`, because `le` links the generated output (a stale file would stop `le` building). Acceptable? | Yes |
| Q4 | Keep each module's source text in the binary for `ze schema show`? | Yes, display only |
| Q5 | Ze-owned compiled type replacing goyang's `Entry` in every runtime reader, against extending the goyang fork with exported constructors | Ze-owned type |
| Q6 | Order against `plan/release/fixes/spec-config-yang-loader-structural-checks.md` (design): land it first into the runtime loader, or into the compiler after C1? | After C1, into the compiler |
| Q7 | `ai/rules/no-layering.md` is met by landing C2 (every reader) and C3 (the producer switch) as one commit each, so no commit has two forms or two producers. The cost is that each sits uncommitted in the shared checkout for about a week, built by several work packages (R-9). The alternative is a bounded coexistence: readers ported one commit at a time while `Resolved` serves both goyang entries and compiled nodes. The rule bans that. Which carrier for C2 and C3: the shared checkout, or a worktree branch you authorize (`AGENTS.md`: never a worktree unless you instruct it)? | A worktree branch per commit, rebased onto main when it lands |

### Q11: OPEN OWNER DECISION (recorded 2026-11-17)

On 2026-11-17, before the session found this spec again, the owner was given
three ways to pre-parse the compiled-in YANG and answered "3". Way 3: a small
generator run before each build, importing only the YANG packages of the
compiled-in plugins, writing a file the build embeds, skipped when a
fingerprint of the YANG module files matches the last output; with a
fingerprint check at start and a build from text when the registered modules
do not match.

His recorded decision of 2026-10-16 is "compile [the YANG] at build time, use
go:generate script and import". This spec designs it as A5. It rejects a
per-tag-set output (A2) and a blob decoded at start (A4), and it allows no
fallback ("No commit registers a module twice, ships `embed.go` beside
`compiled.go`, or keeps a fallback"). Way 3 is close to A2 plus A4 with a
fallback.

The question: which design is built, A5 as recorded, or way 3? His "3" decides
nothing here. A5 stays the recorded design until he answers, and nothing of
C1 to C3 or of way 3 is built before that. Steps P0 to P3 do not wait.

| | The recorded design (A5) | Way 3 |
|---|--------------------------|-------|
| What is stored | One generated Go file per module, committed beside its `.yang`: a literal of the Ze-owned compiled tree, with augments as graft records | One file per build, written by the generator and embedded, not committed: the pre-parsed set of the modules that build links |
| The builders' differing tag sets (the `./ze` launcher, `bin/le`'s in-process builds, the Docker image, the appliance build with `ze_appliance` and no `ze_distro`, a plain `go build` or `go test`) | No builder does anything. A module's file compiles exactly when its package is linked, and its content does not depend on the tag set; the grafts are applied at init | Every builder runs the generator first, with its own tags. A builder that does not, a plain `go build` or `go test`, embeds a file that is absent or made for another set, and the start falls back to the text build |
| Cost at build time | Nothing per build. Regeneration when a `.yang` file changes, and a freshness test and check, as for the glue today | The generator before each build, skipped when the fingerprint is unchanged. Unmeasured |
| What a start needs | Init registration and at most 91 grafts. No decode, no fingerprint, no text build | A fingerprint check, then a decode of the embedded file, or a full build from text on a mismatch |
| A stale or wrong input | Refused before any binary exists, by the freshness check | Found at start, by the fingerprint, and answered by the slow path, silently unless it is reported |
| What a schema reader reads | The Ze-owned type: every goyang reader is ported (C2, 9 to 11 agent-days) | Not settled. Research of 2026-11-17: a stored goyang form cannot hold what Ze reads. `Entry.Parent`, `Entry.Node`, `Errors`, `Deviations`, `YangType.Base` and `Root` are tagged `json:"-"`; a statement's substatements and positions and the enum fields are unexported. Ze reads `Entry.Node` statements for child order, extensions and typedef chains, and the lowered `config.Schema` keeps goyang entries (`choiceMembers`). So way 3 stores either a Ze-owned form, which needs the same port as C2, or a form that still needs goyang's processing at start |
| Rules it meets or departs from | `ai/rules/no-layering.md`: one producer per commit | The fallback keeps the text producer beside the stored one, which that rule bans; the output is a generated build input that is not committed, which no generated input is today |
| What it does for a start with an external plugin | The join compiles one module and links it (AC-18) | Nothing by itself: the join still needs the compiled-in texts and a whole-set resolve |

Recommendation of the supervising session, not a decision: keep the recorded
design. It delivers what he asked for on 2026-11-17, the compiled-in YANG
pre-parsed with no cost at each build, and it needs neither a fingerprint nor
a fallback.

## Design Insights

- Research of 2026-11-17: a prepared compiled-in set does not by itself make a start with an external plugin cheaper. The join needs the compiled-in texts, because an external module imports and augments them. The saving comes from compiling the external module alone and linking it (AC-18).
- Research of 2026-11-17: every Go build input generated today is committed and has a freshness test (`TestGeneratedPluginImportsCurrent`, `TestYANGGlueCurrent`, `./le yang glue check`, the lists in `internal/le/repo/generate.go`). Each plugin's `yang/` package imports only `config/yang` and other `yang/` packages.
- Found while measuring (2026-10-16): the working tree registers `ze-mtu-cmd`, `ze-ping-cmd` and `ze-traceroute-cmd` twice. The cause is the transient in-flight import fix of `spec-plugin-ownership-imports` (route A'): untracked `internal/component/{mtu,ping,traceroute}/yang/` beside the tracked `internal/plugins/*-cmd/yang/`. The loader refuses the duplicates (R-5).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-module compiled tree plus graft records | A1 AST literals, A2 per-tag-set output, A3 goyang literals, A4 serialized blob | Only augments vary with the linked set; grafts isolate that variance; static data costs nothing at init |
| Non-mutating `Link` at registration (path copy to each graft target), no lazy link | A `sync.OnceValue` link on first read; grafting in place into the generated literals | Go init order already places the target first; no once and no cache remains. A path copy (depth at most 8) leaves every earlier value intact, so the same `Link` can add external modules at startup while readers hold the compiled-in value |
| The YANG compiler stays linked into `ze` (owner goal 2026-10-16: "I want people to be able to write their own plugins in Go or any other language and load them at startup") | A generator-only compiler, kept out of `ze` by an arch tier rule (the earlier AC-7) | External plugin YANG arrives at runtime and must be compiled then (follow-on spec `plan/post-release/features/spec-plugin-external-yang-at-startup.md`). The cost of compiled-in modules stays zero because AC-7 proves no compile runs without an external plugin |
| Generator independent of generated output | Run `./le` from the directive | Bootstrap: a stale file must not stop the generator from building |
| Whole-tree check once at generation | Check each tag set | Per-module output is set-independent; every tag set is a subset of the whole tree |

## Known Limitations
- External plugin YANG (`SchemaDecl.YANGText`) is not compiled by this spec; it is stored and printed, as today. Compiling it at startup is `plan/post-release/features/spec-plugin-external-yang-at-startup.md`. This spec enables it in three ways: the compiler stays in `ze`; each compiled module keeps its `Source` text, so an external module's `import` of `ze-types`, `ze-extensions` or an augment target compiles against it; and `Link` is non-mutating, so an external module is linked onto the compiled-in value without writing it.
- Owner ruling (Thomas, 2026-10-16): "if a user use an external plugin the security is on him". There is no trust sandbox for external plugin YANG. Two refusals still apply, in the follow-on spec: invalid YANG is refused, naming the plugin and the module; and a module whose name or namespace clashes with a compiled-in module is refused, because the schema would be ambiguous. Trust design is out of scope.

- Corrected 2026-11-17: the two bullets above describe the tree of 2026-10-16. External plugin YANG is joined at `ze start` today, by the external-plugin spec, from text and with a whole-set resolve. This spec replaces the body of that join (Existing Mechanisms) and owns its cost (AC-18).

## Work Not Done (homed elsewhere)

| Item | Home |
|------|------|
| Compile and link external plugin YANG at startup, deliver config under the plugin's own root, correct `docs/plugin-development/schema.md` once it works | `plan/post-release/features/spec-plugin-external-yang-at-startup.md` (in-progress on 2026-11-17, built through its phase 8; it was a skeleton when this row was written) |

Taken from that spec on 2026-11-17: the cost of the second schema build a
declared external plugin adds to a start (its R-19 and Q10). Owned here, closed
by AC-18. The work that meets AC-18 also writes the start cost of an external
plugin, measured on the owner's Mac, into `docs/plugin-development/schema.md`,
which that spec's phase 9 left without a figure.

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
