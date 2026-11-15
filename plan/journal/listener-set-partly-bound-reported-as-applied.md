# A listener set partly bound is reported as applied

A service configured with several listen addresses binds each one on its own.
One bind fails, the failure is only logged, the others serve, and the start or
the commit reports success, so the service runs half-bound with nothing an
operator reads saying so.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-10 | spec-doctor-coverage-4-api-surfaces (A3 audit, found) | gNMI listener set, `gnmiBuildImpl` (`cmd/ze/hub/service_gnmi.go`) over `resolveGNMIListeners` | Read, not run: `environment gnmi server` is a list, and only the first entry is bound; the rest commit and are never served, with one Warn line ("extra listeners ignored") as the only trace, while web, REST, gRPC and MCP bind every entry | open: gNMI binds every configured listener all or none, as REST and gRPC do, and reload migrates the set |
| 2026-11-09 | spec-bmp-receiver-bind-all-or-none | BMP receiver `applyReceiverConfig` in `receiver.go` | A failed bind was logged while start or reload reported success with a partial listener set | fixed: `bindReceiverListeners` closes the staged set on failure. `registerCallbacks` returns configure/apply/rollback errors and publishes other roots only after receiver success. Socket/SDK units and four internal/external daemon cases passed |
