package attribute

func init() {
	// The core attributes come through the same door a plugin's attribute comes through,
	// so the name, the recognition and the flags specification cannot come apart
	// (flags_spec.go, coreAttributes).
	for _, one := range coreAttributes {
		RegisterName(one.code, one.name, one.flags)
	}

	RegisterJSONFormatter(AttrOrigin, "origin", appendOriginJSON)
	RegisterJSONFormatter(AttrNextHop, "next-hop", appendNextHopJSON)
	RegisterJSONFormatter(AttrASPath, "as-path", appendASPathJSON)
	RegisterJSONFormatter(AttrMED, "med", appendMEDJSON)
	RegisterJSONFormatter(AttrLocalPref, "local-preference", appendLocalPrefJSON)
	RegisterJSONFormatter(AttrPrefixSID, "bgp-prefix-sid", appendPrefixSIDJSON)
}
