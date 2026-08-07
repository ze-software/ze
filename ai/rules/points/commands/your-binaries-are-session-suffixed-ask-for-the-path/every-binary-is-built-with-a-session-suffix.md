---
kind: note
level:
stage:
---
Under an AI session every canonical binary is built as `bin/<name>-<session-id>`
(`mk/session.mk`), so a sibling session's `make ze` cannot overwrite the binary
you are testing against. Off-session (a human shell, CI) the name is the plain
`bin/ze` it always was.
