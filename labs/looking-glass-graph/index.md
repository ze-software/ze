Demo lab

# Looking Glass Graph Demo

A realistic UK topology with real external ASNs populates the Looking Glass topology graph. This is the one lab that is browsable today.

`Daemon`

Injects 36 routes into a running Ze instance that describes a small but realistic network: a core ring (Telehouse, Leeds, Manchester, Birmingham) with edge sites (Slough, Bradford), peered with real external ASNs: NTT (transit), Cogent (transit), Cloudflare (peering), and Akamai (peering), all under AS65000.

Unlike the interop labs, this one starts a real Looking Glass on localhost and prints a URL. Open it in a browser and the topology graph and route views are populated with this data.

- **Proves:** The Looking Glass topology graph and route views render correctly against a realistic multi-site, multi-peer network
- **Topology:** AS65000 core ring + edge sites, external peers NTT/Cogent/Cloudflare/Akamai
- **Requires:** Go toolchain to build ze if not already built -- nothing else
- **Source:** [test/plugin/lg-graph-lab/](https://github.com/ze-software/ze/tree/main/test/plugin/lg-graph-lab)

```
# builds ze if needed, injects routes, starts the looking glass
$ ./test/plugin/lg-graph-lab/run.sh [lg-port]

# then browse to the printed URL (default port 8443)
# Ctrl+C to stop
```

`Good first lab`

No Docker, no QEMU, no root. If you want to see what Ze's operator surfaces actually look like before running anything else, start here.

- [Looking Glass docs peer and route viewer](https://github.com/ze-software/ze/blob/main/docs/features/looking-glass.md)
