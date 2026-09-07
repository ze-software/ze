# Help answers unknown for its own command

A command whose help page is generated from a registry looks the name up in
that registry alone. Its OWN name is registered somewhere else, so asking the
command for help about itself answers "unknown command" and prints the page
that says so. The answer is well formed, which is what makes it expensive: a
reader takes it as a statement about the command they asked about.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | daemon-backed-command-catalog | `ze plugin command help`, judged by `TestRootHandledCommandHelpStatesGeneratedUsage` (`cmd/ze/ze_core_dispatch_help_test.go`) | Red at HEAD with that test file and `internal/component/plugin/cli/` both clean in the working tree. `ze plugin command help help` writes the unknown-command answer instead of the generated usage the test asks for. The command's own doc text states the cause: "The name is looked up in the plugin registry only, so a built-in command is reported as unknown." Walked into while running `go test ./cmd/ze/` for Phase 3 of this spec | not fixed, and not this spec's surface |
