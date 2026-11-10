---
kind: directive
level: MUST
stage:
---
- **A plugin's doctor check over its live state MUST run in the `runtime` phase and MUST be answered by the plugin itself: declared in the `DoctorChecks` of the `sdk.Registration` it sends at Stage 1 and answered by its `OnDoctorCheck` handler, never as a `registry.DoctorCheckDef` function.** That function runs in whichever process answers doctor, so for a plugin running as a separate process it reads the daemon's empty copy of the state and reports a box it never looked at as healthy; the callback reaches the plugin internal and forked alike. A `registry.DoctorCheckDef` MUST carry a config phase only. The routes and the phases are in `docs/architecture/doctor-and-health-checks.md`, "The runtime phase: checks only the daemon runs".
