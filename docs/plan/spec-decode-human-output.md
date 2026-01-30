# Spec: 03 - Human-Readable Decode Output

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/bgp/decode.go` - current decode implementation
4. `internal/plugin/text.go` - existing text formatters (FormatOpen, etc.)
5. `internal/plugin/decode.go` - DecodedOpen, DecodedCapability structs
6. `docs/architecture/api/process-protocol.md` - plugin decode protocol

## Task

Add human-readable output format to `ze bgp decode` command as the default, with `--json` flag to get current ExaBGP-compatible JSON output. Extend plugin decode API to support both formats.

**Current behavior:**
- `ze bgp decode --open <hex>` → Always outputs ExaBGP JSON
- Plugin protocol: `decode capability <code> <hex>` → `decoded json <json>`

**Target behavior:**
- `ze bgp decode --open <hex>` → Human-readable output (default)
- `ze bgp decode --open --json <hex>` → ExaBGP JSON (current behavior)
- Same pattern for `--update` and `--nlri`
- Plugin protocol extended with format argument

## Required Reading

### Architecture Docs
- [ ] `cmd/ze/bgp/decode.go` - current decode implementation
- [ ] `internal/plugin/text.go` - existing text formatters
- [ ] `internal/plugin/decode.go` - DecodedOpen, DecodedCapability structs
- [ ] `docs/architecture/api/process-protocol.md` - plugin decode protocol

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 message formats

**Key insights:**
- Existing `FormatOpen()` in text.go outputs: `peer <ip> <direction> open <msg-id> asn <asn> router-id <id> hold-time <t> [cap <code> <name> <value>]...`
- For decode command, there is no peer context or message ID - format should be simpler
- Human-readable output should be easy to read at a glance
- Plugin decode protocol needs format as first argument after "decode"

## Current Behavior

**Source files read:**
- [ ] `cmd/ze/bgp/decode.go` - decodeHexPacket() returns JSON string
- [ ] `internal/plugin/text.go` - FormatOpen(), FormatNotification() patterns

**Behavior to preserve:**
- All JSON output format when `--json` flag is used
- Error handling returns JSON errors

**Behavior to change:**
- Default output changes from JSON to human-readable
- Add `--json` flag to get previous behavior

## Human-Readable Output Format

### OPEN Message

```
BGP OPEN Message
  Version:     4
  ASN:         65533
  Hold Time:   180 seconds
  Router ID:   10.0.0.2
  Capabilities:
    multiprotocol        ipv4/unicast
    asn4                 65533
    fqdn                 my-host-name.my-domain-name.com
```

### UPDATE Message

```
BGP UPDATE Message
  Attributes:
    origin               igp
    as-path              65001 65002 65003
    next-hop             10.0.0.1
    local-preference     100
    med                  50
  Announced (ipv4/unicast):
    10.0.0.0/24
    10.0.1.0/24
  Withdrawn (ipv4/unicast):
    10.0.2.0/24
```

### NLRI Only

```
FlowSpec NLRI (ipv4/flow):
  destination          10.0.0.0/24
  protocol             6 (TCP)
  destination-port     =80
```

### Errors

```
Error: invalid hex: encoding/hex: odd length hex string
```

## Plugin Decode API Extension

### Current Protocol (JSON only)

```
decode capability <code> <hex>   →  decoded json <json>
decode nlri <family> <hex>       →  decoded json <json>
```

### Extended Protocol (format argument)

Format is the first argument after `decode`:

```
decode json capability <code> <hex>   →  decoded json <json>
decode text capability <code> <hex>   →  decoded text <lines>

decode json nlri <family> <hex>       →  decoded json <json>
decode text nlri <family> <hex>       →  decoded text <lines>
```

### Text Response Format

For `decode text`, response uses `decoded text` followed by the human-readable output.
Multi-line output uses literal newlines (plugins output one logical response per request):

```
decoded text BGP OPEN Message
  Version:     4
  ASN:         65533
  ...
