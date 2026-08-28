---
kind: note
level:
stage:
---
`./le fs-persistence check` (in `./le verify worktree` / `./le verify current mode changed`) runs `internal/le/fspersistence/fspersistence.go`: it flags any non-allowlisted raw filesystem write in the scanned trees. A new legitimate non-state writer needs an allowlist entry (with a reason); genuine state must move to `statestore`.
