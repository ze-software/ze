---
kind: directive
level: MUST
stage:
---
**Code in these categories MAY keep raw writes, on the allowlist:**
- **Kernel/device control:** `/proc`, `/sys`, sysfs, `/dev`, cgroup, ethtool.
- **Ephemeral scratch:** `/tmp`, `/run`, pid files, sockets, probe/ready files.
- **External artifacts:** files produced for another consumer: `resolv.conf`, systemd units, PEM exports, MRT dumps, the ze binary during self-update, the externally-written `config-pushed.conf` inbox.
- **The storage layer itself:** `internal/component/config/storage`, `pkg/zefs`, and crash-time writers (`internal/core/crashlog`) that MUST survive a broken zefs. The append-only audit log (`internal/core/audit`) also stays a tailable file (a blob KV store is the wrong shape for an append log).
