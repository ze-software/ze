---
kind: note
level:
stage:
---
`wait_until` / `dispatch_until` / `wait_for_event(predicate)` are the
payload-predicate waits: prefer them over `time.sleep` + a single-shot assert so
a test blocks exactly until the observed payload matches, not a guessed duration.
