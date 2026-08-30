---
kind: directive
level: MUST NOT
stage:
---
**`fmt` MAY be used in these contexts, and MUST NOT be used anywhere else on a hot path.** Each one runs once, or has no textbuf equivalent. The replacement tables are `docs/architecture/textbuf-string-building.md`.

| Context | Why |
|---------|-----|
| CLI output (`fmt.Println`, `fmt.Fprintf(os.Stdout, ...)`) | User-facing, runs once |
| Startup and shutdown messages | Runs once |
| Config parsing and validation errors | Runs once at load |
| Web page rendering on a cold path | Acceptable outside a per-route loop |
| Test assertions and sub-test naming | Not production code |
| `fmt.Errorf("context: %w", err)` with non-constant context | Error wrapping is the intended use |
| `fmt.Sprintf("%T", value)` | Reflect-based type name; textbuf has no equivalent |
| `fmt.Sprintf("%v", data)` where `data` is `any` | Arbitrary-type formatting; textbuf has no path |
| `http.Error(w, fmt.Sprintf(...))` | One-shot error response, not in a loop |
