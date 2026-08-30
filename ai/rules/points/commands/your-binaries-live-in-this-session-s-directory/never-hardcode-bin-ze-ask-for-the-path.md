---
kind: directive
level: MUST NOT
stage:
---
**`bin/ze` MUST NOT be hardcoded in a command, a script, or a doc. MUST ask the
owning native action (`./le functional <suite>`) for the path it built.** Every
binary a native test action builds lives in the current session's private
directory under a bare name, so a sibling session cannot overwrite the binary
under test. Why the path is looked up rather than recomputed, and how the
session store is seeded, is `docs/contributing/running-commands.md`.
