// Design: docs/architecture/testing/interop.md -- native strongSwan scenario selection, images, topology, and verdicts.
// Detail: checkers.go and helpers.go carry every typed protocol observation.
// Related: test/interop-ipsec/parity_test.go pins the complete fixture population.
package ipsec

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	// Action is the native action identity served by this callable runner.
	Action = "integration/interop-ipsec"

	defaultFRRImage = "quay.io/frrouting/frr:10.3.1"
	zeImage         = "ze-ipsec-interop"
	swanImage       = "ze-ipsec-strongswan"
	natImage        = "ze-ipsec-nat"

	// dockerPrivileged is what every peer of this lab needs: XFRM state, XFRM policy
	// and netfilter NAT rules are all privileged operations inside a container.
	dockerPrivileged = "--privileged"

	zeCLIStore     = "/tmp/ze-cli-store"
	zeCLIUser      = "interop"
	zeCLIPassword  = "testpass"
	zeCLIPort      = "2222"
	zePasswordHash = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO" // #nosec G101 -- published bcrypt hash for the fixture-only testpass account, not a secret.

	// swanLabConfig holds the charon settings every strongSwan peer in this lab
	// needs, and today that is one: bypass-lan off, without which the PASS shunt
	// for the lab subnet outranks the Child SA policy and no scenario can observe
	// protected traffic. It sorts BEFORE a scenario's own 99-interop.conf, so a
	// scenario drop-in still composes with it.
	swanLabConfig = "strongswan-lab.conf"
	swanLabTarget = "/etc/strongswan.d/98-lab.conf"
)

var networkPrefix = netip.MustParsePrefix("172.28.0.0/24")

// Report is the structured result returned by the callable IPsec gate.
type Report struct {
	interoplab.SuiteReport
}

// Text renders the native scenario and summary presentation.
func (r Report) Text() string {
	var out textbuf.Buffer
	out.SetColor(slogutil.UseColor(os.Stdout))
	color := textbuf.C
	out.Str("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	out.Str(" Ze IPsec Interop Lab\n")
	out.Str("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	if r.SetupError != "" {
		out.Str("  ").Colored(color.BoldRed).Str("✗ FAIL: ").Str(r.SetupError).
			Colored(color.Reset).Byte('\n')
	}
	for _, scenario := range r.Scenarios {
		out.Str("── ").Str(scenario.Name).Str(" ──\n")
		if scenario.Passed {
			out.Str("  ").Colored(color.BrightGreen).Str("✓ PASS").
				Colored(color.Reset).Str("\n\n")
			continue
		}
		out.Str("  ").Colored(color.BoldRed).Str("✗ FAIL: ").Str(scenario.Error).
			Colored(color.Reset).Str("\n\n")
	}
	out.Str("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	if r.Code == 0 {
		out.Colored(color.BrightGreen).Str("PASS  ").Int(int64(r.Passed)).
			Str(" scenario(s)").Colored(color.Reset).Byte('\n')
	} else {
		out.Colored(color.BoldRed).Str("FAIL  ").Int(int64(r.Passed)).Str(" passed, ").
			Int(int64(r.Failed)).Str(" failed: ").Join(r.FailedNames, " ").
			Colored(color.Reset).Byte('\n')
	}
	out.Str("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return out.String()
}

// LabBinaries declares the one binary this lab stages into its Docker build
// context, which is what test/interop-ipsec/Dockerfile.ze copies in.
//
// The base is featuretags.DaemonBase, which is the same pair the deleted local
// copy of this producer prepended by hand. What that copy did NOT have is the
// manifest read's refusal: it parsed feature-gates.txt itself and returned the
// base alone for a manifest declaring no gate, where featuretags.DaemonTags
// answers ErrNoGateTags. A silent base-only answer builds a daemon with every
// feature compiled out, which then dies on "unknown top-level keyword" for the
// protocol the scenario was about to prove.
func LabBinaries() []interoplab.LabBinary {
	return []interoplab.LabBinary{
		{Name: "ze", Base: featuretags.DaemonBase, Output: "test/interop-ipsec/ze-linux"},
	}
}

// Run resolves the checkout and invokes the native gate.
func Run() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		report := Report{SetupError: err.Error(), Code: 1}
		return report, 1
	}
	report, code := RunAt(context.Background(), root)
	return report, code
}

// RunAt runs the selected strongSwan scenarios against root.
func RunAt(ctx context.Context, root string) (Report, int) {
	environment := interoplab.ReadEnvironment(interoplab.EnvironmentOptions{
		SelectorVariable: "IPSEC_INTEROP_SCENARIO",
		SuffixVariable:   "ZE_IPSEC_INTEROP_SUFFIX",
		DefaultImage:     defaultFRRImage,
		DefaultSuffix:    strconv.Itoa(os.Getpid()),
	})
	return runAt(ctx, root, environment, interoplab.NewDocker())
}

