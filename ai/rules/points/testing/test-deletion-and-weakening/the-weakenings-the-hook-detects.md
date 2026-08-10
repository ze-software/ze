---
kind: directive
level: MUST NOT
stage:
---
**These weakenings MUST NOT be introduced:**

- adding `t.Skip` / `t.Skipf` / `t.SkipNow` (the test stops running)
- removing assertions (any net drop, not only all-removed)
- downgrading fatal assertions to non-fatal (`require` -> `assert`, `t.Fatal` -> `t.Error`)
- commenting out assertions
- adding an `ignore` build tag (file dropped from the build)
- deleting a `Test`/`Fuzz`/`Benchmark` func, `t.Run` cases, or table rows
- removing `.ci` test lines
