# Pipe Completeness

**When:** Every command that produces output MUST support all pipe operators
**Severity:** blocking

## Directives

Every command that produces output MUST support all pipe operators.

## The Pipe Operators

| Pipe | Purpose | Operates on |
|------|---------|-------------|
| `\| json` | Raw JSON output | JSON string |
| `\| ndjson` | Newline-delimited JSON | JSON string |
| `\| table` | Tabular display (default) | JSON string |
| `\| text` | Plain text | JSON string |
| `\| yaml` | YAML output | JSON string |
| `\| match <pat>` | Grep output lines | formatted string |
| `\| count` | Count results | JSON string |
| `\| resolve` | Add reverse DNS for IPs | JSON (walks values) |
| `\| origin` | Add ASN/network for IPs | JSON (walks values) |
| `\| log` | Streaming log mode | display mode flag |
| `\| no-more` | Paging | display mode flag |

## The Rule

When adding a new command or a new display mode (like `| log`):

1. The command MUST route its output through `ApplyPipes` or a `ProcessPipes*` wrapper.
2. If the command has a custom display path that bypasses `ApplyPipes` (e.g. `| log`
   rendering directly from in-memory state), that path MUST still honor data-transform
   pipes (`| resolve`, `| origin`) by applying them to the data before rendering.
3. Display-mode pipes (`| log`, `| no-more`) are flags, not data transforms. They
   change HOW output is shown, not WHAT data is shown. Data-transform pipes apply
   regardless of display mode.

## Mechanical Check

For every new command or display mode:

```
grep -n 'ApplyPipes\|ProcessPipes\|formatFn' <new-file>
```

If the command has a rendering path that does NOT call `ApplyPipes`/`formatFn`,
verify that `| resolve` and `| origin` are still applied in that path.

## Known Violations (to fix)

| Command | Mode | Missing pipes | Where |
|---------|------|---------------|-------|
| _(none currently)_ | | | |

Both `monitor traceroute | log` and `monitor ping | log` bypass `ApplyPipes`
and render directly from hop/ping stats; they now apply `resolve`/`origin`
to their legend addresses via the shared `enrichAddr` helper
(`internal/component/cli/model_enrich.go`). Functional coverage:
`test/ui/monitor-ping-pipe-resolve-log.ci` drives the headless TUI with
`option=monitor:ping=fake` (deterministic ping factory + PTR/origin fakes in
`internal/component/cli/testing/fake_monitor.go`).
