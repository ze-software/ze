---
kind: directive
level: MUST
stage:
---
**`ai/doors/system.md` and the door of every subsystem a spec targets MUST be read in full before a session or a subagent writes or edits that spec (`plan/**/spec-*.md`).** The owed doors are those covering any repository path the spec names, in the spec on disk or in the text the edit proposes, found through the same `door-base` coverage the code gate uses; a brand-new spec that names no path owes `system.md` alone. `writeSpecDoorsRead` in `internal/le/hookruntime/specdoors.go` refuses the Write, Edit or MultiEdit and names each unread door; a whole Read, or a whole `cat` with no pipe, is the read it counts. A spec rewritten from habit drops a contract the doors hold, such as third-party plugins in any language.