func runAt(ctx context.Context, root string, environment interoplab.Environment, docker *interoplab.Docker) (Report, int) {
	scenarioRoot := filepath.Join(root, "test", "interop-ipsec", "scenarios")
	sources, err := interoplab.Discover(scenarioRoot, environment.Selector, checkerAdapters())
	if err != nil {
		report := Report{SetupError: err.Error(), Code: 1}
		return report, 1
	}

	plans := make([]interoplab.ScenarioPlan, 0, len(sources))
	needFRR := false
	needNAT := false
	for _, source := range sources {
		state := &scenarioState{}
		checker := scenarioCheckers[source.Name]
		source.Checker = func(ctx context.Context, checkContext *interoplab.CheckContext) error {
			return checker(ctx, newScenarioLab(checkContext, environment.SessionTimeout, state))
		}
		plan := scenarioPlan(root, environment, source, state)
		plans = append(plans, plan)
		if fileExists(filepath.Join(source.Directory, "frr.conf")) {
			needFRR = true
		}
		if fileExists(filepath.Join(source.Directory, natConfigName)) {
			needNAT = true
		}
	}

	images := []interoplab.ImageBuild{
		{Name: zePeer, Tag: zeImage, Dockerfile: filepath.Join(root, "test", "interop-ipsec", "Dockerfile.ze"), Context: root, Required: true},
		{Name: swanPeer, Tag: swanImage, Dockerfile: filepath.Join(root, "test", "interop-ipsec", "Dockerfile.strongswan"), Context: filepath.Join(root, "test", "interop-ipsec"), Required: true},
	}
	if needFRR {
		images = append(images, interoplab.ImageBuild{Name: frrPeer, Tag: environment.Image, Required: true, Pull: true})
	}
	if needNAT {
		images = append(images, interoplab.ImageBuild{
			Name:       natPeer,
			Tag:        natImage,
			Dockerfile: filepath.Join(root, "test", "interop-ipsec", "Dockerfile.nat"),
			Context:    filepath.Join(root, "test", "interop-ipsec"),
			Required:   true,
		})
	}

	suite := interoplab.Suite{
		Docker:    docker,
		Preflight: interoplab.StageBinaries(root, environment.NoBuild, LabBinaries()...),
		Images:    images,
		Scenarios: plans,
		NoBuild:   environment.NoBuild,
	}
	suiteReport := suite.Run(ctx)
	report := Report{SuiteReport: suiteReport}
	return report, suiteReport.Code
}

type scenarioState struct {
	root           string
	renderedConfig string
}

func scenarioPlan(root string, environment interoplab.Environment, source interoplab.ScenarioSource, state *scenarioState) interoplab.ScenarioPlan {
	var tb textbuf.Buffer
	zeContainer := tb.Str("ze-ipsec-ze-").Str(environment.Suffix).String()
	swanContainer := tb.Reset().Str("ze-ipsec-swan-").Str(environment.Suffix).String()
	frrContainer := tb.Reset().Str("ze-ipsec-frr-").Str(environment.Suffix).String()
	natContainer := tb.Reset().Str("ze-ipsec-nat-").Str(environment.Suffix).String()
	networkName := tb.Reset().Str("ze-ipsec-").Str(environment.Suffix).String()
	return interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       networkName,
			Candidates: []interoplab.Subnet{{IPv4: networkPrefix}},
		},
		Containers: []string{zeContainer, swanContainer, frrContainer, natContainer},
		Prepare: func(_ context.Context, prepare interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
			return prepareScenario(root, source, state, zeContainer, swanContainer, frrContainer, natContainer)
		},
	}
}

