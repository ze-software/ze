---
spec: doctor-codes-move-to-their-owners
topic: Diagnostics
feature: system-readiness
---

## Before

All feature- and plugin-specific `doctor-*` diagnostic codes were compiled into one shared catalog in every `ze` binary. A minimal build without an optional feature still explained that feature's diagnostic codes instead of indicating that the feature was absent from the build.

## After

Each subsystem and plugin now registers its own `doctor-*` diagnostic codes when compiled into the binary, while built-in `config-*`, core TLS helper, and cross-cutting multi-subsystem codes remain built in. `ze show diagnostic <code>` explains every code the running binary can emit; when asked for a code not present in the build, it reports the code as unknown and notes that if the code belongs to an optional feature, that feature is not in this build.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
