# Design Principles Rationale

Why: `.claude/rules/design-principles.md`

## YAGNI Examples

❌ "Let's add a plugin system in case we need it" → Build concrete implementation now
❌ "This config option might be useful someday" → Add when needed
❌ "Let's make this generic" → Solve specific problem first

## Identity Wrapper Example

```go
// ❌ Just delegates:
func parseOrigin(s string) (uint8, error) { return parse.Origin(s) }
// Call parse.Origin() directly.

// ✅ Transforms interface:
func ParseOrigin(s string) (config.Origin, error) {
    v, err := parse.Origin(s)
    return config.Origin(v), err  // Type conversion justifies wrapper
}
```

## Interface Segregation Example

```go
// ❌ Forces unused methods:
type MessageHandler interface { HandleOpen; HandleUpdate; HandleNotification; HandleKeepAlive; HandleRouteRefresh }

// ✅ Minimal:
type UpdateHandler interface { HandleUpdate(msg *Update) error }
```

## Naming Guidance

Precise names: `wireBytes` not `data`, `peerConfig` not `info`, `parseResult` not `result`.
Consistent: don't mix "peer"/"neighbor" or "message"/"packet".
Length ∝ scope: `i` (loop), `peer` (local), `peerAddr` (field), `DefaultKeepaliveInterval` (constant).

## Error Handling

- Errors should be actionable (what failed, why, how to fix)
- Distinguish recoverable vs fatal
- Don't hide in logs — propagate
- Clean up resources on error paths
