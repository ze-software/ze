---
name: Lint and flaky test fixes completed (2026-03-20)
description: All 5 lint issues and 3 flaky tests fixed. make ze-verify passes clean.
type: project
---

**All resolved** (2026-03-20):
- 5 lint issues fixed in commit 01d4ef37
- 3 flaky tests fixed in commit 34ecdaf0

**Why these were flaky:**
- `TestSessionIBGP`: reactor `Stop()` didn't wait for cleanup, causing port reuse races
- `cli-completion-show-targets`: real bug, not flaky -- `completeShowPath` was missing YANG schema children
- `68 ipv6`: 0.2s inter-message sleep too tight under load

**How to apply:** If similar flakiness recurs, check reactor cleanup waits and plugin test timing.
