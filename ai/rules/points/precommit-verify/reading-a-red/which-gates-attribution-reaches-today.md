---
kind: table
level:
stage:
---
| Gate | What its groups name | Expect |
|------|----------------------|--------|
| `./le verify-lint run`, `./le changed scope` | the `.go` file each finding sits in | a drop when none of them is in your `--file` list |
| `ze-evidence-vet` | the package pattern of each red | a drop when your list holds no file under it |
| `./le doc-wiring` | the files each sub-check is about, one declared group per failure | a drop, except for the ci-sleep ratchet and a delegated target, which name no file and charge |
| Every other stage, `./le repository generated-check`, `./le doc-check links` and `./le test-weakened check` among them | the stage's own name, through `genericGroup` (`internal/le/verifyengine/run.go`) | a charge, always. Read that stage's log and attribute the red by hand |
