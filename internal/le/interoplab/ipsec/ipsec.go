// Design: test/interop-ipsec/run.py -- strongSwan scenario selection, images, and verdicts.
// Detail: test/interop-ipsec/lab.py -- container topology, config rendering, and cleanup.
// Related: checkers.go -- typed replacements for every scenario check.py.
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
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

const (
	// Gate is the forked Make gate this callable runner replaces.
	Gate = "ze-interop-ipsec-test"

	defaultFRRImage = "quay.io/frrouting/frr:10.3.1"
	zeImage         = "ze-ipsec-interop"
	swanImage       = "ze-ipsec-strongswan"

	zeCLIStore     = "/tmp/ze-cli-store"
	zeCLIUser      = "interop"
	zeCLIPassword  = "testpass"
	zeCLIPort      = "2222"
	zePasswordHash = "$2a$04$UlwuiuH82Unfsq.XEMPGJeDkXwbm3KW.nvVaVXOd/JeFK8VjMjrQO" // #nosec G101 -- published bcrypt hash for the fixture-only testpass account, not a secret.
)

var networkPrefix = netip.MustParsePrefix("172.28.0.0/24")

// Report is the structured result returned by the callable IPsec gate.
type Report struct {
	interoplab.SuiteReport
}

// Text preserves the producer's scenario and summary presentation.
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
	}

	images := []interoplab.ImageBuild{
		{Name: zePeer, Tag: zeImage, Dockerfile: filepath.Join(root, "test", "interop-ipsec", "Dockerfile.ze"), Context: root, Required: true},
		{Name: swanPeer, Tag: swanImage, Dockerfile: filepath.Join(root, "test", "interop-ipsec", "Dockerfile.strongswan"), Context: filepath.Join(root, "test", "interop-ipsec"), Required: true},
	}
	if needFRR {
		images = append(images, interoplab.ImageBuild{Name: frrPeer, Tag: environment.Image, Required: true, Pull: true})
	}

	suite := interoplab.Suite{
		Docker: docker,
		Preflight: func(buildContext context.Context, _ *interoplab.Docker) error {
			if environment.NoBuild {
				return nil
			}
			return buildZe(buildContext, root)
		},
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
	networkName := tb.Reset().Str("ze-ipsec-").Str(environment.Suffix).String()
	return interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       networkName,
			Candidates: []interoplab.Subnet{{IPv4: networkPrefix}},
		},
		Containers: []string{zeContainer, swanContainer, frrContainer},
		Prepare: func(_ context.Context, prepare interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
			return prepareScenario(root, source, state, zeContainer, swanContainer, frrContainer)
		},
	}
}

func prepareScenario(root string, source interoplab.ScenarioSource, state *scenarioState, zeContainer, swanContainer, frrContainer string) (interoplab.PreparedScenario, error) {
	scratchRoot := sessionpath.EnsureScratchRoot(root)
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

	peers := make([]interoplab.PeerConfig, 0, 3)
	if swanConfig := filepath.Join(source.Directory, "swanctl.conf"); fileExists(swanConfig) {
		mounts := []interoplab.Mount{{Source: swanConfig, Target: "/etc/swanctl/conf.d/interop.conf", ReadOnly: true}}
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
			Arguments: []string{"--privileged"},
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
		Arguments:   []string{"--privileged"},
		Command:     []string{"start", "/etc/ze/ze.conf"},
	})
	return interoplab.PreparedScenario{Peers: peers, Cleanup: cleanup}, nil
}

func buildZe(ctx context.Context, root string) error {
	tags, err := featureTags(filepath.Join(root, "feature-gates.txt"))
	if err != nil {
		return err
	}
	output := filepath.Join(root, "test", "interop-ipsec", "ze-linux")
	buildContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(buildContext, "go", "build", "-tags", strings.Join(tags, ","), "-o", output, "./cmd/ze") // #nosec G204 -- the fixed Go build consumes only checked-in feature tags and writes the fixed lab binary.
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	combined, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cross-compile ze: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	return nil
}

func featureTags(path string) ([]string, error) {
	content, err := readFileUnder(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("open feature gates: %w", err)
	}
	set := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[strings.Fields(line)[0]] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read feature gates: %w", err)
	}
	features := make([]string, 0, len(set))
	for feature := range set {
		features = append(features, feature)
	}
	sort.Strings(features)
	return append([]string{"ze_core", "ze_distro"}, features...), nil
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
	var body strings.Builder
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
		body.WriteString(line)
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
