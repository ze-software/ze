# Spec: softver-plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin architecture
4. `internal/plugins/bgp-hostname/` - reference implementation (exact same pattern)

## Task

Extract the software-version capability (code 75, draft-ietf-idr-software-version) from the core BGP plugin into its own standalone plugin `bgp-softver`, following the exact pattern established by `bgp-hostname` (FQDN, code 73).

**Motivation:** Software-version and FQDN are structurally identical informational capabilities. FQDN was correctly extracted into `bgp-hostname`. Software-version remains hardcoded in:
- `internal/plugins/bgp/capability/capability.go` (parse/encode, lines 703-750)
- `internal/plugins/bgp/reactor/config.go` (config handling, lines 340-348)
- `internal/plugins/bgp/format/decode.go` (decode, lines 162-163)
- `cmd/ze/bgp/decode.go` (CLI decode, lines 304-305)
- `internal/plugins/bgp/schema/ze-bgp-conf.yang` (YANG, lines 210-213)

This violates the plugin design principle: informational capabilities that have their own config, encoding, and decoding should be isolated plugins, not hardcoded in the core BGP reactor.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` - plugin registration, 5-stage protocol, checklist
  → Constraint: plugins register via `init()` + `registry.Register()`, must have `register.go`
  → Constraint: must register `CapabilityCodes`, `ConfigRoots`, `YANG`, `CLIHandler`
- [ ] `docs/architecture/core-design.md` - engine/plugin separation
  → Decision: plugins receive config via SDK callbacks, not hardcoded in reactor

### Reference Implementation
- [ ] `internal/plugins/bgp-hostname/hostname.go` - exact pattern to follow
  → Constraint: uses `sdk.NewWithConn`, `OnConfigure`, `SetCapabilities`, `Run`
- [ ] `internal/plugins/bgp-hostname/register.go` - registration pattern
  → Constraint: `SupportsCapa: true`, `Features: "capa yang"`, `CapabilityCodes: []uint8{N}`
- [ ] `internal/plugins/bgp-hostname/schema/ze-hostname.yang` - YANG augmentation pattern
  → Constraint: augment `/bgp:bgp/bgp:peer/bgp:capability`

**Key insights:**
- bgp-hostname is the exact template: same structure, same SDK usage, same decode protocol
- Software-version wire format: version-len (1 byte) + version string (variable)
- FQDN wire format: host-len (1) + host + domain-len (1) + domain
- Software-version is simpler (single string vs two strings)
- Current reactor hardcodes version string `"ExaBGP/5.0.0-0+test"` — plugin should use ze's actual version

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp/capability/capability.go` - defines `SoftwareVersion` struct, `parseSoftwareVersion()`, `CodeSoftwareVersion = 75`
- [x] `internal/plugins/bgp/reactor/config.go:340-348` - reads `software-version` from config, hardcodes version string `"ExaBGP/5.0.0-0+test"`
- [x] `internal/plugins/bgp/format/decode.go:162-163` - decodes software-version capability to `DecodedCapability`
- [x] `cmd/ze/bgp/decode.go:304-305` - CLI decode for software-version in OPEN messages
- [x] `internal/plugins/bgp/schema/ze-bgp-conf.yang:210-213` - YANG `container software-version` with `ze:syntax "flex"`
- [x] `internal/plugins/bgp-hostname/` - reference plugin (complete read)

**Behavior to preserve:**
- Wire format: code 75, value = version-len (1) + version-string (variable, max 255)
- Config syntax: `capability { software-version enable; }` under peer
- Capability negotiation: opt-in, absent = disabled
- Decode output JSON: `{"code": 75, "name": "software-version", "value": "<version>"}`
- Existing functional tests: `test/encode/cap-software-version.ci`, `test/decode/bgp-open-sofware-version.ci`

**Behavior to change:**
- Version string should be ze's version, not hardcoded `"ExaBGP/5.0.0-0+test"`
- Software-version parsing/encoding moves from core capability.go to plugin
- Config handling moves from reactor/config.go to plugin's `OnConfigure` callback
- Decode handling moves from format/decode.go and cmd/ze/bgp/decode.go to plugin's decode mode

## Data Flow (MANDATORY)

### Entry Point — Config
- User writes `capability { software-version enable; }` in bgp peer config
- Config parsed via YANG schema → bgp config tree → delivered to plugin via Stage 2

