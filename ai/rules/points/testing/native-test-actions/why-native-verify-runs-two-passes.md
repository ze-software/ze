---
kind: note
level:
stage:
---
`./le verify current mode full` uses a two-pass strategy to avoid recompiling all 349 packages with
`-race` every time:
