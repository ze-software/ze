---
kind: table
level:
stage:
---
| Step | Command |
|------|---------|
| Write it in the incubator | `test/draft/<suite>/<name>.ci` |
| Prove it under load | `./le stress-repro run suite "<suite> --draft" test <id> any-failure` |
| Promote when green | `mv test/draft/<suite>/<name>.ci test/<suite>/` |
