# Verification debt -- commit session 7f5fa7cc

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-21 | 7f5fa7cc | dockerhost: mount Lima's user-data shares at boot (+4 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-21T12:27:12Z) | open |
| 2026-10-21 | 7f5fa7cc | dockerhost: mount Lima's user-data shares at boot | native structural checks (red) | staticcheck parts 2 and 3 were red in the 12:27Z verify before this change; scoped golangci-lint (staticcheck included) over both touched packages reports 0 issues | open |
| 2026-10-21 | 7f5fa7cc | dockerhost: mount Lima's user-data shares at boot (+3 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-21 | 7f5fa7cc | appliance: return to the caller's directory after gok | native structural checks (red) | staticcheck parts 2 and 3 were red in the 12:27Z verify before this change; scoped golangci-lint over internal/appliance reports 0 issues | open |
| 2026-10-21 | 7f5fa7cc | dockerhost: mount binfmt_misc so Colima can enable Rosetta | native structural checks (red) | staticcheck parts 2 and 3 were red in the 12:27Z verify before this change; scoped golangci-lint over internal/appliance/dockerhost reports 0 issues | open |
| 2026-10-21 | 7f5fa7cc | dockerhost: seal /mnt and /Users after Lima's shares | native structural checks (red) | staticcheck parts 2 and 3 were red in the 12:27Z verify before this change; this change touches only internal/appliance/dockerhost, whose vet and race tests are clean | open |
| 2026-10-21 | 7f5fa7cc | spec(docker-host): Phase 3 evidence, A-11 confirmed | native structural checks (red) | staticcheck parts 2 and 3 were red in the 12:27Z verify before this change; this commit carries no Go | open |
