---
spec: dropped-error-9-api-cli-config
topic: Diagnostics
found-by: review
---

## Before

Three failures in the `| resolve` pipe, `config dump`, and the web configuration editor logged only at debug level. Operators at the default WARN log level saw no warning and no entry in `ze show errors` or `ze show doctor`:

- When a reverse DNS lookup failed during `| resolve` because no DNS server was configured or every server failed, the error stayed at debug level. The command printed empty hostnames with no signal of a DNS outage.
- When `$9$` secret encoding failed during `ze config dump`, `show config dump`, or `show config supplied dump`, the dump logged at debug level and substituted `/* SECRET-DATA */` instead of refusing the command.
- When the web configuration editor evicted an idle session and candidate discard failed, the error stayed at debug level.

## After

Each failure now reports through the normal error or diagnostic paths:

- A reverse DNS resolver failure in `| resolve` logs at WARN with doctor code `doctor-resolve-ptr-failed` and fix `ze show system dns`. It also raises a rate-limited event for `ze show errors` and `ze show doctor` (at most once per minute). Upstream command rows still print with empty `-name` fields, and NXDOMAIN responses stay silent.
- A `$9$` secret encoding failure in `ze config dump`, `show config dump`, or `show config supplied dump` refuses the dump with an operator error. The error names `ze config dump --strip-private` and never prints plaintext secrets.
- A failed candidate discard on idle web editor eviction logs at WARN with doctor code `doctor-web-editor-evict-discard-failed` and fix `ze show doctor`. It also raises a rate-limited, secret-free event (at most once per minute). The idle session slot is still freed and saved drafts stay unchanged.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
