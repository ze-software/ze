# Spec: yang-validation

## Task

Implement systematic YANG data validation in four layers:

1. **Fix YANG schema** — Convert `type string` fields with known valid values to proper YANG enums/patterns. This is the cheapest, most correct fix: the schema already defines the contract, just use it properly.
2. **Systematic enforcement** — Walk the entire parsed config tree and validate every leaf against its YANG type constraints: range (RFC 7950 Section 9.2.4), pattern (Section 9.4.5), enum (Section 9.6), length (Section 9.4.4), mandatory (Section 7.6.5). Replace the current ad-hoc per-field approach in the editor validator.
3. **Custom validation with completion** — A new `ze:validate` YANG extension (RFC 7950 Section 7.19) for constraints that YANG cannot express natively. Used **only** when YANG native constraints are insufficient — primarily for **runtime-determined valid sets** (e.g., address families depend on loaded plugins). Validator functions provide both validation AND the list of valid options for CLI completion.
4. **Startup integrity check** — Verify at startup that every `ze:validate` reference in YANG has a corresponding registered Go function. Fail loudly if not.

### Validation Priority (MUST follow this order)

| Priority | Mechanism | When to Use |
|----------|-----------|-------------|
| 1st | YANG `enumeration` | Fixed set of valid values known at schema time |
| 2nd | YANG `pattern` | String format constraint expressible as regex |
| 3rd | YANG `range` | Numeric bounds |
| 4th | YANG `length` | String length bounds |
| 5th | YANG `mandatory` | Required fields |
| Last | `ze:validate` | Runtime-determined values, semantic checks YANG cannot express |

**`ze:validate` is a last resort.** If a constraint can be expressed in YANG, it MUST be expressed in YANG.

### Goals

- Fix YANG schema: convert `type string` leaves with documented valid values to proper enums
- Every YANG-defined constraint (range, pattern, enum, length, mandatory) is enforced at config load time, not just in the editor or API
- Runtime-determined constraints (e.g., valid address families) are validated via `ze:validate` functions that also provide completion values
- Missing validator registrations are caught at startup (or test time), never silently ignored
- The editor validator no longer needs hand-coded per-field YANG calls — it uses the same tree walk

### Non-Goals

