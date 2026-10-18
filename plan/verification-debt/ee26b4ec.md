# Verification debt -- commit session ee26b4ec

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | ee26b4ec | interoplab: settle a failed Docker removal by re-observing the resource (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-18T17:16:37Z) | open |
| 2026-10-18 | ee26b4ec | interoplab: settle a failed Docker removal by re-observing the resource | native structural checks (red) | arch iface-resolution is red on two net.InterfaceByName sites in another session's uncommitted internal/plugins/iface/dhcp work; arch tier check passes when run directly; this commit touches only internal/le | open |
| 2026-10-18 | ee26b4ec | interoplab: settle a failed Docker removal by re-observing the resource | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-18 | ee26b4ec | plan: record the removal re-check, post-fix cleanup and what remains | native structural checks (red) | arch iface-resolution is red on two net.InterfaceByName sites in another session's uncommitted internal/plugins/iface/dhcp work; this commit touches only plan/ | open |
