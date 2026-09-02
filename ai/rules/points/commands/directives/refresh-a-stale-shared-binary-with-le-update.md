---
kind: directive
level: MUST
stage:
---
**A `bin/le` behind the COMMITTED tree MUST be refreshed with `./le --update`, and MUST NOT be refreshed by deleting it.** The launcher renames the new build into place, so a peer keeps running the inode it started on. A command after `--update` runs against the new build, and a failed build leaves the binary as it was.
**The launcher MUST NOT be expected to rebuild on its own, because one peer's half-written source would fail every call in every session.** About one call in sixteen says on stderr that the update is owed. A file git holds as modified never counts, so a peer's work in progress is silent.
