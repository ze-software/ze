# Verification debt -- commit session c9bdcd62

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-12 | c9bdcd62 | feat(le): le publishes the command surface it declares (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-09-11T10:56:02Z) | open |
| 2026-09-12 | c9bdcd62 | feat(le): le publishes the command surface it declares (+1 more) | native structural checks (red) | iface-resolution is red at HEAD on a stale allowlist entry naming internal/le/interoplab/bgp/isis_inject_linux.go. That file is unmodified in this tree and this commit does not include it, so the red predates the change and no path in this population produces it. | open |
| 2026-09-12 | c9bdcd62 | feat(le): le publishes the command surface it declares (+2 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-12 | c9bdcd62 | fix(le): a help word in a value slot is the keyword's value | native structural checks (red) | iface-resolution is red at HEAD on a stale allowlist entry naming internal/le/interoplab/bgp/isis_inject_linux.go. That file is unmodified in this tree and is not in this population, so the red predates the change. | open |
