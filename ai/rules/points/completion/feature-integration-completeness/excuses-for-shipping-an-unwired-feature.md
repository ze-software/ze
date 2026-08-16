---
kind: table
level:
stage:
---
| Banned | Why |
|--------|-----|
| "Deferred to next spec" | Next spec won't pick it up. Feature ships unwired. |
| "Requires infrastructure not yet built" | Then the feature is blocked, not done. |
| "Unit tests cover the logic" | Unit tests prove the algorithm, not the wiring. |
| "make ze-precommit-verify passes" | Passing tests that don't exercise the entry point prove nothing. |
| "Go test exercises the handler" | A Go test with mocked entry points is not a `.ci` test. |
