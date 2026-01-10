# Spec: Lint Fixes (No Interface Changes)

## Task
Fix all lint issues that do NOT require changing function/method signatures.

## Current State
After running `make lint`, ~1500+ issues remaining across these categories:

| Category | Count | Requires Interface Change |
|----------|-------|---------------------------|
| errcheck | 801 | NO - add `//nolint:errcheck` or handle error |
| gocritic | 477 | PARTIAL - some need signature changes |
| godot | 125 | NO - add periods to comments |
| govet | 49 | NO - rename shadowed variables |
| misspell | 24 | NO - fix spelling |
| goimports | 21 | NO - run goimports |
| prealloc | 13 | NO - add make() with capacity |
| staticcheck | 8 | DEPENDS |
| goconst | 4 | NO - extract constants |
| dupl | 2 | REVIEW |
| gosec | 1 | DEPENDS |
| exhaustive | 1 | NO - add missing cases or nolint |

## Rules

1. **DO NOT change .golangci.yml** - fix the actual code
2. **DO NOT change function signatures** - no adding/removing pointer receivers
3. **DO NOT change interface definitions**
4. **hugeParam issues are SKIPPED** - they require interface changes

## Fix Categories

### 1. errcheck (Priority: HIGH)
The config has `check-blank: true`, meaning `_, _ = fn()` counts as unchecked.

**Fix patterns:**
```go
// BAD - still triggers errcheck
_, _ = fmt.Fprintf(w, "text")

// GOOD - explicit nolint
fmt.Fprintf(w, "text") //nolint:errcheck // best effort logging

// OR - actually handle error
if _, err := fmt.Fprintf(w, "text"); err != nil {
    log.Printf("write failed: %v", err)
}
```

**Common cases to nolint:**
- `Close()` in defer
- `Write()` for logging/debug output
- `fmt.Fprintf` to stderr for CLI messages

### 2. godot (Priority: LOW)
Add periods at end of comments.

```go
// BAD
// This function does something

// GOOD
// This function does something.
```

### 3. govet shadow (Priority: MEDIUM)
Rename variables that shadow outer scope.

```go
// BAD
err := doFirst()
if _, err := doSecond(); err != nil { // shadows
    return err
}

// GOOD
err := doFirst()
if _, err2 := doSecond(); err2 != nil {
    return err2
}
```

### 4. misspell (Priority: LOW)
Fix common misspellings.

### 5. goimports (Priority: LOW)
Run `goimports` on affected files.

### 6. prealloc (Priority: MEDIUM)
```go
// BAD
parts := []string{"flow"}
for _, c := range components {
    parts = append(parts, c)
}

// GOOD
parts := make([]string, 0, len(components)+1)
parts = append(parts, "flow")
for _, c := range components {
    parts = append(parts, c)
}
```

### 7. gocritic (Fixable without interface changes)

**builtinShadow** - rename variables:
```go
// BAD
cap := getSomeCap()

// GOOD
capVal := getSomeCap()
```

**typeAssertChain** - use type switch:
```go
// BAD
if m, ok := item.(map[string]any); ok {
    // ...
} else if s, ok := item.(string); ok {
    // ...
}

// GOOD
switch v := item.(type) {
case map[string]any:
    // use v
case string:
    // use v
}
```

**rangeValCopy** - use index:
```go
// BAD
for _, n := range cfg.Peers {  // copies 408 bytes
    process(n)
}

// GOOD
for i := range cfg.Peers {
    n := &cfg.Peers[i]
    process(n)
}
```

**nestingReduce** - invert if:
```go
// BAD
for _, item := range items {
    if condition {
        // long body
    }
}

// GOOD
for _, item := range items {
    if !condition {
        continue
    }
    // long body
}
```

## SKIP These (Interface Changes Required)

### hugeParam
These require changing function receivers to pointers:
```go
// CURRENT - triggers hugeParam
func (m model) Init() tea.Cmd { ... }

// WOULD FIX - but changes interface
func (m *model) Init() tea.Cmd { ... }
```
**DO NOT FIX** - requires interface changes.

### unnamedResult (some)
If naming results would change semantics, skip.

## Execution Order

1. Run `goimports` on all files
2. Fix misspell issues
3. Fix godot (add periods)
4. Fix govet shadow (rename vars)
5. Fix prealloc (add make with capacity)
6. Fix gocritic fixable issues (builtinShadow, typeAssertChain, rangeValCopy, nestingReduce)
7. Fix errcheck - add `//nolint:errcheck` comments where appropriate
8. Run `make lint` to verify

## Commands

```bash
# Check current status
GOCACHE=/tmp/claude/gocache GOLANGCI_LINT_CACHE=/tmp/claude/golangci-cache make lint 2>&1 | head -100

# Run goimports
goimports -w ./...

# Verify after fixes
make lint
```

## Verification

After all fixes:
```bash
make lint 2>&1 | grep -E 'issues:'
# Should show only hugeParam and interface-related issues remaining
```
