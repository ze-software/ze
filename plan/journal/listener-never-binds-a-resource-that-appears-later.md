# A listener bound once at configure never binds a resource that appears later

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-15 | - (walked into ordering the provisioning servers after interface) | `bindListeners` (dhcpserver/register.go), `bindTFTP` (tftpserver/config_tx.go), `bindImage` (imageserver/register.go) | Boot binds each listen-interface once and logs `listen failed` for one that is absent. Nothing binds it when it appears later: a hotplugged device, or the IPv4 address the image server needs arriving through DHCP. The interface is served only after the next reload. | open: no link or address subscription re-runs the bind. |
