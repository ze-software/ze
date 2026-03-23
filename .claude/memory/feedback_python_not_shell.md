---
name: Python not shell
description: Never create shell scripts -- use Python instead. Rewrite existing shell scripts to Python.
type: feedback
---

Do not use shell/bash for scripts. Only use Python.
**Why:** Shell scripts are fragile, hard to debug, and error-prone for complex orchestration.
**How to apply:** When creating new scripts or modifying existing ones, use Python. The interop tests already use Python (`test/interop/run.py`, `test/interop/interop.py`) as precedent.
