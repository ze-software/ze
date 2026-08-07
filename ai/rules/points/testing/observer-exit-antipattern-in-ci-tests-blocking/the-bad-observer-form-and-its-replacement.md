---
kind: table
level:
stage:
---
| Bad | Good |
|-----|------|
| `print('FAIL: ...', file=sys.stderr); sys.exit(1)` | `from ze_api import runtime_fail; runtime_fail('reason')` |
| Relying on `expect=exit:code=0` to catch observer failures | Adding explicit `expect=stderr:pattern=` on production logs the plugin emits |
| `time.sleep(N)` then "INFO: filter not called" with no failure path | `runtime_fail` if the expected event did not arrive |
