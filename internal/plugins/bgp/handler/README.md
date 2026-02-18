# BGP Cache Commands

See [docs/architecture/update-cache.md](../../../../docs/architecture/update-cache.md) for the full cache architecture, consumer model, and lifecycle.

## Commands

| Command | Description |
|---------|-------------|
| `bgp cache list` | List cached message IDs and count |
| `bgp cache <id> retain` | Increment retain count (prevents eviction) |
| `bgp cache <id> release` | Cache consumer: ack without forwarding. Otherwise: decrement retain count |
| `bgp cache <id> expire` | Admin override: force-remove immediately |
| `bgp cache <id> forward <sel>` | Forward wire bytes to matching peers, then record ack |
