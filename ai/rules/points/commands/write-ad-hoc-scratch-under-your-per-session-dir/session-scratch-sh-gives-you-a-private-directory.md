---
kind: fence
level:
stage:
---
```
dir=$(./le session scratch ensure)          # <session-dir>/scratch/, created for you
./le test-unit > "$dir/unit.log" 2>&1
```
