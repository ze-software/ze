---
spec: bmp-route-action-redistribute-acts-as-monitor
topic: Config
---

## Before

A BMP receiver accepted `route-action redistribute` but only stored the routes for monitoring. A warning was the only indication.

## After

Ze refuses `route-action redistribute` when configuration loads or reloads and gives `monitor` as the accepted value. Remove the setting or use `route-action monitor;`. BMP routes remain visible in the BMP RIB and looking glass, separate from BGP best-path selection and the FIB.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
