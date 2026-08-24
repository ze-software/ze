---
kind: note
level:
stage:
---
Under an AI session every canonical binary is built into this session's own
directory, under its BARE name:
`tmp/session/<YYYY-MM-DD>-<session-id>/bin/ze` (`mk/helper-session.mk`). A sibling
session's `make ze-build` therefore cannot overwrite the binary you are testing
against. Off-session (a human shell, CI) the path is the plain `bin/ze` it
always was.
