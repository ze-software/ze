---
kind: note
level:
stage:
---
`tmp/` is shared by every concurrent session in this checkout (it is keyed
per-checkout, not per-session -- `scripts/dev/ensure-links.py`). A fixed name at
the `tmp/` root -- `tmp/out.log`, `tmp/stdout`, `tmp/gotest.log` -- collides with
a sibling session writing the same name, and is never cleaned when your session
ends.
