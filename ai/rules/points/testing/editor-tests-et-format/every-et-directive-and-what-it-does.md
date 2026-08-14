---
kind: table
level:
stage:
---
| Directive | Purpose | Example |
|-----------|---------|---------|
| `tmpfs=<path>:terminator=<TERM>` | Embedded config file | `tmpfs=test.conf:terminator=EOF` |
| `option=file:path=<name>` | Config file to load (required) | `option=file:path=test.conf` |
| `option=timeout:value=<dur>` | Test timeout (default 30s) | `option=timeout:value=10s` |
| `option=width:value=N` | Editor width (default 80) | `option=width:value=120` |
| `option=height:value=N` | Editor height (default 24) | `option=height:value=30` |
| `option=reload:mode=success\|fail` | Mock reload notifier | `option=reload:mode=success` |
| `option=monitor:ping=fake` | Deterministic ping monitor + fake PTR/origin resolvers (offline pipe-enrichment tests; see `internal/component/cli/testing/fake_monitor.go`) | `option=monitor:ping=fake` |
| `option=session:user=X:origin=Y` | Session identity | `option=session:user=alice:origin=ssh` |
| `option=storage:value=blob` | Config storage is a zefs blob, as in the daemon, instead of the filesystem. The tmpfs `*.conf` files migrate into the blob, so `option=file:path=` still names the config. `expect=file:` is refused with it: the editor writes the blob, so the temp directory keeps its migrated state | `option=storage:value=blob` |
| `session=<name>` | Switch to named session | `session=bob` |
| `input=type:text=<string>` | Type text | `input=type:text=show` |
| `input=<keyname>` | Press key | `input=enter`, `input=tab`, `input=up` |
| `input=ctrl:key=<char>` | Ctrl+key | `input=ctrl:key=c` |
