# TDD Rationale

Why: `ai/rules/tdd.md`

## Why Tests Must Fail First

A test that passes immediately proves nothing. The RED→GREEN cycle ensures the test actually validates the behavior, not just the implementation.

## Why Table-Driven Tests

```go
tests := []struct {
    name string; input []byte; want *Header; wantErr error
}{
    {"valid_keepalive", []byte{0xFF...}, &Header{Type: TypeKEEPALIVE}, nil},
    {"truncated", []byte{0xFF, 0xFF, 0xFF}, nil, ErrShortRead},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

## Why Fuzz Tests for Wire Format

External input (BGP messages, config files, CLI commands) can be arbitrary bytes. Fuzz tests prove parsing handles all input without crashing.

```go
func FuzzParseNLRI(f *testing.F) {
    f.Add([]byte{24, 10, 0, 0})
    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = ParseNLRI(data)  // MUST NOT panic
    })
}
```

## Why Non-Default Parameter Testing

Bugs hide when tests only use defaults. Example: `New()` defaults `idx=0`, masking a bug in `p.slots[handle]` vs `p.slots[handle.Slot()]` that only manifests with `idx > 0`.

## Why Boundary 3-Point Rule

Off-by-one errors are the most common numeric bug. Testing exactly at boundaries catches them deterministically.
