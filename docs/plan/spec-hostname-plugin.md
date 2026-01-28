# Spec: hostname-plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/gr/gr.go` - plugin pattern to follow
4. `internal/plugin/bgp/capability/capability.go` - FQDN encoding to move

## Task

Move FQDN/hostname capability (draft-walton-bgp-hostname, code 73) from core engine to a plugin.

**Rationale:** Capabilities not essential to BGP operation should be plugins, not core code. This follows the same pattern as the GR plugin.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/capabilities.md` - capability wire format
- [ ] `docs/architecture/api/capability-contract.md` - plugin capability injection

### RFC Summaries
- [ ] N/A - draft-walton-bgp-hostname is not an RFC

### Source Code
- [ ] `internal/plugin/gr/gr.go` - plugin pattern to follow
- [ ] `cmd/ze/bgp/plugin_gr.go` - CLI entry pattern
- [ ] `internal/plugin/bgp/capability/capability.go` - FQDN struct (lines 629-707)
- [ ] `internal/config/loader.go` - current FQDN injection (lines 285-291)
- [ ] `internal/config/bgp.go` - Hostname/DomainName fields

**Key insights:**
- GR plugin is the pattern: declare config pattern, receive config, emit capability bytes
- FQDN capability code is 73
- Wire format: hostname-len (1) + hostname + domain-len (1) + domain

## Current State

### Current Config Syntax (to be replaced)
Top-level `host-name` and `domain-name` at neighbor level.

### New Config Syntax (per user request)
Nested under capability block: `capability { hostname { host ...; domain ...; } }`

| Block | Field | Example |
|-------|-------|---------|
| `neighbor X { capability { hostname { ... } } }` | `host` | `host my-router;` |
| `neighbor X { capability { hostname { ... } } }` | `domain` | `domain example.com;` |

### Core Engine (to be removed)
| File | Lines | What |
|------|-------|------|
| `internal/plugin/bgp/capability/capability.go` | 629-707 | FQDN struct, Parse, Pack |
| `internal/config/loader.go` | 285-291 | Auto-inject FQDN from config |
| `internal/config/bgp.go` | 141-142, 860-864 | Hostname/DomainName fields |

## Design

### Architecture Overview

Plugin declares YANG schema → ze delivers matching config → plugin encodes → tells ze what to add to OPEN.

### YANG Schema (ze-hostname.yang)

Plugin provides YANG module that augments ze-bgp capability container:

| Element | Value |
|---------|-------|
| module | `ze-hostname` |
| namespace | `urn:ze:hostname` |
| prefix | `hostname` |
| augments | `/bgp:bgp/bgp:peer/bgp:capability` |

**Schema structure:**

| Path | Type | Description |
|------|------|-------------|
| `capability/hostname` | container | FQDN capability (draft-walton-bgp-hostname) |
| `capability/hostname/host` | leaf string | System hostname (max 255 bytes) |
| `capability/hostname/domain` | leaf string | Domain name (max 255 bytes) |

### Stage 1 Declaration

Plugin declares schema via heredoc:

| Declaration | Purpose |
|-------------|---------|
| `declare schema module ze-hostname` | Module name |
| `declare schema namespace urn:ze:hostname` | YANG namespace |
| `declare schema handler bgp.neighbor.capability.hostname` | Config routing |
| `declare schema yang <<EOF ... EOF` | Full YANG module text |
| `declare conf peer * capability hostname host <host:.*>` | Config pattern for host |
| `declare conf peer * capability hostname domain <domain:.*>` | Config pattern for domain |

### Plugin Protocol Flow

| Stage | Direction | Message |
|-------|-----------|---------|
| 1 - Declaration | Plugin → ze | `declare schema module ze-hostname` |
| 1 - Declaration | Plugin → ze | `declare schema handler bgp.neighbor.capability.hostname` |
| 1 - Declaration | Plugin → ze | `declare conf peer * capability hostname host <host:.*>` |
| 1 - Declaration | Plugin → ze | `declare conf peer * capability hostname domain <domain:.*>` |
| 1 - Declaration | Plugin → ze | `declare done` |
| 2 - Config | ze → Plugin | `config peer 192.168.1.1 host my-router` |
| 2 - Config | ze → Plugin | `config peer 192.168.1.1 domain example.com` |
| 2 - Config | ze → Plugin | `config done` |
| 3 - Capability | Plugin → ze | `capability hex 73 <wire-encoded-fqdn> peer 192.168.1.1` |
| 3 - Capability | Plugin → ze | `capability done` |

### Wire Encoding

FQDN capability (code 73) wire format:

| Field | Size | Description |
|-------|------|-------------|
| Hostname Length | 1 byte | Length of hostname (0-255) |
| Hostname | variable | UTF-8 hostname |
| Domain Length | 1 byte | Length of domain (0-255) |
| Domain Name | variable | UTF-8 domain name |

Example: hostname="router1" (7 bytes), domain="example.com" (11 bytes)
Wire bytes: `07 726F7574657231 0B 6578616D706C652E636F6D`

### Config Key Mapping

| Config Path | Capture Name | Delivered As |
|-------------|--------------|--------------|
| `capability hostname host` | `host` | `host` |
| `capability hostname domain` | `domain` | `domain` |

Note: Config delivery uses capture names, not full config path (per GR plugin pattern).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHostnamePluginParseConfig` | `internal/plugin/hostname/hostname_test.go` | Config parsing for hostname/domain | |
| `TestHostnamePluginEncode` | `internal/plugin/hostname/hostname_test.go` | Wire encoding matches current FQDN.Pack() | |
| `TestHostnamePluginMultiplePeers` | `internal/plugin/hostname/hostname_test.go` | Per-peer config stored correctly | |
| `TestHostnamePluginEmptyValues` | `internal/plugin/hostname/hostname_test.go` | Handles missing hostname or domain | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hostname length | 0-255 | 255 bytes | N/A | 256 bytes (truncate) |
| Domain length | 0-255 | 255 bytes | N/A | 256 bytes (truncate) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `hostname-plugin` | `test/encode/hostname-plugin.ci` | Plugin injects FQDN capability, OPEN contains it | |

