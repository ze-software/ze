// Design: docs/architecture/testing/ci-format.md -- what a functional suite is
// Detail: budget.go -- the wall-clock cap each suite runs under
//
// suites.go defines which suites exist, what they run, why they are separate, and which suites gate.
//
// Each suite has one name. Gating supplies the run list and progress denominator.
// The .ci verify tiers are also derived from Gating.
// Suites supplies each command, while each gate and the gating run read the same record.
// The Makefile stored these facts in three lists that drifted.
// For example, `ipsec` increased the denominator and assigned a tier but ran no recipe.

package functional

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Area is the word this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const Area = "functional"

// ZeTest is the name the isolated set carries and every .ci execs by. A suite's
// command opens with it, and CommandLine swaps in the binary this run built.
const ZeTest = "ze-test"

// Suite defines what one functional suite runs and why it is separate.
//
// Args is the ze-test command line without the binary.
// Scaled selects derived concurrency. A suite with -p in Args has fixed concurrency.
// A suite with neither setting uses the runner's default.
type Suite struct {
	Name   string
	Args   []string
	Why    string
	Scaled bool
	// Chaos says this suite starts the chaos dashboard, so the isolated set it
	// runs against needs a second compile of cmd/ze beside the ze binary.
	Chaos bool
}

// Target is the Make target that runs this suite alone. It is the identity
// every shim, doc, rule and journal row spells.
func (s Suite) Target() string {
	var tb textbuf.Buffer
	return tb.Str("ze-").Str(Area).Byte('-').Str(s.Name).Str("-test").String()
}

// Rerun is the command a failure report tells the reader to type.
//
// This is the counterpart of functionalSuiteRerun (scripts/status/verify_run.go).
// A failure group is not actionable when its rerun is empty or names an unknown Make target.
func (s Suite) Rerun() string {
	var tb textbuf.Buffer
	return tb.Str("make ").Str(s.Target()).String()
}

// Command is the suite's own command: the bare binary name and its arguments.
//
// The name rather than a path, because the path depends on which isolated set
// this run built. CommandLine substitutes it.
func (s Suite) Command() []string {
	argv := make([]string, 0, len(s.Args)+3)
	argv = append(argv, ZeTest)
	argv = append(argv, s.Args...)
	if s.Scaled {
		argv = append(argv, "-p", Parallel(s.Name))
	}
	return argv
}

// The suite names, one constant each.
//
// Constants make the run list and suite table use the same names.
// A typo then becomes a compile error instead of an unresolved name.
// This catches the `ipsec` failure before GatingSuites does.
const (
	suiteEncode    = "encode"
	suitePlugin    = "plugin"
	suiteParse     = "parse"
	suiteDecode    = "decode"
	suiteReload    = "reload"
	suiteUi        = "ui"
	suiteEditor    = "editor"
	suiteManaged   = "managed"
	suiteL2tp      = "l2tp"
	suiteFirewall  = "firewall"
	suitePolicy    = "policy"
	suiteIpsec     = "ipsec"
	suiteLdp       = "ldp"
	suiteRsvpte    = "rsvpte"
	suiteIsis      = "isis"
	suiteOspf      = "ospf"
	suiteOspfv3    = "ospfv3"
	suiteWeb       = "web"
	suiteInstall   = "install"
	suiteAppliance = "appliance"
	// gosec reads "l2tp-wire" as a credential because the name ends in a word
	// its heuristic watches for. It is a suite name.
	suiteL2tpWire   = "l2tp-wire" //nolint:gosec // a suite name, not a secret
	suiteIsisWire   = "isis-wire"
	suiteOspfWire   = "ospf-wire"
	suiteRunner     = "runner"
	suiteStatic     = "static"
	suiteTraffic    = "traffic"
	suiteFlowExport = "flow-export"
	suiteVpp        = "vpp"
	suiteVrrp       = "vrrp"
)

// allTests is the ze-test flag that selects every .ci of a suite, and bgpVerb
// is the ze-test subcommand the four BGP suites run under.
const (
	allTests = "--all"
	bgpVerb  = "bgp"
)

// Gating is the run list, in the order the gating run runs them.
//
// It is also the progress denominator and the population from which every
// .ci's verify tier is derived (scripts/dev/rfc_requirements.py,
// functional_suites). A suite missing from here runs only when it is named.
var Gating = []string{
	suiteEncode,
	suitePlugin,
	suiteParse,
	suiteDecode,
	suiteReload,
	suiteUi,
	suiteEditor,
	suiteManaged,
	suiteL2tp,
	suiteFirewall,
	suitePolicy,
	suiteIpsec,
	suiteLdp,
	suiteRsvpte,
	suiteIsis,
	suiteOspf,
	suiteOspfv3,
	suiteWeb,
	suiteInstall,
	suiteAppliance,
	suiteL2tpWire,
	suiteIsisWire,
	suiteOspfWire,
	suiteRunner,
}

