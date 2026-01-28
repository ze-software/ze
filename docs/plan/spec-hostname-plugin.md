# Spec: hostname-plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-plugin-yang-discovery.md` - framework this depends on
4. `internal/plugin/gr/gr.go` - plugin pattern to follow

## Dependencies

**Requires:** `spec-plugin-yang-discovery.md` must be implemented first.

## Task

Create hostname capability plugin (draft-walton-bgp-hostname, code 73) with its own YANG schema, using the plugin discovery framework for auto-injection.

**Rationale:** FQDN/hostname capability is not essential to BGP operation and should be a plugin, not core code. Plugin provides its own YANG and is auto-injected when config uses hostname paths.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/capabilities.md` - capability wire format
- [ ] `docs/architecture/api/capability-contract.md` - plugin capability injection

### Source Code
- [ ] `internal/plugin/gr/gr.go` - plugin pattern to follow
- [ ] `cmd/ze/bgp/plugin_gr.go` - CLI entry pattern
- [ ] `internal/plugin/bgp/capability/capability.go` - FQDN struct (lines 629-707)

**Key insights:**
- FQDN capability code is 73
- Wire format: hostname-len (1) + hostname + domain-len (1) + domain
- GR plugin is the pattern for capability plugins

## Config Syntax

| Syntax | Example | Auto-inject? |
|--------|---------|--------------|
| Legacy | `peer X { host-name foo; domain-name bar.com; }` | Yes |
| New | `peer X { capability { hostname { host foo; domain bar.com; } } }` | Yes |

Both syntaxes produce identical wire encoding.

## Design

### YANG Schema (ze-hostname.yang)

| Element | Value |
|---------|-------|
| module | `ze-hostname` |
| namespace | `urn:ze:hostname` |
| prefix | `hostname` |
| augments | `/ze-bgp:bgp/ze-bgp:peer/ze-bgp:capability` |

**Schema structure:**

| Path | Type | Description |
|------|------|-------------|
| `capability/hostname` | container | FQDN capability container |
| `capability/hostname/host` | leaf string | System hostname (max 255 bytes) |
| `capability/hostname/domain` | leaf string | Domain name (max 255 bytes) |

**Legacy paths** (also in ze-hostname.yang for trigger detection):

| Path | Type | Description |
|------|------|-------------|
| `peer/host-name` | leaf string | Legacy syntax |
| `peer/domain-name` | leaf string | Legacy syntax |

### Plugin Protocol

Plugin declares config patterns for both syntaxes:

| Declaration | Purpose |
|-------------|---------|
| `declare conf peer * capability hostname host <host:.*>` | New syntax |
| `declare conf peer * capability hostname domain <domain:.*>` | New syntax |
| `declare conf peer * host-name <host:.*>` | Legacy syntax |
| `declare conf peer * domain-name <domain:.*>` | Legacy syntax |

Config delivery normalizes to `host`/`domain` regardless of syntax used.

### Wire Encoding

| Field | Size | Description |
|-------|------|-------------|
| Hostname Length | 1 byte | Length of hostname (0-255) |
| Hostname | variable | UTF-8 hostname |
| Domain Length | 1 byte | Length of domain (0-255) |
| Domain Name | variable | UTF-8 domain name |

Example: hostname="router1", domain="example.com"
Wire: `07 726F7574657231 0B 6578616D706C652E636F6D`

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHostnamePluginParseConfig` | `internal/plugin/hostname/hostname_test.go` | Config parsing | |
| `TestHostnamePluginEncode` | `internal/plugin/hostname/hostname_test.go` | Wire encoding | |
| `TestHostnamePluginMultiplePeers` | `internal/plugin/hostname/hostname_test.go` | Per-peer config | |
| `TestHostnamePluginEmptyValues` | `internal/plugin/hostname/hostname_test.go` | Missing values | |
| `TestHostnamePluginLegacySyntax` | `internal/plugin/hostname/hostname_test.go` | Legacy parsing | |
| `TestHostnamePluginYANG` | `internal/plugin/hostname/hostname_test.go` | --yang output | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Above |
|-------|-------|------------|---------------|
| Hostname length | 0-255 | 255 bytes | 256 bytes (truncate with warning) |
| Domain length | 0-255 | 255 bytes | 256 bytes (truncate with warning) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `hostname-new-syntax` | `test/encode/hostname-new-syntax.ci` | New syntax works | |
| `hostname-legacy` | `test/encode/hostname.ci` | Legacy syntax works | |
| `hostname-exabgp` | `test/exabgp-compat/encoding/conf-hostname.ci` | ExaBGP compat | |

## Files to Create

- `internal/plugin/hostname/hostname.go` - Plugin implementation
- `internal/plugin/hostname/hostname_test.go` - Unit tests
- `internal/plugin/hostname/embed.go` - Embedded YANG
- `internal/plugin/hostname/ze-hostname.yang` - YANG schema
- `cmd/ze/bgp/plugin_hostname.go` - CLI entry with `--yang` flag
- `test/encode/hostname-new-syntax.ci` - Functional test

## Files to Modify

- `cmd/ze/bgp/plugin.go` - Add "hostname" subcommand
- `internal/plugin/bgp/schema/ze-bgp.yang` - Remove host-name/domain-name (moved to plugin)
- `internal/config/loader.go` - Remove manual FQDN injection
- `internal/config/bgp.go` - Remove Hostname/DomainName fields
- `internal/exabgp/migrate.go` - Add `NeedsHostnamePlugin()`

## Files to Keep

- `internal/plugin/bgp/capability/capability.go` - Keep FQDN struct for parsing peer OPEN

## Implementation Steps

### Phase 1: Plugin

1. **Write unit tests** - `internal/plugin/hostname/hostname_test.go`
   → **Review:** Edge cases covered?

2. **Run tests** - Verify FAIL

3. **Create YANG** - `internal/plugin/hostname/ze-hostname.yang`
   → **Review:** Valid YANG? Includes legacy paths?

4. **Implement plugin** - `internal/plugin/hostname/hostname.go`
   - Follow GR plugin pattern
   - Wire encoding from FQDN.Pack()
   → **Review:** Matches GR structure?

5. **Run tests** - Verify PASS

### Phase 2: CLI

6. **Create CLI** - `cmd/ze/bgp/plugin_hostname.go`
   - `--yang` flag outputs embedded YANG
   → **Review:** --yang works?

7. **Register command** - Update `cmd/ze/bgp/plugin.go`

### Phase 3: Integration

8. **Remove from core** - Edit loader.go, bgp.go
   - Remove manual FQDN injection
   - Remove config fields (plugin handles delivery)
   → **Review:** No broken references?

9. **Update ze-bgp.yang** - Remove host-name/domain-name leaves
   → **Review:** Schema still valid?

10. **ExaBGP migration** - Add `NeedsHostnamePlugin()` to migrate.go

### Phase 4: Test

11. **Functional tests** - Create/update .ci files

12. **Verify all** - `make verify`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Functional tests pass

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
