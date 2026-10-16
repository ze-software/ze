package flowspec

// The four FlowSpec families, built from the internal/core/family constants
// as the codec itself builds them. This package registers no family: the
// FlowSpec plugin registers them (its register.go), and a core test cannot
// import the plugin, so these values carry no registered name and
// Family.String falls back to the AFI and SAFI names.
var (
	ipv4FlowSpec    = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
	ipv6FlowSpec    = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}
	ipv4FlowSpecVPN = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpecVPN}
	ipv6FlowSpecVPN = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpecVPN}
)
