---
kind: note
level:
stage:
---
Each scenario under `test/interop*/scenarios/` carries the declarative inputs
that its native runner reads: `ze.conf` plus the peer configuration and argument
files required by that topology. Assertions do not live in the scenario
directory. They are typed Go checkers under `internal/le/interoplab/`.
