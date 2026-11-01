# Process-global read across the plugin boundary

A plugin reads a package-level table that only another component's
same-process code fills. Run internal, the read answers. Run forked
(`plugin { external ... }`), the plugin holds its own empty copy of the table,
and the read answers as if nothing were configured: no error, no log line, and
a wrong value that looks like a real one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-29 | spec-iface-resolution-across-the-plugin-boundary | `iface.Resolve` and `iface.ResolveDevice` callers in bfd, ldp, tftpserver, diag, isis, ospf, ospfv3 and traffic | The selector table (`globalResolver.setMapping`, `internal/component/iface/resolve.go`) is filled only by the iface plugin's in-process apply. Forked, it is empty and `ResolveDevice` returns the logical name, so the plugin binds a device that merely wears that name, or none | open: refuse external start as vrrp and iface/dhcp do, or resolve across the boundary; extend `dangerousCalls` |
| 2026-11-01 | spec-filter-wire-0-umbrella | `filterapi.Register` filters of bgp-role and bgp-filter-community | The daemon-side filter functions read package globals (role's peer config and OPEN-learned roles, community's definitions and agreements) that only the plugin's own callbacks fill. Forked, the daemon copy is empty and every route passes unchanged: RFC 9234 Section 5 OTC, RFC 7999 BLACKHOLE withholding and RFC 8195 tags stop, no error | open: the evaluators move into their plugins as stage-attached filters on both transports, each proven by a forked `.ci` red at HEAD |
