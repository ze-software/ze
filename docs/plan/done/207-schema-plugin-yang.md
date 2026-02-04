# Spec: schema-plugin-yang

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/schema/main.go` - schema command implementation
4. `cmd/ze/main.go` - main entry point with --plugin parsing
5. `internal/plugin/inprocess.go` - internal plugin YANG functions

## Task

Make `ze schema` commands work with plugins to display their YANG schemas:
- `ze schema list` auto-discovers internal plugins with YANG
- `ze schema show <module>` outputs actual YANG content
- `--plugin` flag allows specifying external plugins (executes them with `--yang` to get schema)
- Plugins that `declare wants config <root>` are listed as augmenting that root schema

### Expected Output

```
ze schema list

Module           Namespace      Imports
ze-bgp           ze.bgp         -
ze-gr            ze.gr          ze.bgp
ze-hostname      ze.hostname    ze.bgp
```

**Namespace is the plugin identity.** No separate "plugin name" needed - namespaces are guaranteed unique by YANG.

### Display Formatting

| YANG Namespace | Displayed |
|----------------|-----------|
| `urn:ze:bgp` | `ze.bgp` |
| `urn:ze:gr` | `ze.gr` |

- Drop `urn:` prefix for display
- Replace `:` with `.` for display

### Plugin Loading (--plugin syntax)

Three loading modes based on syntax (from spec 198-plugin-invocation):

| Syntax | Mode | Execution | Cost |
|--------|------|-----------|------|
| `ze.name` | **Internal** | Goroutine + io.Pipe | Medium |
| `ze-name` | **Direct** | Synchronous function call | Low |
| `name` or `/path` | **Fork** | Subprocess (exec) | High |

**Examples:**
- `--plugin ze.bgp` → Internal (goroutine, same process)
- `--plugin ze-bgp` → Direct (sync call, no goroutine)
- `--plugin bgp` or `--plugin /usr/bin/plugin` → Fork (subprocess)

**This is not cosmetic** - the prefix determines the execution model.

### Dependency Resolution

Dependencies are expressed via YANG `import` statements:

```yang
module ze-gr {
    namespace "urn:ze:gr";

    import ze-bgp {
        prefix bgp;
    }
}
```

The `import ze-bgp` references namespace `urn:ze:bgp`, establishing the dependency.

When a plugin imports another module:
1. **Imported namespace is internal** → Auto-load it
2. **Imported namespace not available** → Refuse to start with clear error

This applies to:
- `ze schema` commands (auto-load dependencies when displaying schemas)
- Runtime plugin loading (auto-load dependencies when starting daemon)

Example error when dependency missing:
```
error: plugin ze.gr imports ze.bgp but plugin not available
```

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` - Plugin architecture and YANG exposure
- [ ] `.claude/rules/cli-patterns.md` - CLI command patterns

