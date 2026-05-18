# Spec: Firewall Global Options (Sysctl Convenience)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/firewall/schema/ze-firewall-conf.yang` - firewall YANG
4. `internal/plugins/sysctl/sysctl.go` - sysctl plugin store
5. `internal/component/firewall/config.go` - firewall config parsing

## Task

Add a `global-options` container to the firewall YANG schema that maps VyOS-style keyword toggles (all-ping, broadcast-ping, syn-cookies, etc.) to their underlying sysctl settings. This provides migration parity with VyOS and a more user-friendly interface than raw sysctl keys for common network security defaults.

At apply time, the firewall component translates global-options into sysctl key/value pairs and emits them into the sysctl plugin's config layer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation, EventBus
  -> Constraint: firewall component must not call sysctl plugin directly; use EventBus or shared config
- [ ] `internal/plugins/sysctl/sysctl.go` - three-layer priority model (config > transient > default)
  -> Constraint: global-options values should enter at config layer to be authoritative

**Key insights:**
- The sysctl plugin already has a robust apply model with three layers and rollback
- VyOS global-options are pure sysctl writes; no nftables rules involved
- The mapping is static: each keyword maps to exactly one sysctl key and the value is derived from enable/disable
- This is config sugar, not new kernel functionality

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` - no `global-options` container
- [ ] `internal/component/firewall/config.go` - parses firewall tables from config tree
- [ ] `internal/plugins/sysctl/sysctl.go` - `applyConfig(settings map[string]string)` applies config-layer sysctls
- [ ] `internal/plugins/sysctl/register.go` - EventBus wiring for sysctl events

**Behavior to preserve:**
- All existing firewall table/chain/term functionality unchanged
- Sysctl plugin's three-layer priority model
- Users who already set these values via `sysctl { setting { ... } }` should see config-wins behavior (explicit sysctl overrides global-options)

**Behavior to change:**
- Add `global-options` container to firewall YANG
- Firewall component emits sysctl settings derived from global-options at config apply time

## Data Flow (MANDATORY)

### Entry Point
- Config tree -> firewall component config extraction -> global-options parsing

### Transformation Path
1. Config tree contains `firewall { global-options { all-ping enable ... } }`
2. Firewall `ExtractGlobalOptions()` reads the container and produces `map[string]string` of sysctl key->value pairs
3. These pairs are merged into the sysctl config-apply batch (same mechanism as `sysctl { setting { ... } }`)
4. Sysctl plugin's `applyConfig()` writes to kernel

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Firewall YANG -> config tree | Container with boolean/enum leaves | [ ] |
| Config tree -> sysctl settings | `ExtractGlobalOptions()` static mapping | [ ] |
| Sysctl settings -> kernel | `applyConfig()` (existing path) | [ ] |

### Integration Points
- Sysctl plugin `applyConfig()` already handles arbitrary key/value maps
- Config commit flow already calls sysctl apply; global-options entries merge into that batch

### Architectural Verification
- [ ] No bypassed layers (uses sysctl plugin, not direct writes)
- [ ] No unintended coupling (firewall emits sysctl settings, doesn't call sysctl plugin)
- [ ] No duplicated functionality (reuses sysctl plugin entirely)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `firewall { global-options { all-ping enable } }` | -> | `ExtractGlobalOptions()` producing sysctl map | `TestExtractGlobalOptions` |
| Global-options sysctl map | -> | Sysctl plugin `applyConfig()` | Existing sysctl apply path (integration) |

## Keyword-to-Sysctl Mapping

| VyOS Keyword | Type | Sysctl Key | enable value | disable value |
|---|---|---|---|---|
| `all-ping` | enable/disable | `net.ipv4.icmp_echo_ignore_all` | `0` | `1` |
| `broadcast-ping` | enable/disable | `net.ipv4.icmp_echo_ignore_broadcasts` | `0` | `1` |
| `syn-cookies` | enable/disable | `net.ipv4.tcp_syncookies` | `1` | `0` |
| `receive-redirects` | enable/disable | `net.ipv4.conf.all.accept_redirects` | `1` | `0` |
| `send-redirects` | enable/disable | `net.ipv4.conf.all.send_redirects` | `1` | `0` |
| `source-validation` | disable/strict/loose | `net.ipv4.conf.all.rp_filter` | `0`/`1`/`2` | - |
| `log-martians` | enable/disable | `net.ipv4.conf.all.log_martians` | `1` | `0` |
| `ipv6-receive-redirects` | enable/disable | `net.ipv6.conf.all.accept_redirects` | `1` | `0` |
| `ipv6-src-route` | enable/disable | `net.ipv6.conf.all.accept_source_route` | `1` | `0` |

Note: `enable`/`disable` for the `*_ignore_*` sysctls have inverted semantics (enable = 0, disable = 1) because the sysctl controls "ignore" behavior. The YANG description must clarify this.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `all-ping enable` in global-options | `net.ipv4.icmp_echo_ignore_all` set to `0` |
| AC-2 | `syn-cookies disable` | `net.ipv4.tcp_syncookies` set to `0` |
| AC-3 | `source-validation strict` | `net.ipv4.conf.all.rp_filter` set to `1` |
| AC-4 | `source-validation loose` | `net.ipv4.conf.all.rp_filter` set to `2` |
| AC-5 | `source-validation disable` | `net.ipv4.conf.all.rp_filter` set to `0` |
| AC-6 | Global-option AND explicit `sysctl { setting { ... } }` for same key | Explicit sysctl wins (global-options are lower priority) |
| AC-7 | Global-option removed from config | Sysctl falls back to default or original value |
| AC-8 | No global-options block | No sysctl settings emitted (backward compatible) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractGlobalOptions` | `firewall/config_test.go` | Correct sysctl map from all keywords | |
| `TestExtractGlobalOptionsEmpty` | `firewall/config_test.go` | No block = empty map | |
| `TestExtractGlobalOptionsSourceValidation` | `firewall/config_test.go` | Three-way enum mapping | |
| `TestGlobalOptionsInvertedSemantics` | `firewall/config_test.go` | all-ping enable = icmp_ignore 0 | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| source-validation | disable/strict/loose | loose | N/A (enum) | N/A (enum) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-firewall-global-options` | `test/plugin/firewall-global-options.ci` | Global-options produce correct sysctl writes | |

## Files to Modify
- `internal/component/firewall/schema/ze-firewall-conf.yang` - add `global-options` container
- `internal/component/firewall/config.go` - add `ExtractGlobalOptions()` function
- Sysctl config merge point (where firewall global-options feed into sysctl apply batch)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `ze-firewall-conf.yang` |
| CLI commands/flags | [ ] | N/A (show sysctl already works) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/plugin/firewall-global-options.ci` |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` (firewall section) |

## Files to Create
- `test/plugin/firewall-global-options.ci`

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring** - Add YANG container, create `ExtractGlobalOptions()` skeleton, write failing test
   - Tests: `TestExtractGlobalOptions` (fails: function returns empty)
   - Files: `ze-firewall-conf.yang`, `config.go`, `config_test.go`
2. **Phase: Mapping** - Implement the keyword-to-sysctl translation table
   - Tests: All config_test additions
   - Files: `config.go`
3. **Phase: Integration** - Wire global-options sysctl output into config apply path
   - Tests: Integration test showing sysctl values applied
   - Files: merge point (loader or startup flow)
4. **Phase: Priority** - Ensure explicit `sysctl { setting }` overrides global-options
   - Tests: `TestGlobalOptionsSysctlPriority`
   - Files: merge point
5. **Functional tests**
6. **Full verification** - `make ze-verify`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 8 ACs have implementation |
| Correctness | Inverted semantics correct for all-ping, broadcast-ping |
| Naming | YANG keywords match VyOS names exactly for migration |
| Data flow | Global-options -> sysctl config layer, not bypassing |
| Priority | Explicit sysctl settings always win |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG global-options | `grep "global-options" ze-firewall-conf.yang` |
| Mapping function | `go test ./internal/component/firewall/ -run GlobalOptions` |
| Sysctl integration | Functional test passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Enum leaves only; no free-form input reaches sysctl keys |
| No injection | Sysctl keys are static strings, never derived from user input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Inverted mapping wrong | Fix mapping table, add test per keyword |
| Priority conflict | Research sysctl apply ordering in loader |
| Functional test fails | Check sysctl backend reads on non-Linux |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] VyOS migration parity verified

### Quality Gates
- [ ] Implementation Audit complete
