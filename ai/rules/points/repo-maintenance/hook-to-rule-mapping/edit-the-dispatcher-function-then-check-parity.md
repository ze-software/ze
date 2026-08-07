---
kind: directive
level:
stage:
---
**Changing a check:** edit the function in the relevant dispatcher (not a `.sh`), then run `python3 scripts/dev/hook-parity-check.py` to confirm no behaviour changed. If you intentionally changed behaviour, re-bless the golden table with `python3 scripts/dev/hook-parity-check.py --bless` and paste the result back. Also satisfy the "Discovery Updates" section above so future agents can find it.
