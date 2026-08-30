---
kind: directive
level: MUST NOT
stage:
---
**Each line below is a signal that you are about to skip the independent pass. Thinking one MUST NOT be treated as a reason to skip it: stop and spawn the reviewers.**

| Banned | Reality |
|--------|---------|
| "I reviewed it as I wrote it." | That is authoring, not reviewing. Same blind spot. |
| "The tests pass, so it's correct." | Tests can be vacuous (dead exit codes, cumulative-match needles). A reviewer finds the vacuous test; the green bar does not. |
| "It's a small/mechanical change." | Renames collide roots; one-line guards fail open. Size is judged after review. |
| "I already know this code is correct." | The bug is precisely what you're sure isn't there. |
| "./le repository check / lint passed." | Those are mechanical gates. They are not a critical review. |
| "Re-running review is wasteful." | The fix is new code. Unreviewed new code is the next bug. |
