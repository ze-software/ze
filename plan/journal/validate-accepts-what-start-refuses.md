# Validate accepts what start refuses

A config that `ze config validate` passes is refused when the daemon starts,
because a check lives only on the start path.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | - (L2TP dial-source review) | `l2tp.ExtractParameters` (`internal/component/l2tp/config.go`), called only by `registerBNGSubsystems` (`cmd/ze/hub/register_l2tp.go`) | Read, not run: `ze config validate` never calls it, so `validateRelays` (relay to an undeclared remote) and `validateDialSources` (loopback listener with an off-host remote) refuse at start only | open: register the l2tp checks as config validators |
