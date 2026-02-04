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
- NEW: `internal/yang/parse.go:ParseYANGMetadata()` - extract module, namespace, imports from YANG content

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
- `internal/yang/parse.go` - YANG metadata parser (module, namespace, imports)
- `internal/yang/parse_test.go` - Unit tests for YANG parser
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

4. **Add YANG parsing utilities** - Extract metadata from YANG content:
   - Add `ParseYANGMetadata(yang string)` function that extracts:
     - Module name (from `module <name> {`)
     - Namespace (from `namespace "<uri>";`)
     - Imports (from `import <module> {` statements)
   - For internal plugins: parse their embedded YANG to get metadata
   - For external plugins: run `--yang`, parse output
   - Add `ResolveDependencies()` to auto-load based on imports
   → **Review:** Does parser handle all YANG variations? Error handling for malformed YANG?

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

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Design Insights
- [To be filled]

### Deviations from Plan
- [To be filled]

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (N/A)
- [ ] Feature code integrated into codebase (`cmd/ze/schema/`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (N/A - not protocol work)
- [ ] RFC references added to code (N/A)
- [ ] RFC constraint comments added (N/A)

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
