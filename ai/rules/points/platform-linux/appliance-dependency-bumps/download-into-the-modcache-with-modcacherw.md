---
kind: directive
level: MUST
stage:
---
**Anything that downloads into `gokrazy/modcache/` MUST carry `-modcacherw` (`GOFLAGS=-modcacherw`):** go's default read-only cache permissions (dirs `r-x`) make git unable to delete or overwrite modcache files on later checkouts and rebases (a `git pull --rebase` across the 2026-07 init bump wedged exactly this way).
