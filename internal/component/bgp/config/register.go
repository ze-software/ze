// Design: docs/architecture/config/syntax.md -- BGP config registration hooks

package bgpconfig

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/grmarker"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// attachmentOwnerBGP is the registration name of the BGP daemon plugin
// (pluginNameBGP in internal/component/bgp/plugin/register.go), whose config
// holds the peer chains. policy's attachment-owner test refuses an owner that
// is not a registered plugin depending on policy, so the two cannot drift.
const attachmentOwnerBGP = "bgp"

func init() {
	// The filter attachment points a BGP peer's config holds, with the route
	// fields their routes carry and a filter there may set (the route field
	// vocabulary, internal/component/policy/fields.go). They are declared here,
	// in the package whose validateChainNames checks a chain against them, so
	// every binary that links the check links the declaration too. Each
	// carried field has a BGP view reader (internal/component/bgp/policyview).
	// metric is not carried: BGP names that fact med, one name per fact, and
	// the synonym makes a chain reading metric here name med in its refusal.
	carries := []policy.Field{policy.FieldPrefix, policy.FieldFamily, policy.FieldProtocol, policy.FieldNextHop, policy.FieldOriginAS, policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity, policy.FieldASPath, policy.FieldLocalPreference, policy.FieldOrigin, policy.FieldAIGP, policy.FieldMED,
		policy.FieldAtomicAggregate, policy.FieldAggregator, policy.FieldOriginatorID, policy.FieldClusterList}
	for _, field := range carries {
		if _, err := policyview.ReaderFor(field); err != nil {
			panic("BUG: bgp attachments carry " + field.String() + ", which the BGP route view cannot read: " + err.Error())
		}
	}
	settable := []policy.Field{policy.FieldASPath, policy.FieldLocalPreference, policy.FieldCommunity, policy.FieldLargeCommunity, policy.FieldExtendedCommunity, policy.FieldMED, policy.FieldOrigin, policy.FieldNextHop, policy.FieldAIGP}
	// Each attachment's chains sit at `bgp ... filter <direction> filters`,
	// global, group and peer alike. The editor completes a chain, and commit
	// judges it, with the check a peer's chain passes at load.
	for _, attachment := range []string{"bgp import", "bgp export"} {
		policy.MustRegisterAttachment(attachmentOwnerBGP, attachment, carries, settable)
		policy.MustRegisterFieldSynonym(attachment, policy.FieldMetric, policy.FieldMED)
		direction := strings.TrimPrefix(attachment, attachmentOwnerBGP+" ")
		policy.MustRegisterChainLeaf(attachmentOwnerBGP, direction, attachment, func(reg *policy.Registry, member, context string) error {
			return validateChainNames(reg, []policy.Ref{{Name: member}}, attachment, context)
		})
	}

	zeconfig.RegisterPluginExtractor(string(zeconfig.ConfigTypeBGP), extractBGPInlinePlugins)
	if err := zeconfig.RegisterTreeValidator("bgp-filter-policy", validateFilterPolicies); err != nil {
		panic("BUG: BGP filter policy validator registration failed: " + err.Error())
	}
	registry.RegisterReactorFactory(createReactorFromCoordinator)

	// Fill the always-on infra seams (internal/component/config/infra) so
	// `ze config dump|diff|validate` and the daemon reboot path reach BGP
	// behavior without importing this package. With ze_bgp off these stay
	// nil and every caller takes its no-BGP branch.
	infra.SetBGPTreeResolver(ResolveBGPTree)
	infra.SetBGPPeerValidator(validatePeersFromTree)
	infra.SetBGPRolelessPeerReporter(rolelessPeersFromTree)
	infra.SetBGPPolicyValidator(policyAbsencesFromTree)
	infra.SetGRMarkerWriter(writeGRMarker)

	// A peer that asks for BFD strict mode in a configuration with no BFD
	// engine never establishes at all, and nothing on its own surface says why.
	// bfd_strict_doctor.go reports that arrangement before the daemon starts.
	for _, meta := range bfdStrictDiagnosticCodes {
		_ = diagnostic.Register(meta)
	}
	if err := diagnostic.RegisterDoctorCheck(bfdStrictDoctorCheck); err != nil {
		doctorRegistrationFailed("bfd strict", err)
	}

	// A filter chain naming a policy nothing defines leaves the peer with a
	// shorter chain than the operator wrote. filter_reference_doctor.go reports
	// it from the tree, against the one filterInstanceName this package holds.
	if err := diagnostic.RegisterDoctorCheck(filterReferenceDoctorCheck); err != nil {
		doctorRegistrationFailed("filter reference", err)
	}

	// A BGP daemon that cannot read the kernel neighbor table sends every RFC
	// 2545 next hop naming another router without that router's link-local
	// (neighbor_table_doctor.go).
	for _, meta := range neighborTableDiagnosticCodes {
		_ = diagnostic.Register(meta)
	}
	if err := diagnostic.RegisterDoctorCheck(neighborTableDoctorCheck); err != nil {
		doctorRegistrationFailed("neighbor table", err)
	}

	// The peer resolution, TCP MD5, RFC 9234 role and capture directory checks
	// read BGP configuration, so this package owns them (doctor_checks.go).
	registerBGPDoctorChecks()
}