### Future
- Dynamic hostname from system (os.Hostname()) - defer to separate feature

## Files to Modify

- `internal/config/loader.go` - Remove FQDN auto-injection (lines 285-291)
- `internal/config/bgp.go` - Remove Hostname/DomainName fields (lines 141-142, 860-864)
- `internal/plugin/bgp/schema/ze-bgp.yang` - Remove host-name/domain-name leaves (lines 224-232)
- `cmd/ze/bgp/commands.go` - Add "plugin hostname" subcommand

## Files to Create

- `internal/plugin/hostname/hostname.go` - Plugin implementation
- `internal/plugin/hostname/hostname_test.go` - Unit tests
- `internal/plugin/hostname/ze-hostname.yang` - YANG schema for hostname capability
- `cmd/ze/bgp/plugin_hostname.go` - CLI entry point
- `test/encode/hostname-plugin.ci` - Functional test

## Files to Keep (Engine still needs to parse peer FQDN)

- `internal/plugin/bgp/capability/capability.go` - Keep FQDN struct for parsing peer OPEN
  - Remove `ConfigValues()` method (no longer needed for config delivery)

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** - Create `internal/plugin/hostname/hostname_test.go`
   - Test config parsing, wire encoding, multiple peers, empty values
   → **Review:** Are edge cases covered? Boundary tests for string lengths?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason? Not syntax errors?

3. **Implement plugin** - Create `internal/plugin/hostname/hostname.go`
   - Follow GR plugin pattern exactly
   - Wire encoding logic from current FQDN.Pack()
   → **Review:** Is this the simplest solution? Matches GR plugin structure?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL tests pass? Any flaky behavior?

5. **Create CLI entry** - Add `cmd/ze/bgp/plugin_hostname.go`
   - Follow plugin_gr.go pattern
   → **Review:** Matches GR CLI pattern?

6. **Add subcommand** - Update `cmd/ze/bgp/commands.go`
   → **Review:** Command registered correctly?

7. **Remove core injection** - Edit `internal/config/loader.go`
   - Remove lines 285-291 (FQDN auto-injection)
   → **Review:** No broken references?

8. **Clean config struct** - Edit `internal/config/bgp.go`
   - Remove Hostname/DomainName fields and parsing
   → **Review:** Tests still compile?

9. **Functional tests** - Create `test/encode/hostname-plugin.ci`
   → **Review:** Tests cover the user-visible behavior?

10. **Update existing test** - Modify `test/encode/hostname.ci` to use plugin
    → **Review:** Test still validates same behavior?

11. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests deterministic?

12. **Final self-review** - Before claiming done:
    - Re-read all code changes: any bugs, edge cases, or improvements?
    - Check for unused code, debug statements, TODOs
    - Verify error messages are clear and actionable

## RFC Documentation

### Reference Comments
- Add `// draft-walton-bgp-hostname` comments in plugin code

### Constraint Comments
Wire encoding constraint: Hostname and domain lengths are 1-byte fields. Strings longer than 255 bytes MUST be truncated.

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

## Checklist

### Design (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC references added to code
- [ ] Constraint comments added

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
