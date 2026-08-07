---
kind: note
level:
stage:
---
Each scenario in `test/interop/scenarios/` follows the established pattern:
- `ze.conf`: ze configuration for the scenario
- `<peer>.conf`: peer daemon configuration (frr.conf, bird.conf, etc.)
- `check.py`: Python script with a `check()` function that asserts the expected behavior
