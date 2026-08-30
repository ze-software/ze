---
kind: directive
level: MUST
stage:
---
**Before anything destructive you MUST save a patch, write the destructive commands to `tmp/delete-SESSION.sh`, tell the user, and STOP.** The patch is `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`.
