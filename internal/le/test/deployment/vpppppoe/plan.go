// Design: docs/labs/pppoe-interop.md -- the real patched VPP mechanism lab.
// Related: check.go -- assertions against VPP, pppd and an independent host.
package testvpppppoe

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	buildvpp "github.com/ze-software/ze/internal/le/build/vpp"
	"github.com/ze-software/ze/internal/le/interoplab"
	_ "github.com/ze-software/ze/internal/le/interoplab/pppoe" // Reuse its registered pppd image and pin.
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const (
	vppPeer                = "vpp"
	clientPeer             = "client"
	trafficPeer            = "traffic"
	clientImage            = "ze-pppoe-client"
	helperPath             = "/mechanism/le"
	evidencePath           = "/evidence"
	peerAddress            = "10.20.0.2"
	serverAddress          = "10.20.0.1"
	controlInterface       = "cp0"
	sessionID              = 17
	vppCLIPath             = "/run/vpp/cli.sock"
	vppctlCommand          = "vppctl"
	setCommand             = "set"
	nameToken              = "name"
	interfaceKeyword       = "interface"
	readFileCommand        = "cat"
	sessionInterface       = "pppoe_session0"
	networkAdminCapability = "NET_ADMIN"
	baselineProofVariant   = "baseline"
	testCommand            = "test"
	runAction              = "run"

	showCommand         = "show"
	accessDevice        = "eth0"
	vlanAccessDevice    = "eth0.100"
	controlNamespace    = "cp-ns"
	mtuToken            = "mtu"
	pingCommand         = "ping"
	hostAccessInterface = "host-eth0"
	pppdDebugOption     = "debug"
	packetsToken        = "packets"
	helpCommand         = "help"
	vppEvidenceLog      = "/evidence/vpp.log"
	productService      = "internet"
	pppdCommand         = "pppd"
	pppdPluginOption    = "plugin"
	pppdNoAuthOption    = "noauth"
	pppdNoACCompOption  = "noaccomp"
	imageKeyword        = "image"
	addressToken        = "address"
	versionCommand      = "version"
	pppdLogfileOption   = "logfile"
	pppdPPPoEPlugin     = "pppoe.so"
	pppdNoCCPOption     = "noccp"
	imageReferenceValue = "reference"
	pppdNICPrefix       = "nic-"
	pppdNoPCompOption   = "nopcomp"
	pppdNoVJOption      = "novj"
	pppdNoDefaultRoute  = "nodefaultroute"
	pppdMRUOption       = "mru"
	pppdMaxFailOption   = "maxfail"
	pppdNoDetachOption  = "nodetach"
)

// Run builds the mechanism's dependencies and runs three isolated topologies.
// Every container, network and temporary helper belongs to this invocation;
// Suite MUST finish removing its resources before the scratch is removed.
func Run(ctx context.Context, root, image string) (report interoplab.SuiteReport) {
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return setupFailure(err)
	}
	work, err := os.MkdirTemp(filepath.Join(root, paths.Scratch), "vpp-pppoe-mechanism-")
	if err != nil {
		return setupFailure(err)
	}
	defer func() {
		if err := os.RemoveAll(work); err != nil {
			if report.SetupError == "" {
				report.SetupError = fmt.Errorf("remove mechanism scratch: %w", err).Error()
			} else {
				report.SetupError = errors.Join(errors.New(report.SetupError), fmt.Errorf("remove mechanism scratch: %w", err)).Error()
			}
			report.Code = 1
		}
	}()
	if err := os.WriteFile(filepath.Join(work, "startup.conf"), []byte(startupConfig), 0o600); err != nil {
		return setupFailure(err)
	}
	relative, err := filepath.Rel(root, work)
	if err != nil {
		return setupFailure(err)
	}
	binaries := []interoplab.LabBinary{
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: filepath.Join(relative, "ze")},
		{Name: "le", Base: repofeaturetags.LEBase, Output: filepath.Join(relative, "le"), StartProbe: []string{helpCommand}},
	}
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, []string{buildvpp.ImageName, clientImage})
	if err != nil {
		return setupFailure(err)
	}
	docker := interoplab.NewDocker()
	var leasedImages []interoplab.ImageResult
	defer func() {
		releaseMechanismImages(ctx, docker, leasedImages, &report)
	}()
	preflight := interoplab.StageBinaries(root, false, binaries...)
	if image != "" {
		if !immutableImage(image) {
			return setupFailure(errors.New("image requires a complete sha256 image ID from the build report"))
		}
		if err := prepareVPPHost(ctx, docker, preflight, interoplab.StagedZePath(root, binaries)); err != nil {
			return setupFailure(err)
		}
		// Already staged before the client build; Suite repeats its Docker and
		// kernel checks using those same fresh binaries before starting peers.
		preflight = nil
		for index := range builds {
			if builds[index].Name == buildvpp.ImageName {
				builds[index].Tag = image
				continue
			}
			built, buildErr := docker.Build(ctx, builds[index])
			if buildErr != nil {
				return setupFailure(buildErr)
			}
			leasedImages = append(leasedImages, built)
			builds[index].Tag = built.Reference
		}
	}
	var plans []interoplab.ScenarioPlan
	for _, mode := range []mechanismMode{modePlain, modeNamespace, modeVLAN} {
		plan := mechanismPlan(root, work, strconv.Itoa(os.Getpid())+"-"+mode.name())
		plan.Source.Name = "pppoe-vpp-" + mode.name()
		plan.Source.Checker = func(ctx context.Context, check *interoplab.CheckContext) error {
			return checkMechanism(ctx, check, mode)
		}
		plans = append(plans, plan)
	}
	suite := interoplab.Suite{
		Docker: docker, Checkout: root, NoBuild: image != "",
		Preflight: preflight,
		StagedZe:  interoplab.StagedZePath(root, binaries),
		Images:    builds, Scenarios: plans,
	}
	return suite.Run(ctx)
}

