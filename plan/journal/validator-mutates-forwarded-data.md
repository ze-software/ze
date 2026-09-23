| Date | Spec | Surface | Symptom | Fix |
|---|---|---|---|---|
| 2026-09-23 | - | Kernel MPLS IPv4 MTU handling | Native RR/TS packets consumed empty option slots in the first fragment and DF-set ICMP quotation. | Preserve fragment option bytes and attach input-route context before DF-set option processing. Use the current header after checksum handling. Native before-003 fails on corrupt bytes; rebuilt-kernel after-007 passes all nine bounded cases without skips. Evidence: `plan/verification-evidence/2026-09-22-baseline/kernel-mtu-preparation/`. |
