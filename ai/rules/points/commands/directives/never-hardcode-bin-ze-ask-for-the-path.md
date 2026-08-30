---
kind: directive
level: MUST NOT
stage:
---
**`bin/ze` MUST NOT be hardcoded in a command, a script or a doc: every binary a native test action builds lives in the CURRENT session's private directory under a bare name, so a sibling session cannot overwrite the binary under test.** Ask the owning action (`./le functional <suite>`) for the path it built.
