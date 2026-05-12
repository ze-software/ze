# 693 -- host-4-web

## Context

Ze's host hardware inventory detection (`internal/component/host/`) could detect CPU, NIC, memory, storage, thermal, DMI, and kernel information from Linux sysfs/procfs. The workbench had a System > Host Hardware page, but it originally showed placeholder text ("Requires show host/cpu command dispatch"). This spec wired the page to call `host.Detect()` directly and display real detected values with visual indicators for operational state.

## Decisions

- **Direct `host.Detect()` call in the page builder** over RPC dispatch through the command system. The web handler runs in-process and can call the host package directly. Using the RPC path would add unnecessary serialization and deserialization when the data is already available in typed Go structs.
- **Per-core CPU details as flat key-value items** over a nested table or sub-section. Keeps the rendering simple using the existing `HardwareItem` model. Each core shows its ID, package, logical CPU number, current frequency, and role (for hybrid CPUs).
- **CSS class on `HardwareItem`** (`CSSClass` field) over separate indicator elements. The `writeHardwareKV` function applies the class to the `<tr>` element, enabling color-coded NIC carrier status (green for up, red for down) and thermal alarm highlighting (red background).
- **HTMX polling at 10-second intervals** over SSE for live updates. The resources page uses 5-second polling; hardware detection is more expensive (reads multiple sysfs files), so 10 seconds balances freshness against detection cost. SSE would require a change-detection loop in the host package, which is unnecessary for a status display page.
- **Graceful degradation on non-Linux** by catching the `host.Detect()` error and rendering a "Detection Error" section. The host package returns errors when sysfs/procfs nodes are absent, so macOS/other platforms show the error message instead of crashing.

## Consequences

- The host hardware page displays real detected data on Linux, with per-core CPU details, NIC carrier color coding, and thermal alarm indicators.
- Auto-refresh at 10-second intervals keeps the display current without manual page reloads.
- The `CSSClass` field on `HardwareItem` is available for any future hardware section that needs visual indicators.
- The page works on non-Linux platforms with a clear "Detection Error" message instead of a crash or blank page.

## Gotchas

- `host.Detect()` reads from sysfs/procfs on every page load. This is fast (sub-millisecond) but runs on every 10-second poll. If the host has many cores or NICs, the detection data grows linearly but remains bounded.
- The `SensorReading.TempMC` field is in millicelsius. Division by 1000.0 for display must use float64 to avoid integer truncation.
- On macOS (development), `host.Detect()` returns an error because /proc and /sys do not exist. Tests that call `BuildHostHardwareData()` verify only that sections are returned with titles and items, not specific section counts.
- The `CoreRole` type from the host package must be compared with `host.CoreRoleUniform` and `host.CoreRoleUnknown` to avoid displaying "uniform" on every core of a non-hybrid CPU.

## Files

- `internal/component/web/page_system.go` -- `BuildHostHardwareData()` with real detection, per-core details, NIC carrier classes, thermal alarm classes; `buildHostHardwareHTML()` with auto-refresh and `writeHardwareKV()` for CSS-classed rows
- `internal/component/web/page_system_test.go` -- `TestBuildHostHardwareData`, `TestBuildHostHardwareHTML_AlarmIndicator`, `TestBuildHostHardwareHTML_NICCarrierClass`
- `internal/component/web/assets/style.css` -- `.wb-hardware-*` classes for section layout, carrier up/down colors, alarm highlighting
