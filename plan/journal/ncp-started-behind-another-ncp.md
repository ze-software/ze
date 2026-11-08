# An NCP started only after another NCP opened, so a peer that activates the session on the first never brings up the second

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-08 | spec-home-cpe-8-ipv6-cpe-chain | PPPoE client NCP order (`negotiatePhases`, `negotiateIPCP`) | `pppoe-dhcpv6-pd-ze-client`: IPv6CP started only after IPCP opened; accel-ppp with `ipv6=allow` treats IPv6CP as passive and activates the session IPv4-only on IPCP open, so it never runs ND or DHCPv6 and the CPE gets no IPv6 | fixed: 5aa5b17adb sends the IPv6CP Configure-Request beside IPCP's and negotiates it inside `negotiateIPCP`; the scenario went green at 0f7c8dd022, red on the commit before the fix |
