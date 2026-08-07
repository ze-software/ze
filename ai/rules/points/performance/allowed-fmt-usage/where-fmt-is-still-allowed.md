---
kind: table
level:
stage:
---
| Context | Why |
|---------|-----|
| CLI output (`fmt.Println`, `fmt.Fprintf(os.Stdout, ...)`) | User-facing, runs once |
| Startup/shutdown messages | Runs once |
| Config parsing/validation errors | Runs once at load |
| Web page rendering (cold path) | Acceptable if not in a per-route loop |
| Test assertions and sub-test naming | Not production code |
| `fmt.Errorf("context: %w", err)` with non-constant context | Error wrapping is the intended use |
| `fmt.Sprintf("%T", value)` | Reflect-based type name; no textbuf equivalent exists |
| `fmt.Sprintf("%v", data)` where `data` is `any`/`interface{}` | Arbitrary-type formatting; no textbuf path |
| `http.Error(w, fmt.Sprintf(...))` | One-shot error response, not in a loop |
