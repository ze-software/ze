---
kind: directive
level: MUST
stage:
---
**The re-vendor deletes ~60 tracked files and adds ~60 new ones. You MUST NOT use bare `git rm`/`git add`: you MUST stage the whole change through the commit-helper script at closure so the deletion and addition land in one commit.**
