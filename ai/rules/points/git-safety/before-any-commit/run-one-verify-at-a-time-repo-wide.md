---
kind: note
level:
stage:
---
One `make ze-precommit-verify*` (or `ze-chaos-verify`) at a time repo-wide --
parallel runs share build cache + ports + `bin/ze` processes and
trash each other. All variants are wrapped by
`scripts/dev/verify-lock.sh` (`flock` on `tmp/.ze-verify.lock`); a
second invocation blocks automatically.
