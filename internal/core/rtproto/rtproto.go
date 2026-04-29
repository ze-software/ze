// Package rtproto defines Linux route protocol IDs used to mark Ze-owned
// kernel routes by producer.
package rtproto

const (
	// FIBKernel marks routes installed by the BGP/sysrib FIB kernel plugin.
	FIBKernel = 250
	// Static marks routes installed by the static route plugin.
	Static = 251
	// PolicyRoute marks auto-routes installed by the policy routing plugin.
	PolicyRoute = 252
)

// IsZe reports whether protocol is one of Ze's producer-specific route owners.
func IsZe(protocol int) bool {
	_, ok := zeNames[protocol]
	return ok
}

// Name returns the display name for a Ze route protocol.
func Name(protocol int) (string, bool) {
	name, ok := zeNames[protocol]
	return name, ok
}

var zeNames = map[int]string{
	FIBKernel:   "ze-fib",
	Static:      "ze-static",
	PolicyRoute: "ze-policy-route",
}
