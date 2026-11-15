// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- engine-arm helpers for the v6
// views and the shared debug enablement, keeping register.go's switch thin.
// Related: register_multiaf.go -- selectInstances and afTaggedRow, the AF selection and the row
// identity every view in v3Views uses (docs/architecture/ospf/ospf-ext-15-multi-af.md).

package ospf

// debugEnableResult is the typed JSON payload of the shared enable/disable toggle.
type debugEnableResult struct {
	Action  string `json:"action"`
	Enabled bool   `json:"enabled"`
}

// v3ViewFunc answers one OSPFv3 view of command from the address-family engines cmdArgs
// selects (selectInstances). v2 is the OSPFv2 base engine, read only by the
// segment-routing fallback and by the OSPFv2 part of `show ospf database
// router-information` (routerInformationView, ri_show.go).
type v3ViewFunc func(set *v6EngineSet, v2 *engine, command string, cmdArgs []string) (any, error)

// v3Views holds every OSPFv3 view that answers for each running address family, keyed by
// its command; OnExecuteCommand serves a command found here, so a view is one entry. Each
// row or record leads with its instance's `address-family` and `instance-id`. The AF
// listings (`show ospf ipv6`, `show ospf ipv6 instance`) are not here: they list the
// instances themselves. Nor are the two RFC 7770 Router Information views
// (routerInformationView, v3RouterInformationView, ri_show.go): they answer one record of
// RI entries, each carrying the identity after its `af`, and the shared one also answers
// OSPFv2.
//
// RFC 5838 Section 1.1: "The presence of a separate link state database per address family
// is also easier to debug and operate."
var v3Views = map[string]v3ViewFunc{
	// RFC 4552: per-interface IPsec status (protocol/SPI/installed), never the key.
	cmdShowIPv6Interface:       v3RowView((*engine).ipsecInterfaceSnapshot),
	cmdShowIPv6Neighbor:        v3RowView((*engine).neighborSnapshot),
	cmdShowIPv6NeighborDetail:  v3RowView((*engine).v3NeighborDetailSnapshot),
	cmdShowIPv6InterfaceDetail: v3RowView((*engine).v3InterfaceDetailSnapshot),
	cmdShowIPv6SPF:             v3RowView((*engine).spfSnapshot),
	cmdShowIPv6Route:           v3RowView((*engine).routeSnapshot),
	// RFC 5187: the Graceful Restart state, one record per AF instance.
	cmdShowIPv6GracefulRestart: v3RowView(func(e *engine) []any { return []any{e.grSnapshot()} }),
	// RFC 8666: the Segment Routing state, one record per AF instance.
	cmdShowIPv6SegmentRouting:         v3SegmentRoutingView,
	cmdShowIPv6DatabaseSegmentRouting: v3SegmentRoutingView,
	// spfExplainSnapshot already leads its one record with the AF and Instance ID.
	cmdShowIPv6SPFDetail:            v3RecordView((*engine).spfExplainSnapshot),
	cmdShowIPv6Database:             v3DatabaseView("", ""),
	cmdShowIPv6DatabaseDetail:       v3DatabaseView("", ""),
	cmdShowIPv6DatabaseRouterDetail: v3DatabaseView("router", ""),
	cmdShowIPv6DatabaseScopeLink:    v3DatabaseView("", extRegistryLink),
	cmdShowIPv6DatabaseScopeArea:    v3DatabaseView("", scopeAreaName),
	cmdShowIPv6DatabaseScopeAS:      v3DatabaseView("", scopeASName),
	// RFC 8362: the extended LSAs (E-Router / E-Network / ...) decoded into named TLVs.
	cmdShowIPv6DatabaseExtended: v6DatabaseExtended,
}

// v3RowView answers the rows rows reads from each selected engine, each row led by its
// engine's identity. No running AF answers an empty list.
func v3RowView(rows func(*engine) []any) v3ViewFunc {
	return func(set *v6EngineSet, _ *engine, command string, cmdArgs []string) (any, error) {
		instances, err := set.selectInstances(command, cmdArgs)
		if err != nil {
			return nil, err
		}
		return taggedRows(instances, rows), nil
	}
}

// taggedRows gathers the rows of each instance, in instance order, each led by its identity.
func taggedRows(instances []afInstance, rows func(*engine) []any) []any {
	out := make([]any, 0, len(instances))
	for _, inst := range instances {
		for _, row := range rows(inst.eng) {
			out = append(out, inst.tag(row))
		}
	}
	return out
}

// v3RecordView answers the records records reads from each selected engine, for a producer
// that already leads each record with its engine's address family and Instance ID.
func v3RecordView(records func(*engine) []any) v3ViewFunc {
	return func(set *v6EngineSet, _ *engine, command string, cmdArgs []string) (any, error) {
		instances, err := set.selectInstances(command, cmdArgs)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(instances))
		for _, inst := range instances {
			out = append(out, records(inst.eng)...)
		}
		return out, nil
	}
}

// v3SegmentRoutingView answers the RFC 8666 Segment Routing state of each selected engine.
// With no OSPFv3 address family running and none named, it answers the IPv6 SR state of
// the OSPFv2 base engine's Router ID, as it did before the views read every AF: that
// record belongs to no OSPFv3 instance, so it carries no identity keys. Naming an address
// family when none runs is refused like any AF that is not running.
func v3SegmentRoutingView(set *v6EngineSet, v2 *engine, command string, cmdArgs []string) (any, error) {
	instances, err := set.selectInstances(command, cmdArgs)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return []any{v2.srSnapshot(interfaceFamilyIPv6)}, nil
	}
	return taggedRows(instances, func(e *engine) []any { return []any{e.srSnapshot(interfaceFamilyIPv6)} }), nil
}

// v3DatabaseView answers v6DatabaseDetail under one LS-type and scope filter.
func v3DatabaseView(typeFilter, scope string) v3ViewFunc {
	return func(set *v6EngineSet, _ *engine, command string, cmdArgs []string) (any, error) {
		return v6DatabaseDetail(set, command, cmdArgs, typeFilter, scope)
	}
}

// v6DatabaseDetail renders the OSPFv3 database detail of each selected address-family
// engine, one record per AF led by its address family and Instance ID, optionally filtered
// by LS-type name and/or flooding scope. With no OSPFv3 address family running and none
// named it answers the empty record of the default IPv6-unicast family, as it did before
// the views read every AF.
func v6DatabaseDetail(set *v6EngineSet, command string, cmdArgs []string, typeFilter, scope string) ([]any, error) {
	instances, err := set.selectInstances(command, cmdArgs)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return []any{v3DetailDatabase{AddressFamily: afIPv6Unicast.String(), LSAs: []v3DetailLSA{}}}, nil
	}
	out := make([]any, 0, len(instances))
	for _, inst := range instances {
		records, err := inst.eng.v3DatabaseDetailSnapshot(typeFilter, scope)
		if err != nil {
			return nil, err
		}
		out = append(out, records...)
	}
	return out, nil
}

// v6DatabaseExtended renders the RFC 8362 extended OSPFv3 LSAs (E-Router / E-Network / ...)
// of each selected address-family engine, decoded into named TLVs (AC-7).
func v6DatabaseExtended(set *v6EngineSet, _ *engine, command string, cmdArgs []string) (any, error) {
	return v6DatabaseDetail(set, command, cmdArgs, "extended", "")
}
