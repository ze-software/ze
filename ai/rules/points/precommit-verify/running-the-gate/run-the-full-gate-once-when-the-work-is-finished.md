---
kind: directive
level: MUST
stage:
---
**`make ze-precommit-verify` is a 25-stage full gate and takes 25 to 30 minutes. You MUST run it ONE
time, when the work is finished and you are about to prepare the commit script.**
Running it to "check in" mid-change is the single most expensive habit available in
this repository, and it buys nothing a scoped check does not.
