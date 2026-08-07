---
kind: table
level:
stage:
---
| Anti-pattern | Fix |
|-------------|-----|
| `map[string]*Peer` keyed by peer address string | `map[netip.Addr]*Peer` or `map[uint32]*Peer` with `Addr.As4()` |
| Re-parsing the peer string at every map access | Parse ONCE where the JSON IPC event or text command enters the plugin, pass `netip.Addr` to every internal helper (see the RIB plugins for the reference conversion) |
| `map[string]Handler` keyed by command name, looked up per-message | Register commands to `map[uint16]Handler` by numeric code |
| `map[string]bool` as a set of known values | `map[uint8]bool` or bitfield |
| `switch s { case "add": ... case "remove": ... }` on every UPDATE | Parse to enum once, `switch e { case ActionAdd: ... }` |
