---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `postFormatGo` | `quality.md` | existing Go Write/Edit | Runs gofmt, goimports, and changed-code lint, then blocks on reported lint issues. |
| `postFileSize` | Go style guidance, no rule point | production Go | Warns when a source file exceeds 1,000 lines. Advisory. |
| `postDeferral` | heuristic advisory, no rule point | governed Markdown | Warns when an edit adds deferral language outside the accepted ledgers. Advisory. |
| `postJournal` | journal format, no rule point | `plan/journal/*.md` | Runs the native journal row validator. Advisory and fail-speak. |
| `postRFCHeader` | Go style guidance, no rule point | production Go | Suggests an RFC design header when a source repeatedly references RFCs. Advisory. |
| `postTestDocs` | Go style guidance, no rule point | Go tests | Warns when a test file has tests and no VALIDATES or PREVENTS header. Advisory. |
| `postFuzz` | advisory, no rule point | wire parsing Go | Warns when a package with a Parse function has no fuzz test. Advisory. |
| `postVague` | Go style guidance, no rule point | production Go | Warns on the configured vague variable-name forms. Advisory. |
| `postBoundary` | advisory, no rule point | production Go with numeric validation | Warns when the companion test file has no boundary evidence. Advisory. |
