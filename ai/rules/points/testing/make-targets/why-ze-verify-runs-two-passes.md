---
kind: note
level:
stage:
---
`ze-precommit-verify` uses a two-pass strategy to avoid recompiling all 349 packages with
`-race` every time:
