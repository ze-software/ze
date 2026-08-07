---
kind: note
level:
stage:
---
Detection hook: `c_observer_sys_exit` in `.claude/hooks/pretool-writeedit.py`
(warns on Write/Edit of `.ci` files containing `tmpfs=*.run` Python with
`sys.exit(1)` and no `runtime_fail`).
