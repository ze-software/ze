---
kind: note
level:
stage:
---
`make ze-rfc-check` reads the WORKING TREE to judge coverage, and a tree cannot tell
"never proven" from "stopped being proven". Seven comparisons against HEAD supply that
difference. Each fires only on a real downgrade, so a green run means the evidence held,
not that nobody looked.
