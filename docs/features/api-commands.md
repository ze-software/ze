# API Commands

Commands sent through `ze cli`, `ze cli -c`, `ze show`, or process stdin.

### Peer Management

| Command | Description |
|---------|-------------|
| `bgp peer * list` | List peers (brief) |
| `bgp peer * show` | Show peer details and statistics |
| `bgp peer <addr> teardown <code>` | Graceful session closure with NOTIFICATION |
| `set bgp peer <name> with <config>` | Dynamic peer creation |
| `del bgp peer <name>` | Remove peer |
| `bgp peer <addr> pause` | Pause reading from peer (flow control) |
| `bgp peer <addr> resume` | Resume reading from peer |
| `bgp peer <addr> capabilities` | Show negotiated capabilities |
| `bgp summary` | BGP summary table with statistics |

Peer selector supports: `*` (all), exact IP, peer name, ASN (`as65001`), glob patterns (`192.168.*.*`), exclusion (`!addr`, `!as65001`). Tab completion for peer selectors in `ze show` and `ze cli` when daemon is running.
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer management RPC handlers -->

### Route Updates

| Command | Description |
|---------|-------------|
| `bgp peer * update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `bgp peer * update hex <hex>` | Hex-format UPDATE |

Text attribute syntax: `origin set igp`, `nhop set 1.1.1.1`, `local-preference set 100`, `med set 50`, `as-path set [65000 65001]`, `community set [no-export]`, `large-community set [65000:1:1]`.

NLRI operations: `add`, `del`, `eor` per address family.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_test.go -- text update parsing -->
<!-- source: internal/component/bgp/attribute/builder_parse.go -- text attribute parsing -->

### RIB Operations

| Command | Description |
|---------|-------------|
| `rib routes received [peer] [family]` | Show Adj-RIB-In |
| `rib routes sent [peer] [family]` | Show Adj-RIB-Out |
| `rib clear-in [peer] [family]` | Clear Adj-RIB-In |
| `rib clear-out [peer] [family]` | Clear Adj-RIB-Out |
| `rib inject <peer> <family> <prefix> [attrs...]` | Insert route into Adj-RIB-In (no live session needed) |
| `rib withdraw <peer> <family> <prefix>` | Remove route from Adj-RIB-In |

Inject attributes: `origin <igp|egp|incomplete>`, `nhop <ip>`, `aspath <asn,asn,...>`, `localpref <n>`, `med <n>`. Peer address is a label (valid IP, no session required). Only simple prefix families (IPv4/IPv6 unicast/multicast).
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- injectRoute, withdrawRoute -->
<!-- source: internal/component/bgp/plugins/cmd/rib/ -- RIB command handlers -->

### Cache Management

| Command | Description |
|---------|-------------|
| `cache list` | List cached messages |
| `cache retain` | Retain message in cache |
| `cache release` | Release from cache |
| `cache expire` | Set cache expiration |
| `cache forward` | Forward cached message to peer(s) |

### Event Subscription

| Command | Description |
|---------|-------------|
| `subscribe <filter>` | Subscribe to BGP events |
| `unsubscribe <id>` | Unsubscribe from events |

### Commit Workflow

Named update windows for atomic route changes:

| Command | Description |
|---------|-------------|
| `commit start <name>` | Begin named update window |
| `commit end <name>` | End window and send updates |
| `commit eor <name>` | Send End-of-RIB for window |
| `commit rollback <name>` | Discard changes |
| `commit show <name>` | Show commit status |
| `commit withdraw <name>` | Withdraw all routes in window |
| `commit list` | List named commits |

<!-- source: internal/component/cmd/commit/commit.go -- commit workflow handlers -->

### Raw & Introspection

| Command | Description |
|---------|-------------|
| `bgp peer * raw <hex>` | Send raw BGP message bytes |
| `route-refresh <family>` | Send route refresh request |
| `help` | Show available commands |
| `command-list` | List all commands with descriptions |
| `command-help <name>` | Detailed help for command |

<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw BGP message handler -->
