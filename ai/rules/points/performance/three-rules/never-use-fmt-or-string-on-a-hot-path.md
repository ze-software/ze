---
kind: directive
level: MUST NOT
stage:
---
**On a hot path `fmt` MUST NOT be called, `.String()` MUST NOT be concatenated, `+` MUST NOT join two strings, and a value that will be compared MUST be stored typed (`netip.Addr` with `.Compare()`), never as a string.** Build text with `textbuf.Buffer` or an `AppendTo(buf []byte) []byte` method instead, and release a pooled buffer whose `Slice()` you took. `fmt` MAY still be used where it runs once and has no textbuf equivalent: CLI output, startup and shutdown messages, config load errors, a cold web render, tests, `fmt.Errorf("...: %w", err)`, `%T`, `%v` over an `any`, and a one-shot `http.Error`; a compile-time `const x = "foo" + "bar"` MAY use `+` because the compiler folds it, and a cold-path `+` is converted on touch rather than swept. The replacement tables are `docs/architecture/textbuf-string-building.md`.