**Key insights:**
- Plugins expose YANG via `GetYANG()` function in PluginConfig
- `RunPlugin()` in plugin_common.go handles `--yang` flag automatically
- Internal plugins registered in `internalPluginYANG` map in inprocess.go
- Plugins declare `declare wants config <root>` during startup (e.g., gr.go:73, hostname.go:105)
- For schema discovery, we need static metadata about which roots each plugin wants

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/schema/main.go` - Uses demo registry with hardcoded schemas, no actual YANG content
- [ ] `cmd/ze/main.go` - Parses `--plugin <name>` flags but only passes to hub.Run()
- [ ] `internal/plugin/inprocess.go` - Has `internalPluginYANG` map with helper functions
- [ ] `internal/plugin/bgp/schema/embed.go` - Has `ZeBGPYANG` embedded string

**Behavior to preserve:**
- `ze bgp plugin <name> --yang` continues to work for individual plugins
- `ze schema list` format with module, namespace, plugin columns
- `ze schema handlers` shows handler → module mapping
- `ze schema protocol` shows protocol documentation

**Behavior to change (user requested):**
- `ze schema show ze-bgp` outputs actual YANG (currently says "no YANG content available")
- `ze schema show ze-gr` works for internal plugins
- `ze schema list` shows all internal plugins with YANG
- `--plugin` flag works with `ze schema` to load external plugin YANG

## Data Flow (MANDATORY)

### Entry Point
- CLI command: `ze [--plugin X]... schema <subcommand> [args]`
- `--plugin` can be: `ze.name` (internal) or `/path/to/binary` (external)

### Transformation Path
1. `cmd/ze/main.go` parses `--plugin` flags into slice
2. `schema.Run()` receives plugins slice as parameter
3. `getSchemaRegistry()` builds registry with dependency resolution:
   - Load ze-bgp from embedded `schema.ZeBGPYANG` (provides "bgp" root)
   - Load internal plugins from `internalPluginYANG` map
   - For each plugin, check its `wants config` dependencies:
     - If dependency is internal → auto-load it
     - If dependency not available → return error
   - For external plugins: execute `<binary> --yang` and capture output
4. Registry lookup by module name
5. Output YANG content to stdout

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI → Schema | Function parameter (plugins slice) | [ ] |
| Schema → External Plugin | Execute process with --yang flag | [ ] |

### Integration Points
- `internal/plugin/inprocess.go:GetInternalPluginYANG()` - get YANG for internal plugin
- `internal/plugin/inprocess.go:internalPluginYANG` - map of internal plugins with YANG
- `internal/plugin/bgp/schema.ZeBGPYANG` - embedded BGP YANG
- `internal/yang/loader.go:Loader` - existing goyang-based YANG parser
- `github.com/openconfig/goyang/pkg/yang` - provides `Module.Namespace.Name` and `Module.Import`
- NEW: `internal/yang/metadata.go` - `Metadata` struct and extraction functions

### Architectural Verification
- [ ] No bypassed layers (uses existing plugin infrastructure)
- [ ] No unintended coupling (schema cmd uses plugin package functions)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (N/A - CLI tool)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseYANGMetadata` | `internal/yang/parse_test.go` | extracts module, namespace, imports from YANG | |
| `TestParseYANGImports` | `internal/yang/parse_test.go` | handles multiple import statements | |
| `TestSchemaShowZeBGP` | `cmd/ze/schema/main_test.go` | show ze-bgp outputs YANG content | |
| `TestSchemaShowInternalPlugin` | `cmd/ze/schema/main_test.go` | show ze-gr outputs GR YANG | |
| `TestSchemaListIncludesPlugins` | `cmd/ze/schema/main_test.go` | list includes internal plugin modules | |
| `TestDependencyAutoLoad` | `cmd/ze/schema/main_test.go` | imports auto-load internal dependencies | |
| `TestDependencyMissing` | `cmd/ze/schema/main_test.go` | error when imported namespace not available | |
| `TestGetExternalPluginYANG` | `cmd/ze/schema/main_test.go` | executes external binary with --yang | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `schema-show-bgp.ci` | `test/schema/` | `ze schema show ze-bgp` outputs valid YANG | |
| `schema-list.ci` | `test/schema/` | `ze schema list` includes ze-bgp, ze-gr, ze-hostname | |

### Future (if deferring any tests)
- External plugin YANG test deferred until external plugin exists

## Files to Modify
- `cmd/ze/main.go` - Pass plugins slice to schema.Run()
- `cmd/ze/schema/main.go` - Accept plugins, build real registry, external plugin execution

## Files to Create
- `internal/yang/metadata.go` - YANG metadata extraction (uses goyang, no custom parser)
- `internal/yang/metadata_test.go` - Unit tests for metadata extraction
- `cmd/ze/schema/main_test.go` - Unit tests for schema commands
- `test/schema/schema-show-bgp.ci` - Functional test for show
- `test/schema/schema-list.ci` - Functional test for list

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** - Create tests for schema show/list with real YANG
   → **Review:** Are edge cases covered? Missing module error tested?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason? Not syntax errors?

3. **Modify schema.Run() signature** - Accept plugins slice parameter
   → **Review:** Is this the simplest change? Does it follow existing patterns?