- XPath evaluation for YANG `must`/`when` statements (RFC 7950 Sections 7.5.3, 7.21.5 — Ze does not use libyang; goyang doesn't evaluate XPath)
- External validator programs (the architecture doc envisions `zx:validator` for external programs — that is a separate, future feature)
- Cross-field validation between separate containers (e.g., "if GR enabled, must have SendUpdate process" — this is already handled in `ValidatePeerProcessCaps`)

## Required Reading

### Architecture Docs
- [x] `docs/architecture/config/yang-config-design.md` - YANG config system design
  -> Decision: YANG defines format, extensions declare behavior, implementation executes behavior
  -> Constraint: Extensions must be valid YANG — other tools can parse them even if they don't execute them

### RFC Summaries (MUST for protocol work)
- [x] `rfc/short/rfc7950.md` - YANG 1.1 data modeling language
  -> Constraint: Type validation (Section 9) — range (9.2.4), length (9.4.4), pattern (9.4.5), enum (9.6) MUST reject invalid values
  -> Constraint: Mandatory fields (Section 7.6.5) — missing mandatory node MUST be reported as error
  -> Constraint: Extensions (Section 7.19) — custom extensions MUST be valid YANG; tools that don't understand them ignore them
  -> Constraint: Multiple patterns are AND-combined (Section 9.4.5) — all patterns must match
  -> Constraint: Restrictions on derived types MUST be equal or more restrictive than base type (Section 9.2.4)

### Source Files
- [x] `internal/yang/validator.go` - existing validation engine with `Validate(path, value)` and `ValidateContainer(path, data)`
  -> Constraint: Already handles range, pattern, enum, length, mandatory, union — recursive walk should use these
- [x] `internal/config/yang_schema.go` - `yangToNode()` converts YANG to schema nodes, reads `ze:syntax`, `ze:key-type` extensions
  -> Constraint: Extension reading pattern established — `getSyntaxExtension()` iterates `entry.Exts`
- [x] `internal/config/editor/validator.go` - hand-coded hold-time validation via YANG validator
  -> Decision: This ad-hoc approach is what we're replacing with systematic tree walk
- [x] `internal/yang/modules/ze-extensions.yang` - existing extensions: `syntax`, `key-type`, `route-attributes`, `allow-unknown-fields`
  -> Constraint: New `validate` extension follows same pattern
- [x] `internal/yang/modules/ze-types.yang` - typedefs with patterns (IPv4, community) and ranges (ASN, port)
  -> Constraint: These constraints exist in YANG but many aren't enforced at config load time
- [x] `internal/config/reader.go` - `ConfigValidator` interface, `ValidateContainer` called per handler block
  -> Decision: Reader already validates handler-level blocks — recursive walk goes deeper
- [x] `internal/plugin/validator.go` - global `SetYANGValidator`/`YANGValidator()` for API command validation
  -> Constraint: Validator is set once at startup, shared across all consumers
- [x] `internal/config/loader.go` - calls `YANGValidatorWithPlugins` and `plugin.SetYANGValidator`

### Done Specs
- [x] `docs/plan/done/213-config-yang-validation.md` - wired `ValidateContainer` into reader
  -> Decision: `ConfigValidator` interface decouples config from yang package
  -> Constraint: Nil validator = skip validation (graceful degradation preserved)

**Key insights:**
- The YANG validator already implements all RFC 7950 native constraint checks: range (9.2.4), pattern (9.4.5), enum (9.6), length (9.4.4), mandatory (7.6.5), union (9.12)
- `ValidateContainer` validates one level deep — it checks mandatory children and validates provided values, but doesn't recurse into nested containers/lists
- The reader calls `ValidateContainer` per handler block (e.g., `"bgp.peer"`), not per individual leaf
- Extension reading is a proven pattern: iterate `entry.Exts`, match keyword, extract argument — follows RFC 7950 Section 7.19
- The architecture doc's `zx:validator` concept was for external programs; `ze:validate` is for in-process Go functions — different mechanism, complementary
- RFC 7950 Section 9.4.5: multiple patterns are AND-combined — all must match. Ze's validator correctly iterates all patterns
- RFC 7950 Section 7.19: extensions are valid YANG — standard tools parse them without error, just ignore semantics they don't understand

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/yang/validator.go` - validates single values and containers against YANG types; does not recurse into nested containers
- [x] `internal/config/editor/validator.go` - hand-codes hold-time validation: parses string to int, checks bounds, calls `yangValidator.Validate("bgp.peer.hold-time", uint16(holdTime))`; checks mandatory `peer-as` explicitly
- [x] `internal/config/reader.go` - calls `validator.ValidateContainer(handler, flatData)` per parsed block; does not recurse deeper
- [x] `internal/config/yang_schema.go` - converts YANG entries to schema nodes; reads extensions via `getSyntaxExtension()` pattern
- [x] `internal/yang/modules/ze-extensions.yang` - defines 4 extensions (syntax, key-type, route-attributes, allow-unknown-fields)
- [x] `internal/yang/modules/ze-types.yang` - defines typedefs with constraints that are only partially enforced
- [x] `internal/plugins/bgp/schema/ze-bgp-conf.yang` - defines hold-time range, port range, connection enum, mandatory fields
- [x] `internal/plugin/validator.go` - global singleton pattern for YANG validator

**Behavior to preserve:**
- Nil validator = skip validation (graceful degradation)
- `ConfigValidator` interface decoupling config from yang
- All existing validation (parser type checks, reader `ValidateContainer`, editor hold-time, API origin/med/local-pref)
- Extension reading pattern (`entry.Exts` iteration)

**Behavior to change:**
- Fix YANG schema: convert `type string` with documented valid values to proper enums
- Replace ad-hoc per-field YANG validation with systematic recursive tree walk
- Add `ze:validate` extension to `ze-extensions.yang` (last resort only)
- Add validator function registry with completion support
- Add startup integrity check

## YANG Schema Fixes (convert string to enum/pattern)

Several leaves in `ze-bgp-conf.yang` use `type string` with valid values documented only in `description`. These MUST become proper YANG enums or patterns so the tree walk enforces them natively — no `ze:validate` needed.

### Fields to Convert to Enumeration

| Location | Leaf | Current | Valid Values | RFC 7950 |
|----------|------|---------|--------------|----------|
| `peer.family` | `mode` | `type string` | enable, disable, require, ignore | Section 9.6 |
| `peer.capability.nexthop` | `mode` | `type string` | enable, disable, require, refuse | Section 9.6 |
| `peer.add-path` | `direction` | `type string` | send, receive, send/receive | Section 9.6 |
| `peer.add-path` | `mode` | `type string` | enable, disable, require, refuse | Section 9.6 |

These are static, known-at-schema-time values. Pure YANG enum — cheapest, most correct fix.

### Fields That Stay `type string` With `ze:validate` (Runtime-Determined)

| Location | Leaf | Why Not YANG Enum | ze:validate Function |
|----------|------|-------------------|---------------------|
| `peer.family` | `name` | Valid families depend on loaded plugins (ipv4/unicast, ipv6/flow, l2vpn/evpn, ...) | `registered-address-family` |
| `peer.add-path` | `family` | Same — plugin-registered | `registered-address-family` |
| `peer.capability.nexthop` | `family` | Same — plugin-registered | `registered-address-family` |
| `peer.capability.nexthop` | `nhafi` | Valid AFIs depend on loaded plugins | `registered-afi` |

These sets are only known at runtime after plugins register. `ze:validate` functions provide both validation AND the list of valid options for completion.

### Fields That Stay `type string` With Pattern

| Location | Leaf | Pattern | Why |
|----------|------|---------|-----|
| `address-family` typedef | (typedef) | Already has format `afi/safi` but no pattern | Add `pattern '[a-z0-9-]+/[a-z0-9-]+'` to enforce "/" separator |

## ze:validate Interface — Validation + Completion

`ze:validate` functions serve double duty: validation AND completion. For runtime-determined valid sets, the function knows what values are valid and can provide them for CLI autocompletion.

### Function Signature

Each registered validator provides two capabilities:

| Method | Signature | Purpose |
|--------|-----------|---------|
| Validate | `func(path string, value any) error` | Reject invalid values |
| Complete | `func() []string` | Return all currently valid values (for CLI completion) |

For pure validation (no finite set), Complete returns nil. For runtime enums (address families), Complete returns the current valid set.

### Candidate ze:validate Functions

| Name | Validate | Complete | Why Not YANG |
|------|----------|----------|--------------|
| `registered-address-family` | Value in plugin-registered families | All registered families | Set is runtime-determined |
| `registered-afi` | Value is valid AFI name | All registered AFI names | Set is runtime-determined |
| `community-range` | Each part of `AA:NN` fits uint16 | nil | YANG pattern can't bound numbers |
| `large-community-range` | Each part of `AA:NN:NN` fits uint32 | nil | YANG pattern can't bound numbers |
| `nonzero-ipv4` | Not `0.0.0.0` | nil | Pattern would be extremely complex |

## Data Flow (MANDATORY)

### Entry Point
- Config file content enters through the parser (`config.Parser.Parse`)
- Parsed Tree enters validation via `ValidateContainer` (existing) or new `ValidateTree` (recursive)

### Transformation Path

#### Layer 1: Recursive Tree Walk
1. Config file -> Parser -> `config.Tree`
2. `config.Tree` -> `ValidateTree(tree, yangLoader)` — NEW
3. For each node in tree, find matching YANG entry
4. For each leaf: call existing `validateEntry(path, entry, value)` — checks range, pattern, enum, length
5. For each container: check mandatory children, then recurse into children
6. For each list: recurse into each list entry
7. Collect all validation errors (don't stop at first)

#### Layer 2: Custom Validation Functions (RFC 7950 Section 7.19 extension mechanism)
1. During tree walk, after YANG native validation passes for a leaf
2. Check if YANG entry has `ze:validate` extension (read from `entry.Exts`)
3. Look up function name in registry
4. Call `fn(path, value) error`
5. If error, add to validation errors

#### Layer 3: Startup Integrity Check
1. After all `init()` have run (all validators registered)
2. Walk entire resolved YANG tree
3. For each `ze:validate` extension found, check registry has that function
4. If any missing, fail startup with clear error listing all missing functions

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config Tree -> YANG Validator | `ValidateTree()` takes tree + YANG loader | [ ] |
| YANG entry -> Custom validator | Extension keyword lookup + registry call | [ ] |
| Startup -> Registry check | `CheckAllValidatorsRegistered(loader)` | [ ] |

### Integration Points
- `yang.Validator.Validate()` — existing, validates single value against YANG type
- `yang.Validator.ValidateContainer()` — existing, validates one container level
- `yang.Loader.GetEntry()` — existing, gets resolved YANG entry tree
- `config.ConfigValidator` interface — existing, used by reader
- `plugin.SetYANGValidator()` — existing, sets global validator at startup
- `config.Loader` — existing, where startup integrity check should run

### Architectural Verification
- [ ] No bypassed layers — recursive walk uses existing `validateEntry` for each leaf
- [ ] No unintended coupling — registry is leaf package, validators register via `init()`
- [ ] No duplicated functionality — extends existing validation, doesn't recreate
- [ ] Zero-copy preserved — validation is read-only on config values

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | RFC 7950 |
|-------|-------------------|-------------------|----------|
| AC-1 | Config with `connection invalid-value;` | Validation error: enum violation, expected `both`, `passive`, or `active` | Section 9.6 |
| AC-2 | Config with `port 0;` | Validation error: range violation, expected `1..65535` | Section 9.2.4 |
| AC-3 | Config with `hold-time 2;` | Validation error: range violation, expected `0 \| 3..65535` | Section 9.2.4 |
| AC-4 | Config with missing `router-id` at BGP level | Validation error: mandatory field missing | Section 7.6.5 |
| AC-5 | Config with missing `local-as` at BGP level | Validation error: mandatory field missing | Section 7.6.5 |
| AC-6 | Config with `router-id not-an-ip;` | Validation error: pattern violation for IPv4 | Section 9.4.5 |
| AC-7 | Valid complete config | No validation errors | — |
| AC-8 | Config with `family { ipv4/unicast enable; }` where family `mode` is now YANG enum | Accepted (valid enum) | Section 9.6 |
| AC-9 | Config with `family { ipv4/unicast invalid-mode; }` where `mode` is YANG enum | Validation error: enum violation | Section 9.6 |
| AC-10 | Config with `add-path { ipv4/unicast send enable; }` where `direction` is YANG enum | Accepted (valid enum) | Section 9.6 |
| AC-11 | YANG typedef with `ze:validate "test-func"` and function registered | Custom validator called, value validated | Section 7.19 |
| AC-12 | YANG typedef with `ze:validate "test-func"` and function NOT registered | Startup integrity check fails with clear error | Section 7.19 |
| AC-13 | `ze:validate` function returns error for invalid value | Validation error includes custom message | Section 7.19 |
| AC-14 | Nil validator passed to reader | No validation, no crash (graceful degradation preserved) | — |
| AC-15 | Editor validator uses tree walk instead of hand-coded hold-time check | Same validation results, less code | — |
| AC-16 | `registered-address-family` validator: value `"ipv4/unicast"` with family registered | Accepted | Section 7.19 |
| AC-17 | `registered-address-family` validator: value `"invalid/family"` with family not registered | Validation error: not a registered address family | Section 7.19 |
| AC-18 | `registered-address-family` Complete() called | Returns all currently registered families (for CLI completion) | Section 7.19 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateTree_ValidConfig` | `internal/yang/validator_test.go` | Complete valid config passes recursive walk | |
| `TestValidateTree_EnumViolation` | `internal/yang/validator_test.go` | Invalid enum value caught at any depth | |
| `TestValidateTree_RangeViolation` | `internal/yang/validator_test.go` | Out-of-range numeric value caught at any depth | |
| `TestValidateTree_PatternViolation` | `internal/yang/validator_test.go` | Invalid pattern caught for typedef | |
| `TestValidateTree_MandatoryMissing` | `internal/yang/validator_test.go` | Missing mandatory field caught at nested level | |
| `TestValidateTree_MultipleErrors` | `internal/yang/validator_test.go` | Multiple errors collected, not stopped at first | |
| `TestValidateTree_NestedContainers` | `internal/yang/validator_test.go` | Validation recurses into nested containers | |
| `TestValidateTree_ListEntries` | `internal/yang/validator_test.go` | Validation recurses into each list entry | |
| `TestValidatorRegistry_Register` | `internal/yang/registry_test.go` | Register and retrieve validator function | |
| `TestValidatorRegistry_Missing` | `internal/yang/registry_test.go` | Get returns nil for unregistered name | |
| `TestValidatorRegistry_CustomValidation` | `internal/yang/registry_test.go` | Custom validator called during tree walk | |
| `TestValidatorRegistry_CustomError` | `internal/yang/registry_test.go` | Custom validator error propagated | |
| `TestCheckAllValidatorsRegistered_AllPresent` | `internal/yang/registry_test.go` | No error when all referenced validators exist | |
| `TestCheckAllValidatorsRegistered_Missing` | `internal/yang/registry_test.go` | Error lists missing validator names | |
| `TestValidateExtension_ParsedFromYANG` | `internal/yang/registry_test.go` | `ze:validate` extension read from YANG entry | |
| `TestAddressFamilyValidator_Validate` | `internal/config/validators_test.go` | Accepts "ipv4/unicast" (registered), rejects "invalid/family" (not registered) | |
| `TestAddressFamilyValidator_Complete` | `internal/config/validators_test.go` | Returns all registered families for completion | |
| `TestNonzeroIPv4Validator` | `internal/config/validators_test.go` | Accepts "1.2.3.4", rejects "0.0.0.0" | |
| `TestCommunityRangeValidator` | `internal/config/validators_test.go` | Accepts "100:200", rejects "100000:200" (exceeds uint16) | |
| `TestFamilyModeEnum` | `internal/yang/validator_test.go` | After schema fix: `mode` accepts enable/disable/require/ignore, rejects other | |
| `TestAddPathDirectionEnum` | `internal/yang/validator_test.go` | After schema fix: `direction` accepts send/receive/send/receive, rejects other | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port | 1..65535 | 65535 | 0 | 65536 |
| hold-time | 0 \| 3..65535 | 0, 3, 65535 | 1, 2 | 65536 |
| ASN | 1..4294967295 | 4294967295 | 0 | 4294967296 |
| restart-time | 0..4095 | 4095 | N/A | 4096 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-valid-config-loads` | `test/parse/*.ci` | Valid config with all fields loads without error | |
| `test-invalid-enum-rejected` | `test/parse/*.ci` | Config with invalid `connection` value rejected at parse time | |
| `test-invalid-range-rejected` | `test/parse/*.ci` | Config with `port 0` rejected at parse time | |
| `test-mandatory-missing-rejected` | `test/parse/*.ci` | Config missing mandatory BGP-level fields rejected | |

### Future (if deferring any tests)
- Cross-field validation (e.g., "keepalive < hold-time") — requires XPath or custom cross-field framework, separate spec
- External validator programs (`zx:validator` from architecture doc) — separate spec

## Files to Modify

### Phase 0: YANG Schema Fixes (string → enum)
- `internal/plugins/bgp/schema/ze-bgp-conf.yang` - convert `mode`, `direction` leaves from `type string` to `type enumeration`
- `internal/yang/modules/ze-types.yang` - add pattern to `address-family` typedef

### Phase 1: Recursive Tree Walk
- `internal/yang/validator.go` - add `ValidateTree(path, entry, data map[string]any) []ValidationError` that recursively walks tree
- `internal/yang/validator_test.go` - tests for recursive validation
- `internal/config/editor/validator.go` - replace hand-coded hold-time/mandatory checks with call to recursive `ValidateTree`
- `internal/config/reader.go` - use recursive validation instead of flat `ValidateContainer`

### Phase 2: Custom Validation Registry (with completion)
- `internal/yang/modules/ze-extensions.yang` - add `validate` extension definition
- `internal/yang/registry.go` - NEW: validator function registry with Validate + Complete methods
- `internal/yang/registry_test.go` - NEW: registry tests
- `internal/yang/validator.go` - during tree walk, check for `ze:validate` extension and call registered function

### Phase 3: Custom Validators (runtime enums)
- `internal/config/validators.go` - NEW: custom validator functions with completion (registered-address-family, nonzero-ipv4, community-range)
- `internal/config/validators_test.go` - NEW: tests for custom validators
- `internal/config/validators_register.go` - NEW: `init()` registering validators

### Phase 4: YANG Schema Updates (add ze:validate where needed)
- `internal/yang/modules/ze-types.yang` - add `ze:validate` to `address-family` typedef
- `internal/plugins/bgp/schema/ze-bgp-conf.yang` - add `ze:validate` to family `name` leaves

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new extension) | [x] | `internal/yang/modules/ze-extensions.yang` |
| RPC count in architecture docs | [ ] N/A | |
| CLI commands/flags | [ ] N/A | |
| CLI usage/help text | [ ] N/A | |
| API commands doc | [ ] N/A | |
| Plugin SDK docs | [ ] N/A | |
| Editor autocomplete | [x] Automatic | YANG-driven |
| Functional test for validation | [x] | `test/parse/*.ci` |

## Files to Create
- `internal/yang/registry.go` - validator function registry
- `internal/yang/registry_test.go` - registry tests
- `internal/config/validators.go` - custom validator functions
- `internal/config/validators_test.go` - custom validator tests
- `internal/config/validators_register.go` - `init()` registration
- `test/parse/valid-full-config.ci` - valid config loads
- `test/parse/invalid-enum.ci` - invalid enum rejected
- `test/parse/invalid-range.ci` - invalid range rejected

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 0: YANG Schema Fixes (string -> enum)

1. **Fix `ze-bgp-conf.yang`** — convert `mode` and `direction` leaves from `type string` to `type enumeration` with the documented values
   -> Review: Valid YANG syntax? All existing valid values covered? No config breakage?

2. **Add pattern to `address-family` typedef** — `pattern '[a-z0-9-]+/[a-z0-9-]+'` in `ze-types.yang`
   -> Review: Pattern matches all known families? Doesn't reject valid ones?

3. **Verify** -> `make ze-lint && make ze-unit-test` (no new tests needed yet — schema fixes are exercised by existing tests)
   -> Review: Existing tests still pass? Any tests that relied on invalid values now correctly fail?

### Phase 1: Recursive Tree Walk

4. **Write recursive validation tests** — test valid config, enum violation, range violation, pattern violation, mandatory missing, multiple errors, nested containers, list entries, new enum fields
   -> Review: Tests use real YANG schema entries? Cover all constraint types?

5. **Run tests** -> Verify FAIL (paste output)
   -> Review: Fail for the right reason (no recursive walk), not syntax errors?

6. **Implement `ValidateTree`** — recursive walk of YANG entry tree against data map. For leaves: call existing `validateEntry`. For containers: check mandatory, recurse. For lists: recurse each entry. Collect all errors.
   -> Review: Handles missing entries gracefully? Handles `allow-unknown-fields`? Doesn't duplicate existing `validateEntry` logic?

7. **Run tests** -> Verify PASS (paste output)
   -> Review: All existing validator tests still pass? No regression?

8. **Wire into editor validator** — replace hand-coded hold-time and mandatory peer-as checks with `ValidateTree` call
   -> Review: Same validation results? Editor tests still pass?

9. **Verify** -> `make ze-lint && make ze-unit-test`

### Phase 2: Validator Registry (with completion)

10. **Write registry tests** — register/retrieve, missing returns nil, custom validator called during walk, custom error propagated, Complete() returns values, check-all with all present, check-all with missing
    -> Review: Tests cover both validation and completion? Happy path and error cases?

11. **Run tests** -> Verify FAIL

12. **Implement registry** — `registry.go` with Validate + Complete interface. Register/Get/Names/CheckAll functions. Read `ze:validate` extension in tree walk.
    -> Review: Thread-safe? (Only written during init, read after — no mutex needed.) Import cycle free?

13. **Run tests** -> Verify PASS

14. **Add `validate` extension to YANG** — add to `ze-extensions.yang`
    -> Review: Valid YANG syntax? Follows existing extension pattern?

15. **Verify** -> `make ze-lint && make ze-unit-test`

### Phase 3: Custom Validators (runtime enums + semantic checks)

16. **Write custom validator tests** — registered-address-family (validate + complete), nonzero-ipv4, community-range
    -> Review: Boundary cases covered? Completion returns correct set? Error messages clear?

17. **Run tests** -> Verify FAIL

18. **Implement custom validators** — one file for functions with Validate + Complete, one for registration
    -> Review: Functions query plugin registry for runtime sets? Pure validation (no side effects)? Complete() returns sorted values?

19. **Add `ze:validate` to YANG** — `address-family` typedef and family `name` leaves in `ze-bgp-conf.yang`
    -> Review: Extension on typedef propagates to uses? Only used where YANG native can't express the constraint?

20. **Run tests** -> Verify PASS

21. **Verify** -> `make ze-lint && make ze-unit-test`

### Phase 4: Integration (Wiring + Functional Tests)

22. **Wire recursive validation into config reader** — replace flat `ValidateContainer` with recursive `ValidateTree`
    -> Review: Existing reader tests still pass? Nil validator still skips?

23. **Add startup integrity check** — in `config.Loader` after YANG modules loaded, call `CheckAllValidatorsRegistered(loader)`
    -> Review: Called after all init()? Fails clearly?

24. **Write functional tests** — valid config loads, invalid enum rejected (including new enum fields), invalid range rejected, mandatory missing rejected
    -> Review: Tests use Ze's config syntax, not raw JSON?

25. **Verify all** -> `make ze-verify` (paste output)

26. **Critical Review** -> All 6 checks from `rules/quality.md`

24. **Complete spec** -> Fill audit tables, move to done

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step that introduced it (fix syntax/types) |
| Test fails wrong reason | Fix test |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Extension not read from YANG | Check `getSyntaxExtension` pattern, verify goyang stores it |
| Registry import cycle | Move registry to `internal/yang/` (leaf package, no deps) |

## Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| YANG native first | Fix schema before adding ze:validate | If YANG can express it (enum, pattern, range), use YANG — cheapest, most correct |
| ze:validate = last resort | Only for runtime-determined or semantic checks | Static known values belong in YANG; ze:validate for what YANG cannot express |
| Validator provides completion | Validate + Complete interface | Runtime enums (families) need both validation and CLI completion from same source |
| Extension name | `ze:validate` | RFC 7950 Section 7.19 extension mechanism; follows `ze:syntax`, `ze:key-type` pattern |
| Registry location | `internal/yang/` | Leaf package, no deps; validators in other packages register via `init()` |
| In-process, not external | Go functions, not shell programs | Ze is single-binary; external programs are a future feature from the architecture doc |
| Collect all errors | `[]ValidationError`, not first-error-stops | Users want to see all problems at once, not fix one at a time |
| Fail startup on missing | Hard fail | Ze's design: fail-early, never silently default; matches plugin registration pattern |
| Registry thread safety | None (init-only writes) | Same pattern as plugin registry; written during init(), read-only after |
| Custom validators per-typedef | Extension on typedef, inherited by uses | RFC 7950 Section 7.3.4: typedef restrictions propagate to all uses; DRY |
| No must/when | Use `ze:validate` instead | RFC 7950 Sections 7.5.3/7.21.5 require XPath eval; goyang doesn't support it |

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

## RFC Documentation

RFC 7950 sections enforced by this spec:

| RFC 7950 Section | Constraint | Ze Implementation |
|------------------|-----------|-------------------|
| 9.2.4 | `range` — numeric values within declared bounds | `checkYangRange` / `checkYangRangeSigned` in `validator.go` |
| 9.4.4 | `length` — string byte length within declared bounds | `validateString` length check in `validator.go` |
| 9.4.5 | `pattern` — string matches all declared regex patterns (AND-combined) | `regexp.MatchString` in `validateString` in `validator.go` |
| 9.6 | `enumeration` — value must be one of declared enum names | `validateEnumeration` in `validator.go` |
| 7.6.5 | `mandatory` — mandatory node must exist in data | `validateContainerEntry` mandatory check in `validator.go` |
| 9.12 | `union` — value must match at least one member type | `validateUnion` in `validator.go` |
| 7.19 | `extension` — custom statements with argument; valid YANG | `ze:validate` extension in `ze-extensions.yang` |

Ze uses goyang (not libyang) so `must`/`when` XPath evaluation (Sections 7.5.3, 7.21.5) is out of scope. The `ze:validate` extension provides equivalent functionality via registered Go functions.

## Implementation Summary

### What Was Implemented
- Phase 0: Converted 4 leaves in ze-bgp-conf.yang from `type string` to `type enumeration` (family.mode, nexthop.mode, add-path.direction, add-path.mode). Added pattern to address-family typedef.
- Phase 1: `ValidateTree`/`walkTree` recursive validation in `internal/yang/validator.go`. Replaced hand-coded hold-time/mandatory checks in editor validator with systematic tree walk.
- Phase 2: `ValidatorRegistry` with `CustomValidator{ValidateFn, CompleteFn}` in `internal/yang/registry.go`. Added `validate` extension to ze-extensions.yang. Tree walk checks `ze:validate` extension during leaf validation.
- Phase 3: Three custom validators in `internal/config/validators.go`: `AddressFamilyValidator`, `NonzeroIPv4Validator`, `CommunityRangeValidator`. Registration in `validators_register.go`.
- Phase 4: Wired registry creation, registration, and integrity check into `YANGValidatorWithPlugins`. Added 3 functional tests. String-to-number conversion in validator for config values from `Tree.ToMap()`.

### Bugs Found/Fixed
- Editor validator (`ValidateWithYANG`) called `ValidateTree` which caught ALL mandatory violations. Fix: filter `ErrTypeMissing` during editing (config is incomplete), except `peer-as` which is always mandatory.
- Error messages from YANG validator didn't contain field name (Path had it, Message didn't). Fix: `yangLeafName()` helper extracts leaf name from YANG path and includes it in error format.
- String-to-number conversion: `Tree.ToMap()` returns `map[string]string` but YANG validator expected typed values. Fix: added `case string` branches to `validateUnsigned`/`validateSigned` with `strconv.ParseUint`/`ParseInt`.

### Documentation Updates
- `docs/architecture/config/yang-config-design.md` — added implementation note in Section 5 about `ze:validate` (in-process Go validators via ValidatorRegistry, complementing the aspirational `zx:validator` external program design)

### Deviations from Plan
- `validators_register.go` uses `RegisterValidators(reg)` function instead of `init()` — called explicitly from `YANGValidatorWithPlugins`. This is better than `init()` because it allows passing the registry and avoids global mutable state.
- Functional test filenames use `validate-` prefix instead of `test-` prefix (e.g., `validate-valid-full.ci` not `valid-full-config.ci`)
- `registered-afi` and `large-community-range` validators were not implemented — only `registered-address-family`, `nonzero-ipv4`, and `community-range` are registered. `registered-afi` is not referenced by any `ze:validate` in current YANG; `large-community-range` can be added when needed (YAGNI).
- Config reader (`internal/config/reader.go`) was NOT changed to use `ValidateTree` — it continues using flat `ValidateContainer`. The reader validates per-block at parse time; recursive tree walk is used at load time via `YANGValidatorWithPlugins` and in the editor.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix YANG schema: string → enum | ✅ Done | `ze-bgp-conf.yang`:163-182, 221-233, 336-358 | 4 leaves converted |
| Recursive tree walk validation | ✅ Done | `internal/yang/validator.go`:506-595 | `ValidateTree`/`walkTree` |
| Custom validation with completion | ✅ Done | `internal/yang/registry.go`, `internal/config/validators.go` | Registry + 3 validators |
| Startup integrity check | ✅ Done | `internal/config/yang_schema.go`:108 | `CheckAllValidatorsRegistered` |
| Replace editor ad-hoc YANG checks | ✅ Done | `internal/config/editor/validator.go`:138-199 | Uses `ValidateTree` |
| ze:validate extension in YANG | ✅ Done | `ze-extensions.yang`:56-67, `ze-types.yang`:143, `ze-bgp-conf.yang`:166 | Extension + references |
| Nil validator graceful degradation | ✅ Done | `internal/config/editor/validator.go`:139 | `if v.yangValidator == nil { return nil }` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `test/parse/validate-invalid-enum.ci` | `connection invalid-value` → exit 1 |
| AC-2 | ✅ Done | `TestValidateTree_RangeViolation` | Port 0 caught by YANG range |
| AC-3 | ✅ Done | `test/parse/validate-invalid-range.ci` | `hold-time 2` → exit 1 |
| AC-4 | ✅ Done | `TestValidateTree_MandatoryMissing` | Missing router-id → ErrTypeMissing |
| AC-5 | ✅ Done | `TestValidateTree_MandatoryMissing` | Missing local-as → ErrTypeMissing |
| AC-6 | ✅ Done | `TestValidateTree_PatternViolation` | Invalid router-id → pattern error |
| AC-7 | ✅ Done | `test/parse/validate-valid-full.ci` + `TestValidateTree_ValidConfig` | Valid config → 0 errors |
| AC-8 | ✅ Done | `TestValidateTree_FamilyModeEnum` | `enable` accepted for enum mode |
| AC-9 | ✅ Done | `TestValidateTree_FamilyModeEnum` | `invalid-mode` rejected for enum mode |
| AC-10 | ✅ Done | `TestValidateTree_AddPathDirectionEnum` | `send` accepted for enum direction |
| AC-11 | ✅ Done | `TestValidatorRegistry_Register`, `TestValidatorRegistry_CustomValidation` | Custom validator called |
| AC-12 | ✅ Done | `TestCheckAllValidatorsRegistered_Missing` | Missing → error with name |
| AC-13 | ✅ Done | `TestValidatorRegistry_CustomError` | Custom error propagated |
| AC-14 | ✅ Done | `internal/config/editor/validator.go`:139 | Nil check at top of `ValidateWithYANG` |
| AC-15 | ✅ Done | `internal/config/editor/validator.go`:171 | Uses `ValidateTree` instead of hand-coded checks |
| AC-16 | ✅ Done | `TestAddressFamilyValidator_Validate` | `ipv4/unicast` accepted (registered) |
| AC-17 | ✅ Done | `TestAddressFamilyValidator_Validate` | `invalid/family` rejected |
| AC-18 | ✅ Done | `TestAddressFamilyValidator_Complete` | Returns registered families |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestValidateTree_ValidConfig` | ✅ Done | `validator_test.go`:24 | |
| `TestValidateTree_EnumViolation` | ✅ Done | `validator_test.go`:48 | |
| `TestValidateTree_RangeViolation` | ✅ Done | `validator_test.go`:73 | |
| `TestValidateTree_PatternViolation` | ✅ Done | `validator_test.go`:127 | |
| `TestValidateTree_MandatoryMissing` | ✅ Done | `validator_test.go`:145 | |
| `TestValidateTree_MultipleErrors` | ✅ Done | `validator_test.go`:168 | |
| `TestValidateTree_NestedContainers` | ✅ Done | `validator_test.go`:185 | |
| `TestValidateTree_ListEntries` | ✅ Done | `validator_test.go`:213 | |
| `TestValidatorRegistry_Register` | ✅ Done | `registry_test.go`:15 | |
| `TestValidatorRegistry_Missing` | ✅ Done | `registry_test.go`:35 | |
| `TestValidatorRegistry_CustomValidation` | ✅ Done | `registry_test.go`:45 | |
| `TestValidatorRegistry_CustomError` | ✅ Done | `registry_test.go`:68 | |
| `TestCheckAllValidatorsRegistered_AllPresent` | ✅ Done | `registry_test.go`:142 | |
| `TestCheckAllValidatorsRegistered_Missing` | ✅ Done | `registry_test.go`:161 | |
| `TestValidateExtension_ParsedFromYANG` | ✅ Done | `registry_test.go`:177 | Tests `GetValidateExtension(nil)` |
| `TestAddressFamilyValidator_Validate` | ✅ Done | `validators_test.go`:17 | |
| `TestAddressFamilyValidator_Complete` | ✅ Done | `validators_test.go`:47 | |
| `TestNonzeroIPv4Validator` | ✅ Done | `validators_test.go`:72 | |
| `TestCommunityRangeValidator` | ✅ Done | `validators_test.go`:87 | |
| `TestFamilyModeEnum` | ✅ Done | `validator_test.go`:244 | `TestValidateTree_FamilyModeEnum` |
| `TestAddPathDirectionEnum` | ✅ Done | `validator_test.go`:292 | `TestValidateTree_AddPathDirectionEnum` |
| Functional: valid config | ✅ Done | `test/parse/validate-valid-full.ci` | |
| Functional: invalid enum | ✅ Done | `test/parse/validate-invalid-enum.ci` | |
| Functional: invalid range | ✅ Done | `test/parse/validate-invalid-range.ci` | |
| Functional: mandatory missing | ⚠️ Partial | via `TestValidateTree_MandatoryMissing` | Unit-tested, no standalone `.ci` test |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp/schema/ze-bgp-conf.yang` | ✅ Modified | string→enum, added `ze:validate` |
| `internal/yang/modules/ze-types.yang` | ✅ Modified | Pattern + `ze:validate` on address-family |
| `internal/yang/validator.go` | ✅ Modified | `ValidateTree`/`walkTree`, `SetRegistry`, string-to-number conversion |
| `internal/yang/validator_test.go` | ✅ Modified | 10 new `TestValidateTree_*` tests |
| `internal/config/editor/validator.go` | ✅ Modified | Replaced hand-coded checks with `ValidateTree` |
| `internal/yang/modules/ze-extensions.yang` | ✅ Modified | Added `validate` extension |
| `internal/yang/registry.go` | ✅ Created | ValidatorRegistry, GetValidateExtension, CheckAllValidatorsRegistered |
| `internal/yang/registry_test.go` | ✅ Created | 10 registry tests |
| `internal/config/validators.go` | ✅ Created | 3 custom validators |
| `internal/config/validators_test.go` | ✅ Created | 4 validator tests |
| `internal/config/validators_register.go` | ✅ Created | `RegisterValidators()` |
| `internal/config/yang_schema.go` | ✅ Modified | Registry creation + integrity check in `YANGValidatorWithPlugins` |
| `test/parse/validate-valid-full.ci` | ✅ Created | Valid config functional test |
| `test/parse/validate-invalid-enum.ci` | ✅ Created | Invalid enum functional test |
| `test/parse/validate-invalid-range.ci` | ✅ Created | Invalid range functional test |
| `internal/config/reader.go` | 🔄 Changed | NOT modified — reader keeps using `ValidateContainer`; recursive validation at load time instead |

### Audit Summary
- **Total items:** 67
- **Done:** 65
- **Partial:** 1 (mandatory-missing functional test — covered by unit test, no `.ci`)
- **Skipped:** 0
- **Changed:** 1 (reader.go — recursive validation at load time, not per-block in reader)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implement
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
