# A fix command is refused in the state it reports

A report names the command that clears a problem, and that command refuses to
run in the very state that made the report fire. The operator does what the
report says, the command refuses, and the problem stays. The fix text is never
run by the code that prints it, so nothing goes red. Only a test that runs the
printed command in the reported state, and then asks the report again, finds
the gap.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-12 | single-owner-lifecycle-5-session-end-locals (walked into) | `ze doctor`, `internal/plugins/ddos/flowspec/doctor.go` `orphanDiagnostic`, `corruptRecordDiagnostic` | The report says to remove the record with `ze data rm meta/ddos-flowspec/announcement`, but `ze data rm` opens the store as its writer and `storage.lockStoreFile` refuses while the daemon that answered the report owns it, so the warning stays until the daemon stops. | fixed: 6e27af71eb, `ze data rm` beside a running daemon sends `delete data key` to it; proven by `ddos-flowspec-removed-section-orphan-cleared.ci`. |
| 2026-11-12 | spec-doctor-coverage-12-plugin-infra | `recordStoppedLocked` (`process/stopped.go`) | Bus warning omitted the cleanup-owed refusal and last resort shown by `pluginStoppedRow`. | fixed: `PluginStop.CleanupOwed` and shared `StoppedMessage`/`StoppedFix` drive both bus warning and doctor row (command-only `Fix`), refreshed when cleanup finishes; proven by `TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed` and `TestLateCleanupClearRefreshesStoppedBusWarning`. |
