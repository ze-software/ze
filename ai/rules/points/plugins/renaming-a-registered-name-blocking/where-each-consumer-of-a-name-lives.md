---
kind: table
level:
stage:
---
| Consumer | Where to grep | Looks like |
|----------|--------------|-----------|
| Plugin registration | `internal/component/bgp/plugins/*/register.go`, `internal/component/plugin/all/all.go` | `Name: "bgp-gr"` |
| Subsystem logger | `internal/core/slogutil/`, `slogutil.Logger("...")` calls | `slogutil.Logger("bgp.gr")` |
| Env var registration | `env.MustRegister("ze.log.bgp.gr", ...)` | `ze.log.<name>` |
| YANG config keys | `internal/component/*/yang/*.yang`, `grouping`/`container` names | `container gr { ... }` |
| Config consumer | `internal/component/bgp/config/`, anything that does string-keyed lookups in the parsed tree | `tree["bgp"]["gr"]` |
| Dispatch keys | `dispatchCommand("bgp gr ...")`, command prefix matching | `"bgp gr"` |
| Test fixtures | `test/**/*.ci`, `test/**/*.conf`, env vars in tests | `option=env:var=ze.log.bgp.gr` |
| Documentation | `docs/`, `<!-- source: -->` anchors | text references |
| Problem journal | `plan/journal/*.md` | text references |
