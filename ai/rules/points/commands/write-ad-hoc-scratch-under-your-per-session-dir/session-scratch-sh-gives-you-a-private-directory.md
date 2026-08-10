---
kind: fence
level:
stage:
---
```
dir=$(scripts/dev/session-scratch.sh)          # <session-dir>/scratch/, created for you
make ze-unit-test-changed > "$dir/unit.log" 2>&1
```
