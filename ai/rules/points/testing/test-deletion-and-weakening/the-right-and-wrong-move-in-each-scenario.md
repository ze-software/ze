---
kind: table
level:
stage:
---
| Scenario | Correct | Wrong |
|----------|---------|-------|
| New bug in `parsePeer`, existing `TestParsePeer` | Add `TestParsePeerRejectsEmpty` alongside `TestParsePeer` | Rewrite `TestParsePeer` to test the new edge case |
| Table-driven test, new case needed | Add a row to the table | Replace an existing row with the new case |
| Existing test fails because code changed | Fix the code so both old test and new test pass | Rewrite the old test to match the changed (broken) code |
