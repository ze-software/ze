---
kind: table
level:
stage:
---
| Gate | Reads | Missed by a `// Design:`-only grep |
|------|-------|-------------------------------------|
| `./le doc check links` (`internal/le/doc/check`) | `// Design:` lines and tracked path citations | no |
| `./le spec citation` (`internal/le/spec/citation`) | every `plan/spec-*.md` citation inside a spec | yes |
| `internal/le/doc/check.CheckLinks` tracked-citation pass | every tracked path citation, including a `plan/spec-*.md` target | yes |
