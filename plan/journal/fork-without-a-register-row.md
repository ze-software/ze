# Shipped code forks a binary the register does not name

Shipped Ze code runs an external binary through `exec.Command` or
`exec.CommandContext`, but `ai/allowed-system-commands.md` has no row for it.
`ai/rules/go-standards.md` says an exception needs a row before the code lands,
and only Thomas can authorise one. A fork with no row is a dependency on a
binary that nobody signed off.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-15 | - | `internal/plugins/systemd` (`cmd_install.go` lines 163-215, `cmd_uninstall.go` lines 72-81) | `ze install systemd` and `ze uninstall systemd` run `getent`, `groupadd`, `addgroup`, `useradd`, `adduser`, `userdel`, `deluser`, `groupdel` and `delgroup` through `realServiceOps`. The register now holds a row for `systemctl` only (Thomas, 2026-10-15), so these nine have no row | fixed: 210454a85a, register rows for all nine (Thomas, 2026-10-15: "yes list them") |
