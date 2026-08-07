---
kind: note
level:
stage:
---
Both halves of "Fix, don't record" were paid for on 2026-07-26. A shard argued at
length that a rotating failure set proved non-determinism, when a rotating set
across teardown-shaped tests is the signature of one shared timing assumption. The
diagnosis was sitting unread inside its own record.
