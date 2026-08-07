---
kind: note
level:
stage:
---
`make ze-fs-persistence-check` (in `make ze-verify` / `ze-verify-changed`) runs `scripts/checks/direct_fs_persistence.go`: it flags any non-allowlisted raw filesystem write in the scanned trees. A new legitimate non-state writer needs an allowlist entry (with a reason); genuine state must move to `statestore`.
