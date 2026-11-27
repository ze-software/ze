---
spec: dropped-error-8-core-hub-appliance
topic: Diagnostics
found-by: review
---

## Before

Three runtime failures logged only at debug level. Operators did not see them at the default WARN log level:

- When the self-updater failed to save update history under `meta/config/update-history`, the error stayed at debug level. Neither `ze show errors` nor `ze show doctor` reported it.
- When a DNS-over-HTTPS (DoH) reply write failed, the DNS server logged at debug level. Unlike TCP and DoT listeners, it did not increment `ze_dns_reply_write_failure_total`.
- During on-device installer DHCP fallback, a link bring-up error on a network interface stayed at debug level. The serial console later showed only a generic carrier error.

## After

Each of those failures now reports through the normal diagnostic paths:

- A self-update history save failure logs at WARN with doctor code `doctor-update-history-persist-failed`. The line names `meta/config/update-history` and the fix `ze show data check`. It also raises a rate-limited error event so `ze show errors` and `ze show doctor` list it. The update event stays in memory.
- A failed DoH reply write increments `ze_dns_reply_write_failure_total{transport="stream"}`. The handler does not emit per-query WARN logs.
- A link bring-up failure during on-device installer DHCP fallback logs at WARN on the serial console. The line names the interface, the impact on carrier detection and install leases, and the fix (`ze.mac=<mac>` or a NIC hardware and driver check). The installer then scans the other interfaces.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
