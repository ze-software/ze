# How Ze compares

Choose the comparison lens before jumping into the tables. The BGP page compares Ze with BGP daemon implementations. The Network OS page compares Ze with VyOS and freeRtr as full router operating systems.

**BGP**

### [BGP daemon comparison](https://ze-software.net/compare/bgp/)

Ze against BIRD, FRR, OpenBGPd, GoBGP, bio-rd, ExaBGP, RustyBGP, rustbgpd, and freeRtr across AFI/SAFI, core protocol, policy, security, observability, APIs, operations, and best-path behavior.

- Best for protocol capability checks.
- Includes where Ze is behind today.
- Table filter can narrow by feature or implementation.

**NOS**

### [Open Source Network OS comparison](https://ze-software.net/compare/nos/)

Ze against VyOS and freeRtr across routing, interfaces, firewall, NAT, VPN, AAA, services, management APIs, automation, packaging, observability, tests, and implementation model.

- Best for router/NOS product decisions.
- Source-grounded from the local checkouts inspected for this comparison.
- Table filter can limit long evidence rows by section and keyword.

## Reading the pages

Each comparison is intentionally scoped. A `Not found` or `No` entry means the feature was not found in the inspected source roots or comparison source, not that no upstream branch or external daemon can ever provide it. The search box filters rows and sections locally in the browser, and wide matrices add product toggles so readers can hide columns they are not comparing.

## Evidence and fairness policy

Comparisons are advice, not marketing. Capability claims should cite upstream code, official feature documentation, or the integration layer that provides the behavior. For integrated systems such as VyOS, that may mean citing VyOS config/templates and FRR, nftables, Linux, or another integrated project when it owns the runtime feature. `Unclear`, `Partial`, and `Not found` are intentional outcomes when the evidence does not support a stronger claim.
