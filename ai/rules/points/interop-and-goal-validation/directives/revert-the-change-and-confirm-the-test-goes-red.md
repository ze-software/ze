---
kind: directive
level: MUST
stage:
---
**Before you claim an interop or functional test validates a change, you MUST revert the change, rebuild the artifact the test drives (the container image, the daemon binary) so the revert takes effect, confirm the test goes RED, restore the fix, confirm GREEN, and record the RED result.** A test added to ALREADY-WORKING code never had a red phase, so its discrimination is unproven until you force one. The four vacuity traps and their tells are in `docs/architecture/testing/interop.md`.
