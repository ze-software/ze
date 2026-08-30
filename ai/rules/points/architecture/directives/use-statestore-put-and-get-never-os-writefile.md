---
kind: directive
level: MUST
stage:
---
**Runtime state MUST persist through `statestore.Put`/`Get` (`internal/core/statestore`) under a key registered in `pkg/zefs/keys.go`, sharing the one `*zefs.BlobStore` the config system opens at startup; a state blob MUST NOT be written with `os.WriteFile`, `os.Create`, `os.OpenFile(..., O_CREATE...)` or `os.Rename` under the config or state directory.** A separate transient store lets the config flush drop every state key. Raw writes stay correct for kernel and device control (`/proc`, `/sys`, `/dev`, cgroup, ethtool), ephemeral scratch (`/tmp`, `/run`, pid files, sockets), artifacts produced for another consumer (`resolv.conf`, systemd units, PEM exports, MRT dumps, self-update), and the storage layer itself; a new one MUST carry an allowlist entry with its reason, and `./le fs-persistence check` refuses the rest.