### Transformation Path (after migration)
1. YANG schema (`ze-softver.yang`) validates config structure
2. Engine delivers bgp config JSON to plugin via `OnConfigure` callback (Stage 2)
3. Plugin extracts per-peer software-version config
4. Plugin declares capability (code 75, hex-encoded value) via `SetCapabilities` (Stage 3)
5. Engine includes capability in OPEN message wire encoding

### Entry Point — Decode
- OPEN message received with capability code 75
- Engine dispatches to plugin's decode mode (stdin/stdout protocol)
- Plugin returns decoded JSON or text

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Plugin | JSON via SDK `OnConfigure` callback (Stage 2) | [ ] |
| Plugin → Engine | `SetCapabilities` with hex payload (Stage 3) | [ ] |
| Engine → Plugin (decode) | stdin: `decode capability 75 <hex>` | [ ] |
| Plugin → Engine (decode) | stdout: `decoded json <json>` | [ ] |

### Integration Points
- `registry.Register()` — plugin registration with capability code 75
- `sdk.NewWithConn` + `sdk.Registration` — standard plugin lifecycle
- YANG augmentation of `ze-bgp-conf` — config schema for `software-version`
- `cli.RunPlugin` — CLI dispatch for decode commands

### Architectural Verification
- [ ] No bypassed layers — plugin uses standard SDK protocol, not hardcoded reactor logic
- [ ] No unintended coupling — removes direct dependency from reactor on software-version
- [ ] No duplicated functionality — moves code, doesn't copy
- [ ] Zero-copy preserved — capability encoding uses existing `WriteTo` pattern

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `software-version enable` under peer capability | OPEN message includes capability 75 with ze version string |
| AC-2 | Config has no `software-version` | OPEN message does not include capability 75 |
| AC-3 | Received OPEN with capability 75 | `ze bgp decode` outputs `{"code":75,"name":"software-version","value":"..."}` |
| AC-4 | Plugin decode mode: `decode capability 75 <hex>` on stdin | Returns `decoded json {"name":"software-version","version":"..."}` |
| AC-5 | Plugin decode mode: `decode text capability 75 <hex>` on stdin | Returns `decoded text software-version  <version>` |
| AC-6 | `bgp-softver` registered in plugin registry | `TestAllPluginsRegistered` count incremented |
| AC-7 | YANG schema validates `software-version` under capability | Config accepted with `software-version enable` |
| AC-8 | Version string encoding | Wire bytes: len(1) + utf8 string, max 255 bytes |
| AC-9 | Capability mode support | `software-version require` / `software-version refuse` work |
| AC-10 | No software-version code in core BGP reactor | `reactor/config.go` has no `SoftwareVersion` references |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractSoftverCapabilities` | `internal/plugins/bgp-softver/softver_test.go` | Config parsing extracts version per peer | |
| `TestExtractSoftverCapabilitiesEmpty` | `internal/plugins/bgp-softver/softver_test.go` | No config = no capabilities | |
| `TestEncodeValue` | `internal/plugins/bgp-softver/softver_test.go` | Wire encoding round-trip | |
| `TestEncodeValueBoundary` | `internal/plugins/bgp-softver/softver_test.go` | 255-byte and 256-byte version strings | |
| `TestDecodeSoftwareVersion` | `internal/plugins/bgp-softver/softver_test.go` | Wire decoding | |
| `TestRunDecodeMode` | `internal/plugins/bgp-softver/softver_test.go` | JSON decode protocol | |
| `TestRunDecodeModeText` | `internal/plugins/bgp-softver/softver_test.go` | Text decode protocol | |
| `TestRunCLIDecode` | `internal/plugins/bgp-softver/softver_test.go` | CLI hex-to-JSON decode | |
| `TestYANGSchema` | `internal/plugins/bgp-softver/softver_test.go` | Embedded YANG schema validates | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Version string length | 0-255 | 255 bytes | N/A | 256 bytes (truncated) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cap-software-version` | `test/encode/cap-software-version.ci` | Existing — OPEN includes cap 75 when enabled | |
| `bgp-open-sofware-version` | `test/decode/bgp-open-sofware-version.ci` | Existing — decode OPEN with cap 75 | |

### Future
- ExaBGP compat test (`test/exabgp-compat/encoding/conf-cap-software-version.ci`) may need update if version string changes

## Files to Modify

