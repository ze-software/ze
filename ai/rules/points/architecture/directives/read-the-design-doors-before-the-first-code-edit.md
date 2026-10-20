---
kind: directive
level: MUST
stage:
---
**`ai/doors/system.md` and the door covering a file MUST be read in full before a session's or a subagent's first edit of that file when it is Go (tests included), YANG or templ.** The covering door is every page under `ai/doors/` whose `<!-- door-base: ... -->` header names the longest whole-component prefix of the file's path; doors tied on that prefix are all owed, and a file no header reaches owes `system.md` alone. `writeDesignDoorsRead` in `internal/le/hookruntime/writeedit.go` refuses the Write, Edit or MultiEdit and names each unread door; a whole Read, or a whole `cat` with no pipe, is the read it counts. A new door joins the gate by its header alone.
