package rfc

import (
	"strings"
	"testing"
)

// The fixture's kernel producer, and the claim span that names its behavior.
const (
	kernelFixtureFunction = selftestKernelFunction
	kernelFixtureCitation = "answered with Time Exceeded"
	kernelFixtureClaim    = "TTL one is discarded and answered with Time Exceeded."
)

// kernelFixtureSummary is the fixture summary declaring the document mixed,
// with a reason naming the kernel function the escape records.
func kernelFixtureSummary(kind, reason string) string {
	return strings.Replace(selftestSummary,
		"| Implementation | ze |\n| Implementation reason | the fixture's own Go answers it |\n",
		"| Implementation | "+kind+" |\n| Implementation reason | "+reason+" |\n", 1)
}

// kernelFixtureFiles answers the discrimination sources with a mixed summary
// naming the kernel producer, and the tagged unit's file built for Linux only.
func kernelFixtureFiles() map[string]string {
	files := selftestDiscriminationSources()
	files[selftestTestPath] = "//go:build linux\n\n" + files[selftestTestPath]
	files[selftestSummaryRel] = kernelFixtureSummary(implementationMixed,
		"Linux "+kernelFixtureFunction+" discards an expiring datagram and answers it")
	files["go.mod"] = "module " + kernelFixtureModule + "\n\ngo 1.27\n"
	return files
}

// The fixture module, and a Ze producer a kernel unit can reach by import.
const (
	kernelFixtureModule   = "github.com/ze-software/ze"
	kernelFixtureNetlink  = "internal/plugins/iface/netlink/manage_linux.go"
	kernelFixtureProducer = kernelFixtureNetlink + "::netlinkBackend.AddRoute"
	kernelFixtureExcluded = "AddRoute installs the route that carries the datagram to the " +
		"forwarding path; the kernel discards it on TTL before any route is used"
)

// kernelImportingFiles answers kernelFixtureFiles with the tagged unit
// importing Ze's netlink backend and a test-machinery package, the way the
// RFC 792 gateway tests do.
func kernelImportingFiles() map[string]string {
	files := kernelFixtureFiles()
	files[selftestTestPath] = strings.Replace(files[selftestTestPath], "package sample\n\n",
		"package sample\n\nimport (\n\t_ \""+kernelFixtureModule+"/internal/plugins/iface/netlink\"\n"+
			"\t_ \""+kernelFixtureModule+"/internal/test/userns\"\n)\n\n", 1)
	files[kernelFixtureNetlink] = "package netlink\n\ntype netlinkBackend struct{}\n\n" +
		"func (b *netlinkBackend) AddRoute(prefix string) error {\n\tif prefix == \"\" {\n" +
		"\t\treturn nil\n\t}\n\treturn nil\n}\n"
	return files
}

// kernelEscape answers a kernel-producer escape carrying every field it owes.
func kernelEscape() DiscriminationRecord {
	return DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteNoBreak, Reason: escapeKernel,
		KernelFunctions: []string{kernelFixtureFunction}, LaterRoute: laterPatchedKernel,
		Citation: kernelFixtureCitation, Source: selftestDiscriminationRel,
		UnitSHA: strings.Repeat("a", shaHexLen), ClaimSHA: strings.Repeat("b", shaHexLen)}
}

