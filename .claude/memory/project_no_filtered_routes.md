---
name: No filtered/noexport route tracking
description: Ze does not track import-filtered or export-filtered routes separately (unlike BIRD). Birdwatcher API endpoints for these return empty results.
type: project
---

Ze does not track export-filtered routes separately, nor import-filtered routes (BIRD's "import keep filtered on").

**Why:** Ze's RIB pipeline has scope keywords (sent/received/sent-received) and filter stages (path/cidr/community/family/match), but no "filtered" scope. BIRD explicitly stores routes that were received but rejected by import policy; Ze does not.

**How to apply:** The birdwatcher-compatible API endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty route lists for compatibility. If Ze adds filtered route tracking in the future, these endpoints should be updated to query the actual filtered store.
