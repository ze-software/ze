---
kind: directive
level: MUST NOT
stage:
---
**A build in `bin/builds/` MUST NOT be deleted, and `bin/le`, the installed copy of the `./le` dispatcher, MUST NOT be deleted or edited: several sessions share this checkout, so a removed build can be the one a peer is executing.** A build that misbehaves is taken out with `./le rollback`, which moves the newest build aside to `bin/builds/.failed-<name>` and makes the next call run the one before it; that is the only removal a session performs. A session that needs `./le` to carry its own edits asks for a build of its own with `./le --name session <command>...` (or `--name <name>`), which lands in `bin/le-<name>/le` and is rebuilt from the working tree on every call. The option comes first and the launcher consumes it, so everything after it reaches the command unchanged.
**Anything that reads a result back about uncommitted edits MUST use a named build: an interop or functional run, a gate you are about to report on, and any command you run after editing `internal/le/`.** The shared builds carry HEAD's committed inputs, never the working tree, so a probe against them says nothing about an edit that is not committed. `cmd/ze/le_build_name.go` refuses to answer when the running binary is not the one you named, and it prints both names.
