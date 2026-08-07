---
kind: table
level:
stage:
---
| New dependency | Doctor check needed |
|----------------|---------------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel module requirement | `/proc/modules` check (Linux) |
| New listen address/port | Port bind probe |
| New UDP listener | UDP `ListenPacket` bind probe |
| New service with TLS | Certificate validity + expiry check |
| Embedded certificate material | Parse certificate and check validity window |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |
| Procfs/sysctl dependency | Read/write probe for the exact `/proc` path |
| Netlink dependency | Open the specific netlink family/handle |
