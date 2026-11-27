// Design: docs/guide/graceful-restart.md -- graceful restart capability configuration diagnostics
// Related: register.go -- the DoctorChecks entry that installs checkGRCapabilityConfig
// Related: gr_capability.go -- parseGRCapValue warnings carrying codeDoctorBGPGRCapability
// Related: gr_llgr.go -- parseLLGRCapValue warnings carrying codeDoctorBGPGRCapability
//
// The post-config doctor check reads the pruned bgp section the engine
// delivers to this plugin and surfaces the exact Graceful Restart (code 64)
// and Long-Lived Graceful Restart (code 71) capability clamping, truncation,
// and omission conditions that parseGRCapValue and parseLLGRCapValue warn on.

package gr

import (
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// codeDoctorBGPGRCapability names a Graceful Restart or Long-Lived Graceful
// Restart capability configuration issue that causes clamping, truncation, or
// omission of an address family in OPEN capability codes 64 or 71.
const codeDoctorBGPGRCapability = "doctor-bgp-gr-capability"

// codeDoctorBGPGRCapabilityDecode fires when a received or sent BGP OPEN
// carries an undecodable Graceful Restart (code 64) or Long-Lived Graceful
// Restart (code 71) capability payload or hex string.
const codeDoctorBGPGRCapabilityDecode = "doctor-bgp-gr-capability-decode"

const (
	grDoctorFix           = "ze config validate"
	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

var errGRInvalidBGPConfigJSON = errors.New("bgp-gr: invalid bgp configuration JSON")

var grDoctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorBGPGRCapability,
		Title: "Graceful Restart or LLGR capability configuration has clamped or omitted entries",
		Description: "A peer's `graceful-restart` block (`short-lived` or `long-lived`) configures a timer outside " +
			"the wire field range (`restart-time` > 4095 or `stale-time` outside 0..16777215), lists more address " +
			"families than the RFC 4724 / RFC 5492 capability length permits (63 for GR, 36 for LLGR), or names " +
			"an address family that is unknown or not stored by the RIB. Ze clamps out-of-range timers and omits " +
			"unsupported or overflowing tuples from codes 64 and 71. Remedy: adjust `short-lived` and `long-lived` " +
			"timers and family lists under `session capability graceful-restart`, then run `ze config validate`.",
		Examples: []string{exampleDoctorJSON, exampleShowDiagPrefix + codeDoctorBGPGRCapability},
	},
	{
		Code:  codeDoctorBGPGRCapabilityDecode,
		Title: "Graceful Restart or LLGR capability in BGP OPEN could not be decoded",
		Description: "A received or sent BGP OPEN message carried a malformed Graceful Restart (code 64) or Long-Lived Graceful " +
			"Restart (code 71) capability payload, or its OPEN optional parameters could not be unpacked. bgp-gr ignored the " +
			"undecodable capability, so GR or LLGR route retention may not activate for that peer. Warnings are rate-limited per " +
			"peer and capability kind. Remedy: inspect the peer session and negotiated capabilities with " +
			"`ze show bgp peer <peer> detail`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorBGPGRCapabilityDecode},
	},
}

// grDoctorEventCodes declares the transient report-bus event codes this package raises.
var grDoctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorBGPGRCapabilityDecode},
}

func registerGRDoctorCodes() {
	for _, meta := range grDoctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: bgp-gr: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range grDoctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: bgp-gr: doctor event code registration refused: " + err.Error())
		}
	}
}

// checkGRCapabilityConfig inspects the pruned BGP config tree for any GR or
// LLGR capability setting that parseGRCapValue or parseLLGRCapValue clamps,
// truncates, or omits.
func checkGRCapabilityConfig(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	if ctx.Tree == nil {
		return nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     codeDoctorBGPGRCapability,
			Severity: string(diagnostic.SeverityWarning),
			Message:  "bgp-gr: could not inspect graceful-restart capabilities because the doctor runner passed an unexpected config tree type.",
			Fix:      grDoctorFix,
		}}
	}
	if tree == nil {
		return nil
	}
	if tree.GetContainer(configRootBGP) == nil {
		return nil
	}
	issues, err := grCapabilityIssuesFromTree(tree)
	if err != nil {
		return []rpc.DoctorCheckDiagnostic{{
			Code:     codeDoctorBGPGRCapability,
			Severity: string(diagnostic.SeverityWarning),
			Message: textbuf.New().
				Str("bgp-gr: could not inspect graceful-restart capabilities because the bgp configuration could not be read: ").
				Err(err).Str(".").String(),
			Fix: grDoctorFix,
		}}
	}
	if len(issues) == 0 {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     codeDoctorBGPGRCapability,
		Severity: string(diagnostic.SeverityWarning),
		Message: textbuf.New().
			Str("bgp-gr: graceful-restart capability configuration has clamped, truncated, or omitted entries: ").
			Join(issues, "; ").Str(".").String(),
		Fix: grDoctorFix,
	}}
}