func prepareScenario(root string, source interoplab.ScenarioSource, state *scenarioState, zeContainer, swanContainer, frrContainer, natContainer string) (interoplab.PreparedScenario, error) {
	scratchRoot := filepath.Join(root, "tmp", interoplab.RenderedConfigDirectory)
	if err := os.MkdirAll(scratchRoot, 0o750); err != nil {
		return interoplab.PreparedScenario{}, fmt.Errorf("create rendered config directory: %w", err)
	}
	var tb textbuf.Buffer
	pattern := tb.Str("ze-ipsec-").Str(source.Name).Byte('-').String()
	workDir, err := os.MkdirTemp(scratchRoot, pattern)
	if err != nil {
		return interoplab.PreparedScenario{}, fmt.Errorf("create scenario work directory: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(workDir) }
	fail := func(cause error) (interoplab.PreparedScenario, error) {
		return interoplab.PreparedScenario{Cleanup: cleanup}, cause
	}

	pkiDir := findPKIDir(root, source.Directory)
	renderedConfig := filepath.Join(workDir, "ze.conf")
	if err := renderZeConfig(filepath.Join(source.Directory, "ze.conf"), pkiDir, renderedConfig); err != nil {
		return fail(err)
	}
	state.root = root
	state.renderedConfig = renderedConfig

	peers := make([]interoplab.PeerConfig, 0, 4)

	// The NAT box is FIRST, so its secondary addresses answer ARP before either
	// daemon sends its first IKE datagram. A peer that started ahead of it would
	// retry, which turns a scenario's first seconds into a race.
	if natConfig := filepath.Join(source.Directory, natConfigName); fileExists(natConfig) {
		translations, natErr := readNATConfig(source.Directory)
		if natErr != nil {
			return fail(natErr)
		}
		peers = append(peers, interoplab.PeerConfig{
			Name:      natPeer,
			Container: natContainer,
			Image:     natPeer,
			Host:      natHost,
			Arguments: []string{dockerPrivileged},
			Command:   []string{"-c", natSetupScript(translations)},
			Ready: &interoplab.ReadyProbe{
				Command:  []string{"sh", "-c", "iptables -t nat -S | grep -q SNAT"},
				Timeout:  30 * time.Second,
				Interval: time.Second,
			},
		})
	}

	if swanConfig := filepath.Join(source.Directory, "swanctl.conf"); fileExists(swanConfig) {
		mounts := []interoplab.Mount{
			{Source: swanConfig, Target: "/etc/swanctl/conf.d/interop.conf", ReadOnly: true},
			{Source: filepath.Join(root, "test", "interop-ipsec", swanLabConfig), Target: swanLabTarget, ReadOnly: true},
		}
		if daemonConfig := filepath.Join(source.Directory, "strongswan.conf"); fileExists(daemonConfig) {
			mounts = append(mounts, interoplab.Mount{Source: daemonConfig, Target: "/etc/strongswan.d/99-interop.conf", ReadOnly: true})
		}
		if pkiDir != "" {
			for _, mount := range []struct{ source, target string }{
				{"server.pem", "/etc/swanctl/x509/server.pem"},
				{"server-key.pem", "/etc/swanctl/private/server-key.pem"},
				{"ca.pem", "/etc/swanctl/x509ca/ca.pem"},
			} {
				path := filepath.Join(pkiDir, mount.source)
				if !fileExists(path) {
					return fail(fmt.Errorf("missing PKI file %s", path))
				}
				mounts = append(mounts, interoplab.Mount{Source: path, Target: mount.target, ReadOnly: true})
			}
		}
		peers = append(peers, interoplab.PeerConfig{
			Name:      swanPeer,
			Container: swanContainer,
			Image:     swanPeer,
			Host:      3,
			Mounts:    mounts,
			Arguments: []string{dockerPrivileged},
			Ready: &interoplab.ReadyProbe{
				Command:  []string{"sh", "-c", "swanctl --stats | grep -q uptime && swanctl --load-all >/dev/null"},
				Timeout:  30 * time.Second,
				Interval: time.Second,
			},
		})
	}

	if frrConfig := filepath.Join(source.Directory, "frr.conf"); fileExists(frrConfig) {
		peers = append(peers, interoplab.PeerConfig{
			Name:      frrPeer,
			Container: frrContainer,
			Image:     frrPeer,
			Host:      4,
			Mounts: []interoplab.Mount{
				{Source: frrConfig, Target: "/etc/frr/frr.conf", ReadOnly: true},
				{Source: filepath.Join(root, "test", "interop-ipsec", "daemons"), Target: "/etc/frr/daemons", ReadOnly: true},
				{Source: filepath.Join(root, "test", "interop-ipsec", "vtysh.conf"), Target: "/etc/frr/vtysh.conf", ReadOnly: true},
			},
			Capabilities: []string{"NET_ADMIN", "SYS_ADMIN"},
		})
	}

	environment, err := zeEnvironment(source.Directory)
	if err != nil {
		return fail(err)
	}
	peers = append(peers, interoplab.PeerConfig{
		Name:        zePeer,
		Container:   zeContainer,
		Image:       zePeer,
		Host:        2,
		Mounts:      []interoplab.Mount{{Source: renderedConfig, Target: "/etc/ze/ze.conf", ReadOnly: true}},
		Environment: environment,
		Arguments:   []string{dockerPrivileged},
		Command:     []string{"start", "/etc/ze/ze.conf"},
	})
	return interoplab.PreparedScenario{Peers: peers, Cleanup: cleanup}, nil
}

func zeEnvironment(directory string) ([]interoplab.EnvironmentVariable, error) {
	variables := []interoplab.EnvironmentVariable{
		{Name: "ZE_STORAGE_BLOB", Value: "false"},
		{Name: "ZE_LOG_LEVEL", Value: "debug"},
	}
	content, err := readFileUnder(directory, "ze-env")
	if errors.Is(err, os.ErrNotExist) {
		return variables, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ze-env: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("ze-env line in %s is not KEY=VALUE: %q", filepath.Base(directory), line)
		}
		variables = append(variables, interoplab.EnvironmentVariable{Name: name, Value: value})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ze-env: %w", err)
	}
	return variables, nil
}

func renderZeConfig(source, pkiDir, destination string) error {
	content, err := readFileUnder(filepath.Dir(source), filepath.Base(source))
	if err != nil {
		return fmt.Errorf("read ze config: %w", err)
	}
	resolved, err := resolvePKIPlaceholders(string(content), pkiDir)
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	text := tb.Str(strings.TrimRight(resolved, "\n")).Byte('\n').Str(zeCLIConfig()).String()
	if err := os.WriteFile(destination, []byte(text), 0o600); err != nil {
		return fmt.Errorf("write rendered ze config: %w", err)
	}
	return nil
}

func zeCLIConfig() string {
	var tb textbuf.Buffer
	return tb.Str("\nsystem {\n\tauthentication {\n\t\tuser ").Str(zeCLIUser).
		Str(" {\n\t\t\tpassword ").Quoted(zePasswordHash).
		Str("\n\t\t}\n\t}\n}\n\nenvironment {\n\tssh {\n\t\tenabled true\n").
		Str("\t\tserver main {\n\t\t\tip 127.0.0.1;\n\t\t\tport ").Str(zeCLIPort).
		Str(";\n\t\t}\n\t}\n}\n").String()
}

func resolvePKIPlaceholders(content, pkiDir string) (string, error) {
	matches := pkiPlaceholder.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}
	if pkiDir == "" {
		return "", errors.New("config needs PKI material and the scenario has no pki directory")
	}
	cache := make(map[string]string)
	for _, match := range matches {
		name := match[1]
		if _, ok := cache[name]; ok {
			continue
		}
		path := filepath.Join(pkiDir, name)
		text, err := readFileUnder(pkiDir, name)
		if err != nil {
			return "", fmt.Errorf("cannot read PKI file %s: %w", path, err)
		}
		body, err := pemBase64DER(string(text), path)
		if err != nil {
			return "", err
		}
		cache[name] = body
	}
	return pkiPlaceholder.ReplaceAllStringFunc(content, func(token string) string {
		name := pkiPlaceholder.FindStringSubmatch(token)[1]
		return cache[name]
	}), nil
}