// VALIDATES: the kernel-producer escape is accepted only for a Linux-only unit
// carrier, on a summary that declares a lower layer and names the kernel
// function, citing a span of the tag's own claim, and naming the patched-kernel
// route that could still prove it.
// METHOD: the complete record is accepted; each field removed or each fact made
// false is refused, at load time for a shape and by escapeCheck for a tree fact.
// PREVENTS: a kernel-producer escape becoming the blanket opt-out for every test
// that touches the kernel. A reason checked on nothing would discharge every
// Linux integration tag at once (R-9).
func TestKernelProducerEscape(t *testing.T) {
	complete := kernelEscape()
	if err := validateDiscrimination(&complete, selftestDiscriminationRel, 1); err != nil {
		t.Fatalf("the complete kernel-producer escape was refused at load time: %v", err)
	}
	refusedShapes := map[string]func(*DiscriminationRecord){
		"naming an in-tree producer": func(r *DiscriminationRecord) { r.Producer = selftestProducerUnit },
		"naming no kernel function":  func(r *DiscriminationRecord) { r.KernelFunctions = nil },
		"a kernel function with no symbol": func(r *DiscriminationRecord) {
			r.KernelFunctions = []string{"net/ipv4/ip_forward.c"}
		},
		"a kernel function outside a C file": func(r *DiscriminationRecord) {
			r.KernelFunctions = []string{"net/ipv4/ip_forward.go::ip_forward"}
		},
		"a second kernel function that is malformed": func(r *DiscriminationRecord) {
			r.KernelFunctions = append(r.KernelFunctions, "ip_rcv_core")
		},
		"the same kernel function twice": func(r *DiscriminationRecord) {
			r.KernelFunctions = append(r.KernelFunctions, kernelFixtureFunction)
		},
		"an exclusion with no reason": func(r *DiscriminationRecord) {
			r.ZeStateExcluded = []ZeStateExclusion{{Producer: kernelFixtureProducer}}
		},
		"an exclusion with a generic reason": func(r *DiscriminationRecord) {
			r.ZeStateExcluded = []ZeStateExclusion{{Producer: kernelFixtureProducer,
				Reason: "it does not decide the outcome of this claim at all"}}
		},
		"an exclusion with a short reason naming the producer": func(r *DiscriminationRecord) {
			r.ZeStateExcluded = []ZeStateExclusion{{Producer: kernelFixtureProducer,
				Reason: "AddRoute does not decide it"}}
		},
		"an exclusion whose reason is copied onto a second producer": func(r *DiscriminationRecord) {
			r.ZeStateExcluded = []ZeStateExclusion{
				{Producer: kernelFixtureProducer, Reason: kernelFixtureExcluded},
				{Producer: kernelFixtureNetlink + "::netlinkBackend.AddRouteMore",
					Reason: kernelFixtureExcluded}}
		},
		"citing no claim span":          func(r *DiscriminationRecord) { r.Citation = "" },
		"naming no later route":         func(r *DiscriminationRecord) { r.LaterRoute = "" },
		"naming an unknown later route": func(r *DiscriminationRecord) { r.LaterRoute = "someday" },
	}
	for name, mutate := range refusedShapes {
		record := kernelEscape()
		mutate(&record)
		if validateDiscrimination(&record, selftestDiscriminationRel, 1) == nil {
			t.Errorf("a kernel-producer escape %s was accepted at load time", name)
		}
	}
	strays := map[string]*DiscriminationRecord{
		"a declaration-only escape carrying a kernel function": {RID: selftestRIDSend,
			Polarity: PolarityPositive, Unit: selftestDiscriminationUnit, Route: RouteNoBreak,
			Reason: escapeDeclaration, Producer: selftestTablePath,
			KernelFunctions: []string{kernelFixtureFunction}, UnitSHA: strings.Repeat("a", shaHexLen),
			ClaimSHA: strings.Repeat("b", shaHexLen)},
		"a declaration-only escape carrying a later route": {RID: selftestRIDSend,
			Polarity: PolarityPositive, Unit: selftestDiscriminationUnit, Route: RouteNoBreak,
			Reason: escapeDeclaration, Producer: selftestTablePath,
			LaterRoute: laterPatchedKernel, UnitSHA: strings.Repeat("a", shaHexLen),
			ClaimSHA: strings.Repeat("b", shaHexLen)},
		"a declaration-only escape carrying a Ze state exclusion": {RID: selftestRIDSend,
			Polarity: PolarityPositive, Unit: selftestDiscriminationUnit, Route: RouteNoBreak,
			Reason: escapeDeclaration, Producer: selftestTablePath,
			ZeStateExcluded: []ZeStateExclusion{{Producer: kernelFixtureProducer,
				Reason: kernelFixtureExcluded}},
			UnitSHA: strings.Repeat("a", shaHexLen), ClaimSHA: strings.Repeat("b", shaHexLen)},
	}
	for name, record := range strays {
		if validateDiscrimination(record, selftestDiscriminationRel, 1) == nil {
			t.Errorf("%s was accepted at load time", name)
		}
	}

	claim := []Tag{{Claim: kernelFixtureClaim}}
	unit := Carrier{Kind: kindUnit}
	founded := escapeCheck{carrier: unit, carried: true, tagged: claim, record: kernelEscape()}
	founded.reader, founded.index = newSourceReader(discriminationTree(t, kernelFixtureFiles())), newScopeIndex()
	if verdict := founded.verdict(); !verdict.Verified() {
		t.Fatalf("the kernel-producer escape was refused with every fact holding: %s", verdict.Detail)
	}

	portable := kernelFixtureFiles()
	portable[selftestTestPath] = selftestDiscriminationSources()[selftestTestPath]
	ownGo := kernelFixtureFiles()
	ownGo[selftestSummaryRel] = kernelFixtureSummary(implementationZe, "Ze's Go answers it")
	unnamed := kernelFixtureFiles()
	unnamed[selftestSummaryRel] = kernelFixtureSummary(implementationMixed,
		"Linux net/ipv4/icmp.c::__icmp_send answers the datagram")
	noSummary := kernelFixtureFiles()
	delete(noSummary, selftestSummaryRel)
	strangerCitation := kernelEscape()
	strangerCitation.Citation = "answered with Parameter Problem"
	// Both spans sit in kernelFixtureClaim, so only the word minimum refuses them.
	oneWord := kernelEscape()
	oneWord.Citation = "answered"
	twoWords := kernelEscape()
	twoWords.Citation = "Time Exceeded"
	unfounded := map[string]struct {
		files   map[string]string
		carrier Carrier
		record  DiscriminationRecord
	}{
		"on a functional carrier":                    {kernelFixtureFiles(), Carrier{Kind: kindFunctional}, kernelEscape()},
		"on a unit that builds on every system":      {portable, unit, kernelEscape()},
		"on a summary Ze's own Go implements":        {ownGo, unit, kernelEscape()},
		"on a summary whose reason names another":    {unnamed, unit, kernelEscape()},
		"on a requirement whose summary is missing":  {noSummary, unit, kernelEscape()},
		"citing a span the tag's claim does not say": {kernelFixtureFiles(), unit, strangerCitation},
		"citing a one-word span of the claim":        {kernelFixtureFiles(), unit, oneWord},
		"citing a two-word span of the claim":        {kernelFixtureFiles(), unit, twoWords},
	}
	for name, one := range unfounded {
		check := escapeCheck{reader: newSourceReader(discriminationTree(t, one.files)),
			index: newScopeIndex(), carrier: one.carrier, carried: true, tagged: claim,
			record: one.record}
		verdict := check.verdict()
		if verdict.State != ProofEscapeUnfounded {
			t.Errorf("a kernel-producer escape %s answered %q, want %q", name, verdict.State,
				ProofEscapeUnfounded)
		}
	}
}

