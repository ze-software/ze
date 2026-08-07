---
kind: fence
level:
stage:
---
```
internal/plugins/<name>/
    register.go                # CLI registration (RegisterRoot, MustRegisterLocalMeta)
    yang/
        ze-<name>-cmd.yang     # hand-written command YANG
        embed.go               # GENERATED from .yang files
        register.go            # GENERATED yang.RegisterModule()
        self_containment_test.go
    cmd/
        register.go            # pluginserver.RegisterRPCs() in init()
        handler.go             # RPC handler (imports component/<name> for real work)
```
