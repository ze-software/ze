---
kind: directive
level:
stage:
excepted-by: performance/banned-patterns/the-constant-expression-exception, performance/banned-patterns/convert-cold-path-concatenation-on-touch
---
**BLOCKING for new code and all hot paths** (hook-enforced at edit time by
`c_string_concat`). Every `+` between strings allocates a new backing array
and copies both sides. Use `textbuf.Buffer` instead.
