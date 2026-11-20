# IANA -- the terms of the registry data Ze carries

**Nothing in this directory is compiled.** It records what IANA states about
the licensing of the protocol registries whose data `ze` carries. IANA
publishes a statement, not a licence file of its own, so there is no
`LICENSE` here: the statement is quoted below with its address.

## The data Ze carries

| Ze file | Registry data | Source it records |
|---------|---------------|-------------------|
| `internal/core/ipregistry/iana-ipv4-special-registry.xml` | "IPv4 Special-Purpose Address Space", `<updated>2025-10-09</updated>` | The IANA XML itself, embedded by `internal/core/ipregistry/registry.go` |
| `internal/core/ipregistry/iana-ipv6-special-registry.xml` | "IPv6 Special-Purpose Address Space", `<updated>2025-10-09</updated>` | The same |
| `internal/core/portname/services_table.go` | Service names by port and protocol | Its comment: "maps (port, proto) to the IANA service name. Derived from /etc/services." |

## What IANA states

Read on 2026-11-20 from <https://www.iana.org/help/licensing-terms>, the page
"Licensing Terms", section "Joint Statement of IANA and IETF Concerning
Copyright Rights in the Protocol Registries dated 10 November 2021". It says
"Last revised 2021-11-10." Quoted as the page prints it:

> “Protocol Registries” means the technical protocol registry data directly
> linked at either https://www.iana.org/protocols or
> https://www.ietf.org/assignments/; it does not include any other material on
> or linked from those pages (e.g., the RFC documents that are linked on those
> pages are excluded).

> IANA and IETF intend that the Protocol Registries may be freely used by any
> party for any purpose.

> Accordingly, both IANA and IETF affirm that any applicable rights that they
> may have in the Protocol Registries are subject to the Creative Commons CC0
> 1.0 dedication found at
> https://creativecommons.org/publicdomain/zero/1.0/legalcode.

> Disclaimer. While as of the date of this statement neither IANA nor IETF are
> aware of any third parties claiming intellectual property rights in the
> Protocol Registries, neither can guarantee that third parties will not do
> so. Further, the Protocol Registries are made available without any other
> sort of representation or warranty. The limitations and disclaimers found in
> the CC0 1.0 dedication are an integral part of this joint statement.

## What this does not settle

The service-name table was derived from a host's `/etc/services`, not read
from the IANA registry. The file on the build host this was checked on names
IANA as the source of the assignments ("The latest IANA port assignments can
be gotten from") and its origin ("$FreeBSD: src/etc/services,v 1.89
2002/12/17 23:59:10 eric Exp $"); it states no copyright and no licence.
