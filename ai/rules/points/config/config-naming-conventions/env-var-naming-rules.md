---
kind: table
level:
stage:
---
| Rule | Example |
|------|---------|
| Dot-separated, lowercase | `ze.bgp.reactor.forward-queue-size` |
| Prefix: `ze.<component>` | `ze.bgp.reactor.cache-ttl` |
| Leaf name matches YANG leaf exactly | YANG `forward-queue-size` = env `ze.bgp.reactor.forward-queue-size` |
