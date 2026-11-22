// Design: docs/labs/pppoe-interop.md -- actual Ze PPPoE over the native VPP provider.
// Related: product_check.go -- independent pppd and traffic assertions.
package testvpppppoe

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	buildvpp "github.com/ze-software/ze/internal/le/build/vpp"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// RunProduct deliberately shares the existing image registry, binary staging,
// Docker/kernel preflight and Suite resource lifetime. No helper impersonates
// the access concentrator: the staged daemon owns PPP, native sessions and FIB.
func RunProduct(ctx context.Context, root, image string) (report interoplab.SuiteReport) {
	return runProductSuite(ctx, root, image, productSuiteIPv4)
}

// productSuite names which acceptance package one product run carries.
type productSuite uint8

const (
	productSuiteUnspecified productSuite = iota
	// productSuiteIPv4 is the base: four IPv4 topologies with local auth.
	productSuiteIPv4
	// productSuiteIPv6 is the IPv6 service in three topologies.
	productSuiteIPv6
	// productSuiteFramed is one RADIUS subscriber with a non-peer Framed-Route.
	productSuiteFramed
)

// images names the registered images a suite's peers run.
func (suite productSuite) images() []string {
	names := []string{buildvpp.ImageName, clientImage}
	if suite == productSuiteFramed {
		names = append(names, interoplab.FreeRADIUSImageName)
	}
	return names
}

// plans builds the suite's scenario plans over this run's scratch.
func (suite productSuite) plans(root, work string) ([]interoplab.ScenarioPlan, error) {
	pid := strconv.Itoa(os.Getpid())
	var plans []interoplab.ScenarioPlan
	switch suite {
	case productSuiteIPv6:
		for _, mode := range []mechanismMode{modePlain, modeNamespace, modeVLAN} {
			plans = append(plans, productIPv6Plan(root, work, pid+"-"+mode.name(), mode))
		}
	case productSuiteIPv4:
		for _, mode := range []mechanismMode{modePlain, modeNamespace, modeVLAN} {
			plans = append(plans, productPlan(root, work, pid+"-"+mode.name(), mode, 1))
		}
		plans = append(plans, productPlan(root, work, pid+"-hundred", modePlain, 100))
	case productSuiteFramed:
		plan, err := productFramedPlan(root, work, pid+"-framed")
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	case productSuiteUnspecified:
		return nil, errors.New("BUG: product run names no acceptance suite")
	}
	return plans, nil
}

func runProductSuite(ctx context.Context, root, image string, suite productSuite) (report interoplab.SuiteReport) {
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return setupFailure(err)
	}
	work, err := os.MkdirTemp(filepath.Join(root, paths.Scratch), "vpp-pppoe-product-")
	if err != nil {
		return setupFailure(err)
	}
	defer func() {
		if err := os.RemoveAll(work); err != nil {
			report.SetupError = errors.Join(errors.New(report.SetupError), fmt.Errorf("remove product scratch: %w", err)).Error()
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
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, suite.images())
	if err != nil {
		return setupFailure(err)
	}
	docker := interoplab.NewDocker()
	var leased []interoplab.ImageResult
	defer func() { releaseMechanismImages(ctx, docker, leased, &report) }()
	preflight := interoplab.StageBinaries(root, false, binaries...)
	if image != "" {
		if !immutableImage(image) {
			return setupFailure(errors.New("image requires a complete sha256 image ID from the build report"))
		}
		if err := prepareVPPHost(ctx, docker, preflight, interoplab.StagedZePath(root, binaries)); err != nil {
			return setupFailure(err)
		}
		preflight = nil
		for index := range builds {
			if builds[index].Name == buildvpp.ImageName {
				builds[index].Tag = image
				continue
			}
			// Suite pulls nothing under NoBuild, so a pinned pulled peer
			// (FreeRADIUS) is made available here; its tag is its digest.
			if builds[index].Pull {
				if err := docker.Pull(ctx, builds[index].Tag, 0); err != nil {
					return setupFailure(err)
				}
				continue
			}
			built, err := docker.Build(ctx, builds[index])
			if err != nil {
				return setupFailure(err)
			}
			leased = append(leased, built)
			builds[index].Tag = built.Reference
		}
	}
	plans, err := suite.plans(root, work)
	if err != nil {
		return setupFailure(err)
	}
	return (&interoplab.Suite{Docker: docker, Checkout: root, Preflight: preflight, StagedZe: interoplab.StagedZePath(root, binaries), Images: builds, Scenarios: plans, NoBuild: image != ""}).Run(ctx)
}

