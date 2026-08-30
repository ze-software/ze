---
kind: directive
level: MUST
stage:
---
**Each surface MUST be pulled from the source its row names, never pasted:**

| Surface | Pull from |
|---------|----------|
| Error messages ("valid: a,b,c") | the registry's `List()` or `Keys()` |
| `registry.Meta.Subs` and `Description` | derived at `init()` |
| A CLI `flag.NewFlagSet.Usage` | derived at call time |
| Help and `--help` output | derived |
| A `.ci` test expectation listing names | the test pulls the list |
| Generated docs | `./le inventory` |
