---
kind: directive
level: MUST NOT
stage:
---
**Your own inline reasoning about code you just wrote MUST NOT count as a review.** The
author is the one party guaranteed to share the blind spot that produced the bug.
Writing "I checked it, 0 issues" into a Review Gate from your own analysis is the
exact failure this rule exists to stop. It has shipped real bugs that independent
reviewers caught on the same diff minutes later.
