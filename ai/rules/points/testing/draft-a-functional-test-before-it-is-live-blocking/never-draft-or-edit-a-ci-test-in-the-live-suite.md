---
kind: note
level:
stage:
---
Never write or iterate on a `.ci` inside `test/<suite>/`, and never edit a live
one in place. That directory runs on every `make ze-verify` in the checkout,
including runs by OTHER sessions, who then have to work out whether your
half-written test is their regression.
