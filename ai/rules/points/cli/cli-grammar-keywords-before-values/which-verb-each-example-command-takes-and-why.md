---
kind: table
level:
stage:
---
| Command | Verb | Why |
|---------|------|-----|
| `show ospf database opaque-area detail` | `show` | reads the LSDB; observes only |
| `monitor ospf ...` | `monitor` | streams events; observes only |
| `debug ospf inject enable` / `debug ip ospf inject opaque ...` | `debug` | injects a crafted LSA / toggles injection; perturbs the LSDB (double-gated) |
| `set ospf area <a> log level debug` | `set` | verbose logging is config, not perturbation |
