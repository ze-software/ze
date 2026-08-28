---
kind: directive
level: MUST NOT
stage:
---
**A numeric id is a position, not an identity (BLOCKING for anything you keep).**
The runner's one-based ordinal is an internal display position over a sorted
fixture population. Adding or renaming an earlier fixture silently renumbers
later rows. MUST use the stable scenario or Go test name in any verification
command, handover, gate subset, or evidence claim.
This ratchet exists because a concurrent session added `.ci` files and moved id
373 from `resolve-ping` to `remove-private-as-replace-peer` while an id-driven
script reported green for tests it never ran.
