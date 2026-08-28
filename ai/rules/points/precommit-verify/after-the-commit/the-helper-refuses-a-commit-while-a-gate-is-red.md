---
kind: note
level:
stage:
---
This is enforced, not honor-system: `./le commit create` reads
the native failure index produced by `internal/le/verify` and refuses to prepare
a script while `structuralGateReds` in `internal/le/commit/verification.go`
charges a deterministic structural red to this commit.
A red is charged unless every file its failure groups name lies outside the
commit's `--file` list, and a group that names no file is charged as it always
was ("Reading A Red", above). A green verify rewrites the artifact, so a
fixed-and-reverified gate clears automatically.
