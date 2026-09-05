# API Commands

Commands sent through `ze cli`, `ze cli -c`, `ze show`, or process stdin.

### Peer Management

| Command | Description |
|---------|-------------|
| `show bgp peer list` | List peers (brief) |
| `show bgp peer <sel> detail` | Show peer details and statistics |
| `request peer <addr> teardown <code>` | Graceful session closure with NOTIFICATION |
| `delete bgp peer <name>` | Remove peer |
| `request peer <addr> pause` | Pause reading from peer (flow control) |
| `request peer <addr> resume` | Resume reading from peer |
| `show bgp peer <addr> capabilities` | Show negotiated capabilities |
| `show bgp` | BGP summary table with statistics |

Peer selector supports: `*` (all), exact IP, peer name, ASN (`as65001`), glob patterns (`192.168.*.*`), exclusion (`!addr`, `!as65001`). Tab completion for peer selectors in `ze show` and `ze cli` when daemon is running.
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer management RPC handlers -->

### Route Updates

| Command | Description |
|---------|-------------|
| `send bgp * update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `send bgp * update hex <hex>` | Hex-format UPDATE |

Text attribute syntax: `origin set igp`, `nhop set 1.1.1.1`, `local-preference set 100`, `med set 50`, `as-path set [65000 65001]`, `community set [no-export]`, `large-community set [65000:1:1]`.

NLRI operations: `add`, `del`, `eor` per address family.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_test.go -- text update parsing -->
<!-- source: internal/core/bgp/attribute/builder_parse.go -- text attribute parsing -->

### RIB Operations

| Command | Description |
|---------|-------------|
| `show bgp rib received [peer] [family]` | Show Adj-RIB-In |
| `show bgp rib sent [peer] [family]` | Show Adj-RIB-Out |
| `clear bgp rib in [peer] [family]` | Clear Adj-RIB-In |
| `clear bgp rib out [peer] [family]` | Clear Adj-RIB-Out |
| `request bgp rib inject <peer> <family> <prefix> [attrs...]` | Insert route into Adj-RIB-In (no live session needed) |
| `request bgp rib withdraw <peer> <family> <prefix>` | Remove route from Adj-RIB-In |

Inject attributes: `origin <igp|egp|incomplete>`, `nhop|nexthop <ip>`, `aspath <asn,asn,...>`, `localpref <n>`, `med <n>`. Peer address is a label (valid IP, no session required). Only simple prefix families (IPv4/IPv6 unicast/multicast).
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- injectRoute, withdrawRoute -->
<!-- source: internal/component/bgp/plugins/cmd/rib/ -- RIB command handlers -->

### Cache Management

| Command | Description |
|---------|-------------|
| `show cache` | List cached messages |
| `request cache retain` | Retain message in cache |
| `request cache release` | Release from cache |
| `request cache expire` | Set cache expiration |
| `send bgp <sel> cached <id>` | Forward cached message to peer(s) |

### Event Subscription

| Command | Description |
|---------|-------------|
| `request subscribe <filter>` | Subscribe to BGP events |
| `request unsubscribe <filter>` | Unsubscribe from events |

### Commit Workflow

Named update windows for atomic route changes:

| Command | Description |
|---------|-------------|
| `request commit start <name>` | Begin named update window |
| `request commit end <name>` | End window and send updates |
| `request commit eor <name>` | Send End-of-RIB for window |
| `request commit rollback <name>` | Discard changes |
| `request commit show <name>` | Show commit status |
| `request commit withdraw <name>` | Withdraw all routes in window |
| `request commit list` | List named commits |

<!-- source: internal/component/bgp/plugins/cmd/commit/commit.go -- commit workflow handlers -->

### Raw & Introspection

| Command | Description |
|---------|-------------|
| `send bgp * raw hex <data>` | Send raw BGP message bytes |
| `route-refresh <family>` | Send route refresh request |
| `help` | Show available commands |
| `command-list` | List all commands with descriptions |
| `command-help <name>` | Detailed help for command |

<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw BGP message handler -->