func grCapabilityIssuesFromTree(tree *config.Tree) ([]string, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, err
	}
	clone := tree.Clone()
	config.PruneInactive(clone, schema)

	sections, err := config.BuildPluginConfigSections(clone.ToPluginMap(), []string{configRootBGP})
	if err != nil {
		return nil, err
	}
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		bgpSubtree, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return nil, errGRInvalidBGPConfigJSON
		}
		return grCapabilityIssues(bgpSubtree), nil
	}
	return nil, nil
}

func grCapabilityIssues(bgpSubtree map[string]any) []string {
	var issues []string
	configjson.ForEachPeer(bgpSubtree, func(peerAddr string, peerMap, groupMap map[string]any, _ configjson.PeerOrigin) {
		capMap := grCapabilityFor(peerMap, groupMap)
		if capMap == nil {
			return
		}
		grData, ok := capMap[grConfigName].(map[string]any)
		if !ok {
			return
		}
		if shortData, hasShort := grData[grShortLivedNode].(map[string]any); hasShort {
			issues = append(issues, shortLivedCapabilityIssues(peerAddr, shortData, collectPeerFamilies(peerMap, groupMap))...)
		}
		if longData, hasLong := grData[grLongLivedNode].(map[string]any); hasLong {
			issues = append(issues, longLivedCapabilityIssues(peerAddr, longData)...)
		}
	})
	return issues
}

func shortLivedCapabilityIssues(peerAddr string, shortData map[string]any, sessionFamilies []string) []string {
	var issues []string
	if rtVal, exists := shortData[grRestartTimeLeaf]; exists {
		if parsed, ok := configvalue.Int(rtVal); ok && parsed > grRestartTimeMax {
			issues = append(issues, textbuf.New().Str("peer ").Str(peerAddr).Str(" short-lived restart-time ").Int(parsed).Str(" exceeds 12-bit max 4095").String())
		}
	}
	families := grAdvertisedFamilies(shortData, sessionFamilies)
	tuples := 0
	for _, name := range families {
		if tuples == grFamilyMax {
			issues = append(issues, textbuf.New().Str("peer ").Str(peerAddr).Str(" short-lived family list exceeds RFC 4724 maximum of ").Int(grFamilyMax).String())
			break
		}
		fam, ok := family.LookupFamily(name)
		if !ok {
			issues = append(issues, "peer "+peerAddr+" short-lived unknown family "+name)
			continue
		}
		if !nlrisplit.Supported(fam) {
			issues = append(issues, "peer "+peerAddr+" short-lived family "+name+" cannot be stored by RIB")
			continue
		}
		tuples++
	}
	return issues
}

func longLivedCapabilityIssues(peerAddr string, longData map[string]any) []string {
	var issues []string
	entries := configvalue.ListEntries(longData[grFamilyNode])
	tuples := 0
	for _, entry := range entries {
		if tuples == llgrFamilyMax {
			issues = append(issues, textbuf.New().Str("peer ").Str(peerAddr).Str(" long-lived family list exceeds RFC 5492 maximum of ").Int(llgrFamilyMax).String())
			break
		}
		fam, ok := family.LookupFamily(entry.Key)
		if !ok {
			issues = append(issues, "peer "+peerAddr+" long-lived unknown family "+entry.Key)
			continue
		}
		if !nlrisplit.Supported(fam) {
			issues = append(issues, "peer "+peerAddr+" long-lived family "+entry.Key+" cannot be stored by RIB")
			continue
		}
		llstVal, ok := configvalue.Int(entry.Fields[grStaleTimeLeaf])
		if !ok {
			issues = append(issues, "peer "+peerAddr+" long-lived family "+entry.Key+" has non-integer stale-time")
			continue
		}
		if llstVal < 0 {
			issues = append(issues, textbuf.New().Str("peer ").Str(peerAddr).Str(" long-lived family ").Str(entry.Key).Str(" has negative stale-time ").Int(llstVal).String())
			continue
		}
		if llstVal > llgrStaleTimeMax {
			issues = append(issues, textbuf.New().Str("peer ").Str(peerAddr).Str(" long-lived family ").Str(entry.Key).Str(" stale-time ").Int(llstVal).Str(" exceeds 24-bit max 16777215").String())
		}
		tuples++
	}
	return issues
}
