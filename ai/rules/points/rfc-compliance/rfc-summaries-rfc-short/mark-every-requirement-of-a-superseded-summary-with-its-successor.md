---
kind: directive
level: MUST
stage:
---
**A summary whose forward Meta row names a successor MUST carry a `{superseded: ...}` marker on EVERY requirement line it declares.** `checkSuperseded` (`internal/le/rfc/check_core.go`) refuses the summary otherwise. The accepted labels, the four dispositions and the precondition each one carries are in `docs/contributing/rfc-conformance-gates.md`.
**The marker states where the obligation NOW LIVES. It MUST NOT be read, or written, as saying Ze owes less.** It is a fact about the DOCUMENT, so it composes with `{gap}`, `{not-applicable}` and `{single-polarity}` rather than replacing one. A marked requirement stays gated, stays counted, and stays judged by every ratchet.
**A `dropped` obligation is still owed for as long as Ze speaks the wire format the obsoleted document defines.** RFC 3768 is the VRRPv2 format keepalived speaks by default. RFC 9568 removing an obligation says what VRRPv3 requires, and nothing about what a VRRPv2 speaker owes on the wire.
**An `unresolved` or `unextracted` disposition is DEBT. Marking a line MUST NOT be treated as closing it.** Draining either is separable work with its own spec.
