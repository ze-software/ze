---
kind: directive
level: MUST
stage:
---
- **Every command that produces output MUST answer every GLOBAL operator: the six formats, `no-more` and `log`.** These act on the answer whatever it holds, so no command is exempt and none may refuse one. Which operators are global is declared in the operator catalog (`internal/component/command/pipe_catalog.go`), never hand-copied, and that catalog is what this list must be read against: an operator added there is owed by every command from the day it lands.
