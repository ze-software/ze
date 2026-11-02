---
kind: directive
level: MUST
stage:
---
**A probe that cannot run MUST answer Unknown with the reason it could not run, and its caller MUST NOT read Unknown, or its own failure to look, as the absence of the thing it asked about.** A capability probe the sandbox refuses, or a directory the current user cannot read, says nothing about the kernel feature or the file. Read as absent, it selects the wrong test bounds, or rebuilds on every launch what was already there. The caller decides what Unknown means for its own action and names that choice: skip with the reason, refuse, or ask again from a place where it can look.
