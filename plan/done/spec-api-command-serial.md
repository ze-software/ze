# Spec: API Command Serial Numbers

## Status: IMPLEMENTED

## Problem

Current process ↔ ZeBGP communication lacks correlation:

1. **Process → ZeBGP**: Commands get optional "done" ack, but no way to match response to specific command
2. **ZeBGP → Process**: Events are fire-and-forget, no acknowledgment

This causes issues:
- Process sends multiple commands rapidly, can't tell which "done" belongs to which
- No way to implement reliable delivery
- Plugin command routing (spec-api-plugin-commands.md) needs correlation anyway

## Goals

1. Add serial numbers to ALL bidirectional communication
2. Unified protocol for correlation
3. Backward compatible (serial optional for existing commands)
4. Foundation for plugin command routing

## Non-Goals

- Changing message format (text vs JSON)
- Guaranteed delivery (that's a separate concern)

---

## Current Protocol

### Process → ZeBGP (stdout)

```
announce route 10.0.0.0/24 next-hop 1.2.3.4
withdraw route 10.0.0.0/24
session sync enable
```

### ZeBGP → Process (stdin)

```json
{"type": "update", "peer": {...}, "announce": {...}}
{"type": "state", "peer": {...}, "state": "up"}
```

### Current Ack (if enabled)

```json
{"status": "done"}
{"status": "error", "data": "message"}
```

**Problem:** If process sends 3 commands rapidly, it gets 3 responses but can't match them.

---

## Proposed Protocol

### Encoding Direction

| Direction | Format | Notes |
|-----------|--------|-------|
| Process → ZeBGP | Always text | Commands with optional `#N` prefix |
| ZeBGP → Process | JSON or text | Configurable per process |

The `encoding` setting only affects ZeBGP→Process direction.

### Process → ZeBGP (stdout)

Add optional `#<serial>` prefix to any command:

```
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
#2 withdraw route 10.0.0.0/24
#3 session sync enable
```

**Rules:**
- Serial is process-chosen, incrementing integer
- `#` prefix marks process→ZeBGP command
- Without prefix, behaves as today (no correlation)

### ZeBGP → Process (stdin)

All ZeBGP messages use `@` prefix (text) or `serial` field (JSON).

#### JSON Encoding

```json
{"serial": "1", "status": "done"}
{"serial": "2", "status": "done"}
{"serial": "3", "status": "done", "data": {"ack": "enabled"}}
{"serial": "4", "status": "error", "data": "invalid prefix"}
```

**Rules:**
- No `#N` prefix → no response (fire-and-forget)
- `#N` prefix → always get response
- `status`: `"done"` or `"error"`

For events/notifications (no response expected), use empty serial:

```json
{"serial": "", "type": "update", "peer": {...}}
{"serial": "", "type": "state", "peer": {...}, "state": "up"}
```

For ZeBGP-initiated requests (plugin commands), ZeBGP assigns alpha serial:

```json
{"serial": "a", "type": "request", "command": "myapp status", "args": ["web"]}
{"serial": "b", "type": "complete", "command": "myapp status", "partial": "w"}
```

#### Text Encoding

> **Note:** Text encoding for responses defined for completeness. Currently JSON-only.

Responses use `@` prefix (responding to an ID):

```
@1 done
@2 done
@3 done ack enabled
@4 error invalid prefix
```

Events (no serial):

```
peer 10.0.0.1 state up
peer 10.0.0.1 update announce ipv4 unicast 192.168.1.0/24 next-hop 10.0.0.1
```

ZeBGP-initiated requests use `#` prefix (setting an ID):

```
#a request myapp status web
#b complete myapp status w
```

### Response to ZeBGP-initiated requests

Process responds with `@` prefix (echoing the alpha serial). Process→ZeBGP is always text:

```
@a done component web healthy true
@b done completions web
```

---

## Serial Number Format

| Initiator | Serial Format | Examples | Purpose |
|-----------|---------------|----------|---------|
| Process | Numeric | `1`, `2`, `3`... | Process-initiated commands |
| ZeBGP | Alpha | `a`, `b`, ... `z`, `aa`, `ab`... | ZeBGP-initiated requests |
| Events | Empty | `""` | Notifications (no response expected) |

**Prefix meaning (text encoding):**

| Prefix | Meaning | Used by |
|--------|---------|---------|
| `#N` | "I'm setting serial N" | Initiator (command/request) |
| `@N` | "I'm responding to serial N" | Responder (done/error) |

**Why different serial formats:**
- Zero collision by design (numeric vs alpha)
- Instantly distinguishable in logs
- Process: simple `counter++`
- ZeBGP: simple alpha increment (like Excel columns: a-z, aa-az, ba-bz...)

---

## Serial = Ack

No separate `session ack enable/disable`. Serial controls acknowledgment:

### Without serial (fire-and-forget)

```
announce route 10.0.0.0/24 next-hop 1.2.3.4
```

No response. Command executed silently.

### With serial (acknowledged)

```
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
```

ZeBGP responds:
```json
{"serial": "1", "status": "done"}
```

### Detection

ZeBGP detects serial usage per-command (not per-process).
Each command with `#N` gets a response. Commands without `#N` don't.

---

## Examples

### Process sends multiple commands

**JSON encoding (ZeBGP→Process):**
```
Process stdout (text):
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
#2 announce route 10.0.1.0/24 next-hop 1.2.3.4
#3 withdraw route 10.0.2.0/24

Process stdin (JSON, may arrive out of order):
{"serial": "2", "status": "done"}
{"serial": "1", "status": "done"}
{"serial": "3", "status": "error", "data": "route not found"}
```

**Text encoding:**
```
Process stdout:
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
#2 announce route 10.0.1.0/24 next-hop 1.2.3.4
#3 withdraw route 10.0.2.0/24

Process stdin:
@2 done
@1 done
@3 error route not found
```

### Mixed commands and events

**JSON encoding (ZeBGP→Process):**
```
Process stdout (text):
#5 announce route 10.0.0.0/24 next-hop 1.2.3.4

Process stdin (JSON):
{"serial": "", "type": "state", "peer": {"address": "192.0.2.1"}, "state": "up"}
{"serial": "5", "status": "done"}
{"serial": "", "type": "update", "peer": {"address": "192.0.2.1"}, ...}
```

**Text encoding:**
```
Process stdout:
#5 announce route 10.0.0.0/24 next-hop 1.2.3.4

Process stdin:
peer 192.0.2.1 state up
@5 done
peer 192.0.2.1 update announce ipv4 unicast ...
```

### Plugin command flow

**JSON encoding (ZeBGP→Process is JSON, Process→ZeBGP is always text):**
```
Process stdout (text):
#10 register command "myapp status" description "Show status"

Process stdin (JSON):
{"serial": "10", "status": "done"}
{"serial": "a", "type": "request", "command": "myapp status", "args": ["web"]}

Process stdout (text):
@a done healthy true
```

**Text encoding (both directions are text):**
```
Process stdout:
#10 register command "myapp status" description "Show status"

Process stdin:
@10 done
#a request myapp status web

Process stdout:
@a done healthy true
```

---

## Implementation

### Process struct changes

```go
type Process struct {
    // ... existing fields ...

    serialEnabled bool    // Detected from first #N command
    nextSerial    int     // For ZeBGP-initiated requests (alpha: a=0, b=1, ...)
}
```

### Alpha serial generation

```go
// nextAlphaSerial returns next alpha serial: a, b, ..., z, aa, ab, ...
func (p *Process) nextAlphaSerial() string {
    n := p.nextSerial
    p.nextSerial++

    var result []byte
    for {
        result = append([]byte{byte('a' + n%26)}, result...)
        n = n/26 - 1
        if n < 0 {
            break
        }
    }
    return string(result)
}
```

### Parsing process commands

```go
func (p *Process) handleOutput(line string) {
    var serial string
    var hasSerial bool

    // Check for #N prefix (numeric)
    if strings.HasPrefix(line, "#") {
        idx := strings.Index(line, " ")
        if idx > 1 {
            serial = line[1:idx]  // Keep as string
            hasSerial = true
            line = line[idx+1:]   // Remove prefix
            p.serialEnabled = true
        }
    }

    // Process command...
    // Include serial in response if hasSerial
}
```

### Response type

```go
// Response is sent to process stdin for commands with #N serial prefix.
// Commands without serial get no response.
type Response struct {
    Serial string `json:"serial"`           // Correlation ID (always string)
    Status string `json:"status"`           // "done" or "error"
    Data   any    `json:"data,omitempty"`   // Payload (success data or error message)
}
```

### Response formatting

```go
func (p *Process) sendResponse(serial string, status string, data any) {
    // No serial = no response
    if serial == "" {
        return
    }

    resp := map[string]any{
        "serial": serial,
        "status": status,
    }

    if data != nil {
        resp["data"] = data
    }

    // Write JSON to process stdin
}
```

---

## Migration Path

1. **Phase 1**: Add serial parsing, respond with serial when present
2. **Phase 2**: Add serial to events (serial: "") when serialEnabled
3. **Phase 3**: Plugin command routing uses alpha serials
4. **Phase 4**: Document new protocol, encourage adoption

---

## Relationship to spec-api-plugin-commands.md

This spec is a **prerequisite** for plugin commands:

- Plugin commands use ZeBGP-initiated serials (alpha: a, b, c...)
- `response <alpha> done/error` uses same serial mechanism
- Unified correlation for all request/response patterns

Update spec-api-plugin-commands.md to reference this spec for serial details.

---

## Open Questions

1. **Serial prefix syntax**: `#N` vs `@N` vs `[N]`?
   - `#N` chosen: unlikely to conflict with command text

2. **Out-of-order responses**: Allow or require in-order?
   - Allow out-of-order (async processing)

3. **Timeout**: Should ZeBGP timeout waiting for response?
   - Yes, configurable per-process (default 30s)

4. **Alpha overflow**: What happens after `zzzz...`?
   - Practically infinite (26^10 = 141 trillion combinations)
   - If needed, wrap to `a` with warning
