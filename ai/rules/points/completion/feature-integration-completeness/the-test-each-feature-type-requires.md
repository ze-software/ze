---
kind: directive
level: MUST
stage:
---
**Each feature type MUST carry the test its row names:**

| Feature Type | Required Test |
|-------------|---------------|
| Injectable interface | Inject fake, verify component uses it |
| CLI flag | Flag changes program behavior |
| Config option | Option affects runtime behavior |
| YANG config leaf | Env var registered (`env.MustRegister`), appears in `ze env registered` |
| API/RPC | Caller reaches handler through real transport |
| Event/hook | Event fires, subscriber receives |
| Plugin capability | Engine dispatches to plugin correctly |
| Struct field | Field is read and affects a decision |