// Suites defines what each name runs. The first 24 suites gate.
// The next five need platform tooling or a fixture that ze-precommit-verify does not provide.
// Those five provide release evidence, not merge gates, so their .ci files have no verify tier.
var Suites = []Suite{
	{
		Name: suiteEncode, Args: []string{bgpVerb, "encode", allTests}, Scaled: true,
		Why: "BGP wire encoding; one of the two suites whose concurrency was measured",
	},
	{
		Name: suitePlugin, Args: []string{bgpVerb, "plugin", allTests}, Scaled: true,
		Why: "plugin behavior: 663 .ci files, and the one suite with a budget of its own",
	},
	{Name: suiteParse, Args: []string{bgpVerb, "parse", allTests}, Why: "config parsing"},
	{Name: suiteDecode, Args: []string{bgpVerb, "decode", allTests}, Why: "wire decoding"},
	{
		Name: suiteReload, Args: []string{bgpVerb, "reload", allTests, "-p", "1"},
		Why: "config reload; serial, because it shares the kernel routing table with managed",
	},
	{Name: suiteUi, Args: []string{"ui", allTests}, Why: "CLI and completion, against ze-stripped"},
	{Name: suiteEditor, Args: []string{"editor", allTests}, Why: "the TUI editor (.et files)"},
	{
		Name: suiteManaged, Args: []string{"managed", allTests, "-p", "1"},
		Why: "managed config; serial, because it shares the kernel routing table with reload",
	},
	{Name: suiteL2tp, Args: []string{"l2tp", allTests}, Why: "L2TP"},
	{Name: suiteFirewall, Args: []string{"firewall", allTests}, Why: "firewall"},
	{Name: suitePolicy, Args: []string{"policy", allTests}, Why: "policy routing"},
	{
		Name: suiteIpsec, Args: []string{"ipsec", allTests},
		Why: "IPsec/IKEv2 (test/ipsec/*.ci). It was declared gating and dispatched by" +
			" nothing, so it counted toward the progress denominator, never ran, and" +
			" still earned every tag in test/ipsec/ a merge-gate tier",
	},
	{Name: suiteLdp, Args: []string{"ldp", allTests}, Why: "LDP"},
	{Name: suiteRsvpte, Args: []string{"rsvpte", allTests}, Why: "RSVP-TE"},
	{Name: suiteIsis, Args: []string{"isis", allTests}, Why: "IS-IS config and doctor"},
	{Name: suiteOspf, Args: []string{"ospf", allTests}, Why: "OSPF config and doctor"},
	{Name: suiteOspfv3, Args: []string{"ospfv3", allTests}, Why: "OSPFv3 config and doctor"},
	{
		Name: suiteWeb, Args: []string{"web", allTests}, Chaos: true,
		Why: "the web UI; the only suite that starts the chaos dashboard (option=server:kind=chaos)",
	},
	{Name: suiteInstall, Args: []string{"install", allTests}, Why: "installer, PXE, kernel config"},
	{
		Name: suiteAppliance, Args: []string{"appliance", allTests},
		Why: "the appliance CLI: build, iso, list, serial-login",
	},
	{Name: suiteL2tpWire, Args: []string{"l2tp-wire", allTests}, Why: "L2TP wire level"},
	{Name: suiteIsisWire, Args: []string{"isis-wire", allTests}, Why: "IS-IS wire-level decode"},
	{Name: suiteOspfWire, Args: []string{"ospf-wire", allTests}, Why: "OSPFv2 wire-level decode"},
	{
		Name: suiteRunner, Args: []string{"runner", allTests},
		Why: "the test-runner primitives (test/runner/*.ci). Host-safe: it spawns only" +
			" sh and tail helpers, no ze daemon and no privileged tooling, which is why" +
			" it stays in the gating run",
	},
	{
		Name: suiteStatic, Args: []string{"static", allTests},
		Why: "static routes; needs the Linux daemon (release evidence only)",
	},
	{
		Name: suiteTraffic, Args: []string{"traffic", allTests},
		Why: "traffic control; needs the Linux daemon (release evidence only)",
	},
	{
		Name: suiteFlowExport, Args: []string{"flow-export", allTests},
		Why: "sFlow v5, NetFlow v9 and IPFIX export; needs the Linux daemon and, for" +
			" packet sampling, CAP_NET_ADMIN plus kernel psample (release evidence only)",
	},
	{
		Name: suiteVpp, Args: []string{"vpp", allTests},
		Why: "the VPP stub; it carries no -p because its serial default lives in the" +
			" command itself (release evidence only)",
	},
	{
		Name: suiteVrrp, Args: []string{"vrrp", allTests},
		Why: "VRRP config, show and doctor (release evidence only)",
	},
}

// SuiteNamed answers the suite called name, by its bare name or by its Make
// target.
func SuiteNamed(name string) (Suite, bool) {
	for _, suite := range Suites {
		if name == suite.Name || name == suite.Target() {
			return suite, true
		}
	}
	return Suite{}, false
}

// ErrNoSuchSuite says the gating list names something this area does not hold.
var ErrNoSuchSuite = errors.New("functional: the gating run names a suite this area does not declare")

// GatingSuites resolves the run list against the suite table.
//
// GatingSuites refuses a name that has no suite.
// This refusal is the purpose of the function.
// run_gating (scripts/le/application/functional.py) instead removes an unresolved name.
// A typo CAN therefore remove a suite from both the run and the denominator.
func GatingSuites(gating []string, suites []Suite) ([]Suite, error) {
	chosen := make([]Suite, 0, len(gating))
	for _, name := range gating {
		found := false
		for _, suite := range suites {
			if suite.Name == name {
				chosen = append(chosen, suite)
				found = true
				break
			}
		}
		if !found {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Err(ErrNoSuchSuite).Str(": ").Str(name).String())
		}
	}
	return chosen, nil
}
