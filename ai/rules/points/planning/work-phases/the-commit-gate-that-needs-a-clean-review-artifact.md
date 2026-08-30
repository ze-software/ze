---
kind: directive
level: MUST
stage:
---
**A closure commit MUST carry a CLEAN `./le spec session review record` artifact covering every reviewable file in it, whose hashes still match.** Any edit after the review invalidates it and forces a fresh pass, and a code-free closure still owes one. `CheckReview` (`internal/le/commit/review.go`) refuses the commit otherwise, and `review-override <reason>` is an explicit owner decision that records a verification-debt row.
**The artifact proves a fresh, hash-pinned, covering review EXISTS, and it MUST NOT be read as proof that an independent context did the reviewing.** Its coverage check sees only THIS commit, so commit all of a spec's code at closure.
