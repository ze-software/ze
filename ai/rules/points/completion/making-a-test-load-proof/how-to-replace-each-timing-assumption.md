---
kind: directive
level: MUST
stage:
---
**Each timing assumption MUST be replaced by the fix in its row:**

| Symptom | Fix |
|---------|-----|
| `time.Sleep` then assert | poll the condition with `fixture.Poll`, using `fixture.Dispatch` when the state comes from the engine |
| fixed deadline for startup, teardown or reconnect | wait on the readiness signal the daemon emits. If none exists, ADD one: a missing signal is a product gap, not a test problem |
| "at most N events in a window" | count between two state transitions, not between two clock reads |
| assert immediately after a command returns | wait for the effect to be observable, then assert |
| the test genuinely needs a kernel-global surface to itself | `option=exclusive:group=<name>` (`internal/test/runner/record.go`), not a longer timeout |
| a timeout that is "generous enough" | generous is a synonym for unknown. Bound it by a condition |
