---
kind: note
level: MUST
stage:
---
**A command MUST answer one shape whatever its argument.** One command path
carries one declaration. A command answering a row set with no argument, and one
bare object with an argument, declares neither branch truthfully.
`show bgp healthcheck <name>` and `show bgp rpki aspa <asn>` were both corrected
to answer a one-element row set. The other route declares the shape of one
branch and refuses the operators of the other.
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- handleShow -->
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- aspaCommand -->