4. **Add YANG metadata extraction** - Use existing goyang library (no custom parser needed):
   - Add `internal/yang/metadata.go` with `Metadata` struct (Module, Namespace, Imports)
   - Add `ExtractMetadata(mod *yang.Module)` - extracts from goyang's parsed module
   - Add `ParseYANGMetadata(content string)` - convenience wrapper
   - Add `FormatNamespace(ns string)` - converts `urn:ze:bgp` → `ze.bgp`
   - goyang already provides `mod.Namespace.Name` and `mod.Import` fields
   - For internal plugins: use existing `Loader` to parse embedded YANG
   - For external plugins: run `--yang`, parse output with same loader
   - Add `ResolveDependencies()` to auto-load based on imports
   → **Review:** Is goyang import used correctly? Error handling for invalid YANG?

5. **Build real registry in getSchemaRegistry()** - Replace demo data:
   - Import `schema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"`
   - Register ze-bgp with `schema.ZeBGPYANG`
   - Import `plugin "codeberg.org/thomas-mangin/ze/internal/plugin"`
   - Use `plugin.GetInternalPluginInfo()` to register internal plugins with metadata
   - For external plugins: execute `<path> --yang` to get YANG
   → **Review:** Any code duplication? Error handling complete?

6. **Update cmd/ze/main.go** - Pass plugins to schema.Run(args[1:], plugins)
   → **Review:** Consistent with other command dispatching?

7. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL tests pass? Any flaky behavior?

8. **Functional tests** - Create .ci files for end-user verification
   → **Review:** Do tests cover user-visible behavior?