func pemBase64DER(text, source string) (string, error) {
	type block struct{ label, body string }
	var blocks []block
	label := ""
	var body textbuf.Buffer
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "-----BEGIN ") && strings.HasSuffix(line, "-----") {
			label = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "-----BEGIN "), "-----"))
			if label == "ENCRYPTED PRIVATE KEY" {
				return "", fmt.Errorf("%s: PEM block %q holds encrypted key material", source, label)
			}
			body.Reset()
			continue
		}
		if strings.HasPrefix(line, "-----END ") && strings.HasSuffix(line, "-----") {
			if label == "" {
				continue
			}
			if label != "EC PARAMETERS" {
				if body.Len() == 0 {
					return "", fmt.Errorf("%s: PEM block %q holds no data", source, label)
				}
				blocks = append(blocks, block{label: label, body: body.String()})
			}
			label = ""
			body.Reset()
			continue
		}
		if label == "" || line == "" {
			continue
		}
		if pemHeader.MatchString(line) {
			return "", fmt.Errorf("%s: PEM block %q carries encrypted RFC 1421 header %q", source, label, line)
		}
		body.Str(line)
	}
	if len(blocks) == 0 {
		return "", fmt.Errorf("%s: no complete PEM block found", source)
	}
	if len(blocks) != 1 {
		return "", fmt.Errorf("%s: the file holds %d PEM blocks, and a pki leaf holds one value", source, len(blocks))
	}
	if _, err := base64.StdEncoding.DecodeString(blocks[0].body); err != nil {
		return "", fmt.Errorf("%s: PEM block %q does not hold valid base64: %w", source, blocks[0].label, err)
	}
	return blocks[0].body, nil
}

func findPKIDir(root, scenarioDir string) string {
	local := filepath.Join(scenarioDir, "pki")
	if directoryExists(local) {
		return local
	}
	shared := filepath.Join(root, "test", "interop-ipsec", "pki")
	if directoryExists(shared) && fileExists(filepath.Join(shared, "ca.pem")) {
		return shared
	}
	return ""
}

func readFileUnder(directory, name string) ([]byte, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	content, readErr := root.ReadFile(name)
	return content, errors.Join(readErr, root.Close())
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