```

### Backward Compatibility

Plugins that only support JSON can return `decoded unknown` for text requests.
The decode command will fall back to built-in formatting.

### Plugin Implementation Changes

Each plugin that supports decode needs to:
1. Parse format argument (`json` or `text`) after `decode`
2. For `text`: return `decoded text <human-readable>`
3. For `json`: return `decoded json <json>` (existing behavior)
4. Return `decoded unknown` if format not supported

### Files Affected

| File | Change |
|------|--------|
| `cmd/ze/bgp/decode.go` | Pass format to plugin requests |
| `internal/plugin/hostname/hostname.go` | Add text format support |
| `internal/plugin/flowspec/plugin.go` | Add text format support |
| `docs/architecture/api/process-protocol.md` | Document extended protocol |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeOpenHuman` | `cmd/ze/bgp/decode_test.go` | Human-readable OPEN output | |
| `TestDecodeOpenJSON` | `cmd/ze/bgp/decode_test.go` | JSON OPEN output with --json | |
| `TestDecodeUpdateHuman` | `cmd/ze/bgp/decode_test.go` | Human-readable UPDATE output | |
| `TestDecodeUpdateJSON` | `cmd/ze/bgp/decode_test.go` | JSON UPDATE output with --json | |
| `TestDecodeNLRIHuman` | `cmd/ze/bgp/decode_test.go` | Human-readable NLRI output | |
| `TestDecodeNLRIJSON` | `cmd/ze/bgp/decode_test.go` | JSON NLRI output with --json | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `decode-open-human` | `test/decode/decode-human.ci` | Human-readable OPEN decode | |
| `decode-open-json` | `test/decode/decode-json.ci` | JSON OPEN decode with --json | |

## Files to Modify

- `cmd/ze/bgp/decode.go` - Add --json flag, human-readable formatters, pass format to plugins
- `internal/plugin/hostname/hostname.go` - Add text format decode support
- `internal/plugin/flowspec/plugin.go` - Add text format decode support
- `docs/architecture/api/process-protocol.md` - Document extended decode protocol

## Files to Create

- None (all changes in existing files)

## Implementation Steps

### Phase 1: CLI and Built-in Formatters

1. **Add --json flag** to cmdDecode()
   → **Review:** Flag parsing correct? Default is false (human)?

2. **Create formatOpenHuman()** function
   → **Review:** Output matches spec format? Indentation consistent?

3. **Create formatUpdateHuman()** function
   → **Review:** Handles all attribute types? Grouped by family?

4. **Create formatNLRIHuman()** function
   → **Review:** Handles FlowSpec, EVPN, BGP-LS?

5. **Update decodeHexPacket()** to accept outputJSON bool
   → **Review:** JSON format unchanged when outputJSON=true?

### Phase 2: Plugin Protocol Extension

6. **Update invokePluginDecodeRequest()** to pass format
   → **Review:** Sends `decode text capability` or `decode json capability`?

7. **Update hostname plugin** for text decode
   → **Review:** Returns `decoded text <human-readable>`?

8. **Update flowspec plugin** for text decode
   → **Review:** Returns `decoded text <human-readable>`?

9. **Update process-protocol.md** documentation
   → **Review:** New format documented clearly?

### Phase 3: Testing

10. **Write unit tests** for both output formats
    → **Review:** Both human and JSON paths tested?

11. **Write functional tests** for plugin decode
    → **Review:** Tests cover json and text formats?

12. **Verify all** - `make lint && make test && make functional`
    → **Review:** Zero errors?

## Design Decisions

### Why Human-Readable as Default?

| Option | Pros | Cons |
|--------|------|------|
| JSON default | Consistent with current behavior | Not human-friendly for debugging |
| Human default | Easy to read at a glance | Breaking change (mitigated by --json) |

**Decision:** Human default. The decode command is primarily for debugging/inspection. Scripts should use `--json`.

### Output Style

| Option | Example | Pros | Cons |
|--------|---------|------|------|
| Single line | `OPEN asn=65533 rid=10.0.0.2` | Compact | Hard to scan |
| Structured | See format above | Easy to read | More lines |

**Decision:** Structured multi-line. Optimized for human scanning.

## Checklist

### 🏗️ Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit behavior
- [ ] Minimal coupling
- [ ] Next-developer test

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
- [ ] RFC summaries read
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
