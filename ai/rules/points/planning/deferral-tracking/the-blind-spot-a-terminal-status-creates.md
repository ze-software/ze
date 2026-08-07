---
kind: directive
level:
stage:
---
**Blind spot, stated rather than papered over:** a terminal status skips the
destination check entirely, so a `done` row whose Destination is prose is not
flagged. `done` is an assertion the gate trusts. That is tolerable only because
`done` means the work LANDED, so nobody is routed toward it while work is
outstanding, and its Destination is often a commit SHA rather than a file. Marking a
row `done` before the work lands both lies and disables the check, which is why the
row above stays live.
