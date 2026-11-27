---
spec: ai-diagnostics-1-triage-and-fault-visibility
topic: Diagnostics
---

## Before

Investigating a misbehaving Ze daemon over CLI or MCP required running separate commands for build version, component health, doctor checks, errors, warnings, crashes, reload status, and recent logs. When a guarded protocol task recovered from a panic, Ze wrote a crash record and an error log line without a diagnostic code, and did not raise an event on the report bus, so `show errors` and `show doctor` did not surface recovered panics. The bundled `ze-diagnostics` guide covered configuration validation only.

## After

`show system triage` (exposed automatically over CLI, SSH, web, REST, gRPC, and MCP as `ze_show_system` action `triage`) returns a single bounded JSON snapshot covering build provenance, component health, daemon doctor findings, recent errors, active warnings, crash records, unhealthy plugins, reload status, recent warning and error logs, and process runtime vitals. Sections whose runtime source is unavailable report `available: false` with a reason while the remaining sections still answer, and capped lists include `total-count` and `truncated`. Recovered task panics now log `code=doctor-internal-defect` and raise a bounded error event on the report bus with the exact crash-inspection or log-inspection command, surfacing immediately in `show errors`, `show doctor`, and `show system triage`. The bundled `ze` and `ze-diagnostics` skills now include a live runtime triage playbook starting from `show system triage`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
