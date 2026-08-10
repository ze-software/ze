---
kind: directive
level: MUST
stage:
---
**The status record is what forces the second pass, so plan the FIRST one to be
the last.** `commit_helper.py create` refuses unless
`scripts/dev/verify-status.sh check` reports FRESH, and only a full `ze-verify`
writes that record. A narrow fix therefore still needs one more full run before a
commit, which is precisely why the full run MUST come AFTER every gate you can
check cheaply is already green: `make ze-lint`, the touched packages' `go test`,
and the gate owning each surface you changed. A run started before those are clean
is a run you will pay for twice.