// VALIDATES: a kernel-producer escape over a unit that imports a Ze package is
// accepted only when each such package is named by an exclusion whose producer
// this tree holds, and test machinery under internal/test/ needs none.
// METHOD: the RFC 792 gateway shape: the unit imports Ze's netlink backend and
// internal/test/userns. Without an exclusion the escape is refused; with the
// AddRoute exclusion it is accepted; an exclusion naming code the unit does not
// import, or code the tree does not hold, is refused.
// PREVENTS: the escape claiming "no state Ze installs decides it" over a test
// whose outcome rests on a route Ze's own backend installed.
func TestKernelProducerZeState(t *testing.T) {
	claim := []Tag{{Claim: kernelFixtureClaim}}
	excluded := kernelEscape()
	excluded.ZeStateExcluded = []ZeStateExclusion{{Producer: kernelFixtureProducer,
		Reason: kernelFixtureExcluded}}
	if err := validateDiscrimination(&excluded, selftestDiscriminationRel, 1); err != nil {
		t.Fatalf("the escape with a complete exclusion was refused at load time: %v", err)
	}
	stranger := kernelEscape()
	stranger.ZeStateExcluded = []ZeStateExclusion{{Producer: selftestProducerUnit,
		Reason: "SendWidget sends nothing the gateway path forwards, so it decides nothing here"}}
	absent := kernelEscape()
	absent.ZeStateExcluded = []ZeStateExclusion{{
		Producer: kernelFixtureNetlink + "::netlinkBackend.DelRoute",
		Reason:   "DelRoute removes nothing the test installed, so it decides nothing at all"}}
	noModule := kernelImportingFiles()
	delete(noModule, "go.mod")
	cases := []struct {
		name   string
		files  map[string]string
		record DiscriminationRecord
		want   string
	}{
		{"a unit importing netlink with no exclusion", kernelImportingFiles(), kernelEscape(), ProofEscapeUnfounded},
		{"a unit importing netlink with its exclusion", kernelImportingFiles(), excluded, ProofVerified},
		{"an exclusion naming a package the unit does not import", kernelImportingFiles(), stranger, ProofEscapeUnfounded},
		{"an exclusion naming a producer the tree does not hold", kernelImportingFiles(), absent, ProofEscapeUnfounded},
		{"a tree whose go.mod is missing", noModule, excluded, ProofEscapeUnfounded},
		{"a unit importing no Ze package", kernelFixtureFiles(), kernelEscape(), ProofVerified},
	}
	for _, one := range cases {
		check := escapeCheck{reader: newSourceReader(discriminationTree(t, one.files)),
			index: newScopeIndex(), carrier: Carrier{Kind: kindUnit}, carried: true,
			tagged: claim, record: one.record}
		if verdict := check.verdict(); verdict.State != one.want {
			t.Errorf("%s answered %q (%s), want %q", one.name, verdict.State, verdict.Detail, one.want)
		}
	}
}