9. **Verify all** - `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests deterministic?

10. **Final self-review** - Check for unused code, debug statements, clear error messages

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Auto-discover internal plugins | Simpler UX - no flags needed for common case |
| Execute external plugins with --yang | Can't import external code; standard plugin interface |
| Short module names (ze-gr, ze-hostname) | Consistent with existing internal plugin naming |
| Three loading modes | `ze.X` = Internal (goroutine), `ze-X` = Direct (sync), `name`/path = Fork (subprocess) |
| All plugins use ze.X naming | Consistent architecture - ze.bgp, ze.gr, ze.hostname (no "core") |
| Root column shows config dependency | Plugins declaring `wants config bgp` shown with bgp root |
| Namespace = identity | Namespace is the unique plugin identifier (no separate "name" needed) |
| YANG import = dependency | `import ze-bgp` means "depends on urn:ze:bgp" |
| Auto-load internal dependencies | If imported namespace is internal, load it automatically |
| Fail-fast on missing dependencies | If imported namespace not available, refuse to start with clear error |
| Parse YANG for metadata | All identity/dependency info comes from YANG (module, namespace, import) |

## Implementation Summary

### What Was Implemented
- `internal/yang/metadata.go` - YANG metadata extraction using goyang library
  - `Metadata` struct (Module, Namespace, Imports)
  - `ParseYANGMetadata()` - parse YANG content and extract metadata
  - `ExtractMetadata()` - extract from goyang Module object
  - `FormatNamespace()` - convert `urn:ze:bgp` to `ze.bgp` for display
- `cmd/ze/schema/main.go` - Full schema registry with dependency resolution
  - `Run(args, plugins)` - accepts plugins slice from main
  - `buildSchemaRegistryWithDeps()` - builds registry with auto-load
  - `registerYANGWithDeps()` - registers YANG and validates imports
  - `tryAutoLoadInternal()` - auto-loads internal dependencies
  - `getExternalPluginYANG()` - executes external plugins with `--yang`
- `cmd/ze/main.go` - passes plugins to `schema.Run(args, plugins)`
- Unit tests: `internal/yang/metadata_test.go`, `cmd/ze/schema/main_test.go`
- Functional tests: `test/parse/cli-schema-list.ci`, `test/parse/cli-schema-show.ci`

### Bugs Found/Fixed
- None - implementation was straightforward following TDD

### Design Insights
- goyang's `Module.Import` is `[]*Import` (slice), not a map - each Import has `.Name`
- The existing `plugin.GetAllInternalPluginYANG()` returns map with keys like "ze-hostname.yang"
- Namespace formatting is purely display - storage still uses raw URN format
- Dependency auto-load uses `tryAutoLoadInternal()` to recursively load internal modules
- External plugins are queried via `exec.CommandContext` with 10s timeout

### Deviations from Plan
- Used existing functional test location (`test/parse/`) instead of creating new `test/schema/`
- External plugin test uses `go run ..` to avoid needing `ze` binary in PATH during tests

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `ze schema list` auto-discovers internal plugins | ✅ Done | `cmd/ze/schema/main.go:186-195` | Uses GetAllInternalPluginYANG() |
| `ze schema show <module>` outputs actual YANG | ✅ Done | `cmd/ze/schema/main.go:116` | Outputs Yang field from registry |
| `--plugin` flag for external plugins | ✅ Done | `cmd/ze/schema/main.go:197-206` | Executes with --yang via getExternalPluginYANG() |
| Display formatting (urn:ze:X → ze.X) | ✅ Done | `internal/yang/metadata.go:72-80` | FormatNamespace() |
| Dependency resolution via YANG imports | ✅ Done | `cmd/ze/schema/main.go:264-292` | registerYANGWithDeps() + tryAutoLoadInternal() |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseYANGMetadata | ✅ Done | `internal/yang/metadata_test.go:15` | Named TestExtractMetadata |
| TestParseYANGImports | ✅ Done | `internal/yang/metadata_test.go:35-52` | Multi-import test case |
| TestSchemaShowZeBGP | ✅ Done | `cmd/ze/schema/main_test.go:139` | Named TestSchemaShowZeBGPContent |
| TestSchemaShowInternalPlugin | ✅ Done | `cmd/ze/schema/main_test.go:166` | Named TestSchemaRegistryIncludesPlugins |
| TestSchemaListIncludesPlugins | ✅ Done | `cmd/ze/schema/main_test.go:166` | Combined with above |
| TestDependencyAutoLoad | ✅ Done | `cmd/ze/schema/main_test.go:224` | Verifies ze-bgp auto-loaded via import |
| TestDependencyMissing | ✅ Done | `cmd/ze/schema/main_test.go:253` | Verifies error for unknown imports |
| TestGetExternalPluginYANG | ✅ Done | `cmd/ze/schema/main_test.go:283` | Uses `go run` to test external execution |
| TestRunWithPlugins | ✅ Done | `cmd/ze/schema/main_test.go:305` | Verifies Run() accepts plugins param |
| schema-show-bgp.ci | ✅ Done | `test/parse/cli-schema-show.ci` | |
| schema-list.ci | ✅ Done | `test/parse/cli-schema-list.ci` | Updated existing test |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/main.go` | ✅ Modified | Passes plugins to schema.Run() |
| `cmd/ze/schema/main.go` | ✅ Modified | Full dependency resolution + external plugin support |
| `internal/yang/metadata.go` | ✅ Created | |
| `internal/yang/metadata_test.go` | ✅ Created | |
| `cmd/ze/schema/main_test.go` | ✅ Modified | Added 8 new tests |
| `test/schema/schema-show-bgp.ci` | 🔄 Changed | → `test/parse/cli-schema-show.ci` |
| `test/schema/schema-list.ci` | 🔄 Changed | → `test/parse/cli-schema-list.ci` |

### Audit Summary
- **Total items:** 19
- **Done:** 17
- **Changed:** 2 (test file locations)

## Checklist

### 🏗️ Design
- [x] No premature abstraction (3+ concrete use cases exist?)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (each component does ONE thing?)
- [x] Explicit behavior (no hidden magic or conventions?)
- [x] Minimal coupling (components isolated, dependencies minimal?)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified - metadata functions undefined, schema tests showed empty YANG)
- [x] Implementation complete
- [x] Tests PASS (verified - all 12 new tests pass)
- [x] Boundary tests cover all numeric inputs (N/A - no numeric inputs)
- [x] Feature code integrated into codebase (`cmd/ze/schema/`, `internal/yang/`)
- [x] Functional tests verify end-user behavior (`cli-schema-list.ci`, `cli-schema-show.ci`)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes
- [x] `make verify` passes (full verification)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A - not protocol work)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)

### Completion (after tests pass - see Completion Checklist)
- [ ] Architecture docs updated with learnings
- [x] Implementation Audit completed (all items have status + location)
- [x] All Deferred items have user approval (none deferred - all in progress)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