// doctorRegistrationFailed stops the process on a refused registration. Every
// refusal is a programmer error in the table beside the call -- a duplicate
// name, an unknown phase, a code without the doctor prefix -- so the daemon
// must not start one readiness check short of what it declares.
func doctorRegistrationFailed(check string, err error) {
	var tb textbuf.Buffer
	tb.Str("bgp: ").Str(check).Str(" doctor check registration failed: ").Err(err).Byte('\n')
	tb.StdErr() //nolint:errcheck // the process is exiting on the next line
	os.Exit(1)
}

// validatePeersFromTree adapts PeersFromConfigTree to the infra.BGPPeerValidator
// seam: callers use it purely as a gate, and the built []*reactor.PeerSettings
// is a BGP type the always-on side must not name.
func validatePeersFromTree(tree *zeconfig.Tree) error {
	_, err := PeersFromConfigTree(tree)
	return err
}

// policyAbsencesFromTree adapts peersAndDynamicGroups to the
// infra.BGPPolicyValidator seam: the same walk validatePeersFromTree gates on,
// plus each RFC 8212 absence on the peers and dynamic groups it built, worded
// by reactor.RFC8212Absences exactly as the establishment warning words it.
//
// A dynamic group with no remote AS is not judged: its members are EBGP or
// IBGP by the AS each OPEN carries, so only the session can tell, and the
// reactor warns then. The zero PeerAS is that "decided by the OPEN" case, not
// a missing answer.
func policyAbsencesFromTree(tree *zeconfig.Tree) ([]infra.BGPPolicyAbsence, error) {
	peers, groups, err := peersAndDynamicGroups(tree)
	if err != nil {
		return nil, err
	}
	var absences []infra.BGPPolicyAbsence
	for _, ps := range peers {
		absences = appendPolicyAbsences(absences, "peer "+ps.Name, ps)
	}
	for _, group := range groups {
		if group.Settings.PeerAS == 0 {
			continue
		}
		absences = appendPolicyAbsences(absences, "group "+group.GroupName, group.Settings)
	}
	return absences, nil
}

// appendPolicyAbsences appends the RFC 8212 absences of one session template.
func appendPolicyAbsences(absences []infra.BGPPolicyAbsence, owner string, ps *reactor.PeerSettings) []infra.BGPPolicyAbsence {
	for _, a := range reactor.RFC8212Absences(ps) {
		absences = append(absences, infra.BGPPolicyAbsence{Owner: owner, Direction: a.Direction, Message: a.Message, Fix: a.Fix})
	}
	return absences
}

// writeGRMarker persists the RFC 4724 Restarting-Speaker marker so the next
// reactor start sets the R bit in its OPEN capabilities. Called through the
// infra.GRMarkerWriter seam on an operator-initiated restart or reboot.
func writeGRMarker(caps []plugin.InjectedCapability, store storage.Storage) {
	maxRestart := grmarker.MaxRestartTime(caps)
	if maxRestart <= 0 {
		return
	}
	expiresAt := time.Now().Add(time.Duration(maxRestart) * time.Second)
	log := slogutil.Logger("bgp.gr")
	if err := grmarker.Write(store, expiresAt); err != nil {
		log.Error("failed to write GR marker", "error", err)
		return
	}
	log.Info("GR marker written", "expires", expiresAt)
}

