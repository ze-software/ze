// Design: docs/labs/pppoe-interop.md -- native VPP mechanism proof entry points.
// Related: plan.go -- the bounded Docker proof; control_linux.go -- raw PPP control.
package testvpppppoe

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const area = "test deployment vpp-pppoe"

var actions = leaction.New(area,
	leaction.Action{Verb: "product-ipv6", Does: runAction, Heavy: true, Writes: true,
		Summary:    "prove actual Ze IPv6 subscriber service over VPP",
		Why:        "Run staged Ze and pinned pppd in plain, namespace and VLAN topologies. Requires RA prefix, IA_NA/IA_PD, source-specific native routes, bidirectional data and independent-family closure. The Ze-kernel QEMU docker-lab carrier supplies kernel prerequisites.",
		Parameters: []leaction.Parameter{{Keyword: imageKeyword, Kind: leaction.KindString, Value: imageReferenceValue, Requirement: leaction.Optional, Help: "an immutable prebuilt VPP image identity"}}, AnswerArgs: answerProductIPv6},
	leaction.Action{Verb: "product-ipv6-peer", Does: runAction, Heavy: true,
		NoSlot:  "the parent IPv6 product action owns admission; this CPE helper performs one bounded RS and DHCPv6 exchange",
		Summary: "receive actual subscriber IPv6 control replies",
		Why:     "Internal independent CPE helper. It binds only the client's negotiated PPP interface, sends RS and DHCPv6, and returns received wire bytes; it never serves Ze or installs VPP state.",
		Parameters: []leaction.Parameter{
			{Keyword: "interface", Kind: leaction.KindString, Value: nameToken, Requirement: leaction.Required, Help: "the independent CPE PPP interface"},
			{Keyword: "local", Kind: leaction.KindString, Value: addressToken, Requirement: leaction.Required, Help: "the negotiated CPE link-local address"},
		}, AnswerArgs: answerProductIPv6Peer},
	leaction.Action{Verb: "product-family-peer", Does: runAction, Heavy: true,
		NoSlot:  "the parent IPv6 product action owns admission; this helper performs one bounded exact-session exchange",
		Summary: "probe or close one actual negotiated family",
		Why:     "Internal exact-session wire stimulus. Paired positive and negative raw echo probes distinguish VPP receive gates from the independent CPE kernel's NCP mode.",
		Parameters: []leaction.Parameter{
			{Keyword: "interface", Kind: leaction.KindString, Value: nameToken, Requirement: leaction.Required, Help: "the independent CPE access interface"},
			{Keyword: "peer", Kind: leaction.KindString, Value: "MAC", Requirement: leaction.Required, Help: "the observed physical AC address"},
			{Keyword: "sid", Kind: leaction.KindString, Value: "number", Requirement: leaction.Required, Help: "the actual negotiated nonzero session identifier"},
			{Keyword: "operation", Kind: leaction.KindString, Value: nameToken, Requirement: leaction.Required, Help: "probe4, probe6, close4 or close6"},
			{Keyword: "source", Kind: leaction.KindString, Value: addressToken, Requirement: leaction.Required, Help: "the negotiated source address; :: for close"},
			{Keyword: "target", Kind: leaction.KindString, Value: addressToken, Requirement: leaction.Required, Help: "the independent traffic peer; :: for close"},
		}, AnswerArgs: answerProductFamily},
	leaction.Action{Verb: "product", Does: runAction, Heavy: true, Writes: true,
		Summary:    "prove configured Ze PPPoE over native VPP",
		Why:        "Run the actual staged daemon with the VPP backend, independent pppd and a traffic peer in plain, named-namespace, VLAN and hundred-client topologies. Requires Ze's runtime kernel; the QEMU docker-lab route supplies it. The optional image must be an immutable VPP image identity.",
		Parameters: []leaction.Parameter{{Keyword: imageKeyword, Kind: leaction.KindString, Value: imageReferenceValue, Requirement: leaction.Optional, Help: "the immutable prebuilt VPP image to test"}}, AnswerArgs: answerProduct},
	leaction.Action{Verb: "observe", Does: runAction, Heavy: true,
		NoSlot:     "the parent product action owns admission; this helper performs one bounded loopback HTTP observation",
		Summary:    "read the actual product daemon's lab state",
		Why:        "Internal helper for the product lab's authenticated loopback REST show commands. It starts no process, builds nothing and changes no daemon state.",
		Parameters: []leaction.Parameter{{Keyword: "command", Kind: leaction.KindString, Value: "show-command", Requirement: leaction.Required, Help: "show pppoe sessions or show pppoe interfaces"}}, AnswerArgs: answerProductObserve},
	leaction.Action{Verb: runAction, Heavy: true, Writes: true, Summary: "prove the patched VPP PPPoE control mechanism",
		Why: "Run real VPP and pppd in isolated containers, with an independent traffic sender. Requires Ze's runtime kernel; the QEMU docker-lab route supplies it. image pins a prebuilt VPP identity for counterfactual runs, while the helper and independent peer are freshly prepared.", Parameters: []leaction.Parameter{{Keyword: imageKeyword, Kind: leaction.KindString, Value: imageReferenceValue, Requirement: leaction.Optional, Help: "the immutable prebuilt VPP image to test"}}, AnswerArgs: answerRun},
	leaction.Action{Verb: "proof", Does: runAction, Heavy: true, Writes: true,
		Summary: "prove mechanism, bindings and parser discrimination",
		Why:     "Run the complete baseline mechanism, the isolated control-binding API regression using that same immutable image, then the missing-metadata parser oracle on the same Docker daemon. Requires Ze's runtime kernel through the QEMU docker-lab carrier. All three results and their image identities appear in the report.", Answer: answerProof},
	leaction.Action{Verb: "bindings", Does: runAction, Heavy: true, Writes: true,
		Summary: "test VPP PPPoE control-binding API cases",
		Why:     "Run one baseline VPP peer and require all ten control-binding API cases to pass. Requires Ze's runtime kernel through the QEMU docker-lab carrier. This manually invoked regression uses the normal source image and reports its immutable identity.", Answer: answerBindings},
	leaction.Action{Verb: "control", Does: runAction, Heavy: true, NoSlot: "the parent mechanism action owns admission; this bounded socket helper performs no build", Summary: "serve one lab session without a kernel PPP unit",
		Why: "Internal Linux helper used by the VPP mechanism lab. It owns one bounded raw discovery and PPPoX socket lifetime.",
		Parameters: []leaction.Parameter{
			{Keyword: interfaceKeyword, Kind: leaction.KindString, Value: nameToken, Requirement: leaction.Required, Help: "the Linux control TAP"},
			{Keyword: "netns", Kind: leaction.KindString, Value: nameToken, Requirement: leaction.Optional, Help: "the namespace in which sockets are opened"},
		}, AnswerArgs: answerControl},
	leaction.Action{Verb: "bounds", Does: runAction, Heavy: true, NoSlot: "the parent mechanism action owns admission; this helper sends three bounded frames", Summary: "inject malformed PPPoE and a discovery barrier", Why: "Internal wire counterfactual: send physically padded IPv6 with a truncated declared PPPoE payload, then a valid discovery marker.", Parameters: injectParameters(), AnswerArgs: func(args leaction.Arguments) (any, int) { return answerInject(args, false) }},
	leaction.Action{Verb: "ipv6", Does: runAction, Heavy: true, NoSlot: "the parent mechanism action owns admission; this helper sends two bounded frames", Summary: "inject valid IPv6 subscriber control packets", Why: "Internal wire peer: send a Router Solicitation and DHCPv6 Solicit through the established VPP session.", Parameters: injectParameters(), AnswerArgs: func(args leaction.Arguments) (any, int) { return answerInject(args, true) }},
)

