# Help answers unknown for its own command

A command whose help page is generated from a registry looks the name up in
that registry alone. Its OWN name is registered somewhere else, so asking the
command for help about itself answers "unknown command" and prints the page
that says so. The answer is well formed, which is what makes it expensive: a
reader takes it as a statement about the command they asked about.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-07 | spec-web-cli-config-one-off-defects (AC-11) | `ze plugin command help` | `ze plugin command help help` answers unknown instead of generated usage because its registry excludes builtin commands. | fixed: `zeDispatch` uses `declaredCommandHelp` and `commandHelpPage` before the root handler for exact declared paths. `TestRootHandledCommandHelpStatesGeneratedUsage`, `TestRootHandledCommandHelpNamesDeclaredArgument` and `TestRootCommandStillOwnsItsOwnHelp` pass, including help-for-help and both root controls. |
