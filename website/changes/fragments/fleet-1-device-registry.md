---
spec: fleet-1-device-registry
topic: Fleet
feature: fleet-management
---

## Before

A hub serving managed clients tracked only the names currently connected in memory and forgot every device at disconnect or restart. Operators had no command to list declared devices, inspect their labels, versions, or configuration state, or filter devices by label. A configuration reload did not add or remove managed clients or update the managed listener's certificate, and a 128-connection lifetime cap prevented larger fleets from connecting. When a managed client rejected a rotated hub certificate, neither the device's nor the hub's `ze doctor` reported the certificate refusal.

## After

The hub maintains a persistent device registry under `meta/fleet/device/<name>/record` recording connect, disconnect, config fetch, and config acknowledgement facts across restarts. Operators can run `show fleet devices`, `show fleet devices | label <key>=<value>`, and `show fleet device name <name>` on every command surface (CLI, SSH, MCP, REST, gRPC, and the web command line), and attach up to 32 `label` entries per declared client under `bus hub server <block> client <name>`. Live configuration reloads add and remove managed clients, bind or close serving addresses, rotate secrets, and install updated managed TLS certificates from the next handshake without closing active sessions. The 128-connection lifetime cap is replaced with a 32-slot unauthenticated handshake cap, a bounded hub frame reader, and support for up to 4,096 declared clients (2,000+ simultaneous sessions proven). `ze doctor` reports unwritable or unreadable device records, handshake saturation, and certificate or authentication refusals on both the managed device (`doctor-hub-certificate-unverified`, `doctor-hub-handshake-failed`, `doctor-hub-auth-rejected`) and the hub (`doctor-managed-certificate-refused`).

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
