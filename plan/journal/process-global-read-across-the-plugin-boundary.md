# Process-global read across the plugin boundary

A plugin reads a package-level table that only another component's
same-process code fills. Run internal, the read answers. Run forked
(`plugin { external ... }`), the plugin holds its own empty copy of the table,
and the read answers as if nothing were configured: no error, no log line, and
a wrong value that looks like a real one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-29 | spec-iface-resolution-across-the-plugin-boundary | `iface.Resolve` and `iface.ResolveDevice` callers in bfd, ldp, tftpserver, diag, isis, ospf, ospfv3 and traffic | The selector table (`globalResolver.setMapping`, `internal/component/iface/resolve.go`) is filled only by the iface plugin's in-process apply. Forked, it is empty and `ResolveDevice` returns the logical name, so the plugin binds a device that merely wears that name, or none | open: refuse external start as vrrp and iface/dhcp do, or resolve across the boundary; extend `dangerousCalls` |
