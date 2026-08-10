---
kind: directive
level: MUST
stage:
excepted-by: performance/banned-patterns/the-constant-expression-exception, performance/banned-patterns/convert-cold-path-concatenation-on-touch
---
**BLOCKING for new code and all hot paths** (hook-enforced at edit time by
`c_string_concat`). The `+` operator MUST NOT be used between strings: it
allocates a new backing array and copies both sides. `textbuf.Buffer` MUST be
used instead.
