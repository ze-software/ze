---
kind: directive
level: MUST
stage:
---
**The `## Design decisions` section of every page a file's `// Design:` header names MUST be read, as it stands, before a session's or a subagent's edit of that file when it is Go (tests included), YANG or templ, and before an edit of a spec that names the file or the page.** Both the header on disk and the header the edit proposes count, every `// Design:` line of each; a page with no such section owes nothing, because it records no owner decision, and a row added to the section owes it again. A header MUST name a page that exists. `writeDesignDoorsRead` (`internal/le/hookruntime/writeedit.go`) and `writeSpecDoorsRead` (`internal/le/hookruntime/specdoors.go`) refuse the edit beside the unread doors and print the Read that clears each section.
