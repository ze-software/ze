---
spec: dropped-error-4-system-plugins
topic: Diagnostics
found-by: review
---

## Before

Across seven system plugins (DDoS detection, DHCP server, flow export, image server, NTP, TFTP server, and VRRP), 15 operator-actionable runtime failures logged only at debug or info level. Operators at the default WARN log level did not see them in logs, `ze show warnings`, `ze show errors`, or `ze show doctor`:

- DDoS detection hid baseline state-store load and save errors and victim local-address lookup failures at debug level.
- The DHCP server hid UDP reply write failures at debug level without a counter.
- Flow export logged kernel conntrack destroy-listener setup failures only at info level and hid tc sampling filter removal failures at debug level.
- The image server hid failed cleanup of temporary zefs staging directories at debug level.
- NTP hid failed `clock-synced` event notifications, hardware RTC (`/dev/rtc0`) write errors, time state-store persistence errors, and malformed DHCP option-42 lease events at debug level.
- The TFTP server hid logical interface resolution fallback errors, transfer UDP socket dial failures, and mid-transfer local file read errors at debug level.
- VRRP hid failed `vrrp:state-change` event notifications at debug level.

## After

Each of those 15 failures now logs at WARN. Every line names what failed, why it matters, the exact fix command, and a registered doctor code. Each plugin also raises the issue on the daemon report bus over the plugin SDK `report-issue` RPC. As a result, `ze show warnings`, `ze show errors`, and `ze show doctor` (`recent-events`) list it in both in-process and forked deployments.

The 15 new doctor event codes are:

- DDoS detection: `doctor-ddos-detect-baseline-save-failed`, `doctor-ddos-detect-baseline-load-failed`, and `doctor-ddos-detect-direction-fallback`.
- DHCP server: `doctor-dhcp-reply-write-failed`, paired with the new `ze_dhcp_reply_write_errors_total` counter per interface.
- Flow export: `doctor-flowexport-conntrack-destroy-unavailable` (standing warning, cleared when the listener opens or the worker stops) and `doctor-flowexport-sampling-teardown-failed`.
- Image server: `doctor-imageserver-zefs-cleanup-failed`.
- NTP: `doctor-ntp-clock-synced-emit-failed`, `doctor-ntp-rtc-write-failed` (standing warning, cleared on recovery or worker stop), `doctor-ntp-time-persist-failed` (standing warning, cleared on recovery or worker stop), and `doctor-ntp-dhcp-event-malformed`.
- TFTP server: `doctor-tftp-interface-resolve-fallback` (standing warning, cleared on clean apply, rollback, or stop), `doctor-tftp-transfer-dial-failed`, and `doctor-tftp-file-read-failed`.
- VRRP: `doctor-vrrp-state-event-emit-failed`.

Expected platform conditions stay quiet without false alerts:

- hosts without a state store
- hosts without `/dev/rtc0` or running non-Linux RTC stubs
- flow-export interfaces where tc sampling was never installed
- TFTP provisioning runs without an interface backend

High-rate paths (DHCP reply writes, TFTP dials and file reads, NTP DHCP events, and VRRP state changes) rate-limit warnings and raises to one per minute.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
