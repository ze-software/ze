# A fix command is refused in the state it reports

A report names the command that clears a problem, and that command refuses to
run in the very state that made the report fire. The operator does what the
report says, the command refuses, and the problem stays. The fix text is never
run by the code that prints it, so nothing goes red. Only a test that runs the
printed command in the reported state, and then asks the report again, finds
the gap.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-12 | single-owner-lifecycle-5-session-end-locals (walked into) | `ze doctor`, `internal/plugins/ddos/flowspec/doctor.go` `orphanDiagnostic`, `corruptRecordDiagnostic` | The report says to remove the record with `ze data rm meta/ddos-flowspec/announcement`, but `ze data rm` opens the store as its writer and `storage.lockStoreFile` refuses while the daemon that answered the report owns it, so the warning stays until the daemon stops. | open: `ze data rm` gets a daemon route, and the report names the two steps in order. |
| 2026-11-12 | spec-doctor-coverage-12-plugin-infra | `recordStoppedLocked` (`plugin/process/stopped.go`) | The `doctor-plugin-stopped` bus warning always names `RestartCommand(name)` as its fix. After a restart that failed because the stopped process still owes its cleanup, that command is refused (`restartPluginProcess`, `lateCleanups.owed`). `pluginStoppedRow` says so and the warning does not: `PluginStop` has no typed field for it | open: carry the owed cleanup on the stop record so the warning's fix is true |
