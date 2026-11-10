---
spec: ddos-blind-detector-diagnostic
topic: DDoS
feature: volumetric-ddos-detection-and-mitigation
---

## Before

With `ddos detect` enabled, a detector whose interface rate feed delivered
nothing, reported no interface, or reported zero received traffic looked
healthy. Its baseline filled with zeros, read as ready, and its threshold sat
at the configured floor, so it could never trigger. `ze doctor` only checked
that a flow source for characterization was configured, and nothing in the
log said the detector was blind.

## After

`ze doctor` and `show doctor` carry a
`ddos-detect-rate-feed` check. When the armed detector's feed has delivered no
sample, reported no interface, or reported zero received packets and bytes on
every interface for a whole baseline window (`baseline-window` times
`check-interval` seconds), it warns `doctor-ddos-detect-blind`, names which of
the three it saw, and gives the command to check: `ze show interface rate` or
`ze show interface`. The log carries one WARN line when a zero-traffic run
reaches the window and one INFO line when traffic returns. An idle lab or
standby router shows this legitimately: the warning clears on the first sample
with traffic, and `ddos detect enabled false` silences it.
`ze show diagnostic doctor-ddos-detect-blind` explains the code and its fixes.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
