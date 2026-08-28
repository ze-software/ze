---
kind: table
level:
stage:
---
| Surface | Pull from |
|---------|----------|
| Error messages ("valid: a,b,c") | registry `List()` / `Keys()` |
| `registry.Meta.Subs` / `Description` | derived at `init()` |
| CLI `flag.NewFlagSet.Usage` | derived at call time |
| Help / `--help` output | derived |
| `.ci` test expectations listing names | test pulls the list |
| Generated docs | `./le inventory` |
