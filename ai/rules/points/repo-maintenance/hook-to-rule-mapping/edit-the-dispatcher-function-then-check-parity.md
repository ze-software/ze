---
kind: directive
level: MUST
stage:
---
**Changing a check:** the function in the relevant dispatcher (not a `.sh`) MUST be edited, then `python3 scripts/dev/hook-parity-check.py` MUST be run to confirm no behaviour changed. If you intentionally changed behaviour, the golden table MUST be re-blessed with `python3 scripts/dev/hook-parity-check.py --bless`, and the result MUST be pasted back. The "Discovery Updates" section above MUST also be satisfied so future agents can find it.
