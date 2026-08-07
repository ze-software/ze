---
kind: fence
level:
stage:
---
```
[ ] YANG leaf: full words, kebab-case, no abbreviations
[ ] YANG leaf: dimensioned value states its unit via a `units` statement, name unit-free (see Units)
[ ] Env var: ze.<component>.<container>.<yang-leaf-name>
[ ] Env var leaf segment matches YANG leaf name exactly
[ ] Go struct: PascalCase of YANG leaf, same word boundaries
[ ] If legacy env var exists: alias registered matching new convention
[ ] Boolean: positive form (enabled, not disabled)
```
