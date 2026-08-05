// Design: plan/spec-ipsec-dataplane-inspection.md -- kernel dataplane read surface
// RFC: rfc/short/rfc4301.md -- Section 4.4 keeps the SPD and the SAD separate
// RFC: rfc/short/rfc4303.md -- Section 2.1 reserves the low SPI values
// Related: show_ipsec.go -- the engine-belief siblings of these kernel readers
// Owned by the ike component so that removing it removes the
// `show vpn ipsec dataplane ...` commands, their schema, and these handlers
// together. See ai/rules/plugins.md.
//
// EVERY HANDLER HERE RETURNS plugin.Map AND FORMATS NOTHING. command.ApplyPipes
// renders the table and the `| json` form from the keys below
// (ai/rules/cli.md).

package cmd

import (
	"errors"
	"net"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:vpn-ipsec-dataplane-sa",
			Handler:    handleShowVPNIPsecDataplaneSA,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:vpn-ipsec-dataplane-policy",
			Handler:    handleShowVPNIPsecDataplanePolicy,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:vpn-ipsec-dataplane-drift",
			Handler:    handleShowVPNIPsecDataplaneDrift,
		},
	)
}

// driftPeerInfo is the engine-belief half of the drift comparison.
//
// It is a variable so the comparison can be driven from a fixture. PeerInfoMap
// derives its result from live PeerSession values, and building those to test a
// set difference would test the session machinery instead of the comparison. The
// engine keeps no seam of its own here, and adding one would change production
// code to suit a test; this keeps the seam in the package that consumes it.
var driftPeerInfo = engine.PeerInfoMap

// dataplaneReadError turns a failed dataplane read into an error RESPONSE that
// names its cause.
//
// The three causes are distinct and an operator acts differently on each, so
// none of them may collapse into the others and NONE of them may become an empty
// list. An empty table answers "is my tunnel programmed?" with "no", and that is
// a wrong answer whenever the truth is "nobody asked the kernel"
// (ai/rules/evidence.md).
func dataplaneReadError(what string, err error) *plugin.Response {
	var b textbuf.Buffer
	switch {
	case errors.Is(err, dataplane.ErrNotSupported):
		b.Str("the active dataplane backend cannot enumerate the ").Str(what)
		b.Str(": ").Err(err)
		b.Str("; this reports nothing about what the kernel holds, so it is not the same answer as an empty ").Str(what)
	case errors.Is(err, syscall.EPERM), errors.Is(err, syscall.EACCES):
		b.Str("reading the ").Str(what).Str(" needs CAP_NET_ADMIN, and this process does not have it: ").Err(err)
	default:
		b.Str("cannot read the ").Str(what).Str(": ").Err(err)
	}
	return &plugin.Response{Status: plugin.StatusError, Error: b.String()}
}

// activeDataplane returns the loaded backend, or the error response to send.
//
// A nil backend is its own cause: the ike component never started, or no backend
// was registered. Reporting it as an empty dataplane would say the kernel holds
// nothing when nothing has been asked.
func activeDataplane() (dataplane.Dataplane, *plugin.Response) {
	dp := dataplane.Get()
	if dp == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no ipsec dataplane backend is loaded, so the kernel cannot be read; this is not the same answer as an empty dataplane",
		}
	}
	return dp, nil
}

func handleShowVPNIPsecDataplaneSA(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	spi, errResp := dataplaneSPISelector(ctx, args)
	if errResp != nil {
		return errResp, nil
	}

	dp, errResp := activeDataplane()
	if errResp != nil {
		return errResp, nil
	}

	sas, err := dp.ListSAs(0)
	if err != nil {
		return dataplaneReadError("SAD", err), nil
	}

	// Filter and order the records, then map them once. Sorting the RECORDS
	// rather than the rendered rows keeps the order defined by the SPI itself
	// instead of by a string, and the dump order the kernel returns is not
	// stable between calls.
	matched := make([]dataplane.SAInfo, 0, len(sas))
	for i := range sas {
		if spi != 0 && sas[i].SPI != spi {
			continue
		}
		matched = append(matched, sas[i])
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].SPI < matched[j].SPI })

	rows := make([]map[string]any, 0, len(matched))
	for i := range matched {
		rows = append(rows, saInfoToMap(&matched[i]))
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"sas": rows}}, nil
}

