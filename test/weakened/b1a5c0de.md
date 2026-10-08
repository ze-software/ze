# Session b1a5c0de, 2026-10-08

MPLS fragmentation: a test that can livelock a stock kernel runs only where the
kernel capability probe proves the patched datapath. Published rpc names: an
assertion on a deleted dead -api module leaves the suite with the module.

| Test | Reason |
|------|--------|
| TestMinimalBuildRegistersUpdateSchema | The ze-cli-update-api module is deleted: its five rpcs duplicated the ze-update-api ones with no leaf of their own, and no command node reaches them (owner decision Q3). The test still proves its subject, that a minimal build registers the shared ze-cli-update-cmd command tree. |
