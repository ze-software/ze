| Test | Reason |
|------|--------|
| TestEveryCommandIsFoundAtThePathItsNamePredicts | Not weakened: one assertion MOVED, none removed. The reverse-direction walk became the helper `registeringDirectories`, which now carries the `t.Fatalf` for an unreadable `internal/le`, so the count inside the test body falls from four to three while the same four conditions are still asserted. The helper walks two levels instead of one, so the test covers strictly more than before |
