// The family registry's own package, holding two joined family names. The
// registrar composes a name from an AFI name and a SAFI name, so no symbol here
// declares the joined set: this literal took it from the registry exactly as a
// literal next door would, and it is reported.
package family

var seeded = []string{"ipv4/unicast", "ipv6/multicast"}
