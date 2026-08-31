---
kind: directive
level: MUST NOT
stage:
---
**`bin/le` MUST NOT be deleted: the launcher treats it as an existence cache, and several sessions share this checkout, so `rm bin/le` can take the binary out from under a peer that is executing it.** A session that needs `./le` to carry its own edits asks for a build of its own with `./le --name <name> <command>...`, which lands in `bin/le-<name>/le` and is rebuilt on every call. The option comes first and the launcher consumes it, so everything after it reaches the command unchanged.
**Anything that reads a result back MUST use a named build: an interop or functional run, a gate you are about to report on, and any command you run after editing `internal/le/`.** The shared binary answers with whatever was compiled the last time it was absent, so a probe against it is evidence about a binary nobody ships. `cmd/ze/le_build_name.go` refuses to answer when the running binary is not the one you named, and it prints both names.