// dataplaneSPISelector reads the optional `spi <spi>` selector.
//
// RFC 4303 Section 2.1: "The SPI value of zero (0) is reserved for local,
// implementation-specific use and MUST NOT be sent on the wire." A zero
// selector is therefore never a real SA, and accepting it as "every SPI" would
// make a typo look like a successful full dump. It is refused.
func dataplaneSPISelector(ctx *pluginserver.CommandContext, args []string) (uint32, *plugin.Response) {
	raw := ""
	if ctx != nil {
		raw = ctx.Selector("spi")
	}
	if raw == "" && len(args) >= 2 && args[0] == "spi" {
		raw = args[1]
	}
	if raw == "" {
		return 0, nil
	}

	spi, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		var b textbuf.Buffer
		b.Str("spi must be a number from 1 to 4294967295, not ").Quoted(raw)
		return 0, &plugin.Response{Status: plugin.StatusError, Error: b.String()}
	}
	if spi == 0 {
		return 0, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "spi 0 is reserved by RFC 4303 Section 2.1 and never names an installed SA; omit the selector to dump every SA",
		}
	}
	return uint32(spi), nil
}

func saInfoToMap(sa *dataplane.SAInfo) map[string]any {
	return map[string]any{
		"spi":                sa.SPI,
		"src":                ipString(sa.Src),
		"dst":                ipString(sa.Dst),
		"if-id":              sa.IfID,
		"proto":              ipsecProtoName(sa.Proto),
		"mode":               ipsecModeName(sa.Mode),
		"reqid":              sa.ReqID,
		"encryption":         sa.Encryption,
		"encryption-keybits": sa.EncryptionKeyBits,
		"integrity":          sa.Integrity,
		"integrity-keybits":  sa.IntegrityKeyBits,
		"replay-window":      sa.ReplayWindow,
		"bytes":              sa.BytesCurrent,
		"packets":            sa.PacketsCurrent,
		"bytes-hard":         sa.BytesHard,
		"packets-hard":       sa.PacketsHard,
		"added-at":           timeString(sa.AddedAt),
		"used-at":            timeString(sa.UsedAt),
	}
}

func handleShowVPNIPsecDataplanePolicy(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	dp, errResp := activeDataplane()
	if errResp != nil {
		return errResp, nil
	}

	policies, err := dp.ListPolicies()
	if err != nil {
		return dataplaneReadError("SPD", err), nil
	}

	rows := make([]map[string]any, 0, len(policies))
	for i := range policies {
		rows = append(rows, policyInfoToMap(&policies[i]))
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"policies": rows}}, nil
}

func policyInfoToMap(p *dataplane.PolicyInfo) map[string]any {
	return map[string]any{
		"src":         prefixString(p.Src),
		"dst":         prefixString(p.Dst),
		"src-port":    portString(p.SrcPort),
		"dst-port":    portString(p.DstPort),
		"direction":   policyDirName(p.Dir),
		"upper-proto": p.UpperProto,
		"priority":    p.Priority,
		"if-id":       p.IfID,
		"action":      policyActionName(p.Action),
		"mode":        ipsecModeName(p.Mode),
		"reqid":       p.ReqID,
		"tunnel-src":  ipString(p.TunnelSrc),
		"tunnel-dst":  ipString(p.TunnelDst),
		// A policy ze did not install is the common case on a node running
		// another IKE daemon. It is reported as unknown, never as a blank cell
		// that reads as unowned (ai/rules/evidence.md). owner-known keeps the
		// two apart for a JSON reader, and for a peer that is itself called
		// "unknown".
		"owner":       policyOwnerName(p),
		"owner-known": p.OwnerKnown,
	}
}

func handleShowVPNIPsecDataplaneDrift(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	dp, errResp := activeDataplane()
	if errResp != nil {
		return errResp, nil
	}

	sas, err := dp.ListSAs(0)
	if err != nil {
		return dataplaneReadError("SAD", err), nil
	}

	// The comparison runs in ONE direction: an SPI the engine expects that the
	// kernel does not hold is drift. An SPI the kernel holds that the engine does
	// not expect is NOT drift, and that asymmetry is what makes a rekey window
	// quiet. RFC 7296 Section 2.8 has the old and the new Child SA coexist until
	// the old one is deleted, so the kernel legitimately holds both while the
	// engine names only the replacement.
	inKernel := make(map[uint32]bool, len(sas))
	for i := range sas {
		inKernel[sas[i].SPI] = true
	}

	peers := driftPeerInfo()
	names := make([]string, 0, len(peers))
	for name := range peers {
		names = append(names, name)
	}
	sort.Strings(names)

	found := make([]driftFinding, 0)
	for _, name := range names {
		info := peers[name]
		if !info.HasChild {
			continue
		}
		for _, want := range []struct {
			spi uint32
			dir string
		}{
			{info.ChildInSPI, "inbound"},
			{info.ChildOutSPI, "outbound"},
		} {
			if want.spi == 0 || inKernel[want.spi] {
				continue
			}
			found = append(found, driftFinding{
				peer: name, spi: want.spi, dir: want.dir, ifID: info.ChildIfID,
			})
		}
	}

	if len(found) == 0 {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"drift": []map[string]any{}},
		}, nil
	}

	return &plugin.Response{Status: plugin.StatusError, Error: driftMessage(found)}, nil
}

