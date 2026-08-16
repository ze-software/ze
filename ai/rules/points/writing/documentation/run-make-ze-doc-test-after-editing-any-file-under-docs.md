---
kind: note
level:
stage:
---
Run `make ze-doc-verify` after editing any file under `docs/`, after adding or removing a plugin, or after touching a YANG `ze:command` declaration. The umbrella target runs `check-doc-drift` (validates doc counts/lists and narrow stale-claim checks), `validate-commands` (validates YANG `ze:command` <-> RPC handler contract), and the source-anchor stale-path check. These fail the make target on drift and report all issues found.
