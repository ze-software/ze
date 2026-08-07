---
kind: fence
level:
stage:
---
```bash
grep -rn 'pattern' test/plugin/ --include='*.ci'                 # 1 fork
grep -n 'pattern' test/plugin/*.ci                                # 1 fork (glob)
```
