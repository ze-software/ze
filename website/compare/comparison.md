# How Ze compares

Choose the comparison lens before jumping into the tables. The BGP page compares Ze with BGP daemon implementations. It also carries an OSPF table against FRR and BIRD. The Network OS page compares Ze with VyOS and freeRtr as full router operating systems.

<div class="cards compare-dispatch">
  <a class="card cat-routing compare-card-bgp" href="bgp/">
    <span class="cat">BGP</span>
    <h3>BGP daemon comparison</h3>
    <p>Ze against BIRD, FRR, OpenBGPd, GoBGP, bio-rd, ExaBGP, RustyBGP, rustbgpd, and freeRtr across AFI/SAFI, core protocol, policy, security, observability, APIs, operations, and best-path behavior.</p>
      <p>Plus OSPF standards coverage against the two daemons that also implement it.</p>
    <ul>
      <li>Best for protocol capability checks.</li>
      <li>Includes where Ze is behind today.</li>
      <li>Table filter can narrow by feature or implementation.</li>
    </ul>
  </a>
  <a class="card cat-platform compare-card-nos" href="nos/">
    <span class="cat">NOS</span>
    <h3>Open Source Network OS comparison</h3>
    <p>Ze against VyOS and freeRtr across routing, interfaces, firewall, NAT, VPN, AAA, services, management APIs, automation, packaging, observability, tests, and implementation model.</p>
    <ul>
      <li>Best for router/NOS product decisions.</li>
      <li>Source-grounded from the local checkouts inspected for this comparison.</li>
      <li>Table filter can limit long evidence rows by section and keyword.</li>
    </ul>
  </a>
</div>

## Reading the pages

Each comparison is intentionally scoped. A `Not found` or `No` entry means the feature was not found in the inspected source roots or comparison source, not that no upstream branch or external daemon can ever provide it. The search box filters rows and sections locally in the browser, and wide matrices add product toggles so readers can hide columns they are not comparing.

## Evidence and fairness policy

Comparisons are advice, not marketing. Capability claims should cite upstream code, official feature documentation, or the integration layer that provides the behavior. For integrated systems such as VyOS, that may mean citing VyOS config/templates and FRR, nftables, Linux, or another integrated project when it owns the runtime feature. `Unclear`, `Partial`, and `Not found` are intentional outcomes when the evidence does not support a stronger claim.
