# Spec: API Command Serial Numbers

## Status: IMPLEMENTED

Serial correlation is already implemented in ZeBGP. This spec documents the existing design.

## Overview

Serials enable request/response correlation between ZeBGP and external processes.

| Direction | Serial Type | Format | Example |
|-----------|-------------|--------|---------|
| Process → ZeBGP | Numeric | `#N command` | `#1 announce route ...` |
| ZeBGP → Process | Alpha | `#abc command` | `#a request ...` |
| Response | Echo | `@serial result` | `@a done {"status": "ok"}` |

**Alpha encoding:** 0→a, 1→b, ..., 9→j. So `123` → `bcd`.

This avoids collision: process uses `#1`, `#2`; ZeBGP uses `#a`, `#b`.

---

## Process → ZeBGP (stdout)

Optional `#N` prefix for correlation:

```
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
#2 withdraw route 10.0.0.0/24
#3 register command "myapp status" description "Show status"
```

**Without prefix:** No response (fire-and-forget).

**With prefix:** ZeBGP responds with `{"serial": "1", "status": "done"}`.

---

## ZeBGP → Process (stdin)

### Events (no response expected)

No serial field:
```json
{"type": "update", "peer": {...}, "announce": {...}}
{"type": "state", "peer": {...}, "state": "up"}
```

### Responses to process commands

Includes serial as string:
```json
{"serial": "1", "status": "done"}
{"serial": "2", "status": "done", "data": {"routes": 1}}
{"serial": "3", "status": "error", "error": "invalid prefix"}
```

### ZeBGP-initiated requests (alpha serial)

```json
{"serial": "a", "type": "request", "command": "myapp status", "args": ["web"]}
{"serial": "b", "type": "complete", "command": "myapp status", "partial": "w"}
```

---

## Response to ZeBGP Requests (stdout)

Process echoes alpha serial with `@` prefix:

```
@a done {"component": "web", "healthy": true}
@b done {"completions": [{"value": "web"}]}
@c error "component not found"
```

**Format:** `@<serial> done [json]` or `@<serial> error "<message>"`

---

## Alpha Serial Encoding

ZeBGP converts numbers to letters to avoid collision with process numeric serials:

| Number | Alpha |
|--------|-------|
| 0 | a |
| 1 | b |
| 9 | j |
| 10 | ba |
| 123 | bcd |
| 999 | jjj |

```go
// encodeAlphaSerial converts number to alpha serial.
// 0->a, 1->b, ..., 9->j. Example: 123 -> "bcd".
func encodeAlphaSerial(n uint64) string {
    if n == 0 {
        return "a"
    }
    var result []byte
    for n > 0 {
        digit := n % 10
        result = append([]byte{byte('a' + digit)}, result...)
        n /= 10
    }
    return string(result)
}
```

---

## Examples

### Process sends commands with correlation

```
Process stdout:
#1 announce route 10.0.0.0/24 next-hop 1.2.3.4
#2 announce route 10.0.1.0/24 next-hop 1.2.3.4
#3 withdraw route 10.0.2.0/24

Process stdin (may arrive out of order):
{"serial": "2", "status": "done"}
{"serial": "1", "status": "done"}
{"serial": "3", "status": "error", "error": "route not found"}
```

### ZeBGP sends request, process responds

```
Process stdin:
{"serial": "a", "type": "request", "command": "myapp status", "args": ["web"]}

Process stdout:
@a done {"component": "web", "status": "healthy"}
```

### Mixed flow

```
Process stdout:
#5 register command "hello" description "Say hello"

Process stdin:
{"serial": "5", "status": "done"}
{"type": "state", "peer": {"address": "192.0.2.1"}, "state": "up"}
{"serial": "a", "type": "request", "command": "hello", "args": ["world"]}

Process stdout:
@a done {"greeting": "Hello, world!"}
```

---

## Implementation Reference

- `pkg/plugin/server.go`: `parseSerial()`, `encodeAlphaSerial()`, `isAlphaSerial()`
- `pkg/plugin/process.go`: `SendRequest()`, `parseResponseSerial()`
- Serial in Response: `pkg/plugin/types.go` - `Response.Serial`
