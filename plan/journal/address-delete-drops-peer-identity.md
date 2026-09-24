| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-23 | verification-baseline | `netlinkAddrRemover.List`, `netlinkAddrRemover.Delete` | Removing a point-to-point address by local CIDR discarded its installed peer. Linux returned EADDRNOTAVAIL, and PPP could not reuse the address. | Retain the matching installed address, including peer metadata. The Linux 7.2 regression fails with the old adapter and passes with the repair. |
