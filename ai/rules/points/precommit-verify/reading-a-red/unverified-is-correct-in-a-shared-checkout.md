---
kind: directive
level: MUST
stage:
---
**`--unverified` is the CORRECT path in a shared checkout, not a shortcut.** It
exists for exactly this: a full-tree gate whose red belongs to somebody else's
in-flight work. Its own text names the owner override and a failure you tried and could not reproduce;
concurrent-session interference is the third case, and the reason MUST say so.

**Since 2026-08-21 it unlocks nothing, and that is what makes it worth writing.**
A stale verify records a verification-debt row whether or not the flag is given,
and `--push` refuses while that row is open. The flag fills the row's Reason
cell. The checker can say the record is STALE; only a caller can say whose red it
is and which run will cover this commit, and every judgement in this rule is
built on that attribution.
