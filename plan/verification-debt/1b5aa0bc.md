# Verification debt -- commit session 1b5aa0bc

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-20 | 1b5aa0bc | adj-rib-in: key labeled and VPN routes on the route the NLRI names (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-20 | 1b5aa0bc | adj-rib-in: key labeled and VPN routes on the route the NLRI names | native structural checks (red) | arch tier check and arch iface-resolution both exit 0 over this working tree (run 2026-10-20 just before this commit); the red is a recorded stage charged for want of path attribution, not from these files | open |
| 2026-10-20 | 1b5aa0bc | adj-rib-in: key labeled and VPN routes on the route the NLRI names | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-20 | 1b5aa0bc | interop: give bgp-srv6-invalid-sid-ineligible-frr a dual-stack lab | native structural checks (red) | arch tier check and arch iface-resolution both exit 0 over this working tree (run 2026-10-20 before the previous commit a3ed24c523; this commit carries no Go); the red is a recorded stage charged for want of path attribution | open |
