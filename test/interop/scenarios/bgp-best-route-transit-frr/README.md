# Ordinary selected transit through Ze (FRR)

The native source, backup and sink run FRR 10.7.1.
Ze loads and explicitly attaches `bgp-rib` and `bgp-adj-rib-in`, with no
route-server or reflector. Its destination exports only P=10.245.0.0/24 and
C=10.245.1.0/24; Q=10.245.2.0/24 remains selected at Ze and is excluded.

Before changing either source, both original P candidates must be retained at
Ze and the sink must hold A's P and C with AS_PATH 65000 65001. Removing only
A's P selects the existing backup and changes the sink to AS_PATH
65000 65002 65002 65002. Removing B's P then removes P from the native sink.
C remains useful through all three phases. Every negative table verdict
requires C's actual route and exact AS_PATH; an empty query cannot pass.

The checker uses native route withdrawal controls, never an injected candidate
or private reactor call. The extra backup/sink roles use the exact-scenario
peer-preparation registry. Main still owes native runs and producing-code
RED/REPAIR/GREEN proof; this description records assertions, not a result.

Run: `INTEROP_SCENARIO=bgp-best-route-transit-frr ./le test integration interop`.
