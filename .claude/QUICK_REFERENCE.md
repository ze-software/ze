# ZeBGP Quick Reference

**Purpose:** Essential patterns in one place. Read this before writing any code.

---

## Workflow (MANDATORY)

```
1. /prep <task>           # Creates spec, loads context
2. Write test             # MUST fail first
3. Implement              # Minimum to pass
4. make test && make lint # MUST pass
5. Self-review            # Fix issues, re-review
6. Report to user         # Wait for commit request
```

---

## TDD Rules

| Rule | Enforcement |
|------|-------------|
| Test before impl | Delete impl if test doesn't exist |
| Test must fail first | Show failure output |
| Document tests | `VALIDATES:` and `PREVENTS:` comments |
| Paste output | Prove test ran, don't summarize |

---

## Code Patterns

### Error Handling
```go
// ALWAYS wrap errors
if err != nil {
    return fmt.Errorf("parsing header: %w", err)
}

// NEVER ignore errors
f, _ := os.Open(path)  // FORBIDDEN
```

### Context
```go
// All long-running functions take context
func Process(ctx context.Context, data []byte) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // ...
}
```

### Registry Pattern
```go
// Used for: messages, attributes, NLRI, capabilities
var registry = map[TypeCode]Parser{}

func Register(code TypeCode, parser Parser) {
    registry[code] = parser
}
```

---

## Zero-Copy Patterns

### Pool Handle
```go
// Handle = (bufferBit << 31) | slotIndex
type Handle uint32

func (h Handle) BufferBit() uint32  { return uint32(h) >> 31 }
func (h Handle) SlotIndex() uint32  { return uint32(h) & 0x7FFFFFFF }
```

### Encoding Context
```go
// Fast path: context IDs match
if route.sourceCtxID == destCtxID {
    return route.wireBytes  // zero-copy
}
// Slow path: re-encode
return route.PackAttributesWithContext(srcCtx, dstCtx)
```

### NLRI Packed-Bytes
```go
// Store wire format, extract properties lazily
type INET struct {
    packed     []byte  // wire format
    hasAddpath bool
}

func (n *INET) PackNLRI(addpath bool) []byte {
    if n.hasAddpath == addpath {
        return n.packed  // zero-copy
    }
    // re-encode
}
```

---

## Wire Formats (Quick Ref)

### Message Header (19 bytes)
```
[Marker: 16 bytes 0xFF][Length: 2][Type: 1]
Types: 1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE, 5=ROUTE-REFRESH
```

### Attribute Header
```
[Flags: 1][Type: 1][Length: 1-2]
Flags: 0x80=Optional, 0x40=Transitive, 0x20=Partial, 0x10=ExtLen
```

### Capability TLV
```
[Code: 1][Length: 1][Value: variable]
Codes: 1=MP, 65=ASN4, 69=ADD-PATH, 6=ExtMsg
```

---

## Key Type Mappings

| Concept | Interface | Key Method |
|---------|-----------|------------|
| Message | `Message` | `Type()`, `Pack()` |
| Attribute | `Attribute` | `Code()`, `Flags()`, `Pack()` |
| NLRI | `NLRI` | `PackNLRI()`, `Index()` |
| Capability | `Capability` | `Code()`, `Pack()` |

---

## ExaBGP Reference Paths

```
/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/
├── bgp/message/           → pkg/bgp/message/
├── bgp/message/open/capability/  → pkg/bgp/capability/
├── bgp/message/update/attribute/ → pkg/bgp/attribute/
├── bgp/message/update/nlri/      → pkg/bgp/nlri/
└── reactor/peer/          → pkg/reactor/peer.go
```

---

## Verification Commands

```bash
make test      # Unit + integration tests
make lint      # golangci-lint
make functional # Encoding + API tests

# Single functional test
go run ./test/cmd/self-check <nick>
```

---

## Git Rules

| Action | Requirement |
|--------|-------------|
| Commit | User must say "commit" |
| Push | User must say "push" |
| Reset/revert | Ask permission first |
| Before commit | `make test && make lint` must pass |

---

## Forbidden

- `panic()` for error handling
- `f, _ := func()` (ignoring errors)
- Global mutable state
- Impl before test
- Commit without explicit request
- Claiming "done" without running tests
