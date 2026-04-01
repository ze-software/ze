# 504 -- Colored slog output

## Context

Ze had a working colored slog handler (`slogutil/color.go`) with TTY detection and `NO_COLOR` support, but the detection was incomplete: missing `TERM=dumb`, no ze-specific env var for forcing color, and no CLI flags. The test runner (`internal/test/runner/color.go`) had its own independent TTY detection that ignored `NO_COLOR`. There was no way for users to force color on (piped output) or off (TTY but don't want color).

## Decisions

- Extended existing `useColor()` (renamed to exported `UseColor()`) over creating a new package -- the function already existed and all callers used it.
- Used `ze.log.color` env var registered via `env.MustRegister()` over `CLICOLOR_FORCE`/`COLOR=1` because ze has its own env system and those conventions are non-standard.
- System conventions (`NO_COLOR`, `TERM`) use `os.LookupEnv`/`os.Getenv`; ze-specific var uses `env.Get()`/`env.IsEnabled()`.
- CLI `--color`/`--no-color` flags set the env var via `env.Set()` in the manual flag parsing loop, matching the pattern of `--debug` and `--server`.
- Fixed pre-existing bug: `-d`/`--debug` used `os.Setenv` bypassing env cache; changed to `env.Set()`.

## Consequences

- All color decisions flow through one function: `slogutil.UseColor(w io.Writer) bool`.
- Test runner now respects `NO_COLOR`, `TERM=dumb`, and `ze.log.color` for free.
- Precedence: NO_COLOR > TERM=dumb > ze.log.color > TTY detection.
- `ze.log.color` appears in `ze env registered` output automatically.
- Future work: `plan/spec-help-colors.md` builds on UseColor to add structured colored help output.

## Gotchas

- `env.Get()` + `env.IsEnabled()` is a double read of the same key. Non-empty but unrecognized values (e.g. "maybe") are treated as color-off -- this is intentional (safe default).
- Package-level eager loggers (`var logger = slogutil.Logger(...)`) create handlers during init, before `main()` parses flags. These don't respect `--color`/`--no-color`. The `LazyLogger` pattern avoids this.
- The original spec proposed building everything from scratch in a new `internal/color/` package. Review caught that the feature already existed -- the actual work was a 15-line extension to `UseColor()`.

## Files

- `internal/core/slogutil/color.go` -- extended UseColor, registered ze.log.color
- `internal/core/slogutil/slogutil.go` -- renamed callers, added env var to package doc
- `cmd/ze/main.go` -- added --color/--no-color flags, fixed -d to use env.Set
- `internal/test/runner/color.go` -- delegated to slogutil.UseColor
- `docs/guide/command-reference.md` -- documented flags
- `docs/architecture/config/environment.md` -- documented ze.log.color
