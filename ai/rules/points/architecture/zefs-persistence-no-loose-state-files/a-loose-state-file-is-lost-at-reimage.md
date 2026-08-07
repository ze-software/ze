---
kind: note
level:
stage:
---
On the gokrazy appliance the writable `/perm` partition holds exactly one managed artifact: `database.zefs`. It is integrity-checked (`pkg/zefs` check/repair), seeded at install, and understood by the image build/verify tooling. A loose `state/foo.json` next to it is invisible to all of that: it is not backed up, not verified, and silently gone after a reimage. `resolve.Storage()` already resolves zefs-on-appliance / filesystem-fallback-on-dev; `statestore` is the plugin-facing equivalent: it writes through the config system's OWN blob-store handle (registered once at daemon startup via `statestore.SetStore`), so state and config share one in-memory tree.
