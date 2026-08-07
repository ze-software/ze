---
kind: note
level: MUST
stage:
---
Prefer converting the sleep to a deterministic wait
(`ze_api` `wait_until` / `wait_for_event` / `dispatch_until`, see
"Python Observer API" below). Only when conversion is not possible
does the sleep stay, and then it MUST be justified.