// bgpCaptureHandle is the method set the BGP config-transaction callbacks reach
// through a type assertion on the handle this factory returns
// (internal/component/bgp/plugin, captureBGPConfigEvent). That assertion FAILS
// OPEN: a handle that does not satisfy it records nothing and says nothing.
//
// The assertion below makes the failure a BUILD error here instead. It is the
// only place both types are visible: bgp/plugin cannot import bgp/reactor, and
// the interface there is unexported. Without it, wrapping the returned reactor
// would silently kill config event capture (spec improve-3, AC-6).
type bgpCaptureHandle interface {
	CapturesOpen() bool
	CaptureConfigEvent(op, txID string, payload []byte)
}

var _ bgpCaptureHandle = (*reactor.Reactor)(nil)

// createReactorFromCoordinator builds a BGP reactor using config state stored
// in the coordinator by the hub. This keeps bgp/config imports out of the hub.
func createReactorFromCoordinator(coord registry.CoordinatorAccessor) (registry.BGPReactorHandle, error) {
	bs := coord.Bootstrap()
	configPath := bs.ConfigPath
	cliPlugins := bs.CLIPlugins
	configData := bs.ConfigData

	// The hub records the selected source in stored history beside startup
	// (storage.BootRecord); until the record lands, the bound handle answers
	// the boot bytes as active and no candidate, so this reads the same bytes
	// either way. A newly enabled BGP plugin uses the staged reload candidate; a restarted
	// plugin uses the accepted version. Neither reopens a mutable loose file.
	// Captured bytes remain authoritative for stdin and storeless startup.
	store := bs.Store
	data := configData
	if store != nil && configPath != "" && configPath != "-" {
		candidate, _, present, err := storage.ReadCandidateConfig(store, configPath)
		if err != nil {
			return nil, fmt.Errorf("read reactor candidate: %w", err)
		}
		if present {
			data = candidate
		} else {
			data, err = storage.ReadActiveConfig(store, configPath)
			if err != nil {
				return nil, fmt.Errorf("read accepted reactor config: %w", err)
			}
		}
	}

	// Set YANG validator for runtime attribute validation (origin enum, med/local-pref ranges).
	pluginYANG := plugin.CollectPluginYANG(cliPlugins)
	if v, vErr := zeconfig.YANGValidatorWithPlugins(pluginYANG); vErr == nil && v != nil {
		plugin.SetYANGValidator(v)
	}

	result, err := zeconfig.LoadConfig(string(data), configPath, cliPlugins)
	if err != nil {
		return nil, fmt.Errorf("parse config for reactor: %w", err)
	}

	// Production: borrow the hub-owned plugin server (standalone == false). The
	// hub injects it via SetPluginServerAny before StartWithContext.
	r, err := CreateReactor(result, configPath, store, false)
	if err != nil {
		return nil, err
	}

	// Chaos injection from hub-stored config.
	injectChaos(r, coord)

	// GR marker from storage (RFC 4724 Section 4.1). The tree goes with it:
	// the marker says Ze restarted, and the configuration says whether the
	// forwarding plane kept Ze's routes while it was down.
	readGRMarker(r, store, result.Tree.ToPluginMap())

	if cb := bs.HealthPeerCallback; cb != nil {
		r.AddPeerLifecycleCallback(cb)
	}

	// MRT bridges come from the registry seam (MRT self-registers them from its
	// init()), not from BGPBootstrap -- that is what keeps cmd/ze/hub free of an
	// internal/plugins/mrt import so //go:build ze_bgp can drop MRT. nil when MRT
	// is compiled out.
	if mcb := registry.GetMRTMessageCallback(); mcb != nil {
		r.AddMessageCallback(mcb)
	}
	if pcb := registry.GetMRTPeerCallback(); pcb != nil {
		r.AddPeerLifecycleCallback(pcb)
	}

	return r, nil
}
