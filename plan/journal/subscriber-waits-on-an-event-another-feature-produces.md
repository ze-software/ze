# A subscriber waits for an event that only another, optional feature produces

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | bgp-unowned-rfc-rows | reactor_iface.go neighbor-changed; iface OnConfigure monitor start | With no `interface` block no netlink monitor runs, so BGP never re-reads the neighbor table: a third-party router resolved after session up stays unpaired and its routes leave in 16 octets. | open: BGP needs a neighbor-change source that runs without interface config. |