// driftFinding is one Child SA the engine counts as installed that the kernel
// SAD does not hold.
type driftFinding struct {
	peer string
	spi  uint32
	dir  string
	ifID uint32
}

// driftMessage renders every finding into the error string.
//
// EVERY FACT GOES IN THE MESSAGE. Drift returns StatusError so the command exits
// non-zero (AC-3), and the dispatcher rebuilds an error response from its
// message alone and discards Data. A finding left out of this string is a
// finding the operator never sees.
func driftMessage(found []driftFinding) string {
	var b textbuf.Buffer
	b.Str("ipsec dataplane drift: the ike engine believes ").Int(int64(len(found)))
	if len(found) == 1 {
		b.Str(" child SA is installed that the kernel SAD does not hold:")
	} else {
		b.Str(" child SAs are installed that the kernel SAD does not hold:")
	}
	for i := range found {
		b.Str(" peer ").Quoted(found[i].peer)
		b.Byte(' ').Str(found[i].dir)
		b.Str(" spi ").Uint32(found[i].spi)
		if found[i].ifID != 0 {
			b.Str(" if_id ").Uint32(found[i].ifID)
		}
		b.Byte(';')
	}
	return b.String()
}

// unknownValue is what a renderer prints when the answer is not available. It
// is a word rather than a blank so a reader can tell "not known" from "none"
// (ai/rules/evidence.md).
const unknownValue = "unknown"

// The helpers below render ONE VALUE each into the JSON payload. They are not
// output formatting: the table and the `| json` form are both built by
// command.ApplyPipes from the keys above.

func ipString(ip net.IP) string {
	if len(ip) == 0 {
		return ""
	}
	return ip.String()
}

// prefixString names the wildcard rather than rendering an empty span, so a
// site-to-site selector is legible. A nil prefix is the wildcard ze installs.
func prefixString(n *net.IPNet) string {
	if n == nil {
		return "any"
	}
	return n.String()
}

func portString(p dataplane.PortMatch) string {
	if p.IsAny() {
		return "any"
	}
	var b textbuf.Buffer
	return b.Uint16(p.Port).String()
}

// timeString renders the zero time as an empty string. The zero time means the
// kernel never recorded the event, and rendering it as 1970-01-01 would put a
// plausible date where there is no answer.
func timeString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func ipsecProtoName(proto uint8) string {
	switch proto {
	case 50:
		return "esp"
	case 51:
		return "ah"
	default:
		var b textbuf.Buffer
		return b.Str("proto-").Uint8(proto).String()
	}
}

func ipsecModeName(mode uint8) string {
	switch mode {
	case dataplane.ModeTransport:
		return "transport"
	case dataplane.ModeTunnel:
		return "tunnel"
	default:
		return unknownValue
	}
}

func policyDirName(dir dataplane.SADir) string {
	switch dir {
	case dataplane.SADirIn:
		return "in"
	case dataplane.SADirOut:
		return "out"
	case dataplane.SADirFwd:
		return "fwd"
	default:
		return unknownValue
	}
}

func policyActionName(a dataplane.SPAction) string {
	if a == dataplane.SPActionBypass {
		return "bypass"
	}
	return "protect"
}

// policyOwnerName renders the owner join's miss as a word rather than a blank.
//
// A blank owner reads as "this policy belongs to nobody". The truth is "ze did
// not install it", which on a node running another IKE daemon is both common and
// important (ai/rules/evidence.md).
func policyOwnerName(p *dataplane.PolicyInfo) string {
	if !p.OwnerKnown {
		return unknownValue
	}
	if p.Owner == "" {
		return "unnamed"
	}
	return p.Owner
}
