One payload, decoded again by every accessor that reads one field of it. Each
accessor is correct and cheap to add, so the cost is only visible when the
accessors are counted together, on the path that runs per message.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-14 | fixit-dynamic-group-peer-config | bgp plugin event rail (JSON) | `Event.GetPeerGroup` (`internal/component/bgp/event.go`) unmarshals `e.Peer` into a fresh `PeerInfoJSON` to read one string, and it now runs beside `GetPeerASN` + `getLocalASN` in `rib.updatePeerMetadata` (`plugins/rib/rib.go`) and beside `GetPeerAddress` + `GetPeerName` in `rpki.handleEvent` (`plugins/rpki/rpki.go`). That is three full decodes of the same bytes per received UPDATE on the out-of-process rail, where there were two. Every accessor on `Event` is written this way, so the pattern predates this change and this change extended it by one | not fixed. It does not block the goal: the STRUCTURED rail, which is what an in-process plugin uses, carries `PeerGroup` as a struct field and decodes nothing. The fix is one `Event.PeerInfo()` that decodes once and is passed to the readers, which changes every existing accessor's call sites and wants its own spec rather than a drive-by inside a config-delivery change |
