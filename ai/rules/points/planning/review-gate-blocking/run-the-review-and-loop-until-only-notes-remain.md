---
kind: note
level:
stage:
---
Before final testing/verify, run a code review against the diff. Fill the
`## Review Gate` section in the spec with the findings list. If ANY finding
is severity BLOCKER or ISSUE (anything above NOTE), fix it and re-run the
review. Loop until the review returns only NOTEs (or nothing). Paste the
final clean review output into the spec. NOTE-only findings do NOT block.
