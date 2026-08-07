---
kind: table
level:
stage:
---
| Your tool | Convention | Runs because |
|-----------|-----------|--------------|
| Has its own unit tests | Name them `<tool>_test.py` (unittest, with `unittest.main()`) and put them BESIDE the tool -- `scripts/dev/`, `test/scripts/`, or `test/perf/` | `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`) globs `*_test.py` under EVERY root in `pythonTestRoots` and runs each. A new file in an existing root is picked up automatically; a Python tool in a NEW directory needs its root added there first, or its tests never run. Each root carries its own non-empty assertion so a root that stops contributing fails loudly rather than silently covering nothing |
| Wants fixture tests inside the script | Add a `--selftest` flag, then a small Go test that shells out to it | The pattern of `dep_audit.py`, `migrate_module.py`, `qemu-run.py`. See `scripts/dev/migrate_module_test.go` |
