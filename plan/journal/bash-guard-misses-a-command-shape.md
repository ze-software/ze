# Bash guard misses a command shape

A Bash hook guard that judges a line by its text reads one shape of a command
and lets another shape of the same command through: an option value read as
the command word, a flag the text match never names, a boundary character the
pattern does not list. The forbidden command then runs with no refusal.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-11 | spec-agent-economy-1-call-shape | Bash hook `bashLossyPipe`, `bashRawHeavy` | `bash -o pipefail -c` running `./le test unit all` piped through `tail -5`, and `bash -O extglob -c 'go test ./x'` pass: the text judgement (`commandExpensive`, `rawHeavy`) reads the option value as the command word, so the piped expensive run and the raw test inside `-c` are not seen | open: skip a launcher's option and its value before reading the command word, keeping the text verdict fail-closed |
| 2026-11-11 | spec-agent-economy-1-call-shape | Bash hook `bashRootBuild` | `go build -toolexec=x -o bin/y ./cmd/y` passes: on a line judged by its text the guard reads only the `...` pattern and the `-o` path, so a build whose toolchain step runs an arbitrary program is admitted as a bin/ build | open: refuse `-toolexec`, `-overlay` and `-modfile` in the text judgement too |
| 2026-11-11 | spec-agent-economy-1-call-shape | Bash hook `bashSystemTmp` | `cat ${v}/tmp` unquoted passes: the `systemTmp` pattern takes no `}` as a boundary before `/tmp`, so a parent the shell expands to nothing reaches /tmp unseen | open: add `}` to the boundary class of `systemTmp` |
