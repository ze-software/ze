# 021 — API Receive Update

## Objective

Forward received BGP UPDATEs to API processes that have opted in via `api { receive { update; } }`, enabling processes to observe incoming routes in ExaBGP text format.

## Decisions

- Hook point is `session.go handleUpdate()` — after validation, parse UPDATE into routes and forward to all processes with `ReceiveUpdate=true`.
- Output format matches ExaBGP text encoder: `neighbor <ip> receive update start`, one line per route, `neighbor <ip> receive update end`.
- `ReceiveUpdate bool` field added to both `APIProcessConfig` (reactor) and `ProcessConfig` (plugin) — the config flows down through both layers.

## Patterns

- ProcessManager implements an `UpdateReceiver` interface — reactor calls `ForwardUpdate()`, ProcessManager routes to opted-in processes.

## Gotchas

- `check` test (`check.run`) was timing out because received UPDATEs were never forwarded to the process — the process waited for route data that never arrived.
- Text format uses ExaBGP text encoder format (not JSON) for this feature, even though Ze generally uses JSON — matched ExaBGP's `response/text.py` output.

## Files

- `internal/reactor/session.go` — handleUpdate() hook for forwarding
- `internal/plugin/process.go` — WriteEvent() for sending to process stdin
- `internal/plugin/types.go` — ProcessConfig.ReceiveUpdate field
- `internal/reactor/reactor.go` — APIProcessConfig.ReceiveUpdate, ForwardUpdate()
