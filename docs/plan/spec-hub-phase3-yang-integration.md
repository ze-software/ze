# Spec: Hub Phase 3 - YANG Integration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/hub-architecture.md` - Hub design, YANG role
4. `docs/architecture/config/yang-config-design.md` - YANG config design
5. `cmd/ze-config-reader/main.go` - Config Reader (Phase 2)

## Task

Integrate YANG validation into the Config Reader. The Config Reader will validate parsed config against the combined YANG schema before reporting it to the Hub.

### Goals

1. Integrate a YANG library (goyang or libyang)
2. Load and combine YANG modules from all plugins
3. Validate config data against YANG types and constraints
4. Validate leafref references across modules
5. Produce clear error messages with line numbers

### Non-Goals

- Semantic validation (Phase 4 - plugins do this)
- Writing YANG modules for ZeBGP (separate task)
- Config transformation (YANG validates, doesn't transform)

### Dependencies

- Phase 1: Schema Infrastructure (schemas collected)
- Phase 2: Config Reader (basic parsing working)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - [YANG and leafref section]
- [ ] `docs/architecture/config/yang-config-design.md` - [YANG integration approach]
- [ ] `docs/architecture/config/vyos-research.md` - [VyOS YANG usage]

### External Resources
- [ ] goyang library: `github.com/openconfig/goyang`
- [ ] YANG RFC 7950 key sections (types, leafref, must)

**Key insights:**
- goyang is pure Go, no cgo dependency
- libyang (C) is more complete but requires cgo
- Start with goyang for simplicity, can switch if needed
- leafref validation uses plugin CLI commands (e.g., `ze bgp schema validate peer-group name upstream`)
- Config Reader calls plugin schema commands to validate leafrefs

## Design

### YANG Library Choice

**Option A: goyang (recommended for start)**
- Pure Go, easy to integrate
- Parses YANG, provides schema tree
- May need custom validation code for some features

**Option B: libyang**
- Complete YANG implementation
- Requires cgo
- More complex build/deployment

Start with goyang, evaluate if sufficient.

### YANG Module Loading

The Loader handles YANG module loading and resolution:

**Operations:**
- `AddModule(yangText)` - Add a YANG module from text
- `Resolve()` - Resolve imports/includes across all modules
- `Schema()` - Return combined schema tree

### Validation

The Validator checks config data against YANG schema:

**Operations:**
- `Validate(path, data)` - Validate data at path, return errors

**Error types:**
| Type | Description |
|------|-------------|
| TYPE_ERROR | Value doesn't match YANG type |
| RANGE_ERROR | Numeric value outside allowed range |
| PATTERN_ERROR | String doesn't match pattern |
| LEAFREF_MISSING | Referenced target doesn't exist |

### Config Reader Integration

Config Reader workflow:

1. **Init phase**: Load all YANG modules from schema commands
2. **Resolve**: Resolve YANG imports/includes
3. **Parse config**: Use existing tokenizer
4. **Validate blocks**: Check each config block against YANG schema
5. **Report errors**: Return validation errors with path and message

### Error Messages

```
config.conf:42: type error at bgp.peer[address=192.0.2.1].peer-as
  expected: uint32 (range 1..4294967295)
  got: "not-a-number"

config.conf:55: leafref error at bgp.peer[address=192.0.2.1].group
  reference: ../../peer-group/name
  target "nonexistent" not found in peer-group list
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoader_SingleModule` | `internal/yang/loader_test.go` | Load single YANG module | |
| `TestLoader_MultipleModules` | `internal/yang/loader_test.go` | Load and combine modules | |
| `TestLoader_ImportResolution` | `internal/yang/loader_test.go` | Resolve YANG imports | |
| `TestValidator_TypeString` | `internal/yang/validator_test.go` | Validate string type | |
| `TestValidator_TypeUint32` | `internal/yang/validator_test.go` | Validate uint32 type | |
| `TestValidator_TypeRange` | `internal/yang/validator_test.go` | Validate range constraint | |
| `TestValidator_TypePattern` | `internal/yang/validator_test.go` | Validate pattern constraint | |
| `TestValidator_TypeEnum` | `internal/yang/validator_test.go` | Validate enumeration | |
| `TestValidator_Leafref` | `internal/yang/validator_test.go` | Validate leafref reference | |
| `TestValidator_LeafrefMissing` | `internal/yang/validator_test.go` | Error on missing leafref target | |
| `TestValidator_Mandatory` | `internal/yang/validator_test.go` | Error on missing mandatory leaf | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| uint8 | 0-255 | 255 | N/A | 256 |
| uint16 | 0-65535 | 65535 | N/A | 65536 |
| uint32 | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| ASN range | 1-4294967295 | 4294967295 | 0 | 4294967296 |
| Port range | 1-65535 | 65535 | 0 | 65536 |
| String length | varies | max defined | N/A | max+1 |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `yang-type-valid` | `test/data/plugin/yang-type-valid.ci` | All types validate correctly | |
| `yang-type-invalid` | `test/data/plugin/yang-type-invalid.ci` | Type errors reported | |
| `yang-range-boundary` | `test/data/plugin/yang-range-boundary.ci` | Range boundaries checked | |
| `yang-leafref-valid` | `test/data/plugin/yang-leafref-valid.ci` | Valid leafref passes | |
| `yang-leafref-missing` | `test/data/plugin/yang-leafref-missing.ci` | Missing leafref target error | |

## Files to Create

- `internal/yang/loader.go` - YANG module loading
- `internal/yang/loader_test.go` - Loader tests
- `internal/yang/validator.go` - YANG validation
- `internal/yang/validator_test.go` - Validator tests
- `internal/yang/errors.go` - Validation error types
- `test/data/plugin/yang-*.ci` - Functional tests

**Note:** Tests use YANG modules from `yang/` directory (created by YANG Modules spec), not duplicates.

## Files to Modify

- `cmd/ze-config-reader/main.go` - Integrate YANG validation
- `cmd/ze-config-reader/parser.go` - Pass data to validator
- `go.mod` - Add goyang dependency

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Add goyang dependency** - `go get github.com/openconfig/goyang`
2. **Write loader tests** - Create loader_test.go
3. **Run tests** - Verify FAIL (paste output)
4. **Implement Loader** - Create loader.go
5. **Run tests** - Verify PASS
6. **Write validator tests** - Create validator_test.go
7. **Run tests** - Verify FAIL
8. **Implement Validator** - Create validator.go with type checks
9. **Run tests** - Verify partial PASS
10. **Add leafref validation** - Extend validator
11. **Run tests** - Verify PASS (paste output)
12. **Integrate with Config Reader** - Update ze-config-reader
13. **Create test YANG module** - ze-bgp.yang for testing
14. **Functional tests** - Create and run
15. **Verify all** - `make lint && make test && make functional` (paste output)

## Open Questions

| # | Question | Options |
|---|----------|---------|
| 1 | goyang vs libyang | Start goyang, switch if insufficient |
| 2 | Where to store YANG modules | Embedded in binary vs external files |
| 3 | YANG deviation support | Support for local modifications to standard modules |

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

### Documentation
- [ ] Required docs read
- [ ] Code comments added

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-hub-phase3-yang-integration.md`