func releaseMechanismImages(ctx context.Context, builder proofImageBuilder, images []interoplab.ImageResult, report *interoplab.SuiteReport) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), controlCommandBudget)
	defer cancel()
	for _, image := range slices.Backward(images) {
		if err := builder.ReleaseImage(cleanupCtx, image); err != nil {
			report.CleanupErrors = append(report.CleanupErrors, err.Error())
			report.Code = 1
		}
	}
}

func immutableImage(image string) bool {
	if len(image) != 71 {
		return false
	}
	if image[:7] != "sha256:" {
		return false
	}
	for _, character := range image[7:] {
		if character >= '0' {
			if character <= '9' {
				continue
			}
		}
		if character >= 'a' {
			if character <= 'f' {
				continue
			}
		}
		return false
	}
	return true
}

func setupFailure(err error) interoplab.SuiteReport {
	return interoplab.SuiteReport{SetupError: err.Error(), Code: 1}
}

// mechanismPlan gives data traffic an independent sender and mounts only this
// run's staged helper and evidence. The root parameter identifies the checkout
// to callers; no checkout copy or host-wide mount is needed by these peers.
func mechanismPlan(_, work, suffix string) interoplab.ScenarioPlan {
	helper := interoplab.Mount{Source: filepath.Join(work, "le"), Target: helperPath, ReadOnly: true}
	evidence := interoplab.Mount{Source: work, Target: evidencePath}
	return interoplab.ScenarioPlan{
		Source: interoplab.ScenarioSource{Name: "pppoe-vpp-mechanism", Directory: work},
		Network: interoplab.NetworkSpec{Name: "ze-vpp-pppoe-" + suffix, Candidates: []interoplab.Subnet{
			{IPv4: netip.MustParsePrefix("198.19.33.0/24")},
			{IPv4: netip.MustParsePrefix("198.19.34.0/24")},
		}},
		Peers: []interoplab.PeerConfig{
			// AF_PACKET locks 76 MiB for this peer's one default RX/TX ring.
			// Bound the allowance with headroom instead of granting IPC_LOCK.
			{Name: vppPeer, Container: "ze-vpp-pppoe-vpp-" + suffix, Image: buildvpp.ImageName, Host: 2,
				Capabilities: []string{networkAdminCapability, "SYS_ADMIN"}, Mounts: []interoplab.Mount{helper, evidence},
				LogFiles:  []string{vppEvidenceLog, "/evidence/vpp-start.log", "/evidence/control-launch.log", "/evidence/control.error", "/evidence/control.state", "/evidence/discovery.log"},
				Arguments: []string{"--ulimit", "memlock=268435456:268435456", "--device", "/dev/net/tun", "--device", "/dev/vhost-net", "--entrypoint", "sleep"}, Command: []string{"infinity"}},
			{Name: clientPeer, Container: "ze-vpp-pppoe-client-" + suffix, Image: clientImage, Host: 3,
				LogFiles:     []string{"/var/log/ppp/mechanism.log"},
				Capabilities: []string{networkAdminCapability}, Mounts: []interoplab.Mount{helper}, Arguments: interoplab.PPPDeviceArguments()},
			{Name: trafficPeer, Container: "ze-vpp-pppoe-traffic-" + suffix, Image: clientImage, Host: 4,
				Capabilities: []string{networkAdminCapability}, Mounts: []interoplab.Mount{helper}},
		},
	}
}

type mechanismMode uint8

const (
	modeUnspecified mechanismMode = iota
	modePlain
	modeNamespace
	modeVLAN
)

func (mode mechanismMode) name() string {
	switch mode {
	case modePlain:
		return "plain"
	case modeNamespace:
		return "namespace"
	case modeVLAN:
		return "vlan"
	case modeUnspecified:
		return "unspecified"
	default:
		panic("BUG: unknown mechanism mode")
	}
}

const startupConfig = `unix {
  nodaemon
  cli-listen /run/vpp/cli.sock
  log /evidence/vpp.log
}
socksvr { socket-name /run/vpp/api.sock }
plugins {
  plugin default { disable }
  plugin af_packet_plugin.so { enable }
  plugin linux_cp_plugin.so { enable }
  plugin pppoe_plugin.so { enable }
}
buffers { buffers-per-numa 4096 page-size default }
statseg { size 32M page-size default }
pppoe { enable-pass-nd-and-dhcpv6 }
`
