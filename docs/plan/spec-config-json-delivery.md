# Spec: config-json-delivery

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - plugin protocol stages
4. `internal/plugin/registration.go` - current declaration parsing
5. `internal/plugin/server.go` - current deliverConfig()
6. `internal/config/parser.go` - Tree.ToMap() method

## Task

Replace pattern-based config delivery with full JSON config delivery. Plugins declare which config roots they want, receive full JSON on startup (Stage 2) and on config reload.

**Key changes:**
1. Plugin declares config roots: `declare wants config bgp`
2. Stage 2 sends full config JSON filtered by declared roots
3. On config reload, plugins receive updated config (same format)
4. Shared diff library (VyOS-style) computes what changed
5. Remove all pattern-based delivery and hostname-specific extraction

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin protocol stages

### Source Code
- [ ] `internal/plugin/registration.go` - current WantsConfigJSON, ConfigPatterns
- [ ] `internal/plugin/server.go` - deliverConfig() implementation
- [ ] `internal/config/parser.go` - Tree.ToMap() method (lines 292-332)
- [ ] `internal/config/bgp.go` - hostname extraction to remove (lines 989-1001)
- [ ] `internal/plugin/bgp/reactor/reactor.go` - Reload() method

**Key insights:**
- `Tree.ToMap()` already exists for JSON serialization
- Parsed Tree is discarded after `TreeToConfig()` - need to store it
- No plugin notification exists for config reload

## Design

### Declaration Syntax

| Declaration | Meaning |
|-------------|---------|
| `declare wants config bgp` | Want bgp subtree |
| `declare wants config environment` | Want environment subtree |
| `declare wants config *` | Want entire config tree |

Multiple roots can be declared. Plugin must opt-in (no config by default).

### Config Delivery Protocol (Stage 2)

| Step | Message |
|------|---------|
| 1 | `config json bgp {"router-id":"1.2.3.4","peer":{"192.168.1.1":{...}}}` |
| 2 | `config json environment {"log":{...}}` |
| 3 | `config done` |

Format: `config json <root> <json>` for each declared root.

### Config Reload Protocol (Runtime)

| Step | Message |
|------|---------|
| 1 | `config reload json bgp {...new config...}` |
| 2 | `config reload done` |

Same structure, `reload` keyword distinguishes from initial delivery.

### Diff Mechanism (VyOS-style)

Shared library for computing config differences:

| Type | Purpose |
|------|---------|
| `ConfigDiff` | Holds Added/Removed/Changed maps |
| `DiffPair` | Old and New values for changed keys |
| `DiffMaps(old, new)` | Computes deep diff between two `map[string]any` |

Plugins can use this library to determine what changed on reload.

### Data Flow

```
Config File → Parser.Parse() → Tree
                                 ↓
                    Store in BGPConfig.ParsedTree
                                 ↓
                    TreeToConfig() → BGPConfig
                                 ↓
                    Server.deliverConfig()
                                 ↓
              For each plugin with WantsConfigRoots:
                tree.GetContainer(root).ToMap() → JSON
                send "config json <root> <json>"
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiffMapsEmpty` | `internal/config/diff_test.go` | Same maps → empty diff | |
| `TestDiffMapsAdded` | `internal/config/diff_test.go` | Detects new keys | |
| `TestDiffMapsRemoved` | `internal/config/diff_test.go` | Detects deleted keys | |
| `TestDiffMapsChanged` | `internal/config/diff_test.go` | Detects changed values | |
| `TestDiffMapsNested` | `internal/config/diff_test.go` | Deep comparison | |
| `TestParseWantsConfigRoot` | `internal/plugin/registration_test.go` | Parse single root | |
| `TestParseWantsConfigMultiple` | `internal/plugin/registration_test.go` | Parse multiple roots | |
| `TestDeliverConfigByRoot` | `internal/plugin/server_test.go` | Correct JSON per root | |

### Boundary Tests

N/A - no numeric inputs in this feature.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `hostname` | `test/encode/hostname.ci` | Plugin declares `wants config bgp`, receives JSON, responds with hostname capability | |

## Files to Modify

