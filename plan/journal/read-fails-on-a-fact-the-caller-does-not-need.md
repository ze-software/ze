# A read fails on a fact the caller does not need

A caller asks a backend for one fact and receives it through a read that
gathers several. When the read of a fact the caller never uses fails, the whole
read fails, and the caller reports its own fact as unavailable. The device is
present and its counters are readable, yet the answer is an error about
addresses.

The tell is a caller that reads one field of a struct whose producer makes
several independent kernel calls to fill it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-19 | - (review of b06c2b499b) | `show interface <name> counters`, `showInterfaceCounters` (`internal/component/iface/cmd/show_interface.go`) | Reads only `Stats` but goes through `iface.GetInterface`, so a failed or churning address dump fails the counters command for a present device. `iface.GetLink` applies no counter baseline, so switching to it would show raw counters after a clear | open: a dispatch read of one device's link and baselined counters with no address dump, used by `showInterfaceCounters` |
| 2026-10-19 | - (review of b06c2b499b) | iface resolver `mac/match` selector, `matchByMAC` (`internal/component/iface/resolve.go`) | Resolve now reads a named device through `GetLink`, but a MAC-selected logical name still scans `ListInterfaces`, which dumps every device's addresses: one failed address dump on any device makes the MAC-selected name unresolvable | open: a backend link listing with no address dump, used by `matchByMAC` |
