---
kind: note
level:
stage:
---
This lints all packages with uncommitted Go changes, once for each BUILD, not
once. golangci-lint analyzes one GOOS, one GOARCH and one tag set for each run.
As a result, a file outside that build is not merely unchecked: the pass exits 0
and reads as clean over it.

The native action starts with the host build, then runs `GOOS=linux` with the
`integration` build tag. The second pass is the only one that reads a
`//go:build integration` file. On a non-Linux host it is also the only one that
reads a `//go:build linux` file. The rest come from
`internal/le/verifylint/matrix.go`, one for each personality tag (`ze_installer`,
`ze_distro`, `ze_appliance`, `ze_setup`), the capability tags, `tinygo`, and each
GOOS and GOARCH a tracked file names. Each flavor lints only the packages holding
a file the first two passes do not load. That package set is derived from the
tree on every run rather than written down.

One directory answers with the whole tree, on purpose. Every file in
`cmd/ze-installer` carries `//go:build linux && ze_installer`, so `go list` under
the unit tag set reports no package there. The change-set selector then has
nothing narrower to name (`internal/le/changed/scope.go`,
`uncompiledTreeReaders`). It widens to `./...`, and the wide answer is what makes
the `ze_installer` flavor run at all. A narrow answer would hand the driver a
scope the initrd's PID 1 is not in. The gate would then exit 0 over it.

Takes 3-10 seconds once the caches are warm, plus about 2 seconds for each flavor
whose packages the change reaches. The first run after a checkout pays a cold
analysis for each build, which is minutes.
