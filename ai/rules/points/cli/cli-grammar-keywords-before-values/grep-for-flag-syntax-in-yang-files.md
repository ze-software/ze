---
kind: fence
level:
stage:
---
```
grep -rnE '\-\-[a-z]' internal --include='*.yang' | grep -vE 'urn:|http|xml'
```
