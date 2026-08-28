---
kind: note
level:
stage:
---
Use `./le commit create` for commit script preparation. `internal/le/commit`
owns session ID reuse, message creation, explicit add/remove validation,
executable script generation, and the pre-staging gates. Run the path printed
by its `script=` line. A hand-written compatibility path is prohibited.
