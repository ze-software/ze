---
kind: directive
level: MUST
stage:
---
- **Every command that produces output MUST answer every GLOBAL operator: the six formats, `no-more`, `log` and `save`.** These act on the answer whatever it holds, so no command is exempt and a refusal is never correct. Which operators are global is declared in the operator catalog (`internal/component/command/pipe_catalog.go`), never hand-copied, and that catalog is the list to read this against: an operator added there is owed by every command from the day it lands.
