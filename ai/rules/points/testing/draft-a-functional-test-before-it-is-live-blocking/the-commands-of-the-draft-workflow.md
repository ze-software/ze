---
kind: table
level:
stage:
---
| Step | Command |
|------|---------|
| Write it in the incubator | `test/draft/<suite>/<name>.ci` |
| Run only drafts | `ze-test <suite> --draft -a` |
| Prove it under load | `scripts/dev/stress-repro.py "<suite> --draft" --test <id> --any-failure` |
| Promote when green | `mv test/draft/<suite>/<name>.ci test/<suite>/` |
