---
kind: directive
level: MUST
stage:
---
- If you debug something, MUST add a test so it's never re-investigated
- Implementation written before its test → MUST back-fill the test. Working product code MUST NOT be deleted to restore the ordering (`ai/rules/pre-release.md`)
- Test passes immediately → invalid test, MUST add failing assertion
- Claiming "done" without test output → MUST run it once, paste it
