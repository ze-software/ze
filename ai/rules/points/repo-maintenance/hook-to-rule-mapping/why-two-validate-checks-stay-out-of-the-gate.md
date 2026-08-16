---
kind: note
level:
stage:
---
The two changed-file checks take `changed_files` as their subject, which is `git diff HEAD` plus untracked files. Several sessions share this checkout, so that list includes other sessions' half-written work. Both checks demand completeness that a file in the middle of an edit cannot show. They therefore stay out of the gate.
