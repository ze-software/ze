# 380 — Config Archive

## Objective

Add VyOS-inspired remote config archival — fan-out upload to file:// and http(s):// locations on commit or via CLI.

## Decisions

- **Fan-out model (VyOS)** over ordered failover (Junos) — all locations attempted, errors collected per-location, non-fatal to commit
- **Root `archive {}` YANG block** in `ze-hub-conf.yang` — not under `environment`, following user's explicit request
- **Phase 1 protocols:** `file://` (os.WriteFile) and `http(s)://` (net/http POST) — no new dependencies
- **ArchiveNotifier callback pattern** mirrors existing ReloadNotifier — `func(content []byte) []error`
- **Filename format:** `{basename}-{hostname}-YYYYMMDD-HHMMSS.conf` with `X-Archive-Filename` header for HTTP

## Patterns

- **Callback injection for editor features:** `SetArchiveNotifier` / `SetReloadNotifier` — wired at startup in `cmd_edit.go`, called from `cmdCommit()` in `model_commands.go`
- **Config extraction via text parsing:** `ExtractArchiveLocations` parses raw config text for `archive { location <url>; }` — avoids coupling to the config tree API
- **CLI + editor dual path:** `ze config archive` (CLI, on-demand) and editor commit (automatic) share the same `ArchiveToLocations` core
- **Parse test `.ci` semantics:** `expect=stderr:contains=` sets `ExpectError` which triggers negative test mode (expects `ze validate` failure) — use only `expect=exit:code=` for positive parse tests

## Gotchas

- **`file://./relative` parses wrong in Go:** `url.Parse("file://./path")` gives `Host="." Path="/path"` — must use `file:///absolute/path`
- **`hostname, _ := os.Hostname()` is forbidden:** Go standards ban `f, _ :=` — must handle or check the error
- **`extractArchiveLocation` must handle tabs:** Config lines can use tab separators (`location\turl`), not just spaces — need word-boundary check after `CutPrefix("location")`
- **Single-line block parsing:** `archive { location X; }` requires extracting content between `{` and `}` on the same line — a two-pass (count braces then extract) approach fails because depth returns to 0 before extraction runs
- **Archive locations are read at editor startup** from `OriginalContent()`, not `WorkingContent()` — adding `archive {}` during an editing session requires restarting the editor

## Files

- `internal/component/config/archive/archive.go` — core archive logic (NEW)
- `internal/component/config/editor/archive_test.go` — 19 unit tests (NEW)
- `cmd/ze/config/cmd_archive.go` — CLI subcommand (NEW)
- `cmd/ze/config/cmd_archive_test.go` — 8 CLI tests (NEW)
- `cmd/ze/config/cmd_edit.go` — archive notifier wiring
- `internal/component/config/editor/editor.go` — ArchiveNotifier field + methods
- `internal/component/config/editor/model_commands.go` — archive in cmdCommit
- `internal/component/hub/schema/ze-hub-conf.yang` — archive container
- `test/parse/cli-config-archive.ci`, `cli-config-archive-no-location.ci` — functional tests