func productPlan(root, work, suffix string, mode mechanismMode, count int) interoplab.ScenarioPlan {
	plan := mechanismPlan(root, work, "product-"+suffix)
	plan.Source.Name = "pppoe-vpp-ze-ac-pppd-client-" + mode.name()
	if mode == modePlain {
		plan.Source.Name = "pppoe-vpp-ze-ac-pppd-client"
	}
	if count == 100 {
		plan.Source.Name = "pppoe-vpp-ze-ac-pppd-client-hundred"
	}
	plan.Source.Checker = func(ctx context.Context, check *interoplab.CheckContext) error {
		return checkProduct(ctx, check, mode, count)
	}
	plan.Network.Candidates = []interoplab.Subnet{{IPv4: netip.MustParsePrefix("198.19.40.0/24")}, {IPv4: netip.MustParsePrefix("198.19.41.0/24")}}
	for index := range plan.Peers {
		mounts := plan.Peers[index].Mounts[:0]
		for _, mount := range plan.Peers[index].Mounts {
			if mount.Target != helperPath {
				mounts = append(mounts, mount)
			}
		}
		plan.Peers[index].Mounts = mounts
		if plan.Peers[index].Name == clientPeer {
			// The product dials log to productLog, not the mechanism lab's
			// file; the first dial's log shows how PPP negotiation ended.
			// The hundred lab's 101st dial and the capacity capture log
			// answer a refused-capacity failure; elsewhere they are absent.
			plan.Peers[index].LogFiles = []string{productLog(0), productLog(100), "/tmp/product-capacity.log"}
			continue
		}
		if plan.Peers[index].Name != vppPeer {
			continue
		}
		plan.Peers[index].Mounts = append(plan.Peers[index].Mounts,
			interoplab.Mount{Source: filepath.Join(work, "ze"), Target: "/product/ze", ReadOnly: true},
			interoplab.Mount{Source: filepath.Join(work, "le"), Target: productHelper, ReadOnly: true})
		plan.Peers[index].LogFiles = []string{vppEvidenceLog, "/evidence/vpp-start.log", "/evidence/product-ze.log", "/evidence/product-cp.log", "/evidence/product-discovery.log"}
		// VPP keeps its sessions, interfaces and the API calls Ze made only
		// in memory, so a failed product run asks it for them.
		plan.Peers[index].DiagnosticCommands = productVPPDiagnostics
	}
	return plan
}

// productLocalAuth is the base lab's subscriber authentication: Ze's own
// local user, so no RADIUS server takes part.
const productLocalAuth = `auth { local { user alice { password s3cr3t; } } }`

func productConfig(access, namespace string) string {
	return productConfigWith(access, namespace, productLocalAuth)
}

// productConfigWith is the product daemon configuration with auth as the
// l2tp authentication block, which PPPoE subscribers share.
func productConfigWith(access, namespace, auth string) string {
	return fmt.Sprintf(`%s
vpp {
 enabled true;
 external true;
 api-socket /run/vpp/api.sock;
 stats { socket-path /run/vpp/stats.sock; }
 lcp { enabled true; netns %q; }
}
pppoe {
 enabled true;
 ac-name ze-vpp-bng;
 service-name internet;
 auth-method chap-md5;
 max-sessions 100;
 padi-rate-limit 1000;
 interface %s { }
}
l2tp {
 %s
 pool { ipv4 { gateway 10.20.0.1; start 10.20.0.2; end 10.20.0.102;
 dns-primary 192.0.2.53; dns-secondary 192.0.2.54; } }
}
environment {
 api-server { token %q; rest { enabled true; server main { ip 127.0.0.1; port 9099; } } }
}
`, productInterfaces(access), namespace, access, auth, productToken)
}

// productInterfaces is the interface section that declares access, because
// Ze refuses a PPPoE access interface the config does not declare. access is
// the VPP name prepareProduct answers: the af_packet interface host-eth0, or
// its VLAN 100 sub-interface host-eth0.100, which a unit with that vlan-id
// declares. The lab creates both in VPP before Ze starts, and Ze adopts them:
// with no mtu and no unit of its own, the entry only sets host-eth0 up and
// leaves the address the lab gave it, and a VLAN unit whose device already
// exists is kept, never recreated (desiredState, applyConfigPass).
func productInterfaces(access string) string {
	parent, vlanID, vlan := strings.Cut(access, ".")
	var text textbuf.Buffer
	text.Str("interface {\n backend vpp;\n ethernet ").Str(parent).Str(" {")
	if vlan {
		text.Str(" unit ").Str(vlanID).Str(" { vlan-id ").Str(vlanID).Str("; }")
	}
	return text.Str(" }\n}").String()
}

// productVPPDiagnostics are the VPP CLI answers a failed product run prints:
// the PPPoE sessions and interfaces VPP holds, and the trace of the API
// messages Ze sent (startupConfig turns api-trace on).
var productVPPDiagnostics = [][]string{
	{vppctlCommand, "-s", vppCLIPath, showCommand, "pppoe", "session"},
	{vppctlCommand, "-s", vppCLIPath, showCommand, interfaceKeyword},
	{vppctlCommand, "-s", vppCLIPath, "api", "trace", "dump"},
}
