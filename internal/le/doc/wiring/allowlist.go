// Design: docs/architecture/core-design.md -- the reviewed wiring exceptions
// Overview: wiring.go -- the check this list exempts symbols from
//
// allowlist.go holds the exported symbols that are deliberately API surface
// with no production caller. Keep it small. Every entry names the path and the
// symbol exactly, and says why the exemption is real rather than convenient.

package docwiring

import "sort"

// The files that contribute several exempt symbols each, named once so a
// rename edits one line rather than four.
const (
	grpcServer      = "internal/component/api/grpc/server.go"
	goldenNames     = "internal/test/golden/names.go"
	goldenResponse  = "internal/test/golden/response.go"
	goldenRoutes    = "internal/test/golden/routes.go"
	goldenPortcheck = "internal/test/golden/portcheck.go"
	templcheck      = "internal/test/templcheck/templcheck.go"
)

// allowKey identifies one exempt declaration.
type allowKey struct {
	Path string
	Name string
}

// wiringAllowlist are the reviewed exceptions.
var wiringAllowlist = map[allowKey]bool{
	// Cross-package test API: plugins (bgp/plugins/role, say) look up their
	// registered attr-mod handler in their own tests.
	{Path: "internal/component/bgp/filterapi/filterapi.go", Name: "AttrModHandlerFor"}: true,

	// The sibling-collision rule needs sibling token names at one tree level.
	// Only the static grammar gate sees the complete YANG command tree.
	// Per-command registration cannot see siblings. This gate-only function has
	// no production caller, and hasProductionReference ignores gate callers.
	{Path: "internal/component/command/grammar/checker.go", Name: "CheckSiblings"}: true,

	// Cross-package test seam: cliio keeps stdin and stdout in unexported
	// package variables. Tests in the command, mrt, analyze, and doctor packages
	// inject memory streams for the "-" path. A one-shot ze uses os.Stdin and
	// os.Stdout.
	{Path: "internal/core/cliio/cliio.go", Name: "SwapStreams"}: true,

	// Audit has the same shape as CheckSiblings. The config claim audit compares
	// the complete config schema with the complete claim union. Only a gate that
	// links the plugin composition root can assemble both. AuditConfigured is
	// the daemon entry point that the doctor check calls.
	{Path: "internal/component/config/claims/claims.go", Name: "Audit"}: true,

	// grpc-go invokes these methods through stats.Handler. The concrete handler
	// is installed in NewGRPCServer, so no production source names the methods.
	{Path: grpcServer, Name: "TagRPC"}:     true,
	{Path: grpcServer, Name: "HandleRPC"}:  true,
	{Path: grpcServer, Name: "TagConn"}:    true,
	{Path: grpcServer, Name: "HandleConn"}: true,

	// internal/test/golden is the golden-capture harness. Tests in the web and
	// lg packages call it, so its entry points must be exported. The package has
	// no production caller or production code. The web golden gate is its only
	// consumer.
	{Path: goldenNames, Name: "AssertCoversDir"}:          true,
	{Path: goldenNames, Name: "AssertCoversNames"}:        true,
	{Path: goldenNames, Name: "AssertUniqueNames"}:        true,
	{Path: goldenResponse, Name: "VersionHeader"}:         true,
	{Path: goldenRoutes, Name: "RoutePatterns"}:           true,
	{Path: goldenRoutes, Name: "RepoFile"}:                true,
	{Path: goldenResponse, Name: "AssertResponseHasBody"}: true,

	// The templ port-fidelity comparison. It reads the pre-port fixtures out of
	// git at a named revision and compares them against the ones on disk. Its
	// callers are one test in each ported package.
	{Path: goldenPortcheck, Name: "AssertPortFidelity"}: true,
	{Path: goldenPortcheck, Name: "PortRef"}:            true,
	{Path: goldenPortcheck, Name: "PortResponse"}:       true,

	// internal/test/templcheck reads a package's generated templ components. It
	// refuses parameters whose field names the compiler cannot check.
	//
	// internal/test/markupcheck reads a package's CAPTURED pages. It reports htmx
	// attributes whose assets are absent from that page's head. Tests in the
	// capturing package call each tool. Each report returns findings instead of
	// taking testing.T, so the package that owns the fixtures sets the floor.
	// Neither tool has a production caller.
	{Path: "internal/test/markupcheck/head.go", Name: "HeadCoverageFindings"}: true,
	{Path: templcheck, Name: "Report"}:                                        true,
	{Path: templcheck, Name: "AssertTyped"}:                                   true,
}

// Allowlist answers every exempt declaration, sorted by path and then by name.
// It is derived from the table above so the migration's comparison against the
// Python half's set reads the list the check itself uses.
func Allowlist() []Symbol {
	out := make([]Symbol, 0, len(wiringAllowlist))
	for key := range wiringAllowlist {
		out = append(out, Symbol{Path: key.Path, Name: key.Name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Name < out[j].Name
	})
	return out
}
