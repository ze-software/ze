# A unit that failed to start reads as absent

A configured unit (a plugin, a service) fails during startup, the daemon keeps
running without it, and the failure removes the unit from every list a
diagnostic reads. `ze doctor` then answers as if the unit had never been
configured, so the only record is a log line.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-10 | spec-doctor-coverage-1-audit-and-gate (A10 audit, found) | `plugins` health row (`internal/component/plugin/process/register.go`) over `plugin-down` (`ProcessManager.Respawn`, `applyFailurePolicy`) | Read, not run: `plugin-down` is raised only when the respawn limit disables a plugin; an `ignore` or unspecified policy exit and a failed `restartPlugin` raise nothing active, so the row reads healthy while a configured plugin is down | fixed: aa6ebb8d15, every down path raises `doctor-plugin-stopped` and a runtime check reports it; `TestPluginStoppedEveryDownPath` |
| 2026-11-10 | spec-ddos-blind-detector-diagnostic (review round 2) | plugin startup (`stopRemovedProcess`, `DoctorCheckPlugins`, plugin server) | Read, not run: a non-fatal plugin that fails Stage 2 (external ddos-detect, "event bus not configured") is removed from the process manager while ze runs, so `DoctorCheckPlugins` lists none of its checks and `show doctor` is clean | open: `spec-doctor-reports-failed-plugin-startup.md`; ze stops, or doctor reports the plugin with cause and fix |
