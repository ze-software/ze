---
spec: dropped-error-2-bgp-plugins
topic: BGP
found-by: review
---

## Before

Several BGP plugin failures stayed at debug or info level, or exited 0. Operators did not see them at the default WARN log level:

- When a peer sent a malformed Graceful Restart (code 64) or Long-Lived Graceful Restart (code 71) OPEN capability, the plugin logged at debug level. It then ignored the capability.
- Failed BMP collector writes logged only at debug or info level with no drop counter and no `ze show errors` event. That covered Peer Up, Peer Down, Route Monitoring, Route Mirroring, Loc-RIB, Statistics Report, and socket drain. During a configuration bounce, a failed Peer Down write continued to the next peer instead of stopping the Adj-RIB replay.
- Failed BMP receiver route injection, RIB withdrawals, and mid-body session reads logged only at debug level.
- Complex-family NLRI key failures in Adj-RIB-In logged only at debug level. IRR filter invalid peer addresses and unconfirmed post-refresh peer sessions stayed below WARN too.
- When `ze bgp decode` failed writing output for EVPN, BGP-LS, VPN, labeled unicast, MUP, MVPN, RTC, VPLS, GR/LLGR, or Route Refresh, it exited 0. FlowSpec decode errors hid the parse cause behind a generic message.

## After

Each failure now reaches the operator through logs and `ze show doctor`, whether the plugin runs internal or in a separate process:

- Malformed GR and LLGR OPEN capabilities log a rate-limited WARN under `doctor-bgp-gr-capability-decode` with fix `ze show bgp peer <peer> detail`.
- BMP sender delivery failures increment `ze_bmp_message_drops_total{collector,message,reason}` and log a rate-limited WARN under `doctor-bmp-sender-delivery-failed` with fix `ze show bmp collectors`. A failed bounce Peer Down write aborts the Adj-RIB replay for that collector.
- BMP receiver route injection failures increment `ze_bmp_route_inject_errors_total{peer}` and log a rate-limited WARN under `doctor-bmp-rib-sync-failed` with fix `ze show bmp peers`. Mid-body session read errors log a WARN under `doctor-bmp-receiver-session-error` with fix `ze show bmp sessions`. Clean shutdown skips these warnings.
- Complex NLRI key failures in Adj-RIB-In increment `ze_adjribin_nlri_key_errors_total{peer,family,operation}`. They log a rate-limited WARN under `doctor-bgp-adj-rib-in-nlri-key-failed` with fix `ze show bgp adj-rib-in`.
- IRR filter invalid peer addresses log a rate-limited WARN under `doctor-bgp-irr-invalid-peer` with fix `ze show bgp peer list`. Unconfirmed post-refresh sessions log a WARN under `doctor-bgp-irr-readmit-confirm-failed` with fix `ze show bgp peer <peer> detail`.
- NLRI, GR/LLGR, and Route Refresh decode plugins exit 1 on stdout write failures. FlowSpec decode errors include the underlying parse cause.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
