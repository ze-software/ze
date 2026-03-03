# 130 — Logging Functional Tests

## Objective

Add functional test infrastructure to verify engine subsystem logging (via env vars → stderr/syslog) and extend the `.ci` format with `option:env:`, `expect:stderr:`, `reject:stderr:`, and `expect:syslog:` line types.

## Decisions

- Chose to extend the existing `.ci` format rather than create a new test type — reuses existing parser infrastructure and existing `make functional-plugin` target.
- Syslog test uses a UDP server (`internal/test/syslog/`) started by the runner when `expect:syslog:` is present, with dynamic port assignment to avoid conflicts.
- All `.ci` parsing done in `internal/test/runner/record.go`, not in `testpeer/peer.go` — simpler than splitting parsing across two files.

## Patterns

- Syslog messages format: `<priority>timestamp hostname ze-bgp: level=X subsystem=Y msg=Z key=value...` — uses `slog.NewTextHandler` writing to `syslog.Writer`.

## Gotchas

- `logging-plugin` test was removed: `ze.bgp.log.plugin=enabled` dual-purpose issue (see spec 129) means plugin stderr relay with log level detection is broken.
- Test configs must include `send { update; }` block in process definitions when using graceful-restart — GR plugin requires events to function.

## Files

- `internal/test/syslog/syslog.go` — UDP syslog server (`Server`, `Start()`, `Messages()`, `Match()`)
- `internal/test/runner/record.go` — `EnvVars`, `ExpectStderr`, `RejectStderr`, `ExpectSyslog` fields
- `internal/test/runner/runner.go` — `validateLogging()`, test-syslog integration
- `test/data/plugin/logging-stderr.ci`, `logging-syslog.ci`, `logging-level-filter.ci`
