---
kind: table
level:
stage:
---
| Step | What |
|------|------|
| A declares | `Claims: []string{"<role-token>"}` in its `registry.Registration` |
| Engine resolves | Unions the claims of the whole startup set and delivers them on every plugin's **Stage-2 configure** callback (`rpc.ConfigureInput.Claims`) |
| B stands down | Reads `sdk.Plugin.ClaimActive("<role-token>")` from its `OnConfigure` handler |
