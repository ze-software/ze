# Delegation hands the callee a different argv

A wrapper that runs another program IN PROCESS has to reconstruct what the
program would have received as a subprocess. Getting that wrong is invisible
from the outside: the callee's own argument parser rejects the argv and exits
non-zero, so the wrapper reports a failing gate rather than a failing call, and
the reader goes looking for the drift the gate was checking for.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-25 | port-test-functional-to-le | the retired `scripts/le/devtools/inproc.py` (current producer: `internal/le/`) `_call_args`, the retired `scripts/dev/spec-citation-check.py` (current producer: `internal/le/spec/citation/speccitation.go`) | `./le spec citation anchors` and `./le doc check verify` red with `spec-citation-check.py: error: unrecognized arguments: <its own path>`. The same script run as `python3 scripts/dev/spec-citation-check.py (retired; current producer: `internal/le/spec/citation/speccitation.go`)` exits 0. `_call_args` gives a `main(argv)`-shaped script a FULL argv, program name included, because `rfc_requirements.main` is written to be called as `main(sys.argv)` and drops the name itself. `spec-citation-check.py` is written the other way, `main(sys.argv[1:])` at its `__main__` guard, so the name arrives as an extra positional. A signature cannot distinguish the two conventions | Fixed in `bd30679b2`. `_passes_whole_argv` (the retired `scripts/le/devtools/inproc.py` (current producer: `internal/le/`)) READS the script's own `__main__` guard rather than guessing: `main(sys.argv[1:])` wins over `main(sys.argv)`, and the tail form is the default when a script has no guard, because such a script is called by `le` alone. Both conventions now run identically through `le` and standalone |
