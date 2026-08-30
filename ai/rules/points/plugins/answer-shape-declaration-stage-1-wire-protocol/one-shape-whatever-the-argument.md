---
kind: directive
level: MUST
stage:
---
- **A command MUST answer one shape whatever its argument.** One command path carries one declaration, so a command that answers a row set with no argument and a bare object with an argument declares neither branch truthfully. `show bgp healthcheck <name>` was corrected to answer a one-element row set; the other route declares the shape of one branch and refuses the operators of the other.
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- handleShow -->
