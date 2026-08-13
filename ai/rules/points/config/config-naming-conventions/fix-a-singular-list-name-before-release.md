---
kind: directive
level: MUST
stage:
---
**Ze has not shipped a release. So a singular `leaf-list` name MUST be corrected now, and the code that reads it fixed with it.**
A change whose only purpose is grammar consistency is CORRECT work while nothing is released, and MUST NOT be refused as churn.
The BGP YANG holds several singular leaf-lists today: `import`, `export`, `tag`, `strip`, `send`, `value` and `receive` among them. Each costs one commit now.
**After the first release, a configuration name is an API, and a rename MUST NOT land without a configuration migration tool.**
A rename breaks every deployed configuration that writes the old name. An operator whose configuration stops parsing after an upgrade has no way forward without that tool.
Rationale: the `blackhole` container gained `community` and `authorized-covering-prefix` on 2026-08-13. Both moved to `communities` and `prefixes` the same day, for one commit, because nothing had been released. The same edit after a release is a migration project.
**The free period ends when the migration tool is owed, not on the release date alone.** MUST say which of the two states applies before you argue that a rename is too expensive.