- `internal/plugin/registration.go` - Add `WantsConfigRoots []string`, parse new declaration
- `internal/plugin/server.go` - Rewrite `deliverConfig()`, add `notifyConfigReload()`
- `internal/plugin/types.go` - Remove pattern types, simplify `PeerCapabilityConfig`
- `internal/config/bgp.go` - Add `ParsedTree`, remove hostname extraction (989-1001), remove `RawCapabilityConfig`
- `internal/config/loader.go` - Store parsed tree in BGPConfig
- `internal/plugin/bgp/reactor/reactor.go` - Call notification after Reload()
- `internal/plugin/bgp/reactor/peersettings.go` - Remove `CapabilityConfigJSON`
- `internal/plugin/hostname/hostname.go` - Update to use `declare wants config bgp`

## Files to Create

- `internal/config/diff.go` - VyOS-style diff implementation
- `internal/config/diff_test.go` - Diff tests

## Code to Remove

| Location | What |
|----------|------|
| `bgp.go:989-1001` | Hostname-specific extraction code |
| `bgp.go` PeerConfig | `RawCapabilityConfig` field |
| `bgp.go` PeerConfig | `CapabilityConfigJSON` field |
| `peersettings.go` | `CapabilityConfigJSON` field |
| `registration.go` | `ConfigPatterns`, `ConfigPattern`, pattern parsing |
| `registration.go` | `CompileConfigPattern()`, pattern matching |
| `server.go` | `matchConfigPattern()` |
| `types.go` | `ConfigPattern`, `ConfigMatch` types |
| `types.go` | `PeerCapabilityConfig.Values` field |

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Diff Library

1. **Create diff_test.go** - Write tests BEFORE implementation
   → **Review:** Are edge cases covered? Empty maps, nil values, nested structures?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason?

3. **Create diff.go** - `DiffMaps()` function with deep comparison
   → **Review:** Is this the simplest solution? Any code duplication?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL tests pass?

### Phase 2: Declaration and Storage

5. **Write registration tests** - For `declare wants config <root>` parsing
   → **Review:** Multiple roots tested? Invalid syntax tested?

6. **Run tests** - Verify FAIL (paste output)

7. **Add WantsConfigRoots** - To `PluginRegistration` struct

8. **Parse declaration** - `declare wants config <root>` handling
   → **Review:** Error messages clear for invalid syntax?

9. **Add ParsedTree** - To `BGPConfig` struct

10. **Store tree** - In `LoadReactorWithConfig()` after parsing

11. **Run tests** - Verify PASS (paste output)

### Phase 3: Config Delivery Rewrite

12. **Write delivery tests** - For JSON delivery per root
   → **Review:** Tests cover missing root? Empty config?

13. **Run tests** - Verify FAIL (paste output)

14. **Rewrite deliverConfig()** - Check `WantsConfigRoots`, send JSON per root

15. **Store configTree** - In Server for reload comparison

16. **Run tests** - Verify PASS (paste output)

### Phase 4: Reload Notification

17. **Add NotifyConfigReload()** - To Server

18. **Hook into Reload()** - Call notification after success

19. **Run functional test** - Verify config delivery (paste output)

### Phase 5: Cleanup

20. **Remove pattern code** - Types, functions, matching logic
    → **Review:** Any orphaned code remaining?

21. **Remove hostname extraction** - Lines 989-1001 in bgp.go

22. **Remove obsolete fields** - `RawCapabilityConfig`, old `CapabilityConfigJSON`

23. **Update hostname plugin** - Use new declaration syntax

24. **Verify all** - `make verify` (paste output)
    → **Review:** Zero lint issues? All tests pass?

25. **Final self-review** - Before claiming done:
    - Re-read all code changes: any bugs, edge cases, or improvements?
    - Check for unused code, debug statements, TODOs
    - Verify error messages are clear and actionable

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
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
- [ ] Boundary tests cover all numeric inputs (N/A for this spec)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes (26 linters)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (N/A - not protocol work)
- [ ] Code comments added

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## RFC Documentation

N/A - this is infrastructure work, not BGP protocol implementation.

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Design Insights
- [Key learnings that should be documented elsewhere]

### Deviations from Plan
- [Any differences from original plan and why]
