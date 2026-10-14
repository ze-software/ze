| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-12 | plugin-ownership-imports | `InternalPluginNameValidator` completion, `ze plugin <name>` | No-op registrations are offered as choices: `plugin ... use` completes every `registry.Names()` entry, so `loop` and `bgp-nlri-srpolicy` appear, naming one starts an engine that exits 0, and `ze plugin <name>` exits 0 doing nothing. Route I adds `crashes` | open: not fixed: owner decision Q-B (2026-10-12) accepts it; hiding no-op registrations is a separate completion-contract change |
