---
kind: fence
level:
stage:
---
```
# 1. Identify the feature's test directory from the table above
# 2. Check for a functional test covering the behavior
find test/<directory>/ -name "*.ci" -o -name "*.et" | xargs grep -l '<feature-keyword>'
```