### Remove software-version from core BGP plugin
- `internal/plugins/bgp/capability/capability.go` — remove `SoftwareVersion` struct, `parseSoftwareVersion()`, `CodeSoftwareVersion` constant, and switch case in `parseCapability()`
- `internal/plugins/bgp/reactor/config.go` — remove software-version config handling (lines 339-348)
- `internal/plugins/bgp/format/decode.go` — remove software-version decode case (lines 162-163)
- `cmd/ze/bgp/decode.go` — remove software-version decode case (lines 304-305)
- `internal/plugins/bgp/schema/ze-bgp-conf.yang` — remove `container software-version` (lines 210-213)
- `internal/plugins/bgp/reactor/config_test.go` — remove/update software-version assertions
- `internal/plugins/bgp/capability/capability_test.go` — remove software-version test entries

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/plugins/bgp-softver/schema/ze-softver.yang` |
| RPC count in architecture docs | [ ] | N/A (no new RPCs) |
| CLI commands/flags | [ ] | N/A (uses standard cli.RunPlugin) |
| Plugin SDK docs | [ ] | N/A (follows existing pattern) |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | Existing tests should pass unchanged |
| `TestAllPluginsRegistered` count | [x] | `internal/plugin/registry/*_test.go` |

## Files to Create

- `internal/plugins/bgp-softver/softver.go` — plugin implementation (config extraction, encode, decode)
- `internal/plugins/bgp-softver/softver_test.go` — unit tests
- `internal/plugins/bgp-softver/register.go` — registry init()
- `internal/plugins/bgp-softver/schema/embed.go` — YANG embed
- `internal/plugins/bgp-softver/schema/ze-softver.yang` — YANG schema augmenting bgp:capability

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create plugin skeleton** — `register.go`, `schema/embed.go`, `schema/ze-softver.yang`
2. **Write unit tests** in `softver_test.go` → Review: edge cases? Boundary tests?
3. **Run tests** → Verify FAIL (paste output)
4. **Implement `softver.go`** — `RunSoftverPlugin`, `extractSoftverCapabilities`, `encodeValue`, `decodeSoftwareVersion`, `RunDecodeMode`, `RunCLIDecode`
5. **Run tests** → Verify PASS (paste output)
6. **Remove from core** — delete software-version code from capability.go, reactor/config.go, format/decode.go, cmd/ze/bgp/decode.go, ze-bgp-conf.yang
7. **Run `make generate`** — regenerate `all/all.go` to include new plugin
8. **Update `TestAllPluginsRegistered`** — increment expected count
9. **Run full suite** → `make ze-verify`
10. **Verify functional tests** — existing `.ci` tests should pass with plugin-based implementation
11. **Critical Review** → All 6 checks from `rules/quality.md`
12. **Complete spec** → Fill audit tables, move spec to `done/`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 4 (fix implementation) |
| Test fails wrong reason | Step 2 (fix test) |
| Functional test fails | Check if decode dispatch reaches plugin — may need decode registration |
| Lint failure | Fix inline |
| Plugin not invoked | Check `register.go` init, `all/all.go` regeneration |

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
- Software-version hardcodes `"ExaBGP/5.0.0-0+test"` — this is a leftover from ExaBGP migration, should use ze's actual version
- The `CodeSoftwareVersion` constant in capability.go is used by reactor for cap mode enforcement — after extraction, the Unknown capability handler will catch code 75, and the plugin's registered `CapabilityCodes` will handle dispatch

## RFC Documentation

- draft-ietf-idr-software-version: defines capability code 75
- Wire format: version-length (1 octet, max 255) + version-string (UTF-8)
- Add `// draft-ietf-idr-software-version: "<quoted requirement>"` above enforcing code

## Implementation Summary

### What Was Implemented
- [to be filled after implementation]

### Bugs Found/Fixed
- [to be filled]

### Documentation Updates
- [to be filled]

### Deviations from Plan
- [to be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/plugins/bgp-softver/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (if needed)
- [ ] Critical Review passes (all 6 checks in `rules/quality.md`)

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] draft-ietf-idr-software-version comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (follows bgp-hostname exactly)
- [ ] No speculative features
- [ ] Single responsibility (decode + encode + config extraction)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (removes coupling from reactor)

### TDD
- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] Boundary tests for version string length
- [ ] Functional tests pass end-to-end

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-softver-plugin.md`
- [ ] Spec included in commit
