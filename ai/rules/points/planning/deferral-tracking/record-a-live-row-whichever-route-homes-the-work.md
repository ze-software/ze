---
kind: directive
level: MUST NOT
stage:
---
**Both routes record a LIVE row.** Filing work in a spec is not finishing it, so the
row stays `deferred` and keeps naming its home until the work lands. MUST NOT close a
row at step 2 or 3: a `done` row is never destination-checked again, so closing it on
filing is precisely how the work stops being watched (see "Status Vocabulary (the gate reads this)").
