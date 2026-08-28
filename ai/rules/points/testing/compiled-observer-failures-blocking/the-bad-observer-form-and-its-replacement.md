---
kind: table
level:
stage:
---
| Bad | Good |
|-----|------|
| `fmt.Fprintln(os.Stderr, "FAIL: ..."); return nil` | `return errors.New("reason")` |
| Relying on `expect=exit:code=0` to catch observer failures | Return an error and add an explicit assertion on the production result where possible |
| `time.Sleep(N)` then an informational line with no failure path | Use `fixture.Poll`; return an error when it exhausts |
