---
kind: directive
level: MUST NOT
stage:
---
| A `--flag` MUST NOT be | Why | Use instead |
|------------------------|-----|-------------|
| A command name | A flag that dispatches is a verb in disguise: it enters no tree, so completion, `ze help command` and the grammar gate never see it | a registered root, or a path under a read verb |
| One of a mutually exclusive set | Several booleans of which exactly one is legal is a closed keyword set the type system is not checking | one keyword slot |
| A second spelling of a pipe operator | `--json` and `\| json` are one job under two names, and only one of them composes. A flag MAY set a session default only by lowering into the operator, as `commandWithFormat` does (`internal/component/cli/client/main.go`) | `\| json`, over an answer registered through `registry.MustRegisterLocalData` |
| A filter that exists as grammar elsewhere | The operator learns one concept twice, and the two surfaces then disagree about it | the keyword |
| Silently ignored when unknown | The operator's intent is dropped and the exit code reports it was honored | name the token in the error and exit non-zero |
