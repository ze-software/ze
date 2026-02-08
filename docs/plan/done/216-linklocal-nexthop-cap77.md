# Spec: Link-Local Nexthop Capability (Code 77)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugin/llnh/llnh.go` - Plugin implementation
3. `.claude/rules/plugin-design.md` - Plugin patterns

## Task

Implement BGP Link-Local Nexthop Capability (code 77) as a plugin following the hostname plugin pattern.

## Status

**DONE** - Implemented as plugin `ze.llnh`.

## Reference

| Field | Value |
|-------|-------|
| Draft | [draft-ietf-idr-linklocal-capability-02](https://datatracker.ietf.org/doc/draft-ietf-idr-linklocal-capability/) |
| Capability Code | 77 (0x4D) |
| Capability Length | 0 |
| Updates | RFC 2545 |
| Status | IETF Working Group Draft |

## Purpose

Allow IPv6 link-local-only next hops (without global address) in BGP UPDATE messages.

**Use case:** Data center fabrics (RFC 7938) where BGP runs on point-to-point links and only link-local addresses are available.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/wire/capabilities.md` - Capability wire format
- [x] `.claude/rules/plugin-design.md` - Plugin patterns (hostname as reference)

### Source Files
- [x] `internal/plugin/hostname/hostname.go` - Reference plugin pattern
- [x] `internal/plugin/hostname/schema/ze-hostname.yang` - Reference YANG schema

## Wire Format

### Capability Advertisement

```
+---------------------------+
| Cap Code = 77 (1 octet)   |
+---------------------------+
| Cap Length = 0 (1 octet)  |
+---------------------------+
```

No capability value - presence indicates support.

### MP_REACH_NLRI Next Hop Encoding (FUTURE - not in this spec)

~~When link-local capability is negotiated:~~

| Next Hop Length | Content |
|-----------------|---------|
| 16 | IPv6 global address (standard) |
| 16 | IPv6 link-local address (NEW - with this capability) |
| 32 | IPv6 global + IPv6 link-local |

~~**Key change:** Length 16 can now be link-local (fe80::/10) not just global.~~

**Superseded:** UPDATE-level next-hop encoding changes are separate scope (tests K/L in ExaBGP compat). This spec covers only the capability advertisement in OPEN.

## Current Behavior

**Source files read:**
- [x] `internal/plugin/hostname/hostname.go` - Reference plugin for capability advertisement pattern
- [x] `internal/plugin/inprocess.go` - Plugin registration maps
- [x] `internal/exabgp/migrate.go` - Migration enableFields list (excluded link-local-nexthop)

Before this implementation, capability code 77 was not supported. The engine would pass unknown capabilities through as raw hex in OPEN messages. ExaBGP configs with `link-local-nexthop` had the field dropped during migration.

## Data Flow

### Entry Point
- Config file with `capability { link-local-nexthop; }` in peer block
- Engine passes config JSON to plugin via Socket B (Stage 2: configure)

### Transformation Path
1. Engine loads YANG schema from plugin, validates config syntax
2. Engine sends peer config JSON to LLNH plugin via `ze-plugin-callback:configure`
3. Plugin extracts `link-local-nexthop` from peer capability config
4. Plugin declares capability 77 (empty payload) via `ze-plugin-engine:declare-capabilities`
5. Engine includes capability in OPEN message to peer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to Plugin | JSON config via Socket B | Yes |
| Plugin to Engine | Capability declaration via Socket A | Yes |

### Integration Points
- SDK `NewWithConn` + `OnConfigure` + `SetCapabilities` - standard plugin lifecycle
- `internalPluginRunners` map - engine starts plugin in-process
- `pluginCapabilityMap` - CLI decode routing

## Implementation Steps

1. **Write unit tests** - 11 tests covering config extraction, decode modes, CLI decode
   → Tests failed as expected (no implementation)

2. **Implement plugin** - `internal/plugin/llnh/llnh.go` following hostname pattern
   → All 11 tests passed

3. **Create YANG schema** - `ze-link-local-nexthop.yang` with flex syntax augmentation
   → Config validation works

4. **Register in integration points** - inprocess.go, plugin.go, decode.go, plugin_llnh.go
   → Plugin discoverable and loadable

5. **Update ExaBGP migration** - Include link-local-nexthop in enableFields
   → Migration preserves the field

6. **Update ExaBGP wrapper** - Load ze.llnh plugin alongside ze.hostname
   → ExaBGP compat test 3 gets capability 77

7. **Verify all** - `make lint && make test && make functional` pass

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-01-28 | DEFER | Wait for draft to become RFC |
| 2025-02-08 | IMPLEMENT as plugin | Needed for ExaBGP test 3; plugin pattern is correct (like hostname cap 73) |
| 2025-02-08 | Plugin, NOT engine-level | User directive: "NEEDS TO BE A PLUGIN - ORDER - NOT AN OPTION" |
| 2025-02-08 | Defer UPDATE encoding | Tests K/L need MP_REACH_NLRI changes - separate scope |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractLLNHCapabilities` | `llnh_test.go` | Extract cap from config JSON | Done |
| `TestExtractLLNHCapabilitiesWrapped` | `llnh_test.go` | Handles double-wrapped JSON | Done |
| `TestExtractLLNHCapabilitiesNoCap` | `llnh_test.go` | No cap when not configured | Done |
| `TestExtractLLNHCapabilitiesMultiplePeers` | `llnh_test.go` | Multiple peers, only enabled ones | Done |
| `TestRunDecodeModeJSON` | `llnh_test.go` | Stdin decode protocol JSON output | Done |
| `TestRunDecodeModeText` | `llnh_test.go` | Stdin decode protocol text output | Done |
| `TestRunDecodeModeUnknownCode` | `llnh_test.go` | Unknown cap code returns unknown | Done |
| `TestRunCLIDecode` | `llnh_test.go` | CLI decode JSON | Done |
| `TestRunCLIDecodeText` | `llnh_test.go` | CLI decode text | Done |
| `TestLLNHPluginYANG` | `llnh_test.go` | YANG schema non-empty | Done |
| `TestDecodableCapabilities` | `llnh_test.go` | Returns cap code 77 | Done |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| ExaBGP test 3 | `test/exabgp-compat/` | OPEN with cap 77 | Covered by ExaBGP compat suite |

## Files to Modify

- `internal/plugin/inprocess.go` - Register runner, YANG, config roots
- `cmd/ze/bgp/plugin.go` - Add dispatch case
- `cmd/ze/bgp/decode.go` - Add cap 77 to pluginCapabilityMap
- `cmd/ze/main_test.go` - Update expected plugins list
- `internal/exabgp/migrate.go` - Include link-local-nexthop in enableFields
- `internal/exabgp/migrate_test.go` - Update migration expectation
- `test/exabgp-compat/bin/exabgp` - Load ze.llnh plugin

## Files to Create

- `internal/plugin/llnh/llnh.go` - Plugin implementation
- `internal/plugin/llnh/llnh_test.go` - Unit tests
- `internal/plugin/llnh/schema/ze-link-local-nexthop.yang` - YANG schema
- `internal/plugin/llnh/schema/embed.go` - Go embed
- `cmd/ze/bgp/plugin_llnh.go` - CLI wrapper

## Implementation Summary

### What Was Implemented

- Full LLNH plugin following hostname plugin pattern (5-stage SDK RPC)
- YANG schema with `ze:syntax "flex"` augmenting peer capability config
- Capability decode support (code 77, JSON and text output)
- CLI wrapper with `--capa`, `--yang`, `--features`, `--log-level` flags
- Registration in all 7 integration points (inprocess.go x3, plugin.go, decode.go, plugin_llnh.go, exabgp wrapper)
- ExaBGP migration now preserves `link-local-nexthop enable` instead of dropping it

### Deviations from Plan

- Original plan: engine-level changes to `capability.go`, `encoding.go`, `negotiated.go`
- Actual: plugin-based (like hostname cap 73), no engine changes needed
- User directive drove this change: capability plugins are the correct pattern

### Scope Boundary

- This spec covers: capability 77 advertisement in OPEN messages
- NOT covered: UPDATE-level link-local next-hop encoding changes (ExaBGP tests K/L)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Capability code 77 parsing/encoding | Done | `internal/plugin/llnh/llnh.go` | Via SDK capability declaration |
| Config support | Done | `internal/plugin/llnh/schema/ze-link-local-nexthop.yang` | flex syntax |
| Plugin pattern | Done | `internal/plugin/llnh/llnh.go` | Follows hostname pattern exactly |
| ExaBGP compat | Done | `test/exabgp-compat/bin/exabgp`, `internal/exabgp/migrate.go` | Wrapper loads ze.llnh |
| Decode support | Done | `internal/plugin/llnh/llnh.go:RunLLNHDecodeMode`, `RunLLNHCLIDecode` | JSON and text |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestExtractLLNHCapabilities | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestExtractLLNHCapabilitiesWrapped | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestExtractLLNHCapabilitiesNoCap | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestExtractLLNHCapabilitiesMultiplePeers | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestRunDecodeModeJSON | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestRunDecodeModeText | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestRunDecodeModeUnknownCode | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestRunCLIDecode | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestRunCLIDecodeText | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestLLNHPluginYANG | Done | `internal/plugin/llnh/llnh_test.go` | |
| TestDecodableCapabilities | Done | `internal/plugin/llnh/llnh_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/llnh/llnh.go` | Created | 226 lines |
| `internal/plugin/llnh/llnh_test.go` | Created | 265 lines, 11 tests |
| `internal/plugin/llnh/schema/ze-link-local-nexthop.yang` | Created | 46 lines |
| `internal/plugin/llnh/schema/embed.go` | Created | 7 lines |
| `cmd/ze/bgp/plugin_llnh.go` | Created | 37 lines |
| `internal/plugin/inprocess.go` | Modified | 3 map entries + imports |
| `cmd/ze/bgp/plugin.go` | Modified | dispatch + usage |
| `cmd/ze/bgp/decode.go` | Modified | cap 77 mapping |
| `cmd/ze/main_test.go` | Modified | expected plugins list |
| `internal/exabgp/migrate.go` | Modified | enableFields |
| `internal/exabgp/migrate_test.go` | Modified | expectation flip |
| `test/exabgp-compat/bin/exabgp` | Modified | --plugin ze.llnh |

### Audit Summary
- **Total items:** 23
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (plugin approach instead of engine-level, documented in Deviations)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes
