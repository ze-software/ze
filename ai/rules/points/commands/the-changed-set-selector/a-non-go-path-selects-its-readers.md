---
kind: note
level:
stage:
---
A non-Go path seeds the Go packages whose tests read it, so a `.ci` or rule
point selects native tooling packages rather than nothing. The `paths-from`
keyword asks `./le changed scope` about a supplied path list.
