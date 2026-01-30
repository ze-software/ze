# Spec: Encode JSON Format Support

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/flowspec/plugin.go` - handleEncodeRequest(), RunFlowSpecDecode()
4. `docs/architecture/api/process-protocol.md` - decode protocol (model for encode)

## Task

Add JSON input format support to plugin encode protocol for symmetry with decode.

**Current behavior:**
- `encode nlri <family> <text-args>` → text input only
- No way to encode from JSON representation

**Target behavior:**
- `encode nlri <family> <text-args>` → text input (backward compat, default)
- `encode text nlri <family> <text-args>` → text input (explicit)
- `encode json nlri <family> <json>` → JSON input (new)

This enables round-trip workflows: decode to JSON → modify → encode from JSON.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - Current decode format (model this)

### Source Files
- [ ] `internal/plugin/flowspec/plugin.go` - handleEncodeRequest(), EncodeFlowSpecComponents()
- [ ] `internal/plugin/flowspec/encode.go` - FlowSpec encoding implementation
- [ ] `internal/plugin/flowspec/decode.go` - JSON structure returned by decode (input for encode json)

**Key insights:**
- Decode returns JSON like: `{"destination":[["10.0.0.0/24/0"]],"protocol":[["=tcp"]]}`
- Encode should accept this same JSON format
- JSON must be minified (no spaces) for single-argument parsing

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/flowspec/plugin.go` - `handleEncodeRequest()` parses text components
- [ ] `internal/plugin/flowspec/plugin.go` - `RunFlowSpecDecode()` handles `encode nlri` without format

**Encode flow (current):**
```
encode nlri ipv4/flow destination 10.0.0.0/24 protocol 6
       │
       ▼
parseFlowSpecArgs(["destination", "10.0.0.0/24", "protocol", "6"])
       │
       ▼
encoded hex 0801180a0000038106
```

**Behavior to preserve:**
- `encode nlri <family> <args>` continues to work (text input)
- Response format unchanged: `encoded hex <hex>` or `encoded error <msg>`

**Behavior to add:**
- `encode json nlri <family> <json>` parses JSON input
- `encode text nlri <family> <args>` explicit text (same as no prefix)

## Protocol Extension

### Request Formats

| Request | Description |
|---------|-------------|
| `encode nlri <family> <args>` | Text input (default, backward compat) |
| `encode text nlri <family> <args>` | Text input (explicit) |
| `encode json nlri <family> <json>` | JSON input (new) |

### Response Formats (unchanged)

| Response | Description |
|----------|-------------|
| `encoded hex <hex>` | Success - wire bytes |
| `encoded error <msg>` | Failure - error message |

### JSON Input Format

JSON must match decode output format:
```json
{"destination":[["10.0.0.0/24/0"]],"protocol":[["=tcp"]]}
```

**Important:** JSON must be minified (no spaces) since protocol is space-delimited.

### Examples

**Text encode (current):**
```
encode nlri ipv4/flow destination 10.0.0.0/24
encoded hex 0501180a0000
```

**JSON encode (new):**
```
encode json nlri ipv4/flow {"destination":[["10.0.0.0/24/0"]]}
encoded hex 0501180a0000
```

**Round-trip:**
```
# Decode
decode json nlri ipv4/flow 0501180a0000
decoded json {"destination":[["10.0.0.0/24/0"]]}

# Encode (same JSON back)
encode json nlri ipv4/flow {"destination":[["10.0.0.0/24/0"]]}
encoded hex 0501180a0000
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeJSONFormat` | `internal/plugin/flowspec/plugin_test.go` | JSON input parsing | |
| `TestEncodeTextExplicit` | `internal/plugin/flowspec/plugin_test.go` | Explicit text format | |
| `TestEncodeJSONRoundTrip` | `internal/plugin/flowspec/plugin_test.go` | decode→encode round-trip | |
| `TestEncodeJSONInvalid` | `internal/plugin/flowspec/plugin_test.go` | Invalid JSON error | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| JSON encode | `test/plugin/flowspec-encode-json.ci` | Encode from JSON input | |

## Files to Modify

- `internal/plugin/flowspec/plugin.go` - Add format parsing to encode handlers
- `internal/plugin/flowspec/encode.go` - Add `EncodeFromJSON()` function
- `docs/architecture/api/process-protocol.md` - Document encode format specifier

## Files to Create

- `test/plugin/flowspec-encode-json.ci` - Functional test

## Implementation Steps

1. **Write unit tests** - Test JSON encode format

2. **Run tests** - Verify FAIL (paste output)

3. **Add format parsing to RunFlowSpecDecode()**
   - Check for `encode json` and `encode text` prefixes
   - Route to appropriate handler

4. **Implement EncodeFromJSON()**
   - Parse JSON structure
   - Convert to FlowSpec components
   - Reuse existing encoding logic

5. **Update handleEncodeRequest()**
   - Accept format parameter
   - Call EncodeFromJSON() for JSON format

6. **Run tests** - Verify PASS (paste output)

7. **Update documentation**

8. **Verify all** - `make lint && make test && make functional` (paste output)

## Design Decisions

### Why JSON Encode?

| Use Case | Without | With |
|----------|---------|------|
| Round-trip modify | Decode JSON → convert to text → encode | Decode JSON → modify → encode JSON |
| Programmatic use | Build text string | Build JSON object |
| Testing | Manual text construction | Use decode output directly |

### JSON Format

Use same format as decode output - enables direct round-trip without transformation.

### Minified JSON Requirement

Protocol uses space-delimited fields. JSON must be minified (no internal spaces) to be parsed as single argument. This is acceptable because:
- Programmatic callers can easily minify
- Round-trip from decode output is already minified

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
- [ ] Boundary tests cover all numeric inputs (N/A)
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (N/A)
- [ ] RFC references added to code (N/A)
- [ ] RFC constraint comments added (N/A)

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
