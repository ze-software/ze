# No Code in Specs

**BLOCKING:** Specs MUST NOT contain code snippets.

## What Specs Must NOT Contain

- Go code snippets (```go ... ```)
- Python code snippets
- Any programming language code
- Function definitions (func/def/fn)
- Struct definitions
- Implementation details

## What Specs SHOULD Contain

| Instead of... | Use... |
|---------------|--------|
| Go struct | Table with Field/Type/Description columns |
| Function implementation | Prose describing behavior, numbered steps |
| Code example | Text example showing input/output format |
| State machine code | State transition table or diagram |

## Why No Code in Specs

1. **Code belongs in source files** - not documentation
2. **Specs describe WHAT and WHY** - code shows HOW
3. **Code in specs becomes stale** - misleading future readers
4. **Implementation emerges from TDD** - not prescribed in spec

## Examples

### BAD: Code in spec

```
## Implementation

​```go
type Schema struct {
    Module   string
    Handlers []string
}

func (s *Schema) Route(path string) Plugin {
    // find handler...
}
​```
```

### GOOD: Table describing data

```
## Data Structure

| Field | Type | Description |
|-------|------|-------------|
| Module | string | YANG module name |
| Handlers | list | Handler paths this schema provides |
```

### GOOD: Prose describing behavior

```
## Routing Behavior

1. Find handler by longest prefix match
2. Route to plugin that registered this handler
3. Send command via pipe
4. Return response to caller
```

## Validation

The `validate-spec.sh` hook BLOCKS specs containing code blocks.

Fix: Convert code to tables and prose before saving spec.
