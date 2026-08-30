---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/compatibility.md
---
**Ze has never been released and has no users, so compat code, deprecation shims, fallbacks and "keep the old name working" MUST NOT be written anywhere; when something needs to change, change it.** The one frozen surface is the plugin API contract external authors compile against (`pkg/plugin/` types, the JSON event and text command protocol, anything re-exported for plugin consumption): its signatures and documented semantics MUST NOT break once released, while its implementation and every other `internal/` package stay free to change forever. ExaBGP format awareness MUST live only in `ze exabgp plugin` and `ze config migrate`, never in engine code.
