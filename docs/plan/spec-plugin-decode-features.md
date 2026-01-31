# Spec: Plugin Decode Features

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/cli/plugin-modes.md` - current plugin modes doc
4. `cmd/ze/bgp/plugin_evpn.go` - current NLRI plugin pattern
5. `cmd/ze/bgp/plugin_hostname.go` - current capability plugin

## Task

Redesign plugin CLI decode interface to:
1. Differentiate decode types: `--capa` (capabilities), `--nlri`
2. Add `--features` flag to query what a plugin can decode
3. Provide standard "not applicable" error for unsupported features
4. Update all plugins systematically

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - current plugin modes

### Source Files
- [ ] `cmd/ze/bgp/plugin_evpn.go` - NLRI plugin CLI
- [ ] `cmd/ze/bgp/plugin_flowspec.go` - NLRI plugin CLI
- [ ] `cmd/ze/bgp/plugin_hostname.go` - capability plugin CLI
- [ ] `cmd/ze/bgp/plugin_gr.go` - capability plugin CLI

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `cmd/ze/bgp/plugin_evpn.go` - uses `--json <hex>` / `--text <hex>` for NLRI decode
- [x] `cmd/ze/bgp/plugin_flowspec.go` - uses `--json <hex>` / `--text <hex>` for NLRI decode
- [x] `cmd/ze/bgp/plugin_hostname.go` - only has `--decode` for engine mode

**Behavior to preserve:**
- Engine decode mode (`--decode` flag) continues to work
- Engine mode (no flags) continues to work
- JSON and text output formats

**Behavior to change:**
- `--json <hex>` / `--text <hex>` → `--nlri <hex>` or `--capa <hex>` with `--text` flag
- Add `--features` to all plugins
- Standardize error messages for unsupported features

## Design

### Plugin Feature Matrix

| Plugin | --capa | --nlri | --features |
|--------|--------|--------|------------|
| hostname | ✓ FQDN (73) | ✗ | ✓ |
| gr | ✓ GR (64) | ✗ | ✓ |
| evpn | ✗ | ✓ l2vpn/evpn | ✓ |
| flowspec | ✗ | ✓ ipv4/flow etc | ✓ |
| rib | ✗ | ✗ | ✓ |
| rr | ✗ | ✗ | ✓ |

### CLI Interface

**Query features:**
```bash
ze bgp plugin evpn --features
# Output: nlri yang

ze bgp plugin hostname --features
# Output: capa yang

ze bgp plugin rib --features
# Output: (empty - no decode features)
```

**Decode NLRI (evpn, flowspec):**
```bash
# JSON output (default)
ze bgp plugin evpn --nlri 02210001252C...

# Text output
ze bgp plugin evpn --nlri 02210001252C... --text

# From stdin
ze bgp plugin evpn --nlri - --text
```

**Decode capabilities (hostname, gr):**
```bash
# JSON output (default)
ze bgp plugin hostname --capa 07726f7574657231...

# Text output
ze bgp plugin hostname --capa 07726f7574657231... --text

# From stdin
ze bgp plugin hostname --capa -
```

**Unsupported feature - standard error:**
```bash
ze bgp plugin hostname --nlri 02210001252C...
# stderr: error: plugin 'hostname' does not support --nlri (available: --capa)
# exit code: 1

