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

## Encapsulation Onion

Networking protocols are encapsulation onions: Ethernet wraps IP wraps TCP wraps BGP wraps
UPDATE wraps path attributes wraps AS_PATH. The only sane way to work with them is to
allocate once at the outermost layer and slice inward with specialized data-manipulation
programs.

The three onion principles are facets of the same idea:

| Principle | Side | What it governs |
|-----------|------|-----------------|
| Encapsulation onion | Structure | One allocation, slice inward, never copy between layers |
| Buffer-first encoding | Write | `WriteTo(buf, off)` into pooled buffers, no append/make |
| Lazy over eager | Read | Pass raw bytes, iterate with offsets, parse on demand |

Ze's WireUpdate is the canonical example: a single buffer holds the wire UPDATE, and
iterators (NLRI, attributes, MP_REACH) narrow the window without allocating. PackContext
tells each layer how to interpret bytes (ASN4, ADD-PATH) without copying them.

freeRtr's packHolder demonstrates the same pattern at a broader scope: one struct
accumulates metadata as a packet traverses ETH -> IP -> MPLS -> BGP layers, giving any
layer access to all previously-parsed headers without re-parsing or re-allocating.

Currently Ze operates at the BGP layer only. If Ze ever handles lower layers (BMP, TCP
options, MPLS), the same discipline applies: allocate at the outermost layer, slice inward.

## Error Handling

- Errors should be actionable (what failed, why, how to fix)
- Distinguish recoverable vs fatal
- Don't hide in logs -- propagate
- Clean up resources on error paths
