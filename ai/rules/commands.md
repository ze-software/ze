# Running Commands

**When:** running any test, build, lint, or verification command from Bash, or writing a shell loop that forks or waits
**Severity:** blocking

## Directives

- **Every test, build, lint or verify command MUST run through the registered `./le` action that owns it.** A bare `go test` omits Ze's feature build tags, so plugins never register and unrelated packages fail with a phantom red; a `ze-test` runner launched directly rebuilds a daemon without its test-only surface and gives a convincing false red; a bare `golangci-lint` inherits host defaults and reports an environment failure as a code finding. The tag list, the suite table and the log paths are `docs/contributing/running-commands.md`.
- **A command started with `run_in_background` re-invokes the session when it exits, so that notification IS the wait: a polling loop MUST NOT be written.** No `while` or `until` around `sleep`, no `pgrep` in a loop condition, and no shell loop that forks one process per input where one invocation takes them all.
- **The harness's own Bash and Monitor examples show an unbounded `until ... sleep` loop, and this repository overrides them.** A poll that is genuinely the only signal MUST be wrapped in `timeout <seconds>`, MUST NOT wake faster than once every 30 seconds, and MUST be stopped the moment its reason changes. Each wake competes with QEMU, Docker and the verify gate for the same cores, so a watcher can corrupt the run it watches.

- **`go test`, lint analysis and a `ze-test` runner MUST NOT start raw from Bash.** The generic admission grammar is `./le job run label <label> [quiet] command <argv...>`: one job runs while peers queue, and the child exit status is preserved.
- **A run whose output you do not want in the transcript MUST take `quiet`, and MUST NOT be wrapped in a redirect to a scratch path, an exit-code echo and a `grep` over the log.** `quiet` writes the child's output to this session's scratch log and answers the exit code, that log's path and its failure lines. A hand-written wrapper picks its own lines to keep, and the job log it redirects past is removed when the job ends.
- **A one-off that MUST NOT queue states its reason in the command: `ZE_ADMIT_RAW="<reason>" <command>`.** An empty reason admits nothing, and the reason lands in the transcript, which is what makes the escape auditable by reading the session.

- **A file under `plan/` or `ai/rules/` MUST NOT be written from Bash: use the Write or Edit tool, which are the only surfaces the native writeedit checks run on.** The guard binds redirects, in-place editors, `tee`, `cp`, `mv` and interpreter writes, and it refuses a command that merely NAMES those trees beside a write primitive. Reading with `grep`, `cat` or `sed -n` stays free.
- **A refusal that is wrong is answered by `ZE_ADMIT_GOVERNED_WRITE="<reason>"`, and MUST NOT be answered by rewording the command.** An empty reason admits nothing. A false positive costs one environment assignment; a false negative costs the guard.

**`bin/ze` MUST NOT be hardcoded in a command, a script or a doc: every binary a native test action builds lives in the CURRENT session's private directory under a bare name, so a sibling session cannot overwrite the binary under test.** Ask the owning action (`./le functional <suite>`) for the path it built.

**`bin/le` MUST NOT be deleted: the launcher treats it as an existence cache, and several sessions share this checkout, so `rm bin/le` can take the binary out from under a peer that is executing it.** A session that needs `./le` to carry its own edits asks for a build of its own with `./le --name <name> <command>...`, which lands in `bin/le-<name>/le` and is rebuilt on every call. The option comes first and the launcher consumes it, so everything after it reaches the command unchanged.
**Anything that reads a result back MUST use a named build: an interop or functional run, a gate you are about to report on, and any command you run after editing `internal/le/`.** The shared binary answers with whatever was compiled the last time it was absent, so a probe against it is evidence about a binary nobody ships. `cmd/ze/le_build_name.go` refuses to answer when the running binary is not the one you named, and it prints both names.

**A `bin/le` behind the COMMITTED tree MUST be refreshed with `./le --update`, and MUST NOT be refreshed by deleting it.** The launcher renames the new build into place, so a peer keeps running the inode it started on. A command after `--update` runs against the new build, and a failed build leaves the binary as it was.
**The launcher MUST NOT be expected to rebuild on its own, because one peer's half-written source would fail every call in every session.** About one call in sixteen says on stderr that the update is owed. A file git holds as modified never counts, so a peer's work in progress is silent.

**First-party Go compilation MUST set `CGO_ENABLED=0` in the process environment, covering binaries, tests, benchmarks, fuzzing, `go run`, nested helpers and installed project tools; an inherited CGO default MUST NOT be relied on.** A test-only command that uses `-race` MAY set `CGO_ENABLED=1`, and a race binary MUST NOT ship or serve as build evidence.

## Pipes

**`./le`, `go test`, `go build`, `golangci-lint`, `bin/ze*` and any other test, verify or build command MUST NOT be piped through `head`, `tail`, `grep`, `awk`, `sed` or `cat`.** Run it clean, then read the log; losing one failure line costs the whole re-run. `| tee <file>` is the one allowed pipe, because it is not lossy.

## Scratch

**Ad-hoc scratch MUST be written under this session's private directory, `dir=$(./le session scratch ensure)`, and MUST NOT be written at the `tmp/` root.** `tmp/` is keyed per checkout, so a fixed name there is one file for every session in the tree and nothing removes it. Both write surfaces refuse it.
