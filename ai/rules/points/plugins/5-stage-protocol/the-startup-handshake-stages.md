---
kind: table
level:
stage:
---
| Stage | Direction | RPC |
|-------|-----------|-----|
| 1. Declaration | Plugin->Engine (A) | `ze-plugin-engine:declare-registration` |
| 2. Config | Engine->Plugin (B) | `ze-plugin-callback:configure` |
| 3. Capability | Plugin->Engine (A) | `ze-plugin-engine:declare-capabilities` |
| 4. Registry | Engine->Plugin (B) | `ze-plugin-callback:share-registry` |
| 5. Ready | Plugin->Engine (A) | `ze-plugin-engine:ready`, enter event loop |
| Post | Engine->Plugin (B) | `ze-plugin-callback:post-startup`, sent once after every startup phase completes and both the plugin registry and dispatcher command registry are frozen |
