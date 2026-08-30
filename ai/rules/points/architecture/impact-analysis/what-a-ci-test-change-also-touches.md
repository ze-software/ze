---
kind: directive
level: MUST
stage:
---
**A change to a `*.ci` functional test MUST also check everything its row names.**

| What changed | Also check |
|---|---|
| New test file | The correct directory (`ai/rules/testing.md`, test directories table) |
| Compiled observer | That the failing `internal/test/fixture` callback returns an error (`ai/rules/testing.md`) |
| Config in `tmpfs=` | That a parse test validates its syntax |
