---
kind: directive
level: MUST
stage:
---
**The root anchor stays even when it declares zero commands, and is REQUIRED, not optional.** Once every subcommand of a verb has carved out to an owner, the central verb schema is a bare `container <verb>` with no `ze:command` leaf of its own. It MUST NOT be deleted. `internal/component/cmd/clear` is the precedent: `clear interface counters` (iface), `clear dns cache` (resolve), `clear vpn ipsec sa` (ike), `clear l2tp ...` (l2tp), and `clear bgp rib ...` (bgp) are all owner-owned, so `ze-cli-clear-cmd.yang` declares only the bare `container clear` anchor. Owners attach to that anchor two ways, and the second one has a hard dependency on it:
