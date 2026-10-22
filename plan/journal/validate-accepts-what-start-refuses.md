# Validate accepts what start refuses

A config that `ze config validate` passes is refused when the daemon starts,
because a check lives only on the start path.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-18 | - (VyOS T6304 comparison) | `system archive` location and timeout (`config/archive/archive.go`) | The scheme check ran only in the manual archive command, so `location "ftp://h/"` committed and later archives failed at runtime; a bad `timeout` became a silent 30s | fixed: both leaves carry `ze:validate` (`config/validators_archive.go`), so validate and commit refuse them; test `cli-config-archive-invalid-refused.ci`, red at HEAD |
| 2026-10-17 | - (L2TP dial-source review) | `l2tp.ExtractParameters` (`internal/component/l2tp/config.go`), called only by `registerBNGSubsystems` (`cmd/ze/hub/register_l2tp.go`) | Read, not run: `ze config validate` never calls it, so `validateRelays` (relay to an undeclared remote) and `validateDialSources` (loopback listener with an off-host remote) refuse at start only | open: register the l2tp checks as config validators |
| 2026-10-22 | spec-exabgp-6-api-compatibility | old-command judge `zeRoutes` (`internal/exabgp/bridge/old_commands_test.go`) against `handleUpdateText` (`internal/component/bgp/plugins/cmd/update/update_text.go`) | The judge runs `update.ParseUpdateText` alone, which accepts `watchdog <name>`; the handler then answers "watchdog not yet implemented for update text". About 95 5.0 lines carrying `watchdog` count as behaving as on 5.0 while the daemon refuses them | fixed: afddd8ffe0; since M25 the bridge gives it to bgp-watchdog (`watchdog give`) |
