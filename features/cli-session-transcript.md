# CLI Session Transcript

## Meta

| Field | Value |
|-------|-------|
| Name | CLI Session Transcript |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/cli/transcript.go |
| Real-path tests | test/ui/cli-transcript-redacts-credential.ci |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: transcript.go creates $XDG_DATA_HOME/ze/transcripts and redacts the command before writing; matches the row |
| Defect review | 2026-10-07: plan/journal/refactor-removes-feature.md row 2026-10-07 is open: a `ze cli -c` command the client serves itself (cmd.ServeLocal, internal/component/cli/client/main.go) writes no transcript; the level stays experimental until it is fixed |
| Extra criteria | supported: a functional test asserting transcript file content and credential redaction = test/ui/cli-transcript-redacts-credential.ci |

## Description

Transcript recording of `ze cli` and `ze config edit` sessions to `$XDG_DATA_HOME/ze/transcripts/`, written by the process that runs the session's model: the daemon for a session on a stored configuration, the client for `ze cli -c`. Preserves command input and output for post-disconnect recovery, with credential tokens on the command line replaced by `<redacted>`. Enabled via `environment { cli { transcript enabled } }` config or `ze.cli.transcript` env var. Best-effort writes never block CLI operation. <!-- source: internal/component/cli/transcript.go -- TranscriptWriter -->
