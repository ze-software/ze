---
kind: table
level:
stage:
---
| Banned pattern | Why | Fix |
|---------------|-----|-----|
| `*Str` suffix (`famStr`, `levelStr`, `addrStr`) | Encodes "string" type into the name | `family`, `level`, `addr` |
| `*Int`, `*Bool`, `*Bytes` suffixes | Same problem | Name the concept |
| Field/type-as-prefix on enum constants (`SurfaceSSH`, `SurfaceWeb`) | Encodes the struct field into the name; use the package for context | `audit.SSH`, `audit.Web` |
