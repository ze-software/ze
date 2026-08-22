| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | - | lint | `prealloc` v1.0.2 nil-dereferenced on `for range iter.Seq[rpc.Record](rows)` and took all of `goanalysis_metalinter` down. The runner error named the package, never the expression, so the crash reads as a broken tool rather than as one new line of source | wrote the sequence into a typed variable, so the range expression is an identifier instead of a generic conversion |
