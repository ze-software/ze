---
kind: directive
level: MUST
stage:
---
1. Am I using `+` to concatenate strings? `textbuf.Buffer` chain MUST be used instead.
2. Am I using `fmt.Sprintf`? `textbuf.Buffer` or standalone functions MUST be used.
3. Am I using `strings.Join`? `textbuf.Join` or `b.Join(items, sep)` MUST be used.
4. Am I using `strings.Builder`? `textbuf.Buffer` MUST be used (128B inline, poolable).
5. Am I calling `.String()` just to concatenate? `textbuf.Buffer.Addr()` etc. MUST be used.
6. Am I storing a string that will be parsed back for comparison? The typed value MUST be stored.
7. Am I building a string that gets immediately discarded? The function MUST be split.
8. Could this error be a package-level sentinel?
9. Do I have multiple `var tb textbuf.Buffer` in one function? ONE buffer MUST be used, with `Reset()`.
10. Is my `.String()` result consumed immediately (function arg, comparison)? `.Slice()` MUST be used.
