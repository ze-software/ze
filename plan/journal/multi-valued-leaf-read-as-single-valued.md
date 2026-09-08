# Multi-valued leaf read as single-valued

A `leaf-list` promises the operator that every entry is used. The parser keeps
that promise: it collects all of them into a slice field. The consumer then
reads index zero and returns. Nothing logs the entries it dropped, so the
operator sees a service that started, on one of the things they named, with no
line telling them which.

The tell is a single index literal on a field a `leaf-list` fills. It is easy
to miss because the parser and its test are correct, and because the sibling
service that reads the SAME leaf name usually loops. Two consumers disagreeing
about one leaf's arity is the defect, not a style difference: an operator who
configures both for one job gets one of them on every entry and the other on
one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-08 | image-server-listen-interface-drops-entries | `startServer` (`internal/plugins/imageserver/register.go`) reading `listen-interface` | `parseConfig` collected every entry into `imageConfig.ListenInterfaces`; `startServer` resolved `ListenInterfaces[0]` alone and built one `http.Server`. Entry one and every later entry were dropped with no log line, and the `ze:help` on the leaf described that drop as the behavior while the leaf beside it, `listen-port`, described binding "on the IPv4 address of the first listen-interface". `startServer` in `internal/plugins/tftpserver/register.go` loops over the same leaf name, so a PXE install configured with two networks got TFTP on both and HTTP on one | fixed at f4aa60f61. `listenTargets` resolves every entry and reports the ones that do not resolve, `startTargets` binds one server per target and skips the ones that do not bind, and an empty result logs `no interfaces bound; server not serving` instead of `started`. Each listener builds its own mux, because `newMux` stores the bind address and `serveBootIPXE` writes it into `/install/boot/boot.ipxe`, so a shared mux would send a client on the second network to the first network's address. `test/install/image-multi-interface.ci` drives a real daemon and asserts both entry names reach the log |
