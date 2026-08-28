---
kind: note
level:
stage:
---
It sets `GOTRACEBACK=all` so a panic dumps every goroutine (the one racing on
the corrupt buffer shows up next to the crasher), reuses the isolated binary set
prepared by `internal/le/functional` during the loaded window, and writes the
full capture to `tmp/stress-repro/<slug>-<ts>.log`. Exit
0 = reproduced, 1 = not reproduced, 2 = setup error.
