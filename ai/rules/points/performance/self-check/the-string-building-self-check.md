---
kind: directive
level:
stage:
---
1. Am I using `+` to concatenate strings? Use `textbuf.Buffer` chain instead.
2. Am I using `fmt.Sprintf`? Use `textbuf.Buffer` or standalone functions.
3. Am I using `strings.Join`? Use `textbuf.Join` or `b.Join(items, sep)`.
4. Am I using `strings.Builder`? Use `textbuf.Buffer` (128B inline, poolable).
5. Am I calling `.String()` just to concatenate? Use `textbuf.Buffer.Addr()` etc.
6. Am I storing a string that will be parsed back for comparison? Store the typed value.
7. Am I building a string that gets immediately discarded? Split the function.
8. Could this error be a package-level sentinel?
9. Do I have multiple `var tb textbuf.Buffer` in one function? Use ONE buffer with `Reset()`.
10. Is my `.String()` result consumed immediately (function arg, comparison)? Use `.Slice()`.
