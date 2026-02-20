# Spec No Code Rationale

Why: `.claude/rules/spec-no-code.md`

## Why No Code in Specs
1. Code belongs in source files -- not documentation
2. Specs describe WHAT and WHY -- code shows HOW
3. Code in specs becomes stale -- misleading future readers
4. Implementation emerges from TDD -- not prescribed in spec

## Explicit Prohibitions
- Go code snippets (triple-backtick go blocks)
- Python code snippets
- Any programming language code
- Function definitions (func/def/fn)
- Struct definitions
- Implementation details

## BAD Example: Code in spec
A spec with `type Schema struct { Module string; Handlers []string }` and `func (s *Schema) Route(path string) Plugin { ... }` -- this is implementation, not specification.

## GOOD Example: Table describing data
| Field | Type | Description |
|-------|------|-------------|
| Module | string | YANG module name |
| Handlers | list | Handler paths this schema provides |

## GOOD Example: Prose describing behavior
1. Find handler by longest prefix match
2. Route to plugin that registered this handler
3. Send command via pipe
4. Return response to caller

## Validation
The `validate-spec.sh` hook BLOCKS specs containing code blocks. Fix: convert code to tables and prose before saving.
