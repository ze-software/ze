---
kind: directive
level: MUST
stage:
---
**Anything that downloads into `gokrazy/modcache/` MUST carry `-modcacherw` (`GOFLAGS=-modcacherw`).** Go's default read-only cache permissions leave directories `r-x`, which makes git unable to delete or overwrite modcache files on a later checkout or rebase. Which tools already set the flag, and how to repair a cache written without it, is `docs/architecture/appliance/gokrazy-build-pins.md`.
