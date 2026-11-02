# An interop scenario's checker waits on a peer the scenario never starts

The checker names a peer container the scenario directory carries no
configuration for, so the lab never starts it and the first assertion fails
before Ze's behaviour is read. The scenario proves nothing, whatever Ze does.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | route-policy-0-umbrella (found, not caused) | `ospf-multiaf-v4-frr` checker (`checkers.go`) | No FRR config in the scenario, yet the checker waits on `peerFRR`: "unknown peer frr" at c356b97d17^ and 697dc6a618. Ze also logs "no usable link-local source" on eth0 | fixed: FRR ospf6d has no IPv4-unicast AF, so 5ce5a9e38c starts a second Ze from ze-peer.conf and 1c7c181359 checks it (kernel route 10.99.0.0/24 via Ze). The warning was an IPv4-only lab network (bc9f7c9bea); b5fdf9a87f and 8c9423c5d0 fixed the Instance ID and RFC 5838 next hop. Green in the docker-lab guest |
