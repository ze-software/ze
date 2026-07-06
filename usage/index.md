# Usage

Use these pages when you want Ze to play a concrete role in a network.

They are deployment examples, with the Ze config, the adjacent network config, and the lab evidence that backs the shape. The feature guides explain every knob. Usage pages show how the knobs fit together.

## Examples

| Example | Ze role | Network side | Evidence |
| --- | --- | --- | --- |
| [AS112 anycast DNS inside a network](as112/) | Authoritative AS112 DNS sink plus BGP origin for the AS112 covering prefixes | VyOS, Junos, Cisco IOS XR, BIRD, and FRR receive the routes from Ze | Existing AS112 interop scenarios cover the DNS server, BGP redistribution, origin AS, communities, and the covering-prefix guard |

## What belongs here

Usage pages are for complete operator shapes:

- A topology with clear addresses and ASNs.
- A full Ze config for the role.
- The neighboring router or daemon config.
- The verification command or lab scenario that proves the important behavior.

Reference-only material stays in [Documentation](../docs/). Raw interop evidence stays in [Labs](../labs/).