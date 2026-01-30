# Spec: 02 - FlowSpec Family Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-01-family-plugin-infrastructure.md` - prerequisite spec
4. `internal/plugin/bgp/nlri/flowspec.go` - current FlowSpec implementation
5. `internal/plugin/hostname/hostname.go` - reference plugin pattern

## Task

Move FlowSpec NLRI implementation from `internal/plugin/bgp/nlri/flowspec.go` to a standalone family plugin at `internal/plugin/flowspec/`. This is the first family plugin using the infrastructure from spec-01.

**Prerequisites:** spec-01-family-plugin-infrastructure must be complete.

**Key decisions from user:**
- MOVE code (not copy) - delete from original location
- NO per-message caching
- Plugin registers families with `decode` keyword

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-01-family-plugin-infrastructure.md` - infrastructure this builds on
- [ ] `internal/plugin/bgp/nlri/flowspec.go` - current implementation (2700+ lines)
- [ ] `internal/plugin/hostname/hostname.go` - plugin pattern to follow
- [ ] `docs/architecture/wire/nlri.md` - NLRI wire formats

### RFC Summaries
- [ ] `rfc/short/rfc8955.md` - FlowSpec IPv4
- [ ] `rfc/short/rfc8956.md` - FlowSpec IPv6

**Key insights:**
- FlowSpec has 13 component types (destination, source, protocol, ports, etc.)
- FlowSpec VPN adds Route Distinguisher prefix
- FlowSpec does NOT support ADD-PATH (RFC 8955)
- Current implementation is complete and well-tested

## Current Behavior

**Source files read:**
- [x] `internal/plugin/flowspec/plugin.go` - Plugin with decode/encode handlers
- [x] `internal/plugin/flowspec/types.go` - FlowSpec types (moved from nlri/)
- [x] `internal/plugin/update_text.go` - Text parsing, calls flowspec directly

**Behavior to preserve:**
- FlowSpec JSON output format (nested arrays for components)
- All 13 component types supported
- VPN variants with Route Distinguisher
- Wire encoding/decoding round-trip

**Behavior to change:**
- Engine currently calls `flowspec.EncodeFlowSpecComponents()` as Go function
- Should use plugin API (`encode nlri ...` protocol) for language independence
- RD syntax should be part of components, not special prefix before `add`

## Current Implementation Analysis

### Files to Move

| Source | Lines | Content |
|--------|-------|---------|
| `internal/plugin/bgp/nlri/flowspec.go` | ~2700 | All FlowSpec types and parsing |
| `internal/plugin/bgp/nlri/flowspec_test.go` | ~1080 | Comprehensive tests |

### FlowSpec Families

| Family | AFI | SAFI | Description |
|--------|-----|------|-------------|
| `ipv4/flowspec` | 1 | 133 | IPv4 FlowSpec |
| `ipv6/flowspec` | 2 | 133 | IPv6 FlowSpec |
| `ipv4/flowspec-vpn` | 1 | 134 | IPv4 FlowSpec VPN |
| `ipv6/flowspec-vpn` | 2 | 134 | IPv6 FlowSpec VPN |

### Component Types (RFC 8955 Section 4)

| Type | Name | Description |
|------|------|-------------|
| 1 | Destination Prefix | IPv4/IPv6 destination |
| 2 | Source Prefix | IPv4/IPv6 source |
| 3 | IP Protocol | Protocol number |
| 4 | Port | Any port (src or dst) |
| 5 | Destination Port | Destination port |
| 6 | Source Port | Source port |
| 7 | ICMP Type | ICMP type |
| 8 | ICMP Code | ICMP code |
| 9 | TCP Flags | TCP flags bitmask |
| 10 | Packet Length | Total packet length |
| 11 | DSCP | DiffServ code point |
| 12 | Fragment | Fragment flags |
| 13 | Flow Label | IPv6 flow label |

## Target State

### Plugin Structure

| File | Purpose |
|------|---------|
| `internal/plugin/flowspec/flowspec.go` | Plugin main, decode mode handler |
| `internal/plugin/flowspec/parse.go` | Wire bytes → FlowSpec struct |
| `internal/plugin/flowspec/encode.go` | FlowSpec struct → wire bytes |
| `internal/plugin/flowspec/json.go` | FlowSpec ↔ JSON conversion |
| `internal/plugin/flowspec/components.go` | Component type definitions |
| `internal/plugin/flowspec/flowspec_test.go` | Unit tests (moved + new) |
| `cmd/ze/bgp/plugin_flowspec.go` | CLI entry point |

### Plugin Registration

Stage 1 declarations:

| Declaration | Purpose |
|-------------|---------|
| `declare family ipv4 flowspec decode` | Claim IPv4 FlowSpec decoding |
| `declare family ipv6 flowspec decode` | Claim IPv6 FlowSpec decoding |
| `declare family ipv4 flowspec-vpn decode` | Claim IPv4 FlowSpec VPN decoding |
| `declare family ipv6 flowspec-vpn decode` | Claim IPv6 FlowSpec VPN decoding |
| `declare rfc 8955` | RFC reference |
| `declare rfc 8956` | RFC reference |
| `declare encoding hex` | Wire encoding |

### JSON Format

FlowSpec NLRI as JSON:

| Field | Type | Description |
|-------|------|-------------|
| `family` | string | e.g., `"ipv4/flowspec"` |
| `components` | array | List of component objects |

Component object:

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Component type name |
| `prefix` | string | For destination/source (e.g., `"10.0.0.0/24"`) |
| `values` | array | For numeric components |
| `operators` | array | Operator specifications |

Example JSON:

| Input (text) | JSON Output |
|--------------|-------------|
| `destination 10.0.0.0/24 protocol 6` | `{"family":"ipv4/flowspec","components":[{"type":"destination","prefix":"10.0.0.0/24"},{"type":"protocol","values":[6]}]}` |

### Decode Mode Protocol

| Direction | Format | Example |
|-----------|--------|---------|
| Request | `decode nlri <family> <hex>` | `decode nlri ipv4/flowspec 0701180a0000` |
| Response | `decoded json <json>` | `decoded json {"family":"ipv4/flowspec",...}` |

### CLI Integration

| Command | Purpose |
|---------|---------|
| `ze bgp plugin flowspec` | Run plugin (normal mode) |
| `ze bgp plugin flowspec --decode` | Run in decode mode |
| `ze bgp plugin flowspec --yang` | Output YANG schema |
| `ze bgp decode --plugin flowspec --nlri ipv4/flowspec <hex>` | Decode via CLI |

## Files to Modify

- `internal/plugin/inprocess.go` - register flowspec in internalPluginRunners
- `internal/plugin/bgp/nlri/nlri.go` - remove FlowSpec family constants (or keep as aliases)
- `internal/plugin/bgp/nlri/other.go` - remove FlowSpec SAFI if present
- `cmd/ze/bgp/bgp.go` - add `plugin flowspec` subcommand

## Files to Create

- `internal/plugin/flowspec/flowspec.go` - plugin main
- `internal/plugin/flowspec/parse.go` - wire parsing
- `internal/plugin/flowspec/encode.go` - wire encoding
- `internal/plugin/flowspec/json.go` - JSON conversion
- `internal/plugin/flowspec/components.go` - component types
- `internal/plugin/flowspec/flowspec_test.go` - unit tests
- `cmd/ze/bgp/plugin_flowspec.go` - CLI entry

## Files to Delete

- `internal/plugin/bgp/nlri/flowspec.go` - moved to plugin
- `internal/plugin/bgp/nlri/flowspec_test.go` - moved to plugin

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create plugin directory** - `internal/plugin/flowspec/`
   → **Review:** Directory structure matches hostname plugin?

2. **Move flowspec.go** - Copy to `internal/plugin/flowspec/components.go`
   → **Review:** All types and constants moved?

3. **Create parse.go** - Extract parsing functions
   → **Review:** Public API clean? No circular deps?

4. **Create encode.go** - Extract encoding functions
   → **Review:** WriteTo methods work standalone?

5. **Create json.go** - Add JSON marshal/unmarshal
   → **Review:** JSON format documented? Round-trip works?

6. **Create flowspec.go** - Plugin main with decode mode
   → **Review:** Follows hostname pattern? Stage 1 declarations correct?

7. **Move tests** - Copy flowspec_test.go, adapt imports
   → **Review:** All tests still pass with new package?

8. **Write decode mode tests** - Test decode nlri protocol
   → **Review:** All 13 component types tested?

9. **Run tests** - Verify PASS
   → **Review:** Coverage maintained?

10. **Create CLI entry** - `cmd/ze/bgp/plugin_flowspec.go`
    → **Review:** Matches hostname pattern? Flags correct?

11. **Register in inprocess.go** - Add to internalPluginRunners
    → **Review:** Logger configured? Name correct?

12. **Delete original files** - Remove from nlri/
    → **Review:** No remaining references? No broken imports?

13. **Fix import errors** - Update any code that imported flowspec from nlri
    → **Review:** All imports updated? No cycles?

14. **Create functional test** - `test/data/decode/flowspec-plugin.ci`
    → **Review:** Tests CLI decode path?

15. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero errors? No regressions?

16. **Final self-review** - Before claiming done:
    - All FlowSpec functionality preserved
    - No code duplication
    - Tests comprehensive
    - JSON format documented

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFlowSpecDecodeMode` | `internal/plugin/flowspec/flowspec_test.go` | Decode mode protocol | ✅ |
| `TestFlowSpecJSONRoundTrip` | `internal/plugin/flowspec/flowspec_test.go` | JSON ↔ struct | ✅ |
| `TestFlowSpecWireRoundTrip` | `internal/plugin/flowspec/flowspec_test.go` | wire ↔ struct | ✅ |
| `TestFlowSpecAllComponents` | `internal/plugin/flowspec/flowspec_test.go` | All 13 component types | ✅ |
| `TestFlowSpecVPN` | `internal/plugin/flowspec/flowspec_test.go` | VPN variant with RD | ✅ |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Component type | 1-13 | 13 | 0 | 14 |
| Prefix length IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix length IPv6 | 0-128 | 128 | N/A | 129 |
| Port | 0-65535 | 65535 | N/A | N/A (16-bit) |
| DSCP | 0-63 | 63 | N/A | 64 |
| Protocol | 0-255 | 255 | N/A | N/A (8-bit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-decode` | `test/data/decode/flowspec-plugin.ci` | Decode FlowSpec NLRI via CLI | ✅ |
| `flowspec-roundtrip` | `test/data/encode/flowspec-plugin.ci` | Encode then decode FlowSpec | |

## Design Decisions

### Why Move (Not Copy)?

| Option | Pros | Cons |
|--------|------|------|
| Copy then delete | Safe transition | Temporary duplication |
| Move directly | Clean, no duplication | Must fix all refs at once |

**Decision:** Move directly. FlowSpec is self-contained, no partial migration needed.

### JSON Format Design

| Option | Pros | Cons |
|--------|------|------|
| Flat key-value | Simple | Loses structure |
| Nested components | Preserves structure | More complex |
| Wire-like format | Close to RFC | Not human friendly |

**Decision:** Nested components. Matches RFC structure, human readable.

## RFC Documentation

### Reference Comments
- RFC 8955 Section 4 - FlowSpec NLRI encoding
- RFC 8955 Section 4.2 - Component ordering requirement
- RFC 8955 Section 4.2.1 - Numeric operator format
- RFC 8956 Section 3 - IPv6 FlowSpec extensions

### Constraint Comments
- Component ordering MUST be by type (RFC 8955 Section 4.2.2)
- ADD-PATH not supported for FlowSpec families
- IPv6 offset field only valid for type 1 and 2 components

## Implementation Summary (Phase 1)

### What Was Implemented
- `internal/plugin/flowspec/plugin.go` - Plugin with decode mode and startup protocol
- `internal/plugin/flowspec/plugin_test.go` - Unit tests (decode, boundary, VPN)
- `cmd/ze/bgp/plugin_flowspec.go` - CLI entry point
- `test/decode/flowspec-plugin.ci` - Functional test
- Auto-invoke for flowspec families in `pluginFamilyMap`
- Registered in `internalPluginRunners` for in-process execution

### Bugs Found/Fixed
- Wire encoding in tests: flow label op byte 0xa1 (not 0x91) for 4-byte values
- Wire encoding in tests: IPv6 VPN length 0x0f (not 0x11)
- Missing RD verification in VPN test assertions
- Dead code in `lookupFamilyPlugin` (speculative loop removed)

### Design Insights
- Plugin is thin wrapper over existing `nlri.ParseFlowSpec` / `nlri.ParseFlowSpecVPN`
- No code moved from nlri package - reused via import
- Auto-invoke simplifies CLI: `ze bgp decode` auto-uses flowspec plugin for flowspec families

### Deviations from Plan
- Did NOT move flowspec.go from nlri package (reused existing code instead)
- Plugin is decode-only for now (full event handling not implemented)
- Files to Delete section not executed (code reused, not moved)

## Checklist (Phase 1)

### 🏗️ Design
- [x] No premature abstraction (3+ concrete use cases exist?)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (each component does ONE thing?)
- [x] Explicit behavior (no hidden magic or conventions?)
- [x] Minimal coupling (components isolated, dependencies minimal?)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)
- [x] Boundary tests cover all numeric inputs
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code
- [x] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together

---

# Phase 2: Engine FlowSpec Isolation

## Task

Make the engine FlowSpec-agnostic. Only the plugin knows FlowSpec semantics.

**Architecture:**
- Engine passes raw NLRI bytes via API
- Plugin handles encode (text→wire) and decode (wire→JSON)
- Engine uses `WireNLRI` for FlowSpec, never parses components
- FlowSpec types move from `nlri/` to plugin

## Phase 2 Status

| Component | Status |
|-----------|--------|
| Plugin decode mode | ✅ Done (Phase 1) |
| Plugin encode mode | ✅ Added |
| Engine text parsing | ✅ Uses direct Go call (in-process shortcut) |
| FlowSpec types | ✅ Moved to `internal/plugin/flowspec/types.go` |
| `nlri/flowspec.go` | ✅ Deleted |

## Plugin Invocation Design

### Long-Lived Plugin Architecture

Family plugins (like flowspec) run as **long-lived goroutines** (or subprocesses for external plugins).
The plugin starts once and handles all encode/decode requests for its registered families.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ENGINE                                          │
│                                                                              │
│   update_text.go                           Plugin Registry                   │
│   ┌─────────────┐                         ┌─────────────────┐               │
│   │ parse nlri  │ ──── lookup family ───► │ ipv4/flowspec   │───┐           │
│   │ ipv4/flow.. │                         │ ipv6/flowspec   │   │           │
│   └─────────────┘                         │ ipv4/flowspec-vpn│   │           │
│                                           │ ipv6/flowspec-vpn│   │           │
│                                           └─────────────────┘   │           │
│                                                                  │           │
│   ┌──────────────────────────────────────────────────────────────┼──────┐   │
│   │                     io.Pipe (stdin/stdout)                   │      │   │
│   └──────────────────────────────────────────────────────────────┼──────┘   │
│                                                                  │           │
│   ┌──────────────────────────────────────────────────────────────▼──────┐   │
│   │                    FLOWSPEC PLUGIN (goroutine)                      │   │
│   │                                                                      │   │
│   │  Startup: declare family ipv4 flowspec decode                       │   │
│   │           declare family ipv6 flowspec decode                       │   │
│   │           declare family ipv4 flowspec-vpn decode                   │   │
│   │           declare family ipv6 flowspec-vpn decode                   │   │
│   │           declare done                                               │   │
│   │                                                                      │   │
│   │  Loop:    encode nlri ipv4/flowspec destination 10.0.0.0/24 ...     │   │
│   │           → encoded hex 0701180A...                                  │   │
│   │                                                                      │   │
│   │           decode nlri ipv4/flowspec 0701180A...                      │   │
│   │           → decoded json {"destination":[["10.0.0.0/24/0"]]}        │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Startup Protocol

1. **Engine starts plugin** (goroutine + io.Pipe, or fork for external)
2. **Plugin declares families** it handles:
   ```
   declare family ipv4 flowspec decode
   declare family ipv4 flowspec encode
   declare family ipv6 flowspec decode
   declare family ipv6 flowspec encode
   ...
   declare done
   ```
3. **Engine registers plugin** in family→plugin map
4. **Plugin enters request loop** - handles encode/decode requests

### Request/Response Protocol

| Direction | Format | Example |
|-----------|--------|---------|
| Encode request | `encode nlri <family> <args>` | `encode nlri ipv4/flowspec destination 10.0.0.0/24` |
| Encode response | `encoded hex <bytes>` | `encoded hex 0701180A0000` |
| Decode request | `decode nlri <family> <hex>` | `decode nlri ipv4/flowspec 0701180A0000` |
| Decode response | `decoded json <json>` | `decoded json {"destination":[["10.0.0.0/24/0"]]}` |
| Error | `encoded error <msg>` | `encoded error invalid prefix` |

### Benefits

- **Single plugin instance** - no per-request overhead
- **Language agnostic** - same protocol for Go, Python, Rust
- **Hot-swappable** - restart plugin without engine restart
- **Testable** - can test plugin protocol independently

### Current Status

✅ **Phase 4 Complete** - Generic NLRI routing implemented.

**Architecture:**
1. Flowspec plugin handles encode/decode in event loop
2. `LookupFamily()` returns plugin name for any family
3. Plugin protocol works (encode/decode requests via pipe)
4. Generic `server.EncodeNLRI()` and `server.DecodeNLRI()` for external plugins
5. In-process plugins (flowspec) called directly for efficiency

---

## Phase 4: Generic NLRI Routing (DONE)

### Design

```
For external plugins:
update_text.go → server.EncodeNLRI(family, args)
                        │
                        ▼
               registry.LookupFamily(family) → plugin name
                        │
                        ▼
               plugin.SendRequest("encode nlri <family> <args>")

For in-process plugins (flowspec):
update_text.go → flowspec.EncodeFlowSpecComponents(family, args)
                 (direct call for efficiency)
```

### What Was Implemented

1. **Generic `EncodeNLRI(family, args)` method on Server**
   - Looks up plugin via `registry.LookupFamily()`
   - Sends request via pipe, returns wire bytes
   - Returns error if no plugin registered or server not configured
   - Includes nil checks for registry and procManager

2. **Generic `DecodeNLRI(family, hex)` method on Server**
   - Looks up plugin via `registry.LookupFamily()`
   - Sends request via pipe, returns JSON
   - Includes nil checks for registry and procManager

3. **Fixed "encode" family registration**
   - `declare family <afi> <safi> encode` now registers the family
   - Previously "encode" was silently ignored, only "decode" worked
   - Both encode and decode register to same map (deduplication)

4. **Removed FlowSpec-specific code:**
   - `FlowSpecEncoder` variable removed
   - `EncodeFlowSpecViaPlugin()` removed
   - `SetupFlowSpecEncoder()` removed

5. **Update `update_text.go`**
   - FlowSpec encoding calls `flowspec.EncodeFlowSpecComponents()` directly
   - No indirection for in-process plugins (better performance)
   - External plugins would use `server.EncodeNLRI()`

6. **Unit tests added**
   - `TestEncodeNLRI_NotConfigured`, `TestEncodeNLRI_NoPlugin`
   - `TestDecodeNLRI_NotConfigured`, `TestDecodeNLRI_NoPlugin`
   - `TestFamilyRegistrationWithEncode`, `TestFamilyRegistrationBothEncodeAndDecode`
   - `TestFamilyAllCannotEncode`

### Files Modified

| File | Change |
|------|--------|
| `server.go` | Added `EncodeNLRI()`, `DecodeNLRI()` with nil checks, removed FlowSpec methods |
| `update_text.go` | Removed `FlowSpecEncoder`, calls flowspec package directly |
| `registration.go` | Fixed parseFamily to handle "encode" keyword same as "decode" |
| `server_test.go` | Added tests for EncodeNLRI/DecodeNLRI error cases |
| `registration_family_test.go` | Added tests for "encode" keyword registration |
| `docs/architecture/api/process-protocol.md` | Updated Engine Routing section |

### Phase 4 Checklist

- [x] Generic `EncodeNLRI(family, args)` method
- [x] Generic `DecodeNLRI(family, hex)` method
- [x] `FlowSpecEncoder` variable removed
- [x] `update_text.go` uses direct call for in-process plugins
- [x] Works for any family plugin (not just FlowSpec)
- [x] Nil pointer checks added
- [x] "encode" family registration fixed
- [x] Unit tests for new methods

## RD Syntax

The current syntax places `rd` before `add`:

```
nlri ipv4/flowspec-vpn rd 65000:100 add destination 10.0.0.0/24
```

This is consistent with other VPN families and is the documented API.

## Encode Mode Protocol

| Direction | Format | Example |
|-----------|--------|---------|
| Request | `encode nlri <family> <components>` | `encode nlri ipv4/flowspec destination 10.0.0.0/24 protocol 6` |
| Response | `encoded hex <bytes>` | `encoded hex 0701180A00000381068150` |
| Error | `encoded error <msg>` | `encoded error invalid prefix` |

## Phase 2 Implementation Steps

### Step 1: Plugin encode mode ✅
- Added `handleEncodeNLRI` to plugin
- Parses text components (destination, protocol, etc.)
- Returns hex wire bytes

### Step 2: Engine uses direct shortcut ✅
- `update_text.go` calls `flowspec.EncodeFlowSpecComponents()` directly
- This is the in-process shortcut (no subprocess overhead)
- External plugins would use subprocess + protocol

### Step 3: Move types to plugin ✅
- Types moved to `internal/plugin/flowspec/types.go`
- Plugin is self-contained

### Step 4: Delete nlri/flowspec.go ✅
- Verified no flowspec files in nlri/
- All FlowSpec code now in plugin package

### Step 5: Verify ✅
- `make verify` passes
- All FlowSpec encoding/decoding works

## Phase 2 Checklist

### 🏗️ Design
- [x] Plugin handles both encode and decode
- [x] Engine uses WireNLRI for FlowSpec
- [x] FlowSpec parsing isolated in plugin (direct shortcut)
- [x] Types moved to plugin package

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Completion
- [x] FlowSpec types removed from nlri/
- [x] Engine uses plugin for FlowSpec (direct shortcut)
- [ ] All files committed together

---

## Phase 5: FlowSpec Family Advertisement in OPEN

### Problem Statement

**Gap identified:** Plugin family declarations (Stage 1) are NOT linked to OPEN capability advertisement.

| What plugin does | What engine does | Gap |
|------------------|------------------|-----|
| `declare family ipv4 flow decode` | Routes FlowSpec NLRIs to plugin | ✅ Works |
| Plugin loads | Auto-add Multiprotocol(1,133) to OPEN | ❌ Not implemented |

**Current requirement:** Config must explicitly specify FlowSpec families even when FlowSpec plugin is loaded:

```
peer 192.0.2.1 {
    family {
        ipv4/flow;      # Must be explicit
        ipv6/flow;      # Must be explicit
    }
}
```

### Architecture Analysis

**Current Flow:**

```
Config File                    PeerSettings                 OPEN Message
────────────────              ────────────────              ─────────────
family {                      Capabilities: []cap{         Multiprotocol
  ipv4/flow;        ───►        &Multiprotocol{      ───►  AFI=1, SAFI=133
}                               AFI:1, SAFI:133}}
```

**Missing Link:**

```
Plugin Stage 1                Plugin Registry              PeerSettings
────────────────              ────────────────             ─────────────
declare family                families["ipv4/flow"]        Capabilities: ???
ipv4 flow decode    ───►      = "flowspec"                 (no auto-add)
```

### Key Source Files

| File | Role |
|------|------|
| `internal/plugin/flowspec/plugin.go:66-81` | Stage 1 family declarations |
| `internal/plugin/registration.go:156-157` | Registry stores family→plugin mapping |
| `internal/plugin/bgp/reactor/peersettings.go:215` | `Capabilities []capability.Capability` |
| `internal/plugin/bgp/capability/capability.go:247-272` | `Multiprotocol` struct |
| `internal/config/bgp.go` | Config → PeerSettings conversion |

### RFC Requirements

| RFC | Requirement |
|-----|-------------|
| RFC 4760 Section 8 | Multiprotocol capability (Code 1) advertises AFI/SAFI support |
| RFC 8955 Section 7 | FlowSpec uses AFI=1 (IPv4) or AFI=2 (IPv6), SAFI=133 or SAFI=134 |
| RFC 5492 Section 4 | Capabilities must be advertised in OPEN for negotiation |

### Implementation Options

#### Option A: Explicit Config (Current Design - Document Only)

**Keep current behavior.** Config explicitly lists families to advertise.

**Pros:**
- Explicit is better than implicit
- No magic behavior
- User controls exactly what's negotiated
- Plugin loading doesn't change OPEN behavior

**Cons:**
- User must remember to add families when loading plugin
- Error-prone: plugin loaded but families not configured

**Action:** Document this requirement clearly.

#### Option B: Auto-Inject Multiprotocol Capabilities (Recommended)

**Plugin declares families → Engine auto-adds Multiprotocol capabilities to OPEN.**

**Mechanism:**

1. Plugin Stage 1: `declare family ipv4 flow decode` (existing)
2. Plugin Stage 3: `capability multiprotocol ipv4 flow` (new - auto-generated)
3. Engine merges plugin capabilities into `PeerSettings.Capabilities`

**Changes Required:**

| File | Change |
|------|--------|
| `internal/plugin/flowspec/plugin.go` | Add Stage 3 capability injection |
| `internal/plugin/bgp/reactor/peer.go` | Merge plugin caps into OPEN |
| `internal/plugin/registration.go` | Track families→Multiprotocol mapping |

**Stage 3 Addition to FlowSpec Plugin:**

```
# In doStartupProtocol(), after Stage 2 config:

# Stage 3: Inject Multiprotocol capabilities for declared families
capability hex 1 00010085  # AFI=1, SAFI=133 (ipv4/flowspec)
capability hex 1 00010086  # AFI=1, SAFI=134 (ipv4/flowspec-vpn)
capability hex 1 00020085  # AFI=2, SAFI=133 (ipv6/flowspec)
capability hex 1 00020086  # AFI=2, SAFI=134 (ipv6/flowspec-vpn)
capability done
```

**Pros:**
- Plugin self-describes its requirements
- No config changes needed when loading plugin
- Follows plugin protocol (Stage 3 is for this purpose)

**Cons:**
- Implicit behavior (plugin loading changes OPEN)
- May conflict with explicit config families

#### Option C: Validation Only

**Validate that advertised families have a decoding plugin registered.**

**Mechanism:**

1. Config specifies families (as now)
2. At peer startup, check: `registry.LookupFamily(family) != ""`
3. Warn/error if family advertised but no plugin can decode it

**Pros:**
- Catches misconfiguration early
- No behavior change, just validation
- Works with any family, not just FlowSpec

**Cons:**
- Doesn't solve "plugin loaded but family not configured" case
- Just validation, not auto-injection

### Recommendation

**Implement Option B + Option C:**

1. **FlowSpec plugin injects Multiprotocol capabilities** in Stage 3
2. **Engine validates** advertised families have decoders
3. **Document** the behavior clearly

### Implementation (Option B: Infer from Decode) ✅ DONE

**Design Decision:** Instead of explicit `capability hex 1` injection, the engine
automatically adds Multiprotocol capabilities for families that have `decode` plugins.

**Rationale:**
- Simpler: Plugin declares decode → Engine infers capability
- No duplicate capability code issues (multiple cap code 1)
- Less protocol overhead
- If plugin can decode a family, peers should be able to send it

**Implementation:**

1. **Registry tracks decode families**
   - `internal/plugin/registration.go:GetDecodeFamilies()` returns sorted list
   - Families sorted alphabetically for deterministic OPEN ordering

2. **Session injects capabilities in sendOpen()**
   - `internal/plugin/bgp/reactor/session.go:sendOpen()` calls `pluginFamiliesGetter`
   - Converts family strings to Multiprotocol capabilities
   - Deduplicates: skips families already in config

3. **FlowSpec plugin simplified**
   - Removed `capability hex 1` lines from Stage 3
   - Engine infers from `declare family ... decode` in Stage 1

4. **Functional test added**
   - `test/plugin/flowspec-open-capability.ci`
   - Verifies OPEN contains flowspec Multiprotocol caps without explicit family config

### Files Modified

| File | Change |
|------|--------|
| `internal/plugin/registration.go` | Added `GetDecodeFamilies()` with sorting |
| `internal/plugin/server.go` | Added `GetDecodeFamilies()` wrapper |
| `internal/plugin/bgp/reactor/session.go` | Added `pluginFamiliesGetter`, inject caps in `sendOpen()` |
| `internal/plugin/bgp/reactor/peer.go` | Added `getPluginFamilies()`, wired up callback |
| `internal/plugin/flowspec/plugin.go` | Removed `capability hex 1` lines |
| `internal/plugin/flowspec/plugin_test.go` | Updated test expectations |
| `docs/architecture/api/process-protocol.md` | Documented automatic capability injection |

### Phase 5 Checklist

- [x] Engine auto-adds Multiprotocol for decode families
- [x] FlowSpec plugin no longer sends `capability hex 1` lines
- [x] Deduplication: config families not duplicated by plugin
- [x] Sorted ordering for deterministic OPEN
- [x] Functional test: `test/plugin/flowspec-open-capability.ci` (auto-load for known family)
- [x] Functional test: `test/plugin/family-no-plugin-failure.ci` (failure for unknown family)
- [x] Functional test: `test/plugin/explicit-plugin-precedence.ci` (--plugin prevents auto-load)
- [x] Functional test: `test/plugin/explicit-plugin-config.ci` (config plugin prevents auto-load)
- [x] Documentation updated: `docs/architecture/api/process-protocol.md`
- [x] Unit tests: `TestSessionSendOpenWithPluginFamilies`, `TestSessionSendOpenPluginFamiliesDedup`
