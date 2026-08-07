---
kind: note
level:
stage:
---
It sets `GOTRACEBACK=all` so a panic dumps every goroutine (the one racing on
the corrupt buffer shows up next to the crasher), reuses the prebuilt
`bin/ze`/`bin/ze-test` via `ze.bin` + `ZE_TEST_NO_BUILD` (no rebuilds under
load), and writes the full capture to `tmp/stress-repro/<slug>-<ts>.log`. Exit
0 = reproduced, 1 = not reproduced, 2 = setup error.
