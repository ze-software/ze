---
kind: table
level:
stage:
---
| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show interface <name>` | `show interface name <name> detail` | `<name>` is untyped and could collide with keywords (`brief`, `errors`) |
| `show interface <name> counters` | `show interface name <name> counters` | Selector value appears before selector kind |
| `show l2tp session <id>` | `show l2tp session id <id> detail` | ID must be typed before use |
| `show vpn ipsec peer <name>` | `show vpn ipsec peer name <name> detail` | Named lookup needs an explicit selector kind |
| `cache <id> retain` | `cache retain <id>` | ID before action |
| `commit <name> start` | `commit start <name>` | Name before action |
