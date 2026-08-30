# A diagnostic bypasses the writer it was given

A command is handed an output stream and an error stream by its caller, and
then writes its failure straight to the process. The caller sees an empty
stream, so nothing above the command can capture, compare or re-route what a
person would have read.

The tell is a `fmt.Fprintln(os.Stderr, ...)` inside a function whose caller
already accepted an `io.Writer` for exactly that purpose.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-26 | le-is-a-ze-binary | `leaction.ReportError`, `Area.refuseVerb`, and `Area.refuseValue` (`leaction/leaction.go` in the former top-level tool tree) | All three functions write directly to `os.Stderr`. But `leroot.Run(name, answer, args, out, errOut)` accepts an error writer from its caller. Every ported `le` tool reports through these functions. Thus, a caller-supplied writer captures empty stderr. `devPyRunCommand` (the retired `scripts/dev/parity_python_test.go` (current producer: `internal/le/`)) supplies such a writer for every migration parity case. It cannot compare the refusals from the two implementations | not fixed. The defect reaches every ported tool and belongs with the swap instead of with one port. The `test-health` parity cases avoid it by calling the tool's function and reading its returned error (`healthDiagnosis`, the retired `scripts/dev/testing_health_parity_test.go` (current producer: `internal/le/`)). This proof is weaker than a comparison of the stream that a person sees |
