# OSPFv3 stub-area default origination and export hygiene

Two results the interop-coverage work produced: the v6 stub default origination
that was missing, and the interface-seam rule that `scripts/dev/validate.py`
now applies.

## Decisions

- **OSPFv3 stub default origination is implemented, not worked around.** Only
  the v4 path applied the area-type policy. A v6 stub ABR now originates a
  single `::/0` Inter-Area-Prefix default at `default-cost`, suppresses
  Inter-Area-Router-LSAs into stub and NSSA areas, and under totally-stubby
  suppresses the other inter-area prefixes. This is symmetric with the v4 policy
  in `ospf-11-stub-nssa.md`.
  <!-- source: internal/plugins/ospf/origination_v6_stub.go -- v6ApplyAreaTypePolicy -->
  <!-- source: internal/plugins/ospf/origination_v6_summary.go -- v6OriginateSummaries -->
- **`validate.py` learned an interface-seam exemption instead of the code losing
  an export.** A method on an UNEXPORTED receiver type that satisfies a
  same-package exported interface is exempt. An interface method cannot be
  unexported without breaking an external implementor, and a cross-package test
  backend implements this one.
- **A passive dummy interface with a configured global IPv6 address gives the
  single-bridge interop harness a unique advertisable prefix.** The peer then
  installs a real route, which is a stronger assertion than an LSDB-only check.
  A three-container topology is not needed. OSPFv3 advertises the global
  prefixes of a passive interface and filters link-local and loopback.

## Traps

- **`validate.py` scans untracked files**, so a wholly new tree IS scanned, and a
  non-zero exit can be mostly other work's symbols. Scope the count before
  believing a claim about one package.
- **Unexporting a type that only tests name breaks the tests that name it.**
  Compare a value against a known instance instead of naming the type.
- A stub adjacency that forms at all proves the v6 Hello option bits are cleared
  for a stub area. Area options are address-family neutral, so this is not
  receive-side suppression alone.
