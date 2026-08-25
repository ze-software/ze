---
kind: directive
level: MUST
stage:
---
The sleep MUST be converted to a deterministic wait (`ze_api` `wait_until` /
`wait_for_event` / `dispatch_until`, see "Python Observer API" below) whenever a
condition exists to wait on. Only when no such condition exists does the sleep
stay, and then it MUST be justified. See "Try a sync primitive before you write
a sleep" below for what trying means and how the comment records it.
