# A local repair is undone by the next upstream refresh

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-11 | spec-rfc-verdict-fix-routing | `resolveReceivedPath` (`internal/plugins/rsvpte/routing.go`) at a Ze PLR whose protected LSP is locally repaired | `plr-backup-path-to-egress-merge-point` FAIL: the ingress PATH refresh names a strict hop across the failed link, the PLR refuses it (PathErr 24/2), the repair expires (RFC 4090 Section 6.4.3) | fixed: fixed in 9114a04414; lab green, red without it |
