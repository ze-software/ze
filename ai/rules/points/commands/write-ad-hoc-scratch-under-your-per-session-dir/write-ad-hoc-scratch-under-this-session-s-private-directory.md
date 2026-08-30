---
kind: directive
level: MUST
stage:
---
**Ad-hoc scratch MUST be written under this session's private directory,
`dir=$(./le session scratch ensure)`, and MUST NOT be written at the `tmp/`
root.** `tmp/` is keyed per checkout, so a fixed name there is one file for every
session in the tree and nothing removes it. Both write surfaces refuse it. Which
root names are shared by design, and what outlives a session, is
`docs/contributing/running-commands.md`.
