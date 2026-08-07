---
kind: note
level:
stage:
---
On `gpg failed to sign` / `cannot open /dev/tty`, ask the user to run
`! echo test | gpg --clearsign` to unlock the agent, then re-run the
commit script.
