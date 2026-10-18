# Verification debt -- commit session ba0dfc83

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-18 | ba0dfc83 | test(dhcp): DHCPv4 lease-maintenance interop against dnsmasq (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-18T17:16:37Z) | open |
| 2026-10-18 | ba0dfc83 | test(dhcp): DHCPv4 lease-maintenance interop against dnsmasq (+1 more) | native structural checks (red) | arch gates charged from a stale tmp/ze-verify-failures.json: ./le --name dhcp4c arch tier check and ./le --name dhcp4c arch iface-resolution both exit 0 on this tree (2026-10-19); scoped go lint run over internal/le/interoplab/bgp reports 0 issues | open |
| 2026-10-18 | ba0dfc83 | test(dhcp): DHCPv4 lease-maintenance interop against dnsmasq (+2 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-18 | ba0dfc83 | fix(dhcp): clear the DHCPv4 lint findings | native structural checks (red) | arch gates charged from a stale tmp/ze-verify-failures.json: ./le --name dhcp4c arch tier check and ./le --name dhcp4c arch iface-resolution both exit 0 on this tree (2026-10-19); the go lint/run red left names only the DHCPv6 files, not in this commit | open |