func init() {
	leroot.Register(area, leroot.GroupSuite, actions.Answer, registry.Meta{ShortHelp: "prove the VPP PPPoE mechanism", Description: "The Phase 0 control-socket and data-plane mechanism proof.", Mode: "offline", Section: registry.SectionTest, SubsFunc: actions.Subs})
	leroot.RegisterActions(area, actions.Actions)
	leroot.RegisterShape(area, command.ShapeDoc)
}

func answerRun(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the mechanism lab cannot find the checkout", "Its source-built peers cannot be prepared.", "./le test deployment vpp-pppoe run"))
		return nil, 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report := Run(ctx, root, args.One(imageKeyword))
	return report, report.Code
}

func answerProof() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the mechanism proof cannot find the checkout",
			"Its baseline and counterfactual images cannot be prepared.", "./le test deployment vpp-pppoe proof"))
		return nil, 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report := runProof(ctx, root)
	return &report, report.Code
}

func answerBindings() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the binding regression cannot find the checkout",
			"Its baseline VPP image cannot be prepared.", "./le test deployment vpp-pppoe bindings"))
		return nil, 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report := runBindingProof(ctx, root)
	return report, report.Code
}

func injectParameters() []leaction.Parameter {
	return []leaction.Parameter{
		{Keyword: interfaceKeyword, Kind: leaction.KindString, Value: nameToken, Requirement: leaction.Required, Help: "the sender's access interface"},
		{Keyword: "peer", Kind: leaction.KindString, Value: "MAC", Requirement: leaction.Required, Help: "the VPP access MAC address"},
	}
}

func answerInject(args leaction.Arguments, ipv6 bool) (any, int) {
	if err := injectFrames(args.One(interfaceKeyword), args.One("peer"), ipv6); err != nil {
		leaction.ReportError(opererr.Wrap(err, "the mechanism wire injection failed", "No packet-path verdict can be taken without the injection.", "./le test deployment vpp-pppoe run"))
		return nil, 1
	}
	return map[string]bool{"sent": true}, 0
}

func answerControl(args leaction.Arguments) (any, int) {
	ctx, cancel := context.WithTimeout(context.Background(), controlLifetime)
	defer cancel()
	result, err := runControl(ctx, args.One(interfaceKeyword), args.One("netns"))
	if err != nil {
		if writeErr := os.WriteFile("/evidence/control.error", []byte(err.Error()), 0o600); writeErr != nil {
			err = errors.Join(err, writeErr)
		}
		leaction.ReportError(opererr.Wrap(err, "the lab PPP control channel failed", "No mechanism verdict can be taken from an incomplete control exchange.", "./le test deployment vpp-pppoe run"))
		return result, 1
	}
	if err = os.WriteFile("/evidence/control.done", []byte("done\n"), 0o600); err != nil {
		leaction.ReportError(opererr.Wrap(err, "the control helper could not publish completion", "The caller cannot join the helper's lifetime.", "./le test deployment vpp-pppoe run"))
		return result, 1
	}
	return result, 0
}
