---
kind: directive
level: MUST
stage:
---
**You MUST pick the directory from these rules:**
- 1. Pure library, no `sdk.NewWithConn`, no plugin lifecycle, no component domain owner -> `internal/core/<x>`.
- 2. Framework or host-service infrastructure -> classify it in `internal/le/tier_non_engine_categories.txt` and keep it under `internal/component/<x>` unless this rule says setup-package placement belongs under `internal/plugins/<x>`.
- 3. Domain library -> keep it with the owning domain only when the manifest names the domain category. Today that means BNG and VPN; AAA, traffic, firewall, and CoS stay flat.
- 4. Engine that other plugins will depend on -> `internal/component/<x>`.
- 5. Engine that is a self-contained leaf feature -> `internal/plugins/<x>`.
