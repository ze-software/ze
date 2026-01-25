# Spec: hub-phase3-schema-routing

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `internal/plugin/schema.go` - existing SchemaRegistry

## Task

Integrate SchemaRegistry with hub for config routing (VyOS-inspired):
1. Plugins declare YANG + handlers + priority in Stage 1
2. Hub registers schemas with priority ordering
3. Hub validates config against YANG
4. Hub notifies plugins to verify/apply (by priority, lower first)
5. Plugins query live/edit config, compute diff, validate/apply

**Scope:** Schema registration, YANG validation, verify/apply protocol, config query. Uses existing SchemaRegistry.

**Depends on:** Phase 2 complete

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - schema registration details

### Source Files
- [ ] `internal/plugin/schema.go` - existing SchemaRegistry
- [ ] `internal/plugin/subsystem.go` - Stage 1 parsing
- [ ] `internal/yang/loader.go` - YANG module loading
- [ ] `internal/yang/validator.go` - YANG validation

**Key insights:**
- SchemaRegistry already exists with Register(), FindHandler()
- Stage 1 already parses `declare schema` messages
- YANG loader/validator already exist
- Need to wire these together in hub
- Add `declare priority` parsing for config ordering

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHubCollectsSchemas` | `internal/hub/schema_test.go` | Hub collects schemas from Stage 1 | |
| `TestHubValidatesConfig` | `internal/hub/schema_test.go` | Config validated against YANG | |
| `TestHubRoutesConfigByHandler` | `internal/hub/schema_test.go` | FindHandler routes to correct plugin | |
| `TestHubDeliversJSON` | `internal/hub/schema_test.go` | Config delivered as JSON | |
| `TestHubSubRootHandler` | `internal/hub/schema_test.go` | Sub-root handler gets subtree only | |
| `TestHubQueryConfigLive` | `internal/hub/schema_test.go` | Plugin queries live config | |
| `TestHubQueryConfigEdit` | `internal/hub/schema_test.go` | Plugin queries edit config | |
| `TestHubQueryConfigPath` | `internal/hub/schema_test.go` | Plugin queries specific path | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - YANG validation handles ranges | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Deferred to Phase 5 | | | |

## Files to Modify

- `internal/hub/hub.go` - Add schema collection, config routing

## Files to Create

- `internal/hub/schema.go` - Schema handling, JSON conversion
- `internal/hub/config.go` - Live/edit config storage, query handling
- `internal/hub/schema_test.go` - Unit tests
- `internal/config/diff/diff.go` - Shared diff library for plugins

## Implementation Steps

1. **Write unit tests** - Test schema collection and routing

   → **Review:** Test sub-root handlers (like GR)?

2. **Run tests** - Verify FAIL (paste output)

3. **Integrate SchemaRegistry** - Hub uses existing registry

   **Collect schemas behavior:**
   1. For each forked process, read Stage 1 declarations
   2. Register schemas in SchemaRegistry

   → **Review:** Reuses existing parsing code?

4. **Add YANG validation** - Validate config before routing

   **Validate config behavior:**
   1. Load all YANG modules declared by plugins
   2. Validate stored config against combined schema

   → **Review:** Uses existing yang.Validator?

5. **Add JSON conversion** - Convert config blocks to JSON for delivery

6. **Add config delivery** - Route JSON to plugins in Stage 2

7. **Add config query** - Allow plugins to query live/edit config
   ```
   # Plugin sends:
   #1 query config live path "bgp.peer[address=192.0.2.1]"

   # Hub responds:
   @1 done data '{"address": "192.0.2.1", "peer-as": 65002, ...}'
   ```

   → **Review:** Query command handler in hub?

8. **Run tests** - Verify PASS (paste output)

9. **Verify** - `make lint && make test && make functional`

## Design Decisions

### Root vs sub-root handlers

| Handler | Plugin receives |
|---------|-----------------|
| `bgp` (root) | Entire `bgp { }` as JSON |
| `bgp.peer.capability.graceful-restart` | Just that subtree |

Hub uses FindHandler() longest-prefix match to determine routing.

### Live/Edit Configuration Model (VyOS-style)

Hub maintains two config states:

| State | Purpose |
|-------|---------|
| **Live** | Running configuration (what plugins are using) |
| **Edit** | Candidate configuration (being modified) |

**Pull model (hub never pushes config):**
| Trigger | Hub Action |
|---------|------------|
| Startup | Notifies: `config verify` then `config apply` |
| SIGHUP | Notifies: `config verify` then `config apply` |
| CLI commit | Notifies: `config verify` then `config apply` |

Hub notifies plugins, plugins query hub for config data. Hub never sends config unprompted.

**Query protocol (text with #serial):**
```
#1 query config live path "bgp.peer"
@1 done data '[{"address": "192.0.2.1", "peer-as": 65002}, ...]'
```

**Verify/Apply workflow (triggered by commit):**
1. User edits candidate config (or hub loads from file)
2. User requests commit (startup, SIGHUP, or CLI)
3. For each plugin (by priority, lower first):
   - Hub sends: `#N config verify`
   - Plugin queries: `#M query config live path "..."`
   - Plugin queries: `#M query config edit path "..."`
   - Plugin computes diff, validates
   - Plugin responds: `@N done` or `@N error <reason>`
4. If all verify pass:
   - For each plugin (by priority):
     - Hub sends: `#N config apply`
     - Plugin applies changes
     - Plugin responds: `@N done`
   - Hub: edit becomes live
5. If any verify fails: reject, edit unchanged

**Priority ordering:**
| Priority | Plugin | Reason |
|----------|--------|--------|
| 100 | BGP | Core protocol |
| 200 | RIB | Depends on BGP |
| 300 | GR | Augments BGP |

**Diff responsibility:** Hub serves raw config. Plugins compute diff using shared library code (`internal/config/diff/`).

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [List actual changes]

### Bugs Found/Fixed
- [Any bugs discovered]

### Deviations from Plan
- [Any differences and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
