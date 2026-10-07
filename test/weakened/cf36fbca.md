# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| TestArchivePrefix | ArchivePrefix is deleted: its prefix never matched a dated archive name, so commit-revisions pruned nothing. ArchiveMatcher replaces it, and TestArchiveMatcher checks the default format's own names, including that a shared prefix with the wrong host or a leading word does not match |
| TestArchivePrefix_NoTimeTokens | Same deletion. TestArchiveMatcher_NoTimeTokens keeps the case: a format with no {date} or {time} matches exactly ze-r1.conf, and pruning leaves both that file and a foreign .conf |
