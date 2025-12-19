# ZeBGP Coding Standards

---

## TDD-First Development (BLOCKING RULE)

```
┌─────────────────────────────────────────────────────────────────┐
│  WRITE TESTS BEFORE IMPLEMENTATION - NO EXCEPTIONS              │
│                                                                 │
│  See TDD_ENFORCEMENT.md for complete workflow                  │
└─────────────────────────────────────────────────────────────────┘
```

**The workflow:**
1. Write test (with VALIDATES/PREVENTS documentation)
2. Run test → MUST FAIL
3. Write implementation
4. Run test → MUST PASS

**Violation:** Writing ANY implementation code before test exists.

---

## Think Before You Code

**BEFORE writing ANY code:**
1. **Write tests first** (see TDD_ENFORCEMENT.md)
2. Read and understand ALL related existing code
3. Trace through data flows and type signatures
4. Verify assumptions against actual implementation
5. Check for edge cases and error conditions
6. Look for similar patterns already in the codebase

**NEVER:**
- Write implementation before tests exist
- Guess at APIs or function signatures - READ the actual code
- Make assumptions about behavior - VERIFY by reading implementation
- Copy patterns without understanding why they work

---

## Go Version Requirements

**Required:** Go 1.21+

This enables:
- `slog` for structured logging
- Modern generics
- `slices` and `maps` packages

---

## Linting

```bash
make lint  # golangci-lint run
```

**Must pass with zero issues.**

---

## Testing (MANDATORY)

Before declaring "fixed"/"ready"/"working"/"complete":

```bash
make test  # go test -race ./...
```

---

## Error Handling

### Never Ignore Errors

```go
// WRONG
f, _ := os.Open(path)

// RIGHT
f, err := os.Open(path)
if err != nil {
    return fmt.Errorf("open %s: %w", path, err)
}
```

### Error Wrapping

Use `fmt.Errorf` with `%w` for wrapping:

```go
if err != nil {
    return fmt.Errorf("parsing header: %w", err)
}
```

### Sentinel Errors

Define package-level sentinel errors:

```go
var (
    ErrShortRead     = errors.New("short read")
    ErrInvalidMarker = errors.New("invalid marker")
)
```

---

## Context Usage

### Always Accept Context

```go
// WRONG
func (p *Peer) Run() error

// RIGHT
func (p *Peer) Run(ctx context.Context) error
```

### Respect Cancellation

```go
func (p *Peer) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case msg := <-p.incoming:
            // handle message
        }
    }
}
```

---

## Concurrency Patterns

### Prefer Channels Over Mutexes

```go
// Preferred: channel-based coordination
type Store struct {
    requests chan request
}

// When mutex is appropriate: simple data protection
type Cache struct {
    mu    sync.RWMutex
    items map[string]Item
}
```

### Goroutine Lifecycle

Always ensure goroutines can be stopped:

```go
func (w *Worker) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case work := <-w.tasks:
            w.process(work)
        }
    }
}
```

---

## Zero-Copy Patterns

### Slice Semantics

```go
// Zero-copy: slicing shares underlying array
data := buffer[offset : offset+length]

// Copy when needed: releasing large buffer
dataCopy := make([]byte, length)
copy(dataCopy, buffer[offset:offset+length])
```

### When to Copy

| Copy | Don't Copy |
|------|------------|
| Storing data beyond buffer lifetime | Parsing within same function |
| Using as map key | Passing to struct.unpack equivalent |
| Returning from function that receives large buffer | Creating sub-slices for parsing |

---

## Struct Design

### Wire Format Structs

Match RFC wire format exactly:

```go
// Header represents BGP message header (RFC 4271)
type Header struct {
    Marker [16]byte
    Length uint16
    Type   MessageType
}
```

### Interface Design

```go
// Message is implemented by all BGP messages
type Message interface {
    Type() MessageType
    Pack(negotiated *Negotiated) ([]byte, error)
}
```

---

## Package Organization

### Public vs Internal

```
pkg/          # Public API - stable interfaces
internal/     # Private - can change freely
```

### Package Naming

- Singular: `message`, `attribute`, `nlri`
- Descriptive: `capability`, `flowspec`
- No stuttering: `message.Message`, not `message.BGPMessage`

---

## Naming Conventions

