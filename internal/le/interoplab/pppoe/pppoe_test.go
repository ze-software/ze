package pppoe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type fakeLab struct {
	execFn        func(string, []string) (interoplab.CommandResult, error)
	detachedFn    func(string, []string) error
	queryFn       func(string, []string) (string, error)
	logsFn        func(string, int) (interoplab.LogResult, error)
	stopFn        func(string, int) error
	detachedCalls [][]string
	execCalls     [][]string
	queryCalls    [][]string
	stopPeer      string
	stopGrace     int
}

func (f *fakeLab) Exec(
	_ context.Context,
	peer string,
	argv []string,
	_ []interoplab.EnvironmentVariable,
) (interoplab.CommandResult, error) {
	f.execCalls = append(f.execCalls, append([]string{peer}, argv...))
	if f.execFn == nil {
		return interoplab.CommandResult{}, errors.New("unexpected Exec")
	}
	return f.execFn(peer, argv)
}

func (f *fakeLab) ExecDetached(
	_ context.Context,
	peer string,
	argv []string,
	_ []interoplab.EnvironmentVariable,
) error {
	f.detachedCalls = append(f.detachedCalls, append([]string{peer}, argv...))
	if f.detachedFn == nil {
		return errors.New("unexpected ExecDetached")
	}
	return f.detachedFn(peer, argv)
}

func (f *fakeLab) Query(
	_ context.Context,
	peer string,
	argv []string,
	_ []interoplab.EnvironmentVariable,
) (string, error) {
	f.queryCalls = append(f.queryCalls, append([]string{peer}, argv...))
	if f.queryFn == nil {
		return "", errors.New("unexpected Query")
	}
	return f.queryFn(peer, argv)
}

func (f *fakeLab) Logs(
	_ context.Context,
	peer string,
	lines int,
) (interoplab.LogResult, error) {
	if f.logsFn == nil {
		return interoplab.LogResult{Available: true}, nil
	}
	return f.logsFn(peer, lines)
}

func (f *fakeLab) PeerPID(context.Context, string) (int, error) {
	return 0, errors.New("unexpected PeerPID")
}

func (f *fakeLab) Pause(context.Context, string) error {
	return errors.New("unexpected Pause")
}

func (f *fakeLab) Unpause(context.Context, string) error {
	return errors.New("unexpected Unpause")
}

func (f *fakeLab) Signal(context.Context, string, string) error {
	return errors.New("unexpected Signal")
}

func (f *fakeLab) Start(context.Context, string) error {
	return errors.New("unexpected Start")
}

func (f *fakeLab) Stop(_ context.Context, peer string, grace int) error {
	f.stopPeer = peer
	f.stopGrace = grace
	if f.stopFn == nil {
		return nil
	}
	return f.stopFn(peer, grace)
}

func TestZeClientScenarioProvesAccelAndDataplane(t *testing.T) {
	// VALIDATES: Ze completes CHAP/IPCP with accel-ppp, installs the point-to-point
	// address, forwards ICMP, and tears the accel-ppp session down on stop.
	// PREVENTS: A checker that accepts a local PPP interface without consulting
	// accel-ppp or exercising the data plane.
	stopped := false
	pinged := false
	lab := &fakeLab{}
	lab.execFn = func(peer string, argv []string) (interoplab.CommandResult, error) {
		command := strings.Join(argv, " ")
		switch command {
		case "ip -o link show type ppp":
			return interoplab.CommandResult{Stdout: "12: ppp0: <POINTOPOINT> mtu 1492\n"}, nil
		case "ip -o addr show dev ppp0":
			return interoplab.CommandResult{Stdout: "12: ppp0 inet 10.11.0.2 peer 10.11.0.1/32\n"}, nil
		case "ip -o route show 10.11.0.1 dev ppp0":
			return interoplab.CommandResult{Stdout: "10.11.0.1 dev ppp0 scope link src 10.11.0.2\n"}, nil
		case "ping -c 3 -W 3 10.11.0.1":
			pinged = true
			return interoplab.CommandResult{ExitCode: 0}, nil
		default:
			return interoplab.CommandResult{}, errors.New("unexpected command: " + peer + " " + command)
		}
	}
	lab.queryFn = func(peer string, argv []string) (string, error) {
		if peer != accelImageName || strings.Join(argv, " ") != "accel-cmd show sessions" {
			return "", errors.New("unexpected query")
		}
		if stopped {
			return "ifname | username\n", nil
		}
		return "ifname | username\nppp0 | alice\n", nil
	}
	lab.stopFn = func(peer string, grace int) error {
		stopped = true
		return nil
	}

	err := checkZeClient(context.Background(), &interoplab.CheckContext{Lab: lab})
	if err != nil {
		t.Fatalf("check Ze client scenario: %v", err)
	}
	if !pinged {
		t.Fatal("checker never exercised the PPPoE dataplane ping")
	}
	if lab.stopPeer != zeImageName || lab.stopGrace != 5 {
		t.Fatalf("stop = (%q, %d), want (ze, 5)", lab.stopPeer, lab.stopGrace)
	}
	if len(lab.queryCalls) < 2 {
		t.Fatalf("accel session queries = %d, want active and gone", len(lab.queryCalls))
	}
}

