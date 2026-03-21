# 259 — BGP Chaos Reporting

## Objective

Add production-quality reporting to the ze-bgp-chaos tool: live terminal dashboard, NDJSON event log, Prometheus metrics, and enhanced exit summary with iBGP/eBGP peer counts.

## Decisions

- Chose raw ANSI escape codes over bubbletea for the live dashboard — bubbletea takes over stdin and is designed for interactive TUIs; the chaos dashboard is passive (read-only during a run).
- Reporter is a synchronous fan-out multiplexer (not a goroutine) — ProcessEvent() calls all consumers in order on the main event loop goroutine. No channels, no goroutines needed.
- Per-instance `prometheus.Registry` (not the global default) — essential for test isolation; avoids "metric already registered" panics when tests create multiple instances.
- `orchestratorConfig` struct introduced to consolidate 12 parameters to `runOrchestrator` — discovered during wiring, not anticipated in the spec.

## Patterns

- `reportWriter` pattern (track first error, skip subsequent writes) is reusable across all `report/*` output code — already existed in `summary.go`, extended to `dashboard.go` and `jsonlog.go`.
- TTY detection: `term.IsTerminal(int(os.Stdout.Fd()))` — same pattern as `internal/test/runner/color.go`.
- `strings.SplitSeq` (Go 1.24+) preferred over `strings.Split` by the `modernize` linter.

## Gotchas

- `block-ignored-errors.sh` hook blocks both `_, _ = fmt.Fprintf(...)` and `_ = c.Close()` — must use the `reportWriter` pattern or `errors.Join(errs...)` return respectively.
- The `exhaustive` linter requires all 10 EventType cases listed explicitly in dashboard switches — no `default:` shortcut.

## Files

- `cmd/ze-bgp-chaos/report/reporter.go` — Consumer interface + synchronous fan-out
- `cmd/ze-bgp-chaos/report/dashboard.go` — ANSI TTY / line-based fallback
- `cmd/ze-bgp-chaos/report/jsonlog.go` — NDJSON event log
- `cmd/ze-bgp-chaos/report/metrics.go` — Prometheus endpoint (per-instance registry)
- `cmd/ze-bgp-chaos/peer/event_string.go` — EventType.String() kebab-case names
- `cmd/ze-bgp-chaos/report/summary.go` — Extended with IBGPCount/EBGPCount
