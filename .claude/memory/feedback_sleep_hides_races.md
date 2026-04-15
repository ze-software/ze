---
name: Sleep-based tests hide concurrency bugs
description: Replacing time.Sleep with proper synchronization exposes real data races -- treat sleep removal as a bug-finding technique, not just test cleanup
type: feedback
originSessionId: e7a7ff86-ec52-4b35-a8cf-057497053ae4
---
Replacing `time.Sleep` in tests with proper synchronization (channels, `require.Eventually`, ready signals) is not just a test quality improvement -- it actively finds concurrency bugs.

**Why:** The `peer.go` `printf` method had a data race (shared `bytes.Buffer` written from multiple goroutines without a mutex). The old `Sleep(50ms)` masked it because only one connection ever arrived before the test proceeded. Replacing the sleep with a TCP probe created a second connection and the race detector caught it immediately.

**How to apply:** When removing `time.Sleep` from tests, always run with `-race`. The synchronization change itself may expose bugs that the sleep was accidentally hiding. If a race appears, fix the production code (add the mutex), don't revert to the sleep.
