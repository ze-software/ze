# 794 -- CLI Session Transcript

## Context

Network engineers using `ze cli` or `ze config edit` lose all screen content when the remote device crashes or the SSH connection drops, because Bubbletea uses the terminal's alternate screen buffer which is destroyed on disconnect. Since both commands run a local Go process that receives all command output as strings before display, the local process can save a transcript without depending on the remote device. Direct SSH sessions have no local Ze process and cannot benefit from this.

## Decisions

- Chose local transcript file over server-side log on the device: the device may be crashed or unreachable, which is exactly when the transcript is needed
- Chose executor wrapping over tapping outputBuf/viewportContent at render time: executor wrapping captures the command+response pair cleanly at the dispatch boundary; render-time tapping would miss non-displayed data and couple to the rendering pipeline
- Chose boolean enable/disable (default off) over always-on: avoids surprising disk usage; engineers enable in config when needed
- Chose hardcoded XDG path (`~/.local/share/ze/transcripts/`) over configurable directory: no config complexity for v1, can be made configurable later
- Chose YANG `enumeration` (enabled/disabled) over `boolean`: matches the pattern used by other YANG leaves with semantic values
- File creation and stderr warnings live in `cmd/ze/cli/` and `cmd/ze/config/` (not `internal/`): hooks block fmt.Fprintf to stderr in internal packages, and the warning messages are program-level concerns

## Consequences

- Transcript files accumulate without rotation in v1; operators must clean up manually
- Dashboard, monitor, traceroute, and ping live output are not captured (only command/response pairs through the executor)
- The same `openTranscriptFile()` pattern is duplicated in `cmd/ze/cli/` and `cmd/ze/config/` because the packages cannot import each other; if a third entry point needs it, consider extracting to a shared `cmd/ze/internal/transcript/` package
- `wireSSHCommandExecutor` in `cmd/ze/config/cmd_edit.go` now takes username and remoteHost parameters for transcript header metadata

## Gotchas

- `textbuf.Buffer` uses fluent chainable methods (`.Str()`, `.Byte()`) not `strings.Builder`-style (`.WriteString()`, `.WriteByte()`); forgetting this causes compile errors
- Hooks block `fmt.Fprintf` to any file (not just stderr) in `internal/` packages; all file I/O for transcript must use `textbuf.Buffer` or raw `file.WriteString()`
- The `ze.cli.transcript` env var must be registered via `env.MustRegister` in the cli package; config tests that check plumbing need their own registration since the cli package is not imported transitively

## Files

- `internal/component/cli/transcript.go` -- TranscriptWriter, WrapExecutorWithTranscript, TranscriptEnabled, env var registration
- `internal/component/cli/transcript_test.go` -- 8 unit tests
- `internal/component/hub/yang/ze-hub-conf.yang` -- transcript leaf under cli container
- `internal/component/config/apply_env.go` -- cli.transcript -> ze.cli.transcript plumbing entry
- `internal/component/config/apply_env_test.go` -- TestTranscriptEnvPlumbing
- `internal/component/cli/client/transcript.go` -- openTranscriptFile helper
- `internal/component/cli/client/main.go` -- wired in runInteractiveSession, runInteractiveWithDispatch, one-shot -c mode
- `internal/component/config/cli/transcript.go` -- openTranscriptFile helper
- `internal/component/config/cli/cmd_edit.go` -- wireSSHCommandExecutor updated with transcript support
- `docs/features.md` -- CLI Session Transcript feature entry
- `docs/guide/configuration.md` -- transcript config documentation