func TestAccelSessionQueryRefusesEmptyOutput(t *testing.T) {
	// VALIDATES: An unread accel-ppp session table fails closed.
	// PREVENTS: Empty peer output being interpreted as zero sessions.
	lab := &fakeLab{queryFn: func(string, []string) (string, error) { return "", nil }}
	_, err := accelSessionCount(context.Background(), lab)
	if err == nil || !strings.Contains(err.Error(), "empty output") {
		t.Fatalf("empty accel query error = %v", err)
	}
}

func TestPreflightRefusesUnmeasuredOrMissingKernelState(t *testing.T) {
	// VALIDATES: The host probe proves both /dev/ppp and PPPoE kernel support.
	// PREVENTS: Empty probe output or one positive check being accepted as full proof.
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{"empty", "", "/dev/ppp (PPP character device), pppoe (PPPoE pppox kernel module)"},
		{"device only", "DEV_PPP=ok\nPPPOE=missing\n", "pppoe (PPPoE pppox kernel module)"},
		{"module only", "DEV_PPP=missing\nPPPOE=ok\n", "/dev/ppp (PPP character device)"},
		{"both", "DEV_PPP=ok\nPPPOE=ok\n", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePreflightOutput(test.output)
			if test.want == "" {
				if err != nil {
					t.Fatalf("preflight error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("preflight error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAccessConcentratorFailureIncludesPPPDTrace(t *testing.T) {
	// VALIDATES: AC failures carry the negotiation trace redirected by pppd_dial.
	// PREVENTS: Docker logs alone hiding discovery, LCP, CHAP, and IPCP diagnostics.
	lab := &fakeLab{execFn: func(peer string, argv []string) (interoplab.CommandResult, error) {
		if peer != clientImageName ||
			strings.Join(argv, " ") != "sh -c cat /var/log/ppp/dial.log 2>/dev/null" {
			return interoplab.CommandResult{}, errors.New("unexpected diagnostic command")
		}
		return interoplab.CommandResult{Stdout: "rcvd [CHAP Failure id=3]\n"}, nil
	}}
	err := appendPPPDLog(context.Background(), lab, errors.New("auth failed"))
	if err == nil ||
		!strings.Contains(err.Error(), "auth failed") ||
		!strings.Contains(err.Error(), "rcvd [CHAP Failure id=3]") {
		t.Fatalf("diagnostic error = %v", err)
	}
}

func TestZeAccessConcentratorScenarioExercisesEveryStage(t *testing.T) {
	// VALIDATES: A real pppd-shaped trace proves discovery, bidirectional LCP,
	// CHAP success and refusal, IPCP, positive ICMP, PADT, and empty teardown state.
	// PREVENTS: A success-only AC check that never dials the wrong credential.
	goodLog := strings.Join([]string{
		"PADI",
		"PADO",
		"PADR",
		"PADS",
		"sent [LCP ConfReq id=1 <mru 1492> <magic 0x1111>]",
		"rcvd [LCP ConfAck id=1]",
		"rcvd [LCP ConfReq id=2 <mru 1492> <magic 0x2222> <auth chap MD5>]",
		"sent [LCP ConfAck id=2]",
		"rcvd [CHAP Challenge id=3]",
		"sent [CHAP Response id=3 name = \"alice\"]",
		"rcvd [CHAP Success id=3]",
		"rcvd [IPCP ConfAck id=4]",
	}, "\n")
	rejectedLog := strings.Join([]string{
		"PADO",
		"PADS",
		"rcvd [LCP ConfReq id=2 <auth chap MD5>]",
		"sent [LCP ConfAck id=2]",
		"rcvd [LCP ConfAck id=1]",
		"rcvd [CHAP Challenge id=3]",
		"rcvd [CHAP Failure id=3]",
	}, "\n")

	restCalls := 0
	logCalls := 0
	dials := 0
	pinged := false
	stopped := false
	lab := &fakeLab{}
	lab.logsFn = func(peer string, _ int) (interoplab.LogResult, error) {
		if peer == zeImageName {
			return interoplab.LogResult{Text: "PPPoE interface configured", Available: true}, nil
		}
		return interoplab.LogResult{Available: true}, nil
	}
	lab.detachedFn = func(peer string, argv []string) error {
		if peer != clientImageName || len(argv) != 3 || argv[0] != "sh" || argv[1] != "-c" {
			return errors.New("unexpected detached pppd command")
		}
		dials++
		stopped = dials == 2
		return nil
	}
	lab.execFn = func(peer string, argv []string) (interoplab.CommandResult, error) {
		command := strings.Join(argv, " ")
		switch command {
		case "sh -c rm -f /var/log/ppp/dial.log":
			return interoplab.CommandResult{}, nil
		case "ip -o link show type ppp":
			if dials == 1 && !stopped {
				return interoplab.CommandResult{Stdout: "9: ppp0: <POINTOPOINT>\n"}, nil
			}
			return interoplab.CommandResult{}, nil
		case "ip -o addr show dev ppp0":
			return interoplab.CommandResult{Stdout: "9: ppp0 inet 10.20.0.2 peer 10.20.0.1/32\n"}, nil
		case "ip -o route show 10.20.0.1 dev ppp0":
			return interoplab.CommandResult{Stdout: "10.20.0.1 dev ppp0 scope link src 10.20.0.2\n"}, nil
		case "ping -c 3 -W 3 -I ppp0 10.20.0.1":
			pinged = true
			return interoplab.CommandResult{}, nil
		case "pkill -TERM -x pppd":
			stopped = true
			return interoplab.CommandResult{}, nil
		case "pgrep -x pppd":
			if stopped || dials == 2 {
				return interoplab.CommandResult{ExitCode: 1}, errors.New("exit 1")
			}
			return interoplab.CommandResult{}, nil
		case "sh -c cat /var/log/ppp/dial.log 2>/dev/null":
			return interoplab.CommandResult{Stdout: goodLog}, nil
		default:
			return interoplab.CommandResult{}, errors.New("unexpected command: " + peer + " " + command)
		}
	}
	lab.queryFn = func(peer string, argv []string) (string, error) {
		command := strings.Join(argv, " ")
		if peer == zeImageName && strings.HasPrefix(command, "curl ") {
			restCalls++
			if restCalls == 2 {
				return `{"status":"done","data":[{"sid":7,"service-name":"internet","interface":"eth0"}]}`, nil
			}
			return `{"status":"done","data":[]}`, nil
		}
		if peer == clientImageName && command == "sh -c cat /var/log/ppp/dial.log 2>/dev/null" {
			logCalls++
			switch logCalls {
			case 1:
				return goodLog, nil
			case 2:
				return goodLog + "\nSent PADT\n", nil
			default:
				return rejectedLog, nil
			}
		}
		return "", errors.New("unexpected query: " + peer + " " + command)
	}

	err := checkZeAccessConcentrator(
		context.Background(),
		&interoplab.CheckContext{Lab: lab},
	)
	if err != nil {
		t.Fatalf("check Ze access concentrator: %v", err)
	}
	if !pinged {
		t.Fatal("checker never exercised client-to-Ze ICMP")
	}
	if len(lab.detachedCalls) != 2 {
		t.Fatalf("pppd dials = %d, want success and refusal", len(lab.detachedCalls))
	}
	wantGood := "exec pppd plugin pppoe.so nic-eth0 user alice password s3cr3t noauth " +
		"refuse-pap refuse-eap refuse-mschap refuse-mschap-v2 noipdefault nodefaultroute " +
		"noaccomp nopcomp mtu 1492 mru 1492 lcp-echo-interval 10 lcp-echo-failure 5 " +
		"maxfail 1 nodetach debug rp_pppoe_service internet >/var/log/ppp/dial.log 2>&1"
	wantBad := strings.Replace(wantGood, "password s3cr3t", "password wrong-secret", 1)
	if got := lab.detachedCalls[0][3]; got != wantGood {
		t.Fatalf("good pppd argv shell = %q\nwant %q", got, wantGood)
	}
	if got := lab.detachedCalls[1][3]; got != wantBad {
		t.Fatalf("bad pppd argv shell = %q\nwant %q", got, wantBad)
	}
}

func TestRESTSessionQueryRefusesEmptyOutput(t *testing.T) {
	// VALIDATES: An unread Ze REST response fails closed.
	// PREVENTS: Empty peer output being interpreted as an empty session list.
	lab := &fakeLab{queryFn: func(string, []string) (string, error) { return "", nil }}
	_, err := zeSessions(context.Background(), lab)
	if err == nil || !strings.Contains(err.Error(), "empty output") {
		t.Fatalf("empty REST query error = %v", err)
	}
}

func TestScenarioPlansPreserveImagesConfigsAndArguments(t *testing.T) {
	// VALIDATES: The native suite builds the same three images and mounts the
	// producer's config files with the same container argv.
	// PREVENTS: A native port that silently substitutes a different peer or config.
	root := t.TempDir()
	suite := filepath.Join(root, suitePath)
	scenarioClient := filepath.Join(suite, "scenarios", "01-pppoe-chap-ipv4")
	scenarioAC := filepath.Join(suite, "scenarios", "02-ze-ac-pppd-client")
	for _, file := range []string{
		filepath.Join(scenarioClient, "accel-ppp.conf"),
		filepath.Join(scenarioClient, "chap-secrets"),
		filepath.Join(scenarioClient, "ze.conf"),
		filepath.Join(scenarioAC, "ze.conf"),
	} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(scenarioClient, "role"), []byte(roleZeClient+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioAC, "role"), []byte(roleZeAC+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	source := interoplab.ScenarioSource{
		Name:      "01-pppoe-chap-ipv4",
		Directory: scenarioClient,
		Checker:   checkZeClient,
	}
	plan, err := scenarioPlan(source, "parity")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Network.Name != "ze-pppoe-parity" ||
		len(plan.Network.Candidates) != 1 ||
		plan.Network.Candidates[0].IPv4.String() != "172.30.0.0/24" {
		t.Fatalf("network plan = %#v", plan.Network)
	}
	wantContainers := []string{
		"ze-pppoe-ze-parity",
		"ze-pppoe-accel-parity",
		"ze-pppoe-client-parity",
	}
	if !reflect.DeepEqual(plan.Containers, wantContainers) {
		t.Fatalf("cleanup containers = %v, want %v", plan.Containers, wantContainers)
	}

	images := imageBuilds(root)
	wantTags := []string{zeImageTag, accelImageTag, clientImageTag}
	gotTags := []string{images[0].Tag, images[1].Tag, images[2].Tag}
	if !reflect.DeepEqual(gotTags, wantTags) {
		t.Fatalf("image tags = %v, want %v", gotTags, wantTags)
	}
	if images[0].Context != root || images[1].Context != suite || images[2].Context != suite {
		t.Fatalf("image contexts = %#v", images)
	}
	wantTimeouts := []time.Duration{10 * time.Minute, 15 * time.Minute, 10 * time.Minute}
	gotTimeouts := []time.Duration{images[0].Timeout, images[1].Timeout, images[2].Timeout}
	if !reflect.DeepEqual(gotTimeouts, wantTimeouts) {
		t.Fatalf("image timeouts = %v, want %v", gotTimeouts, wantTimeouts)
	}

	clientPeers, err := prepareZeClient(
		interoplab.ScenarioSource{Name: "01-pppoe-chap-ipv4", Directory: scenarioClient},
		containerNames("parity"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if clientPeers[0].Image != accelImageName || clientPeers[1].Image != zeImageName {
		t.Fatalf("Ze-client peer order = %q then %q", clientPeers[0].Image, clientPeers[1].Image)
	}
	if got := clientPeers[0].Ready.Command; !reflect.DeepEqual(got, []string{"accel-cmd", "show", "stat"}) {
		t.Fatalf("accel readiness argv = %v", got)
	}
	if got := clientPeers[1].Arguments; !reflect.DeepEqual(got, []string{
		"--privileged", "-e", "ze.log.interface=debug", "-e", "ZE_STORAGE_BLOB=false",
	}) {
		t.Fatalf("Ze-client docker arguments = %v", got)
	}
	if got := clientPeers[1].Command; !reflect.DeepEqual(got, []string{"start", "/etc/ze/ze.conf"}) {
		t.Fatalf("Ze-client command = %v", got)
	}
	if clientPeers[0].Mounts[0].Source != filepath.Join(scenarioClient, "accel-ppp.conf") ||
		clientPeers[0].Mounts[1].Source != filepath.Join(scenarioClient, "chap-secrets") ||
		clientPeers[1].Mounts[0].Source != filepath.Join(scenarioClient, "ze.conf") {
		t.Fatalf("Ze-client mounts = %#v / %#v", clientPeers[0].Mounts, clientPeers[1].Mounts)
	}

	acPeers, err := prepareZeAccessConcentrator(
		interoplab.ScenarioSource{Name: "02-ze-ac-pppd-client", Directory: scenarioAC},
		containerNames("parity"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if acPeers[0].Image != zeImageName || acPeers[1].Image != clientImageName {
		t.Fatalf("Ze-AC peer order = %q then %q", acPeers[0].Image, acPeers[1].Image)
	}
	wantACArguments := []string{
		"--privileged",
		"-e", "ze.log.pppoe=debug",
		"-e", "ze.log.l2tp=debug",
		"-e", "ZE_STORAGE_BLOB=false",
	}
	if !reflect.DeepEqual(acPeers[0].Arguments, wantACArguments) {
		t.Fatalf("Ze-AC docker arguments = %v, want %v", acPeers[0].Arguments, wantACArguments)
	}
}

func TestPPPDFailureStagePreservesEveryDiagnosticBranch(t *testing.T) {
	// VALIDATES: Every producer diagnostic branch remains callable natively.
	// PREVENTS: Collapsing discovery, LCP, auth, and IPCP failures into one timeout.
	tests := []struct {
		name string
		log  string
		want string
	}{
		{"no PADO", "", "discovery: the AC sent no PADO"},
		{"no PADS", "PADO", "discovery: the AC sent no PADS"},
		{"auth refused", "PADO PADS rcvd [CHAP Failure", "auth: the AC refused the credential"},
		{"no AC LCP request", "PADO PADS", "lcp: the AC never sent its own Configure-Request"},
		{"missing ack", "PADO PADS rcvd [LCP ConfReq", "lcp: a Configure-Ack is missing in one direction"},
		{
			"auth never started",
			"PADO PADS rcvd [LCP ConfReq sent [LCP ConfAck rcvd [LCP ConfAck",
			"auth: the AC asked for a method and never started it",
		},
		{
			"no IPCP",
			"PADO PADS rcvd [LCP ConfReq sent [LCP ConfAck rcvd [LCP ConfAck rcvd [CHAP Challenge",
			"ipcp: no address was agreed",
		},
		{
			"after IPCP",
			"PADO PADS rcvd [LCP ConfReq sent [LCP ConfAck rcvd [LCP ConfAck rcvd [CHAP Challenge rcvd [IPCP ConfAck",
			"after IPCP",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := pppdFailureStage(test.log); got != test.want {
				t.Fatalf("stage = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScenarioRoleFailsClosed(t *testing.T) {
	// VALIDATES: Each scenario explicitly selects Ze client or access-concentrator topology.
	// PREVENTS: A missing or unknown role silently running the wrong peer layout.
	directory := t.TempDir()
	if _, err := readRole(directory); err == nil || !strings.Contains(err.Error(), "missing role file") {
		t.Fatalf("missing role error = %v", err)
	}
	rolePath := filepath.Join(directory, "role")
	if err := os.WriteFile(rolePath, []byte("other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRole(directory); err == nil || !strings.Contains(err.Error(), "unknown role") {
		t.Fatalf("unknown role error = %v", err)
	}
	for _, role := range []string{roleZeClient, roleZeAC} {
		if err := os.WriteFile(rolePath, []byte(role+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readRole(directory)
		if err != nil || got != role {
			t.Fatalf("role = %q, error = %v, want %q", got, err, role)
		}
	}
}