ze bgp plugin evpn --capa 07726f7574657231...
# stderr: error: plugin 'evpn' does not support --capa (available: --nlri)
# exit code: 1
```

### Flag Changes

| Old | New | Notes |
|-----|-----|-------|
| `--json <hex>` | `--nlri <hex>` | For NLRI plugins (evpn, flowspec) |
| `--json <hex>` | `--capa <hex>` | For capability plugins (hostname, gr) |
| `--text <hex>` | `--text` (bool) | Output format modifier |
| `--decode` | `--decode` | Unchanged - engine protocol mode |
| (new) | `--features` | List supported decode features |

### Standard Error Format

```
error: plugin '<name>' does not support --<feature> (available: <list>)
```

Examples:
- `error: plugin 'hostname' does not support --nlri (available: --capa)`
- `error: plugin 'rib' does not support --nlri (available: none)`

### Features Output Format

Space-separated list of supported features:
- `nlri` - supports `--nlri <hex>`
- `capa` - supports `--capa <hex>`
- `yang` - supports `--yang`

Example: `nlri yang` or `capa yang` or empty string for plugins with no decode.

## Files to Modify

| File | Changes |
|------|---------|
| `cmd/ze/bgp/plugin_evpn.go` | `--json` → `--nlri`, `--text <hex>` → `--text` bool, add `--features` |
| `cmd/ze/bgp/plugin_flowspec.go` | `--json` → `--nlri`, `--text <hex>` → `--text` bool, add `--features` |
| `cmd/ze/bgp/plugin_hostname.go` | Add `--capa`, `--text`, `--features` |
| `cmd/ze/bgp/plugin_gr.go` | Add `--capa`, `--text`, `--features` |
| `cmd/ze/bgp/plugin_rib.go` | Add `--features` (returns empty) |
| `cmd/ze/bgp/plugin_rr.go` | Add `--features` (returns empty) |
| `internal/plugin/hostname/hostname.go` | Add `RunCLIDecode()` |
| `internal/plugin/gr/gr.go` | Add `RunCLIDecode()` if missing |
| `docs/architecture/cli/plugin-modes.md` | Update with new interface |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunCLIDecode` | `internal/plugin/hostname/hostname_test.go` | CLI decode works | |
| `TestFeaturesOutput` | `cmd/ze/bgp/plugin_test.go` | --features returns correct list | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-features` | `test/cli/plugin-features.ci` | Query plugin features | |
| `plugin-decode-nlri` | `test/cli/plugin-decode-nlri.ci` | Decode NLRI via --nlri | |
| `plugin-decode-capa` | `test/cli/plugin-decode-capa.ci` | Decode capability via --capa | |
| `plugin-unsupported` | `test/cli/plugin-unsupported.ci` | Standard error for unsupported | |

## Implementation Steps

### Phase 1: Update evpn plugin (reference implementation)

1. Change `--json <hex>` to `--nlri <hex>`
2. Change `--text <hex>` to `--text` bool flag
3. Add `--features` flag
4. Test: `ze bgp plugin evpn --features` → `nlri yang`
5. Test: `ze bgp plugin evpn --nlri <hex>` works
6. Test: `ze bgp plugin evpn --nlri <hex> --text` works
7. Test: `ze bgp plugin evpn --capa <hex>` → standard error

### Phase 2: Update flowspec plugin

1. Same changes as evpn
2. Verify `--family` flag still works with `--nlri`

### Phase 3: Add hostname CLI decode

1. Add `RunCLIDecode()` to `internal/plugin/hostname/hostname.go`
2. Add `--capa <hex>`, `--text`, `--features` to CLI
3. Test: `ze bgp plugin hostname --features` → `capa yang`
4. Test: `ze bgp plugin hostname --capa <hex>` works
5. Test: `ze bgp plugin hostname --nlri <hex>` → standard error

### Phase 4: Update gr plugin

1. Check if gr has decode capability
2. If yes: add `--capa`, `--text`, `--features`
3. If no: add only `--features` (returns `yang` or empty)

### Phase 5: Update rib/rr plugins

1. Add `--features` only (returns empty or minimal)
2. Standard error for any decode attempts

### Phase 6: Update documentation

1. Update `docs/architecture/cli/plugin-modes.md`
2. Verify `--help` output is consistent

### Phase 7: Verification

1. `make lint` passes
2. `make test` passes
3. `make functional` passes

## RFC Documentation

N/A - This is a CLI design change, not protocol.

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (simple flag changes)
- [ ] No speculative features (only what's needed now)
- [ ] Single responsibility (each flag does one thing)
- [ ] Explicit behavior (feature type in flag name)
- [ ] Minimal coupling (plugins independent)
- [ ] Next-developer test (--features makes capabilities discoverable)

### 🧪 TDD
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

### Documentation
- [ ] Required docs read
- [ ] `docs/architecture/cli/plugin-modes.md` updated
- [ ] Plugin --help updated with new usage

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
