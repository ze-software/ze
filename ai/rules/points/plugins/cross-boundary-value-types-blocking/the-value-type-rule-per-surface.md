---
kind: table
level:
stage:
---
| Surface | Rule |
|---------|------|
| Event bus (`Emit`/`Subscribe`) | Value types only: numeric IDs, `family.Family`, `netip.Prefix`, `netip.Addr`, enum uint8 |
| Cross-plugin identifiers | Registered numeric IDs, not pointers into a shared registry |
| Cross-component IPC | `*foopkg.Something` as a payload field is forbidden even when `foopkg` is "shared core" |
| Registry surfaces | Store value types only (IDs, immutable string copies, bits), not pointers to producer-allocated handles |
