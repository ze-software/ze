package attribute

func init() {
	RegisterJSONFormatter(AttrOrigin, "origin", appendOriginJSON)
	RegisterJSONFormatter(AttrNextHop, "next-hop", appendNextHopJSON)
	RegisterJSONFormatter(AttrASPath, "as-path", appendASPathJSON)
	RegisterJSONFormatter(AttrMED, "med", appendMEDJSON)
	RegisterJSONFormatter(AttrLocalPref, "local-preference", appendLocalPrefJSON)
}