// VALIDATES: the summary names each kernel function a record carries as a
// whole token, so a record resting on two functions needs both named, and
// ip_forward is not found inside ip_forward_finish.
// METHOD: one record naming two functions over summaries naming both, one, and
// a longer function sharing the prefix.
// PREVENTS: a substring match accepting a function the summary never names.
func TestKernelProducerSummaryNamesEachFunction(t *testing.T) {
	const options = "net/ipv4/ip_options.c::__ip_options_compile"
	claim := []Tag{{Claim: kernelFixtureClaim}}
	both := kernelEscape()
	both.KernelFunctions = []string{kernelFixtureFunction, options}
	summaries := []struct {
		name   string
		reason string
		record DiscriminationRecord
		want   string
	}{
		{"both functions named", "Linux " + kernelFixtureFunction + " and " + options + ". answer it",
			both, ProofVerified},
		{"only the first function named", "Linux " + kernelFixtureFunction + " answers it",
			both, ProofEscapeUnfounded},
		{"a longer function sharing the prefix", "Linux " + kernelFixtureFunction + "_finish answers it",
			kernelEscape(), ProofEscapeUnfounded},
		{"the function at the end of the reason", "it is answered by Linux " + kernelFixtureFunction,
			kernelEscape(), ProofVerified},
	}
	for _, one := range summaries {
		files := kernelFixtureFiles()
		files[selftestSummaryRel] = kernelFixtureSummary(implementationMixed, one.reason)
		check := escapeCheck{reader: newSourceReader(discriminationTree(t, files)),
			index: newScopeIndex(), carrier: Carrier{Kind: kindUnit}, carried: true,
			tagged: claim, record: one.record}
		if verdict := check.verdict(); verdict.State != one.want {
			t.Errorf("%s answered %q (%s), want %q", one.name, verdict.State, verdict.Detail, one.want)
		}
	}
}
