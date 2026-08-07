---
kind: note
level:
stage:
---
The suffixed binaries stay in `bin/` on purpose: a binary's location decides where
`ze` resolves its config and database (`internal/core/paths/paths.go`
`ConfigDirFromBinary`), so moving them under `tmp/s/<id>/` would repoint the daemon
away from the repository's live `etc/ze`. They are swept by name at session end
(`scripts/dev/session-scratch.sh` `reap_binaries`).
