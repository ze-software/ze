---
kind: directive
level: MUST
stage:
---
- **A subsystem or log key MUST use dots (`bgp.gr`), and a plugin name registered with `registry.Register()` MUST use hyphens (`bgp-gr`).** The two are NOT the same string. The hub canonicalizes hyphen to dot for in-process subsystem names, so a new plugin MUST be registered in the hyphen form while every config, log and env consumer uses the dot form, or the canonicalized form, depending on which side of the hub it sits on.