### Variables

```go
// Short names for short scopes
for i, v := range items { }

// Descriptive for longer scopes
negotiatedCapabilities := peer.Capabilities()
```

### Constants

```go
// Grouped with type
type MessageType uint8

const (
    TypeOPEN         MessageType = 1
    TypeUPDATE       MessageType = 2
    TypeNOTIFICATION MessageType = 3
)
```

### Acronyms

```go
// Acronyms are all caps or all lower
var bgpID uint32      // not bgpId
type AFI uint16       // not Afi
func ParseASPath()    // not ParseAsPath
```

---

## Documentation

### Package Comments

```go
// Package message implements BGP message types per RFC 4271.
package message
```

### Function Comments

```go
// ParseHeader reads a BGP header from data.
// It returns ErrShortRead if data is less than 19 bytes.
func ParseHeader(data []byte) (*Header, error)
```

### Exported Types

```go
// Negotiated holds the result of BGP capability negotiation.
// It is populated after OPEN message exchange and used
// throughout the session for encoding/decoding decisions.
type Negotiated struct {
    // ...
}
```

---

## Prohibited Patterns

### No Panic for Errors

```go
// WRONG
if err != nil {
    panic(err)
}

// RIGHT
if err != nil {
    return fmt.Errorf("...: %w", err)
}
```

### No Global Mutable State

```go
// WRONG
var globalConfig *Config

// RIGHT - pass dependencies explicitly
type Server struct {
    config *Config
}
```

### Limited init() Usage

Only use `init()` for:
- Registering message/attribute/NLRI types
- Package-level sentinel errors

```go
// Acceptable
func init() {
    RegisterMessage(TypeOPEN, &OpenUnpacker{})
}
```

---

## Registry Pattern

For extensible types (messages, attributes, NLRI):

```go
var messageRegistry = map[MessageType]MessageUnpacker{}

func RegisterMessage(t MessageType, u MessageUnpacker) {
    messageRegistry[t] = u
}

func init() {
    RegisterMessage(TypeOPEN, &OpenUnpacker{})
}
```

---

## Testing Standards

**See:** TDD_ENFORCEMENT.md for complete TDD workflow.

### Test Documentation (MANDATORY)

Every test MUST document what it validates and prevents:

```go
// TestParseHeader verifies BGP header parsing for valid and invalid inputs.
//
// VALIDATES: RFC 4271 Section 4.1 header format compliance.
//
// PREVENTS: Parsing crashes on malformed input, buffer overflows,
// incorrect message type detection.
func TestParseHeader(t *testing.T) {
    // ...
}
```

### Table-Driven Tests

```go
// TestParseHeader verifies header parsing across multiple scenarios.
//
// VALIDATES: RFC 4271 Section 4.1 - Message Header Format
//
// PREVENTS: Parsing failures, buffer overflows, incorrect type detection.
func TestParseHeader(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        want    *Header
        wantErr error
    }{
        {
            name:  "valid header",
            input: validHeaderBytes,
            want:  &Header{...},
        },
        {
            name:    "short read",
            input:   []byte{0xFF},
            wantErr: ErrShortRead,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseHeader(tt.input)
            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Test File Naming

```
message.go       -> message_test.go
header.go        -> header_test.go
```

### Testify Assertions

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// require: stops test on failure (use for setup/preconditions)
require.NoError(t, err, "setup must succeed")

// assert: continues test on failure (use for assertions)
assert.Equal(t, expected, actual, "values must match")
```

### TDD Workflow Reminder

1. Write test FIRST
2. Run test → MUST FAIL
3. Write implementation
4. Run test → MUST PASS

---

## Git Workflow

**NEVER commit without explicit user request**

User must say: "commit", "make a commit", "git commit"

**Before ANY git operation:**
```bash
git status && git log --oneline -5
```

---

## Quick Checklist

- [ ] Go 1.21+ features only
- [ ] All errors handled (no `_` for errors)
- [ ] Context passed through call chains
- [ ] `golangci-lint run` passes
- [ ] `go test -race ./...` passes
- [ ] No global mutable state
- [ ] No `panic()` for errors
- [ ] Interfaces defined where needed
- [ ] Table-driven tests
- [ ] User explicitly requested commit/push

---

**Updated:** 2025-12-19
