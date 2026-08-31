Demo lab

# Looking Glass Graph Demo

A realistic UK topology with real external ASNs populates the Looking Glass topology graph. The native plugin suite runs it as an executable browser proof.

`Daemon`

Injects 36 routes into a running Ze instance that describes a small but realistic network: a core ring (Telehouse, Leeds, Manchester, Birmingham) with edge sites (Slough, Bradford), peered with real external ASNs: NTT (transit), Cogent (transit), Cloudflare (peering), and Akamai (peering), all under AS65000.

The Go test runner starts a real Looking Glass in an isolated workspace, injects the routes, checks the topology graph and route views, then tears the instance down.

- **Proves:** The Looking Glass topology graph and route views render correctly against a realistic multi-site, multi-peer network
- **Topology:** AS65000 core ring + edge sites, external peers NTT/Cogent/Cloudflare/Akamai
- **Requires:** Go toolchain
- **Source:** [internal/le/interoplab/bgp/](https://github.com/ze-software/ze/tree/main/internal/le/interoplab/bgp)

```
# runs the native plugin suite, including the graph lab
$ ./le functional plugin-test
```

`Good first lab`

No Docker, no QEMU, no root. If you want to see what Ze's operator surfaces actually look like before running anything else, start here.

- [Looking Glass docs peer and route viewer](https://github.com/ze-software/ze/blob/main/docs/features/looking-glass.md)
