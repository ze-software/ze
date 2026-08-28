---
kind: table
level:
stage:
---
| Requirement | How |
|-------------|-----|
| Apply/Undo pipeline | `fakeOps` scripted tests in `apply_test.go` covering create, update, delete, partial-failure undo, and reconciliation |
| Translate functions | Pure-function unit tests in `translate_test.go` for every supported config shape |
| Verify/reject logic | `verify_test.go` asserting accepted configs pass and unsupported configs return clear errors |
| Registration side-effects | `register_test.go` confirming `init()` wires the backend into the correct registry |
